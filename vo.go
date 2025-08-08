package dvo

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"time"

	"github.com/kcmvp/dvo/constraint"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/tidwall/gjson"
)

// A private type to prevent key collisions in context.
type viewObjectKeyType struct{}

// ViewObjectKey is the key used to store the validated valueObject map in the request context.
var ViewObjectKey = viewObjectKeyType{}

// validationError is a custom error type that holds a map of validation errors,
// ensuring that there is only one error per field.
type validationError struct {
	errors map[string]error
}

// Error implements the error interface, formatting all contained errors.
func (e *validationError) Error() string {
	if e == nil || len(e.errors) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("validation failed with the following errors:")
	for _, err := range e.errors {
		b.WriteString(fmt.Sprintf("- %s", err.Error()))
	}
	return b.String()
}

// Add adds a new error to the map.
func (e *validationError) Add(fieldName string, err error) {
	if err != nil {
		if e.errors == nil {
			e.errors = make(map[string]error)
		}
		e.errors[fieldName] = err
	}
}

// Err returns the validationError as a single error if it contains any errors.
func (e *validationError) Err() error {
	if e == nil || len(e.errors) == 0 {
		return nil
	}
	return e
}

// viewField is an internal, non-generic interface that allows ViewObject
// to hold a collection of fields with different underlying generic types.
type viewField interface {
	Name() string
	// validate checks the json string. It returns the value, whether it was found, and any error.
	validate(json string) (value any, found bool, err error)
}

type ViewField[T constraint.JSONType] struct {
	name       string
	required   bool
	validators []constraint.Validator[T]
}

var _ viewField = (*ViewField[string])(nil)

func (f *ViewField[T]) Name() string {
	return f.name
}

func (f *ViewField[T]) Optional() *ViewField[T] {
	f.required = false
	return f
}

// validate implements the internal viewField interface. It uses the public Validate
// method and translates its result into the format required by the ViewObject.
func (f *ViewField[T]) validate(json string) (any, bool, error) {
	rs, found := f.Validate(json)
	// If there's an error, we always propagate it, along with the found status.
	if rs.IsError() {
		return nil, found, rs.Error()
	}
	// If not found and no error, it was an optional field. Signal to skip it.
	if !found {
		return nil, false, nil
	}
	// Otherwise, it was found and is valid.
	return rs.MustGet(), true, nil
}

// Validate checks the given JSON string for the field. It returns a Result monad
// containing the typed value or an error, and a boolean indicating if the field
// was present in the JSON.
func (f *ViewField[T]) Validate(json string) (mo.Result[T], bool) {
	res := gjson.Get(json, f.name)
	if !res.Exists() {
		if f.required {
			return mo.Err[T](fmt.Errorf("%s %w", f.name, constraint.ErrRequired)), false
		}
		// For a missing optional field, return a zero value but signal it was not found.
		return mo.Ok(*new(T)), false
	}

	typedVal := typed[T](res)
	if typedVal.IsError() {
		err := fmt.Errorf("field '%s': %w", f.name, typedVal.Error())
		return mo.Err[T](err), true
	}

	val := typedVal.MustGet()
	for _, v := range f.validators {
		if err := v(val); err != nil {
			err = fmt.Errorf("field '%s': %w", f.name, err)
			return mo.Err[T](err), true
		}
	}
	return mo.Ok(val), true
}

// overflowError creates a standard error for integer overflow.
func overflowError[T any](v T) error {
	return fmt.Errorf("for type %T: %w", v, constraint.ErrIntegerOverflow)
}

