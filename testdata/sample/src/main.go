package main

import "fmt"

// greet returns a greeting for the given name.
func greet(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

func main() {
	fmt.Println(greet("world"))
}
