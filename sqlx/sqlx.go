package sqlx

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/kcmvp/dvo/entity"
	"github.com/kcmvp/dvo/view"
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
	*view.Schema
	providers []entity.FieldProvider[T]
	_         [0]T
}

// ValueObject (Persistent Object) is a type-safe, generic wrapper around a universal dvo.ValueObject.
// It uses the zero-length array idiom to associate the entity type T at compile time
// without any memory cost, making it a lightweight, type-safe handle.
type ValueObject[T entity.Entity] struct {
	view.ValueObject
	_ [0]T
}

// NewSchema creates a new, type-safe, generic schema for a persistent object.
// It requires providers to be of the type entity.FieldProvider[T],
// guaranteeing at compile time that only fields belonging to entity T can be used.
func NewSchema[T entity.Entity](providers ...entity.FieldProvider[T]) *Schema[T] {
	// We need to convert the slice of entity.FieldProvider[T] back to []dvo.FieldProvider
	// for the universal WithFields function.
	dvoProviders := make([]view.FieldProvider, len(providers))
	for i, p := range providers {
		dvoProviders[i] = p
	}

	// 1. Create the universal Schema first using the core library function.
	universalSchema := view.WithFields(dvoProviders...)

	// 2. Wrap it in our generic Schema.
	return &Schema[T]{
		Schema:    universalSchema,
		providers: providers,
	}
}

