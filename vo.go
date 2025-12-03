package dvo

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kcmvp/dvo/constraint"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"github.com/tidwall/gjson"
)

// timeLayouts defines the supported time formats for parsing time.Time fields.
var timeLayouts = []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"}

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
	// Sort keys for deterministic error messages, which is good for testing.
	keys := make([]string, 0, len(e.errors))
	for k := range e.errors {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("validation failed with the following errors:")
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("- %s: %s", k, e.errors[k]))
	}
	return b.String()
}

// add adds a new error to the map.
func (e *validationError) add(fieldName string, err error) {
	if err != nil {
		if e.errors == nil {
			e.errors = make(map[string]error)
		}
		e.errors[fieldName] = err
	}
}

// err returns the validationError as a single error if it contains any errors.
func (e *validationError) err() error {
	if e == nil || len(e.errors) == 0 {
		return nil
	}
	return e
}

// ViewField is an internal, non-generic interface that allows ViewObject
// to hold a collection of fields with different underlying generic types.
type ViewField interface {
	Name() string
	IsArray() bool
	IsObject() bool
	Required() bool
	validate(node gjson.Result) mo.Result[any]
	validateRaw(v string) mo.Result[any]
	embeddedObject() mo.Option[*ViewObject]
}

type JSONField[T constraint.FieldType] struct {
	name       string
	required   bool
	array      bool
	object     bool
	embedded   *ViewObject
	validators []constraint.Validator[T]
}

func (f *JSONField[T]) toViewField() ViewField {
	return f
}

func (f *JSONField[T]) Required() bool {
	return f.required
}

func (f *JSONField[T]) IsArray() bool {
	return f.array
}

func (f *JSONField[T]) IsObject() bool {
	return f.object
}

func (f *JSONField[T]) embeddedObject() mo.Option[*ViewObject] {
	return lo.Ternary(f.embedded == nil, mo.None[*ViewObject](), mo.Some(f.embedded))
}

var _ ViewField = (*JSONField[string])(nil)
var _ FieldProvider = (*JSONField[string])(nil)

func (f *JSONField[T]) Name() string {
	return f.name
}

func (f *JSONField[T]) Optional() *JSONField[T] {
	f.required = false
	return f
}

func (f *JSONField[T]) validateRaw(v string) mo.Result[any] {
	// typedString[T] returns mo.Result[T]
	// validateRaw needs to return mo.Result[any]
	typedValResult := typedString[T](v)
	if typedValResult.IsError() {
		// Wrap the error to provide more context about the field.
		err := fmt.Errorf("field '%s': %w", f.Name(), typedValResult.Error())
		return mo.Err[any](err)
	}

	val := typedValResult.MustGet()
	// Run validators on the successfully parsed value.
	for _, validator := range f.validators {
		if err := validator(val); err != nil {
			err = fmt.Errorf("field '%s': %w", f.Name(), err)
			return mo.Err[any](err)
		}
	}

	return mo.Ok[any](val)
}

