package sqlx

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/kcmvp/dvo/sample/entity"
	acctfield "github.com/kcmvp/dvo/sample/gen/field/account"
	orderfield "github.com/kcmvp/dvo/sample/gen/field/order"
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

func (s *SQLXTestSuite) TestQueryAllAccounts() {
	ctx := context.Background()
	schema := NewSchema[entity.Account](acctfield.All()...)
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
	schema := NewSchema[entity.Account](acctfield.All()...)
	w := Eq[entity.Account](acctfield.Email, "alice@example.com")
	res, err := Query(ctx, schema, w)
	require.NoError(s.T(), err)
	require.Len(s.T(), res, 1)
	require.Equal(s.T(), "alice@example.com", res[0].MstString("Email"))
}

func (s *SQLXTestSuite) TestQueryOrdersByAccountID() {
	ctx := context.Background()
	schema := NewSchema[entity.Order](orderfield.All()...)
	w := Eq[entity.Order](orderfield.AccountID, int64(1))
	res, err := Query(ctx, schema, w)
	require.NoError(s.T(), err)
	require.GreaterOrEqual(s.T(), len(res), 1)
	// verify AccountID in result
	require.Equal(s.T(), int64(1), res[0].MstInt64("AccountID"))
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
