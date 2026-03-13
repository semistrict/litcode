package cmd

import (
	"bytes"
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
	if err := os.WriteFile(configFile, []byte(`{"docs":["docs"],"source":["src"],"lenient":["generated/**"],"exclude":["vendor/**"]}`), 0o644); err != nil {
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
	if len(cfg.Lenient) != 1 || cfg.Lenient[0] != "generated/**" {
		t.Fatalf("unexpected lenient: %+v", cfg.Lenient)
	}
	if len(cfg.Exclude) != 1 || cfg.Exclude[0] != "vendor/**" {
		t.Fatalf("unexpected exclude: %+v", cfg.Exclude)
	}
}

func TestLoadConfig_AllowsJSONCComments(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(configFile, []byte(`{
  // docs globs
  "docs": ["docs/**/*.md"],
  // source globs
  "source": ["src/**/*.go"],
  "lenient": [],
  "exclude": []
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if len(cfg.Docs) != 1 || cfg.Docs[0] != "docs/**/*.md" {
		t.Fatalf("unexpected docs: %+v", cfg.Docs)
	}
	if len(cfg.Source) != 1 || cfg.Source[0] != "src/**/*.go" {
		t.Fatalf("unexpected source: %+v", cfg.Source)
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

func TestWriteConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, configFile)

	if err := writeConfig(path, defaultConfig()); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Contains(data, []byte(`"docs": [`)) || !bytes.Contains(data, []byte(`"source": [`)) {
		t.Fatalf("unexpected config contents:\n%s", data)
	}
	if !bytes.Contains(data, []byte(`"lenient": []`)) {
		t.Fatalf("expected explicit empty lenient array, got:\n%s", data)
	}
	if !bytes.Contains(data, []byte(`"exclude": []`)) {
		t.Fatalf("expected explicit empty exclude array, got:\n%s", data)
	}
	if data[len(data)-1] != '\n' {
		t.Fatalf("config file should end with newline")
	}
}

func TestInitCmd(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var stdout bytes.Buffer
	initCmd.SetOut(&stdout)
	t.Cleanup(func() {
		initCmd.SetOut(os.Stdout)
	})

	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	if got := stdout.String(); !strings.Contains(got, "Created .litcode.json") {
		t.Fatalf("unexpected output: %q", got)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Contains(data, []byte(`// Markdown files to scan for documented code blocks.`)) {
		t.Fatalf("expected init config comments, got:\n%s", data)
	}
	if !bytes.Contains(data, []byte(`"lenient": []`)) {
		t.Fatalf("expected lenient entry in init config, got:\n%s", data)
	}

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if len(cfg.Docs) != 1 || cfg.Docs[0] != "docs/**/*.md" {
		t.Fatalf("unexpected docs: %+v", cfg.Docs)
	}
	if len(cfg.Source) < 4 || cfg.Source[0] != "**/*.go" || cfg.Source[1] != "**/*.ts" {
		t.Fatalf("unexpected source: %+v", cfg.Source)
	}
	if cfg.Lenient == nil || len(cfg.Lenient) != 0 {
		t.Fatalf("unexpected lenient: %+v", cfg.Lenient)
	}
	if cfg.Exclude == nil || len(cfg.Exclude) != 0 {
		t.Fatalf("unexpected exclude: %+v", cfg.Exclude)
	}
}

func TestInitCmd_ErrorsWhenConfigExists(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := writeConfig(configFile, defaultConfig()); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	if err := initCmd.RunE(initCmd, nil); err == nil || !strings.Contains(err.Error(), ".litcode.json already exists") {
		t.Fatalf("expected already-exists error, got %v", err)
	}
}
