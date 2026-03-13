package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "litcode",
	Short: "Literate programming checker",
	Long:  "Validates that markdown documentation covers all non-comment source lines.",
}

func Execute() error {
	return rootCmd.Execute()
}
