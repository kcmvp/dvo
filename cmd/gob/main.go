package main

import (
	"fmt"
	"os"

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
		if internal.Project != nil {
			fmt.Printf("Project root is %s", internal.Project.Root)
		} else {
			fmt.Printf("Project is not initialized")
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
