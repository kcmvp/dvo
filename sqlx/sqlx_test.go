package sqlx

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/kcmvp/dvo/entity"
	sample "github.com/kcmvp/dvo/sample/entity"
	acctfield "github.com/kcmvp/dvo/sample/gen/field/account"
	accrolefield "github.com/kcmvp/dvo/sample/gen/field/accountrole"
	orderfield "github.com/kcmvp/dvo/sample/gen/field/order"
	profilefield "github.com/kcmvp/dvo/sample/gen/field/profile"
	rolefield "github.com/kcmvp/dvo/sample/gen/field/role"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/tidwall/gjson"
)

type SQLXTestSuite struct {
	suite.Suite
}

func (s *SQLXTestSuite) SetupSuite() {
	// Try to get DefaultDS first (this relies on initDataSources reading application_test.yml).
	db, ok := DefaultDS()
	s.Require().True(ok && db != nil, "default datasource not configured; please provide application_test.yml with a 'datasource.DefaultDS' entry or register a DefaultDS before running tests")

	// If we got an existing DB (from DefaultDS), ensure schemas and fixtures are applied.
	// This path is used when application_test.yml points to a test DB.
	// Apply schema files from testdata/schemas/sqlite
	schemaDir := filepath.Join("..", "testdata", "schemas", "sqlite")
	files := []string{"account_schema.sql", "profile_schema.sql", "order_schema.sql", "order_item_schema.sql", "product_schema.sql", "role_schema.sql", "account_role_schema.sql"}
	ctx := context.Background()
	for _, f := range files {
		p := filepath.Join(schemaDir, f)
		b, err := os.ReadFile(p)
		s.Require().NoError(err, "read schema %s", p)
		_, err = db.ExecContext(ctx, string(b))
		s.Require().NoError(err, "exec schema %s", p)
	}

	// load test data file from testdata/sqlite_data.json using gjson
	b, err := os.ReadFile(filepath.Join("..", "testdata", "sqlite_data.json"))
	s.Require().NoError(err)

	tables := []string{"accounts", "profiles", "orders", "order_items", "products", "roles", "account_roles"}
	for _, tbl := range tables {
		arr := gjson.GetBytes(b, tbl)
		if !arr.Exists() || !arr.IsArray() {
			continue
		}
		arr.ForEach(func(_, v gjson.Result) bool {
			var m map[string]any
			// use encoding/json to unmarshal into map[string]any to get native Go types
			if err := json.Unmarshal([]byte(v.Raw), &m); err != nil {
				// fail the test setup on invalid JSON
				s.Require().NoError(err)
				return false
			}

			cols := make([]string, 0, len(m))
			ph := make([]string, 0, len(m))
			args := make([]any, 0, len(m))
			for k, val := range m {
				cols = append(cols, k)
				ph = append(ph, "?")
				// convert float64 that represent integers into int64 to match schema expectations
				switch t := val.(type) {
				case float64:
					if math.Mod(t, 1.0) == 0 {
						args = append(args, int64(t))
					} else {
						args = append(args, t)
					}
				default:
					args = append(args, t)
				}
			}
			if len(cols) == 0 {
				return true
			}
			insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tbl, stringsJoin(cols, ", "), stringsJoin(ph, ", "))
			_, err := db.ExecContext(ctx, insertSQL, args...)
			s.Require().NoError(err, "insert into %s failed: %v", tbl, err)
			return true
		})
	}
}

func (s *SQLXTestSuite) TearDownSuite() {
	// ensure registry is cleared and DBs closed
	require.NoError(s.T(), CloseAllDataSources())
}

