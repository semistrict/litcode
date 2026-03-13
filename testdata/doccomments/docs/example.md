# Example

The greet function with an explanatory comment in the doc block:

```go file=src/example.go lines=3-5
func greet(name string) string {
	// This comment only exists in the doc, not in source
	return "hello " + name
}
```

The farewell function without line numbers, also with a doc-only comment:

```go file=src/example.go
func farewell(name string) string {
	// Another doc-only annotation
	return "goodbye " + name
}
```
