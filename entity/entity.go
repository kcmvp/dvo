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

// FieldProvider is a generic marker interface for fields that carry persistence metadata.
// The unexported method `seal(E)` ensures that only a field created for a specific
// entity `E` can satisfy this interface, providing strong, compile-time type safety.
type FieldProvider[E Entity] interface {
	dvo.FieldProvider
	QualifiedName() string
	seal(E)
}

// persistentField is the private generic struct that implements FieldProvider.
type persistentField[E Entity] struct {
	dvo.FieldProvider
	table string
}

// QualifiedName constructs the fully qualified "table.column" name at runtime.
// It first converts the embedded dvo.FieldProvider to a dvo.SchemaField, then calls Name().
func (f persistentField[E]) QualifiedName() string {
	return fmt.Sprintf("%s.%s", f.table, f.FieldProvider.AsSchemaField().Name())
}

// seal is the marker method that "uses" the generic type E,
// making the interface unique for each entity type and "sealing" it from external implementation.
func (f persistentField[E]) seal(e E) {}

// POField returns a entity.FieldProvider for use in persistence-layer schemas.
// The returned provider is strictly typed to the entity `E`, preventing cross-entity field mixing.
// It is a top-level factory function that constructs the fully qualified "table.column" name.
func POField[E Entity, T constraint.FieldType](name string, vfs ...constraint.ValidateFunc[T]) FieldProvider[E] {
	var entity E
	tableName := entity.Table()
	// Create the standard field provider with the simple name.
	fieldProvider := dvo.Field[T](name, vfs...)

	// Wrap it in our private generic struct to satisfy the FieldProvider[E] interface.
	return persistentField[E]{FieldProvider: fieldProvider, table: tableName}
}
