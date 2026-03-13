package cmd

import (
	"fmt"

	"github.com/semistrict/litcode/internal/checker"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(fixCmd)
}

var fixCmd = &cobra.Command{
	Use:           "fix",
	Short:         "Automatically fix minor doc/source mismatches",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `Reads .litcode.json from the current directory and automatically fixes minor
mismatches such as line drift and whitespace differences. Genuine content
mismatches are reported but not modified.

This is equivalent to running "litcode check" but with automatic fixing enabled.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		litcodeCfg, err := loadConfig()
		if err != nil {
			return err
		}
		cfg := checker.Config{
			DocsDirs:   litcodeCfg.Docs,
			SourceDirs: litcodeCfg.Source,
			Exclude:    litcodeCfg.Exclude,
			Fix:        true,
		}

		result, err := checker.Check(cfg)
		if err != nil {
			return err
		}

		if len(result.Fixed) > 0 {
			errf("%s Fixed %d issue(s)\n", styleSuccess.Render("✓"), len(result.Fixed))
			for _, f := range result.Fixed {
				errf("  %s:%d — %s\n", styleFile.Render(f.DocFile), f.DocLine, f.Reason)
			}
		}

		unfixable := 0
		for _, inv := range result.Invalid {
			unfixable++
			errf("%s %s:%d references %s%s — %s\n",
				styleInvalid.Render("INVALID:"),
				styleFile.Render(inv.DocFile), inv.DocLine,
				styleFile.Render(inv.SourceFile), formatLinesRef(inv.StartLine, inv.EndLine), inv.Reason)
			if inv.Diff != "" {
				errf("%s\n", inv.Diff)
			}
		}

		if len(result.Invalid) == 0 && len(result.Missing) == 0 && len(result.Fixed) == 0 {
			fmt.Println(styleSuccess.Render("✓ All source lines are covered."))
			return nil
		}

		printMissing(result.Missing, litcodeCfg.Source)

		if unfixable > 0 || len(result.Missing) > 0 {
			return fmt.Errorf("found %d unfixable issue(s) and %d missing range(s)",
				unfixable, len(result.Missing))
		}
		return nil
	},
}