// Validate checks the given raw string for the field. It returns a Result monad
// containing the typedJson value or an error
func (f *JSONField[T]) validate(node gjson.Result) mo.Result[any] {
	// Case: Nested Single Object
	if f.IsObject() && !f.IsArray() {
		// Recursively validate. The result will be a mo.Result[ValueObject].
		nestedResult := f.embeddedObject().MustGet().Validate(node.Raw)
		if nestedResult.IsError() {
			// Wrap the error to provide context.
			return mo.Err[any](fmt.Errorf("field '%s' validation failed, %w", f.Name(), nestedResult.Error()))
		}
		// Return the embedded ValueObject itself.
		return mo.Ok[any](nestedResult.MustGet())
	}

	// Case: Array
	if f.IsArray() {
		if !node.IsArray() {
			return mo.Err[any](fmt.Errorf("dvo: field '%s' expected a JSON array but got %s", f.Name(), node.Type))
		}
		errs := &validationError{}
		// Subcase: Array of Objects
		if f.embeddedObject().IsPresent() {
			var values []ValueObject
			node.ForEach(func(index, element gjson.Result) bool {
				if !element.IsObject() {
					errs.add(fmt.Sprintf("%s[%d]", f.Name(), index.Int()), fmt.Errorf("expected a JSON object but got %s", element.Type))
					return true // continue
				}
				result := f.embedded.Validate(element.Raw)
				if result.IsError() {
					// To avoid embedded error messages, if the embedded validation returns a
					// validationError with a single underlying error, we extract it.
					// This makes the final error message cleaner.
					errToAdd := result.Error()
					var nested *validationError
					if errors.As(errToAdd, &nested) && len(nested.errors) == 1 {
						for _, v := range nested.errors {
							errToAdd = v
						}
					}
					errs.add(fmt.Sprintf("%s[%d]", f.Name(), index.Int()), errToAdd)
				} else if errs.err() == nil {
					values = append(values, result.MustGet())
				}
				return true // continue
			})
			return lo.Ternary(errs.err() != nil, mo.Err[any](errs.err()), mo.Ok[any](values))
		}

		// Subcase: Array of Primitives
		var values []T
		node.ForEach(func(index, element gjson.Result) bool {
			// We need to validate each element of the array.
			typedVal := typedJson[T](element)
			if typedVal.IsError() {
				errs.add(fmt.Sprintf("%s[%d]", f.Name(), index.Int()), typedVal.Error())
				return true // continue to collect all errors
			}

			val := typedVal.MustGet()
			// Run validators on each element
			for _, v := range f.validators {
				if err := v(val); err != nil {
					errs.add(fmt.Sprintf("%s[%d]", f.Name(), index.Int()), err)
				}
			}

			// Only append if there were no errors for this specific element
			if errs.err() == nil {
				values = append(values, val)
			}
			return true
		})
		return lo.Ternary(errs.err() != nil, mo.Err[any](errs.err()), mo.Ok[any](values))
	}
	// --- Fallback for simple, non-array, non-object fields ---
	typedVal := typedJson[T](node)
	if typedVal.IsError() {
		err := fmt.Errorf("field '%s': %w", f.Name(), typedVal.Error())
		return mo.Err[any](err)
	}
	val := typedVal.MustGet()
	for _, v := range f.validators {
		if err := v(val); err != nil {
			err = fmt.Errorf("field '%s': %w", f.Name(), err)
			return mo.Err[any](err)
		}
	}
	return mo.Ok[any](val)
}

// overflowError creates a standard error for integer overflow.
func overflowError[T any](v T) error {
	return fmt.Errorf("for type %T: %w", v, constraint.ErrIntegerOverflow)
}

