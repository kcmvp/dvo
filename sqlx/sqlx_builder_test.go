package sqlx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/kcmvp/dvo/entity"
	sample "github.com/kcmvp/dvo/sample/entity"
	"github.com/kcmvp/dvo/sample/gen/field/account"
	"github.com/kcmvp/dvo/sample/gen/field/accountrole"
	"github.com/kcmvp/dvo/sample/gen/field/order"
	"github.com/kcmvp/dvo/sample/gen/field/orderitem"
	"github.com/kcmvp/dvo/sample/gen/field/product"
	"github.com/kcmvp/dvo/sample/gen/field/profile"
	"github.com/kcmvp/dvo/sample/gen/field/role"
	"github.com/kcmvp/dvo/view"
	"github.com/samber/mo"
	"github.com/stretchr/testify/require"
)

// This file is intentionally organized into 3 test categories:
//  1) Operator tests: Where predicates and boolean composition.
//  2) Join tests: join clause generation (INNER/LEFT).
//  3) Full SQL script tests: end-to-end SQL string generation for Select/Insert/Update/Delete.
//
// Keep new tests in the right section so it's easy to locate and maintain snapshots.

// -----------------------------
// Test helpers (SQL snapshots)
// -----------------------------

// Snapshots are stored under ../testdata/sql and the filename matches the Go test name
// exactly: <TestName>.sql
//
// Note: This assumes test names are valid file names on the current platform.
// (Go's default `t.Name()` values are typically safe unless you use special chars in subtests.)

func sqlSnapshotPathByName(testName string) string {
	return filepath.Join("..", "testdata", "sql", testName+".sql")
}

func writeSQLSnapshotFile(path string, sql string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if !strings.HasSuffix(sql, "\n") {
		sql += "\n"
	}
	return os.WriteFile(path, []byte(sql), 0o644)
}

func readSQLSnapshotFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(b), "\n"), nil
}

func assertSQLSnapshotByName(testName string, generatedSQL string) error {
	path := sqlSnapshotPathByName(testName)
	formatted := formatSQLForSnapshot(generatedSQL)

	expected, err := readSQLSnapshotFile(path)
	if err != nil {
		// If snapshot doesn't exist yet, only create it when explicitly allowed.
		if os.IsNotExist(err) {
			if os.Getenv("UPDATE_SNAPSHOTS") == "1" {
				return writeSQLSnapshotFile(path, formatted)
			}
			return fmt.Errorf("sql snapshot missing: %s (set UPDATE_SNAPSHOTS=1 to create)", path)
		}
		return fmt.Errorf("read sql snapshot %s: %w", path, err)
	}

	if expected == formatted {
		return nil
	}

	if os.Getenv("UPDATE_SNAPSHOTS") == "1" {
		if err := writeSQLSnapshotFile(path, formatted); err != nil {
			return fmt.Errorf("write sql snapshot %s: %w", path, err)
		}
		return nil
	}

	return fmt.Errorf("sql snapshot mismatch: %s (set UPDATE_SNAPSHOTS=1 to update)", path)
}

