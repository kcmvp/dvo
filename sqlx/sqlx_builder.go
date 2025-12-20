package sqlx

import (
	"fmt"
	"strings"

	"github.com/kcmvp/dvo/entity"
	"github.com/samber/mo"
)

// -----------------------------
// Internal WHERE helpers
// -----------------------------

type whereFunc[T entity.Entity] func() (string, []any)

func (f whereFunc[T]) Build() (string, []any) { return f() }

func and[T entity.Entity](wheres ...Where[T]) Where[T] {
	f := func() (string, []any) {
		clauses := make([]string, 0, len(wheres))
		var allArgs []any
		for _, w := range wheres {
			if w == nil {
				continue
			}
			clause, args := w.Build()
			if clause == "" {
				continue
			}
			clauses = append(clauses, clause)
			allArgs = append(allArgs, args...)
		}
		if len(clauses) == 0 {
			return "", nil
		}
		return fmt.Sprintf("(%s)", strings.Join(clauses, " AND ")), allArgs
	}
	return whereFunc[T](f)
}

func or[T entity.Entity](wheres ...Where[T]) Where[T] {
	f := func() (string, []any) {
		clauses := make([]string, 0, len(wheres))
		var allArgs []any
		for _, w := range wheres {
			if w == nil {
				continue
			}
			clause, args := w.Build()
			if clause == "" {
				continue
			}
			clauses = append(clauses, clause)
			allArgs = append(allArgs, args...)
		}
		if len(clauses) == 0 {
			return "", nil
		}
		return fmt.Sprintf("(%s)", strings.Join(clauses, " OR ")), allArgs
	}
	return whereFunc[T](f)
}

func dbQualifiedNameFromQName(q string) string {
	parts := strings.Split(q, ".")
	if len(parts) != 2 {
		return q
	}
	table := parts[0]
	col := parts[1]
	// Use the column name exactly as provided by the FieldProvider/Schema.
	return fmt.Sprintf("%s.%s", table, col)
}

// makePlaceholders returns a comma-separated list of '?' placeholders for SQL IN clauses.
func makePlaceholders(n int) string {
	if n <= 0 {
		return ""
	}
	ps := make([]string, n)
	for i := 0; i < n; i++ {
		ps[i] = "?"
	}
	return strings.Join(ps, ",")
}

func op[E entity.Entity](field entity.FieldProvider[E], operator string, value any) Where[E] {
	f := func() (string, []any) {
		clause := fmt.Sprintf("%s %s ?", dbQualifiedNameFromQName(field.QualifiedName()), operator)
		return clause, []any{value}
	}
	return whereFunc[E](f)
}

// inWhere is used by the public In() API in sqlx.go.
func inWhere[E entity.Entity](field entity.FieldProvider[E], values ...any) Where[E] {
	if len(values) == 0 {
		return whereFunc[E](func() (string, []any) { return "1=0", nil })
	}
	placeholders := makePlaceholders(len(values))
	clause := fmt.Sprintf("%s IN (%s)", dbQualifiedNameFromQName(field.QualifiedName()), placeholders)
	return whereFunc[E](func() (string, []any) { return clause, values })
}

// -----------------------------
// Internal SELECT builder
// -----------------------------

func selectSQL[T entity.Entity](schema *Schema[T], where Where[T]) (string, []any, error) {
	if schema == nil || schema.Schema == nil {
		return "", nil, fmt.Errorf("schema is required")
	}
	if len(schema.providers) == 0 {
		return "", nil, fmt.Errorf("schema has no fields")
	}

	var ent T
	table := ent.Table()

	cols := make([]string, 0, len(schema.providers))
	for _, p := range schema.providers {
		col := p.AsSchemaField().Name()
		// Use the provided field name as DB column name (tests expect this behavior).
		dbCol := col
		alias := fmt.Sprintf("%s__%s", table, col)
		cols = append(cols, fmt.Sprintf("%s.%s AS %s", table, dbCol, alias))
	}

	sql := fmt.Sprintf("SELECT %s FROM %s", strings.Join(cols, ", "), table)
	if where == nil {
		return sql, nil, nil
	}
	clause, args := where.Build()
	if clause == "" {
		return sql, nil, nil
	}
	return sql + " WHERE " + clause, args, nil
}

