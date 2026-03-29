// Package display formats parsed session messages for CLI output.
package display

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/a2d2-dev/cc-history/internal/parser"
)

const (
	summaryMaxRunes = 120
	argMaxRunes     = 80
)

// UseColors enables ANSI color output. Set to true when stdout is a terminal.
var UseColors bool

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"   // user role
	colorYellow = "\033[33m"   // assistant role
	colorRed    = "\033[31m"   // tool errors
	colorBold   = "\033[1m"    // session header
	colorDim    = "\033[2m"    // session header separator
)

func colorize(code, s string) string {
	if !UseColors {
		return s
	}
	return code + s + colorReset
}

// PrintSession writes all messages in session to w in chronological order.
// A session header is printed first, then each message on its own line(s).
// Format per line: [HH:MM:SS]  role  summary
func PrintSession(w io.Writer, session *parser.Session) {
	printSessionHeader(w, session)
	for _, msg := range session.Messages {
		printMessage(w, msg)
	}
}

// printSessionHeader prints a compact one-line summary of the session.
func printSessionHeader(w io.Writer, session *parser.Session) {
	// Collect timestamps to compute time range.
	var first, last time.Time
	count := 0
	for _, m := range session.Messages {
		if m.Role == "" {
			continue
		}
		count++
		if first.IsZero() || m.Timestamp.Before(first) {
			first = m.Timestamp
		}
		if m.Timestamp.After(last) {
			last = m.Timestamp
		}
	}

	idStr := session.ID
	if len(idStr) > 8 {
		idStr = idStr[:8]
	}

	var timeRange string
	if !first.IsZero() {
		if first.Format("2006-01-02") == last.Format("2006-01-02") {
			timeRange = first.Format("2006-01-02 15:04:05") + " – " + last.Format("15:04:05")
		} else {
			timeRange = first.Format("2006-01-02 15:04") + " – " + last.Format("2006-01-02 15:04")
		}
	}

	header := fmt.Sprintf("─── session %s  %s  %d messages  %s",
		idStr, session.FilePath, count, timeRange)
	fmt.Fprintln(w, colorize(colorBold+colorDim, header))
}

func printMessage(w io.Writer, msg *parser.Message) {
	// Skip messages with no role (e.g. system/turn_duration records).
	if msg.Role == "" {
		return
	}

	ts := msg.Timestamp.Format("15:04:05")
	role := formatRole(msg.Role)
	hasText := strings.TrimSpace(msg.Text) != ""

	// Print text summary if present.
	if hasText {
		fmt.Fprintf(w, "[%s]  %s  %s\n", ts, role, truncate(strings.TrimSpace(msg.Text), summaryMaxRunes))
	}

	// Print each tool call on its own line.
	for _, tc := range msg.ToolCalls {
		fmt.Fprintf(w, "[%s]  %s  [%s %s]\n", ts, role, tc.Name, formatArgs(tc.Arguments))
	}

	// For messages with no text and no tool calls:
	if !hasText && len(msg.ToolCalls) == 0 {
		// User messages that consist only of tool_result blocks: show errors,
		// skip successful results (already visible via the assistant tool-call line).
		if msg.Role == "user" && len(msg.ToolResults) > 0 {
			for _, tr := range msg.ToolResults {
				if tr.IsError {
					name := tr.ToolName
					if name == "" {
						name = tr.ToolUseID
					}
					fmt.Fprintf(w, "[%s]  %s  %s\n", ts, role, colorize(colorRed, "[tool error: "+name+"]"))
				}
			}
			return
		}

		// Assistant messages that contain only thinking blocks: skip silently.
		if msg.Role == "assistant" && msg.HasThinking {
			return
		}

		// Fallback: emit a placeholder so truly empty messages are visible.
		fmt.Fprintf(w, "[%s]  %s  (no content)\n", ts, role)
	}
}

// formatRole returns a fixed-width role label, optionally colorized.
func formatRole(role string) string {
	switch role {
	case "user":
		return colorize(colorCyan, "user")
	case "assistant":
		return colorize(colorYellow, "asst")
	default:
		r := role
		if utf8.RuneCountInString(r) > 4 {
			runes := []rune(r)
			r = string(runes[:4])
		}
		return fmt.Sprintf("%-4s", r)
	}
}

// truncate returns s truncated to maxRunes runes, appending "…" if cut.
func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}

// formatArgs extracts a short human-readable summary from JSON tool arguments.
// It shows up to the first 2 key=value pairs and truncates long values.
func formatArgs(raw string) string {
	if raw == "" || raw == "{}" {
		return ""
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		// Not valid JSON — show raw truncated.
		return truncate(raw, argMaxRunes)
	}

	var parts []string
	count := 0
	for k, v := range m {
		if count >= 2 {
			parts = append(parts, "…")
			break
		}
		valStr := fmt.Sprintf("%v", v)
		valStr = truncate(strings.ReplaceAll(valStr, "\n", "\\n"), argMaxRunes)
		parts = append(parts, k+"="+valStr)
		count++
	}
	return strings.Join(parts, " ")
}