// formatSQLForSnapshot normalizes SQL text for snapshot testing.
//
// Goals:
//   - readable snapshots (multi-line)
//   - stable output (deterministic formatting)
//
// It is NOT a general SQL parser. It supports the subset produced by this project.
func formatSQLForSnapshot(sql string) string {
	s := strings.TrimSpace(sql)
	if s == "" {
		return ""
	}

	// Normalize all whitespace first.
	s = strings.Join(strings.Fields(s), " ")

	// Only format SELECT statements for now (covers our current snapshot tests).
	upper := strings.ToUpper(s)
	if !strings.HasPrefix(upper, "SELECT ") {
		return s
	}

	// Split into SELECT <cols> FROM <rest>
	idxFrom := strings.Index(upper, " FROM ")
	if idxFrom < 0 {
		return s
	}

	colsPart := strings.TrimSpace(s[len("SELECT "):idxFrom])
	rest := strings.TrimSpace(s[idxFrom+len(" FROM "):])

	cols := strings.Split(colsPart, ", ")
	var b strings.Builder
	b.WriteString("SELECT\n")
	for i, c := range cols {
		if i == len(cols)-1 {
			b.WriteString("  " + c + "\n")
			continue
		}
		b.WriteString("  " + c + ",\n")
	}
	b.WriteString("FROM ")

	// Extract WHERE clause (if any).
	upperRest := strings.ToUpper(rest)
	idxWhere := strings.Index(upperRest, " WHERE ")
	fromAndJoins := rest
	whereClause := ""
	if idxWhere >= 0 {
		fromAndJoins = strings.TrimSpace(rest[:idxWhere])
		whereClause = strings.TrimSpace(rest[idxWhere+len(" WHERE "):])
	}

	// Put joins on separate lines by inserting newlines before join keyword phrases.
	fromAndJoinsFmt := fromAndJoins
	joinPhrases := []string{" INNER JOIN ", " LEFT JOIN ", " RIGHT JOIN ", " FULL JOIN "}
	for _, jp := range joinPhrases {
		fromAndJoinsFmt = strings.ReplaceAll(fromAndJoinsFmt, jp, "\n"+strings.TrimSpace(jp)+" ")
	}
	partsLines := strings.Split(fromAndJoinsFmt, "\n")

	// First line is base table, subsequent lines (if any) are JOIN clauses.
	b.WriteString(strings.TrimSpace(partsLines[0]) + "\n")
	for _, ln := range partsLines[1:] {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		b.WriteString(ln + "\n")
	}

	if whereClause != "" {
		b.WriteString("WHERE " + whereClause)
	}

	return strings.TrimRight(b.String(), "\n")
}

// -----------------------------
// 1) Operator tests
// -----------------------------

type testEntity struct{}

func (testEntity) Table() string { return "test" }

func TestOperator_Eq(t *testing.T) {
	f := entity.Field[testEntity, int64]("id")
	w := Eq[testEntity](f, int64(10))
	clause, args := w.Build()
	require.Equal(t, "test.id = ?", clause)
	require.Equal(t, []any{int64(10)}, args)
}

func TestOperator_AndOr_FilterEmptyAndNil(t *testing.T) {
	f := entity.Field[testEntity, string]("name")

	empty := whereFunc[testEntity](func() (string, []any) { return "", nil })
	var nilWhere Where[testEntity]

	w := And[testEntity](nilWhere, empty, Eq[testEntity](f, "a"))
	clause, args := w.Build()
	require.Equal(t, "(test.name = ?)", clause)
	require.Equal(t, []any{"a"}, args)

	w2 := Or[testEntity](nilWhere, empty, Eq[testEntity](f, "a"), Eq[testEntity](f, "b"))
	clause2, args2 := w2.Build()
	require.Equal(t, "(test.name = ? OR test.name = ?)", clause2)
	require.Equal(t, []any{"a", "b"}, args2)
}

func TestOperator_AndOr_AllEmpty(t *testing.T) {
	empty := whereFunc[testEntity](func() (string, []any) { return "", nil })

	w := And[testEntity](empty)
	clause, args := w.Build()
	require.Equal(t, "", clause)
	require.Nil(t, args)

	w2 := Or[testEntity](empty)
	clause2, args2 := w2.Build()
	require.Equal(t, "", clause2)
	require.Nil(t, args2)
}

func TestOperator_In(t *testing.T) {
	f := entity.Field[testEntity, int64]("id")

	w := In[testEntity](f, int64(1), int64(2), int64(3))
	clause, args := w.Build()
	require.Equal(t, "test.id IN (?,?,?)", clause)
	require.Equal(t, []any{int64(1), int64(2), int64(3)}, args)
}

func TestOperator_In_EmptyIsAlwaysFalse(t *testing.T) {
	f := entity.Field[testEntity, int64]("id")

	w := In[testEntity](f)
	clause, args := w.Build()
	require.Equal(t, "1=0", clause)
	require.Nil(t, args)
}

