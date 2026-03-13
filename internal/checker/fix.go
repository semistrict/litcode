package checker

import (
	"fmt"
	"os"
	"strings"
)

// rewriteBlock modifies a markdown file in place, updating the code block
// that starts at docLine. It replaces the lines= annotation and the block
// content.
func rewriteBlock(docFile string, docLine int, oldContent, newContent []string, newStart, newEnd int, removeLines bool) {
	data, err := os.ReadFile(docFile)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	if docLine < 1 || docLine > len(lines) {
		return
	}

	// Update or remove the lines= annotation on the fence line.
	fenceLine := lines[docLine-1]
	if removeLines {
		fenceLine = stripLinesAnnotation(fenceLine)
	} else {
		fenceLine = updateLinesAnnotation(fenceLine, newStart, newEnd)
	}
	lines[docLine-1] = fenceLine

	// Replace content lines (they start at docLine+1, one per old content line).
	contentStart := docLine // 0-based index of first content line
	contentEnd := contentStart + len(oldContent)
	if contentEnd > len(lines) {
		return
	}

	var rebuilt []string
	rebuilt = append(rebuilt, lines[:contentStart]...)
	rebuilt = append(rebuilt, newContent...)
	rebuilt = append(rebuilt, lines[contentEnd:]...)

	os.WriteFile(docFile, []byte(strings.Join(rebuilt, "\n")), 0o644)
}

func stripLinesAnnotation(fenceLine string) string {
	idx := strings.Index(fenceLine, " lines=")
	if idx < 0 {
		idx = strings.Index(fenceLine, "\tlines=")
	}
	if idx < 0 {
		return fenceLine
	}
	rest := fenceLine[idx+1:] // skip the space/tab before "lines="
	end := strings.IndexFunc(rest, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '`'
	})
	if end < 0 {
		return fenceLine[:idx]
	}
	return fenceLine[:idx] + rest[end:]
}

func updateLinesAnnotation(fenceLine string, newStart, newEnd int) string {
	// Find and replace the lines=N-M or lines=N portion.
	idx := strings.Index(fenceLine, "lines=")
	if idx < 0 {
		return fenceLine
	}
	// Find the end of the lines= value.
	rest := fenceLine[idx:]
	end := strings.IndexFunc(rest, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '`'
	})
	if end < 0 {
		end = len(rest)
	}
	var newAnnotation string
	if newStart == newEnd {
		newAnnotation = fmt.Sprintf("lines=%d", newStart)
	} else {
		newAnnotation = fmt.Sprintf("lines=%d-%d", newStart, newEnd)
	}
	return fenceLine[:idx] + newAnnotation + rest[end:]
}
