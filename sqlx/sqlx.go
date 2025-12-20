package sqlx

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kcmvp/dvo"
	"github.com/kcmvp/dvo/entity"
	"github.com/samber/lo"
)

// -----------------------------
// Public DSL (end-user API)
// -----------------------------

// Where is a generic, single-method interface representing a query condition.
// It is bound to a specific entity type T.
type Where[T entity.Entity] interface {
	// Build returns the SQL clause string and its corresponding arguments.
	Build() (string, []any)
}

// And combines multiple Where conditions with the AND operator.
// It filters out any nil or empty Where functions.
func And[T entity.Entity](wheres ...Where[T]) Where[T] {
	return and[T](wheres...)
}

// Or combines multiple Where conditions with the OR operator.
// It filters out any nil or empty Where functions.
func Or[T entity.Entity](wheres ...Where[T]) Where[T] {
	return or[T](wheres...)
}

func Eq[E entity.Entity](field entity.FieldProvider[E], value any) Where[E] {
	return op[E](field, "=", value)
}
func Ne[E entity.Entity](field entity.FieldProvider[E], value any) Where[E] {
	return op[E](field, "!=", value)
}
func Gt[E entity.Entity](field entity.FieldProvider[E], value any) Where[E] {
	return op[E](field, ">", value)
}
func Gte[E entity.Entity](field entity.FieldProvider[E], value any) Where[E] {
	return op[E](field, ">=", value)
}
func Lt[E entity.Entity](field entity.FieldProvider[E], value any) Where[E] {
	return op[E](field, "<", value)
}
func Lte[E entity.Entity](field entity.FieldProvider[E], value any) Where[E] {
	return op[E](field, "<=", value)
}
func Like[E entity.Entity](field entity.FieldProvider[E], value string) Where[E] {
	return op[E](field, "LIKE", value)
}

// In creates an "IN (... )" condition.
// It handles empty value slices gracefully by returning an always-false condition
// to prevent SQL syntax errors.
func In[E entity.Entity](field entity.FieldProvider[E], values ...any) Where[E] {
	return inWhere[E](field, values...)
}

// -----------------------------
// Public core API (end-user API)
// -----------------------------

// Schema is a type-safe, generic schema bound to a specific entity T.
// It embeds a universal dvo.Schema to remain compatible with core validation logic.
// The zero-length array `_ [0]T` is a common Go idiom to associate a generic type
// with a struct without incurring any memory overhead for a stored field.
type Schema[T entity.Entity] struct {
	*dvo.Schema
	providers []entity.FieldProvider[T]
	_         [0]T
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
		Schema:    universalSchema,
		providers: providers,
	}
}

