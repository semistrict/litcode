package srcmv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMoveBasic(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "src.go")
	destFile := filepath.Join(dir, "dest.go")

	writeTestFile(t, srcFile, `package main

func foo() {
	println("foo")
}

func bar() {
	println("bar")
}
`)
	writeTestFile(t, destFile, `package main
`)

	err := Move(srcFile, 3, 0, destFile, 2)
	if err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, srcFile, `package main

func bar() {
	println("bar")
}
`)
	assertFileContent(t, destFile, `package main

func foo() {
	println("foo")
}
`)
}

func TestMoveWithDocComment(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "src.go")
	destFile := filepath.Join(dir, "dest.go")

	writeTestFile(t, srcFile, `package main

// foo does something important.
// It has a multi-line comment.
func foo() {
	println("foo")
}

func bar() {
	println("bar")
}
`)
	writeTestFile(t, destFile, `package main
`)

	err := Move(srcFile, 5, 0, destFile, 2)
	if err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, srcFile, `package main

func bar() {
	println("bar")
}
`)
	assertFileContent(t, destFile, `package main

// foo does something important.
// It has a multi-line comment.
func foo() {
	println("foo")
}
`)
}

func TestMoveSameFileUpward(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "src.go")

	writeTestFile(t, srcFile, `package main

func foo() {
	println("foo")
}

func bar() {
	println("bar")
}

func baz() {
	println("baz")
}
`)

	// Move baz (line 11) to after package (line 2), before foo.
	err := Move(srcFile, 11, 0, srcFile, 2)
	if err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, srcFile, `package main

func baz() {
	println("baz")
}

func foo() {
	println("foo")
}

func bar() {
	println("bar")
}
`)
}

func TestMoveSameFileDownward(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "src.go")

	writeTestFile(t, srcFile, `package main

func foo() {
	println("foo")
}

func bar() {
	println("bar")
}

func baz() {
	println("baz")
}
`)

	// Move foo (line 3) to after baz — line 14 (past end, append).
	err := Move(srcFile, 3, 0, srcFile, 14)
	if err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, srcFile, `package main

func bar() {
	println("bar")
}

func baz() {
	println("baz")
}

func foo() {
	println("foo")
}
`)
}

func TestMoveLastDeclaration(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "src.go")
	destFile := filepath.Join(dir, "dest.go")

	writeTestFile(t, srcFile, `package main

func only() {
	println("only")
}
`)
	writeTestFile(t, destFile, `package main
`)

	err := Move(srcFile, 3, 0, destFile, 2)
	if err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, srcFile, `package main
`)
	assertFileContent(t, destFile, `package main

func only() {
	println("only")
}
`)
}

func TestMoveCreatesDestFile(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "src.go")
	destFile := filepath.Join(dir, "new.go")

	writeTestFile(t, srcFile, `package main

func foo() {
	println("foo")
}
`)

	err := Move(srcFile, 3, 0, destFile, 1)
	if err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, destFile, `func foo() {
	println("foo")
}
`)
}

func TestMoveErrorNoNode(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "src.go")
	destFile := filepath.Join(dir, "dest.go")

	writeTestFile(t, srcFile, `package main

func foo() {}
`)
	writeTestFile(t, destFile, `package main
`)

	err := Move(srcFile, 1, 0, destFile, 2)
	if err == nil {
		t.Fatal("expected error for line with no declaration")
	}
	if !strings.Contains(err.Error(), "no top-level declaration") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMoveErrorUnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "src.txt")
	destFile := filepath.Join(dir, "dest.txt")

	writeTestFile(t, srcFile, `hello world`)
	writeTestFile(t, destFile, ``)

	err := Move(srcFile, 1, 0, destFile, 1)
	if err == nil {
		t.Fatal("expected error for unsupported extension")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if string(got) != expected {
		t.Errorf("file %s:\n--- got ---\n%s\n--- expected ---\n%s", filepath.Base(path), string(got), expected)
	}
}
