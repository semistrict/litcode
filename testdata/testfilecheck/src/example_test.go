package example

import "testing"

func TestClassify(t *testing.T) {
	got := classify(1)
	if got != "positive" {
		t.Fatalf("classify(1) = %q", got)
	}
}
