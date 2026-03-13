package checkdiff

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDiff(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/src/main.go b/src/main.go",
		"index 1111111..2222222 100644",
		"--- a/src/main.go",
		"+++ b/src/main.go",
		"@@ -3,3 +3,3 @@ func OldThing() string {",
		"-func OldThing() string {",
		"-    return \"old\"",
		"-}",
		"+func NewThing() string {",
		"+    return \"new\"",
		"+}",
		"",
	}, "\n")

	parsed, err := parseDiff(diff)
	if err != nil {
		t.Fatalf("parseDiff: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("len(parsed) = %d, want 1", len(parsed))
	}
	if got := parsed[0].Added[0]; got.Start != 3 || got.End != 5 {
		t.Fatalf("added range = %+v, want 3-5", got)
	}
	if got := parsed[0].Removed[0]; got.File != "src/main.go" || got.Symbol != "OldThing" {
		t.Fatalf("removed chunk = %+v, want src/main.go / OldThing", got)
	}
}

func TestHasRemovalMention_IgnoresCodeBlocks(t *testing.T) {
	doc := filepath.Join(t.TempDir(), "change.md")
	writeFile(t, doc, strings.Join([]string{
		"```text",
		"Removed OldThing in src/main.go",
		"```",
		"",
		"Real prose with no removal summary.",
		"",
	}, "\n"))

	paragraphs, err := collectProseParagraphs([]string{doc})
	if err != nil {
		t.Fatalf("collectProseParagraphs: %v", err)
	}
	if hasRemovalMention(paragraphs, "src/main.go", "OldThing") {
		t.Fatal("code-fence mention should not satisfy removal summary")
	}
}