// typedJson attempts to convert a gjson.Result into the specified FieldType.
// It returns a mo.Result[T] which contains the typedJson value on success,
// or an error if the type conversion fails or the raw type does not match
// the expected Go type.
func typedJson[T constraint.FieldType](res gjson.Result) mo.Result[T] {
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
		// To detect overflow and prevent floats, we get the int value, format it back
		// to a string, and compare it with the raw input. If they differ, it means
		// gjson saturated the value (overflow) or truncated a float.
		val := res.Int()
		if strconv.FormatInt(val, 10) != res.Raw {
			if strings.Contains(res.Raw, ".") {
				return mo.Err[T](fmt.Errorf("%w: cannot assign float value %s to integer type", constraint.ErrTypeMismatch, res.Raw))
			}
			return mo.Err[T](overflowError(zero))
		}
		// Now check if the int64 value overflows the specific target type (e.g., int8, int16).
		if reflect.New(targetType).Elem().OverflowInt(val) {
			return mo.Err[T](overflowError(zero))
		}
		return mo.Ok(reflect.ValueOf(val).Convert(targetType).Interface().(T))

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if res.Type != gjson.Number {
			break
		}
		// Check for negative numbers, which is an overflow for unsigned types.
		if strings.Contains(res.Raw, "-") {
			return mo.Err[T](overflowError(zero))
		}
		// Similar to the signed int case, we compare string representations to
		// detect saturation on overflow or truncation of floats.
		val := res.Uint()
		if strconv.FormatUint(val, 10) != res.Raw {
			if strings.Contains(res.Raw, ".") {
				return mo.Err[T](fmt.Errorf("%w: cannot assign float value %s to unsigned integer type", constraint.ErrTypeMismatch, res.Raw))
			}
			return mo.Err[T](overflowError(zero))
		}
		// Now check if the uint64 value overflows the specific target type (e.g., uint8, uint16).
		if reflect.New(targetType).Elem().OverflowUint(val) {
			return mo.Err[T](overflowError(zero))
		}
		return mo.Ok(reflect.ValueOf(val).Convert(targetType).Interface().(T))

	case reflect.Float32, reflect.Float64:
		var val float64
		var err error
		if res.Type == gjson.Number {
			val = res.Float()
		} else if res.Type == gjson.String {
			// Explicitly parse string to float, capturing any errors.
			val, err = strconv.ParseFloat(res.String(), 64)
			if err != nil {
				return mo.Err[T](fmt.Errorf("could not parse string '%s' as float: %w", res.String(), err))
			}
		} else {
			// For any other type, fall through to the default type mismatch error.
			break
		}
		if reflect.New(targetType).Elem().OverflowFloat(val) {
			return mo.Err[T](fmt.Errorf("value %f overflows type %T", val, zero))
		}
		return mo.Ok(reflect.ValueOf(val).Convert(targetType).Interface().(T))

	case reflect.Struct:
		if targetType == reflect.TypeOf(time.Time{}) {
			if res.Type == gjson.String {
				dateStr := res.String()
				for _, layout := range timeLayouts {
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
	return mo.Err[T](fmt.Errorf("%w: expected %T but got raw type %s", constraint.ErrTypeMismatch, zero, res.Type))
}

// typedString attempts to convert a string into the specified FieldType.
// It returns a mo.Result[T] which contains the typed value on success,
// or an error if the type conversion fails or the string cannot be parsed
// into the expected Go type.
func typedString[T constraint.FieldType](s string) mo.Result[T] {
	var zero T
	targetType := reflect.TypeOf(zero)

	switch targetType.Kind() {
	case reflect.String:
		return mo.Ok(any(s).(T))
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return mo.Err[T](fmt.Errorf("could not parse '%s' as bool: %w", s, err))
		}
		return mo.Ok(any(b).(T))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return mo.Err[T](fmt.Errorf("could not parse '%s' as int: %w", s, err))
		}
		if reflect.New(targetType).Elem().OverflowInt(val) {
			return mo.Err[T](overflowError(zero))
		}
		return mo.Ok(reflect.ValueOf(val).Convert(targetType).Interface().(T))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return mo.Err[T](fmt.Errorf("could not parse '%s' as uint: %w", s, err))
		}
		if reflect.New(targetType).Elem().OverflowUint(val) {
			return mo.Err[T](overflowError(zero))
		}
		return mo.Ok(reflect.ValueOf(val).Convert(targetType).Interface().(T))
	case reflect.Float32, reflect.Float64:
		val, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return mo.Err[T](fmt.Errorf("could not parse '%s' as float: %w", s, err))
		}
		if reflect.New(targetType).Elem().OverflowFloat(val) {
			return mo.Err[T](fmt.Errorf("value %f overflows type %T", val, zero))
		}
		return mo.Ok(reflect.ValueOf(val).Convert(targetType).Interface().(T))
	case reflect.Struct:
		if targetType == reflect.TypeOf(time.Time{}) {
			for _, layout := range timeLayouts {
				if t, err := time.Parse(layout, s); err == nil {
					return mo.Ok(any(t).(T))
				}
			}
			return mo.Err[T](fmt.Errorf("incorrect date format for string '%s'", s))
		}
		fallthrough
	default:
		return mo.Err[T](fmt.Errorf("%w: unsupported type %T for URL parameter", constraint.ErrTypeMismatch, zero))
	}
}

// FieldProvider is an interface for any type that can provide a ViewField.
// It's used to create a type-safe and unified API for WithFields.
type FieldProvider interface {
	toViewField() ViewField
}

// ObjectField creates a slice of ViewField for a embeddedObject object.
// It takes the name of the object field and a ViewObject representing its schema.
// Each field in the embeddedObject ViewObject will be prefixed with the object's name.
// The name of the object field should not contain '#' and `.`.
func ObjectField(name string, nested *ViewObject) *JSONField[string] {
	lo.Assertf(nested != nil, "Nested ViewObject is null for ObjectField %s", name)
	return trait[string](name, false, true, nested)
}

// ArrayOfObjectField creates a slice of ViewField for an array of embeddedObject objects.
// It takes the name of the array field and a ViewObject representing the schema of its elements.
// The name of the array field should not contain '#' and `.`.
func ArrayOfObjectField(name string, nested *ViewObject) *JSONField[string] {
	lo.Assertf(nested != nil, "Nested ViewObject is null for ArrayOfObjectField %s", name)
	return trait[string](name, true, true, nested)
}

// ArrayField creates a FieldFunc for an array field.
// It is intended to be used for array fields that contain primitive types.
// The name of the array field should not contain '#' and `.`.
func ArrayField[T constraint.FieldType](name string, vfs ...constraint.ValidateFunc[T]) *JSONField[T] {
	return trait[T](name, true, false, nil, vfs...)
}

