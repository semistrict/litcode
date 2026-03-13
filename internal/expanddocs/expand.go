package expanddocs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/semistrict/litcode/internal/filematch"
	"github.com/semistrict/litcode/internal/markdown"
)

// ExpandAbbreviatedBlock expands a code block that omits a single contiguous
// middle section using a comment-only ellipsis marker like `// ...` or `# ...`.
// It returns the full source lines for the uniquely matched range.
func ExpandAbbreviatedBlock(block markdown.CodeBlock, srcLines []string) ([]string, bool, error) {
	marker := -1
	for i, line := range block.Content {
		if isOmissionMarker(line) {
			if marker >= 0 {
				return nil, true, fmt.Errorf("abbreviated block may contain only one omission marker")
			}
			marker = i
		}
	}
	if marker < 0 {
		return nil, false, nil
	}
	if marker == 0 || marker == len(block.Content)-1 {
		return nil, true, fmt.Errorf("omission marker must appear between a real prefix and suffix")
	}
	prefix := block.Content[:marker]
	suffix := block.Content[marker+1:]
	if block.StartLine != 0 {
		if block.StartLine < 1 || block.EndLine > len(srcLines) {
			return nil, true, fmt.Errorf("line range %d-%d out of bounds (file has %d lines)", block.StartLine, block.EndLine, len(srcLines))
		}
		actual := srcLines[block.StartLine-1 : block.EndLine]
		if err := validateAbbreviatedMatch(prefix, suffix, actual, block.StartLine, block.EndLine); err != nil {
			return nil, true, err
		}
		expanded := make([]string, len(actual))
		copy(expanded, actual)
		return expanded, true, nil
	}

	matches := findAbbreviatedMatches(prefix, suffix, srcLines)
	switch len(matches) {
	case 0:
		return nil, true, fmt.Errorf("abbreviated block content not found in source file")
	case 1:
		actual := srcLines[matches[0].start : matches[0].end+1]
		expanded := make([]string, len(actual))
		copy(expanded, actual)
		return expanded, true, nil
	default:
		locs := make([]string, len(matches))
		for i, m := range matches {
			locs[i] = fmt.Sprintf("%d-%d", m.start+1, m.end+1)
		}
		return nil, true, fmt.Errorf("abbreviated block is ambiguous: matches at lines %s", strings.Join(locs, ", "))
	}
}

// ExpandedMarkdown returns the markdown file content with abbreviated blocks
// expanded to the full source lines they reference.
func ExpandedMarkdown(docPath string, sourceDirs []string) ([]byte, error) {
	blocks, err := markdown.ParseFile(docPath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(docPath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")

	sourceIndex, err := filematch.Index(sourceDirs)
	if err != nil {
		return nil, fmt.Errorf("collecting source files: %w", err)
	}

	resolveCache := make(map[string]string)
	linesCache := make(map[string][]string)

	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].DocLine > blocks[j].DocLine
	})

	for _, block := range blocks {
		srcPath, found := func() (string, bool) {
			if cached, ok := resolveCache[block.File]; ok {
				return cached, cached != ""
			}
			p, ok := resolveSourceFile(block.File, sourceIndex)
			if ok {
				resolveCache[block.File] = p
			} else {
				resolveCache[block.File] = ""
			}
			return p, ok
		}()
		if !found {
			return nil, fmt.Errorf("%s:%d: file not found in any source directory: %s", docPath, block.DocLine, block.File)
		}

		srcLines, ok := linesCache[srcPath]
		if !ok {
			srcLines, err = readLines(srcPath)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: cannot read source file %s: %w", docPath, block.DocLine, block.File, err)
			}
			linesCache[srcPath] = srcLines
		}

		expanded, abbreviated, err := ExpandAbbreviatedBlock(block, srcLines)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", docPath, block.DocLine, err)
		}
		if !abbreviated {
			continue
		}

		contentStart := block.DocLine
		contentEnd := contentStart + len(block.Content)
		lines = append(append(lines[:contentStart], expanded...), lines[contentEnd:]...)
	}

	return []byte(strings.Join(lines, "\n")), nil
}

// ExpandTree expands all markdown files under srcDir into outDir, preserving
// the relative directory structure.
func ExpandTree(srcDir, outDir string, sourceDirs []string) error {
	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return err
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(absOut, 0o755); err != nil {
		return err
	}

	return filepath.Walk(absSrc, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		rel, err := filepath.Rel(absSrc, path)
		if err != nil {
			return err
		}
		outPath := filepath.Join(absOut, rel)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}

		expanded, err := ExpandedMarkdown(path, sourceDirs)
		if err != nil {
			return err
		}
		return os.WriteFile(outPath, expanded, 0o644)
	})
}

func resolveSourceFile(file string, sourceIndex map[string]string) (string, bool) {
	path, ok := sourceIndex[filepath.ToSlash(filepath.Clean(file))]
	return path, ok
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

func isOmissionMarker(line string) bool {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "//"):
		return isEllipsis(strings.TrimSpace(strings.TrimPrefix(trimmed, "//")))
	case strings.HasPrefix(trimmed, "#"):
		return isEllipsis(strings.TrimSpace(strings.TrimPrefix(trimmed, "#")))
	case strings.HasPrefix(trimmed, "/*") && strings.HasSuffix(trimmed, "*/"):
		body := strings.TrimSuffix(strings.TrimPrefix(trimmed, "/*"), "*/")
		return isEllipsis(strings.TrimSpace(body))
	default:
		return false
	}
}

func isEllipsis(s string) bool {
	return s == "..." || s == "…"
}

type abbreviatedMatch struct {
	start int
	end   int
}

func findAbbreviatedMatches(prefix, suffix, srcLines []string) []abbreviatedMatch {
	if len(prefix) == 0 || len(suffix) == 0 {
		return nil
	}
	var matches []abbreviatedMatch
	for start := 0; start < len(srcLines); start++ {
		if start+len(prefix) > len(srcLines) {
			break
		}
		if !linesEqualNormalized(prefix, srcLines[start:start+len(prefix)]) {
			continue
		}
		minSuffixStart := start + len(prefix) + 1
		for suffixStart := minSuffixStart; suffixStart+len(suffix) <= len(srcLines); suffixStart++ {
			if !linesEqualNormalized(suffix, srcLines[suffixStart:suffixStart+len(suffix)]) {
				continue
			}
			matches = append(matches, abbreviatedMatch{
				start: start,
				end:   suffixStart + len(suffix) - 1,
			})
		}
	}
	return matches
}

func validateAbbreviatedMatch(prefix, suffix, actual []string, startLine, endLine int) error {
	if len(actual) < len(prefix)+len(suffix)+1 {
		return fmt.Errorf("abbreviated block omits too little content for lines %d-%d", startLine, endLine)
	}
	if !linesEqualNormalized(prefix, actual[:len(prefix)]) {
		return fmt.Errorf("abbreviated block prefix does not match referenced source")
	}
	if !linesEqualNormalized(suffix, actual[len(actual)-len(suffix):]) {
		return fmt.Errorf("abbreviated block suffix does not match referenced source")
	}
	return nil
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
