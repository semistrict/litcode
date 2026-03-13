# litcode — A Literate Programming Checker

`litcode` is a command-line tool that validates markdown documentation against
source code. It ensures that every meaningful line of source code is explained
somewhere in a set of markdown files, and that all code blocks in those markdown
files are accurate copies of the source they reference.

## Documentation index

- [Markdown Parsing](markdown-parsing.md) — How code blocks are extracted from documentation
- [Tree-sitter Classification](tree-sitter.md) — Identifying lines that don't need documentation coverage
- **The Checker:**
  - [Types & Configuration](checker-types.md) — Data types and configuration
  - [Check Function](checker.md) — The main validation entry point
  - [Matching](checker-matching.md) — Line normalization, drift detection, and similarity matching
  - [Diff & Ranges](checker-diff.md) — Diff generation, range merging, and display intervals
  - [Auto-fix](checker-fix.md) — Rewriting markdown blocks to fix drift
  - [Warnings](checker-warnings.md) — Duplicate comment detection and leading comment warnings
- [CLI](cli.md) — The `check` command and terminal output

## How it works

The tool operates in three phases:

1. **Parse** — Scan markdown files for fenced code blocks that carry
   `file=` references and optional `lines=` annotations in their info string. See
   [Markdown Parsing](markdown-parsing.md).

2. **Classify** — Use tree-sitter to parse each source file and identify
   lines that don't need documentation coverage (comments, imports, package
   declarations, blank lines). See [Tree-sitter Classification](tree-sitter.md).

3. **Check** — Compare every annotated code block against the actual source,
   then verify that all non-skippable source lines are covered by at least one
   block. See [Check Function](checker.md).

## Entry point

The program starts in `main.go`, which delegates immediately to the
cobra command tree:

```go file=main.go
func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

`cmd.Execute` is a thin wrapper around cobra's root command:

```go file=cmd/root.go
var rootCmd = &cobra.Command{
	Use:   "litcode",
	Short: "Literate programming checker",
	Long:  "Validates that markdown documentation covers all non-comment source lines.",
}
```

```go file=cmd/root.go
func Execute() error {
	return rootCmd.Execute()
}
```

## Subcommands

The main validation entry point is `litcode check`. It accepts `--docs`, `--source`,
and `--exclude` flags — all repeatable. See [CLI](cli.md) for the full
flag definitions and output formatting.

The tool also includes `litcode html-docs` to render the markdown docs as HTML and
`litcode expand-docs` to write a markdown tree with abbreviated code blocks expanded
to full source.

Continue to [Markdown Parsing](markdown-parsing.md) to see how code blocks
are extracted from documentation.