// -----------------------------
// Internal CRUD builders
// -----------------------------

type getter interface {
	Get(string) mo.Option[any]
}

func insertSQL[T entity.Entity](schema *Schema[T], g getter) (string, []any, error) {
	if schema == nil || schema.Schema == nil {
		return "", nil, fmt.Errorf("schema is required")
	}
	if g == nil {
		return "", nil, fmt.Errorf("value is required")
	}

	var ent T
	table := ent.Table()

	cols := make([]string, 0, len(schema.providers))
	ph := make([]string, 0, len(schema.providers))
	args := make([]any, 0, len(schema.providers))

	for _, p := range schema.providers {
		col := p.AsSchemaField().Name()
		vOpt := g.Get(col)
		if vOpt.IsAbsent() {
			continue
		}
		// Use provided column name as DB column name.
		dbCol := col
		cols = append(cols, dbCol)
		ph = append(ph, "?")
		args = append(args, vOpt.MustGet())
	}

	if len(cols) == 0 {
		return "", nil, fmt.Errorf("no fields to insert")
	}

	sql := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(cols, ", "),
		strings.Join(ph, ", "),
	)
	return sql, args, nil
}

func updateSQL[T entity.Entity](schema *Schema[T], g getter, where Where[T]) (string, []any, error) {
	if schema == nil || schema.Schema == nil {
		return "", nil, fmt.Errorf("schema is required")
	}
	if g == nil {
		return "", nil, fmt.Errorf("setter is required")
	}
	if where == nil {
		return "", nil, fmt.Errorf("where is required")
	}
	whereClause, whereArgs := where.Build()
	if whereClause == "" {
		return "", nil, fmt.Errorf("where is required")
	}

	var ent T
	table := ent.Table()

	sets := make([]string, 0, len(schema.providers))
	args := make([]any, 0, len(schema.providers)+len(whereArgs))

	for _, p := range schema.providers {
		col := p.AsSchemaField().Name()
		vOpt := g.Get(col)
		if vOpt.IsAbsent() {
			continue
		}
		// use provided column name in SET clause
		dbCol := col
		sets = append(sets, fmt.Sprintf("%s.%s = ?", table, dbCol))
		args = append(args, vOpt.MustGet())
	}

	if len(sets) == 0 {
		return "", nil, fmt.Errorf("no fields to update")
	}

	sql := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s",
		table,
		strings.Join(sets, ", "),
		whereClause,
	)
	args = append(args, whereArgs...)
	return sql, args, nil
}

func deleteSQL[T entity.Entity](where Where[T]) (string, []any, error) {
	if where == nil {
		return "", nil, fmt.Errorf("where is required")
	}
	clause, args := where.Build()
	if clause == "" {
		return "", nil, fmt.Errorf("where is required")
	}

	var ent T
	table := ent.Table()
	return fmt.Sprintf("DELETE FROM %s WHERE %s", table, clause), args, nil
}

// -----------------------------
// Internal JOIN predicate helpers
// -----------------------------

// (removed) joinExistsWhere / joinViewWhere: legacy helpers for the old JointNs/JoinPred design.

// whereFromJoinE1 converts a join clause (INNER/LEFT JOIN ... ON (...)) into a Where[E1]
// by translating it into an EXISTS subquery. This allows join logic to be composed with
// Where.And/Or.
func whereFromJoinE1[E1 entity.Entity, E2 entity.Entity](j Joint[E1, E2]) Where[E1] {
	return whereFunc[E1](func() (string, []any) {
		if j == nil {
			return "", nil
		}
		pred, err := joinClauseToExistsPredicate[E1, E2](j)
		if err != nil {
			return "", nil
		}
		return fmt.Sprintf("EXISTS (SELECT 1 FROM %s WHERE %s)", joinRelatedTable[E2](), pred), nil
	})
}

// whereFromJoinOn converts a join clause into a Where[entity.Entity] using its ON predicate
// as a parenthesized AND-group: "(<p1> AND <p2> ...)".
//
// Notes:
//   - This helper is intentionally args-free (returns nil args).
//   - It is designed to be composed via Where.And/Or.
func whereFromJoinOn[E1 entity.Entity, E2 entity.Entity](j Joint[E1, E2]) Where[entity.Entity] {
	return whereFunc[entity.Entity](func() (string, []any) {
		if j == nil {
			return "", nil
		}

		pred, err := joinClauseToExistsPredicate[E1, E2](j)
		if err != nil {
			return "", nil
		}
		return pred, nil
	})
}

