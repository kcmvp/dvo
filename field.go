package dvo

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kcmvp/dvo/entity"
	"github.com/samber/lo"
	"github.com/samber/mo"
)

// Package meta provides a minimal, stable metadata representation for
// entity fields used by the generator, SQL layer and view layer.
//
// The package is intentionally small: it describes the identity of fields
// (provider name, DB-qualified column, and view/JSON name) and supplies a
// tiny factory helper `NewField` that derives the table name from the
// concrete entity type. Validation logic may be attached to fields via
// validator factories but validators are considered a higher-level concern
// (often bound by the `view` package in runtime scenarios).

// Field is a sealed interface describing a single field's metadata.
//
// Implementations provide three read-only accessors:
//   - Name(): the canonical provider name (usually the exported Go field name)
//   - Qualified(): the DB-qualified column name in the form "table.column"
//   - ViewName(): the JSON/view facing name (the key used in validated objects)
//
// The unexported seal() method prevents external packages from
// implementing Field; only code inside this module (and generator-produced
// code that lives in the same module) may implement Field.
type Field interface {
	// Name returns the provider identifier used by generated code and by
	// view validation. It should be unique inside a Schema.
	Name() string
	// QualifiedName returns a DB-qualified column reference in the form
	// "table.column". Consumers (SQL builders) rely on this format.
	QualifiedName() string
	// ViewName returns the JSON/view key used when building or validating
	// ValueObjects. It may differ from Name() when a separate JSON key is
	// desired for presentation.
	ViewName() string
	// seal prevents external implementations of Field.
	seal()
}

type Number interface {
	uint | uint8 | uint16 | uint32 | uint64 | int | int8 | int16 | int32 | int64 | float32 | float64
}

// FieldType is a constraint for the actual Go types we want to validate.
type FieldType interface {
	Number | string | time.Time | bool
}

// PersistentField is a generic alias indicating this Field carries a
// Go type parameter used for validators or type hints. It currently does
// not add methods beyond Field but documents the intended semantic role.
type PersistentField[E FieldType] interface {
	Field
}

// persistentField is the concrete implementation of PersistentField.
// It is intentionally a simple, immutable value object holding:
//   - table: owning table name
//   - name: provider name (Go exported field name)
//   - column: DB column name
//   - viewName: JSON key name used by the view layer
//   - vfs: optional validator factory functions associated with the field
//
// Notes:
//   - The vfs slice is provided for convenience so generator code may
//     attach validator factories. If you adopt a strict layering rule
//     (validators belong to `view`), you may choose to omit or ignore vfs.
//   - persistentField is immutable after construction; callers should not
//     mutate its fields.
type persistentField[E FieldType] struct {
	table    string
	name     string
	column   string
	viewName string
	vfs      []ValidateFunc[E]
}

// Name returns the provider identifier.
func (f persistentField[E]) Name() string {
	return f.name
}

// ViewName returns the JSON/view-facing key associated with this field.
func (f persistentField[E]) ViewName() string {
	return f.viewName
}

// seal implements the sealed interface marker to prevent third-party
// implementations of Field.
func (f persistentField[E]) seal() {}

// QualifiedName returns the DB-qualified column name in the form "table.column".
// This is the canonical format used across SQL generation and token
// rewriting logic.
func (f persistentField[E]) QualifiedName() string {
	return fmt.Sprintf("%s.%s", f.table, f.column)
}

var _ PersistentField[int64] = (*persistentField[int64])(nil)

// NewField is a convenience factory for creating a PersistentField.
//
// Parameters:
//   - E: a concrete type that implements entity.Entity. The factory will
//     obtain the table name by calling a zero value of E: `var e E; e.Table()`.
//   - name: provider name (usually the exported Go field name)
//   - column: the DB column name (snake_case or explicitly provided by tags)
//   - view: the view/JSON key name
//   - vfs: optional validator factory functions for the field (typically
//     used by generator or view binding code)
//
// Behavior:
//   - The factory asserts that table, name, column and view are not empty
//     (developer-time assertions via lo.Assert). These assertions will
//     panic on programmer error (generator misconfiguration or wrong usage).
//   - The returned PersistentField is safe to share and is treated as
//     immutable.
//
// Usage example (generator or manual):
//
//	var f = NewField[Account, int64]("ID", "id", "id")
func NewField[E entity.Entity, T FieldType](name string, column string, view string, vfs ...ValidateFunc[T]) PersistentField[T] {
	var e E
	table := e.Table()
	lo.Assert(table != "", "table must not return empty string")
	lo.Assert(name != "", "name must not return empty string")
	lo.Assert(column != "", "column must not return empty string")
	lo.Assert(view != "", "view must not return empty string")
	return &persistentField[T]{
		table:    table,
		name:     name,
		column:   column,
		viewName: view,
		vfs:      vfs,
	}
}

