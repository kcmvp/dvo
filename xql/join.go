package xql

import (
	"context"
	"fmt"

	"github.comcom/kcmvp/dvo"
	"github.com/kcmvp/dvo/entity"
)

// JointPO is a type-safe, generic wrapper for a joined persistent object result.
// T is expected to be a struct that holds the combined fields from a JOIN query.
type JointPO[T any] struct {
	dvo.ValueObject
	_ [0]T
}

// NewJointSchema creates a new schema for a JOIN query.
// It is a generic function that accepts a struct type `T` which will hold the query result.
// Crucially, it accepts FieldProviders from ANY entity, providing the flexibility needed for JOINs.
// The responsibility of providing the correct fields from the joined tables lies with the developer.
func NewJointSchema[T any](providers ...dvo.FieldProvider) *Schema[T] {
	// This function is very similar to NewSchema, but the generic constraint is `any`,
	// allowing it to be used for arbitrary result structs.
	universalSchema := dvo.WithFields(providers...)
	return &Schema[T]{
		Schema: universalSchema,
	}
}

// XJoin represents a single JOIN clause in a multi-table query.
type XJoin interface {
	Clause() string
}

// Joint represents an INNER JOIN operation.
// It holds the left and right fields of the ON clause.
type Joint[E1 entity.Entity, E2 entity.Entity] struct {
	Left  entity.FieldProvider[E1]
	Right entity.FieldProvider[E2]
}

func (j Joint[E1, E2]) Clause() string {
	var leftEntity E1
	var rightEntity E2
	return fmt.Sprintf("INNER JOIN %s ON %s = %s",
		rightEntity.Table(),
		j.Left.QualifiedName(),
		j.Right.QualifiedName(),
	)
}

// LeftJoint represents a LEFT JOIN operation.
type LeftJoint[E1 entity.Entity, E2 entity.Entity] struct {
	Left  entity.FieldProvider[E1]
	Right entity.FieldProvider[E2]
}

func (j LeftJoint[E1, E2]) Clause() string {
	var leftEntity E1
	var rightEntity E2
	return fmt.Sprintf("LEFT JOIN %s ON %s = %s",
		rightEntity.Table(),
		j.Left.QualifiedName(),
		j.Right.QualifiedName(),
	)
}

// JoinQuery executes a multi-table JOIN query.
// The function itself is not generic over the result type `T` in its signature,
// but it achieves type safety through the generic `schema` parameter.
func JoinQuery(ctx context.Context, schema *Schema[any], where Where[any], joints ...XJoin) ([]JointPO[any], error) {
	panic("designing")
}

func JoinDelete() {
	panic("designing")
}

func JoinUpdate() {
	panic("designing")
}
