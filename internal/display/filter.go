package display

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/a2d2-dev/cc-history/internal/parser"
)

// messageText returns all searchable text from a message (role text + tool names/args).
func messageText(msg *parser.Message) string {
	var sb strings.Builder
	sb.WriteString(msg.Text)
	for _, tc := range msg.ToolCalls {
		sb.WriteByte(' ')
		sb.WriteString(tc.Name)
		sb.WriteByte(' ')
		sb.WriteString(tc.Arguments)
	}
	return sb.String()
}

// FilterOptions controls how FilterSession operates.
type FilterOptions struct {
	UseRegex bool
	// Before is the number of context messages to show before each match (-B).
	Before int
	// After is the number of context messages to show after each match (-A).
	After int
}

// FilterSession writes only messages that match pattern to w.
// If useRegex is true, pattern is compiled as a regular expression.
// Non-contiguous matching groups are separated by a "--" line.
// Before/After context lines are included when opts.Before/After > 0.
func FilterSession(w io.Writer, session *parser.Session, pattern string, opts FilterOptions) error {
	match, err := buildMatcher(pattern, opts.UseRegex)
	if err != nil {
		return err
	}

	msgs := session.Messages

	// Build a slice of "visible" indices (messages with a role).
	visible := make([]int, 0, len(msgs))
	for i, m := range msgs {
		if m.Role != "" {
			visible = append(visible, i)
		}
	}

	// Mark which visible positions match.
	isMatch := make([]bool, len(visible))
	for vi, mi := range visible {
		isMatch[vi] = match(messageText(msgs[mi]))
	}

	// Expand matches with before/after context into a set of visible indices to print.
	// We track which visible positions are included and whether a separator is needed.
	type printEntry struct {
		visibleIdx int
		sep        bool // print "--" before this entry
	}
	var entries []printEntry
	lastIncluded := -1

	for vi := range visible {
		if !isMatch[vi] {
			continue
		}
		// Compute the context window around this match.
		start := vi - opts.Before
		if start < 0 {
			start = 0
		}
		end := vi + opts.After
		if end >= len(visible) {
			end = len(visible) - 1
		}

		for ci := start; ci <= end; ci++ {
			if ci <= lastIncluded {
				continue // already queued
			}
			needSep := lastIncluded >= 0 && ci > lastIncluded+1
			entries = append(entries, printEntry{visibleIdx: ci, sep: needSep})
			lastIncluded = ci
		}
	}

	for _, e := range entries {
		if e.sep {
			fmt.Fprintln(w, "--")
		}
		printMessage(w, msgs[visible[e.visibleIdx]])
	}
	return nil
}

// buildMatcher returns a function that reports whether text contains the pattern.
func buildMatcher(pattern string, useRegex bool) (func(string) bool, error) {
	if useRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", pattern, err)
		}
		return re.MatchString, nil
	}
	lower := strings.ToLower(pattern)
	return func(text string) bool {
		return strings.Contains(strings.ToLower(text), lower)
	}, nil
}