// Field creates a FieldFunc for a single field.
// It takes the name of the field and an optional list of validators.
// The returned FieldFunc can then be used to create a JSONField,
// allowing for additional validators to be chained.
// The name of the field should not contain '#' and `.`.
func Field[T constraint.FieldType](name string, vfs ...constraint.ValidateFunc[T]) *JSONField[T] {
	return trait[T](name, false, false, nil, vfs...)
}

func trait[T constraint.FieldType](name string, isArray, isObject bool, nested *ViewObject, vfs ...constraint.ValidateFunc[T]) *JSONField[T] {
	if strings.ContainsAny(name, ".#") {
		panic(fmt.Sprintf("dvo: field name '%s' cannot contain '.' or '#'", name))
	}
	names := make(map[string]struct{})
	var nf []constraint.Validator[T]
	for _, v := range vfs {
		n, f := v()
		if _, exists := names[n]; exists {
			panic(fmt.Sprintf("dvo: duplicate validator '%s' for field '%s'", n, name))
		}
		names[n] = struct{}{}
		nf = append(nf, f)
	}
	return &JSONField[T]{
		name:       name,
		array:      isArray,
		object:     isObject,
		embedded:   nested,
		validators: nf,
		required:   true,
	}
}

// ViewObject is a blueprint for validating a raw object.
type ViewObject struct {
	fields             []ViewField
	allowUnknownFields bool
}

// WithFields is the new constructor for a ViewObject blueprint that accepts FieldProvider.
// This allows for a more fluent and type-safe API.
func WithFields(providers ...FieldProvider) *ViewObject {
	fields := make([]ViewField, len(providers))
	for i, p := range providers {
		fields[i] = p.toViewField()
	}
	names := make(map[string]struct{})
	for _, f := range fields {
		if _, exists := names[f.Name()]; exists {
			panic(fmt.Sprintf("dvo: duplicate field name '%s' in ViewObject definition", f.Name()))
		}
		names[f.Name()] = struct{}{}
	}
	return &ViewObject{fields: fields, allowUnknownFields: false}
}

// AllowUnknownFields is a fluent method to make the ViewObject accept raw
// that contains fields not defined in the schema. Default behavior is to disallow.
func (vo *ViewObject) AllowUnknownFields() *ViewObject {
	vo.allowUnknownFields = true
	return vo
}

