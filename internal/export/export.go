// Package export formats parsed sessions as Markdown or JSON.
package export

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/a2d2-dev/cc-history/internal/parser"
)

// ToMarkdown writes session as a Markdown document to w.
//
// Structure:
//
//	# Session: <id>
//
//	## [HH:MM:SS] user/assistant
//
//	<text>
//
//	### Tool Call: <name> (<id>)
//	**Arguments:**
//	```json
//	<args>
//	```
//	**Result:** <result>
func ToMarkdown(w io.Writer, session *parser.Session) error {
	fmt.Fprintf(w, "# Session: %s\n\n", session.ID)

	for _, msg := range session.Messages {
		if msg.Role == "" {
			continue
		}

		ts := msg.Timestamp.Format("2006-01-02 15:04:05")
		fmt.Fprintf(w, "## [%s] %s\n\n", ts, msg.Role)

		if text := strings.TrimSpace(msg.Text); text != "" {
			fmt.Fprintf(w, "%s\n\n", text)
		}

		for _, tc := range msg.ToolCalls {
			fmt.Fprintf(w, "### Tool Call: %s (`%s`)\n\n", tc.Name, tc.ID)

			// Pretty-print arguments JSON.
			args := prettyJSON(tc.Arguments)
			fmt.Fprintf(w, "**Arguments:**\n```json\n%s\n```\n\n", args)

			if tc.Result != "" {
				result := tc.Result
				if tc.IsError {
					fmt.Fprintf(w, "**Error:**\n```\n%s\n```\n\n", result)
				} else {
					// Truncate very long results for readability.
					const maxResult = 2000
					runes := []rune(result)
					if len(runes) > maxResult {
						result = string(runes[:maxResult]) + "\n… (truncated)"
					}
					fmt.Fprintf(w, "**Result:**\n```\n%s\n```\n\n", result)
				}
				if tc.DurationMs > 0 {
					fmt.Fprintf(w, "*Duration: %.0f ms*\n\n", tc.DurationMs)
				}
			}
		}

		for _, tr := range msg.ToolResults {
			fmt.Fprintf(w, "### Tool Result: `%s`\n\n", tr.ToolUseID)
			if tr.Content != "" {
				label := "Result"
				if tr.IsError {
					label = "Error"
				}
				fmt.Fprintf(w, "**%s:**\n```\n%s\n```\n\n", label, tr.Content)
			}
		}
	}
	return nil
}

// ToJSON writes session as JSON to w.
// The output structure mirrors the internal parser.Session model exactly.
func ToJSON(w io.Writer, session *parser.Session) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(session)
}

// prettyJSON attempts to pretty-print a JSON string.
// Falls back to the original string on parse failure.
func prettyJSON(raw string) string {
	if raw == "" || raw == "{}" {
		return "{}"
	}
	var v interface{}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return raw
	}
	return string(b)
}
