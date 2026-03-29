package main

import (
	"flag"
	"fmt"
	"os"

	ccexport "github.com/a2d2-dev/cc-history/internal/export"
	"github.com/a2d2-dev/cc-history/internal/loader"
	"github.com/a2d2-dev/cc-history/internal/parser"
)

const version = "0.1.0"

func main() {
	// Top-level flags.
	versionFlag := flag.Bool("version", false, "print version and exit")
	pathFlag := flag.String("path", "", "session directory (default: ~/.claude/projects)")
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

	files, err := loader.ScanJSONL(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	for _, f := range files {
		fmt.Println(f)
	}
}

// runExport handles the "export" subcommand.
func runExport(args []string, globalPath string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	sessionFlag := fs.String("session", "", "session ID to export (default: current session)")
	formatFlag := fs.String("format", "markdown", "output format: markdown or json")
	outputFlag := fs.String("output", "", "output file path (default: stdout)")
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

	// Determine output writer.
	out := os.Stdout
	if *outputFlag != "" {
		f, err := os.Create(*outputFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: cannot create output file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		out = f
	}

	switch *formatFlag {
	case "markdown", "md":
		if err := ccexport.ToMarkdown(out, session); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "json":
		if err := ccexport.ToJSON(out, session); err != nil {
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
