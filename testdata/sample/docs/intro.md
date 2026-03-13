# Introduction

This document covers the main entry point.

## The greet function

The `greet` function builds a greeting string:

```go file=src/main.go lines=6-8
func greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}
```

## The main function

The program entry point calls `greet`:

```go file=src/main.go lines=10-12
func main() {
	fmt.Println(greet("world"))
}
```
