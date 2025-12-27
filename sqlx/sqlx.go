package sqlx

import (
	"context"
	"database/sql"

	"github.com/kcmvp/xql"
	"github.com/kcmvp/xql/entity"
	"github.com/kcmvp/xql/internal"
	"github.com/samber/mo"
)

// Package sqlx
//
// sqlx provides a small, generator-friendly SQL DSL and execution layer
// for single-table CRUD operations. The package is intentionally compact
// and designed to work with generator-produced metadata (`meta.Schema`,
// `meta.Field`, `meta.ValueObject`).
//
// Design principles (short):
//  - SQL generation is pure: `sql()` returns only the SQL string and error.
//    Execution-time arguments are produced by the lower-level builder
//    helpers (e.g. `selectSQL`, `insertSQL`, `updateSQL`, `deleteSQL`) so
//    generation and execution responsibilities are clearly separated.
//  - Public API is tiny: consumers construct `Executor` via `Query` /
//    `Delete` / `Update` factory functions and call `Execute(ctx, db)` to
//    run the statement. `Execute` returns a union `mo.Either` value where
//    the left side is `[]meta.ValueObject` (SELECT results) and the right
//    side is `sql.Result` (non-query statements).
//  - The package depends on generator-provided `meta.Schema` to determine
//    projection order and mapping between SQL result columns and value
//    object keys.
//
// Important helper contracts (implemented in this package):
//  - selectSQL[T](schema *meta.Schema, where Where) (string, []any, error)
//  - insertSQL[T](schema meta.Schema, g getter) (string, []any, error)
//  - updateSQL[T](schema meta.Schema, g getter, where Where) (string, []any, error)
//  - deleteSQL[T](where Where) (string, []any, error)
//
// NOTE: `updateExec` is currently a placeholder (not implemented).

// Where is the only contract used to express predicates for CRUD.
//
// The DSL helpers in this package (And/Or/Eq/In/Like/...) produce values
// that implement `Where`. `Where.Build()` returns a SQL fragment and the
// parameter list suitable for use in a prepared statement.
type Where interface {
	Build() (string, []any)
}

type Schema []xql.Field

// --- Where DSL helpers (public) ---

// And combines multiple Where conditions with the AND operator.
// Nil or empty clauses are ignored.
func And(wheres ...Where) Where {
	return and(wheres...)
}

// Or combines multiple Where conditions with the OR operator.
// Nil or empty clauses are ignored.
func Or(wheres ...Where) Where {
	return or(wheres...)
}

// Eq builds a "field = ?" predicate.
func Eq(field xql.Field, value any) Where {
	return op(field, "=", value)
}

// Ne builds a "field != ?" predicate.
func Ne(field xql.Field, value any) Where {
	return op(field, "!=", value)
}

// Gt builds a "field > ?" predicate.
func Gt(field xql.Field, value any) Where {
	return op(field, ">", value)
}

// Gte builds a "field >= ?" predicate.
func Gte(field xql.Field, value any) Where {
	return op(field, ">=", value)
}

// Lt builds a "field < ?" predicate.
func Lt(field xql.Field, value any) Where {
	return op(field, "<", value)
}

// Lte builds a "field <= ?" predicate.
func Lte(field xql.Field, value any) Where {
	return op(field, "<=", value)
}

// Like builds a "field LIKE ?" predicate.
func Like(field xql.Field, value string) Where {
	return op(field, "LIKE", value)
}

// In builds a "field IN (?, ?, ...)" predicate.
// Empty values produce an always-false clause (1=0).
func In(field xql.Field, values ...any) Where {
	return inWhere(field, values...)
}

// Executor represents the delayed execution step constructed by the
// top-level factory helpers (`Query`, `Delete`, `Update`).
//
// Execution contract:
//   - Callers obtain an Executor via one of the factory functions and then
//     call `Execute(ctx, db)` to run the statement. Parameter values for the
//     prepared statement are set during `Where.Build()` (the Where returns
//     the SQL fragment and its argument list). For INSERT/UPDATE builders
//     which consume a `meta.ValueObject` for column values, those helpers
//     read the ValueObject inside their builder functions.
//   - `Execute` will call the appropriate lower-level builder helper to
//     obtain both the SQL string and the argument list (e.g. `selectSQL`).
//   - For SELECT statements the left side of `mo.Either` holds the
//     `[]meta.ValueObject` results; for non-SELECT statements the right
//     side holds the `sql.Result`.
//
// Note: `sql()` is kept pure and only returns the generated SQL string and
// an error. It is primarily useful for testing and inspection.
type Executor interface {
	Execute(ctx context.Context, ds *sql.DB) (mo.Either[[]ValueObject, sql.Result], error)
	// sql generates the SQL string only (pure). Arguments are produced by lower-level helpers
	// (selectSQL/insertSQL/updateSQL/deleteSQL) and consumed by Execute when running against DB.
	sql() (string, error)
}

// Query builds a single-table SELECT query.
//
// Usage example:
//
//	// build executor
//	exec := Query[Account](schema)(Eq(field, value))
//	// run
//	resEither, err := exec.Execute(ctx, db)
//	// check left/right and handle accordingly
func Query[T entity.Entity](schema Schema) func(where Where) Executor {
	return func(where Where) Executor {
		return queryExec[T]{schema: schema, where: where}
	}
}

// Delete builds a single-table DELETE query.
//
// Note: per design, callers should provide a non-empty where clause; the
// implementation enforces this at builder time (deleteSQL returns error if
// where is empty).
func Delete[T entity.Entity](where Where) Executor {
	return deleteExec[T]{where: where}
}

// Update builds a single-table UPDATE query.
//
// New design: public Update accepts the update payload as a meta.ValueObject;
// the ValueObject may include a special "__schema" entry or provide its own
// Fields() listing. This avoids a global runtime schema registry.
func Update[T entity.Entity](values ValueObject) func(where Where) Executor {
	return func(where Where) Executor {
		return updateExec[T]{values: values, where: where}
	}
}

// QueryJoin builds a select executor that injects `joinstmt` into the FROM
// clause. The returned Executor follows the existing `Executor` contract.
func QueryJoin(schema Schema) func(joinstmt string, where Where) Executor {
	return func(joinstmt string, where Where) Executor {
		return joinQueryExec{schema: schema, joinstmt: joinstmt, where: where}
	}
}

// DeleteJoin builds a delete executor that uses an EXISTS-correlated subquery
// to apply the join-based filter. It derives base table from generic type T.
func DeleteJoin[T entity.Entity](joinstmt string, where Where) Executor {
	var ent T
	baseTable := ent.Table()
	return joinDeleteExec{baseTable: baseTable, joinstmt: joinstmt, where: where}
}

// UpdateJoin builds an update executor that applies an EXISTS-correlated
// join filter. The update payload values are supplied as a meta.ValueObject
// when creating the executor via UpdateJoin[T](values)(joinstmt, where).
func UpdateJoin[T entity.Entity](values ValueObject) func(joinstmt string, where Where) Executor {
	return func(joinstmt string, where Where) Executor {
		return updateJoinExec[T]{values: values, joinstmt: joinstmt, where: where}
	}
}

// ValueObject is a thin alias over internal.ValueObject to expose it
type ValueObject interface {
	internal.ValueObject
	seal()
}

type valueObject struct {
	internal.Data
}

var _ ValueObject = (*valueObject)(nil)

func (vo valueObject) seal() {}

func NewValueObject(m map[string]any) ValueObject {
	if m == nil {
		m = map[string]any{}
	}
	return valueObject{Data: m}
}
