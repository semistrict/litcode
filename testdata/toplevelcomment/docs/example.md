# Example

This block includes two top-level declarations and should be split around the comment:

```go file=src/example.go lines=3-8
func first() string {
	return "first"
}

// second returns the other label.
func second() string {
	return "second"
}
```

This block contains a nested comment inside a function body and should not warn:

```go file=src/example.go lines=10-15
func combined() string {
	value := first()
	// This nested comment belongs with the implementation.
	if value == "" {
		return second()
	}
	return value
}
```
