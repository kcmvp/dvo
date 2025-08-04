package dvo

import (
	"fmt"
	"github.com/samber/mo"
	"github.com/tidwall/gjson"
	"strings"
	"time"
)

// A private type to prevent key collisions in context.
type viewObjectKeyType struct{}

// ViewObjectKey is the key used to store the validated data map in the request context.
var ViewObjectKey = viewObjectKeyType{}

type Number interface {
	int | int8 | int16 | int32 | int64 | float32 | float64
}

// JSONType is a constraint for the actual Go types we want to validate.
type JSONType interface {
	Number | string | time.Time | bool
}

// Constraint is a pure, type-safe function that receives an already-typed value.
type Constraint[T JSONType] func(val T) error

// ValidationErrors is a custom error type that holds a slice of validation errors.
type ValidationErrors []error

// Error implements the error interface, formatting all contained errors.
func (v *ValidationErrors) Error() string {
	if v == nil || len(*v) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("validation failed with the following errors:\n")
	for _, err := range *v {
		b.WriteString(fmt.Sprintf("- %s\n", err.Error()))
	}
	return b.String()
}

// Add appends a new error to the list if it's not nil.
func (v *ValidationErrors) Add(err error) {
	if err != nil {
		*v = append(*v, err)
	}
}

// Err returns the ValidationErrors as a single error if it contains any errors.
func (v *ValidationErrors) Err() error {
	if v == nil || len(*v) == 0 {
		return nil
	}
	return v
}

// viewField is an internal, non-generic interface that allows ViewObject
// to hold a collection of fields with different underlying generic types.
type viewField interface {
	Name() string
	Required() bool
	validate(json string) (any, error)
}

type ViewField[T JSONType] struct {
	name        string
	required    bool
	constraints []Constraint[T]
}

var _ viewField = (*ViewField[string])(nil)

func (f *ViewField[T]) Name() string {
	return f.name
}

func (f *ViewField[T]) Required() bool {
	return f.required
}

func (f *ViewField[T]) validate(json string) (any, error) {
	rs := f.Validate(json)
	if rs.IsError() {
		return nil, rs.Error()
	}
	return rs.MustGet(), nil
}

func (f *ViewField[T]) Validate(json string) mo.Result[T] {
	panic("@todo")
}

// typed is an unexported helper utility to convert a gjson.Result to a specific JSONType.
func typed[T JSONType](res gjson.Result) mo.Result[T] {
	var zero T
	switch any(zero).(type) {
	case string:
		if res.Type == gjson.String {
			return mo.Ok(any(res.String()).(T))
		}
	case bool:
		if res.Type == gjson.True || res.Type == gjson.False {
			return mo.Ok(any(res.Bool()).(T))
		}
	case int, int8, int16, int32, int64:
		if res.Type == gjson.Number {
			return mo.Ok(any(int(res.Int())).(T))
		}
	case float32, float64:
		if res.Type == gjson.Number {
			return mo.Ok(any(res.Float()).(T))
		}
	case time.Time:
		if res.Type == gjson.String {
			t, err := time.Parse(time.RFC3339, res.String())
			if err == nil {
				return mo.Ok(any(t).(T))
			}
		}
	}
	return mo.Err[T](fmt.Errorf("type mismatch: expected %T but got JSON type %s", zero, res.Type))
}

// Field is a factory that creates a new field definition.
// Use the fluent methods like .Required() to add validation rules.
func Field[T JSONType](name string, constraints ...Constraint[T]) *ViewField[T] {
	return &ViewField[T]{
		name:        name,
		constraints: constraints,
		required:    true,
	}
}

// ViewObject is a blueprint for validating a JSON object.
type ViewObject struct {
	fields             []viewField
	allowUnknownFields bool
}

// WithFields is the constructor for a ViewObject blueprint.
func WithFields(fields ...viewField) *ViewObject {
	return &ViewObject{fields: fields, allowUnknownFields: false}
}

// AllowUnknownFields is a fluent method to make the ViewObject accept JSON
// that contains fields not defined in the schema. Default behavior is to disallow.
func (vo *ViewObject) AllowUnknownFields() *ViewObject {
	vo.allowUnknownFields = true
	return vo
}

type Data map[string]any

func (data Data) Int(name string) int {
	panic("")
}

func (data Data) Bool(name string) bool {
	panic("")
}

func (data Data) Int64(name string) int64 {
	panic("")
}

func (data Data) String(name string) string {
	panic("")
}

func (data Data) Time(name string) time.Time {
	panic("")
}

func (data Data) Float(name string) float64 {
	panic("")
}

func (vo *ViewObject) Validate(json string) mo.Result[Data] {
	errs := ValidationErrors{}
	data := Data{}
	for _, field := range vo.fields {
		v, err := field.validate(json)
		if err != nil {
			errs.Add(err)
			continue
		}
		data[field.Name()] = v
	}
	return mo.TupleToResult(data, errs.Err())
}
