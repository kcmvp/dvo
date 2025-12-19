package entity

import (
	"fmt"

	"github.com/kcmvp/dvo"
	"github.com/kcmvp/dvo/constraint"
)

// Entity defines the contract for database-aware models.
type Entity interface {
	Table() string
}

// JoinFieldProvider is a non-generic interface that all entity.FieldProvider[E] instances satisfy.
// It is used to accept fields from different entities in join queries.
// The unexported method ensures that only types from this package can implement it.
type JoinFieldProvider interface {
	dvo.FieldProvider
	QualifiedName() string
	seal()
}

// FieldProvider is a generic marker interface for fields that carry persistence metadata.
// It embeds the sealed JoinFieldProvider interface.
type FieldProvider[E Entity] interface {
	JoinFieldProvider
}

// persistentField is the private generic struct that implements FieldProvider.
type persistentField[E Entity] struct {
	dvo.FieldProvider
	table string
}

// seal is a marker method to satisfy the JoinFieldProvider interface.
func (f persistentField[E]) seal() {}

// QualifiedName constructs the fully qualified "table.column" name at runtime.
// It first converts the embedded dvo.FieldProvider to a dvo.SchemaField, then calls Name().
func (f persistentField[E]) QualifiedName() string {
	return fmt.Sprintf("%s.%s", f.table, f.FieldProvider.AsSchemaField().Name())
}

// Field returns a entity.FieldProvider for use in persistence-layer schemas.
// The returned provider is strictly typed to the entity `E`, preventing cross-entity field mixing.
// It is a top-level factory function that constructs the fully qualified "table.column" name.
func Field[E Entity, T constraint.FieldType](name string, vfs ...constraint.ValidateFunc[T]) FieldProvider[E] {
	var entity E
	tableName := entity.Table()
	// Create the standard field provider with the simple name.
	fieldProvider := dvo.Field[T](name, vfs...)

	// Wrap it in our private generic struct to satisfy the FieldProvider[E] interface.
	return persistentField[E]{FieldProvider: fieldProvider, table: tableName}
}
