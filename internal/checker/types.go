package checker

// InvalidBlock records a code block whose content doesn't match the source file.
type InvalidBlock struct {
	DocFile    string
	DocLine    int
	SourceFile string
	StartLine  int
	EndLine    int
	Reason     string
	Diff       string
	Fixable    bool   // true if "litcode fix" can resolve this
	FixKind    string // "drift" or "whitespace"
}

// MissingRange records a contiguous range of uncovered non-comment source lines.
type MissingRange struct {
	File      string
	StartLine int
	EndLine   int
}

// FixedBlock records a code block that was automatically fixed.
type FixedBlock struct {
	DocFile string
	DocLine int
	Reason  string
}

// Result holds the outcome of a check.
type Result struct {
	Invalid  []InvalidBlock
	Missing  []MissingRange
	Fixed    []FixedBlock
	Warnings []Warning
}

// Warning is a non-fatal issue detected during checking.
type Warning struct {
	DocFile string
	DocLine int
	Message string
}

// Config controls what Check scans.
type Config struct {
	DocsDirs   []string // file patterns for markdown docs, relative to the current working directory
	SourceDirs []string // file patterns for source files, relative to the current working directory
	Lenient    []string // source globs to validate when referenced but skip from missing-coverage reporting
	Exclude    []string // glob patterns to exclude (matched against project-relative paths)
	Fix        bool     // when true, automatically fix minor mismatches in-place
	Files      []string // when non-empty, only check blocks in these doc files or referencing these source files
}

// DefaultExclude patterns that do not require missing-coverage reporting.
var DefaultExclude = []string{
	"*_test.go",
	"*_test.ts",
	"*_test.js",
	"*_test.py",
	"*_test.rs",
	"*_test.java",
	"*_test.cpp",
	"*_test.c",
	"**/*_test.*",
	"**/test_*",
	"**/testdata/**",
	"**/__tests__/**",
	"**/__test__/**",
	"**/tests/**",
	"**/vendor/**",
	"**/node_modules/**",
}

// DefaultValidationExclude patterns that are skipped entirely, even if
// referenced by documentation. These are external dependencies or fixtures
// rather than user-authored code.
var DefaultValidationExclude = []string{
	"**/testdata/**",
	"**/vendor/**",
	"**/node_modules/**",
}

// DisplayInterval is a range of lines to display, with context merged.
type DisplayInterval struct {
	From, To int
}
