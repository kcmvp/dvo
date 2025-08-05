package dvo

import (
	"errors"
	"fmt"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/tidwall/gjson"
	"math"
	"math/big"
	"strings"
	"time"
)

// A private type to prevent key collisions in context.
type viewObjectKeyType struct{}

// ViewObjectKey is the key used to store the validated dataObject map in the request context.
var ViewObjectKey = viewObjectKeyType{}

type Number interface {
	int | int8 | int16 | int32 | int64 | float32 | float64
}

// JSONType is a constraint for the actual Go types we want to validate.
type JSONType interface {
	Number | string | time.Time | bool
}

// Validator is a pure, type-safe function that receives an already-typed value.
// It has a name to prevent duplicate validators on a single field.
type Validator[T JSONType] interface {
	// Name returns a unique machine-readable name for the validator, e.g., "min".
	Name() string
	// Validate performs the validation and returns an error if it fails.
	Validate(val T) error
}

// validator is an internal implementation of the Validator interface.
type validator[T JSONType] struct {
	name string
	fn   func(val T) error
}

// Name implements the Validator interface.
func (v validator[T]) Name() string {
	return v.name
}

// Validate implements the Validator interface.
func (v validator[T]) Validate(val T) error {
	return v.fn(val)
}

// NewValidator is a helper for validator implementations to create a Validator.
// It is not intended for direct use by the end-user, but must be exported
// for use by the validator package.
func NewValidator[T JSONType](name string, fn func(val T) error) Validator[T] {
	return validator[T]{name: name, fn: fn}
}

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

type ViewField[T JSONType] struct {
	name       string
	required   bool
	validators []Validator[T]
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
			return mo.Err[T](fmt.Errorf("%s %w", f.name, ErrRequired)), false
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
		if err := v.Validate(val); err != nil {
			err = fmt.Errorf("field '%s': %w", f.name, err)
			return mo.Err[T](err), true
		}
	}
	return mo.Ok(val), true
}

var ErrIntegerOverflow = errors.New("integer overflow")
var ErrTypeMismatch = errors.New("type mismatch")
var ErrRequired = errors.New("is required but not found")

// checkBounds is a helper to check if a big.Float is within the given int64 min/max boundaries.
func checkBounds(bf *big.Float, min, max int64) bool {
	maxV := new(big.Float).SetInt64(max)
	minV := new(big.Float).SetInt64(min)
	return bf.Cmp(maxV) > 0 || bf.Cmp(minV) < 0
}

// typed attempts to convert a gjson.Result into the specified JSONType.
// It returns a mo.Result[T] which contains the typed value on success,
// or an error if the type conversion fails or the JSON type does not match
// the expected Go type.
func typed[T JSONType](res gjson.Result) mo.Result[T] {
	var zero T
	switch tt := any(zero).(type) {
	case string:
		if res.Type == gjson.String {
			return mo.Ok(any(res.String()).(T))
		}
	case bool:
		if res.Type == gjson.True || res.Type == gjson.False {
			return mo.Ok(any(res.Bool()).(T))
		}
	case int, int8, int16, int32, int64:
		if res.Type != gjson.Number {
			return mo.Err[T](fmt.Errorf("%w: expected %T but got JSON type %s", ErrTypeMismatch, tt, res.Type))
		}

		// Use big.Float for arbitrary-precision parsing to avoid float64 precision loss.
		bf, _, err := new(big.Float).Parse(res.Raw, 10)
		if err != nil {
			return mo.Err[T](fmt.Errorf("could not parse number: %w", err))
		}

		// Check if the number is a whole number.
		if !bf.IsInt() {
			return mo.Err[T](fmt.Errorf("%w: cannot assign float value %s to integer type", ErrTypeMismatch, res.Raw))
		}

		// Check for overflow against the specific integer type and convert.
		val, _ := bf.Int64()
		switch any(zero).(type) {
		case int:
			if checkBounds(bf, math.MinInt, math.MaxInt) {
				return mo.Err[T](fmt.Errorf("for type %T: %w", tt, ErrIntegerOverflow))
			}
			return mo.Ok(any(int(val)).(T))
		case int8:
			if checkBounds(bf, math.MinInt8, math.MaxInt8) {
				return mo.Err[T](fmt.Errorf("for type %T: %w", tt, ErrIntegerOverflow))
			}
			return mo.Ok(any(int8(val)).(T))
		case int16:
			if checkBounds(bf, math.MinInt16, math.MaxInt16) {
				return mo.Err[T](fmt.Errorf("for type %T: %w", tt, ErrIntegerOverflow))
			}
			return mo.Ok(any(int16(val)).(T))
		case int32:
			if checkBounds(bf, math.MinInt32, math.MaxInt32) {
				return mo.Err[T](fmt.Errorf("for type %T: %w", tt, ErrIntegerOverflow))
			}
			return mo.Ok(any(int32(val)).(T))
		case int64:
			if checkBounds(bf, math.MinInt64, math.MaxInt64) {
				return mo.Err[T](fmt.Errorf("for type %T: %w", tt, ErrIntegerOverflow))
			}
			return mo.Ok(any(val).(T))
		}

	case float32, float64:
		if res.Type != gjson.Number {
			return mo.Err[T](fmt.Errorf("%w: expected number but got JSON type %s", ErrTypeMismatch, res.Type))
		}
		val := res.Float() // This is float64
		switch any(zero).(type) {
		case float32:
			if val > math.MaxFloat32 || val < -math.MaxFloat32 {
				return mo.Err[T](fmt.Errorf("value %f overflows type float32", val))
			}
			return mo.Ok(any(float32(val)).(T))
		case float64:
			return mo.Ok(any(val).(T))
		}
	case time.Time:
		if res.Type == gjson.String {
			dateStr := res.String()
			// List of layouts to try, from most to least specific.
			layouts := []string{
				time.RFC3339Nano,
				time.RFC3339,
				"2006-01-02T15:04:05", // Local time without timezone
				"2006-01-02",          // Date only
			}
			for _, layout := range layouts {
				if t, err := time.Parse(layout, dateStr); err == nil {
					return mo.Ok(any(t).(T))
				}
			}
			// If no layout matched, return a specific error.
			return mo.Err[T](fmt.Errorf("incorrect date format for string '%s'", res.String()))
		}
	}
	return mo.Err[T](fmt.Errorf("%w: expected %T but got JSON type %s", ErrTypeMismatch, zero, res.Type))
}