// typed attempts to convert a gjson.Result into the specified JSONType.
// It returns a mo.Result[T] which contains the typed value on success,
// or an error if the type conversion fails or the JSON type does not match
// the expected Go type.
func typed[T constraint.JSONType](res gjson.Result) mo.Result[T] {
	var zero T
	targetType := reflect.TypeOf(zero)

	switch targetType.Kind() {
	case reflect.String:
		if res.Type == gjson.String {
			return mo.Ok(any(res.String()).(T))
		}
	case reflect.Bool:
		if res.Type == gjson.True || res.Type == gjson.False {
			return mo.Ok(any(res.Bool()).(T))
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if res.Type != gjson.Number {
			break // Fall through to the default error at the end.
		}
		bf, _, err := new(big.Float).Parse(res.Raw, 10)
		if err != nil {
			return mo.Err[T](fmt.Errorf("could not parse number: %w", err))
		}
		if !bf.IsInt() {
			return mo.Err[T](fmt.Errorf("%w: cannot assign float value %s to integer type", constraint.ErrTypeMismatch, res.Raw))
		}
		// Convert to big.Int to safely check bounds.
		bi, _ := bf.Int(nil)
		// Check if the big.Int value fits into a standard int64.
		if !bi.IsInt64() {
			return mo.Err[T](overflowError(zero))
		}
		val := bi.Int64()
		// Now check if the int64 value overflows the specific target type (e.g., int8, int16).
		if reflect.New(targetType).Elem().OverflowInt(val) {
			return mo.Err[T](overflowError(zero))
		}
		return mo.Ok(any(reflect.ValueOf(val).Convert(targetType).Interface()).(T))

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if res.Type != gjson.Number {
			break
		}
		bf, _, err := new(big.Float).Parse(res.Raw, 10)
		if err != nil {
			return mo.Err[T](fmt.Errorf("could not parse number: %w", err))
		}
		// Check for negative numbers, which is an overflow for unsigned types.
		if bf.Sign() < 0 {
			return mo.Err[T](overflowError(zero))
		}
		if !bf.IsInt() {
			return mo.Err[T](fmt.Errorf("%w: cannot assign float value %s to unsigned integer type", constraint.ErrTypeMismatch, res.Raw))
		}
		// Convert to big.Int to safely check bounds.
		bi, _ := bf.Int(nil)
		// Check if the big.Int value fits into a standard uint64.
		if !bi.IsUint64() {
			return mo.Err[T](overflowError(zero))
		}
		val := bi.Uint64()
		// Now check if the uint64 value overflows the specific target type (e.g., uint8, uint16).
		if reflect.New(targetType).Elem().OverflowUint(val) {
			return mo.Err[T](overflowError(zero))
		}
		return mo.Ok(any(reflect.ValueOf(val).Convert(targetType).Interface()).(T))

	case reflect.Float32, reflect.Float64:
		if res.Type != gjson.Number {
			break
		}
		val := res.Float()
		if reflect.New(targetType).Elem().OverflowFloat(val) {
			return mo.Err[T](fmt.Errorf("value %f overflows type %T", val, zero))
		}
		return mo.Ok(any(reflect.ValueOf(val).Convert(targetType).Interface()).(T))

	case reflect.Struct:
		if targetType == reflect.TypeOf(time.Time{}) {
			if res.Type == gjson.String {
				dateStr := res.String()
				layouts := []string{
					time.RFC3339Nano,
					time.RFC3339,
					"2006-01-02T15:04:05",
					"2006-01-02",
				}
				for _, layout := range layouts {
					if t, err := time.Parse(layout, dateStr); err == nil {
						return mo.Ok(any(t).(T))
					}
				}
				return mo.Err[T](fmt.Errorf("incorrect date format for string '%s'", res.String()))
			}
			break
		}
		fallthrough
	default:
		return mo.Err[T](fmt.Errorf("%w: unsupported type %T", constraint.ErrTypeMismatch, zero))
	}

	// Default error for unhandled or mismatched types.
	return mo.Err[T](fmt.Errorf("%w: expected %T but got JSON type %s", constraint.ErrTypeMismatch, zero, res.Type))
}

type FF[T constraint.JSONType] func(...constraint.ValidateFunc[T]) *ViewField[T]

func Field[T constraint.JSONType](name string, vfs ...constraint.ValidateFunc[T]) FF[T] {
	return func(fs ...constraint.ValidateFunc[T]) *ViewField[T] {
		afs := append(vfs, fs...)
		names := make(map[string]struct{})
		var nf []constraint.Validator[T]
		for _, v := range afs {
			n, f := v()
			if _, exists := names[n]; exists {
				panic(fmt.Sprintf("dvo: duplicate validator '%s' for field '%s'", n, name))
			}
			names[n] = struct{}{}
			nf = append(nf, f)
		}
		return &ViewField[T]{
			name:       name,
			validators: nf,
			required:   true,
		}
	}
}

// ViewObject is a blueprint for validating a JSON object.
type ViewObject struct {
	fields             []viewField
	allowUnknownFields bool
}

// WithFields is the constructor for a ViewObject blueprint.
func WithFields(fields ...viewField) *ViewObject {
	names := make(map[string]struct{})
	for _, f := range fields {
		if _, exists := names[f.Name()]; exists {
			panic(fmt.Sprintf("dvo: duplicate field name '%s' in ViewObject definition", f.Name()))
		}
		names[f.Name()] = struct{}{}
	}
	return &ViewObject{fields: fields, allowUnknownFields: false}
}

// AllowUnknownFields is a fluent method to make the ViewObject accept JSON
// that contains fields not defined in the schema. Default behavior is to disallow.
func (vo *ViewObject) AllowUnknownFields() *ViewObject {
	vo.allowUnknownFields = true
	return vo
}

// ValueObject is a sealed interface for a type-safe map holding validated ViewObject.
// The seal method prevents implementations outside this package.
type ValueObject interface {
	String(name string) mo.Option[string]
	Int(name string) mo.Option[int]
	Int8(name string) mo.Option[int8]
	Int16(name string) mo.Option[int16]
	Int32(name string) mo.Option[int32]
	Int64(name string) mo.Option[int64]
	Uint(name string) mo.Option[uint]
	Uint8(name string) mo.Option[uint8]
	Uint16(name string) mo.Option[uint16]
	Uint32(name string) mo.Option[uint32]
	Uint64(name string) mo.Option[uint64]
	Float64(name string) mo.Option[float64]
	Float32(name string) mo.Option[float32]
	Bool(name string) mo.Option[bool]
	Time(name string) mo.Option[time.Time]
	Set(name string, value any)
	seal()
}

// valueObject is the private, concrete implementation of the ValueObject interface.
type valueObject map[string]any

func (vo valueObject) Set(name string, value any) {
	_, ok := vo[name]
	lo.Assertf(ok, "property %s exists, can not overwrite it", name)
	vo[name] = value
}

var _ ValueObject = (*valueObject)(nil)

// seal is an empty method to satisfy the sealed ValueObject interface.
func (vo valueObject) seal() {}

// get is a generic helper to retrieve a value and assert its type.
// It returns an Option, which will be empty if the key was not present.
// It panics if the key exists but the type is incorrect.
func get[T any](d valueObject, name string) mo.Option[T] {
	value, ok := d[name]
	if !ok {
		return mo.None[T]()
	}
	typedValue, ok := value.(T)
	if !ok {
		panic(fmt.Sprintf("dvo: field '%s' has wrong type: expected %T, got %T", name, *new(T), value))
	}
	return mo.Some(typedValue)
}

// String returns an Option containing the string value for the given name.
// It panics if the field exists but is not a string.
func (vo valueObject) String(name string) mo.Option[string] {
	return get[string](vo, name)
}

// Int returns an Option containing the int value for the given name.
// It panics if the field exists but is not an int.
func (vo valueObject) Int(name string) mo.Option[int] {
	return get[int](vo, name)
}

// Int8 returns an Option containing the int8 value for the given name.
// It panics if the field exists but is not an int8.
func (vo valueObject) Int8(name string) mo.Option[int8] {
	return get[int8](vo, name)
}

// Int16 returns an Option containing the int16 value for the given name.
// It panics if the field exists but is not an int16.
func (vo valueObject) Int16(name string) mo.Option[int16] {
	return get[int16](vo, name)
}

// Int32 returns an Option containing the int32 value for the given name.
// It panics if the field exists but is not an int32.
func (vo valueObject) Int32(name string) mo.Option[int32] {
	return get[int32](vo, name)
}

// Int64 returns an Option containing the int64 value for the given name.
// It panics if the field exists but is not an int64.
func (vo valueObject) Int64(name string) mo.Option[int64] {
	return get[int64](vo, name)
}

// Uint returns an Option containing the uint value for the given name.
// It panics if the field exists but is not a uint.
func (vo valueObject) Uint(name string) mo.Option[uint] {
	return get[uint](vo, name)
}

// Uint8 returns an Option containing the uint8 value for the given name.
// It panics if the field exists but is not a uint8.
func (vo valueObject) Uint8(name string) mo.Option[uint8] {
	return get[uint8](vo, name)
}

// Uint16 returns an Option containing the uint16 value for the given name.
// It panics if the field exists but is not a uint16.
func (vo valueObject) Uint16(name string) mo.Option[uint16] {
	return get[uint16](vo, name)
}

// Uint32 returns an Option containing the uint32 value for the given name.
// It panics if the field exists but is not a uint32.
func (vo valueObject) Uint32(name string) mo.Option[uint32] {
	return get[uint32](vo, name)
}

// Uint64 returns an Option containing the uint64 value for the given name.
// It panics if the field exists but is not a uint64.
func (vo valueObject) Uint64(name string) mo.Option[uint64] {
	return get[uint64](vo, name)
}

// Float returns an Option containing the float64 value for the given name.
// It panics if the field exists but is not a float64.
func (vo valueObject) Float64(name string) mo.Option[float64] {
	return get[float64](vo, name)
}

// Float32 returns an Option containing the float32 value for the given name.
// It panics if the field exists but is not a float32.
func (vo valueObject) Float32(name string) mo.Option[float32] {
	return get[float32](vo, name)
}

// Bool returns an Option containing the bool value for the given name.
// It panics if the field exists but is not a bool.
func (vo valueObject) Bool(name string) mo.Option[bool] {
	return get[bool](vo, name)
}

// Time returns an Option containing the time.Time value for the given name.
// It panics if the field exists but is not a time.Time.
func (vo valueObject) Time(name string) mo.Option[time.Time] {
	return get[time.Time](vo, name)
}

func (vo *ViewObject) Validate(json string) mo.Result[ValueObject] {
	errs := &validationError{}
	object := valueObject{}
	// Check for unknown fields first if not allowed.
	if !vo.allowUnknownFields {
		knownFields := make(map[string]struct{}, len(vo.fields))
		for _, field := range vo.fields {
			knownFields[field.Name()] = struct{}{}
		}
		gjson.Parse(json).ForEach(func(key, value gjson.Result) bool {
			if _, exists := knownFields[key.String()]; !exists {
				errs.Add(key.String(), fmt.Errorf("unknown field '%s'", key.String()))
			}
			return true // continue iterating
		})
	}

	for _, field := range vo.fields {
		v, found, err := field.validate(json)
		if err != nil {
			errs.Add(field.Name(), err)
			// We continue even if a field is not found but required,
			// to collect all errors.
			continue
		}
		// Only add the field to the final map if it was present in the JSON.
		if found {
			object[field.Name()] = v
		}
	}
	if err := errs.Err(); err != nil {
		return mo.Err[ValueObject](err)
	}
	return mo.Ok[ValueObject](object)
}
