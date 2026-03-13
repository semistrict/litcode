package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/semistrict/litcode/internal/checker"
	"github.com/semistrict/litcode/internal/filematch"
	"github.com/spf13/cobra"
)

var (
	styleSuccess  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))   // green
	styleFixable  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))   // yellow
	styleInvalid  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))   // red
	styleMissing  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))   // cyan
	styleFile     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))   // blue
	styleLineNum  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))              // gray
	styleMissLine = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))              // yellow/muted
	styleDiffAdd  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))              // green
	styleDiffDel  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))              // red
	styleHint     = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Italic(true) // yellow italic
	styleWarn     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))   // yellow bold
)

func init() {
	rootCmd.AddCommand(checkCmd)
}

var checkCmd = &cobra.Command{
	Use:           "check [files...]",
	Short:         "Check that docs cover all source lines",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `Reads .litcode.jsonc from the current directory and validates that markdown
documentation covers all non-comment source lines.

1. Each code block's content matches the referenced source lines exactly.
2. Every non-comment, non-blank source line is covered by at least one code block.

Test files are excluded from missing coverage by default, but still validated if referenced.
testdata/, vendor/, and node_modules/ are excluded entirely.
Use "litcode fix" to automatically correct minor mismatches (line drift, whitespace).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		litcodeCfg, err := loadConfig()
		if err != nil {
			return err
		}
		cfg := checker.Config{
			DocsDirs:   litcodeCfg.Docs,
			SourceDirs: litcodeCfg.Source,
			Lenient:    litcodeCfg.Lenient,
			Exclude:    litcodeCfg.Exclude,
			Files:      args,
		}

		result, err := checker.Check(cfg)
		if err != nil {
			return err
		}

		for _, w := range result.Warnings {
			errf("%s %s:%d — %s\n",
				styleWarn.Render("WARNING:"),
				styleFile.Render(w.DocFile), w.DocLine, w.Message)
		}

		if len(result.Invalid) == 0 && len(result.Missing) == 0 {
			fmt.Println(styleSuccess.Render("✓ All source lines are covered."))
			return nil
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
		printMissing(result.Missing, litcodeCfg.Source)

		if fixable > 0 {
			errf("\n%s %d issue(s) can be fixed automatically. Run:\n  %s\n",
				styleHint.Render("hint:"), fixable, styleHint.Render("litcode fix"))
		}

		return fmt.Errorf("found %d invalid block(s) and %d missing range(s)",
			len(result.Invalid), len(result.Missing))
	},
}

// printMissing groups missing ranges by file and prints one block per file
// with each hunk shown with 3 lines of context.
func printMissing(missing []checker.MissingRange, sourceDirs []string) {
	// Group by file, preserving order.
	type fileRanges struct {
		file   string
		ranges []checker.MissingRange
	}
	var groups []fileRanges
	idx := make(map[string]int)
	for _, m := range missing {
		if i, ok := idx[m.File]; ok {
			groups[i].ranges = append(groups[i].ranges, m)
		} else {
			idx[m.File] = len(groups)
			groups = append(groups, fileRanges{file: m.File, ranges: []checker.MissingRange{m}})
		}
	}

	for _, g := range groups {
		errf("%s %s\n", styleMissing.Render("MISSING:"), styleFile.Render(g.file))

		lines := readSourceFile(g.file, sourceDirs)
		if lines == nil {
			for _, r := range g.ranges {
				errf("  lines %s\n", formatRange(r.StartLine, r.EndLine))
			}
			errf("\n")
			continue
		}

		// Build set of missing line numbers for ">" markers.
		isMissing := make(map[int]bool)
		for _, r := range g.ranges {
			for l := r.StartLine; l <= r.EndLine; l++ {
				isMissing[l] = true
			}
		}

		intervals := checker.MergeDisplayIntervals(g.ranges, len(lines), 3)

		for _, iv := range intervals {
			for i := iv.From; i <= iv.To; i++ {
				num := styleLineNum.Render(fmt.Sprintf("%4d", i))
				src := expandTabs(lines[i-1], 4)
				if isMissing[i] {
					errf("  %s %s %s\n", num, styleMissLine.Render(">"), styleMissLine.Render(src))
				} else {
					errf("  %s   %s\n", num, src)
				}
			}
			errf("\n")
		}
	}
}

// docContext returns a few lines of markdown prose before the code fence at
// docLine, giving the reader context about what the block was documenting.
func docContext(docFile string, docLine int) string {
	data, err := os.ReadFile(docFile)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	// docLine is 1-based and points at the fence line itself.
	// Walk backward from the line before the fence to collect prose context.
	end := docLine - 2 // 0-based index of line before fence
	if end < 0 {
		return ""
	}
	// Skip blank lines immediately above the fence.
	for end >= 0 && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	if end < 0 {
		return ""
	}
	// Collect up to 3 non-blank prose lines.
	const maxContext = 3
	start := end
	for start > 0 && (end-start+1) < maxContext {
		if strings.TrimSpace(lines[start-1]) == "" {
			break
		}
		start--
	}
	var b strings.Builder
	for i := start; i <= end; i++ {
		b.WriteString("  > ")
		b.WriteString(lines[i])
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func readSourceFile(file string, sourcePatterns []string) []string {
	if data, err := os.ReadFile(file); err == nil {
		return strings.Split(string(data), "\n")
	}

	sourceIndex, err := filematch.Index(sourcePatterns)
	if err != nil {
		return nil
	}
	if path, ok := sourceIndex[file]; ok {
		data, err := os.ReadFile(path)
		if err == nil {
			return strings.Split(string(data), "\n")
		}
	}
	return nil
}

func errf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
}

func renderDiff(diff string) string {
	var b strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case '+':
			b.WriteString(styleDiffAdd.Render(line))
		case '-':
			b.WriteString(styleDiffDel.Render(line))
		default:
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func expandTabs(s string, tabWidth int) string {
	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := tabWidth - (col % tabWidth)
			for range spaces {
				b.WriteByte(' ')
			}
			col += spaces
		} else {
			b.WriteRune(r)
			col++
		}
	}
	return b.String()
}

// formatLinesRef returns " lines N-M" or "" if no lines specified.
func formatLinesRef(start, end int) string {
	if start == 0 && end == 0 {
		return ""
	}
	return " lines " + formatRange(start, end)
}

func formatRange(start, end int) string {
	if start == end {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}
