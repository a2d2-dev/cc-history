package display

import (
	"fmt"
	"io"
	"sort"

	"github.com/a2d2-dev/cc-history/internal/parser"
)

// sessionMsg pairs a message with its originating session for cross-session output.
type sessionMsg struct {
	session *parser.Session
	msg     *parser.Message
}

// mergeAllSessions collects all messages from every session and sorts them by
// timestamp (oldest first). Messages with an equal timestamp preserve their
// original order within each session, sessions ordered by their first message.
func mergeAllSessions(sessions []*parser.Session) []sessionMsg {
	var all []sessionMsg
	for _, s := range sessions {
		for _, m := range s.Messages {
			all = append(all, sessionMsg{session: s, msg: m})
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].msg.Timestamp.Before(all[j].msg.Timestamp)
	})
	return all
}

// printSeparator writes a session boundary separator line.
// Format: --- session <id>  <time>  <dir> ---
func printSeparator(w io.Writer, s *parser.Session) {
	// Find the first message with a non-zero timestamp to extract time and CWD.
	var ts, cwd string
	for _, m := range s.Messages {
		if !m.Timestamp.IsZero() {
			ts = m.Timestamp.Format("2006-01-02 15:04:05")
			cwd = m.CWD
			break
		}
	}
	id := s.ID
	if id == "" {
		id = s.FilePath
	}
	fmt.Fprintf(w, "--- session %s  %s  %s ---\n", id, ts, cwd)
}

// PrintAllSessions writes messages from all sessions to w in global
// chronological order. A separator line is printed whenever the session
// changes, unless noSep is true.
func PrintAllSessions(w io.Writer, sessions []*parser.Session, noSep bool) {
	all := mergeAllSessions(sessions)
	var currentSession *parser.Session
	for _, sm := range all {
		if sm.msg.Role == "" {
			continue
		}
		if !noSep && sm.session != currentSession {
			printSeparator(w, sm.session)
			currentSession = sm.session
		} else if noSep {
			currentSession = sm.session
		}
		printMessage(w, sm.msg)
	}
}

// FilterAllSessions writes messages from all sessions that match pattern,
// with optional context lines. Separators are printed at session boundaries
// (unless noSep is true) and "--" is printed between non-contiguous groups.
func FilterAllSessions(w io.Writer, sessions []*parser.Session, pattern string, opts FilterOptions, noSep bool) error {
	match, err := buildMatcher(pattern, opts.UseRegex)
	if err != nil {
		return err
	}

	all := mergeAllSessions(sessions)

	// Build visible slice (messages with a role).
	type visEntry struct {
		idx     int // index into all
		session *parser.Session
	}
	visible := make([]visEntry, 0, len(all))
	for i, sm := range all {
		if sm.msg.Role != "" {
			visible = append(visible, visEntry{idx: i, session: sm.session})
		}
	}

	// Mark matches.
	isMatch := make([]bool, len(visible))
	for vi, ve := range visible {
		isMatch[vi] = match(messageText(all[ve.idx].msg))
	}

	// Expand with before/after context.
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
				continue
			}
			needSep := lastIncluded >= 0 && ci > lastIncluded+1
			entries = append(entries, printEntry{visibleIdx: ci, sep: needSep})
			lastIncluded = ci
		}
	}

	var currentSession *parser.Session
	for _, e := range entries {
		ve := visible[e.visibleIdx]
		sm := all[ve.idx]

		// Session separator (between sessions, not between non-contiguous groups).
		if !noSep && sm.session != currentSession {
			if e.sep {
				// Already printing "--", replace with full session header only when
				// the group separator and session change coincide.
				printSeparator(w, sm.session)
			} else {
				printSeparator(w, sm.session)
			}
			currentSession = sm.session
		} else {
			if e.sep {
				fmt.Fprintln(w, "--")
			}
		}
		printMessage(w, sm.msg)
	}
	return nil
}