// ValueObject is a sealed interface for a type-safe map holding validated ViewObject.
// The seal method prevents implementations outside this package.
//
// All getter methods (String, Int, Get, etc.) support dot notation for hierarchical
// access to embedded objects and arrays.
//
// For example, given a ValueObject `vo` representing the JSON:
//
//	{
//	  "user": { "email": "test@example.com" },
//	  "items": [ { "id": 101 } ]
//	}
//
// You can access embedded values like this:
//
//	email := vo.MstString("user.email") // "test@example.com"
//	itemID := vo.MstInt("items.0.id")   // 101
//
// If a path is invalid (e.g., key not found), the `Option`
// based getters (like `String`) will return `mo.None`, while the `Mst` prefixed
// getters (like `MstString`) will panic.
//
// If a path is malformed (e.g., non-integer index for an array, out-of-bounds index)
// or a type mismatch occurs, all getters will panic.
type ValueObject interface {
	// String returns an Option containing the string value for the given name.
	// It supports dot notation for hierarchical access (e.g., "user.name").
	// It panics if the field exists but is not a string.
	String(name string) mo.Option[string]
	// MstString returns the string value for the given name.
	// It supports dot notation for hierarchical access (e.g., "user.name").
	// It panics if the key is not found or the value is not a string.
	MstString(name string) string
	// Int returns an Option containing the int value for the given name.
	// It supports dot notation for hierarchical access (e.g., "user.age").
	// It panics if the field exists but is not an int.
	Int(name string) mo.Option[int]
	// MstInt returns the int value for the given name.
	// It supports dot notation for hierarchical access (e.g., "user.age").
	// It panics if the key is not found or the value is not an int.
	MstInt(name string) int
	// Int8 returns an Option containing the int8 value for the given name.
	// It panics if the field exists but is not an int8.
	Int8(name string) mo.Option[int8]
	// MstInt8 returns the int8 value for the given name.
	// It panics if the key is not found or the value is not an int8.
	MstInt8(name string) int8
	// Int16 returns an Option containing the int16 value for the given name.
	// It panics if the field exists but is not an int16.
	Int16(name string) mo.Option[int16]
	// MstInt16 returns the int16 value for the given name.
	// It panics if the key is not found or the value is not an int16.
	MstInt16(name string) int16
	// Int32 returns an Option containing the int32 value for the given name.
	// It panics if the field exists but is not an int32.
	Int32(name string) mo.Option[int32]
	// MstInt32 returns the int32 value for the given name.
	// It panics if the key is not found or the value is not an int32.
	MstInt32(name string) int32
	// Int64 returns an Option containing the int64 value for the given name.
	// It panics if the field exists but is not an int64.
	Int64(name string) mo.Option[int64]
	// MstInt64 returns the int64 value for the given name.
	// It panics if the key is not found or the value is not an int64.
	MstInt64(name string) int64
	// Uint returns an Option containing the uint value for the given name.
	// It panics if the field exists but is not a uint.
	Uint(name string) mo.Option[uint]
	// MstUint returns the uint value for the given name.
	// It panics if the key is not found or the value is not a uint.
	MstUint(name string) uint
	// Uint8 returns an Option containing the uint8 value for the given name.
	// It panics if the field exists but is not a uint8.
	Uint8(name string) mo.Option[uint8]
	// MstUint8 returns the uint8 value for the given name.
	// It panics if the key is not found or the value is not a uint8.
	MstUint8(name string) uint8
	// Uint16 returns an Option containing the uint16 value for the given name.
	// It panics if the field exists but is not a uint16.
	Uint16(name string) mo.Option[uint16]
	// MstUint16 returns the uint16 value for the given name.
	// It panics if the key is not found or the value is not a uint16.
	MstUint16(name string) uint16
	// Uint32 returns an Option containing the uint32 value for the given name.
	// It panics if the field exists but is not a uint32.
	Uint32(name string) mo.Option[uint32]
	// MstUint32 returns the uint32 value for the given name.
	// It panics if the key is not found or the value is not a uint32.
	MstUint32(name string) uint32
	// Uint64 returns an Option containing the uint64 value for the given name.
	// It panics if the field exists but is not a uint64.
	Uint64(name string) mo.Option[uint64]
	// MstUint64 returns the uint64 value for the given name.
	// It panics if the key is not found or the value is not a uint64.
	MstUint64(name string) uint64
	// Float64 returns an Option containing the float64 value for the given name.
	// It panics if the field exists but is not a float64.
	Float64(name string) mo.Option[float64]
	// MstFloat64 returns the float64 value for the given name.
	// It panics if the key is not found or the value is not a float64.
	MstFloat64(name string) float64
	// Float32 returns an Option containing the float32 value for the given name.
	// It panics if the field exists but is not a float32.
	Float32(name string) mo.Option[float32]
	// MstFloat32 returns the float32 value for the given name.
	// It panics if the key is not found or the value is not a float32.
	MstFloat32(name string) float32
	// Bool returns an Option containing the bool value for the given name.
	// It panics if the field exists but is not a bool.
	Bool(name string) mo.Option[bool]
	// MstBool returns the bool value for the given name.
	// It panics if the key is not found or the value is not a bool.
	MstBool(name string) bool
	// Time returns an Option containing the time.Time value for the given name.
	// It panics if the field exists but is not a time.Time.
	Time(name string) mo.Option[time.Time]
	// MstTime returns the time.Time value for the given name.
	// It panics if the key is not found or the value is not a time.Time.
	MstTime(name string) time.Time
	// StringArray returns an Option containing a slice of strings for the given name.
	// It panics if the field exists but is not a []string.
	StringArray(name string) mo.Option[[]string]
	// MstStringArray returns a slice of strings for the given name.
	// It panics if the key is not found or the value is not a []string.
	MstStringArray(name string) []string
	// IntArray returns an Option containing a slice of ints for the given name.
	// It panics if the field exists but is not a []int.
	IntArray(name string) mo.Option[[]int]
	// MstIntArray returns a slice of ints for the given name.
	// It panics if the key is not found or the value is not a []int.
	MstIntArray(name string) []int
	// Int64Array returns an Option containing a slice of int64s for the given name.
	// It panics if the field exists but is not a []int64.
	Int64Array(name string) mo.Option[[]int64]
	// MstInt64Array returns a slice of int64s for the given name.
	// It panics if the key is not found or the value is not a []int64.
	MstInt64Array(name string) []int64
	// Float64Array returns an Option containing a slice of float64s for the given name.
	// It panics if the field exists but is not a []float64.
	Float64Array(name string) mo.Option[[]float64]
	// MstFloat64Array returns a slice of float64s for the given name.
	// It panics if the key is not found or the value is not a []float64.
	MstFloat64Array(name string) []float64
	// BoolArray returns an Option containing a slice of bools for the given name.
	// It panics if the field exists but is not a []bool.
	BoolArray(name string) mo.Option[[]bool]
	// MstBoolArray returns a slice of bools for the given name.
	// It panics if the key is not found or the value is not a []bool.
	MstBoolArray(name string) []bool
	// Get retrieves a value of any type from the ValueObject.
	// It supports dot notation for hierarchical access (e.g., "user.name", "items.0.id").
	// It returns an Option, which will be `None` if the path is not found.
	Get(string) mo.Option[any]
	// Add adds a new property to the value object at the top level.
	// It does not support dot notation.
	// It panics if the property already exists.
	Add(name string, value any)
	// Update modifies an existing property in the value object at the top level.
	// It does not support dot notation.
	// It panics if the property does not exist.
	Update(name string, value any)
	seal()
}