// helper to execute a join SQL and return first row as map[col]any
func execJoinSQLFirstRow(t *testing.T, sqlStr string, args []any) map[string]any {
	ctx := context.Background()
	db, ok := DefaultDS()
	require.True(t, ok && db != nil)
	rows, err := db.QueryContext(ctx, sqlStr, args...)
	require.NoError(t, err)
	defer rows.Close()
	cols, err := rows.Columns()
	require.NoError(t, err)
	if !rows.Next() {
		t.Fatalf("expected at least one row for SQL: %s", sqlStr)
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	require.NoError(t, rows.Scan(ptrs...))
	m := make(map[string]any, len(cols))
	for i, c := range cols {
		v := vals[i]
		switch vt := v.(type) {
		case nil:
			m[c] = nil
		case []byte:
			m[c] = string(vt)
		default:
			m[c] = vt
		}
	}
	return m
}

func (s *SQLXTestSuite) TestQueryAllAccounts() {
	ctx := context.Background()
	schema := NewSchema[sample.Account](acctfield.All()...)
	res, err := Query(ctx, schema, nil)
	require.NoError(s.T(), err)
	require.GreaterOrEqual(s.T(), len(res), 1)
	// verify basic fields exist
	first := res[0]
	require.NotZero(s.T(), first.MstInt64("ID"))
	require.NotEmpty(s.T(), first.MstString("Email"))
}

func (s *SQLXTestSuite) TestQueryAccountByEmail() {
	ctx := context.Background()
	schema := NewSchema[sample.Account](acctfield.All()...)
	w := Eq[sample.Account](acctfield.Email, "alice@example.com")
	res, err := Query(ctx, schema, w)
	require.NoError(s.T(), err)
	require.Len(s.T(), res, 1)
	require.Equal(s.T(), "alice@example.com", res[0].MstString("Email"))
}

func (s *SQLXTestSuite) TestQueryOrdersByAccountID() {
	ctx := context.Background()
	schema := NewSchema[sample.Order](orderfield.All()...)
	w := Eq[sample.Order](orderfield.AccountID, int64(1))
	res, err := Query(ctx, schema, w)
	require.NoError(s.T(), err)
	require.GreaterOrEqual(s.T(), len(res), 1)
	// verify AccountID in result
	require.Equal(s.T(), int64(1), res[0].MstInt64("AccountID"))
}

// New join execution tests below: account <-> profile (1:1), account <-> order (1:N), account <-> role (N:N)
func (s *SQLXTestSuite) TestQueryJoin_AccountWithProfile() {
	schema := NewSchema[sample.Account](acctfield.All()...)
	joins := []JoinClause{Join[sample.Account, sample.Profile](acctfield.ID, profilefield.AccountID)}
	w := whereFunc[entity.Entity](func() (string, []any) { return fmt.Sprintf("%s = ?", acctfield.ID.QualifiedName()), []any{int64(1)} })
	res, err := QueryJoin[sample.Account](context.Background(), schema, joins, w)
	require.NoError(s.T(), err)
	require.GreaterOrEqual(s.T(), len(res), 1)
	first := res[0]
	require.Equal(s.T(), "alice@example.com", first.MstString("Email"))
}

func (s *SQLXTestSuite) TestQueryJoin_AccountWithOrders() {
	schema := NewSchema[sample.Account](acctfield.All()...)
	joins := []JoinClause{Join[sample.Account, sample.Order](acctfield.ID, orderfield.AccountID)}
	w := whereFunc[entity.Entity](func() (string, []any) { return fmt.Sprintf("%s = ?", acctfield.ID.QualifiedName()), []any{int64(1)} })
	res, err := QueryJoin[sample.Account](context.Background(), schema, joins, w)
	require.NoError(s.T(), err)
	require.GreaterOrEqual(s.T(), len(res), 1)
	first := res[0]
	require.Equal(s.T(), "alice@example.com", first.MstString("Email"))
}

func (s *SQLXTestSuite) TestQueryJoin_AccountWithRoleViaJoinTable() {
	schema := NewSchema[sample.Account](acctfield.All()...)
	j1 := Join[sample.Account, sample.AccountRole](acctfield.ID, accrolefield.AccountID)
	j2 := Join[sample.AccountRole, sample.Role](accrolefield.RoleID, rolefield.ID)
	joins := []JoinClause{j1, j2}
	res, err := QueryJoin[sample.Account](context.Background(), schema, joins, nil)
	require.NoError(s.T(), err)
	require.GreaterOrEqual(s.T(), len(res), 1)
	first := res[0]
	require.NotEmpty(s.T(), first.MstString("Email"))
}

func (s *SQLXTestSuite) TestQueryJoin_LeftJoin_ProfileOptional() {
	schema := NewSchema[sample.Account](acctfield.All()...)
	joins := []JoinClause{LeftJoin[sample.Account, sample.Profile](acctfield.ID, profilefield.AccountID)}
	w := whereFunc[entity.Entity](func() (string, []any) { return fmt.Sprintf("%s = ?", acctfield.ID.QualifiedName()), []any{int64(4)} })
	res, err := QueryJoin[sample.Account](context.Background(), schema, joins, w)
	require.NoError(s.T(), err)
	require.GreaterOrEqual(s.T(), len(res), 1)
	first := res[0]
	require.Equal(s.T(), "dave+4@example.com", first.MstString("Email"))
}

func (s *SQLXTestSuite) TestCount() {
	db, ok := DefaultDS()
	s.Require().True(ok && db != nil)

	b, err := os.ReadFile(filepath.Join("..", "testdata", "sqlite_data.json"))
	s.Require().NoError(err)

	// build expected counts map once
	tables := []string{"accounts", "profiles", "orders", "order_items", "products", "roles", "account_roles"}
	expected := make(map[string]int64, len(tables))
	for _, tbl := range tables {
		arr := gjson.GetBytes(b, tbl)
		if arr.Exists() && arr.IsArray() {
			expected[tbl] = int64(len(arr.Array()))
		} else {
			expected[tbl] = 0
		}
	}

	// table-driven cases: only include the concrete entity-backed tables for now
	tests := []struct {
		name  string
		table string
		call  func() (int64, error)
	}{
		{
			name:  "accounts",
			table: "accounts",
			call: func() (int64, error) {
				c := NewSchema[sample.Account](acctfield.All()...)
				return Count[sample.Account](context.Background(), c, nil)
			},
		},
		{
			name:  "orders",
			table: "orders",
			call: func() (int64, error) {
				c := NewSchema[sample.Order](orderfield.All()...)
				return Count[sample.Order](context.Background(), c, nil)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			got, err := tc.call()
			require.NoError(t, err)
			require.Equal(t, expected[tc.table], got, "table %s count mismatch", tc.table)
		})
	}
}

func TestSQLXTestSuite(t *testing.T) {
	suite.Run(t, new(SQLXTestSuite))
}

// stringsJoin is a tiny helper to avoid importing strings in many places
func stringsJoin(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	op := parts[0]
	for i := 1; i < len(parts); i++ {
		op += sep + parts[i]
	}
	return op
}