func TestCheck_ExternalDocsPass(t *testing.T) {
	repoDir, docsDir := setupRepo(t)
	t.Chdir(repoDir)

	writeFile(t, filepath.Join(repoDir, "src", "main.go"), strings.Join([]string{
		"package main",
		"",
		"func NewThing() string {",
		"\treturn \"new\"",
		"}",
		"",
	}, "\n"))

	doc := filepath.Join(docsDir, "change.md")
	writeFile(t, doc, strings.Join([]string{
		"Removed OldThing in src/main.go.",
		"",
		"```go file=src/main.go lines=3-5",
		"func NewThing() string {",
		"\treturn \"new\"",
		"}",
		"```",
		"",
	}, "\n"))

	result, err := Check(Config{
		DocsDirs:   []string{doc},
		SourceDirs: []string{"src/**/*.go"},
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(result.Invalid) != 0 || len(result.MissingAdded) != 0 || len(result.MissingRemoved) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCheck_MissingAddedCoverage(t *testing.T) {
	repoDir, docsDir := setupRepo(t)
	t.Chdir(repoDir)

	writeFile(t, filepath.Join(repoDir, "src", "main.go"), strings.Join([]string{
		"package main",
		"",
		"func NewThing() string {",
		"\treturn \"new\"",
		"}",
		"",
	}, "\n"))

	doc := filepath.Join(docsDir, "change.md")
	writeFile(t, doc, "Removed OldThing in src/main.go.\n")

	result, err := Check(Config{
		DocsDirs:   []string{doc},
		SourceDirs: []string{"src/**/*.go"},
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(result.MissingAdded) != 1 {
		t.Fatalf("len(MissingAdded) = %d, want 1; result=%+v", len(result.MissingAdded), result)
	}
	if got := result.MissingAdded[0]; got.File != "src/main.go" || got.StartLine != 3 || got.EndLine != 4 {
		t.Fatalf("MissingAdded[0] = %+v, want src/main.go:3-4", got)
	}
}

func TestCheck_MissingRemovalMention(t *testing.T) {
	repoDir, docsDir := setupRepo(t)
	t.Chdir(repoDir)

	writeFile(t, filepath.Join(repoDir, "src", "main.go"), strings.Join([]string{
		"package main",
		"",
		"func NewThing() string {",
		"\treturn \"new\"",
		"}",
		"",
	}, "\n"))

	doc := filepath.Join(docsDir, "change.md")
	writeFile(t, doc, strings.Join([]string{
		"```text",
		"Removed OldThing in src/main.go.",
		"```",
		"",
		"```go file=src/main.go lines=3-5",
		"func NewThing() string {",
		"\treturn \"new\"",
		"}",
		"```",
		"",
	}, "\n"))

	result, err := Check(Config{
		DocsDirs:   []string{doc},
		SourceDirs: []string{"src/**/*.go"},
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(result.MissingRemoved) != 1 {
		t.Fatalf("len(MissingRemoved) = %d, want 1; result=%+v", len(result.MissingRemoved), result)
	}
	if got := result.MissingRemoved[0]; got.File != "src/main.go" || got.Symbol != "OldThing" {
		t.Fatalf("MissingRemoved[0] = %+v, want src/main.go / OldThing", got)
	}
}

func TestCheck_FixUpdatesExternalMarkdown(t *testing.T) {
	base := t.TempDir()
	repoDir := filepath.Join(base, "repo")
	docsDir := filepath.Join(base, "docs")
	if err := os.MkdirAll(filepath.Join(repoDir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(repoDir, "src", "main.go"), strings.Join([]string{
		"package main",
		"",
		"func OldThing(name string) string {",
		"\tgreeting := \"old\"",
		"\tsuffix := \"!\"",
		"\treturn greeting + name + suffix",
		"}",
		"",
	}, "\n"))

	gitRun(t, repoDir, "init")
	gitRun(t, repoDir, "config", "user.name", "Test User")
	gitRun(t, repoDir, "config", "user.email", "test@example.com")
	gitRun(t, repoDir, "add", "src/main.go")
	gitRun(t, repoDir, "commit", "-m", "baseline")

	t.Chdir(repoDir)

	writeFile(t, filepath.Join(repoDir, "src", "main.go"), strings.Join([]string{
		"package main",
		"",
		"func OldThing(name string) string {",
		"\tgreeting := \"hello\"",
		"\tsuffix := \"!\"",
		"\treturn greeting + name + suffix",
		"}",
		"",
	}, "\n"))

	doc := filepath.Join(docsDir, "change.md")
	writeFile(t, doc, strings.Join([]string{
		"Removed the old greeting from OldThing in src/main.go.",
		"",
		"```go file=src/main.go lines=3-7",
		"func OldThing(name string) string {",
		"\tgreeting := \"old\"",
		"\tsuffix := \"!\"",
		"\treturn greeting + name + suffix",
		"}",
		"```",
		"",
	}, "\n"))

	result, err := Check(Config{
		DocsDirs:   []string{doc},
		SourceDirs: []string{"src/**/*.go"},
		Fix:        true,
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(result.Fixed) != 1 {
		t.Fatalf("len(Fixed) = %d, want 1; result=%+v", len(result.Fixed), result)
	}
	if len(result.Invalid) != 0 || len(result.MissingAdded) != 0 || len(result.MissingRemoved) != 0 {
		t.Fatalf("unexpected result after fix: %+v", result)
	}

	updated, err := os.ReadFile(doc)
	if err != nil {
		t.Fatalf("ReadFile(doc): %v", err)
	}
	if !strings.Contains(string(updated), `greeting := "hello"`) {
		t.Fatalf("fixed doc did not pick up updated source:\n%s", updated)
	}
}

func TestCheck_SingleSHA(t *testing.T) {
	repoDir, docsDir := setupRepo(t)
	t.Chdir(repoDir)

	writeFile(t, filepath.Join(repoDir, "src", "main.go"), strings.Join([]string{
		"package main",
		"",
		"func NewThing() string {",
		"\treturn \"new\"",
		"}",
		"",
	}, "\n"))
	gitRun(t, repoDir, "add", "src/main.go")
	gitRun(t, repoDir, "commit", "-m", "replace old thing")
	sha := gitRun(t, repoDir, "rev-parse", "HEAD")

	doc := filepath.Join(docsDir, "change.md")
	writeFile(t, doc, strings.Join([]string{
		"Removed OldThing in src/main.go.",
		"",
		"```go file=src/main.go lines=3-5",
		"func NewThing() string {",
		"\treturn \"new\"",
		"}",
		"```",
		"",
	}, "\n"))

	result, err := Check(Config{
		DocsDirs:   []string{doc},
		SourceDirs: []string{"src/**/*.go"},
		GitArgs:    []string{sha},
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(result.Invalid) != 0 || len(result.MissingAdded) != 0 || len(result.MissingRemoved) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func setupRepo(t *testing.T) (string, string) {
	t.Helper()

	base := t.TempDir()
	repoDir := filepath.Join(base, "repo")
	docsDir := filepath.Join(base, "docs")
	if err := os.MkdirAll(filepath.Join(repoDir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(repoDir, "src", "main.go"), strings.Join([]string{
		"package main",
		"",
		"func OldThing() string {",
		"\treturn \"old\"",
		"}",
		"",
	}, "\n"))

	gitRun(t, repoDir, "init")
	gitRun(t, repoDir, "config", "user.name", "Test User")
	gitRun(t, repoDir, "config", "user.email", "test@example.com")
	gitRun(t, repoDir, "add", "src/main.go")
	gitRun(t, repoDir, "commit", "-m", "baseline")

	return repoDir, docsDir
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}
