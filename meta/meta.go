package meta

import (
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/kcmvp/dvo/validator"
)

// FieldMeta contains canonical metadata for a field emitted by the generator
// and consumed at runtime by entity/field/value/view layers.
// This struct is intentionally lightweight and has no dependency on higher-level
// packages; it is used as the canonical description of a struct field.
type FieldMeta struct {
	// Provider is the identifier used in generated Go code (e.g. "ID").
	Provider string
	// Column is the database column name (e.g. "id").
	Column string
	// JSONName is the JSON key for this field (e.g. "id").
	JSONName string
	// GoType is the full Go type name (e.g. "int64" or "time.Time").
	GoType string
	// SQLType is an optional mapped SQL type name (e.g. "INTEGER", "TEXT").
	SQLType string
	// IsPK marks this field as primary key.
	IsPK bool
	// Order signals the preferred ordering of columns when generating schemas.
	// Lower numbers are emitted earlier. Zero means unspecified.
	Order int
	// Tags stores struct tag values parsed from source (e.g. map of "xql" entries).
	Tags map[string]string
	// IsExported indicates whether the original struct field was exported.
	IsExported bool
	// IsEmbedded indicates the field came from an anonymous embedded struct.
	IsEmbedded bool
	// Comment optionally carries a developer comment for generated output.
	Comment string
}

// Field creates a FieldMeta value. It intentionally does not depend on dvo
// to avoid cyclic imports. Validators are not captured here; they belong to
// the view layer. Generator may enrich the returned FieldMeta with type/SQL/tag
// information after scanning.
func Field(name string, col string, jsonName string) FieldMeta {
	return FieldMeta{
		Provider:   name,
		Column:     col,
		JSONName:   jsonName,
		GoType:     "",
		SQLType:    "",
		IsPK:       false,
		Order:      0,
		Tags:       nil,
		IsExported: true,
		IsEmbedded: false,
		Comment:    "",
	}
}

// DefaultTimeLayouts are the default layouts used to parse time strings.
var DefaultTimeLayouts = []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"}

// ParseStringTo converts a string into the specified FieldType T. It returns the
// parsed value or an error. This function centralizes string->typed conversions
// for reuse by view and value layers.
func ParseStringTo[T validator.FieldType](s string) (T, error) {
	var zero T
	targetType := reflect.TypeOf(zero)

	switch targetType.Kind() {
	case reflect.String:
		return any(s).(T), nil
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return zero, fmt.Errorf("could not parse '%s' as bool: %w", s, err)
		}
		return any(b).(T), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return zero, fmt.Errorf("could not parse '%s' as int: %w", s, err)
		}
		if reflect.New(targetType).Elem().OverflowInt(val) {
			return zero, OverflowError(zero)
		}
		return reflect.ValueOf(val).Convert(targetType).Interface().(T), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return zero, fmt.Errorf("could not parse '%s' as uint: %w", s, err)
		}
		if reflect.New(targetType).Elem().OverflowUint(val) {
			return zero, OverflowError(zero)
		}
		return reflect.ValueOf(val).Convert(targetType).Interface().(T), nil
	case reflect.Float32, reflect.Float64:
		val, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return zero, fmt.Errorf("could not parse '%s' as float: %w", s, err)
		}
		if reflect.New(targetType).Elem().OverflowFloat(val) {
			return zero, fmt.Errorf("value %f overflows type %T", val, zero)
		}
		return reflect.ValueOf(val).Convert(targetType).Interface().(T), nil
	case reflect.Struct:
		if targetType == reflect.TypeOf(time.Time{}) {
			for _, layout := range DefaultTimeLayouts {
				if t, err := time.Parse(layout, s); err == nil {
					return any(t).(T), nil
				}
			}
			return zero, fmt.Errorf("incorrect date format for string '%s'", s)
		}
		fallthrough
	default:
		return zero, fmt.Errorf("type mismatch or unsupported type %T", zero)
	}
}

// OverflowError returns a standard overflow error wrapping the validator ErrIntegerOverflow.
func OverflowError[T any](v T) error {
	return fmt.Errorf("for type %T: %w", v, validator.ErrIntegerOverflow)
}
