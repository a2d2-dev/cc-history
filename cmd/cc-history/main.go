package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	ccexport "github.com/a2d2-dev/cc-history/internal/export"
	"github.com/a2d2-dev/cc-history/internal/loader"
	"github.com/a2d2-dev/cc-history/internal/parser"
	ccprompt "github.com/a2d2-dev/cc-history/internal/prompt"
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
	if len(args) > 0 && args[0] == "prompt" {
		runPrompt(args[1:], *pathFlag)
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

// runPrompt handles the "prompt" subcommand.
func runPrompt(args []string, globalPath string) {
	fs := flag.NewFlagSet("prompt", flag.ExitOnError)
	sessionFlag := fs.String("session", "", "session ID (default: current session)")
	rangeFlag := fs.String("range", "", "message range, e.g. 1-5 (default: all)")
	outputFlag := fs.String("output", "", "save output to file (default: stdout)")
	copyFlag := fs.Bool("copy", false, "copy reconstructed prompt to clipboard")
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
			fmt.Fprintf(os.Stderr, "note: CLAUDE_SESSION_ID not set — using most recent session\n")
		}
	}

	session, err := parser.ParseFile(sessionPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	opts := ccprompt.Options{}
	if *rangeFlag != "" {
		start, end, err := parseRange(*rangeFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: --range %q: %v\n", *rangeFlag, err)
			os.Exit(1)
		}
		opts.Start = start
		opts.End = end
	}

	result, err := ccprompt.Build(session, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Write full output.
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
	if err := ccprompt.Write(out, result); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Copy reconstructed prompt to clipboard if requested.
	if *copyFlag {
		if err := copyToClipboard(result.ReconstructedPrompt); err != nil {
			fmt.Fprintf(os.Stderr, "warning: clipboard copy failed: %v\n", err)
		} else {
			fmt.Fprintln(os.Stderr, "note: reconstructed prompt copied to clipboard")
		}
	}
}

// parseRange parses a "start-end" range string into 1-based inclusive bounds.
func parseRange(s string) (start, end int, err error) {
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected format start-end (e.g. 1-5)")
	}
	start, err = strconv.Atoi(parts[0])
	if err != nil || start < 1 {
		return 0, 0, fmt.Errorf("start must be a positive integer")
	}
	end, err = strconv.Atoi(parts[1])
	if err != nil || end < 1 {
		return 0, 0, fmt.Errorf("end must be a positive integer")
	}
	if start > end {
		return 0, 0, fmt.Errorf("start (%d) must be <= end (%d)", start, end)
	}
	return start, end, nil
}

// copyToClipboard writes text to the system clipboard.
// It tries xclip, then xsel on Linux/X11.
func copyToClipboard(text string) error {
	tools := [][]string{
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
	}
	for _, args := range tools {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = bytes.NewBufferString(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no clipboard tool found (install xclip or xsel)")
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
