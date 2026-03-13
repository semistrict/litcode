package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/semistrict/litcode/internal/expanddocs"
	"github.com/semistrict/litcode/internal/filematch"
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
	Long: `Reads .litcode.jsonc from the current directory and expands abbreviated markdown
code blocks that omit a middle section with an ellipsis comment marker, writing
the fully expanded markdown to disk. Output defaults to ./out/expanded-docs.`,
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
			expanded, err := expanddocs.ExpandedMarkdown(match.AbsPath, litcodeCfg.Source)
			if err != nil {
				return err
			}
			outPath := filepath.Join(expandDocsOut, filepath.FromSlash(match.RelPath))
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(outPath, expanded, 0o644); err != nil {
				return err
			}
		}
		return nil
	},
}