// joinClauseToExistsPredicate converts a join clause into a predicate string that can be placed
// inside the WHERE of an EXISTS subquery.
//
// It rewrites table names so that:
//   - E1 side always uses E1.Table()
//   - E2 side uses E2.Table() or an alias in case of self-join
func joinClauseToExistsPredicate[E1 entity.Entity, E2 entity.Entity](j Joint[E1, E2]) (string, error) {
	if j == nil {
		return "", fmt.Errorf("join is nil")
	}

	m, ok := any(j).(interface{ joinOnParts() []string })
	if !ok {
		return "", fmt.Errorf("unsupported join implementation")
	}
	onParts := m.joinOnParts()
	if len(onParts) == 0 {
		return "", fmt.Errorf("empty join")
	}

	var e1 E1
	var e2 E2
	baseTable := e1.Table()
	relatedTable := e2.Table()

	innerAlias := relatedTable
	if baseTable == relatedTable {
		innerAlias = relatedTable + "_2"
	}

	conds := make([]string, 0, len(onParts))
	for _, p := range onParts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		parts := strings.Split(p, "=")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid join predicate: %s", p)
		}
		lhs := strings.TrimSpace(parts[0])
		rhs := strings.TrimSpace(parts[1])

		l := strings.Split(lhs, ".")
		r := strings.Split(rhs, ".")
		if len(l) != 2 || len(r) != 2 {
			return "", fmt.Errorf("invalid qualified name in join predicate: %s", p)
		}
		// use provided column names directly
		lcol := l[1]
		rcol := r[1]
		conds = append(conds, fmt.Sprintf("%s.%s = %s.%s", baseTable, lcol, innerAlias, rcol))
	}
	if len(conds) == 0 {
		return "", fmt.Errorf("empty join")
	}

	// Keep this as a parenthesized AND-group so it can be composed by where.And/Or.
	return fmt.Sprintf("(%s)", strings.Join(conds, " AND ")), nil
}

// joinRelatedTable returns E2.Table(). It is a helper to keep generic table resolution in one place.
func joinRelatedTable[E2 entity.Entity]() string {
	var e2 E2
	return e2.Table()
}

// -----------------------------
// Internal JOIN SELECT builder
// -----------------------------

// selectJoinSQL builds a SELECT statement across multiple tables.
//
// It is a lower-level builder used by join-related APIs/tests.
//
// Contract:
//   - projection order is preserved exactly as passed in.
//   - each projected column must be qualified as <table>.<col>.
//   - every projected column is aliased as <table>__<col>.
//   - JOIN clauses are appended in the given order.
func selectJoinSQL(baseTable string, projection []entity.JoinFieldProvider, joins []JoinClause, where Where[entity.Entity]) (string, []any, error) {
	if baseTable == "" {
		return "", nil, fmt.Errorf("base table is required")
	}
	if len(projection) == 0 {
		return "", nil, fmt.Errorf("projection is required")
	}

	cols := make([]string, 0, len(projection))
	for _, p := range projection {
		q := p.QualifiedName()
		parts := strings.Split(q, ".")
		if len(parts) != 2 {
			return "", nil, fmt.Errorf("invalid qualified field name: %s", q)
		}
		table := parts[0]
		col := parts[1]
		// Use provided column name as DB column name (match snapshots).
		dbCol := col
		alias := fmt.Sprintf("%s__%s", table, col)
		cols = append(cols, fmt.Sprintf("%s.%s AS %s", table, dbCol, alias))
	}

	sql := fmt.Sprintf("SELECT %s FROM %s", strings.Join(cols, ", "), baseTable)
	for _, j := range joins {
		if j == nil {
			continue
		}
		c := strings.TrimSpace(j.Clause())
		if c == "" {
			continue
		}
		sql += " " + c
	}

	if where == nil {
		return sql, nil, nil
	}
	clause, args := where.Build()
	if clause == "" {
		return sql, nil, nil
	}
	return sql + " WHERE " + clause, args, nil
}
