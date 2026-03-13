package checker_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/semistrict/litcode/internal/checker"
)

func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

func sampleConfig(base string) checker.Config {
	return checker.Config{
		DocsDirs:   []string{filepath.Join(base, "docs")},
		SourceDirs: []string{base},
	}
}

func TestCheck_SampleAllCovered(t *testing.T) {
	cfg := sampleConfig(filepath.Join(testdataDir(), "sample"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	for _, inv := range result.Invalid {
		t.Errorf("INVALID: %s:%d -> %s lines %d-%d — %s",
			inv.DocFile, inv.DocLine, inv.SourceFile, inv.StartLine, inv.EndLine, inv.Reason)
		if inv.Diff != "" {
			t.Log(inv.Diff)
		}
	}
	for _, m := range result.Missing {
		t.Errorf("MISSING: %s lines %d-%d", m.File, m.StartLine, m.EndLine)
	}
}

func TestCheck_MismatchDetected(t *testing.T) {
	cfg := sampleConfig(filepath.Join(testdataDir(), "mismatch"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if len(result.Invalid) != 1 {
		t.Fatalf("expected 1 invalid block, got %d", len(result.Invalid))
	}
	inv := result.Invalid[0]
	if inv.Reason != "content mismatch" {
		t.Errorf("reason = %q, want %q", inv.Reason, "content mismatch")
	}
	if inv.Diff == "" {
		t.Error("expected non-empty diff")
	}
	if inv.Fixable {
		t.Error("genuine mismatch should not be fixable")
	}
}

func TestCheck_MissingLines(t *testing.T) {
	cfg := sampleConfig(filepath.Join(testdataDir(), "partial"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if len(result.Invalid) != 0 {
		t.Errorf("expected 0 invalid blocks, got %d", len(result.Invalid))
	}
	if len(result.Missing) == 0 {
		t.Fatal("expected missing ranges but got none")
	}
	found := false
	for _, m := range result.Missing {
		if m.File == "src/math.go" && m.StartLine == 9 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing range starting at line 9 in src/math.go, got: %+v", result.Missing)
	}
}

func TestCheck_ExcludePattern(t *testing.T) {
	base := filepath.Join(testdataDir(), "partial")
	cfg := checker.Config{
		DocsDirs:   []string{filepath.Join(base, "docs")},
		SourceDirs: []string{base},
		Exclude:    []string{"src/math.go"},
	}
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if len(result.Invalid) != 0 {
		t.Errorf("expected 0 invalid, got %d", len(result.Invalid))
	}
	if len(result.Missing) != 0 {
		t.Errorf("expected 0 missing with exclude, got %d: %+v", len(result.Missing), result.Missing)
	}
}

func TestCheck_TestFilesExcludedByDefault(t *testing.T) {
	base := filepath.Join(testdataDir(), "sample")
	cfg := checker.Config{
		DocsDirs:   []string{filepath.Join(base, "docs")},
		SourceDirs: []string{base},
	}
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	for _, m := range result.Missing {
		if strings.HasSuffix(m.File, "_test.go") {
			t.Errorf("test file should be excluded: %s", m.File)
		}
	}
}

func TestCheck_TestFilesStillValidateWhenReferenced(t *testing.T) {
	cfg := sampleConfig(filepath.Join(testdataDir(), "testfilecheck"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if len(result.Invalid) != 1 {
		t.Fatalf("expected 1 invalid block for referenced test file, got %d", len(result.Invalid))
	}
	if result.Invalid[0].SourceFile != "src/example_test.go" {
		t.Fatalf("invalid block source = %q, want %q", result.Invalid[0].SourceFile, "src/example_test.go")
	}
	if result.Invalid[0].Reason == "" {
		t.Fatal("expected non-empty validation reason for referenced test file")
	}
	if len(result.Missing) != 0 {
		t.Fatalf("expected 0 missing ranges for test file coverage, got %+v", result.Missing)
	}
}

func TestCheck_DriftDetected(t *testing.T) {
	cfg := sampleConfig(filepath.Join(testdataDir(), "drift"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if len(result.Invalid) != 2 {
		t.Fatalf("expected 2 invalid blocks (drifted), got %d", len(result.Invalid))
	}
	for _, inv := range result.Invalid {
		if !inv.Fixable {
			t.Errorf("drifted block should be fixable: %s", inv.Reason)
		}
		if inv.FixKind != "drift" {
			t.Errorf("fix kind = %q, want %q", inv.FixKind, "drift")
		}
		if !strings.Contains(inv.Reason, "found at lines") {
			t.Errorf("reason should mention new location: %s", inv.Reason)
		}
	}
}

func TestCheck_DriftFix(t *testing.T) {
	// Copy drift testdata to a temp dir so we can modify it.
	tmp := t.TempDir()
	copyDir(t, filepath.Join(testdataDir(), "drift"), tmp)

	cfg := checker.Config{
		DocsDirs:   []string{filepath.Join(tmp, "docs")},
		SourceDirs: []string{tmp},
		Fix:        true,
	}
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	// After fix, no invalid blocks should remain (drift was fixed).
	if len(result.Invalid) != 0 {
		t.Errorf("expected 0 invalid after fix, got %d", len(result.Invalid))
		for _, inv := range result.Invalid {
			t.Logf("  %s:%d %s — %s", inv.DocFile, inv.DocLine, inv.SourceFile, inv.Reason)
		}
	}

	// Verify the doc was updated: lines= should be stripped since content is unique.
	data, err := os.ReadFile(filepath.Join(tmp, "docs", "example.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Contains(content, "lines=") {
		t.Errorf("expected lines= to be stripped (content is unique), got:\n%s", content)
	}
}

func TestCheck_WhitespaceTolerated(t *testing.T) {
	// Whitespace differences (leading, trailing, internal) should be
	// silently tolerated — no invalid blocks reported.
	cfg := sampleConfig(filepath.Join(testdataDir(), "wsfix"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(result.Invalid) != 0 {
		t.Errorf("expected 0 invalid (whitespace tolerated), got %d", len(result.Invalid))
		for _, inv := range result.Invalid {
			t.Logf("  %s:%d %s — %s", inv.DocFile, inv.DocLine, inv.SourceFile, inv.Reason)
		}
	}
}

func TestCheck_GenuineMismatchNotFixable(t *testing.T) {
	cfg := sampleConfig(filepath.Join(testdataDir(), "mismatch"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	for _, inv := range result.Invalid {
		if inv.Fixable {
			t.Error("genuine content mismatch should not be fixable")
		}
	}
}

func TestCheck_CommentAndWhitespaceTolerated(t *testing.T) {
	// Differences in leading whitespace, trailing whitespace, and
	// presence/absence of trailing comments should be tolerated.
	cfg := sampleConfig(filepath.Join(testdataDir(), "commentdiff"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(result.Invalid) != 0 {
		t.Errorf("expected 0 invalid, got %d", len(result.Invalid))
		for _, inv := range result.Invalid {
			t.Logf("  %s:%d %s lines %d-%d — %s", inv.DocFile, inv.DocLine, inv.SourceFile, inv.StartLine, inv.EndLine, inv.Reason)
			if len(inv.Diff) > 0 {
				t.Logf("%+v", inv.Diff)
			}
		}
	}
}

func TestCheck_MermaidDiagramCoversReferencedLines(t *testing.T) {
	cfg := sampleConfig(filepath.Join(testdataDir(), "mermaid"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if len(result.Invalid) != 0 {
		t.Fatalf("expected 0 invalid blocks, got %d: %+v", len(result.Invalid), result.Invalid)
	}
	if len(result.Missing) != 0 {
		t.Fatalf("expected 0 missing ranges, got %d: %+v", len(result.Missing), result.Missing)
	}
}

func TestCheck_AllowsAbbreviatedMiddleLines(t *testing.T) {
	cfg := sampleConfig(filepath.Join(testdataDir(), "ellipsis"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if len(result.Invalid) != 0 {
		t.Fatalf("expected 0 invalid blocks, got %d: %+v", len(result.Invalid), result.Invalid)
	}
	if len(result.Missing) != 0 {
		t.Fatalf("expected 0 missing ranges, got %d: %+v", len(result.Missing), result.Missing)
	}
}

func TestCheck_AllowsAbbreviatedMiddleLinesWithoutLinesAnnotation(t *testing.T) {
	cfg := sampleConfig(filepath.Join(testdataDir(), "ellipsis_nolines"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if len(result.Invalid) != 0 {
		t.Fatalf("expected 0 invalid blocks, got %d: %+v", len(result.Invalid), result.Invalid)
	}
	if len(result.Missing) != 0 {
		t.Fatalf("expected 0 missing ranges, got %d: %+v", len(result.Missing), result.Missing)
	}
}

func TestMergeDisplayIntervals(t *testing.T) {
	tests := []struct {
		name     string
		ranges   []checker.MissingRange
		total    int
		context  int
		expected []checker.DisplayInterval
	}{
		{
			name:     "single range",
			ranges:   []checker.MissingRange{{File: "f", StartLine: 10, EndLine: 12}},
			total:    100,
			context:  3,
			expected: []checker.DisplayInterval{{From: 7, To: 15}},
		},
		{
			name: "two disjoint ranges",
			ranges: []checker.MissingRange{
				{File: "f", StartLine: 5, EndLine: 5},
				{File: "f", StartLine: 20, EndLine: 22},
			},
			total:   100,
			context: 3,
			expected: []checker.DisplayInterval{
				{From: 2, To: 8},
				{From: 17, To: 25},
			},
		},
		{
			name: "overlapping context merges",
			ranges: []checker.MissingRange{
				{File: "f", StartLine: 5, EndLine: 5},
				{File: "f", StartLine: 10, EndLine: 10},
			},
			total:   100,
			context: 3,
			expected: []checker.DisplayInterval{
				{From: 2, To: 13},
			},
		},
		{
			name: "adjacent context merges",
			ranges: []checker.MissingRange{
				{File: "f", StartLine: 5, EndLine: 5},
				{File: "f", StartLine: 12, EndLine: 12},
			},
			total:   100,
			context: 3,
			expected: []checker.DisplayInterval{
				{From: 2, To: 15},
			},
		},
		{
			name: "three ranges two merge one separate",
			ranges: []checker.MissingRange{
				{File: "f", StartLine: 5, EndLine: 5},
				{File: "f", StartLine: 8, EndLine: 8},
				{File: "f", StartLine: 50, EndLine: 52},
			},
			total:   100,
			context: 3,
			expected: []checker.DisplayInterval{
				{From: 2, To: 11},
				{From: 47, To: 55},
			},
		},
		{
			name:     "clamps to file boundaries",
			ranges:   []checker.MissingRange{{File: "f", StartLine: 1, EndLine: 2}},
			total:    5,
			context:  3,
			expected: []checker.DisplayInterval{{From: 1, To: 5}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checker.MergeDisplayIntervals(tt.ranges, tt.total, tt.context)
			if len(got) != len(tt.expected) {
				t.Fatalf("got %d intervals, want %d: %+v", len(got), len(tt.expected), got)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("interval %d: got %+v, want %+v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestCheck_DriftNoDiff(t *testing.T) {
	// When content drifts to a new location, the diff should be empty.
	// Previously it showed a misleading diff comparing doc content against
	// whatever happened to be at the old line numbers.
	cfg := sampleConfig(filepath.Join(testdataDir(), "drift"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, inv := range result.Invalid {
		if inv.FixKind == "drift" && inv.Diff != "" {
			t.Errorf("drift block at %s:%d should have no diff, got:\n%s",
				inv.DocFile, inv.DocLine, inv.Diff)
		}
	}
}

func TestCheck_DuplicateCommentWarning(t *testing.T) {
	// When prose before a code block duplicates a comment inside the block,
	// the checker should emit a warning.
	cfg := sampleConfig(filepath.Join(testdataDir(), "dupcomment"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	// The first block has prose "greet returns a greeting for the given name"
	// which duplicates the comment "// greet returns a greeting for the given name."
	found := false
	for _, w := range result.Warnings {
		if w.DocLine == 5 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate comment warning at doc line 5, got warnings: %+v", result.Warnings)
	}
	// The second block's prose "This is unique prose..." does NOT duplicate
	// the comment, so no duplicate-comment warning for that block.
	for _, w := range result.Warnings {
		if w.DocLine == 14 && strings.Contains(w.Message, "duplicates comment") {
			t.Errorf("unexpected duplicate-comment warning for non-duplicate block at line 14: %s", w.Message)
		}
	}
}

func TestCheck_TopLevelCommentInsideBlockWarning(t *testing.T) {
	cfg := sampleConfig(filepath.Join(testdataDir(), "toplevelcomment"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	foundTopLevel := false
	for _, w := range result.Warnings {
		if w.DocLine == 5 && strings.Contains(w.Message, "break the block up") {
			foundTopLevel = true
		}
		if w.DocLine == 13 && strings.Contains(w.Message, "break the block up") {
			t.Fatalf("unexpected top-level comment warning for nested comment block: %+v", w)
		}
	}
	if !foundTopLevel {
		t.Fatalf("expected top-level comment warning at doc line 5, got warnings: %+v", result.Warnings)
	}
}

func TestCheck_WarningSuppressions(t *testing.T) {
	cfg := sampleConfig(filepath.Join(testdataDir(), "warningsuppress"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	for _, w := range result.Warnings {
		if w.DocLine == 7 {
			t.Fatalf("expected block at doc line 6 to suppress duplicate/leading warnings, got: %+v", w)
		}
		if w.DocLine == 18 {
			t.Fatalf("expected block at doc line 18 to suppress top-level comment warnings, got: %+v", w)
		}
	}

	foundUnsuppressed := false
	for _, w := range result.Warnings {
		if w.DocLine == 32 && strings.Contains(w.Message, "comment line") {
			foundUnsuppressed = true
		}
	}
	if !foundUnsuppressed {
		t.Fatalf("expected unsuppressed warning at doc line 32, got warnings: %+v", result.Warnings)
	}
}

func TestCheck_DocOnlyComments_WithLines(t *testing.T) {
	// Doc blocks may contain comment-only lines that don't exist in source.
	// These should be ignored during comparison.
	cfg := sampleConfig(filepath.Join(testdataDir(), "doccomments"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(result.Invalid) != 0 {
		for _, inv := range result.Invalid {
			t.Errorf("unexpected invalid: %s:%d %s — %s", inv.DocFile, inv.DocLine, inv.SourceFile, inv.Reason)
		}
	}
}

func TestCheck_DocOnlyComments_NoLines(t *testing.T) {
	// Same as above but for the no-lines path (fuzzy find).
	cfg := sampleConfig(filepath.Join(testdataDir(), "doccomments"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	// The farewell block has no lines= and a doc-only comment.
	// It should match uniquely.
	for _, inv := range result.Invalid {
		if strings.Contains(inv.Reason, "farewell") || (inv.DocLine == 13) {
			t.Errorf("farewell block should be valid, got: %s:%d — %s", inv.DocFile, inv.DocLine, inv.Reason)
		}
	}
}

func TestCheck_DocOnlyExplanationLinesAreIgnoredForMatching(t *testing.T) {
	tmp := t.TempDir()
	docsDir := filepath.Join(tmp, "docs")
	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	src := "def greet(name):\n    return f\"hello {name}\"\n"
	if err := os.WriteFile(filepath.Join(srcDir, "example.py"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	doc := "# Example\n\n" +
		"```python file=src/example.py lines=1-2\n" +
		"def greet(name):\n" +
		"    # This note is for readers and should be ignored when matching.\n" +
		"    return f\"hello {name}\"\n" +
		"```\n"
	if err := os.WriteFile(filepath.Join(docsDir, "example.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := checker.Config{
		DocsDirs:   []string{docsDir},
		SourceDirs: []string{tmp},
	}
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(result.Invalid) != 0 {
		t.Fatalf("expected 0 invalid blocks, got %d: %+v", len(result.Invalid), result.Invalid)
	}
	if len(result.Missing) != 0 {
		t.Fatalf("expected 0 missing ranges, got %d: %+v", len(result.Missing), result.Missing)
	}
}

func TestCheck_LeadingCommentWarning(t *testing.T) {
	tmp := t.TempDir()
	docsDir := filepath.Join(tmp, "docs")
	srcDir := filepath.Join(tmp, "src")
	os.MkdirAll(docsDir, 0o755)
	os.MkdirAll(srcDir, 0o755)

	src := "// greet returns a greeting.\nfunc greet(name string) string {\n\treturn \"hello \" + name\n}\n\nfunc farewell() string {\n\treturn \"bye\"\n}\n"
	os.WriteFile(filepath.Join(srcDir, "example.go"), []byte(src), 0o644)

	doc := "# Example\n\n" +
		"Block with leading comment:\n\n" +
		"```go file=src/example.go lines=1-4\n" +
		"// greet returns a greeting.\n" +
		"func greet(name string) string {\n" +
		"\treturn \"hello \" + name\n" +
		"}\n" +
		"```\n\n" +
		"Block without leading comment:\n\n" +
		"```go file=src/example.go lines=6-8\n" +
		"func farewell() string {\n" +
		"\treturn \"bye\"\n" +
		"}\n" +
		"```\n"
	os.WriteFile(filepath.Join(docsDir, "example.md"), []byte(doc), 0o644)

	cfg := checker.Config{
		DocsDirs:   []string{docsDir},
		SourceDirs: []string{tmp},
	}
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	// The first block starts with a comment — should warn.
	foundLeading := false
	for _, w := range result.Warnings {
		if w.DocLine == 5 && strings.Contains(w.Message, "comment line") {
			foundLeading = true
		}
	}
	if !foundLeading {
		t.Errorf("expected leading-comment warning at doc line 5, got warnings: %+v", result.Warnings)
	}

	// The second block does NOT start with a comment — no leading-comment warning.
	for _, w := range result.Warnings {
		if w.DocLine == 13 && strings.Contains(w.Message, "comment line") {
			t.Errorf("unexpected leading-comment warning at doc line 13: %s", w.Message)
		}
	}
}

func TestCheck_LeadingCommentWarning_SkipsDirectives(t *testing.T) {
	tmp := t.TempDir()
	docsDir := filepath.Join(tmp, "docs")
	srcDir := filepath.Join(tmp, "src")
	os.MkdirAll(docsDir, 0o755)
	os.MkdirAll(srcDir, 0o755)

	src := "//go:build linux\n\npackage foo\n"
	os.WriteFile(filepath.Join(srcDir, "example.go"), []byte(src), 0o644)

	doc := "# Example\n\n" +
		"Build constraint:\n\n" +
		"```go file=src/example.go lines=1-3\n" +
		"//go:build linux\n" +
		"\n" +
		"package foo\n" +
		"```\n"
	os.WriteFile(filepath.Join(docsDir, "example.md"), []byte(doc), 0o644)

	cfg := checker.Config{
		DocsDirs:   []string{docsDir},
		SourceDirs: []string{tmp},
	}
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	for _, w := range result.Warnings {
		if strings.Contains(w.Message, "comment line") {
			t.Errorf("should not warn on directive lines, got: %s", w.Message)
		}
	}
}

func TestSetIntersectionScore_DifferentStructsNotSimilar(t *testing.T) {
	// Two completely different Go structs should NOT be considered similar,
	// even though they share boilerplate like "}", field indentation, etc.
	content := []string{
		"type MissingRange struct {",
		"\tFile      string",
		"\tStartLine int",
		"\tEndLine   int",
		"}",
	}
	candidate := []string{
		"type Result struct {",
		"\tInvalid []InvalidBlock",
		"\tMissing []MissingRange",
		"\tFixed   []FixedBlock",
		"}",
	}
	contentCounts := make(map[string]int)
	for _, l := range checker.NormalizeLines(content) {
		contentCounts[l]++
	}
	score := checker.SetIntersectionScore(contentCounts, len(content), checker.NormalizeLines(candidate))
	if score >= 0.70 {
		t.Errorf("different structs scored %.2f, should be below 0.70", score)
	}
}

func TestSetIntersectionScore_SameStructSimilar(t *testing.T) {
	// Same struct with a minor field edit should be similar.
	content := []string{
		"type Config struct {",
		"\tDocsDirs   []string",
		"\tSourceDirs []string",
		"\tExclude    []string",
		"}",
	}
	candidate := []string{
		"type Config struct {",
		"\tDocsDirs   []string",
		"\tSourceDirs []string",
		"\tExclude    []string",
		"\tFix        bool",
		"}",
	}
	contentCounts := make(map[string]int)
	for _, l := range checker.NormalizeLines(content) {
		contentCounts[l]++
	}
	score := checker.SetIntersectionScore(contentCounts, len(content), checker.NormalizeLines(candidate))
	if score < 0.70 {
		t.Errorf("similar structs scored %.2f, should be >= 0.70", score)
	}
}

func TestCheck_NoLinesFuzzyMatch(t *testing.T) {
	// A no-lines block with a one-line change should report as a minor edit
	// with a focused diff, not "content not found" with all lines deleted.
	cfg := sampleConfig(filepath.Join(testdataDir(), "nolines-minor-edit"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if len(result.Invalid) != 1 {
		t.Fatalf("expected 1 invalid block, got %d", len(result.Invalid))
	}
	inv := result.Invalid[0]
	if strings.Contains(inv.Reason, "content not found") {
		t.Errorf("should not report 'content not found' for a minor edit, got: %s", inv.Reason)
	}
	if !strings.Contains(inv.Reason, "similar") {
		t.Errorf("reason should mention similarity, got: %s", inv.Reason)
	}
	// The diff should be focused (not all lines deleted).
	if strings.Count(inv.Diff, "\n") > 4 {
		t.Errorf("diff should be focused (few lines), got:\n%s", inv.Diff)
	}
}

func TestCheck_NoLinesClosestMatchDiff(t *testing.T) {
	// A block whose content has diverged significantly should still show
	// a diff against the best match, not just "- every line".
	cfg := sampleConfig(filepath.Join(testdataDir(), "nolines-diverged"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if len(result.Invalid) != 1 {
		t.Fatalf("expected 1 invalid block, got %d", len(result.Invalid))
	}
	inv := result.Invalid[0]
	// Should have a diff with both + and - lines (comparative diff),
	// not just all - lines.
	if inv.Diff == "" {
		t.Fatal("expected non-empty diff")
	}
	hasPlus := strings.Contains(inv.Diff, "+ ")
	hasMinus := strings.Contains(inv.Diff, "- ")
	if !hasPlus || !hasMinus {
		t.Errorf("diff should show both additions and deletions (comparative diff), got:\n%s", inv.Diff)
	}
}

func TestCheck_InvalidBlockSuppressesMissing(t *testing.T) {
	// Lines covered by an INVALID block (with lines=) should not also
	// appear in result.Missing. The user already knows about the problem.
	cfg := sampleConfig(filepath.Join(testdataDir(), "mismatch"))
	result, err := checker.Check(cfg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	// The mismatch testdata has lines 5-7 as invalid.
	// Those lines should NOT appear in Missing.
	for _, m := range result.Missing {
		if strings.HasSuffix(m.File, "hello.go") {
			for l := m.StartLine; l <= m.EndLine; l++ {
				if l >= 5 && l <= 7 {
					t.Errorf("line %d is in an INVALID block and should not appear in Missing", l)
				}
			}
		}
	}
}

func repoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func BenchmarkCheck_Repo(b *testing.B) {
	root := repoRoot()
	cfg := checker.Config{
		DocsDirs:   []string{filepath.Join(root, "docs")},
		SourceDirs: []string{root},
	}
	for b.Loop() {
		_, err := checker.Check(cfg)
		if err != nil {
			b.Fatalf("Check: %v", err)
		}
	}
}

// copyDir recursively copies src to dst.
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Fatal(err)
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			os.MkdirAll(target, 0o755)
		} else {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			os.WriteFile(target, data, 0o644)
		}
		return nil
	})
}
