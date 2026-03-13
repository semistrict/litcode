package filematch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{pattern: "docs/**/*.md", path: "docs/overview.md", want: true},
		{pattern: "docs/**/*.md", path: "docs/api/cli.md", want: true},
		{pattern: "**/*.go", path: "cmd/check.go", want: true},
		{pattern: "**/*.go", path: "docs/check.md", want: false},
		{pattern: "src/*.go", path: "src/main.go", want: true},
		{pattern: "src/*.go", path: "src/lib/util.go", want: false},
	}

	for _, tc := range tests {
		if got := MatchPath(tc.pattern, tc.path); got != tc.want {
			t.Fatalf("MatchPath(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestCollect(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	mustWriteFile(t, filepath.Join("docs", "overview.md"))
	mustWriteFile(t, filepath.Join("docs", "api", "cli.md"))
	mustWriteFile(t, filepath.Join("src", "main.go"))
	mustWriteFile(t, filepath.Join("src", "notes.txt"))

	matches, err := Collect([]string{"docs/**/*.md", "src/**/*.go"}, nil)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}
	if matches[0].RelPath != "docs/api/cli.md" || matches[1].RelPath != "docs/overview.md" || matches[2].RelPath != "src/main.go" {
		t.Fatalf("unexpected matches: %+v", matches)
	}
}

func mustWriteFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
