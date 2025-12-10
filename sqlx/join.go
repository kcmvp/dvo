package sqlx

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/kcmvp/dvo"
	"github.com/kcmvp/dvo/entity"
)

// Joint represents a single SQL join clause.
type Joint interface {
	// Clause returns the SQL string for the join.
	Clause() string
}

// join is a private implementation of the Joint interface.
type join struct {
	clause string
}

func (j *join) Clause() string {
	return j.clause
}

// getTableFromField extracts the table name from a field's qualified name.
func getTableFromField(f entity.ViewFieldProvider) string {
	return strings.Split(f.QualifiedName(), ".")[0]
}

// Join creates an INNER JOIN clause between two fields.
func Join(f1, f2 entity.ViewFieldProvider) Joint {
	table2 := getTableFromField(f2)
	clause := fmt.Sprintf("INNER JOIN %s ON %s = %s", table2, f1.QualifiedName(), f2.QualifiedName())
	return &join{clause: clause}
}

// LeftJoin creates a LEFT JOIN clause between two fields.
func LeftJoin(f1, f2 entity.ViewFieldProvider) Joint {
	table2 := getTableFromField(f2)
	clause := fmt.Sprintf("LEFT JOIN %s ON %s = %s", table2, f1.QualifiedName(), f2.QualifiedName())
	return &join{clause: clause}
}

func View(providers ...entity.ViewFieldProvider) *Schema[entity.Entity] {
	dvoProviders := make([]dvo.FieldProvider, len(providers))
	for i, p := range providers {
		dvoProviders[i] = p
	}
	universalSchema := dvo.WithFields(dvoProviders...)
	return &Schema[entity.Entity]{
		Schema: universalSchema,
	}
}

// JoinQuery executes a multi-table SELECT...JOIN query.
func JoinQuery(ctx context.Context, schema *Schema[entity.Entity], where Where[entity.Entity], joints ...Joint) ([]ValueObject[entity.Entity], error) {
	panic("use joints to generate all kinds of join between entities")
}

// JoinDelete executes a multi-table DELETE...JOIN query.
// The target table for the deletion is specified via the generic type parameter T,
// which must be provided explicitly when calling the function.
//
// Example:
//   // This will generate a query like: DELETE FROM order ...
//   // The type 'Order' is used to identify the table to delete from.
//   JoinDelete[Order](ctx, where, join1, join2)
//
// The 'where' clause is of type Where[entity.Entity], allowing it to include
// conditions that reference any table involved in the join.
func JoinDelete[T entity.Entity](ctx context.Context, where Where[entity.Entity], joints ...Joint) (sql.Result, error) {
	var target T
	targetTable := target.Table()
	_ = targetTable // This would be used to build the query.
	panic("use joints to generate all kinds of join between entities")
}

// JoinUpdate executes a type-safe, multi-table UPDATE...JOIN query.
// The target table is inferred from the generic type T of the setter.
// The where clause is non-generic to allow conditions on multiple tables.
func JoinUpdate[T entity.Entity](ctx context.Context, where Where[entity.Entity], joints []Joint, setter ValueObject[T]) (sql.Result, error) {
	// Generates: UPDATE <T's table> JOIN ... SET ... WHERE ...
	var target T
	targetTable := target.Table()
	_ = targetTable // This would be used to build the query.
	panic("use joints to generate all kinds of join between entities")
}
