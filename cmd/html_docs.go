package cmd

import (
	"fmt"
	"os"

	"github.com/semistrict/litcode/internal/renderdocs"
	"github.com/spf13/cobra"
)

var renderDocsOut string

func init() {
	htmlDocsCmd.Flags().StringVar(&renderDocsOut, "out", "out/docs", "output directory for rendered HTML")
	rootCmd.AddCommand(htmlDocsCmd)
}

var htmlDocsCmd = &cobra.Command{
	Use:           "html-docs",
	Short:         "Render markdown docs to HTML",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `Reads .litcode.json from the current directory and renders markdown documentation
to static HTML, preserving internal links and converting Mermaid fences into
live diagrams. Output defaults to ./out/docs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		litcodeCfg, err := loadConfig()
		if err != nil {
			return err
		}
		for _, dir := range litcodeCfg.Docs {
			if err := renderdocs.RenderTree(dir, renderDocsOut, litcodeCfg.Source, os.Stdout); err != nil {
				return fmt.Errorf("rendering %s: %w", dir, err)
			}
		}
		return nil
	},
}
