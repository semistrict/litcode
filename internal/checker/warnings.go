package checker

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/semistrict/litcode/internal/comments"
	"github.com/semistrict/litcode/internal/markdown"
)

const (
	warningDuplicateComment = "duplicate-comment"
	warningLeadingComment   = "leading-comment"
	warningTopLevelComment  = "top-level-comment"
	warningAll              = "all"
)

var warningIgnoreRegex = regexp.MustCompile(`^<!--\s*litcode-ignore\s+([a-z0-9_,\-\s]+)\s*-->$`)

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

// checkDuplicateComments detects when the markdown prose immediately before a
// code block duplicates a comment inside the block. It reads each doc file once,
// extracts the paragraph before each fence, and compares against comment lines.
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

// precedingParagraph returns the concatenated non-blank lines immediately
// above fenceLine (0-based index), skipping any blank lines between the
// paragraph and the fence, then stopping at the next blank line.
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

// extractCommentText pulls consecutive comment lines from the start of a code
// block, strips the comment markers, and joins them into plain text.
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

// textOverlaps returns true if the prose and comment share enough words to be
// considered duplicates. It uses word-level Jaccard similarity.
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

// checkLeadingComments warns when a code block starts with comment lines or
// contains a top-level comment after code has already started. Leading comments
// should be prose above the block; mid-block top-level comments suggest the
// block should be split into smaller pieces.
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

		seenCode := false
		midBlockCount := 0
		for i, line := range block.Content {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if topLevel[i+1] && isCommentOnlyLine(trimmed) && (len(line) > 0 && line[0] != ' ' && line[0] != '\t') {
				if seenCode && !isDirective(trimmed) {
					midBlockCount++
				}
				continue
			}
			seenCode = true
		}
		if midBlockCount == 0 {
			continue
		}
		if warningSuppressed(lines, block.DocLine-1, warningTopLevelComment) {
			continue
		}

		warnings = append(warnings, Warning{
			DocFile: block.DocFile,
			DocLine: block.DocLine,
			Message: fmt.Sprintf("code block contains %d top-level comment line(s); break the block up and move the comment to prose above the relevant block", midBlockCount),
		})
	}
	return warnings
}

// countLeadingCommentLines returns the number of consecutive comment-only lines
// at the start of content. It skips compiler directives (//go:, //nolint, etc.).
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

// isDirective returns true for lines that look like compiler directives
// or pragmas rather than doc comments.
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
