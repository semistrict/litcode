# Example

The add function (doc has no trailing comment, different indentation):

```go file=src/example.go lines=4-6
func add(a, b int) int {
    return a + b
}
```

The multiply function (doc has extra trailing comment):

```go file=src/example.go lines=9-11
func multiply(a, b int) int {
	return a * b   // product
}
```
