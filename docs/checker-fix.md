# Checker — Auto-fix

[Back to Overview](overview.md) | Previous: [Diff & Ranges](checker-diff.md) | Next: [Warnings](checker-warnings.md)

Each pending fix must update two things in the markdown: the `lines=`
annotation on the fence line (or remove it if the content is unique) and the
block body itself:

```go file=internal/checker/fix.go
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

	if err := os.WriteFile(docFile, []byte(strings.Join(rebuilt, "\n")), 0o644); err != nil {
		return
	}
}
```

`updateLinesAnnotation` finds and replaces the `lines=N-M` portion of a fence
line:

```go file=internal/checker/fix.go
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
```

`stripLinesAnnotation` removes the `lines=` annotation entirely, used when
content is unique and line numbers are unnecessary:

```go file=internal/checker/fix.go
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
```

Continue to [Warnings](checker-warnings.md) to see how duplicate comments and
leading comment warnings work.
