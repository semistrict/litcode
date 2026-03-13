package renderdocs

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
)

func testRenderer() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("catppuccin-mocha"),
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(true),
					chromahtml.ClassPrefix("tok-"),
				),
			),
		),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(goldmarkhtml.WithUnsafe()),
	)
}

func TestRenderPage_RewritesLinksAndMermaid(t *testing.T) {
	src := []byte("# Parser\n\n" +
		"See [Checker](04-checker.md#phase-2).\n\n" +
		"```go\n" +
		"func greet() string { return \"hi\" }\n" +
		"```\n\n" +
		"```mermaid\n" +
		"sequenceDiagram\n" +
		"    title function: ParseFile\n" +
		"```\n")

	page, err := renderPage(testRenderer(), "02-markdown-parsing.md", src)
	if err != nil {
		t.Fatalf("renderPage: %v", err)
	}

	html := string(page)
	if !strings.Contains(html, `href="04-checker.html#phase-2"`) {
		t.Fatalf("expected markdown link rewrite, got:\n%s", html)
	}
	if !strings.Contains(html, `<div class="mermaid">`) {
		t.Fatalf("expected mermaid block rewrite, got:\n%s", html)
	}
	if !strings.Contains(html, `class="tok-`) {
		t.Fatalf("expected syntax highlighting classes, got:\n%s", html)
	}
	if !strings.Contains(html, `<title>Parser</title>`) {
		t.Fatalf("expected title from heading, got:\n%s", html)
	}
	if !strings.Contains(html, `color-scheme: dark;`) {
		t.Fatalf("expected dark mode defaults, got:\n%s", html)
	}
	if !strings.Contains(html, `theme: "dark"`) {
		t.Fatalf("expected dark mermaid theme, got:\n%s", html)
	}
}

func TestRenderTree_WritesHTMLFiles(t *testing.T) {
	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "docs")

	if err := os.WriteFile(filepath.Join(srcDir, "index.md"), []byte("# Home\n\nSee [next](guide.md).\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "nested", "guide.md"), []byte("# Guide\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := RenderTree(srcDir, outDir, []string{srcDir}, nil); err != nil {
		t.Fatalf("RenderTree: %v", err)
	}

	indexHTML, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if !strings.Contains(string(indexHTML), `href="guide.html"`) {
		t.Fatalf("expected rewritten internal link, got:\n%s", string(indexHTML))
	}

	if _, err := os.Stat(filepath.Join(outDir, "nested", "guide.html")); err != nil {
		t.Fatalf("expected nested guide.html: %v", err)
	}
}

func TestRenderTree_PrintsWrittenFiles(t *testing.T) {
	rootDir := t.TempDir()
	t.Chdir(rootDir)

	srcDir := filepath.Join(rootDir, "src")
	outDir := filepath.Join(rootDir, "out", "docs")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "index.md"), []byte("# Home\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := RenderTree(srcDir, outDir, []string{srcDir}, &output); err != nil {
		t.Fatalf("RenderTree: %v", err)
	}

	if !strings.Contains(output.String(), filepath.Join("out", "docs", "index.html")) {
		t.Fatalf("expected output to list written file, got:\n%s", output.String())
	}
}

func TestRenderTree_ExpandsAbbreviatedBlocksBeforeRendering(t *testing.T) {
	srcDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "docs")

	if err := os.MkdirAll(filepath.Join(srcDir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "src"), 0o755); err != nil {
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
	if err := os.WriteFile(filepath.Join(srcDir, "src", "example.go"), []byte(source), 0o644); err != nil {
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
	if err := os.WriteFile(filepath.Join(srcDir, "docs", "example.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := RenderTree(filepath.Join(srcDir, "docs"), outDir, []string{srcDir}, nil); err != nil {
		t.Fatalf("RenderTree: %v", err)
	}

	html, err := os.ReadFile(filepath.Join(outDir, "example.html"))
	if err != nil {
		t.Fatalf("read example.html: %v", err)
	}
	if strings.Contains(string(html), "// ...") {
		t.Fatalf("expected rendered HTML to omit the omission marker, got:\n%s", string(html))
	}
	if !strings.Contains(string(html), ":=") {
		t.Fatalf("expected rendered HTML to include expanded middle lines, got:\n%s", string(html))
	}
}
