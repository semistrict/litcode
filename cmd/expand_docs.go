package cmd

import (
	"fmt"

	"github.com/semistrict/litcode/internal/expanddocs"
	"github.com/spf13/cobra"
)

var expandDocsOut string

func init() {
	expandDocsCmd.Flags().StringVar(&expandDocsOut, "out", "out/expanded-docs", "output directory for expanded markdown")
	rootCmd.AddCommand(expandDocsCmd)
}

var expandDocsCmd = &cobra.Command{
	Use:           "expand-docs",
	Short:         "Write markdown docs with abbreviated blocks expanded",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `Reads .litcode.json from the current directory and expands abbreviated markdown
code blocks that omit a middle section with an ellipsis comment marker, writing
the fully expanded markdown to disk. Output defaults to ./out/expanded-docs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		litcodeCfg, err := loadConfig()
		if err != nil {
			return err
		}
		for _, dir := range litcodeCfg.Docs {
			if err := expanddocs.ExpandTree(dir, expandDocsOut, litcodeCfg.Source); err != nil {
				return fmt.Errorf("expanding %s: %w", dir, err)
			}
		}
		return nil
	},
}