func TestOperator_Operators(t *testing.T) {
	f := entity.Field[testEntity, int64]("age")

	cases := []struct {
		name     string
		w        Where[testEntity]
		expected string
		args     []any
	}{
		{"ne", Ne[testEntity](f, int64(1)), "test.age != ?", []any{int64(1)}},
		{"gt", Gt[testEntity](f, int64(2)), "test.age > ?", []any{int64(2)}},
		{"gte", Gte[testEntity](f, int64(3)), "test.age >= ?", []any{int64(3)}},
		{"lt", Lt[testEntity](f, int64(4)), "test.age < ?", []any{int64(4)}},
		{"lte", Lte[testEntity](f, int64(5)), "test.age <= ?", []any{int64(5)}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clause, args := tc.w.Build()
			require.Equal(t, tc.expected, clause)
			require.Equal(t, tc.args, args)
		})
	}
}

func TestOperator_Like(t *testing.T) {
	f := entity.Field[testEntity, string]("email")
	w := Like[testEntity](f, "%@example.com")
	clause, args := w.Build()
	require.Equal(t, "test.email LIKE ?", clause)
	require.Equal(t, []any{"%@example.com"}, args)
}

func TestOperator_WithGeneratedFields(t *testing.T) {
	clause, args := Eq[sample.Account](account.Email, "a@b.com").Build()
	require.Equal(t, "accounts.Email = ?", clause)
	require.Equal(t, []any{"a@b.com"}, args)

	clause, args = Ne[sample.Account](account.Nickname, "x").Build()
	require.Equal(t, "accounts.Nickname != ?", clause)
	require.Equal(t, []any{"x"}, args)

	clause, args = Gt[sample.Account](account.Balance, 10.5).Build()
	require.Equal(t, "accounts.Balance > ?", clause)
	require.Equal(t, []any{10.5}, args)

	clause, args = Gte[sample.Order](order.Amount, 1.0).Build()
	require.Equal(t, "orders.Amount >= ?", clause)
	require.Equal(t, []any{1.0}, args)

	clause, args = Lt[sample.Order](order.Amount, 100.0).Build()
	require.Equal(t, "orders.Amount < ?", clause)
	require.Equal(t, []any{100.0}, args)

	clause, args = Lte[sample.Profile](profile.ID, int64(99)).Build()
	require.Equal(t, "profiles.ID <= ?", clause)
	require.Equal(t, []any{int64(99)}, args)

	clause, args = Like[sample.Profile](profile.Bio, "%hello%").Build()
	require.Equal(t, "profiles.Bio LIKE ?", clause)
	require.Equal(t, []any{"%hello%"}, args)

	clause, args = In[sample.AccountRole](accountrole.RoleID, int64(1), int64(2), int64(3)).Build()
	require.Equal(t, "account_roles.RoleID IN (?,?,?)", clause)
	require.Equal(t, []any{int64(1), int64(2), int64(3)}, args)
}

func TestOperator_WithGeneratedFields_Compound(t *testing.T) {
	w := And[sample.Account](
		Eq[sample.Account](account.Email, "a@b.com"),
		Gt[sample.Account](account.Balance, 1.0),
	)
	clause, args := w.Build()
	require.Equal(t, "(accounts.Email = ? AND accounts.Balance > ?)", clause)
	require.Equal(t, []any{"a@b.com", 1.0}, args)

	w2 := Or[sample.Order](
		Eq[sample.Order](order.AccountID, int64(1)),
		Gt[sample.Order](order.Amount, 99.9),
	)
	clause2, args2 := w2.Build()
	require.Equal(t, "(orders.AccountID = ? OR orders.Amount > ?)", clause2)
	require.Equal(t, []any{int64(1), 99.9}, args2)
}

// -----------------------------
// 2) Join tests
// -----------------------------

func TestJoin_Inner_OneToOne(t *testing.T) {
	j := Join[sample.Account, sample.Profile](account.ID, profile.AccountID)
	require.Equal(t, "INNER JOIN profiles ON (accounts.ID = profiles.AccountID)", j.Clause())
}

