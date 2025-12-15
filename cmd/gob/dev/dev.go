package dev

import "github.com/spf13/cobra"

// DevCmd represents the dev command group
var DevCmd = &cobra.Command{
	Use:   "dev",
	Short: "Commands for development utilities (dependency checks, git hooks).",
}

func init() {
	// Here we will add subcommands to DevCmd
}
