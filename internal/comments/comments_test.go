package comments_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/semistrict/litcode/internal/comments"
)

func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

func readTestFile(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataDir(), "sample", rel))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSkippableLines_Go(t *testing.T) {
	src := readTestFile(t, "src/main.go")
	skip := comments.SkippableLines("main.go", src)

	// Line 1: package main → skippable
	if !skip[1] {
		t.Error("line 1 (package) should be skippable")
	}
	// Line 3: import "fmt" → skippable
	if !skip[3] {
		t.Error("line 3 (import) should be skippable")
	}
	// Line 5: // greet comment → skippable
	if !skip[5] {
		t.Error("line 5 (comment) should be skippable")
	}
	// Line 6: func greet → NOT skippable
	if skip[6] {
		t.Error("line 6 (func decl) should not be skippable")
	}
	// Line 10: func main → NOT skippable
	if skip[10] {
		t.Error("line 10 (func main) should not be skippable")
	}
}

func TestSkippableLines_Python(t *testing.T) {
	src := readTestFile(t, "src/py/calculator.py")
	skip := comments.SkippableLines("calculator.py", src)

	// Line 1: import math → skippable
	if !skip[1] {
		t.Error("line 1 (import) should be skippable")
	}
	// Line 2: from typing import List → skippable
	if !skip[2] {
		t.Error("line 2 (from import) should be skippable")
	}
	// Line 4: # comment → skippable
	if !skip[4] {
		t.Error("line 4 (comment) should be skippable")
	}
	// Line 6: def add → NOT skippable
	if skip[6] {
		t.Error("line 6 (def) should not be skippable")
	}
}

func TestSkippableLines_Rust(t *testing.T) {
	src := readTestFile(t, "src/rs/lib.rs")
	skip := comments.SkippableLines("lib.rs", src)

	// Line 1: use std::collections::HashMap → skippable
	if !skip[1] {
		t.Error("line 1 (use) should be skippable")
	}
	// Line 3: /// doc comment → skippable
	if !skip[3] {
		t.Error("line 3 (doc comment) should be skippable")
	}
	// Line 4: fn word_count → NOT skippable
	if skip[4] {
		t.Error("line 4 (fn) should not be skippable")
	}
}

func TestSkippableLines_Java(t *testing.T) {
	src := readTestFile(t, "src/java/Hello.java")
	skip := comments.SkippableLines("Hello.java", src)

	// Line 1: package → skippable
	if !skip[1] {
		t.Error("line 1 (package) should be skippable")
	}
	// Line 3: import → skippable
	if !skip[3] {
		t.Error("line 3 (import) should be skippable")
	}
	// Line 5: // comment → skippable
	if !skip[5] {
		t.Error("line 5 (comment) should be skippable")
	}
	// Line 6: public class Hello → NOT skippable
	if skip[6] {
		t.Error("line 6 (class decl) should not be skippable")
	}
}

func TestSkippableLines_Cpp(t *testing.T) {
	src := readTestFile(t, "src/cpp/vector.cpp")
	skip := comments.SkippableLines("vector.cpp", src)

	// Line 1: #include → skippable
	if !skip[1] {
		t.Error("line 1 (#include) should be skippable")
	}
	// Line 2: #include → skippable
	if !skip[2] {
		t.Error("line 2 (#include) should be skippable")
	}
	// Line 4: // comment → skippable
	if !skip[4] {
		t.Error("line 4 (comment) should be skippable")
	}
	// Line 5: int sum → NOT skippable
	if skip[5] {
		t.Error("line 5 (function) should not be skippable")
	}
}

func TestSkippableLines_TypeScript(t *testing.T) {
	src := readTestFile(t, "src/ts/greeter.ts")
	skip := comments.SkippableLines("greeter.ts", src)

	// Line 1: import → skippable
	if !skip[1] {
		t.Error("line 1 (import) should be skippable")
	}
	// Line 3: // comment → skippable
	if !skip[3] {
		t.Error("line 3 (comment) should be skippable")
	}
	// Line 4: export class → NOT skippable
	if skip[4] {
		t.Error("line 4 (class decl) should not be skippable")
	}
}

func TestSkippableLines_SyntaxOnlyLines(t *testing.T) {
	// Lines that are only brackets/keywords should be skippable.
	// Line numbers:
	// 1: package main
	// 2: (blank)
	// 3: func main() {
	// 4: 	if true {
	// 5: 		doSomething()
	// 6: 	} else {
	// 7: 		doOther()
	// 8: 	}
	// 9: }
	src := []byte(`package main

func main() {
	if true {
		doSomething()
	} else {
		doOther()
	}
}
`)
	skip := comments.SkippableLines("test.go", src)

	// "} else {" — all leaf nodes are anon → skippable
	if !skip[6] {
		t.Error("line 6 (} else {) should be skippable (syntax-only)")
	}
	// "}" closing if/else → skippable
	if !skip[8] {
		t.Error("line 8 (}) should be skippable (syntax-only)")
	}
	// "}" closing func → skippable
	if !skip[9] {
		t.Error("line 9 (}) should be skippable (syntax-only)")
	}
	// "doSomething()" is real code, NOT skippable
	if skip[5] {
		t.Error("line 5 (doSomething()) should not be skippable")
	}
	// "doOther()" is real code, NOT skippable
	if skip[7] {
		t.Error("line 7 (doOther()) should not be skippable")
	}
	// "func main() {" has named leaf "main" → NOT skippable
	if skip[3] {
		t.Error("line 3 (func main) should not be skippable")
	}
}

func TestSkippableLines_UnsupportedExtension(t *testing.T) {
	skip := comments.SkippableLines("foo.xyz", []byte("some content"))
	if skip != nil {
		t.Error("expected nil for unsupported extension")
	}
}

func TestTopLevelCommentLines_Go(t *testing.T) {
	src := []byte(`func first() string {
	return "first"
}

// second returns the other label.
func second() string {
	return "second"
}

func combined() string {
	value := first()
	// This nested comment belongs with the implementation.
	if value == "" {
		return second()
	}
	return value
}
`)

	lines := comments.TopLevelCommentLines("example.go", src)
	if !lines[5] {
		t.Fatal("expected top-level comment line 5 to be detected")
	}
	if lines[11] {
		t.Fatal("did not expect nested function-body comment line 11 to be detected as top-level")
	}
}
