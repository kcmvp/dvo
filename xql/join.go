package xql

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
// It assumes the second field's table is the one being joined.
func Join(f1, f2 entity.ViewFieldProvider) Joint {
	table2 := getTableFromField(f2)
	clause := fmt.Sprintf("INNER JOIN %s ON %s = %s", table2, f1.QualifiedName(), f2.QualifiedName())
	return &join{clause: clause}
}

// LeftJoin creates a LEFT JOIN clause between two fields.
// The order is significant: FROM f1's table LEFT JOIN f2's table.
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

// JoinQuery executes a multi-table JOIN query.
func JoinQuery(ctx context.Context, schema *Schema[entity.Entity], where Where[entity.Entity], joints ...Joint) ([]ValueObject[entity.Entity], error) {
	panic("use joints to generate all kinds of join between entities")
}

func JoinDelete(ctx context.Context, where Where[entity.Entity], joints ...Joint) (sql.Result, error) {
	panic("use joints to generate all kinds of join between entities")
}

func JoinUpdate(ctx context.Context, set ValueObject[entity.Entity], where Where[entity.Entity], joints ...Joint) (sql.Result, error) {
	panic("use joints to generate all kinds of join between entities")
}
