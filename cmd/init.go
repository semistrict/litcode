package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:           "init",
	Short:         "Create a default .litcode.json",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long:          "Creates a default .litcode.json in the current directory.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat(configFile); err == nil {
			return fmt.Errorf("%s already exists", configFile)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("checking %s: %w", configFile, err)
		}

		if err := writeConfig(configFile, defaultConfig()); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", configFile)
		return nil
	},
}
