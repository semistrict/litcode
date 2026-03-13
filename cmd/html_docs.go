package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/semistrict/litcode/internal/filematch"
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
		docMatches, err := filematch.Collect(litcodeCfg.Docs, func(relPath string) bool {
			return strings.HasSuffix(relPath, ".md")
		})
		if err != nil {
			return fmt.Errorf("collecting docs: %w", err)
		}
		for _, match := range docMatches {
			if err := renderdocs.RenderFile(match.AbsPath, match.RelPath, renderDocsOut, litcodeCfg.Source, os.Stdout); err != nil {
				return err
			}
		}
		return nil
	},
}