// Query is a generic function that queries the database and returns a slice of T.
// It uses the provided schema to build the SELECT clause and the Where interface to build the WHERE clause.
func Query[T entity.Entity](ctx context.Context, schema *Schema[T], where Where[T]) ([]ValueObject[T], error) {
	// We'll build SQL here because we need to resolve actual DB column names at runtime
	if schema == nil || schema.Schema == nil {
		return nil, fmt.Errorf("schema is required")
	}

	var ent T
	table := ent.Table()

	db, ok := DefaultDS()
	if !ok || db == nil {
		return nil, fmt.Errorf("default datasource is not initialized")
	}

	// 1. get table columns from DB (SQLite PRAGMA)
	colsInDB := map[string]struct{}{}
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info('%s')", table))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var cid int
		var name string
		var ctype any
		var notnull any
		var dflt any
		var pk any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		colsInDB[name] = struct{}{}
	}

	// DEBUG: show cols in DB
	fmt.Println("[sqlx] colsInDB:")
	for c := range colsInDB {
		fmt.Println("  -", c)
	}

	// 2. map schema field names to actual DB columns using normalized matching
	fieldNames := make([]string, 0, len(schema.providers))
	fieldToDB := map[string]string{} // key: table.FieldName -> table.db_col
	for _, p := range schema.providers {
		col := p.AsSchemaField().Name()
		fieldNames = append(fieldNames, col)
		// find matching DB column by normalization
		var matched string
		normField := strings.ToLower(strings.ReplaceAll(col, "_", ""))
		for dbcol := range colsInDB {
			if strings.ToLower(strings.ReplaceAll(dbcol, "_", "")) == normField {
				matched = dbcol
				break
			}
		}
		if matched == "" {
			// fallback to snake_case(col)
			matched = lo.SnakeCase(col)
		}
		fieldToDB[fmt.Sprintf("%s.%s", table, col)] = fmt.Sprintf("%s.%s", table, matched)
	}

	// DEBUG: show mapping
	fmt.Println("[sqlx] fieldToDB mapping:")
	for k, v := range fieldToDB {
		fmt.Println("  ", k, "=>", v)
	}

	// 3. build SELECT clause using mapped DB column names and aliases
	cols := make([]string, 0, len(schema.providers))
	for _, col := range fieldNames {
		dbQualified := fieldToDB[fmt.Sprintf("%s.%s", table, col)]
		parts := strings.Split(dbQualified, ".")
		dbCol := parts[1]
		alias := fmt.Sprintf("%s__%s", table, col)
		cols = append(cols, fmt.Sprintf("%s.%s AS %s", table, dbCol, alias))
	}

	sql := fmt.Sprintf("SELECT %s FROM %s", strings.Join(cols, ", "), table)

	// DEBUG: print generated SQL for diagnostics
	fmt.Println("[sqlx] SQL =>", sql)

	var args []any
	if where != nil {
		clause, a := where.Build()
		if clause != "" {
			// rewrite clause to map schema-qualified names to DB-qualified names
			for k, v := range fieldToDB {
				clause = strings.ReplaceAll(clause, k, v)
			}
			sql += " WHERE " + clause
			args = a
		}
	}

	rows2, err := db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows2.Close() }()

	out := make([]ValueObject[T], 0)
	for rows2.Next() {
		vals := make([]any, len(fieldNames))
		ptrs := make([]any, len(fieldNames))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows2.Scan(ptrs...); err != nil {
			return nil, err
		}

		m := make(map[string]any, len(fieldNames))
		for i, name := range fieldNames {
			m[name] = dbValueToJSON(vals[i])
		}

		b, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		res := schema.Schema.Validate(string(b))
		if res.IsError() {
			return nil, res.Error()
		}
		out = append(out, ValueObject[T]{ValueObject: res.MustGet()})
	}
	if err := rows2.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

// Delete[T] is not yet implemented.
func Delete[T entity.Entity](ctx context.Context, where Where[T]) (sql.Result, error) {
	_ = ctx
	_ = where
	panic("implement me")
}

// Insert[T] is not yet implemented.
func Insert[T entity.Entity](ctx context.Context, po ValueObject[T]) (sql.Result, error) {
	_ = ctx
	_ = po
	panic("implement me")
}

// Update[T] is not yet implemented.
func Update[T entity.Entity](ctx context.Context, setter ValueObject[T], where Where[T]) (sql.Result, error) {
	_ = ctx
	_ = setter
	_ = where
	panic("implement me")
}

// JoinClause is the non-generic interface consumed by join SQL builders (selectJoinSQL).
//
// We keep this non-generic because Go slices cannot hold values of an uninstantiated generic interface.
// Public APIs can still return/accept the generic Joint[E1,E2], which implements JoinClause.
type JoinClause interface {
	Clause() string
}

