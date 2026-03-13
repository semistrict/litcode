# Checker — Diff & Ranges

[Back to Overview](overview.md) | Previous: [Matching](checker-matching.md) | Next: [Auto-fix](checker-fix.md)

## Diff generation

When a code block doesn't match, `lineDiff` produces a unified-style diff
string using the `github.com/sergi/go-diff/diffmatchpatch` library. It joins
the doc and source lines, computes a line-level diff, and outputs only the
changed lines prefixed with `+` or `-`:

```go file=internal/checker/diff.go
func lineDiff(doc, source []string) string {
	a := strings.Join(doc, "\n") + "\n"
	b := strings.Join(source, "\n") + "\n"
	dmp := diffmatchpatch.New()
	ca, cb, lines := dmp.DiffLinesToChars(a, b)
	diffs := dmp.DiffMain(ca, cb, false)
	diffs = dmp.DiffCharsToLines(diffs, lines)
	diffs = dmp.DiffCleanupSemantic(diffs)
	// Only output changed lines (no context), prefixed with +/-.
	var buf strings.Builder
	for _, d := range diffs {
		if d.Type == diffmatchpatch.DiffEqual {
			continue
		}
		text := strings.TrimSuffix(d.Text, "\n")
		for _, line := range strings.Split(text, "\n") {
			switch d.Type {
			case diffmatchpatch.DiffInsert:
				fmt.Fprintf(&buf, "+ %s\n", line)
			case diffmatchpatch.DiffDelete:
				fmt.Fprintf(&buf, "- %s\n", line)
			}
		}
	}
	return buf.String()
}
```

When an entire block has been deleted from source, a simpler format shows every
line as a deletion:

```go file=internal/checker/diff.go
func allDeleted(lines []string) string {
	var buf strings.Builder
	for _, l := range lines {
		fmt.Fprintf(&buf, "- %s\n", l)
	}
	return buf.String()
}
```

## Range merging

`mergeRanges` takes a sorted list of individual line numbers and collapses
consecutive lines into ranges. This keeps the "MISSING" output minimal:

```go file=internal/checker/diff.go
func mergeRanges(file string, lines []int) []MissingRange {
	if len(lines) == 0 {
		return nil
	}
	sort.Ints(lines)
	var ranges []MissingRange
	start := lines[0]
	end := lines[0]
	for _, l := range lines[1:] {
		if l == end+1 {
			end = l
		} else {
			ranges = append(ranges, MissingRange{File: file, StartLine: start, EndLine: end})
			start = l
			end = l
		}
	}
	ranges = append(ranges, MissingRange{File: file, StartLine: start, EndLine: end})
	return ranges
}
```

## Display intervals

When multiple missing ranges sit close together, printing each with its own
context would produce overlapping or redundant output. These types let the CLI
merge nearby hunks into a single continuous display block:

```go file=internal/checker/types.go
type DisplayInterval struct {
	From, To int
}
```

`MergeDisplayIntervals` expands each range by the context size and merges
overlapping hunks:

```go file=internal/checker/diff.go
func MergeDisplayIntervals(ranges []MissingRange, totalLines, context int) []DisplayInterval {
	var intervals []DisplayInterval
	for _, r := range ranges {
		from := r.StartLine - context
		if from < 1 {
			from = 1
		}
		to := r.EndLine + context
		if to > totalLines {
			to = totalLines
		}
		if len(intervals) > 0 && from <= intervals[len(intervals)-1].To+1 {
			intervals[len(intervals)-1].To = to
		} else {
			intervals = append(intervals, DisplayInterval{from, to})
		}
	}
	return intervals
}
```

Continue to [Auto-fix](checker-fix.md) to see how the fixer rewrites markdown blocks.
