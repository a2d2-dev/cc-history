package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "cc-history v%s\n", version)
	fmt.Fprintln(os.Stderr, "Usage: cc-history [--version]")
	os.Exit(1)
}
