// Package tui provides a full-screen terminal UI for browsing session history.
package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/a2d2-dev/cc-history/internal/parser"
)

// --------------------------------------------------------------------------
// Styles
// --------------------------------------------------------------------------

var (
	styleUser      = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)   // cyan
	styleAssistant = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)   // green
	styleTool      = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))              // yellow
	styleToolFold  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))              // dark gray
	styleHelp      = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true) // gray italic
	styleHeader    = lipgloss.NewStyle().Bold(true)
	styleBorder    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleDim       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// --------------------------------------------------------------------------
// Model
// --------------------------------------------------------------------------

// item represents a single rendered line (or block) in the viewport.
type item struct {
	text      string // rendered line text
	msgIndex  int    // which message this item belongs to (-1 = separator/meta)
	toolIndex int    // which tool call within that message (-1 = not a tool line)
}

// model is the bubbletea model for the TUI.
type model struct {
	session     *parser.Session
	items       []item
	expanded    map[int]bool // msgIndex -> expanded (tool calls)
	cursor      int          // viewport top line index
	height      int          // terminal height
	width       int          // terminal width
	showHelp    bool
	totalLines  int
}

// --------------------------------------------------------------------------
// Launch
// --------------------------------------------------------------------------

// RunSession starts the TUI for a single session and blocks until the user quits.
func RunSession(session *parser.Session) error {
	m := newModel(session)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newModel(session *parser.Session) model {
	m := model{
		session:  session,
		expanded: make(map[int]bool),
		height:   24,
		width:    80,
	}
	m.rebuildItems()
	return m
}

// --------------------------------------------------------------------------
// bubbletea interface
// --------------------------------------------------------------------------

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
		m.rebuildItems()
		m.clampCursor()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q", "ctrl+c":
			return m, tea.Quit

		case "?":
			m.showHelp = !m.showHelp

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			maxTop := m.maxCursor()
			if m.cursor < maxTop {
				m.cursor++
			}

		case "pgup", "b", "ctrl+u":
			m.cursor -= m.viewHeight()
			if m.cursor < 0 {
				m.cursor = 0
			}

		case "pgdown", "f", "ctrl+d":
			m.cursor += m.viewHeight()
			m.clampCursor()

		case "home", "g":
			m.cursor = 0

		case "end", "G":
			m.cursor = m.maxCursor()

		case "t", "T":
			// Toggle tool calls for the message at the visible center of view.
			msgIdx := m.focusedMsgIndex()
			if msgIdx >= 0 {
				m.expanded[msgIdx] = !m.expanded[msgIdx]
				m.rebuildItems()
				m.clampCursor()
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.showHelp {
		return m.helpView()
	}
	return m.mainView()
}

// --------------------------------------------------------------------------
// Rendering
// --------------------------------------------------------------------------

func (m model) mainView() string {
	vh := m.viewHeight()
	lines := make([]string, 0, vh)

	end := m.cursor + vh
	if end > len(m.items) {
		end = len(m.items)
	}
	for _, it := range m.items[m.cursor:end] {
		lines = append(lines, it.text)
	}

	// Pad to fill terminal.
	for len(lines) < vh {
		lines = append(lines, "")
	}

	statusBar := m.statusBar()
	return strings.Join(lines, "\n") + "\n" + statusBar
}

func (m model) statusBar() string {
	pct := 0
	if len(m.items) > 0 {
		pct = (m.cursor + m.viewHeight()) * 100 / len(m.items)
		if pct > 100 {
			pct = 100
		}
	}
	hint := styleHelp.Render("↑↓/jk scroll · t toggle tools · ? help · q quit")
	pos := styleDim.Render(fmt.Sprintf(" %d%%", pct))
	gap := m.width - lipgloss.Width(hint) - lipgloss.Width(pos)
	if gap < 0 {
		gap = 0
	}
	return hint + strings.Repeat(" ", gap) + pos
}

func (m model) helpView() string {
	lines := []string{
		styleHeader.Render("cc-history TUI — keyboard shortcuts"),
		styleBorder.Render(strings.Repeat("─", 40)),
		"",
		"  ↑ / k        scroll up",
		"  ↓ / j        scroll down",
		"  PgUp / b     scroll up one page",
		"  PgDn / f     scroll down one page",
		"  g / Home     go to top",
		"  G / End      go to bottom",
		"  t            toggle tool calls for focused message",
		"  ?            toggle this help",
		"  q            quit",
	}
	body := strings.Join(lines, "\n")
	vh := m.viewHeight()
	bodyLines := strings.Count(body, "\n") + 1
	for i := bodyLines; i < vh; i++ {
		body += "\n"
	}
	return body + "\n" + styleHelp.Render("press ? to close help")
}

// --------------------------------------------------------------------------
// Item builder
// --------------------------------------------------------------------------

func (m *model) rebuildItems() {
	m.items = nil
	for i, msg := range m.session.Messages {
		if msg.Role == "" {
			continue
		}
		m.items = append(m.items, m.renderMessage(i, msg)...)
	}
	m.totalLines = len(m.items)
}

func (m *model) renderMessage(idx int, msg *parser.Message) []item {
	var items []item

	ts := msg.Timestamp.Format("15:04:05")
	role, roleStyle := roleLabel(msg.Role)

	// Header line.
	header := fmt.Sprintf("[%s]  %s", styleDim.Render(ts), roleStyle.Render(role))

	// Text content.
	if text := strings.TrimSpace(msg.Text); text != "" {
		wrapped := wordWrap(text, m.width-12) // leave room for indent
		for j, line := range strings.Split(wrapped, "\n") {
			prefix := "        "
			if j == 0 {
				prefix = header + "  "
			}
			items = append(items, item{text: prefix + line, msgIndex: idx, toolIndex: -1})
		}
	} else {
		items = append(items, item{text: header, msgIndex: idx, toolIndex: -1})
	}

	// Tool calls.
	for ti, tc := range msg.ToolCalls {
		items = append(items, m.renderToolCall(idx, ti, tc)...)
	}

	return items
}

func (m *model) renderToolCall(msgIdx, toolIdx int, tc *parser.ToolCall) []item {
	expanded := m.expanded[msgIdx]

	// Folded summary line.
	sym := "▶"
	if expanded {
		sym = "▼"
	}
	foldStyle := styleToolFold
	if expanded {
		foldStyle = styleTool
	}
	summary := foldStyle.Render(fmt.Sprintf("  %s [tool] %s %s", sym, tc.Name, shortArgs(tc.Arguments)))
	items := []item{{text: summary, msgIndex: msgIdx, toolIndex: toolIdx}}

	if !expanded {
		return items
	}

	// Expanded: show arguments.
	if tc.Arguments != "" && tc.Arguments != "{}" {
		pretty := prettyJSON(tc.Arguments)
		for _, line := range strings.Split(pretty, "\n") {
			items = append(items, item{
				text:      styleTool.Render("     │ ") + line,
				msgIndex:  msgIdx,
				toolIndex: toolIdx,
			})
		}
	}

	// Expanded: show result.
	if tc.Result != "" {
		label := styleToolFold.Render("  result: ")
		wrapped := wordWrap(tc.Result, m.width-14)
		for j, line := range strings.Split(wrapped, "\n") {
			prefix := "           "
			if j == 0 {
				prefix = label
			}
			items = append(items, item{
				text:      prefix + line,
				msgIndex:  msgIdx,
				toolIndex: toolIdx,
			})
		}
	}

	return items
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func roleLabel(role string) (string, lipgloss.Style) {
	switch role {
	case "user":
		return "user", styleUser
	case "assistant":
		return "asst", styleAssistant
	default:
		return fmt.Sprintf("%-4s", role), styleTool
	}
}

func shortArgs(raw string) string {
	if raw == "" || raw == "{}" {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		if len(raw) > 60 {
			return raw[:60] + "…"
		}
		return raw
	}
	var parts []string
	count := 0
	for k, v := range m {
		if count >= 2 {
			parts = append(parts, "…")
			break
		}
		vs := fmt.Sprintf("%v", v)
		vs = strings.ReplaceAll(vs, "\n", "\\n")
		if len(vs) > 40 {
			vs = vs[:40] + "…"
		}
		parts = append(parts, k+"="+vs)
		count++
	}
	return strings.Join(parts, " ")
}

func prettyJSON(raw string) string {
	var v interface{}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	b, err := json.MarshalIndent(v, "     ", "  ")
	if err != nil {
		return raw
	}
	return string(b)
}

// wordWrap wraps s at maxWidth runes.
func wordWrap(s string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = 80
	}
	var sb strings.Builder
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			sb.WriteByte('\n')
			continue
		}
		lineLen := 0
		for i, w := range words {
			wlen := len([]rune(w))
			if lineLen > 0 && lineLen+1+wlen > maxWidth {
				sb.WriteByte('\n')
				lineLen = 0
			} else if i > 0 {
				sb.WriteByte(' ')
				lineLen++
			}
			sb.WriteString(w)
			lineLen += wlen
		}
		sb.WriteByte('\n')
	}
	result := sb.String()
	// Trim trailing newline added by the loop.
	return strings.TrimRight(result, "\n")
}

func (m model) viewHeight() int {
	h := m.height - 1 // reserve status bar
	if h < 1 {
		return 1
	}
	return h
}

func (m model) maxCursor() int {
	max := len(m.items) - m.viewHeight()
	if max < 0 {
		return 0
	}
	return max
}

func (m *model) clampCursor() {
	max := m.maxCursor()
	if m.cursor > max {
		m.cursor = max
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// focusedMsgIndex returns the message index at the approximate center of the
// visible viewport, used for the `t` key binding.
func (m model) focusedMsgIndex() int {
	mid := m.cursor + m.viewHeight()/2
	if mid >= len(m.items) {
		mid = len(m.items) - 1
	}
	if mid < 0 {
		return -1
	}
	return m.items[mid].msgIndex
}
