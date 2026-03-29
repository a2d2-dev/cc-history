package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/a2d2-dev/cc-history/internal/display"
	"github.com/a2d2-dev/cc-history/internal/loader"
	"github.com/a2d2-dev/cc-history/internal/parser"
)

const version = "0.1.0"

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	pathFlag := flag.String("path", "", "session directory (default: ~/.claude/projects)")
	flag.Parse()

	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	root := *pathFlag
	if root == "" {
		var err error
		root, err = loader.DefaultSessionsPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	sessionPath, isFallback, err := loader.FindCurrentSession(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if isFallback {
		fmt.Fprintf(os.Stderr, "note: CLAUDE_SESSION_ID not set — showing most recent session\n")
	}

	session, err := parser.ParseFile(sessionPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	display.PrintSession(os.Stdout, session)
}
