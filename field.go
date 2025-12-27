package xql

import (
	"fmt"
	"time"

	"github.com/kcmvp/xql/entity"
	"github.com/samber/lo"
)

// Package dvo provides a compact, stable representation of entity field
// metadata used by code generation, SQL builders and view/JSON validation.
//
// Key goals:
//  - Keep the metadata model small and deterministic so generated Schemas
//    are easy to inspect and consume at runtime.
//  - Expose only a read-only API for field descriptors. Concrete
//    implementations are sealed to this module to avoid accidental
//    third-party implementations that break expectations.
//  - Provide a tiny factory helper `NewField` that derives the DB table
//    name from a concrete entity type (via `entity.Entity`) and validates
//    basic invariants early.
//
// Typical usage (generator-emitted code):
//
//   var ID = NewField[Account, int64]("ID", "id", "id")
//   // Schema values are slices of Field produced by generator code.
//
// Notes on layering:
//  - Validator factories (`ValidateFunc`) may be attached to fields by the
//    generator for convenience. In strict layering designs validators can
//    be owned by the `view` package at runtime.

// Field is a sealed interface describing a single field's metadata.
//
// Implementations provide three read-only accessors:
//   - Name(): the canonical provider name (usually the exported Go field name)
//   - QualifiedName(): the DB-qualified column name in the form "table.column"
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

// Number is a type constraint for numeric native Go types.
type Number interface {
	uint | uint8 | uint16 | uint32 | uint64 |
		int | int8 | int16 | int32 | int64 |
		float32 | float64
}

// FieldType is a constraint for the concrete Go types that fields may
// carry as type hints for validators and code generation.
type FieldType interface {
	Number | string | time.Time | bool
}

// PersistentField is a semantic alias: a Field that carries a Go type
// parameter to enable type-safe validator factories or codegen hints.
type PersistentField[E FieldType] interface {
	Field
}

// persistentField is the internal, immutable implementation of
// PersistentField. The fields are all unexported; instances are produced
// using `NewField`.
type persistentField[E FieldType] struct {
	table    string
	name     string
	column   string
	viewName string
	vfs      []ValidateFunc[E]
}

// Name returns the provider identifier (usually the Go exported field name).
func (f persistentField[E]) Name() string { return f.name }

// ViewName returns the JSON/view-facing key associated with this field.
func (f persistentField[E]) ViewName() string { return f.viewName }

// seal implements the package-only sealing marker.
func (f persistentField[E]) seal() {}

// QualifiedName returns the DB-qualified column name in the canonical
// "table.column" format used by SQL builders and token rewriting.
func (f persistentField[E]) QualifiedName() string {
	return fmt.Sprintf("%s.%s", f.table, f.column)
}

var _ PersistentField[int64] = (*persistentField[int64])(nil)

// NewField creates a PersistentField for entity type E with Go type hint T.
//
// Parameters:
//   - name: provider identifier (Go field name). Must be non-empty.
//   - column: DB column name. Must be non-empty.
//   - view: JSON/view key name. Must be non-empty.
//   - vfs: optional validator factory functions for the field.
//
// Behavior:
//   - The table name is derived by instantiating a zero value of E and
//     calling its Table() method. The function asserts the table and the
//     provided strings are non-empty using `lo.Assert`.
//
// Example:
//
//	var ID = NewField[Account, int64]("ID", "id", "id")
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