// valueObject is the private, concrete implementation of the ValueObject interface.
type valueObject map[string]any

func (vo valueObject) Get(s string) mo.Option[any] {
	return get[any](vo, s)
}

// Add adds a new property to the value object.
// It panics if the property already exists.
func (vo valueObject) Add(name string, value any) {
	_, ok := vo[name]
	lo.Assertf(!ok, "dvo: property '%s' already exists", name)
	lo.Assertf(!strings.Contains(name, "."), "dov: property '%s' contains '.'", name)
	vo[name] = value
}

// Update modifies an existing property in the value object.
// It panics if the property does not exist.
func (vo valueObject) Update(name string, value any) {
	if _, ok := vo[name]; !ok {
		panic(fmt.Sprintf("dvo: property '%s' does not exist", name))
	}
	vo[name] = value
}

var _ ValueObject = (*valueObject)(nil)

// seal is an empty method to satisfy the sealed ValueObject interface.
func (vo valueObject) seal() {}

// get is a generic helper to retrieve a value and assert its type.
// It returns an Option, which will be empty if the key was not present.
// It panics if the key exists but the type is incorrect.
// If the path contains an invalid index for a slice, it will panic. This function
// supports dot notation for embedded objects and array indexing (e.g., "field.0.nestedField").
func get[T any](data valueObject, name string) mo.Option[T] {
	parts := strings.Split(name, ".")
	var currentValue any = data
	for _, part := range parts {
		if currentValue == nil {
			return mo.None[T]()
		}
		// If it's a map, look up the key.
		if vo, ok := currentValue.(valueObject); ok {
			nextValue, exists := vo[part]
			if !exists {
				return mo.None[T]()
			}
			currentValue = nextValue
			continue
		}
		// If it's a slice, look up the index.
		val := reflect.ValueOf(currentValue)
		if val.Kind() == reflect.Slice {
			index, err := strconv.Atoi(part)
			lo.Assertf(err == nil, "dvo: path part '%s' in '%s' is not a valid integer index for a slice", part, name)
			lo.Assertf(index >= 0 && index < val.Len(), "dvo: array bound exceed: %v", val)
			currentValue = val.Index(index).Interface()
			continue
		}
		// If we are here, we are trying to traverse into a primitive from a non-final path segment.
		return mo.None[T]()
	}

	typedValue, ok := currentValue.(T)
	lo.Assertf(ok, "dvo: field '%s' has wrong type: expected %T, got %T", name, *new(T), currentValue)
	return mo.Some(typedValue)
}

func (vo valueObject) StringArray(name string) mo.Option[[]string] {
	return get[[]string](vo, name)
}

func (vo valueObject) MstStringArray(name string) []string {
	return vo.StringArray(name).MustGet()
}

func (vo valueObject) IntArray(name string) mo.Option[[]int] {
	return get[[]int](vo, name)
}

func (vo valueObject) MstIntArray(name string) []int {
	return vo.IntArray(name).MustGet()
}

func (vo valueObject) Int64Array(name string) mo.Option[[]int64] {
	return get[[]int64](vo, name)
}

func (vo valueObject) MstInt64Array(name string) []int64 {
	return vo.Int64Array(name).MustGet()
}

func (vo valueObject) Float64Array(name string) mo.Option[[]float64] {
	return get[[]float64](vo, name)
}

func (vo valueObject) MstFloat64Array(name string) []float64 {
	return vo.Float64Array(name).MustGet()
}

func (vo valueObject) BoolArray(name string) mo.Option[[]bool] {
	return get[[]bool](vo, name)
}

func (vo valueObject) MstBoolArray(name string) []bool {
	return vo.BoolArray(name).MustGet()
}

// String returns an Option containing the string value for the given name.
// It panics if the field exists but is not a string.
func (vo valueObject) String(name string) mo.Option[string] {
	return get[string](vo, name)
}

