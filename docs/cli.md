# CLI — The `check` Command

[Back to Overview](overview.md) | Previous: [Warnings](checker-warnings.md)

The CLI layer loads `.litcode.json`, translates it into a `checker.Config`, and
formats the results for the terminal.

## Package and imports

The `cmd` package imports `fmt` and `os` for output, `strings` and `filepath`
for diff rendering and source-file lookup, `checker` for the core logic, and
`cobra` for CLI scaffolding:

```go file=cmd/check.go
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
```

## Styled output

The package defines lipgloss styles for terminal output — color-coded prefixes
for each status category, plus styles for line numbers, diffs, and hints:

```go file=cmd/check.go
var (
	styleSuccess  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))  // green
	styleFixable  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))  // yellow
	styleInvalid  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))  // red
	styleMissing  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))  // cyan
	styleFile     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))  // blue
	styleLineNum  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))             // gray
	styleMissLine = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))             // yellow/muted
	styleDiffAdd  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))             // green
	styleDiffDel  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))             // red
	styleHint     = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Italic(true) // yellow italic
	styleWarn     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))  // yellow bold
)
```

## Command registration

The `check` command is registered during package initialization:

```go file=cmd/check.go
func init() {
	rootCmd.AddCommand(checkCmd)
}
```

## Command definition

The command is defined with `SilenceUsage` and `SilenceErrors` to prevent
cobra from printing usage on validation errors:

```go file=cmd/check.go
var checkCmd = &cobra.Command{
	Use:           "check [files...]",
	Short:         "Check that docs cover all source lines",
	SilenceUsage:  true,
	SilenceErrors: true,
	Long: `Reads .litcode.json from the current directory and validates that markdown
documentation covers all non-comment source lines.

1. Each code block's content matches the referenced source lines exactly.
2. Every non-comment, non-blank source line is covered by at least one code block.

Test files are excluded from missing coverage by default, but still validated if referenced.
testdata/, vendor/, and node_modules/ are excluded entirely.
Use "litcode fix" to automatically correct minor mismatches (line drift, whitespace).`,
```

## Command execution

The `RunE` function loads `.litcode.json`, builds a `Config`, and calls
`checker.Check`:

```go file=cmd/check.go
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
```

## Output formatting

Warnings (non-fatal issues like duplicate comments) are always printed first:

```go file=cmd/check.go
		for _, w := range result.Warnings {
			errf("%s %s:%d — %s\n",
				styleWarn.Render("WARNING:"),
				styleFile.Render(w.DocFile), w.DocLine, w.Message)
		}
```

When everything passes, a single line is printed to stdout:

```go file=cmd/check.go
		if len(result.Invalid) == 0 && len(result.Missing) == 0 {
			fmt.Println(styleSuccess.Render("✓ All source lines are covered."))
			return nil
		}
```

Invalid blocks are printed to stderr with full context — fixable issues are
prefixed with `FIXABLE:`, others with `INVALID:`. Diffs are included when
available:

```go file=cmd/check.go
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
```

The small `formatLinesRef` helper omits the ` lines ...` suffix entirely when a
block has no explicit line range, which keeps Mermaid and other content-matched
errors readable:

```go file=cmd/check.go
func formatLinesRef(start, end int) string {
	if start == 0 && end == 0 {
		return ""
	}
	return " lines " + formatRange(start, end)
}
```

Missing ranges are grouped by file and printed with the actual source lines
and 3 lines of context:

```go file=cmd/check.go
		printMissing(result.Missing, litcodeCfg.Source)
```

When fixable issues are found, the actual fix command is suggested:

```go file=cmd/check.go
		if fixable > 0 {
			errf("\n%s %d issue(s) can be fixed automatically. Run:\n  %s\n",
				styleHint.Render("hint:"), fixable, styleHint.Render("litcode fix"))
		}
```

The command exits with an error that summarizes the counts:

```go file=cmd/check.go
		return fmt.Errorf("found %d invalid block(s) and %d missing range(s)",
			len(result.Invalid), len(result.Missing))
	},
}
```

## Range formatting

The `formatRange` helper produces either `"5"` for single lines or `"5-10"`
for ranges:

```go file=cmd/check.go
func formatRange(start, end int) string {
	if start == end {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}
```

## Missing-range display

When the check fails, the user needs to see exactly which lines are undocumented.
`printMissing` renders a diff-style view so developers can quickly locate gaps
without switching to another tool. Missing lines are marked with `>`:

```go file=cmd/check.go
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
```

## Reading source files

The `readSourceFile` helper first tries reading the file directly, then falls
back to using the `filematch` index to resolve the path:

```go file=cmd/check.go
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
```

## Doc context

When a block is invalid or fixable, `docContext` extracts up to 3 lines of
prose from the markdown file immediately above the fence. This helps the reader
understand what the block was documenting without switching to the doc file:

```go file=cmd/check.go
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
```

## Stderr helper

The `errf` function wraps `fmt.Fprintf(os.Stderr, ...)` for concise
styled error output throughout the command:

```go file=cmd/check.go
func errf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
}
```

## Diff rendering

The `renderDiff` function prints a colored diff for invalid code blocks,
showing added lines in green and deleted lines in red:

```go file=cmd/check.go
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
```

## Tab expansion

Source lines may contain tabs, which don't align well when mixed with the
space-based line number prefix. `expandTabs` converts tabs to spaces:

```go file=cmd/check.go
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
```
