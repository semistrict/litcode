package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/semistrict/litcode/internal/checkdiff"
	"github.com/spf13/cobra"
)

func init() {
	checkdiffCmd.Flags().StringArrayVar(&checkdiffDocs, "docs", nil, "Markdown file(s) or glob(s) describing the diff; may live outside the repo")
	checkdiffCmd.Flags().StringArrayVar(&checkdiffSource, "source", nil, "Source file globs to validate when .litcode.json is unavailable or should be overridden")
	checkdiffCmd.Flags().StringArrayVar(&checkdiffLenient, "lenient", nil, "Source globs that are validated when referenced but excluded from added-line coverage requirements")
	checkdiffCmd.Flags().StringArrayVar(&checkdiffExclude, "exclude", nil, "Source globs to skip entirely")
	checkdiffCmd.Flags().BoolVar(&checkdiffServe, "serve", false, "after a passing check, render the supplied markdown to temporary HTML, serve it on localhost, and open it in a browser")
	rootCmd.AddCommand(checkdiffCmd)
}

var (
	checkdiffDocs    []string
	checkdiffSource  []string
	checkdiffLenient []string
	checkdiffExclude []string
	checkdiffServe   bool
)

var checkdiffCmd = &cobra.Command{
	Use:           "checkdiff --docs path/to/change.md [git-diff-args...]",
	Short:         "Check that a git diff is explained by markdown",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `Validates that a git diff is covered by the supplied markdown files.

Added lines must be covered like a normal "litcode check". Removed code must be
mentioned in markdown prose by symbol name and file, for example "Removed
ParseConfig in cmd/config.go".

The markdown files are provided explicitly via --docs and may live outside the
repository. Source globs and exclusions come from .litcode.json in the current
repository, unless overridden with --source/--lenient/--exclude.

Examples:

  litcode checkdiff --docs ../notes/change.md
  litcode checkdiff --docs ../notes/change.md 1a2b3c4
  litcode checkdiff --serve --docs ../notes/change.md
  litcode checkdiff --docs ../notes/change.md -- --cached -- cmd/check.go`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveCheckdiffConfig()
		if err != nil {
			return err
		}

		result, err := checkdiff.Check(checkdiff.Config{
			DocsDirs:   cfg.Docs,
			SourceDirs: cfg.Source,
			Lenient:    cfg.Lenient,
			Exclude:    cfg.Exclude,
			GitArgs:    args,
		})
		if err != nil {
			return err
		}

		for _, w := range result.Warnings {
			errf("%s %s:%d — %s\n",
				styleWarn.Render("WARNING:"),
				styleFile.Render(w.DocFile), w.DocLine, w.Message)
		}

		fixable := 0
		for _, inv := range result.Invalid {
			if inv.Fixable {
				fixable++
				errf("%s %s:%d references %s%s — %s\n",
					styleFixable.Render("FIXABLE:"),
					styleFile.Render(inv.DocFile), inv.DocLine,
					styleFile.Render(inv.SourceFile), formatLinesRef(inv.StartLine, inv.EndLine), inv.Reason)
			} else {
				errf("%s %s:%d references %s%s — %s\n",
					styleInvalid.Render("INVALID:"),
					styleFile.Render(inv.DocFile), inv.DocLine,
					styleFile.Render(inv.SourceFile), formatLinesRef(inv.StartLine, inv.EndLine), inv.Reason)
			}
			if ctx := docContext(inv.DocFile, inv.DocLine); ctx != "" {
				errf("%s\n", styleLineNum.Render(ctx))
			}
			if inv.Diff != "" {
				errf("%s", renderDiff(inv.Diff))
			}
		}

		printMissing(result.MissingAdded, cfg.Source)

		for _, missing := range result.MissingRemoved {
			if missing.Symbol != "" {
				errf("%s markdown must mention %s in %s\n",
					styleMissing.Render("REMOVED:"),
					styleFile.Render(missing.Symbol),
					styleFile.Render(missing.File))
				continue
			}
			errf("%s markdown must mention removed code in %s\n",
				styleMissing.Render("REMOVED:"),
				styleFile.Render(missing.File))
		}

		if len(result.Invalid) == 0 && len(result.MissingAdded) == 0 && len(result.MissingRemoved) == 0 {
			fmt.Println(styleSuccess.Render("✓ Diff is fully documented."))
			if checkdiffServe {
				return serveCheckdiffDocs(cfg.Docs, cfg.Source)
			}
			return nil
		}

		if fixable > 0 {
			errf("\n%s %d changed block(s) can be fixed automatically with %s and then re-checked here.\n",
				styleHint.Render("hint:"), fixable, styleHint.Render("litcode fix"))
		}

		return fmt.Errorf("found %d invalid block(s), %d missing added range(s), and %d missing removal mention(s)",
			len(result.Invalid), len(result.MissingAdded), len(result.MissingRemoved))
	},
}

func resolveCheckdiffConfig() (litcodeConfig, error) {
	if len(checkdiffDocs) == 0 {
		return litcodeConfig{}, errors.New(`checkdiff requires at least one --docs path or glob`)
	}

	cfg, err := loadConfigIfPresent()
	if err != nil {
		return litcodeConfig{}, err
	}

	cfg.Docs = append([]string{}, checkdiffDocs...)
	if len(checkdiffSource) > 0 {
		cfg.Source = append([]string{}, checkdiffSource...)
	}
	if len(checkdiffLenient) > 0 {
		cfg.Lenient = append([]string{}, checkdiffLenient...)
	}
	if len(checkdiffExclude) > 0 {
		cfg.Exclude = append([]string{}, checkdiffExclude...)
	}

	if len(cfg.Source) == 0 {
		return litcodeConfig{}, errors.New(`checkdiff needs source globs; add .litcode.json or pass --source`)
	}
	return cfg, nil
}

func loadConfigIfPresent() (litcodeConfig, error) {
	_, err := os.Stat(configFile)
	if err == nil {
		return loadConfig()
	}
	if errors.Is(err, os.ErrNotExist) {
		return litcodeConfig{
			Lenient: []string{},
			Exclude: []string{},
		}, nil
	}
	return litcodeConfig{}, fmt.Errorf("reading %s: %w", configFile, err)
}
