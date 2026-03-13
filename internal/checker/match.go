package checker

import "strings"

// NormalizeLines normalizes each line in the slice (strips whitespace, comments).
func NormalizeLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = normalizeLine(l)
	}
	return out
}

// normalizeLine strips leading/trailing whitespace, collapses internal
// whitespace runs to a single space, and removes trailing line comments.
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

// stripTrailingComment removes trailing // and # style comments.
// It respects strings (won't strip // inside "...").
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

// isCommentOnlyLine returns true if the line is entirely a comment
// (after trimming whitespace). Supports // and # style comments.
func isCommentOnlyLine(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#")
}

// stripCommentOnlyLines returns a copy of lines with comment-only lines removed.
// This allows doc blocks to contain extra annotations that don't exist in source.
func stripCommentOnlyLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if !isCommentOnlyLine(l) {
			out = append(out, l)
		}
	}
	return out
}

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

func slicesEqual(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// findContent searches for the block's content in the source file using
// whitespace-normalized comparison. Returns the 0-based start index, or -1.
// normSrc should be the pre-normalized version of srcLines.
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

// findAllContent returns all 0-based offsets where content appears in srcLines.
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

// findBestMatch searches near the original position for a contiguous range
// in the source that is most similar to content. normSrc is the pre-normalized
// source lines. It uses set-intersection similarity (O(m+n) per candidate)
// rather than LCS. Returns 0-based start, end (inclusive), and similarity.
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

// findBestMatchAnywhere searches the entire source file for the contiguous range
// most similar to content, with no positional hint. Used by the no-lines path.
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

// SetIntersectionScore computes similarity as the fraction of lines shared
// between a content multiset and a candidate slice, normalized by the larger size.
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
