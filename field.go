package dvo

import (
	"fmt"
	"time"

	"github.com/kcmvp/dvo/entity"
	"github.com/samber/lo"
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

// var f = NewField[Account, int64]("ID", "id", "id")
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
