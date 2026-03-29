package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/a2d2-dev/cc-history/internal/display"
	ccexport "github.com/a2d2-dev/cc-history/internal/export"
	"github.com/a2d2-dev/cc-history/internal/loader"
	"github.com/a2d2-dev/cc-history/internal/parser"
	"github.com/a2d2-dev/cc-history/internal/tui"
)

const version = "0.1.0"

// isTerminal reports whether f is connected to a terminal.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func main() {
	// Enable ANSI colors when stdout is a terminal.
	display.UseColors = isTerminal(os.Stdout)

	// Top-level flags.
	versionFlag := flag.Bool("version", false, "print version and exit")
	pathFlag := flag.String("path", "", "session directory (default: ~/.claude/projects)")
	regexFlag := flag.Bool("E", false, "treat pattern as extended regular expression")
	afterFlag := flag.Int("A", 0, "show N messages after each match")
	beforeFlag := flag.Int("B", 0, "show N messages before each match")
	contextFlag := flag.Int("C", 0, "show N messages before and after each match")
	allFlag := flag.Bool("all", false, "show all sessions sorted by time")
	noSepFlag := flag.Bool("no-sep", false, "disable session separator lines (use with --all)")
	listFlag := flag.Bool("list", false, "list all sessions; mark current session and show its last message")
	flag.BoolVar(listFlag, "l", false, "alias for --list")
	sinceFlag := flag.String("since", "", "only show messages on or after this date (YYYY-MM-DD)")
	untilFlag := flag.String("until", "", "only show messages on or before this date (YYYY-MM-DD)")
	tuiFlag := flag.Bool("tui", false, "open full-screen TUI for the current session")
	interactiveFlag := flag.Bool("i", false, "alias for --tui")
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

	// Parse --since and --until date flags (YYYY-MM-DD, UTC).
	if *sinceFlag != "" {
		t, err := time.Parse("2006-01-02", *sinceFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid --since date %q (expected YYYY-MM-DD)\n", *sinceFlag)
			os.Exit(1)
		}
		opts.Since = t.UTC()
	}
	if *untilFlag != "" {
		t, err := time.Parse("2006-01-02", *untilFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid --until date %q (expected YYYY-MM-DD)\n", *untilFlag)
			os.Exit(1)
		}
		opts.Until = t.UTC()
	}

	pattern := flag.Arg(0)

	if *tuiFlag || *interactiveFlag {
		sessions, err := loader.LoadAllSessions(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if len(sessions) == 0 {
			fmt.Fprintf(os.Stderr, "error: no sessions found in %s\n", root)
			os.Exit(1)
		}
		// Find current session index.
		currentIdx := len(sessions) - 1 // default: last (most recent)
		sessionPath, isFallback, err := loader.FindCurrentSession(root)
		if err == nil {
			for i, s := range sessions {
				if s.FilePath == sessionPath {
					currentIdx = i
					break
				}
			}
		}
		if isFallback {
			fmt.Fprintf(os.Stderr, "note: CLAUDE_SESSION_ID not set — opening most recent session\n")
		}
		if err := tui.RunTUI(sessions, currentIdx, root); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *listFlag {
		currentPath, _, _ := loader.FindCurrentSession(root)
		metas, err := loader.LoadAllSessionsMeta(root, currentPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		display.ListSessions(os.Stdout, metas, currentPath)
		return
	}

	if *allFlag {
		sessions, err := loader.LoadAllSessions(root)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		// Route through FilterAllSessions when a pattern or date filters are set.
		if pattern != "" || !opts.Since.IsZero() || !opts.Until.IsZero() {
			if err := display.FilterAllSessions(os.Stdout, sessions, pattern, opts, *noSepFlag); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		} else {
			display.PrintAllSessions(os.Stdout, sessions, *noSepFlag)
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

	// Route through FilterSession when a pattern or date filters are set.
	if pattern != "" || !opts.Since.IsZero() || !opts.Until.IsZero() {
		if err := display.FilterSession(os.Stdout, session, pattern, opts); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	display.PrintSession(os.Stdout, session)
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
