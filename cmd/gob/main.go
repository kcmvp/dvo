package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/kcmvp/dvo/cmd/gob/dev"
	"github.com/kcmvp/dvo/cmd/gob/sca"
	"github.com/kcmvp/dvo/cmd/gob/xql"
	"github.com/kcmvp/dvo/cmd/internal"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gob",
	Short: "gob is a comprehensive Go project assistant.",
	Long: `gob (Go Booter) is a command-line tool that provides code generation,
project scaffolding, and development utilities to accelerate Go development.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if internal.Current != nil {
			fmt.Printf("Project root is %s\n", internal.Current.Root)
			blockPrefixes := []string{
				fmt.Sprintf("%s/cmd", internal.ToolModulePath()),
				fmt.Sprintf("%s/sample", internal.ToolModulePath()),
			}

			for _, pkg := range internal.Current.Pkgs {
				for impPath := range pkg.Imports {
					if slices.Contains(blockPrefixes, impPath) {
						return fmt.Errorf("invalid import '%s' in package %s; user projects must not import '%v'", impPath, pkg.PkgPath, blockPrefixes)
					}
				}
			}
		} else {
			fmt.Printf("Project is not initialized\n")
		}
		return nil
	},
}

func init() {
	// Add subcommands from their respective packages to the root command
	rootCmd.AddCommand(xql.XqlCmd)
	rootCmd.AddCommand(sca.ScaCmd)
	rootCmd.AddCommand(dev.DevCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
