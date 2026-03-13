# Example

`greet` returns a greeting for the given name:

```go file=src/example.go lines=3-6
// greet returns a greeting for the given name.
func greet(name string) string {
	return "hello " + name
}
```

This is unique prose that doesn't duplicate the comment:

```go file=src/example.go lines=8-11
// farewell says goodbye.
func farewell(name string) string {
	return "goodbye " + name
}
```