// Schema is a minimal container: an ordered list of Field descriptors for
// an entity. The generator produces Schema values and the runtime code
// (SQL builders, view validators) consumes them. Keep Schema small and
// prefer generator-emitted Schemas to ad-hoc reflection at runtime.
type Schema []Field

// ValueObject is a minimal, sealed interface that represents a validated
// or mapped payload. meta.ValueObject is intentionally small and non-panicking
// (returns options) so higher-level packages (view) may wrap it with
// convenience panic-based getters if desired.
//
// The interface mirrors the access patterns consumers expect: typed getters
// returning `mo.Option[T]` and Must* variants that return the typed value
// directly (and may panic). Since the meta layer aims to be minimal, prefer
// using the Option-based getters at API boundaries.
type ValueObject interface {
	String(name string) mo.Option[string]
	MstString(name string) string
	Int(name string) mo.Option[int]
	MstInt(name string) int
	Int8(name string) mo.Option[int8]
	MstInt8(name string) int8
	Int16(name string) mo.Option[int16]
	MstInt16(name string) int16
	Int32(name string) mo.Option[int32]
	MstInt32(name string) int32
	Int64(name string) mo.Option[int64]
	MstInt64(name string) int64
	Uint(name string) mo.Option[uint]
	MstUint(name string) uint
	Uint8(name string) mo.Option[uint8]
	MstUint8(name string) uint8
	Uint16(name string) mo.Option[uint16]
	MstUint16(name string) uint16
	Uint32(name string) mo.Option[uint32]
	MstUint32(name string) uint32
	Uint64(name string) mo.Option[uint64]
	MstUint64(name string) uint64
	Float64(name string) mo.Option[float64]
	MstFloat64(name string) float64
	Float32(name string) mo.Option[float32]
	MstFloat32(name string) float32
	Bool(name string) mo.Option[bool]
	MstBool(name string) bool
	Time(name string) mo.Option[time.Time]
	MstTime(name string) time.Time
	Get(string) mo.Option[any]
	Add(name string, value any)
	Update(name string, value any)
	// Fields returns the list of field names of the value object.
	Fields() []string
	seal()
}

// valueObject is a concrete map-backed implementation of ValueObject.
// It intentionally follows the same semantics as the view layer's
// implementation: getters use dot-notation for nested access and typed
// Option results; Must* variants call Option.MustGet() (panicking on None).
// Add and Update operate only on top-level keys and panic on contract violations.
type valueObject map[string]any

func (vo valueObject) Get(s string) mo.Option[any] {
	return get[any](vo, s)
}

// Keys returns all top-level keys present in the value object. The list is
// sorted to ensure a deterministic order for callers and tests.
func (vo valueObject) Fields() []string {
	ks := make([]string, 0, len(vo))
	for k := range vo {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// Add adds a new property to the value object.
// It panics if the property already exists or if the name contains '.'.
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
// It panics if the key exists but the type is incorrect. This function
// supports dot notation for embedded objects and array indexing (e.g., "field.0.nestedField").
func get[T any](data valueObject, name string) mo.Option[T] {
	parts := strings.Split(name, ".")
	var currentValue any = data
	for _, part := range parts {
		if currentValue == nil {
			return mo.None[T]()
		}
		// If it's a map, look up the key.
		if voMap, ok := currentValue.(valueObject); ok {
			nextValue, exists := voMap[part]
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
// It panics if the field exists but is not a uint.
func (vo valueObject) Uint(name string) mo.Option[uint] {
	return get[uint](vo, name)
}

// MstUint returns the uint value for the given name.
// It panics if the key is not found or the value is not a uint.
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

// NewValueObject creates a meta.ValueObject backed by the provided map.
//
// This is primarily used by lower layers (like sqlx) when mapping database rows
// into value objects.
//
// Notes:
//   - The returned ValueObject is backed by the given map; callers should treat
//     the map as immutable after passing it in.
func NewValueObject(m map[string]any) ValueObject {
	if m == nil {
		m = map[string]any{}
	}
	// valueObject is the package-private implementation of ValueObject.
	return valueObject(m)
}
