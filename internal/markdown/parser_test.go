package markdown_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/semistrict/litcode/internal/markdown"
)

func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

func TestParseFile_GoIntro(t *testing.T) {
	path := filepath.Join(testdataDir(), "sample", "docs", "intro.md")
	blocks, err := markdown.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}

	b := blocks[0]
	if b.Language != "go" {
		t.Errorf("language = %q, want %q", b.Language, "go")
	}
	if b.File != "src/main.go" {
		t.Errorf("file = %q, want %q", b.File, "src/main.go")
	}
	if b.StartLine != 6 || b.EndLine != 8 {
		t.Errorf("lines = %d-%d, want 6-8", b.StartLine, b.EndLine)
	}
	if len(b.Content) != 3 {
		t.Fatalf("content lines = %d, want 3", len(b.Content))
	}
	if b.Content[0] != "func greet(name string) string {" {
		t.Errorf("content[0] = %q", b.Content[0])
	}

	b1 := blocks[1]
	if b1.StartLine != 10 || b1.EndLine != 12 {
		t.Errorf("block 1 lines = %d-%d, want 10-12", b1.StartLine, b1.EndLine)
	}
}

func TestParseFile_GoUtil(t *testing.T) {
	path := filepath.Join(testdataDir(), "sample", "docs", "util.md")
	blocks, err := markdown.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}

	if blocks[0].File != "src/util.go" || blocks[0].StartLine != 4 || blocks[0].EndLine != 6 {
		t.Errorf("block 0: file=%q lines=%d-%d", blocks[0].File, blocks[0].StartLine, blocks[0].EndLine)
	}
	if blocks[1].File != "src/util.go" || blocks[1].StartLine != 9 || blocks[1].EndLine != 11 {
		t.Errorf("block 1: file=%q lines=%d-%d", blocks[1].File, blocks[1].StartLine, blocks[1].EndLine)
	}
}

func TestParseFile_TypeScript(t *testing.T) {
	path := filepath.Join(testdataDir(), "sample", "docs", "typescript.md")
	blocks, err := markdown.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Language != "ts" {
		t.Errorf("language = %q, want %q", blocks[0].Language, "ts")
	}
	if blocks[0].File != "src/ts/greeter.ts" {
		t.Errorf("file = %q", blocks[0].File)
	}
	if blocks[0].StartLine != 4 || blocks[0].EndLine != 14 {
		t.Errorf("lines = %d-%d, want 4-14", blocks[0].StartLine, blocks[0].EndLine)
	}
}

func TestParseFile_Python(t *testing.T) {
	path := filepath.Join(testdataDir(), "sample", "docs", "python.md")
	blocks, err := markdown.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Language != "py" {
		t.Errorf("language = %q, want %q", blocks[0].Language, "py")
	}
}

func TestParseFile_Rust(t *testing.T) {
	path := filepath.Join(testdataDir(), "sample", "docs", "rust.md")
	blocks, err := markdown.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Language != "rust" {
		t.Errorf("language = %q, want %q", blocks[0].Language, "rust")
	}
	if blocks[0].File != "src/rs/lib.rs" {
		t.Errorf("file = %q", blocks[0].File)
	}
}

func TestParseFile_Cpp(t *testing.T) {
	path := filepath.Join(testdataDir(), "sample", "docs", "cpp.md")
	blocks, err := markdown.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].Language != "cpp" {
		t.Errorf("language = %q, want %q", blocks[0].Language, "cpp")
	}
}

func TestParseFile_Java(t *testing.T) {
	path := filepath.Join(testdataDir(), "sample", "docs", "java.md")
	blocks, err := markdown.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Language != "java" {
		t.Errorf("language = %q, want %q", blocks[0].Language, "java")
	}
	if blocks[0].StartLine != 6 || blocks[0].EndLine != 16 {
		t.Errorf("lines = %d-%d, want 6-16", blocks[0].StartLine, blocks[0].EndLine)
	}
}
