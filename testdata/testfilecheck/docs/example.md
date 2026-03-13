# Test Doc

```go file=src/example_test.go lines=4-8
func TestClassify(t *testing.T) {
	got := classify(1)
	if got != "wrong" {
		t.Fatalf("classify(1) = %q", got)
	}
}
```
