# Checker — Warnings

[Back to Overview](overview.md) | Previous: [Auto-fix](checker-fix.md) | Next: [CLI](cli.md)

## Warning suppression

Warnings can be suppressed per-block by placing a `<!-- litcode-ignore <code> -->`
HTML comment above the fence. The supported warning codes are:

```go file=internal/checker/warnings.go
const (
	warningDuplicateComment = "duplicate-comment"
	warningLeadingComment   = "leading-comment"
	warningTopLevelComment  = "top-level-comment"
	warningAll              = "all"
)

var warningIgnoreRegex = regexp.MustCompile(`^<!--\s*litcode-ignore\s+([a-z0-9_,\-\s]+)\s*-->$`)
```

Both warning helpers share a `readDocLines` cache to avoid reading the same
doc file multiple times:

```go file=internal/checker/warnings.go
func readDocLines(cache map[string][]string, path string) []string {
	if lines, ok := cache[path]; ok {
		return lines
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	cache[path] = lines
	return lines
}
```

`warningSuppressed` scans backward from the fence line, skipping blanks, and
checks whether the line above is a `litcode-ignore` comment matching the given code:

```go file=internal/checker/warnings.go
func warningSuppressed(lines []string, fenceLine int, code string) bool {
	if len(lines) == 0 {
		return false
	}
	i := fenceLine - 1
	for i >= 0 && strings.TrimSpace(lines[i]) == "" {
		i--
	}
	if i < 0 {
		return false
	}
	m := warningIgnoreRegex.FindStringSubmatch(strings.TrimSpace(lines[i]))
	if m == nil {
		return false
	}
	for _, part := range strings.Split(m[1], ",") {
		name := strings.ToLower(strings.TrimSpace(part))
		if name == warningAll || name == code {
			return true
		}
	}
	return false
}
```

## Duplicate comment detection

Prose that merely restates a doc comment adds no value and drifts independently,
so the checker flags these duplicates as warnings. The detection runs after all
blocks have been validated:

```go file=internal/checker/checker.go
	result.Warnings = checkDuplicateComments(allBlocks)
```

`checkDuplicateComments` reads each doc file, extracts the paragraph before each
fence, and compares it against leading comment lines in the block:

```go file=internal/checker/warnings.go
func checkDuplicateComments(blocks []markdown.CodeBlock) []Warning {
	// Cache doc file lines.
	docLines := map[string][]string{}

	var warnings []Warning
	for _, block := range blocks {
		lines := readDocLines(docLines, block.DocFile)
		if lines == nil {
			continue
		}
		if warningSuppressed(lines, block.DocLine-1, warningDuplicateComment) {
			continue
		}

		// Extract the paragraph before the fence (non-blank lines above the fence).
		prose := precedingParagraph(lines, block.DocLine-1)
		if prose == "" {
			continue
		}

		// Extract comment text from the block content.
		commentText := extractCommentText(block.Content)
		if commentText == "" {
			continue
		}

		// Compare: if the prose contains most of the comment text, warn.
		if textOverlaps(prose, commentText) {
			warnings = append(warnings, Warning{
				DocFile: block.DocFile,
				DocLine: block.DocLine,
				Message: "prose before code block duplicates comment inside block; comment-only lines don't need doc coverage, so the comment can be omitted from the block or the prose rewritten",
			})
		}
	}
	return warnings
}
```

`precedingParagraph` collects the non-blank lines above the fence into a single
string, skipping any blank lines between the paragraph and the fence:

```go file=internal/checker/warnings.go
func precedingParagraph(lines []string, fenceLine int) string {
	i := fenceLine - 1
	// Skip blank lines between fence and paragraph.
	for i >= 0 && strings.TrimSpace(lines[i]) == "" {
		i--
	}
	var parts []string
	for ; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			break
		}
		parts = append(parts, t)
	}
	// Reverse to get original order.
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, " ")
}
```

`extractCommentText` pulls consecutive comment lines from the start of a block
and strips markers:

```go file=internal/checker/warnings.go
func extractCommentText(content []string) string {
	var parts []string
	for _, line := range content {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "//") {
			parts = append(parts, strings.TrimSpace(strings.TrimPrefix(t, "//")))
		} else if strings.HasPrefix(t, "#") {
			parts = append(parts, strings.TrimSpace(strings.TrimPrefix(t, "#")))
		} else {
			break
		}
	}
	return strings.Join(parts, " ")
}
```

`textOverlaps` uses word-level Jaccard similarity — if at least 50% of the
comment's words appear in the prose, it's a duplicate:

```go file=internal/checker/warnings.go
func textOverlaps(prose, comment string) bool {
	proseWords := wordSet(prose)
	commentWords := wordSet(comment)
	if len(commentWords) == 0 {
		return false
	}
	intersection := 0
	for w := range commentWords {
		if proseWords[w] {
			intersection++
		}
	}
	return float64(intersection)/float64(len(commentWords)) >= 0.5
}

func wordSet(s string) map[string]bool {
	s = strings.ToLower(s)
	// Strip common punctuation.
	for _, c := range []string{".", ",", ":", ";", "`", "'", "\"", "(", ")"} {
		s = strings.ReplaceAll(s, c, "")
	}
	words := strings.Fields(s)
	set := make(map[string]bool, len(words))
	for _, w := range words {
		set[w] = true
	}
	return set
}
```

## Leading comment warnings

The checker also warns when commentary is embedded directly inside a code block.
Leading comment-only lines should become prose above the fence, and a
top-level comment after code has started usually means the block should be
split into smaller blocks with prose between them:

```go file=internal/checker/checker.go
	result.Warnings = append(result.Warnings, checkLeadingComments(allBlocks)...)
```

```go file=internal/checker/warnings.go
func checkLeadingComments(blocks []markdown.CodeBlock) []Warning {
	docLines := map[string][]string{}
	var warnings []Warning
	for _, block := range blocks {
		if strings.EqualFold(block.Language, "mermaid") {
			continue
		}
		lines := readDocLines(docLines, block.DocFile)
		if lines == nil {
			continue
		}
		n := countLeadingCommentLines(block.Content)
		if n > 0 {
			if warningSuppressed(lines, block.DocLine-1, warningLeadingComment) {
				continue
			}
			warnings = append(warnings, Warning{
				DocFile: block.DocFile,
				DocLine: block.DocLine,
				Message: fmt.Sprintf("code block starts with %d comment line(s); move the comment to prose above the code block", n),
			})
			continue
		}

		topLevel := comments.TopLevelCommentLines(block.File, []byte(strings.Join(block.Content, "\n")))
		if len(topLevel) == 0 {
			continue
		}
		// ...
	}
	return warnings
}
```

Directive comments like `//go:build`, `//nolint`, and Python shebang or lint
pragmas are treated specially: they are allowed to stay in the block, and they
do not count as documentation prose. The helper logic is small enough to show
verbatim:

```go file=internal/checker/warnings.go
func countLeadingCommentLines(content []string) int {
	n := 0
	for _, line := range content {
		t := strings.TrimSpace(line)
		if t == "" {
			break
		}
		if !isCommentOnlyLine(t) {
			break
		}
		if isDirective(t) {
			break
		}
		n++
	}
	return n
}
```

`isDirective` recognizes compiler directives and pragmas that belong in code
rather than prose:

```go file=internal/checker/warnings.go
func isDirective(trimmed string) bool {
	if strings.HasPrefix(trimmed, "//go:") ||
		strings.HasPrefix(trimmed, "//nolint") ||
		strings.HasPrefix(trimmed, "//lint:") ||
		strings.HasPrefix(trimmed, "//export") ||
		strings.HasPrefix(trimmed, "#!") ||
		strings.HasPrefix(trimmed, "# type:") ||
		strings.HasPrefix(trimmed, "# noqa") ||
		strings.HasPrefix(trimmed, "# pylint:") ||
		strings.HasPrefix(trimmed, "# -*- ") {
		return true
	}
	return false
}
```

Continue to [CLI](cli.md) to see how the check command is wired up.