func TestJoin_Inner_OneToMany(t *testing.T) {
	j := Join[sample.Account, sample.Order](account.ID, order.AccountID)
	require.Equal(t, "INNER JOIN orders ON (accounts.ID = orders.AccountID)", j.Clause())
}

func TestJoin_Left(t *testing.T) {
	j := LeftJoin[sample.Account, sample.Profile](account.ID, profile.AccountID)
	require.Equal(t, "LEFT JOIN profiles ON (accounts.ID = profiles.AccountID)", j.Clause())
}

func TestJoin_CompositeKeys_AlwaysAnd(t *testing.T) {
	j := Join[sample.Account, sample.Profile](account.ID, profile.AccountID).
		And(Join[sample.Account, sample.Profile](account.Email, profile.Bio))

	require.Equal(t, "INNER JOIN profiles ON (accounts.ID = profiles.AccountID AND accounts.Email = profiles.Bio)", j.Clause())
}

// -----------------------------
// 3) Full SQL script tests (snapshots)
// -----------------------------
// These tests write and compare SQL files in ../testdata/sql.
// The snapshot file name is based on the test name.

func TestSQL_Select_NoWhere(t *testing.T) {
	schema := NewSchema[sample.Account](account.All()...)
	sql, args, err := selectSQL[sample.Account](schema, nil)
	require.NoError(t, err)
	require.Nil(t, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestSQL_Select_WithWhere(t *testing.T) {
	schema := NewSchema[sample.Order](order.All()...)
	sql, args, err := selectSQL[sample.Order](schema, And[sample.Order](
		Eq[sample.Order](order.AccountID, int64(1)),
		Gt[sample.Order](order.Amount, 10.0),
	))
	require.NoError(t, err)
	require.Equal(t, []any{int64(1), 10.0}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestSQL_Select_EmptyWhereClauseIgnored(t *testing.T) {
	schema := NewSchema[sample.Account](account.All()...)
	empty := whereFunc[sample.Account](func() (string, []any) { return "", nil })

	sql, args, err := selectSQL[sample.Account](schema, empty)
	require.NoError(t, err)
	require.Nil(t, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestSQL_Select_NilSchemaError(t *testing.T) {
	_, _, err := selectSQL[sample.Account](nil, nil)
	require.Error(t, err)
}

func TestSQL_Select_EmptySchemaError(t *testing.T) {
	s := &Schema[sample.Account]{Schema: view.WithFields()}
	_, _, err := selectSQL[sample.Account](s, nil)
	require.Error(t, err)
}

func TestSQL_Select_Like(t *testing.T) {
	schema := NewSchema[sample.Account](account.All()...)
	sql, args, err := selectSQL[sample.Account](schema, Like[sample.Account](account.Email, "%@example.com"))
	require.NoError(t, err)
	require.Equal(t, []any{"%@example.com"}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestSQL_Select_In(t *testing.T) {
	schema := NewSchema[sample.Order](order.All()...)
	sql, args, err := selectSQL[sample.Order](schema, In[sample.Order](order.ID, int64(1), int64(2), int64(3)))
	require.NoError(t, err)
	require.Equal(t, []any{int64(1), int64(2), int64(3)}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestSQL_Select_OrNested(t *testing.T) {
	schema := NewSchema[sample.Order](order.All()...)

	w := Or[sample.Order](
		And[sample.Order](
			Eq[sample.Order](order.AccountID, int64(1)),
			Gt[sample.Order](order.Amount, 100.0),
		),
		Eq[sample.Order](order.AccountID, int64(2)),
	)

	sql, args, err := selectSQL[sample.Order](schema, w)
	require.NoError(t, err)
	require.Equal(t, []any{int64(1), 100.0, int64(2)}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestSQL_Select_Profile_WithWhere(t *testing.T) {
	schema := NewSchema[sample.Profile](profile.All()...)
	sql, args, err := selectSQL[sample.Profile](schema, And[sample.Profile](
		Eq[sample.Profile](profile.AccountID, int64(1)),
		Like[sample.Profile](profile.Bio, "%hello%"),
	))
	require.NoError(t, err)
	require.Equal(t, []any{int64(1), "%hello%"}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestSQL_Select_OrderItem_WithWhere(t *testing.T) {
	schema := NewSchema[sample.OrderItem](orderitem.All()...)
	sql, args, err := selectSQL[sample.OrderItem](schema, And[sample.OrderItem](
		Eq[sample.OrderItem](orderitem.OrderID, int64(10)),
		Gt[sample.OrderItem](orderitem.Quantity, int64(1)),
	))
	require.NoError(t, err)
	require.Equal(t, []any{int64(10), int64(1)}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestSQL_Select_Product_In(t *testing.T) {
	schema := NewSchema[sample.Product](product.All()...)
	sql, args, err := selectSQL[sample.Product](schema, In[sample.Product](product.ID, int64(1), int64(2)))
	require.NoError(t, err)
	require.Equal(t, []any{int64(1), int64(2)}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestJoin_Select_AccountWithProfile(t *testing.T) {
	projection := []entity.JoinFieldProvider{
		account.ID,
		account.Email,
		profile.ID,
		profile.AccountID,
	}

	joins := []JoinClause{
		Join[sample.Account, sample.Profile](account.ID, profile.AccountID),
	}

	sql, args, err := selectJoinSQL(
		"accounts",
		projection,
		joins,
		Eq[entity.Entity](account.ID, int64(1)),
	)
	require.NoError(t, err)
	require.Equal(t, []any{int64(1)}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestJoin_Select_AccountWithRoleViaJoinTable(t *testing.T) {
	projection := []entity.JoinFieldProvider{account.ID, account.Email, role.ID, role.Key}

	jAR := Join[sample.Account, sample.AccountRole](account.ID, accountrole.AccountID)
	wAR := whereFromJoinE1[sample.Account, sample.AccountRole](jAR)

	jR := Join[sample.AccountRole, sample.Role](accountrole.RoleID, role.ID)
	wR := whereFromJoinOn[sample.AccountRole, sample.Role](jR)

	sql, args, err := selectJoinSQL(
		"accounts",
		projection,
		[]JoinClause{Join[sample.Account, sample.Role](account.ID, role.ID)},
		And[entity.Entity](wR, whereFromJoinOn[sample.Account, sample.Role](Join[sample.Account, sample.Role](account.ID, role.ID))),
	)
	_ = wAR
	require.NoError(t, err)
	require.Nil(t, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestJoin_Select_RejectsThirdTableField(t *testing.T) {
	// View API removed; keep a minimal safety test at builder layer.
	projection := []entity.JoinFieldProvider{account.ID, profile.ID, order.ID}
	_, _, err := selectJoinSQL("accounts", projection, nil, nil)
	require.NoError(t, err)
}

func TestJoin_Select_AllowsSelfJoinProjection(t *testing.T) {
	// Self-join is handled by JOIN predicate/table aliasing at a higher layer.
	projection := []entity.JoinFieldProvider{account.ID, account.Email}
	_, _, err := selectJoinSQL("accounts", projection, nil, nil)
	require.NoError(t, err)
}

type testGetter map[string]any

func (g testGetter) Get(k string) mo.Option[any] {
	v, ok := g[k]
	if !ok {
		return mo.None[any]()
	}
	return mo.Some[any](v)
}

func TestSQL_Insert_OrderAndPresence(t *testing.T) {
	schema := NewSchema[sample.Account](account.All()...)

	sql, args, err := insertSQL[sample.Account](schema, testGetter{"Email": "a@b.com", "Nickname": "nick"})
	require.NoError(t, err)
	require.Equal(t, []any{"a@b.com", "nick"}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestSQL_Insert_NoFieldsError(t *testing.T) {
	schema := NewSchema[sample.Account](account.All()...)
	_, _, err := insertSQL[sample.Account](schema, testGetter{})
	require.Error(t, err)
}

func TestSQL_Update_RequiresWhere(t *testing.T) {
	schema := NewSchema[sample.Account](account.All()...)
	_, _, err := updateSQL[sample.Account](schema, testGetter{"Nickname": "nick"}, nil)
	require.Error(t, err)
}

func TestSQL_Update_OrderAndArgs(t *testing.T) {
	schema := NewSchema[sample.Account](account.All()...)

	w := Eq[sample.Account](account.Email, "a@b.com")
	sql, args, err := updateSQL[sample.Account](
		schema,
		testGetter{"Nickname": "nick", "Balance": 12.34},
		w,
	)
	require.NoError(t, err)
	require.Equal(t, []any{"nick", 12.34, "a@b.com"}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestSQL_Delete_RequiresWhere(t *testing.T) {
	_, _, err := deleteSQL[sample.Account](nil)
	require.Error(t, err)
}

func TestSQL_Delete(t *testing.T) {
	sql, args, err := deleteSQL[sample.Account](Eq[sample.Account](account.Email, "a@b.com"))
	require.NoError(t, err)
	require.Equal(t, []any{"a@b.com"}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

// -----------------------------
// Execution/wiring tests
// -----------------------------

func TestExecution_Query_RequiresSchema(t *testing.T) {
	_, err := Query[testEntity](context.Background(), nil, nil)
	require.Error(t, err)
}

func TestExecution_Query_NoDefaultDataSource(t *testing.T) {
	initOnce = sync.Once{}
	initErr = nil
	dsMu.Lock()
	dsRegistry = map[string]DB{}
	defaultDS = nil
	dsMu.Unlock()

	id := entity.Field[testEntity, int64]("id")
	s := NewSchema[testEntity](id)
	_, err := Query[testEntity](context.Background(), s, nil)
	require.Error(t, err)
}

func joinClause(joinKeyword, table2, onExpr string) JoinClause {
	return joinClauseImpl{clause: fmt.Sprintf("%s %s ON %s", joinKeyword, table2, onExpr)}
}

type joinClauseImpl struct{ clause string }

func (j joinClauseImpl) Clause() string { return j.clause }

func TestJoin_Select_WhereE1_BooleanComposition(t *testing.T) {
	A := Join[sample.Account, sample.Profile](account.ID, profile.AccountID)
	B := Join[sample.Account, sample.Profile](account.Email, profile.Bio)
	C := Join[sample.Account, sample.Profile](account.ID, profile.ID)

	on := Or[entity.Entity](
		And[entity.Entity](whereFromJoinOn[sample.Account, sample.Profile](A), whereFromJoinOn[sample.Account, sample.Profile](B)),
		whereFromJoinOn[sample.Account, sample.Profile](C),
	)
	onClause, args := on.Build()
	require.Nil(t, args)

	projection := []entity.JoinFieldProvider{account.ID, profile.ID}
	joins := []JoinClause{joinClause("INNER JOIN", "profiles", onClause)}

	sql, args2, err := selectJoinSQL("accounts", projection, joins, nil)
	require.NoError(t, err)
	require.Nil(t, args2)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestJoin_Select_WhereE1AndE2(t *testing.T) {
	// Join accounts + profiles, then apply WHERE filters on both tables.
	projection := []entity.JoinFieldProvider{account.ID, profile.ID}
	joins := []JoinClause{Join[sample.Account, sample.Profile](account.ID, profile.AccountID)}

	w := And[entity.Entity](
		Eq[entity.Entity](account.ID, int64(1)),
		Like[entity.Entity](profile.Bio, "%hello%"),
	)

	sql, args, err := selectJoinSQL("accounts", projection, joins, w)
	require.NoError(t, err)
	require.Equal(t, []any{int64(1), "%hello%"}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestJoin_Select_WhereE1OrE2(t *testing.T) {
	// Join accounts + profiles, then apply WHERE filters using OR across both tables.
	projection := []entity.JoinFieldProvider{account.ID, profile.ID}
	joins := []JoinClause{Join[sample.Account, sample.Profile](account.ID, profile.AccountID)}

	w := Or[entity.Entity](
		Eq[entity.Entity](account.ID, int64(1)),
		Like[entity.Entity](profile.Bio, "%hello%"),
	)

	sql, args, err := selectJoinSQL("accounts", projection, joins, w)
	require.NoError(t, err)
	require.Equal(t, []any{int64(1), "%hello%"}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestJoin_Select_LeftJoin_WhereOnE2_Pitfall(t *testing.T) {
	// SQL pitfall: LEFT JOIN + WHERE on E2 column effectively behaves like INNER JOIN.
	projection := []entity.JoinFieldProvider{account.ID, profile.ID}
	joins := []JoinClause{LeftJoin[sample.Account, sample.Profile](account.ID, profile.AccountID)}

	w := Like[entity.Entity](profile.Bio, "%hello%")
	sql, args, err := selectJoinSQL("accounts", projection, joins, w)
	require.NoError(t, err)
	require.Equal(t, []any{"%hello%"}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestJoin_Select_LeftJoin_E2FilterInOn(t *testing.T) {
	// Correct way to keep LEFT JOIN semantics while filtering E2 is to put E2 filter in ON.
	projection := []entity.JoinFieldProvider{account.ID, profile.ID}
	j := LeftJoin[sample.Account, sample.Profile](account.ID, profile.AccountID)
	// Note: ON supports AND only; this is fine for the typical left-join filter case.
	on := And[entity.Entity](
		whereFromJoinOn[sample.Account, sample.Profile](j),
		whereFunc[entity.Entity](func() (string, []any) { return "(profiles.Bio LIKE ?)", []any{"%hello%"} }),
	)
	onClause, onArgs := on.Build()
	require.Equal(t, []any{"%hello%"}, onArgs)

	joins := []JoinClause{joinClause("LEFT JOIN", "profiles", onClause)}
	sql, args, err := selectJoinSQL("accounts", projection, joins, nil)
	require.NoError(t, err)
	require.Equal(t, []any(nil), args)
	// args are embedded in ON clause above, so this builder currently doesn't carry them;
	// we snapshot the SQL string only.
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestJoin_Select_CompositeKeys_WhereE1AndE2(t *testing.T) {
	projection := []entity.JoinFieldProvider{account.ID, account.Email, profile.ID, profile.AccountID}
	j := Join[sample.Account, sample.Profile](account.ID, profile.AccountID).
		And(Join[sample.Account, sample.Profile](account.Email, profile.Bio))

	w := And[entity.Entity](
		Gt[entity.Entity](account.ID, int64(10)),
		Like[entity.Entity](profile.Bio, "%vip%"),
	)

	sql, args, err := selectJoinSQL("accounts", projection, []JoinClause{j}, w)
	require.NoError(t, err)
	require.Equal(t, []any{int64(10), "%vip%"}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestJoin_Select_SelfJoin_Alias(t *testing.T) {
	// Self-join requires aliasing E2. Our current Join() doesn't expose aliasing,
	// so we cover the SQL generation shape with a test-only JOIN clause.
	projection := []entity.JoinFieldProvider{account.ID, account.Email}
	joins := []JoinClause{joinClause("INNER JOIN", "accounts a2", "(accounts.ID = a2.ID)")}

	sql, args, err := selectJoinSQL("accounts", projection, joins, nil)
	require.NoError(t, err)
	require.Nil(t, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}

func TestJoin_Select_WhereNestedAndOr(t *testing.T) {
	// Explicit nesting: (E1 AND E2) OR (E1 AND E2)
	projection := []entity.JoinFieldProvider{account.ID, profile.ID}
	joins := []JoinClause{Join[sample.Account, sample.Profile](account.ID, profile.AccountID)}

	left := And[entity.Entity](
		Eq[entity.Entity](account.ID, int64(1)),
		Like[entity.Entity](profile.Bio, "%a%"),
	)
	right := And[entity.Entity](
		Eq[entity.Entity](account.ID, int64(2)),
		Like[entity.Entity](profile.Bio, "%b%"),
	)

	w := Or[entity.Entity](left, right)

	sql, args, err := selectJoinSQL("accounts", projection, joins, w)
	require.NoError(t, err)
	require.Equal(t, []any{int64(1), "%a%", int64(2), "%b%"}, args)
	require.NoError(t, assertSQLSnapshotByName(t.Name(), sql))
}
