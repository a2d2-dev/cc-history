package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/a2d2-dev/cc-history/internal/display"
	ccexport "github.com/a2d2-dev/cc-history/internal/export"
	"github.com/a2d2-dev/cc-history/internal/loader"
	"github.com/a2d2-dev/cc-history/internal/parser"
)

const version = "0.1.0"

func main() {
	// Top-level flags.
	versionFlag := flag.Bool("version", false, "print version and exit")
	pathFlag := flag.String("path", "", "session directory (default: ~/.claude/projects)")
	regexFlag := flag.Bool("E", false, "treat pattern as extended regular expression")
	afterFlag := flag.Int("A", 0, "show N messages after each match")
	beforeFlag := flag.Int("B", 0, "show N messages before each match")
	contextFlag := flag.Int("C", 0, "show N messages before and after each match")
	allFlag := flag.Bool("all", false, "show all sessions sorted by time")
	noSepFlag := flag.Bool("no-sep", false, "disable session separator lines (use with --all)")
	flag.Parse()

	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) > 0 && args[0] == "export" {
		runExport(args[1:], *pathFlag)
		return
	}

	root := resolveRoot(*pathFlag)

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
	pattern := flag.Arg(0)

	if *allFlag {
		sessions, err := loader.LoadAllSessions(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if pattern == "" {
			display.PrintAllSessions(os.Stdout, sessions, *noSepFlag)
		} else {
			if err := display.FilterAllSessions(os.Stdout, sessions, pattern, opts, *noSepFlag); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		}
		return
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

	if pattern == "" {
		display.PrintSession(os.Stdout, session)
		return
	}

	if err := display.FilterSession(os.Stdout, session, pattern, opts); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// runExport handles the "export" subcommand.
func runExport(args []string, globalPath string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	sessionFlag := fs.String("session", "", "session ID to export (default: current session)")
	formatFlag := fs.String("format", "markdown", "output format: markdown or json")
	fs.Parse(args) //nolint:errcheck // ExitOnError handles errors

	root := resolveRoot(globalPath)

	var sessionPath string
	if *sessionFlag != "" {
		var err error
		sessionPath, err = loader.FindSessionByID(root, *sessionFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else {
		isFallback := false
		var err error
		sessionPath, isFallback, err = loader.FindCurrentSession(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if isFallback {
			fmt.Fprintf(os.Stderr, "note: CLAUDE_SESSION_ID not set — exporting most recent session\n")
		}
	}

	session, err := parser.ParseFile(sessionPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	switch *formatFlag {
	case "markdown", "md":
		if err := ccexport.ToMarkdown(os.Stdout, session); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "json":
		if err := ccexport.ToJSON(os.Stdout, session); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "error: unknown format %q (use markdown or json)\n", *formatFlag)
		os.Exit(1)
	}
}

func resolveRoot(pathFlag string) string {
	if pathFlag != "" {
		return pathFlag
	}
	root, err := loader.DefaultSessionsPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return root
}
