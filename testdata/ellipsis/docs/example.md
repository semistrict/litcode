# Abbreviated Block

```go file=src/example.go lines=3-16
func classify(n int) string {
	if n < 0 {
		return "negative"
	}
	// ...
	if total > 10 {
		return "large"
	}

	return "positive"
}
```
