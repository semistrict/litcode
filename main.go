package main

import (
	"os"

	"github.com/semistrict/litcode/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
