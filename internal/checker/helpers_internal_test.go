package checker

import (
	"strings"
	"testing"
)

func TestAllDeleted(t *testing.T) {
	got := allDeleted([]string{"one", "two"})
	want := "- one\n- two\n"
	if got != want {
		t.Fatalf("allDeleted = %q, want %q", got, want)
	}
}

func TestUpdateAndStripLinesAnnotation(t *testing.T) {
	if got := updateLinesAnnotation("```go file=foo.go lines=1-2", 7, 9); got != "```go file=foo.go lines=7-9" {
		t.Fatalf("updateLinesAnnotation range = %q", got)
	}
	if got := updateLinesAnnotation("```go file=foo.go lines=1-2", 7, 7); got != "```go file=foo.go lines=7" {
		t.Fatalf("updateLinesAnnotation single = %q", got)
	}
	if got := stripLinesAnnotation("```go file=foo.go lines=7-9"); strings.Contains(got, "lines=") {
		t.Fatalf("stripLinesAnnotation did not remove lines=: %q", got)
	}
}
