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

// FilterSession writes only messages that match pattern to w.
// If useRegex is true, pattern is compiled as a regular expression.
// Non-contiguous matching groups are separated by a "--" line.
func FilterSession(w io.Writer, session *parser.Session, pattern string, useRegex bool) error {
	match, err := buildMatcher(pattern, useRegex)
	if err != nil {
		return err
	}

	prevMatchIdx := -2
	for i, msg := range session.Messages {
		if msg.Role == "" {
			continue
		}
		if !match(messageText(msg)) {
			continue
		}
		// Print separator when there is a gap between match groups.
		if prevMatchIdx >= 0 && i > prevMatchIdx+1 {
			fmt.Fprintln(w, "--")
		}
		printMessage(w, msg)
		prevMatchIdx = i
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
