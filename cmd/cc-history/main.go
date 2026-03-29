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
	regexFlag := flag.Bool("E", false, "treat pattern as extended regular expression")
	afterFlag := flag.Int("A", 0, "show N messages after each match")
	beforeFlag := flag.Int("B", 0, "show N messages before each match")
	contextFlag := flag.Int("C", 0, "show N messages before and after each match")
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

	pattern := flag.Arg(0)
	if pattern == "" {
		display.PrintSession(os.Stdout, session)
		return
	}

	after := *afterFlag
	before := *beforeFlag
	if *contextFlag > 0 {
		if after < *contextFlag {
			after = *contextFlag
		}
		if before < *contextFlag {
			before = *contextFlag
		}
	}

	opts := display.FilterOptions{
		UseRegex: *regexFlag,
		After:    after,
		Before:   before,
	}
	if err := display.FilterSession(os.Stdout, session, pattern, opts); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
