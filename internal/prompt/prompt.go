// Package prompt extracts and reconstructs prompt templates from session messages.
package prompt

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/a2d2-dev/cc-history/internal/parser"
)

// Options controls prompt reconstruction output.
type Options struct {
	// Start and End are 1-indexed, inclusive bounds of the message range.
	// A "message" counts each user or assistant role entry.
	Start int
	End   int
	// MaxSummaryRunes truncates long assistant responses. 0 means 400.
	MaxSummaryRunes int
}

// Result is the output of a prompt reconstruction.
type Result struct {
	// CWD is the working directory from the first message in range.
	CWD string
	// Date is the formatted date from the first message in range.
	Date string
	// GitBranch is the git branch from the first message in range.
	GitBranch string
	// Turns holds each in-range conversation turn.
	Turns []Turn
	// ReconstructedPrompt is the combined user inputs formatted as a reusable prompt.
	ReconstructedPrompt string
}

// Turn represents one message in the conversation.
type Turn struct {
	Index   int    // 1-based index across all role messages in the session
	Role    string // "user" or "assistant"
	Text    string // full text for user; summary for assistant
	Summary bool   // true when assistant text has been truncated
}

// Build extracts the requested range from session and returns a Result.
// Messages are indexed 1-based across all entries with a non-empty Role.
func Build(session *parser.Session, opts Options) (*Result, error) {
	if opts.MaxSummaryRunes == 0 {
		opts.MaxSummaryRunes = 400
	}

	// Collect role messages (user + assistant) with a 1-based index.
	type indexed struct {
		idx int
		msg *parser.Message
	}
	var roleMessages []indexed
	for _, m := range session.Messages {
		if m.Role == "" {
			continue
		}
		roleMessages = append(roleMessages, indexed{idx: len(roleMessages) + 1, msg: m})
	}

	if len(roleMessages) == 0 {
		return nil, fmt.Errorf("session has no messages")
	}

	end := opts.End
	if end == 0 || end > len(roleMessages) {
		end = len(roleMessages)
	}
	start := opts.Start
	if start < 1 {
		start = 1
	}
	if start > end {
		return nil, fmt.Errorf("invalid range %d-%d: start > end", opts.Start, opts.End)
	}

	result := &Result{}

	var userInputs []string

	for _, rm := range roleMessages {
		if rm.idx < start || rm.idx > end {
			continue
		}

		msg := rm.msg
		text := strings.TrimSpace(msg.Text)

		// Capture context from the first message in range.
		if result.CWD == "" && msg.CWD != "" {
			result.CWD = msg.CWD
		}
		if result.Date == "" && !msg.Timestamp.IsZero() {
			result.Date = msg.Timestamp.Format("2006-01-02")
		}
		if result.GitBranch == "" && msg.GitBranch != "" {
			result.GitBranch = msg.GitBranch
		}

		turn := Turn{
			Index: rm.idx,
			Role:  msg.Role,
		}

		switch msg.Role {
		case "user":
			turn.Text = text
			if text != "" {
				userInputs = append(userInputs, text)
			}
		case "assistant":
			runes := []rune(text)
			if utf8.RuneCountInString(text) > opts.MaxSummaryRunes {
				turn.Text = string(runes[:opts.MaxSummaryRunes]) + "…"
				turn.Summary = true
			} else {
				turn.Text = text
			}
		}

		result.Turns = append(result.Turns, turn)
	}

	if len(result.Turns) == 0 {
		return nil, fmt.Errorf("range %d-%d is out of bounds (session has %d messages)", start, end, len(roleMessages))
	}

	result.ReconstructedPrompt = buildPrompt(result, userInputs)
	return result, nil
}

// buildPrompt assembles the reusable prompt template from the extracted data.
func buildPrompt(r *Result, userInputs []string) string {
	var sb strings.Builder

	if r.CWD != "" || r.Date != "" || r.GitBranch != "" {
		sb.WriteString("## Context\n\n")
		if r.CWD != "" {
			fmt.Fprintf(&sb, "- Working directory: `%s`\n", r.CWD)
		}
		if r.Date != "" {
			fmt.Fprintf(&sb, "- Date: %s\n", r.Date)
		}
		if r.GitBranch != "" {
			fmt.Fprintf(&sb, "- Branch: `%s`\n", r.GitBranch)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Prompt\n\n")
	for i, input := range userInputs {
		if len(userInputs) > 1 {
			fmt.Fprintf(&sb, "### Step %d\n\n", i+1)
		}
		sb.WriteString(input)
		sb.WriteString("\n\n")
	}

	return strings.TrimRight(sb.String(), "\n") + "\n"
}

// Write formats a Result as human-readable text and writes it to w.
func Write(w io.Writer, r *Result) error {
	// Header
	fmt.Fprintf(w, "## Conversation\n\n")
	if r.CWD != "" {
		fmt.Fprintf(w, "- **Working directory:** `%s`\n", r.CWD)
	}
	if r.Date != "" {
		fmt.Fprintf(w, "- **Date:** %s\n", r.Date)
	}
	if r.GitBranch != "" {
		fmt.Fprintf(w, "- **Branch:** `%s`\n", r.GitBranch)
	}
	fmt.Fprintln(w)

	// Turns
	for _, t := range r.Turns {
		label := strings.Title(t.Role) //nolint:staticcheck
		summaryNote := ""
		if t.Summary {
			summaryNote = " (summary)"
		}
		fmt.Fprintf(w, "### [%d] %s%s\n\n", t.Index, label, summaryNote)
		if t.Text != "" {
			fmt.Fprintf(w, "%s\n\n", t.Text)
		}
	}

	// Reconstructed prompt
	fmt.Fprintf(w, "---\n\n## Reconstructed Prompt\n\n")
	fmt.Fprint(w, r.ReconstructedPrompt)
	return nil
}
