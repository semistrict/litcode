# Example

This prose duplicates the comment in the next block, but both duplicate-comment
and leading-comment warnings are suppressed for that block only.

<!-- litcode-ignore duplicate-comment,leading-comment -->
```go file=src/example.go lines=3-6
// greet returns a greeting.
func greet(name string) string {
	return "hello " + name
}
```

This block contains a top-level comment between declarations, but that warning
is suppressed here:

<!-- litcode-ignore top-level-comment -->
```go file=src/example.go lines=8-14
func first() string {
	return "first"
}

// second returns the other label.
func second() string {
	return "second"
}
```

This final block is left unsuppressed and should still warn about the leading
comment line:

```go file=src/example.go lines=16-19
// farewell returns goodbye.
func farewell() string {
	return "bye"
}
```
