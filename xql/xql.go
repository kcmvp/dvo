package xql

import (
	"context"
	"database/sql"
	"strings"

	"fmt"

	"github.com/kcmvp/dvo"
	"github.com/kcmvp/dvo/entity"
	"github.com/samber/lo"
)

// Schema is a type-safe, generic schema bound to a specific entity T.
// It embeds a universal dvo.Schema to remain compatible with core validation logic.
// The zero-length array `_ [0]T` is a common Go idiom to associate a generic type
// with a struct without incurring any memory overhead for a stored field.
type Schema[T entity.Entity] struct {
	*dvo.Schema
	_ [0]T
}

// ValueObject (Persistent Object) is a type-safe, generic wrapper around a universal dvo.ValueObject.
// It uses the zero-length array idiom to associate the entity type T at compile time
// without any memory cost, making it a lightweight, type-safe handle.
type ValueObject[T entity.Entity] struct {
	dvo.ValueObject
	_ [0]T
}

// NewSchema creates a new, type-safe, generic schema for a persistent object.
// It requires providers to be of the type entity.FieldProvider[T],
// guaranteeing at compile time that only fields belonging to entity T can be used.
func NewSchema[T entity.Entity](providers ...entity.FieldProvider[T]) *Schema[T] {
	// We need to convert the slice of entity.FieldProvider[T] back to []dvo.FieldProvider
	// for the universal WithFields function.
	dvoProviders := make([]dvo.FieldProvider, len(providers))
	for i, p := range providers {
		dvoProviders[i] = p
	}

	// 1. Create the universal Schema first using the core library function.
	universalSchema := dvo.WithFields(dvoProviders...)

	// 2. Wrap it in our generic Schema.
	return &Schema[T]{
		Schema: universalSchema,
	}
}

// Query is a generic function that queries the database and returns a slice of T.
// It uses the provided schema to build the SELECT clause and the Where interface to build the WHERE clause.
func Query[T entity.Entity](ctx context.Context, schema *Schema[T], where Where[T]) ([]ValueObject[T], error) {
	panic("implement me")
}

func Delete[T entity.Entity](ctx context.Context, where Where[T]) (sql.Result, error) {
	panic("implement me")
}

func Insert[T entity.Entity](ctx context.Context, po ValueObject[T]) (sql.Result, error) {
	panic("implement me")
}

func Update[T entity.Entity](ctx context.Context, set ValueObject[T], where Where[T]) (sql.Result, error) {
	panic("implement me")
}

// Where is a generic, single-method interface representing a query condition.
// It is bound to a specific entity type T.
type Where[T entity.Entity] interface {
	// Build returns the SQL clause string and its corresponding arguments.
	Build() (string, []any)
}

// whereFunc is a function type that implements the Where interface.
// This is a common Go pattern that allows plain functions to be used as interfaces,
// enabling a fluent, functional API for building queries.
type whereFunc[T entity.Entity] func() (string, []any)

// Build calls the function itself, satisfying the Where interface.
func (f whereFunc[T]) Build() (string, []any) {
	return f()
}

// And combines multiple Where conditions with the AND operator.
// It filters out any nil or empty Where functions.
func And[T entity.Entity](wheres ...Where[T]) Where[T] {
	f := func() (string, []any) {
		clauses := make([]string, 0, len(wheres))
		var allArgs []any
		for _, w := range wheres {
			if w != nil {
				clause, args := w.Build()
				if clause != "" {
					clauses = append(clauses, clause)
					allArgs = append(allArgs, args...)
				}
			}
		}
		if len(clauses) == 0 {
			return "", nil
		}
		return fmt.Sprintf("(%s)", strings.Join(clauses, " AND ")), allArgs
	}
	return whereFunc[T](f)
}

// Or combines multiple Where conditions with the OR operator.
// It filters out any nil or empty Where functions.
func Or[T entity.Entity](wheres ...Where[T]) Where[T] {
	f := func() (string, []any) {
		clauses := make([]string, 0, len(wheres))
		var allArgs []any
		for _, w := range wheres {
			if w != nil {
				clause, args := w.Build()
				if clause != "" {
					clauses = append(clauses, clause)
					allArgs = append(allArgs, args...)
				}
			}
		}
		if len(clauses) == 0 {
			return "", nil
		}
		return fmt.Sprintf("(%s)", strings.Join(clauses, " OR ")), allArgs
	}
	return whereFunc[T](f)
}

// op is a helper function to create a simple binary operator condition.
func op[E entity.Entity](field entity.FieldProvider[E], operator string, value any) Where[E] {
	f := func() (string, []any) {
		clause := fmt.Sprintf("%s %s ?", field.QualifiedName())
		args := []any{value}
		return clause, args
	}
	return whereFunc[E](f)
}

// Eq creates an "equal to" condition (e.g., "name = ?").
func Eq[E entity.Entity](field entity.FieldProvider[E], value any) Where[E] {
	return op(field, "=", value)
}

// Ne creates a "not equal to" condition (e.g., "status != ?").
func Ne[E entity.Entity](field entity.FieldProvider[E], value any) Where[E] {
	return op(field, "!=", value)
}

// Gt creates a "greater than" condition (e.g., "price > ?").
func Gt[E entity.Entity](field entity.FieldProvider[E], value any) Where[E] {
	return op(field, ">", value)
}

// Gte creates a "greater than or equal to" condition (e.g., "stock >= ?").
func Gte[E entity.Entity](field entity.FieldProvider[E], value any) Where[E] {
	return op(field, ">=", value)
}

// Lt creates a "less than" condition (e.g., "age < ?").
func Lt[E entity.Entity](field entity.FieldProvider[E], value any) Where[E] {
	return op(field, "<", value)
}

// Lte creates a "less than or equal to" condition (e.g., "discount <= ?").
func Lte[E entity.Entity](field entity.FieldProvider[E], value any) Where[E] {
	return op(field, "<=", value)
}

// Like creates a "LIKE" condition (e.g., "email LIKE ?").
func Like[E entity.Entity](field entity.FieldProvider[E], value string) Where[E] {
	return op(field, "LIKE", value)
}

// In creates an "IN (...)" condition.
// It handles empty value slices gracefully by returning an always-false condition
// to prevent SQL syntax errors, which is safer than returning an empty string.
func In[E entity.Entity](field entity.FieldProvider[E], values ...any) Where[E] {
	f := func() (string, []any) {
		if len(values) == 0 {
			return "1=0", nil // Always false, prevents syntax error with empty IN ()
		}
		placeholders := strings.Join(lo.RepeatBy(len(values), func(_ int) string {
			return "?"
		}), ",")
		clause := fmt.Sprintf("%s IN (%s)", field.QualifiedName(), placeholders)
		return clause, values
	}
	return whereFunc[E](f)
}