// MstString returns the string value for the given name.
// It panics if the key is not found or the value is not a string.
func (vo valueObject) MstString(name string) string {
	return vo.String(name).MustGet()
}

// Int returns an Option containing the int value for the given name.
// It panics if the field exists but is not an int.
func (vo valueObject) Int(name string) mo.Option[int] {
	return get[int](vo, name)
}

// MstInt returns the int value for the given name.
// It panics if the key is not found or the value is not an int.
func (vo valueObject) MstInt(name string) int {
	return vo.Int(name).MustGet()
}

// Int8 returns an Option containing the int8 value for the given name.
// It panics if the field exists but is not an int8.
func (vo valueObject) Int8(name string) mo.Option[int8] {
	return get[int8](vo, name)
}

// MstInt8 returns the int8 value for the given name.
// It panics if the key is not found or the value is not an int8.
func (vo valueObject) MstInt8(name string) int8 {
	return vo.Int8(name).MustGet()
}

// Int16 returns an Option containing the int16 value for the given name.
// It panics if the field exists but is not an int16.
func (vo valueObject) Int16(name string) mo.Option[int16] {
	return get[int16](vo, name)
}

// MstInt16 returns the int16 value for the given name.
// It panics if the key is not found or the value is not an int16.
func (vo valueObject) MstInt16(name string) int16 {
	return vo.Int16(name).MustGet()
}

// Int32 returns an Option containing the int32 value for the given name.
// It panics if the field exists but is not an int32.
func (vo valueObject) Int32(name string) mo.Option[int32] {
	return get[int32](vo, name)
}

// MstInt32 returns the int32 value for the given name.
// It panics if the key is not found or the value is not an int32.
func (vo valueObject) MstInt32(name string) int32 {
	return vo.Int32(name).MustGet()
}

// Int64 returns an Option containing the int64 value for the given name.
// It panics if the field exists but is not an int64.
func (vo valueObject) Int64(name string) mo.Option[int64] {
	return get[int64](vo, name)
}

// MstInt64 returns the int64 value for the given name.
// It panics if the key is not found or the value is not an int64.
func (vo valueObject) MstInt64(name string) int64 {
	return vo.Int64(name).MustGet()
}

// Uint returns an Option containing the uint value for the given name.
// It panics if the field exists but is not a unit.
func (vo valueObject) Uint(name string) mo.Option[uint] {
	return get[uint](vo, name)
}

// MstUint returns the uint value for the given name.
// It panics if the key is not found or the value is not a unit.
func (vo valueObject) MstUint(name string) uint {
	return vo.Uint(name).MustGet()
}

// Uint8 returns an Option containing the uint8 value for the given name.
// It panics if the field exists but is not an unit8.
func (vo valueObject) Uint8(name string) mo.Option[uint8] {
	return get[uint8](vo, name)
}

// MstUint8 returns the uint8 value for the given name.
// It panics if the key is not found or the value is not an unit8.
func (vo valueObject) MstUint8(name string) uint8 {
	return vo.Uint8(name).MustGet()
}

// Uint16 returns an Option containing the uint16 value for the given name.
// It panics if the field exists but is not an unit16.
func (vo valueObject) Uint16(name string) mo.Option[uint16] {
	return get[uint16](vo, name)
}

// MstUint16 returns the uint16 value for the given name.
// It panics if the key is not found or the value is not an unit16.
func (vo valueObject) MstUint16(name string) uint16 {
	return vo.Uint16(name).MustGet()
}

// Uint32 returns an Option containing the uint32 value for the given name.
// It panics if the field exists but is not an unit32.
func (vo valueObject) Uint32(name string) mo.Option[uint32] {
	return get[uint32](vo, name)
}

// MstUint32 returns the uint32 value for the given name.
// It panics if the key is not found or the value is not an unit32.
func (vo valueObject) MstUint32(name string) uint32 {
	return vo.Uint32(name).MustGet()
}

// Uint64 returns an Option containing the uint64 value for the given name.
// It panics if the field exists but is not an unit64.
func (vo valueObject) Uint64(name string) mo.Option[uint64] {
	return get[uint64](vo, name)
}

// MstUint64 returns the uint64 value for the given name.
// It panics if the key is not found or the value is not an unit64.
func (vo valueObject) MstUint64(name string) uint64 {
	return vo.Uint64(name).MustGet()
}

// Float64 Float returns an Option containing the float64 value for the given name.
// It panics if the field exists but is not a float64.
func (vo valueObject) Float64(name string) mo.Option[float64] {
	return get[float64](vo, name)
}

