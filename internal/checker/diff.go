package checker

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

func allDeleted(lines []string) string {
	var buf strings.Builder
	for _, l := range lines {
		fmt.Fprintf(&buf, "- %s\n", l)
	}
	return buf.String()
}

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

// MergeDisplayIntervals takes missing ranges and merges their display intervals
// (each range expanded by `context` lines) so overlapping hunks are combined.
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