// Query is a generic function that queries the database and returns a slice of T.
// It uses the provided schema to build the SELECT clause and the Where interface to build the WHERE clause.
func Query[T entity.Entity](ctx context.Context, schema *Schema[T], where Where[T]) ([]ValueObject[T], error) {
	if schema == nil || schema.Schema == nil {
		return nil, fmt.Errorf("schema is required")
	}

	var ent T
	table := ent.Table()

	db, ok := DefaultDS()
	if !ok || db == nil {
		return nil, fmt.Errorf("default datasource is not initialized")
	}

	// Helper to find `name:...` in `xql` tag for a given field name on the entity type.
	findXQLName := func(fieldName string) (string, bool) {
		rt := reflect.TypeOf(ent)
		if rt == nil {
			return "", false
		}
		if rt.Kind() == reflect.Ptr {
			rt = rt.Elem()
		}
		var walk func(reflect.Type) (string, bool)
		walk = func(t reflect.Type) (string, bool) {
			if t.Kind() != reflect.Struct {
				return "", false
			}
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				if f.Name == fieldName {
					tag := f.Tag.Get("xql")
					if tag != "" {
						parts := strings.Split(tag, ";")
						for _, p := range parts {
							p = strings.TrimSpace(p)
							if strings.HasPrefix(p, "name:") {
								return strings.TrimPrefix(p, "name:"), true
							}
						}
					}
					return "", false
				}
				if f.Anonymous {
					ft := f.Type
					if ft.Kind() == reflect.Ptr {
						ft = ft.Elem()
					}
					if name, ok := walk(ft); ok {
						return name, true
					}
				}
			}
			return "", false
		}
		return walk(rt)
	}

	// Map schema field names to DB columns: prefer `xql:"name:..."` tag when present, else snake_case
	fieldNames := make([]string, 0, len(schema.providers))
	fieldToDB := map[string]string{}
	for _, p := range schema.providers {
		col := p.AsSchemaField().Name()
		fieldNames = append(fieldNames, col)
		if tagName, ok := findXQLName(col); ok && tagName != "" {
			fieldToDB[fmt.Sprintf("%s.%s", table, col)] = fmt.Sprintf("%s.%s", table, tagName)
		} else {
			fieldToDB[fmt.Sprintf("%s.%s", table, col)] = fmt.Sprintf("%s.%s", table, lo.SnakeCase(col))
		}
	}

	// build SELECT clause using mapped DB column names and aliases
	cols := make([]string, 0, len(schema.providers))
	for _, col := range fieldNames {
		dbQualified := fieldToDB[fmt.Sprintf("%s.%s", table, col)]
		parts := strings.Split(dbQualified, ".")
		dbCol := parts[1]
		alias := fmt.Sprintf("%s__%s", table, col)
		cols = append(cols, fmt.Sprintf("%s.%s AS %s", table, dbCol, alias))
	}

	sql := fmt.Sprintf("SELECT %s FROM %s", strings.Join(cols, ", "), table)

	var args []any
	if where != nil {
		clause, a := where.Build()
		if clause != "" {
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

// convertQualifiedFieldsToDB helper to convert qualified CamelCase field names like `accounts.AccountID`
// to DB snake_case `accounts.account_id` before executing join SQL in QueryJoin; uses regexp and lo.SnakeCase.
func convertQualifiedFieldsToDB(sqlStr string) string {
	// Replace occurrences of <table>.<Field> with <table>.<snake_case(field)>
	re := regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\b`)
	return re.ReplaceAllStringFunc(sqlStr, func(m string) string {
		sm := re.FindStringSubmatch(m)
		if len(sm) < 3 {
			return m
		}
		table := sm[1]
		field := sm[2]
		return fmt.Sprintf("%s.%s", table, lo.SnakeCase(field))
	})
}

// QueryJoin executes a join SELECT and returns typed base-entity results ([]ValueObject[T]).
// - schema: the base entity schema (defines the projection and validation)
// - joins: list of JoinClause produced by Join/LeftJoin
// - where: a Where[entity.Entity] that may reference joined tables using qualified names
// Notes:
// - Projection is derived from `schema.providers` (only base table columns will be selected and validated).
// - This implementation does not collect bind-parameters embedded in JOIN ON clauses; ON should contain only column predicates.
func QueryJoin[T entity.Entity](ctx context.Context, schema *Schema[T], joins []JoinClause, where Where[entity.Entity]) ([]ValueObject[T], error) {
	if schema == nil || schema.Schema == nil {
		return nil, fmt.Errorf("schema is required")
	}

	var ent T
	baseTable := ent.Table()

	db, ok := DefaultDS()
	if !ok || db == nil {
		return nil, fmt.Errorf("default datasource is not initialized")
	}

	// Enforce two-table-only when explicitly requested via env var
	if os.Getenv("SQLX_ENFORCE_TWO_TABLE_JOIN") == "1" {
		targets := extractJoinTargetTables(joins)
		if len(targets) > 1 {
			return nil, fmt.Errorf("sqlx: only two-table joins are supported (detected targets: %v)", targets)
		}
	}

	// Build projection from schema.providers (these implement entity.JoinFieldProvider)
	projection := make([]entity.JoinFieldProvider, len(schema.providers))
	for i, p := range schema.providers {
		projection[i] = p
	}

	// Build SQL using the join SQL builder. Projection is the base table fields only.
	sqlStr, args, err := selectJoinSQL(baseTable, projection, joins, where)
	if err != nil {
		return nil, err
	}

	// Helper to find `name:...` in `xql` tag for a given field name on the entity type.
	findXQLName := func(fieldName string) (string, bool) {
		rt := reflect.TypeOf(ent)
		if rt == nil {
			return "", false
		}
		if rt.Kind() == reflect.Ptr {
			rt = rt.Elem()
		}
		var walk func(reflect.Type) (string, bool)
		walk = func(t reflect.Type) (string, bool) {
			if t.Kind() != reflect.Struct {
				return "", false
			}
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				if f.Name == fieldName {
					tag := f.Tag.Get("xql")
					if tag != "" {
						parts := strings.Split(tag, ";")
						for _, p := range parts {
							p = strings.TrimSpace(p)
							if strings.HasPrefix(p, "name:") {
								return strings.TrimPrefix(p, "name:"), true
							}
						}
					}
					return "", false
				}
				if f.Anonymous {
					ft := f.Type
					if ft.Kind() == reflect.Ptr {
						ft = ft.Elem()
					}
					if name, ok := walk(ft); ok {
						return name, true
					}
				}
			}
			return "", false
		}
		return walk(rt)
	}

	// Map schema field names to DB columns for base table: prefer xql tag name then snake_case
	fieldNames := make([]string, 0, len(schema.providers))
	fieldToDB := map[string]string{}
	for _, p := range schema.providers {
		col := p.AsSchemaField().Name()
		fieldNames = append(fieldNames, col)
		if tagName, ok := findXQLName(col); ok && tagName != "" {
			fieldToDB[fmt.Sprintf("%s.%s", baseTable, col)] = fmt.Sprintf("%s.%s", baseTable, tagName)
		} else {
			fieldToDB[fmt.Sprintf("%s.%s", baseTable, col)] = fmt.Sprintf("%s.%s", baseTable, lo.SnakeCase(col))
		}
	}

	// Replace base-qualified tokens (table.Field) with DB-qualified tokens (table.db_col)
	for k, v := range fieldToDB {
		sqlStr = strings.ReplaceAll(sqlStr, k, v)
	}

	// Convert any remaining qualified CamelCase tokens for other tables -> snake_case
	sqlStr = convertQualifiedFieldsToDB(sqlStr)

	// Execute
	rows, err := db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]ValueObject[T], 0)
	for rows.Next() {
		vals := make([]any, len(fieldNames))
		ptrs := make([]any, len(fieldNames))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
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
	if err := rows.Err(); err != nil {
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

// Count returns the number of rows matching the provided where condition for entity T.
// If schema is nil, do not require schema: rewrite qualified provider tokens
// <table>.<Field> by converting Field -> snake_case(Field) and execute COUNT(*).
func Count[T entity.Entity](ctx context.Context, schema *Schema[T], where Where[T]) (int64, error) {
	// Accept nil schema: only reject if non-nil but invalid
	if schema != nil && schema.Schema == nil {
		return 0, fmt.Errorf("invalid schema")
	}
	var ent T
	table := ent.Table()
	db, ok := DefaultDS()
	if !ok || db == nil {
		return 0, fmt.Errorf("default datasource is not initialized")
	}

	// Build field->db mapping deterministically if schema provided.
	var fieldToDB map[string]string
	if schema != nil {
		fieldToDB = map[string]string{}
		for _, p := range schema.providers {
			col := p.AsSchemaField().Name()
			dbCol := lo.SnakeCase(col)
			fieldToDB[fmt.Sprintf("%s.%s", table, col)] = fmt.Sprintf("%s.%s", table, dbCol)
		}
	}

	// build SQL
	sqlStr := fmt.Sprintf("SELECT COUNT(1) FROM %s", table)
	var args []any
	if where != nil {
		clause, a := where.Build()
		if clause != "" {
			if fieldToDB != nil {
				for k, v := range fieldToDB {
					clause = strings.ReplaceAll(clause, k, v)
				}
			} else {
				// rewrite occurrences of `<table>.<Field>` -> `<table>.<snake_case(Field)>` using regex
				re := regexp.MustCompile(`\b` + regexp.QuoteMeta(table) + `\.([A-Za-z_][A-ZaZ0-9_]*)\b`)
				clause = re.ReplaceAllStringFunc(clause, func(m string) string {
					sm := re.FindStringSubmatch(m)
					if len(sm) >= 2 {
						field := sm[1]
						return fmt.Sprintf("%s.%s", table, lo.SnakeCase(field))
					}
					return m
				})
			}
			sqlStr += " WHERE " + clause
			args = a
		}
	}

	// execute with QueryRow for COUNT
	rows, err := db.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	var cnt sql.NullInt64
	if rows.Next() {
		if err := rows.Scan(&cnt); err != nil {
			return 0, err
		}
		if cnt.Valid {
			return cnt.Int64, nil
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return 0, nil
}

func extractJoinTargetTables(joins []JoinClause) []string {
	set := map[string]struct{}{}
	re := regexp.MustCompile(`(?i)\b(INNER JOIN|LEFT JOIN|RIGHT JOIN|FULL JOIN)\s+([A-Za-z0-9_]+)`)
	for _, j := range joins {
		if j == nil {
			continue
		}
		c := j.Clause()
		matches := re.FindStringSubmatch(c)
		if len(matches) >= 3 {
			table := matches[2]
			set[table] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}
