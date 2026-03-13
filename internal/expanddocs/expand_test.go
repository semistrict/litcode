package expanddocs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandedMarkdown_ExpandsAbbreviatedBlocks(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	srcDir := filepath.Join(root, "src")

	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	source := "package example\n\n" +
		"func classify(n int) string {\n" +
		"\tif n < 0 {\n" +
		"\t\treturn \"negative\"\n" +
		"\t}\n\n" +
		"\ttotal := n + 1\n" +
		"\ttotal += 2\n\n" +
		"\tif total > 10 {\n" +
		"\t\treturn \"large\"\n" +
		"\t}\n\n" +
		"\treturn \"positive\"\n" +
		"}\n"
	if err := os.WriteFile(filepath.Join(srcDir, "example.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	doc := "# Abbreviated\n\n" +
		"```go file=src/example.go lines=3-16\n" +
		"func classify(n int) string {\n" +
		"\tif n < 0 {\n" +
		"\t\treturn \"negative\"\n" +
		"\t}\n" +
		"\t// ...\n" +
		"\tif total > 10 {\n" +
		"\t\treturn \"large\"\n" +
		"\t}\n\n" +
		"\treturn \"positive\"\n" +
		"}\n" +
		"```\n"
	docPath := filepath.Join(docsDir, "example.md")
	if err := os.WriteFile(docPath, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	expanded, err := ExpandedMarkdown(docPath, []string{root})
	if err != nil {
		t.Fatalf("ExpandedMarkdown: %v", err)
	}

	text := string(expanded)
	if strings.Contains(text, "// ...") {
		t.Fatalf("expected omission marker to be removed, got:\n%s", text)
	}
	if !strings.Contains(text, "total := n + 1") {
		t.Fatalf("expected expanded markdown to contain omitted middle line, got:\n%s", text)
	}
}

func TestExpandedMarkdown_ExpandsAbbreviatedBlocksWithoutLinesAnnotation(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	srcDir := filepath.Join(root, "src")

	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	source := "package example\n\n" +
		"func classify(n int) string {\n" +
		"\tif n < 0 {\n" +
		"\t\treturn \"negative\"\n" +
		"\t}\n\n" +
		"\ttotal := n + 1\n" +
		"\ttotal += 2\n\n" +
		"\tif total > 10 {\n" +
		"\t\treturn \"large\"\n" +
		"\t}\n\n" +
		"\treturn \"positive\"\n" +
		"}\n"
	if err := os.WriteFile(filepath.Join(srcDir, "example.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	doc := "# Abbreviated\n\n" +
		"```go file=src/example.go\n" +
		"func classify(n int) string {\n" +
		"\tif n < 0 {\n" +
		"\t\treturn \"negative\"\n" +
		"\t}\n" +
		"\t// ...\n" +
		"\tif total > 10 {\n" +
		"\t\treturn \"large\"\n" +
		"\t}\n\n" +
		"\treturn \"positive\"\n" +
		"}\n" +
		"```\n"
	docPath := filepath.Join(docsDir, "example.md")
	if err := os.WriteFile(docPath, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	expanded, err := ExpandedMarkdown(docPath, []string{root})
	if err != nil {
		t.Fatalf("ExpandedMarkdown: %v", err)
	}

	text := string(expanded)
	if strings.Contains(text, "// ...") {
		t.Fatalf("expected omission marker to be removed, got:\n%s", text)
	}
	if !strings.Contains(text, "total := n + 1") {
		t.Fatalf("expected expanded markdown to contain omitted middle line, got:\n%s", text)
	}
}

func TestExpandTree_WritesExpandedMarkdownTree(t *testing.T) {
	root := t.TempDir()
	docsDir := filepath.Join(root, "docs")
	srcDir := filepath.Join(root, "src")
	outDir := filepath.Join(root, "out")

	if err := os.MkdirAll(filepath.Join(docsDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	source := "package example\n\nfunc greet() string {\n\tmsg := \"hello\"\n\treturn msg\n}\n"
	if err := os.WriteFile(filepath.Join(srcDir, "example.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	doc := "# Example\n\n" +
		"```go file=src/example.go lines=3-6\n" +
		"func greet() string {\n" +
		"\t// ...\n" +
		"\treturn msg\n" +
		"}\n" +
		"```\n"
	if err := os.WriteFile(filepath.Join(docsDir, "nested", "example.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "notes.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ExpandTree(docsDir, outDir, []string{root}); err != nil {
		t.Fatalf("ExpandTree: %v", err)
	}

	expanded, err := os.ReadFile(filepath.Join(outDir, "nested", "example.md"))
	if err != nil {
		t.Fatalf("read expanded markdown: %v", err)
	}
	text := string(expanded)
	if strings.Contains(text, "// ...") {
		t.Fatalf("expected omitted marker removed, got:\n%s", text)
	}
	if !strings.Contains(text, `msg := "hello"`) {
		t.Fatalf("expected expanded source line in output, got:\n%s", text)
	}
	if _, err := os.Stat(filepath.Join(outDir, "notes.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected non-markdown file to be ignored, stat err=%v", err)
	}
}