type FieldFunc[T JSONType] func(...Validator[T]) *ViewField[T]

func Field[T JSONType](name string, validators ...Validator[T]) FieldFunc[T] {
	return func(additional ...Validator[T]) *ViewField[T] {
		// Both slices are now of the same type ([]Validator[T]), so we can append directly.
		allValidators := append(validators, additional...)

		names := make(map[string]struct{})
		for _, v := range allValidators {
			if _, exists := names[v.Name()]; exists {
				panic(fmt.Sprintf("dvo: duplicate validator '%s' for field '%s'", v.Name(), name))
			}
			names[v.Name()] = struct{}{}
		}
		// Now we use the combined slice to create the final field.
		return &ViewField[T]{
			name:       name,
			validators: allValidators,
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

// DataObject is a sealed interface for a type-safe map holding validated dataObject.
// The seal method prevents implementations outside this package.
type DataObject interface {
	String(name string) mo.Option[string]
	Int(name string) mo.Option[int]
	Int64(name string) mo.Option[int64]
	Float(name string) mo.Option[float64]
	Float32(name string) mo.Option[float32]
	Bool(name string) mo.Option[bool]
	Time(name string) mo.Option[time.Time]
	Set(name string, value any)
	seal()
}

// dataObject is the private, concrete implementation of the DataObject interface.
type dataObject map[string]any

func (do dataObject) Set(name string, value any) {
	_, ok := do[name]
	lo.Assertf(ok, "property %s exists, can not overwrite it", name)
	do[name] = value
}

var _ DataObject = (*dataObject)(nil)

// seal is an empty method to satisfy the sealed DataObject interface.
func (do dataObject) seal() {}

// get is a generic helper to retrieve a value and assert its type.
// It returns an Option, which will be empty if the key was not present.
// It panics if the key exists but the type is incorrect.
func get[T any](d dataObject, name string) mo.Option[T] {
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
func (do dataObject) String(name string) mo.Option[string] {
	return get[string](do, name)
}

// Int returns an Option containing the int value for the given name.
// It panics if the field exists but is not an int.
func (do dataObject) Int(name string) mo.Option[int] {
	return get[int](do, name)
}

// Int64 returns an Option containing the int64 value for the given name.
// It panics if the field exists but is not an int64.
func (do dataObject) Int64(name string) mo.Option[int64] {
	return get[int64](do, name)
}

// Float returns an Option containing the float64 value for the given name.
// It panics if the field exists but is not a float64.
func (do dataObject) Float(name string) mo.Option[float64] {
	return get[float64](do, name)
}

// Float32 returns an Option containing the float32 value for the given name.
// It panics if the field exists but is not a float32.
func (do dataObject) Float32(name string) mo.Option[float32] {
	return get[float32](do, name)
}

// Bool returns an Option containing the bool value for the given name.
// It panics if the field exists but is not a bool.
func (do dataObject) Bool(name string) mo.Option[bool] {
	return get[bool](do, name)
}

// Time returns an Option containing the time.Time value for the given name.
// It panics if the field exists but is not a time.Time.
func (do dataObject) Time(name string) mo.Option[time.Time] {
	return get[time.Time](do, name)
}

func (vo *ViewObject) Validate(json string) mo.Result[DataObject] {
	errs := &validationError{}
	object := dataObject{}
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
		return mo.Err[DataObject](err)
	}
	return mo.Ok[DataObject](object)
}
