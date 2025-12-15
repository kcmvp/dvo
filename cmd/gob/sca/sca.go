package sca

import "github.com/spf13/cobra"

// ScaCmd represents the sca command group
var ScaCmd = &cobra.Command{
	Use:   "sca",
	Short: "Commands for project scaffolding.",
}

func init() {
	// Here we will add subcommands to ScaCmd
}
