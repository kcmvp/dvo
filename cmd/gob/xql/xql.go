package xql

import (
	"context"
	"fmt"
	"strings"

	_ "embed"

	"github.com/kcmvp/dvo/cmd/internal"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/tidwall/gjson"
)

// Context keys and other shared constants used across the tools.
const (
	// dbaAdapterKey is the context key used to store the detected DB adapter
	// for xql subcommands.
	dbaAdapterKey = "xql.dbAdapter"
	// entityFilterKey is the context key used to store the entity filter function.
	entityFilterKey = "xql.entityFilter"
)

//go:embed resources/drivers.json
var driversJSON []byte

// XqlCmd represents the xql command group
var XqlCmd = &cobra.Command{
	Use:   "xql",
	Short: "Commands for XQL code generation (entities, schemas).",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// 1: ensure the working project depends on this tool's module (inferred at runtime)
		if internal.Current == nil {
			return fmt.Errorf("project context not initialized")
		}
		if !internal.Current.DependsOnTool() {
			return fmt.Errorf("project does not depend on %s; add it to go.mod", internal.ToolModulePath())
		}
		// 2: ensure the project depends on at least one database driver
		driverMap := make(map[string][]string)
		root := gjson.ParseBytes(driversJSON)
		root.ForEach(func(key, value gjson.Result) bool {
			dbName := key.String()
			var drivers []string
			if items := value.Get("drivers"); items.Exists() {
				drivers = append(drivers, lo.Map(items.Array(), func(item gjson.Result, _ int) string {
					return item.String()
				})...)
			}
			driverMap[dbName] = drivers
			return true
		})
		drivers := lo.Flatten(lo.Values(driverMap))
		driverOpt := internal.Current.DependsOn(drivers...)
		if driverOpt.IsAbsent() {
			return fmt.Errorf("project does not depend on any database %s; add it to go.mod", drivers)
		}
		registered := lo.FilterMapToSlice(driverMap, func(key string, values []string) (string, bool) {
			return key, len(lo.Intersect(driverOpt.MustGet(), values)) > 0
		})
		// 3: in all the structs which implements internal.ToolEntityInterface()
		// 4: put registered database names into context for subcommands to use
		parent := cmd.Context()
		if parent == nil {
			parent = context.Background()
		}
		ctx := context.WithValue(parent, dbaAdapterKey, registered)
		cmd.SetContext(ctx)
		return nil
	},
}

var schemaCmd = &cobra.Command{
	Use:   "schema [entities...]",
	Short: "Generate schemas for all entities, or for a subset by passing space-separated entity names (e.g. `xql schema Account Order`).",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		// Cobra already splits args by spaces. We keep it simple and treat each arg as an entity name.
		names := lo.Uniq(lo.FilterMap(args, func(a string, _ int) (string, bool) {
			a = strings.TrimSpace(a)
			return a, a != ""
		}))
		if len(names) > 0 {
			ctx = context.WithValue(ctx, entityFilterKey, names)
		}
		return generate(ctx)
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