// MstFloat64 returns the float64 value for the given name.
// It panics if the key is not found or the value is not a float64.
func (vo valueObject) MstFloat64(name string) float64 {
	return vo.Float64(name).MustGet()
}

// Float32 returns an Option containing the float32 value for the given name.
// It panics if the field exists but is not a float32.
func (vo valueObject) Float32(name string) mo.Option[float32] {
	return get[float32](vo, name)
}

// MstFloat32 returns the float32 value for the given name.
// It panics if the key is not found or the value is not a float32.
func (vo valueObject) MstFloat32(name string) float32 {
	return vo.Float32(name).MustGet()
}

// Bool returns an Option containing the bool value for the given name.
// It panics if the field exists but is not a bool.
func (vo valueObject) Bool(name string) mo.Option[bool] {
	return get[bool](vo, name)
}

// MstBool returns the bool value for the given name.
// It panics if the key is not found or the value is not a bool.
func (vo valueObject) MstBool(name string) bool {
	return vo.Bool(name).MustGet()
}

// Time returns an Option containing the time.Time value for the given name.
// It panics if the field exists but is not a time.Time.
func (vo valueObject) Time(name string) mo.Option[time.Time] {
	return get[time.Time](vo, name)
}

// MstTime returns the time.Time value for the given name.
// It panics if the key is not found or the value is not a time.Time.
func (vo valueObject) MstTime(name string) time.Time {
	return vo.Time(name).MustGet()
}

func (vo *ViewObject) Validate(json string, urlParams ...map[string]string) mo.Result[ValueObject] {
	if len(json) > 0 && !gjson.Valid(json) {
		return mo.Err[ValueObject](fmt.Errorf("invalid json %s", json))
	}
	object := valueObject{}
	errs := &validationError{}
	// Check for unknown fields first if not allowed.
	voFields := lo.SliceToMap(vo.fields, func(field ViewField) (string, bool) {
		return field.Name(), field.IsArray() || field.IsObject()
	})
	urlPair := map[string]string{}
	for _, pair := range urlParams {
		for k, v := range pair {
			// self conflict check
			if _, ok := urlPair[k]; ok {
				errs.add(k, fmt.Errorf("duplicated url parameter '%s'", k))
			}
			if !vo.allowUnknownFields {
				if nested, ok := voFields[k]; !ok {
					errs.add(k, fmt.Errorf("unknown url parameter '%s'", k))
				} else if nested {
					errs.add(k, fmt.Errorf("url parameter '%s' is mapped to a embedded object", k))
				}
			}
			urlPair[k] = v
		}
	}

	lo.ForEach(gjson.Get(json, "@keys").Array(), func(field gjson.Result, index int) {
		jsonKey := field.String()
		if _, ok := urlPair[jsonKey]; ok {
			errs.add(jsonKey, fmt.Errorf("duplicate parameter in url and json '%s'", jsonKey))
		}
		if !vo.allowUnknownFields {
			if _, ok := voFields[jsonKey]; !ok {
				errs.add(jsonKey, fmt.Errorf("unknown json field '%s'", jsonKey))
			}
		}
	})

	// fail first for conflict
	if errs.err() != nil {
		return mo.Err[ValueObject](errs.err())
	}

	for _, field := range vo.fields {
		var rs mo.Result[any]
		node := gjson.Get(json, field.Name())
		if !node.Exists() {
			// need to check in urlPair
			urlValue, ok := urlPair[field.Name()]
			if !ok {
				if field.Required() {
					errs.add(field.Name(), fmt.Errorf("%s %w", field.Name(), constraint.ErrRequired))
				}
				continue
			}
			rs = field.validateRaw(urlValue)
		} else {
			rs = field.validate(node)
		}
		if rs.IsError() {
			// If the returned error is a validationError, it likely came from a
			// embedded validation (like an array). We should merge its errors
			// instead of nesting the error object, which would create ugly, duplicated messages.
			var nestedErr *validationError
			if errors.As(rs.Error(), &nestedErr) {
				for key, err := range nestedErr.errors {
					errs.add(key, err)
				}
			} else {
				errs.add(field.Name(), rs.Error())
			}
			continue
		}
		object[field.Name()] = rs.MustGet()
	}

	// Add unknown URL parameters to the final object if allowed.
	if vo.allowUnknownFields {
		for k, v := range urlPair {
			if _, exists := object[k]; !exists {
				object[k] = v
			}
		}
	}
	return lo.Ternary(errs.err() != nil, mo.Err[ValueObject](errs.err()), mo.Ok[ValueObject](object))
}