// Joint represents a single SQL join clause between exactly 2 tables (E1 and E2).
//
// Notes:
//   - Clause() returns a full SQL join clause string, e.g.:
//     "INNER JOIN profiles ON (accounts.ID = profiles.AccountID AND accounts.TenantID = profiles.TenantID)"
//   - Joint supports joining on multiple columns via And(...), which always produces a parenthesized AND group.
//   - More complex boolean composition should be done by wrapping Clause() into Where (as documented in join.md).
//
// E1 is the driven/base table; E2 is the joined table.
type Joint[E1 entity.Entity, E2 entity.Entity] interface {
	JoinClause
	// And chains additional column-pair predicates into the ON condition using AND.
	// Each additional clause is expected to be another join predicate between E1 and E2.
	And(joints ...Joint[E1, E2]) Joint[E1, E2]
}

type joinMeta interface {
	joinTable2() string
	joinOnParts() []string
}

type join[E1 entity.Entity, E2 entity.Entity] struct {
	joinKeyword string
	table2      string
	onParts     []string
}

func (j join[E1, E2]) joinTable2() string    { return j.table2 }
func (j join[E1, E2]) joinOnParts() []string { return append([]string{}, j.onParts...) }

func (j join[E1, E2]) Clause() string {
	on := "(" + joinPartsWithAnd(j.onParts) + ")"
	return fmt.Sprintf("%s %s ON %s", j.joinKeyword, j.table2, on)
}

func (j join[E1, E2]) And(joints ...Joint[E1, E2]) Joint[E1, E2] {
	onParts := append([]string{}, j.onParts...)
	for _, jt := range joints {
		if jt == nil {
			continue
		}
		m, ok := jt.(joinMeta)
		if !ok {
			panic("sqlx: Joint.And() only supports joins created by sqlx.Join/sqlx.LeftJoin")
		}
		if m.joinTable2() != j.table2 {
			panic(fmt.Sprintf("sqlx: cannot And() joins with different right table: %s vs %s", j.table2, m.joinTable2()))
		}
		onParts = append(onParts, m.joinOnParts()...)
	}
	return join[E1, E2]{
		joinKeyword: j.joinKeyword,
		table2:      j.table2,
		onParts:     onParts,
	}
}

// Join creates an INNER JOIN clause with a single column equality predicate.
func Join[E1 entity.Entity, E2 entity.Entity](l entity.FieldProvider[E1], r entity.FieldProvider[E2]) Joint[E1, E2] {
	var e2 E2
	table2 := e2.Table()
	pred := fmt.Sprintf("%s = %s", l.QualifiedName(), r.QualifiedName())
	return join[E1, E2]{joinKeyword: "INNER JOIN", table2: table2, onParts: []string{pred}}
}

// LeftJoin creates a LEFT JOIN clause with a single column equality predicate.
func LeftJoin[E1 entity.Entity, E2 entity.Entity](l entity.FieldProvider[E1], r entity.FieldProvider[E2]) Joint[E1, E2] {
	var e2 E2
	table2 := e2.Table()
	pred := fmt.Sprintf("%s = %s", l.QualifiedName(), r.QualifiedName())
	return join[E1, E2]{joinKeyword: "LEFT JOIN", table2: table2, onParts: []string{pred}}
}

func joinPartsWithAnd(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += " AND " + parts[i]
	}
	return out
}

// dbValueToJSON normalizes common DB driver scan types into JSON-friendly Go values.
// - []byte / sql.RawBytes => string
// - sql.NullString, sql.NullInt64, sql.NullFloat64, sql.NullBool, sql.NullTime => nil or underlying value
// - time.Time and other native types are returned as-is
func dbValueToJSON(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(t)
	case sql.RawBytes:
		return string(t)
	case sql.NullString:
		if t.Valid {
			return t.String
		}
		return nil
	case sql.NullInt64:
		if t.Valid {
			return t.Int64
		}
		return nil
	case sql.NullFloat64:
		if t.Valid {
			return t.Float64
		}
		return nil
	case sql.NullBool:
		if t.Valid {
			return t.Bool
		}
		return nil
	case sql.NullTime:
		if t.Valid {
			return t.Time
		}
		return nil
	default:
		return t
	}
}
