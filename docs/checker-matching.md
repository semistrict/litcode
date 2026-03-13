# Checker — Matching

[Back to Overview](overview.md) | Previous: [Check Function](checker.md) | Next: [Diff & Ranges](checker-diff.md)

## Comment-only lines

Comment-only lines that exist only in documentation are ignored during
content matching. This lets a doc block include lightweight annotations without
forcing them into the source file:

```go file=internal/checker/match.go
func isCommentOnlyLine(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#")
}
```

`stripCommentOnlyLines` removes comment-only lines so doc blocks can include
extra annotations that don't exist in the source file:

```go file=internal/checker/match.go
func stripCommentOnlyLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if !isCommentOnlyLine(l) {
			out = append(out, l)
		}
	}
	return out
}
```

## Line normalization

Tolerant comparison is essential because documentation formatting often differs
from source -- tabs versus spaces, extra alignment padding, or trailing comments
added for readers. This function canonicalizes a line before comparison:

```go file=internal/checker/match.go
func normalizeLine(s string) string {
	s = stripTrailingComment(s)
	s = strings.TrimSpace(s)
	var b strings.Builder
	inWS := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !inWS {
				b.WriteByte(' ')
				inWS = true
			}
		} else {
			b.WriteRune(r)
			inWS = false
		}
	}
	return strings.TrimRight(b.String(), " ")
}
```

Trailing comments are stripped before normalization so that documentation-only
annotations (like `// added for clarity`) do not cause false mismatches. The
function must track string literal state to avoid stripping inside quotes:

```go file=internal/checker/match.go
func stripTrailingComment(s string) string {
	inString := rune(0)
	escaped := false
	for i, r := range s {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inString != 0 {
			escaped = true
			continue
		}
		if inString != 0 {
			if r == inString {
				inString = 0
			}
			continue
		}
		if r == '"' || r == '\'' || r == '`' {
			inString = r
			continue
		}
		if r == '/' && i+1 < len(s) && rune(s[i+1]) == '/' {
			return s[:i]
		}
		if r == '#' {
			return s[:i]
		}
	}
	return s
}
```

`linesEqual` and `linesEqualNormalized` compare two slices of lines, the latter
using `normalizeLine`:

```go file=internal/checker/match.go
func linesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func linesEqualNormalized(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if normalizeLine(a[i]) != normalizeLine(b[i]) {
			return false
		}
	}
	return true
}
```

## Drift detection

Drift detection relies on scanning the full source file for a match. Accepting
a pre-normalized slice avoids re-normalizing the same file for every block that
references it:

```go file=internal/checker/match.go
func findContent(content, srcLines []string, normSrc []string) int {
	if len(content) == 0 || len(content) > len(srcLines) {
		return -1
	}
	normContent := NormalizeLines(content)
	for i := 0; i <= len(normSrc)-len(normContent); i++ {
		if slicesEqual(normContent, normSrc[i:i+len(normContent)]) {
			return i
		}
	}
	return -1
}
```

Uniqueness matters for two reasons: ambiguous blocks without `lines=` are
flagged as errors, and the fixer strips `lines=` annotations when the content
appears exactly once. This multi-match variant supports both decisions:

```go file=internal/checker/match.go
func findAllContent(content, srcLines []string, normSrc []string) []int {
	if len(content) == 0 || len(content) > len(srcLines) {
		return nil
	}
	normContent := NormalizeLines(content)
	var matches []int
	for i := 0; i <= len(normSrc)-len(normContent); i++ {
		if slicesEqual(normContent, normSrc[i:i+len(normContent)]) {
			matches = append(matches, i)
		}
	}
	return matches
}
```

`NormalizeLines` applies `normalizeLine` to every element:

```go file=internal/checker/match.go
func NormalizeLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = normalizeLine(l)
	}
	return out
}
```

`slicesEqual` is a fast equality check used by `findContent`:

```go file=internal/checker/match.go
func slicesEqual(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

## Similarity-based matching

Full LCS would be too expensive when trying dozens of candidate windows, so
the fuzzy matcher uses a cheaper set-intersection heuristic. It varies both
the window length (to handle insertions and deletions) and position (to handle
drift) around the original location:

```go file=internal/checker/match.go
func findBestMatch(content []string, normSrc []string, origStart int) (bestStart, bestEnd int, bestSim float64) {
	if len(content) == 0 {
		return 0, 0, 0
	}
	normContent := NormalizeLines(content)

	// Build a multiset of content lines (handles duplicate lines correctly).
	contentCounts := make(map[string]int, len(normContent))
	for _, l := range normContent {
		contentCounts[l]++
	}

	contentLen := len(content)
	minLen := contentLen / 2
	if minLen < 1 {
		minLen = 1
	}
	maxLen := contentLen * 3 / 2

	searchStart := origStart - 20
	if searchStart < 0 {
		searchStart = 0
	}
	searchEnd := origStart + 20

	for tryLen := minLen; tryLen <= maxLen; tryLen++ {
		for i := searchStart; i <= searchEnd && i+tryLen <= len(normSrc); i++ {
			sim := SetIntersectionScore(contentCounts, len(normContent), normSrc[i:i+tryLen])
			if sim > bestSim {
				bestSim = sim
				bestStart = i
				bestEnd = i + tryLen - 1
			}
		}
	}
	return
}
```

For blocks without `lines=`, there is no positional hint, so the search covers
the entire file:

```go file=internal/checker/match.go
func findBestMatchAnywhere(content []string, normSrc []string) (bestStart, bestEnd int, bestSim float64) {
	if len(content) == 0 {
		return 0, 0, 0
	}
	normContent := NormalizeLines(content)

	contentCounts := make(map[string]int, len(normContent))
	for _, l := range normContent {
		contentCounts[l]++
	}

	contentLen := len(content)
	minLen := contentLen / 2
	if minLen < 1 {
		minLen = 1
	}
	maxLen := contentLen * 3 / 2

	for tryLen := minLen; tryLen <= maxLen; tryLen++ {
		for i := 0; i+tryLen <= len(normSrc); i++ {
			sim := SetIntersectionScore(contentCounts, len(normContent), normSrc[i:i+tryLen])
			if sim > bestSim {
				bestSim = sim
				bestStart = i
				bestEnd = i + tryLen - 1
			}
		}
	}
	return
}
```

The similarity metric counts how many lines (respecting duplicates) appear in
both the documentation block and a candidate source window, then normalizes
by the larger of the two sizes to penalize length mismatches:

```go file=internal/checker/match.go
func SetIntersectionScore(contentCounts map[string]int, contentLen int, candidate []string) float64 {
	maxLen := contentLen
	if len(candidate) > maxLen {
		maxLen = len(candidate)
	}
	if maxLen == 0 {
		return 1.0
	}
	// Count matches, respecting multiplicity.
	candCounts := make(map[string]int, len(candidate))
	for _, l := range candidate {
		candCounts[l]++
	}
	matches := 0
	for line, cCount := range contentCounts {
		if sCount := candCounts[line]; sCount > 0 {
			if cCount < sCount {
				matches += cCount
			} else {
				matches += sCount
			}
		}
	}
	return float64(matches) / float64(maxLen)
}
```

Continue to [Diff & Ranges](checker-diff.md) to see how diffs and missing ranges
are generated.
