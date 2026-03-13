# Checker — Types & Configuration

[Back to Overview](overview.md) | Previous: [Tree-sitter Classification](tree-sitter.md) | Next: [Check Function](checker.md)

## Data types

The checker reports two kinds of problems. An `InvalidBlock` means a code block
in the documentation doesn't match the source file it claims to reference:

```go file=internal/checker/types.go
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
```

A `MissingRange` means a contiguous run of non-comment source lines that aren't
covered by any code block:

```go file=internal/checker/types.go
type MissingRange struct {
	File      string
	StartLine int
	EndLine   int
}
```

When `litcode fix` resolves a mismatch, it creates a `FixedBlock` so the CLI can report what changed:

```go file=internal/checker/types.go
type FixedBlock struct {
	DocFile string
	DocLine int
	Reason  string
}
```

All three are collected into a `Result`:

```go file=internal/checker/types.go
type Result struct {
	Invalid  []InvalidBlock
	Missing  []MissingRange
	Fixed    []FixedBlock
	Warnings []Warning
}
```

A `Warning` is a non-fatal issue detected during checking:

```go file=internal/checker/types.go
type Warning struct {
	DocFile string
	DocLine int
	Message string
}
```

## Configuration

The `Config` struct controls what gets scanned:

```go file=internal/checker/types.go
type Config struct {
	DocsDirs   []string // file patterns for markdown docs, relative to the current working directory
	SourceDirs []string // file patterns for source files, relative to the current working directory
	Lenient    []string // source globs to validate when referenced but skip from missing-coverage reporting
	Exclude    []string // glob patterns to exclude (matched against project-relative paths)
	Fix        bool     // when true, automatically fix minor mismatches in-place
	Files      []string // when non-empty, only check blocks in these doc files or referencing these source files
}
```

A set of default exclusions prevents test files, test fixtures, vendored
dependencies, and node_modules from requiring missing-coverage reporting:

```go file=internal/checker/types.go
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
```

Validation uses a narrower default list. Test code can still be referenced and
checked for drift or mismatches, but fixtures and external dependencies are
skipped entirely:

```go file=internal/checker/types.go
var DefaultValidationExclude = []string{
	"**/testdata/**",
	"**/vendor/**",
	"**/node_modules/**",
}
```

Continue to [Check Function](checker.md) to see how the main validation loop works.
