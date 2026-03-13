package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/semistrict/litcode/internal/checker"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = old
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(configFile, []byte(`{"docs":["docs"],"source":["src"],"exclude":["vendor/**"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if len(cfg.Docs) != 1 || cfg.Docs[0] != "docs" {
		t.Fatalf("unexpected docs: %+v", cfg.Docs)
	}
	if len(cfg.Source) != 1 || cfg.Source[0] != "src" {
		t.Fatalf("unexpected source: %+v", cfg.Source)
	}
	if len(cfg.Exclude) != 1 || cfg.Exclude[0] != "vendor/**" {
		t.Fatalf("unexpected exclude: %+v", cfg.Exclude)
	}
}

func TestLoadConfig_Errors(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), "run litcode in a directory with a .litcode.json file") {
		t.Fatalf("expected missing-config error, got %v", err)
	}

	if err := os.WriteFile(configFile, []byte(`{"docs":[],"source":["src"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), `"docs" must be a non-empty array`) {
		t.Fatalf("expected docs validation error, got %v", err)
	}

	if err := os.WriteFile(configFile, []byte(`{"docs":["docs"],"source":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(); err == nil || !strings.Contains(err.Error(), `"source" must be a non-empty array`) {
		t.Fatalf("expected source validation error, got %v", err)
	}
}

func TestPrintMissing_PrintsContextAndFallback(t *testing.T) {
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"
	if err := os.WriteFile(filepath.Join(sourceDir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}

	output := captureStderr(t, func() {
		printMissing([]checker.MissingRange{
			{File: "main.go", StartLine: 3, EndLine: 4},
			{File: "missing.go", StartLine: 7, EndLine: 8},
		}, []string{sourceDir})
	})

	if !strings.Contains(output, "MISSING:") || !strings.Contains(output, "main.go") {
		t.Fatalf("expected file header in output, got:\n%s", output)
	}
	if !strings.Contains(output, ">") || !strings.Contains(output, "println(\"hi\")") {
		t.Fatalf("expected highlighted missing source lines, got:\n%s", output)
	}
	if !strings.Contains(output, "missing.go") || !strings.Contains(output, "lines 7-8") {
		t.Fatalf("expected fallback missing-range output for absent file, got:\n%s", output)
	}
}

func TestCheckHelpers(t *testing.T) {
	if got := formatLinesRef(0, 0); got != "" {
		t.Fatalf("formatLinesRef(0,0) = %q, want empty", got)
	}
	if got := formatLinesRef(3, 5); got != " lines 3-5" {
		t.Fatalf("formatLinesRef(3,5) = %q", got)
	}
	if got := formatRange(7, 7); got != "7" {
		t.Fatalf("formatRange single = %q", got)
	}
	if got := formatRange(7, 9); got != "7-9" {
		t.Fatalf("formatRange range = %q", got)
	}
	if got := expandTabs("a\tb", 4); got != "a   b" {
		t.Fatalf("expandTabs = %q", got)
	}
	diff := renderDiff("+ added\n- removed\n")
	if !strings.Contains(diff, "+ added") || !strings.Contains(diff, "- removed") {
		t.Fatalf("renderDiff missing changed lines: %q", diff)
	}
}
