package xql

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kcmvp/dvo/cmd/internal"
	"github.com/spf13/cobra"
)

// typed context key to avoid collisions
type ctxKey string

var dbAdapterKey ctxKey = "xql.dbAdapter"

// AdapterInfo represents the shape of each adapter entry in drivers.json
type AdapterInfo struct {
	Drivers     []string          `json:"drivers"`
	TypeMapping map[string]string `json:"typeMapping"`
	PK          map[string]string `json:"pk"`
}

// loadDrivers attempts to read cmd/gob/xql/drivers.json and parse a mapping of
// adapter names -> AdapterInfo. If the file is missing or cannot be parsed,
// it returns an error and the caller should fallback to legacy driver name groups.
func loadDrivers() (map[string]AdapterInfo, error) {
	base := "cmd/gob/xql/drivers.json"
	// path relative to project root
	p := filepath.Join(internal.Project.Root, base)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var m map[string]AdapterInfo
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// normalizeSQLType lowercases the SQL type and strips any size/precision
// parameters (e.g. "TINYINT(1)" -> "tinyint").
func normalizeSQLType(t string) string {
	if t == "" {
		return ""
	}
	t = strings.ToLower(strings.TrimSpace(t))
	if i := strings.Index(t, "("); i >= 0 {
		t = strings.TrimSpace(t[:i])
	}
	return t
}

// sqlTypeFor returns the SQL type for a given Go type and adapter using the
// parsed drivers map. If no mapping exists, it falls back to a sensible default.
func sqlTypeFor(goType string, adapter string, drivers map[string]AdapterInfo) string {
	if info, ok := drivers[adapter]; ok {
		if typ, ok := info.TypeMapping[goType]; ok {
			return typ
		}
	}
	// fallback defaults (conservative)
	switch goType {
	case "int64", "int":
		return "BIGINT"
	case "int32":
		return "INTEGER"
	case "int16":
		return "SMALLINT"
	case "int8":
		return "SMALLINT"
	case "bool":
		// MySQL historically uses TINYINT(1) for booleans, but BOOLEAN is OK too
		if adapter == "mysql" {
			return "TINYINT(1)"
		}
		return "BOOLEAN"
	case "string":
		return "TEXT"
	case "float32":
		return "REAL"
	case "float64":
		if adapter == "postgres" {
			return "DOUBLE PRECISION"
		}
		return "DOUBLE"
	case "time.Time":
		if adapter == "postgres" {
			return "TIMESTAMP WITH TIME ZONE"
		}
		return "DATETIME"
	case "[]byte":
		if adapter == "postgres" {
			return "BYTEA"
		}
		return "BLOB"
	default:
		return "TEXT"
	}
}

// pkConstraintFor returns the PK constraint clause for the given Go type and
// SQL type for the adapter. It normalizes the SQL type, tries exact and family
// fallbacks, and returns an optional warning if PK is used on a discouraged Go type.
func pkConstraintFor(goType string, sqlType string, adapter string, drivers map[string]AdapterInfo) (string, string) {
	info, ok := drivers[adapter]
	if !ok {
		return "", ""
	}
	// If the adapter defines a PK mapping, attempt to use it.
	norm := normalizeSQLType(sqlType)
	// exact match
	if v, ok := info.PK[norm]; ok {
		if goType == "int8" {
			return v, "primary key defined on int8: small integer PKs are discouraged"
		}
		return v, ""
	}
	// fall back to generic integer mapping (we only map PK on 'integer')
	if v, ok := info.PK["integer"]; ok {
		if strings.HasPrefix(norm, "int") || norm == "bigint" || norm == "smallint" || norm == "tinyint" {
			if goType == "int8" {
				return v, "primary key defined on int8: small integer PKs are discouraged"
			}
			return v, ""
		}
	}
	return "", ""
}

// XqlCmd represents the xql command group
var XqlCmd = &cobra.Command{
	Use:   "xql",
	Short: "Commands for XQL code generation (entities, schemas).",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// 1: ensure the project depends on 'github.com/kcmvp/dvo'
		if !internal.Project.DependsOn("github.com/kcmvp/dvo") {
			return fmt.Errorf("project does not depend on github.com/kcmvp/dvo; add it to go.mod")
		}

		// 2: detect database adapter by checking for adapter submodules like
		// github.com/kcmvp/dvo/<driver> and additional known driver modules from
		// cmd/gob/xql/drivers.json
		dbAdapter := ""

		// try to load drivers.json; ignore errors and fall back to legacy groups
		driversMap, _ := loadDrivers()

		// helper to check both github.com/kcmvp/dvo/<name> and the modules listed in drivers.json
		checkAdapter := func(names []string, adapter string) bool {
			// first, check the canonical dvo submodule paths e.g. github.com/kcmvp/dvo/sqlite
			for _, n := range names {
				mod := fmt.Sprintf("github.com/kcmvp/dvo/%s", n)
				if internal.Project.DependsOn(mod) {
					dbAdapter = adapter
					return true
				}
			}
			// next, check any explicit driver modules from drivers.json
			if info, ok := driversMap[adapter]; ok {
				for _, mod := range info.Drivers {
					if internal.Project.DependsOn(mod) {
						dbAdapter = adapter
						return true
					}
				}
			}
			return false
		}

		// Run checks in order; prefer sqlite, then mysql, then postgres (order is arbitrary)
		if checkAdapter([]string{"sqlite"}, "sqlite") || checkAdapter([]string{"mysql", "mariadb"}, "mysql") || checkAdapter([]string{"postgres", "cockroachdb"}, "postgres") {
			// adapter detected
		}

		// attach the detected adapter (may be empty) to the command context for subcommands
		ctx := context.WithValue(cmd.Context(), dbAdapterKey, dbAdapter)
		cmd.SetContext(ctx)

		// 3: TODO - discover structs implementing entity.Entity using go/packages
		return nil
	},
}

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Generate schemas for all entities.",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement schema generation logic
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate entity and schema definitions.",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement validation logic
	},
}

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Generate or update index files.",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement index generation logic
	},
}

func init() {
	XqlCmd.AddCommand(schemaCmd)
	XqlCmd.AddCommand(validateCmd)
	XqlCmd.AddCommand(indexCmd)
}
