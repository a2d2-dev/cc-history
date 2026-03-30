// Package tui provides a full-screen terminal UI for browsing session history.
package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/a2d2-dev/cc-history/internal/config"
	"github.com/a2d2-dev/cc-history/internal/loader"
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
	styleMatch     = lipgloss.NewStyle().Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0"))  // yellow bg
	stylePickSel   = lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15")) // blue bg
	stylePickBox   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	styleRemoved   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // red  (diff -)
	styleAdded     = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green (diff +)
	styleArchive   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)                       // red bold
	styleOK        = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))                                  // green
)

// --------------------------------------------------------------------------
// Mode
// --------------------------------------------------------------------------

type tuiMode int

const (
	modeNormal  tuiMode = iota
	modePicker          // session list overlay
	modeSearch          // inline search
	modeInfo            // session info modal
	modeHelp            // help modal overlay
	modeRename          // inline rename input (status bar)
	modeConfirm         // y/n confirmation prompt (e.g. duplicate)
	modeRepath          // inline text input for changing session CWD
)

// --------------------------------------------------------------------------
// Grouped-view types
// --------------------------------------------------------------------------

// rowKind distinguishes group headers from individual session rows in the
// grouped session list.
type rowKind int

const (
	rowKindGroup   rowKind = iota // collapsible CWD group header
	rowKindSession                // individual session entry within a group
)

// displayRow is one rendered entry in the grouped session list.
type displayRow struct {
	kind       rowKind
	cwd        string // abbreviated group path (both types)
	sessionIdx int    // index into model.sessions; -1 for group rows
	count      int    // for group rows: number of sessions in group
}

// --------------------------------------------------------------------------
// Archive undo record
// --------------------------------------------------------------------------

// archiveAction records a single archive or restore operation for undo.
type archiveAction struct {
	fromPath    string // original path
	toPath      string // destination path
	wasArchived bool   // true = archive action, false = restore action
}

// --------------------------------------------------------------------------
// Model
// --------------------------------------------------------------------------

// item represents a single rendered line (or block) in the viewport.
type item struct {
	text      string // rendered line text (may contain ANSI)
	plain     string // stripped version for search matching
	msgIndex  int    // which message this item belongs to (-1 = separator/meta)
	toolIndex int    // which tool call within that message (-1 = not a tool line)
}

// model is the bubbletea model for the TUI.
type model struct {
	sessions   []*parser.Session
	sessionIdx int
	items      []item
	expanded   map[int]bool // msgIndex -> expanded (tool call details)
	showTools  bool         // whether tool call lines are visible at all
	cursor     int          // viewport top line index
	height     int          // terminal height
	width      int          // terminal width
	totalLines int

	// search
	mode        tuiMode
	searchQuery string
	matches     []int // item indices that match the search query
	matchCursor int   // current position in matches

	// session picker (flat list)
	pickCursor int

	// grouped session list
	groupedPicker  bool            // true = show grouped picker, false = flat picker
	expandedGroups map[string]bool // cwd -> expanded; persists for the session
	groupRows      []displayRow    // computed rows for grouped view (rebuilt on toggle)
	groupPickCursor int            // cursor position in groupRows

	// archive / restore
	sessionsRoot string         // live sessions root (e.g. ~/.claude/projects)
	showArchived bool           // true = currently viewing archived sessions
	lastAction   *archiveAction // last archive/restore action (for undo)
	statusMsg    string         // transient one-line status displayed in status bar

	// rename
	names            map[string]string // sessionID -> custom name (loaded from disk)
	renameInput      string            // current text in rename input
	renamingSessionID string           // session being renamed
	renameReturnMode tuiMode           // mode to restore after rename completes

	// duplicate confirm
	duplicatingSessionID string  // session pending duplication confirmation
	confirmReturnMode    tuiMode // mode to restore if confirm is cancelled

	// repath
	repathInput     string // current typed CWD in repath mode
	repathSessionID string // session being repathed
}

// resumeFinishedMsg is sent back by the ExecProcess callback after claude exits.
type resumeFinishedMsg struct{ err error }

// session returns the currently active session.
func (m model) session() *parser.Session {
	if len(m.sessions) == 0 {
		return nil
	}
	return m.sessions[m.sessionIdx]
}

// currentRoot returns the root directory being viewed (live or archive).
func (m model) currentRoot() string {
	if m.showArchived {
		return loader.ArchiveRoot(m.sessionsRoot)
	}
	return m.sessionsRoot
}

// --------------------------------------------------------------------------
// Launch
// --------------------------------------------------------------------------

// RunTUI starts the TUI with a list of sessions, opening the one at currentIdx.
// sessionsRoot is the live sessions directory (used for archive operations).
// It blocks until the user quits.
func RunTUI(sessions []*parser.Session, currentIdx int, sessionsRoot string) error {
	if len(sessions) == 0 {
		return fmt.Errorf("no sessions to display")
	}
	if currentIdx < 0 || currentIdx >= len(sessions) {
		currentIdx = 0
	}
	m := newModelMulti(sessions, currentIdx, sessionsRoot)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// RunSession starts the TUI for a single session and blocks until the user quits.
// Kept for backward compatibility.
func RunSession(session *parser.Session) error {
	return RunTUI([]*parser.Session{session}, 0, "")
}

func newModelMulti(sessions []*parser.Session, idx int, sessionsRoot string) model {
	cfg := config.Load()
	m := model{
		sessions:       sessions,
		sessionIdx:     idx,
		expanded:       make(map[int]bool),
		expandedGroups: make(map[string]bool),
		showTools:      cfg.ShowTools,
		height:         24,
		width:          80,
		sessionsRoot:   sessionsRoot,
		names:          config.LoadNames(),
	}
	m.rebuildItems()
	return m
}

// newModel creates a model from a single session (used by tests).
func newModel(session *parser.Session) model {
	return newModelMulti([]*parser.Session{session}, 0, "")
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

	case resumeFinishedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("resume error: %v", msg.err)
		} else {
			m.statusMsg = "returned from claude"
		}
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modePicker:
			return m.updatePicker(msg)
		case modeSearch:
			return m.updateSearch(msg)
		case modeInfo:
			return m.updateInfo(msg)
		case modeHelp:
			return m.updateHelp(msg)
		case modeRename:
			return m.updateRename(msg)
		case modeConfirm:
			return m.updateConfirm(msg)
		case modeRepath:
			return m.updateRepath(msg)
		default:
			return m.updateNormal(msg)
		}
	}
	return m, nil
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear transient status on any key.
	m.statusMsg = ""

	switch msg.String() {
	case "q", "Q", "ctrl+c":
		return m, tea.Quit

	case "?":
		m.mode = modeHelp

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

	case "t":
		m.showTools = !m.showTools
		cfg := config.Load()
		cfg.ShowTools = m.showTools
		config.Save(cfg) //nolint:errcheck // best-effort persistence
		m.rebuildItems()
		m.clampCursor()

	case "T":
		if m.showTools {
			msgIdx := m.focusedMsgIndex()
			if msgIdx >= 0 {
				m.expanded[msgIdx] = !m.expanded[msgIdx]
				m.rebuildItems()
				m.clampCursor()
			}
		}

	case "s":
		if len(m.sessions) > 1 {
			m.mode = modePicker
			m.groupedPicker = false
			m.pickCursor = m.sessionIdx
		}

	case "tab":
		if len(m.sessions) > 1 {
			m.mode = modePicker
			m.groupedPicker = true
			m.buildGroupRows()
		}

	case "/":
		m.mode = modeSearch
		m.searchQuery = ""
		m.matches = nil
		m.matchCursor = 0

	case "n":
		if len(m.matches) > 0 {
			m.matchCursor = (m.matchCursor + 1) % len(m.matches)
			m.scrollToMatch()
		} else if m.searchQuery == "" {
			if sess := m.session(); sess != nil {
				m = m.startRename(sess.ID, modeNormal)
			}
		}

	case "N":
		if len(m.matches) > 0 {
			m.matchCursor = (m.matchCursor - 1 + len(m.matches)) % len(m.matches)
			m.scrollToMatch()
		}

	case "i":
		m.mode = modeInfo

	case "a":
		m = m.doArchive()

	case "A":
		m = m.doToggleArchiveView()

	case "u":
		m = m.doUndo()

	case "r":
		return m.doResume()

	case "d":
		if sess := m.session(); sess != nil {
			m = m.startConfirmDuplicate(sess.ID, modeNormal)
		}

	case "p":
		if sess := m.session(); sess != nil {
			m.repathSessionID = sess.ID
			m.repathInput = sessionCWD(sess)
			m.mode = modeRepath
		}
	}
	return m, nil
}

// doArchive archives (or restores) the currently selected session.
func (m model) doArchive() model {
	if m.sessionsRoot == "" {
		m.statusMsg = "archive unavailable: no session root"
		return m
	}
	sess := m.session()
	if sess == nil {
		return m
	}

	var dstPath string
	var err error
	var action archiveAction

	if m.showArchived {
		// Currently in archive view → restore to live.
		dstPath, err = loader.RestoreSession(m.sessionsRoot, sess.FilePath)
		if err != nil {
			m.statusMsg = fmt.Sprintf("restore failed: %v", err)
			return m
		}
		action = archiveAction{
			fromPath:    sess.FilePath,
			toPath:      dstPath,
			wasArchived: false,
		}
		m.statusMsg = fmt.Sprintf("restored: %s", shortPath(dstPath))
	} else {
		// Currently in live view → archive.
		dstPath, err = loader.ArchiveSession(m.sessionsRoot, sess.FilePath)
		if err != nil {
			m.statusMsg = fmt.Sprintf("archive failed: %v", err)
			return m
		}
		action = archiveAction{
			fromPath:    sess.FilePath,
			toPath:      dstPath,
			wasArchived: true,
		}
		m.statusMsg = fmt.Sprintf("archived: %s", shortPath(dstPath))
	}

	m.lastAction = &action
	// Remove the session from the current list and update the index.
	m.sessions = append(m.sessions[:m.sessionIdx], m.sessions[m.sessionIdx+1:]...)
	if m.sessionIdx >= len(m.sessions) && m.sessionIdx > 0 {
		m.sessionIdx--
	}
	m.expanded = make(map[int]bool)
	m.cursor = 0
	m.matches = nil
	m.matchCursor = 0
	m.rebuildItems()
	return m
}

// doToggleArchiveView switches between live and archived session views.
func (m model) doToggleArchiveView() model {
	if m.sessionsRoot == "" {
		m.statusMsg = "archive unavailable: no session root"
		return m
	}
	m.showArchived = !m.showArchived
	newRoot := m.currentRoot()
	newSessions, err := loader.LoadAllSessions(newRoot)
	if err != nil || len(newSessions) == 0 {
		// Toggle back on empty archive; show message.
		m.showArchived = !m.showArchived
		if m.showArchived {
			m.statusMsg = "no archived sessions found"
		} else {
			m.statusMsg = fmt.Sprintf("load error: %v", err)
		}
		return m
	}
	m.sessions = newSessions
	m.sessionIdx = len(newSessions) - 1 // most recent
	m.expanded = make(map[int]bool)
	m.cursor = 0
	m.searchQuery = ""
	m.matches = nil
	m.matchCursor = 0
	m.rebuildItems()
	if m.showArchived {
		m.statusMsg = fmt.Sprintf("archive view: %d sessions", len(newSessions))
	} else {
		m.statusMsg = fmt.Sprintf("live view: %d sessions", len(newSessions))
	}
	return m
}

// doUndo reverses the last archive or restore operation.
func (m model) doUndo() model {
	if m.lastAction == nil {
		m.statusMsg = "nothing to undo"
		return m
	}
	act := m.lastAction

	// Determine which direction to move: from toPath back to fromPath.
	srcRoot := m.currentRoot()
	var err error
	var restoredPath string
	if act.wasArchived {
		// Last action was archive (live→archive). Undo = restore (archive→live).
		restoredPath, err = loader.RestoreSession(m.sessionsRoot, act.toPath)
	} else {
		// Last action was restore (archive→live). Undo = re-archive (live→archive).
		restoredPath, err = loader.ArchiveSession(m.sessionsRoot, act.toPath)
	}
	if err != nil {
		m.statusMsg = fmt.Sprintf("undo failed: %v", err)
		return m
	}
	_ = srcRoot
	m.lastAction = nil

	// Reload the current view.
	newSessions, err := loader.LoadAllSessions(m.currentRoot())
	if err != nil {
		m.statusMsg = fmt.Sprintf("undo ok, reload error: %v", err)
		return m
	}
	m.sessions = newSessions
	// Try to restore the cursor to the affected session.
	m.sessionIdx = len(newSessions) - 1
	for i, s := range newSessions {
		if s.FilePath == restoredPath {
			m.sessionIdx = i
			break
		}
	}
	m.expanded = make(map[int]bool)
	m.cursor = 0
	m.matches = nil
	m.matchCursor = 0
	m.rebuildItems()
	m.statusMsg = fmt.Sprintf("undone: %s", shortPath(restoredPath))
	return m
}

// startRename enters rename mode for sessionID, recording returnMode to restore after.
func (m model) startRename(sessionID string, returnMode tuiMode) model {
	m.renamingSessionID = sessionID
	m.renameInput = m.names[sessionID] // pre-fill with existing custom name (or "")
	m.renameReturnMode = returnMode
	m.mode = modeRename
	return m
}

// updateRename handles key events while the rename input is active.
func (m model) updateRename(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		// Cancel without saving.
		m.mode = m.renameReturnMode
		m.renameInput = ""
		m.renamingSessionID = ""

	case "enter":
		// Save and return.
		name := strings.TrimSpace(m.renameInput)
		if m.renamingSessionID != "" {
			if err := config.SaveName(m.renamingSessionID, name); err != nil {
				m.statusMsg = fmt.Sprintf("rename failed: %v", err)
			} else {
				if name == "" {
					delete(m.names, m.renamingSessionID)
					m.statusMsg = "name cleared"
				} else {
					m.names[m.renamingSessionID] = name
					m.statusMsg = fmt.Sprintf("renamed: %s", name)
				}
			}
		}
		m.mode = m.renameReturnMode
		m.renameInput = ""
		m.renamingSessionID = ""

	case "backspace", "ctrl+h":
		if len(m.renameInput) > 0 {
			runes := []rune(m.renameInput)
			m.renameInput = string(runes[:len(runes)-1])
		}

	default:
		if len(msg.Runes) > 0 {
			m.renameInput += string(msg.Runes)
		}
	}
	return m, nil
}

// startConfirmDuplicate enters confirm mode for the given session ID.
func (m model) startConfirmDuplicate(sessionID string, returnMode tuiMode) model {
	m.duplicatingSessionID = sessionID
	m.confirmReturnMode = returnMode
	m.mode = modeConfirm
	return m
}

// updateConfirm handles key events while the y/n duplicate confirmation is active.
func (m model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "y", "Y":
		m = m.doDuplicate()
		m.duplicatingSessionID = ""
	case "esc", "n", "N", "q":
		m.statusMsg = "duplicate cancelled"
		m.duplicatingSessionID = ""
		m.mode = m.confirmReturnMode
	}
	return m, nil
}

// doDuplicate copies the session file with a new UUID and reloads the session list.
func (m model) doDuplicate() model {
	if m.sessionsRoot == "" {
		m.statusMsg = "duplicate unavailable: no session root"
		m.mode = m.confirmReturnMode
		return m
	}
	sess := m.session()
	if sess == nil || sess.ID != m.duplicatingSessionID {
		// Fall back to finding the target session.
		for _, s := range m.sessions {
			if s.ID == m.duplicatingSessionID {
				sess = s
				break
			}
		}
	}
	if sess == nil {
		m.statusMsg = "duplicate failed: session not found"
		m.mode = m.confirmReturnMode
		return m
	}

	_, newID, err := loader.DuplicateSession(sess.FilePath)
	if err != nil {
		m.statusMsg = fmt.Sprintf("duplicate failed: %v", err)
		m.mode = m.confirmReturnMode
		return m
	}

	// Reload sessions from disk.
	newSessions, err := loader.LoadAllSessions(m.currentRoot())
	if err != nil {
		m.statusMsg = fmt.Sprintf("duplicate ok (%s) but reload failed: %v", newID[:8], err)
		m.mode = m.confirmReturnMode
		return m
	}
	m.sessions = newSessions
	// Keep current session selected (find by original ID).
	for i, s := range m.sessions {
		if s.ID == sess.ID {
			m.sessionIdx = i
			break
		}
	}
	m.expanded = make(map[int]bool)
	m.cursor = 0
	m.matches = nil
	m.matchCursor = 0
	m.rebuildItems()
	m.statusMsg = fmt.Sprintf("duplicated: %s", newID[:8]+"…")
	m.mode = m.confirmReturnMode
	return m
}

func (m model) updatePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.groupedPicker {
		return m.updateGroupedPicker(msg)
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc", "q":
		m.mode = modeNormal

	case "tab":
		m.groupedPicker = true
		m.buildGroupRows()

	case "up", "k":
		if m.pickCursor > 0 {
			m.pickCursor--
		}

	case "down", "j":
		if m.pickCursor < len(m.sessions)-1 {
			m.pickCursor++
		}

	case "n":
		if m.pickCursor < len(m.sessions) {
			m = m.startRename(m.sessions[m.pickCursor].ID, modePicker)
		}

	case "enter":
		if m.pickCursor != m.sessionIdx {
			m.sessionIdx = m.pickCursor
			m.expanded = make(map[int]bool)
			m.cursor = 0
			m.searchQuery = ""
			m.matches = nil
			m.matchCursor = 0
			m.rebuildItems()
		}
		m.mode = modeNormal

	case "d":
		if m.pickCursor < len(m.sessions) {
			m = m.startConfirmDuplicate(m.sessions[m.pickCursor].ID, modePicker)
		}
	}
	return m, nil
}

func (m model) updateGroupedPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc", "q":
		m.mode = modeNormal

	case "tab":
		m.groupedPicker = false
		m.pickCursor = m.sessionIdx

	case "up", "k":
		if m.groupPickCursor > 0 {
			m.groupPickCursor--
		}

	case "down", "j":
		if m.groupPickCursor < len(m.groupRows)-1 {
			m.groupPickCursor++
		}

	case "n":
		if len(m.groupRows) > 0 {
			row := m.groupRows[m.groupPickCursor]
			if row.kind == rowKindSession && row.sessionIdx < len(m.sessions) {
				m = m.startRename(m.sessions[row.sessionIdx].ID, modePicker)
				m.groupedPicker = true
			}
		}

	case "left", "right", "enter":
		if len(m.groupRows) == 0 {
			break
		}
		row := m.groupRows[m.groupPickCursor]
		switch row.kind {
		case rowKindGroup:
			// Expand or collapse the group.
			m.expandedGroups[row.cwd] = !m.expandedGroups[row.cwd]
			m.buildGroupRows()
			// Keep cursor on the same group after rebuild.
			for i, r := range m.groupRows {
				if r.kind == rowKindGroup && r.cwd == row.cwd {
					m.groupPickCursor = i
					break
				}
			}
		case rowKindSession:
			// Select this session and return to message view.
			if row.sessionIdx != m.sessionIdx {
				m.sessionIdx = row.sessionIdx
				m.expanded = make(map[int]bool)
				m.cursor = 0
				m.searchQuery = ""
				m.matches = nil
				m.matchCursor = 0
				m.rebuildItems()
			}
			m.mode = modeNormal
		}

	case "d":
		if len(m.groupRows) > 0 {
			row := m.groupRows[m.groupPickCursor]
			if row.kind == rowKindSession && row.sessionIdx < len(m.sessions) {
				m = m.startConfirmDuplicate(m.sessions[row.sessionIdx].ID, modePicker)
			}
		}
	}
	return m, nil
}

func (m model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		m.mode = modeNormal
		m.searchQuery = ""
		m.matches = nil
		m.matchCursor = 0

	case "enter":
		// Confirm search, stay in search mode but allow n/N navigation.
		m.mode = modeNormal

	case "backspace", "ctrl+h":
		if len(m.searchQuery) > 0 {
			runes := []rune(m.searchQuery)
			m.searchQuery = string(runes[:len(runes)-1])
			m.recomputeMatches()
		}

	default:
		if len(msg.Runes) > 0 {
			m.searchQuery += string(msg.Runes)
			m.recomputeMatches()
			if len(m.matches) > 0 {
				m.matchCursor = 0
				m.scrollToMatch()
			}
		}
	}
	return m, nil
}

func (m model) updateInfo(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "i", "q":
		m.mode = modeNormal
	}
	return m, nil
}

func (m model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "?", "q":
		m.mode = modeNormal
	}
	return m, nil
}

// updateRepath handles key events while the inline CWD repath input is active.
func (m model) updateRepath(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.statusMsg = "repath cancelled"
		m.repathSessionID = ""
		m.mode = modeNormal
	case "enter":
		m = m.doRepath()
	case "backspace", "ctrl+h":
		if len(m.repathInput) > 0 {
			runes := []rune(m.repathInput)
			m.repathInput = string(runes[:len(runes)-1])
		}
	default:
		if len(msg.Runes) > 0 {
			m.repathInput += string(msg.Runes)
		}
	}
	return m, nil
}

// doRepath writes the new CWD into the session JSONL file and reloads.
func (m model) doRepath() model {
	if m.sessionsRoot == "" {
		m.statusMsg = "repath unavailable: no session root"
		m.mode = modeNormal
		return m
	}
	newCWD := strings.TrimSpace(m.repathInput)
	if newCWD == "" {
		m.statusMsg = "repath cancelled: empty path"
		m.repathSessionID = ""
		m.mode = modeNormal
		return m
	}

	var sess *parser.Session
	for _, s := range m.sessions {
		if s.ID == m.repathSessionID {
			sess = s
			break
		}
	}
	if sess == nil {
		m.statusMsg = "repath failed: session not found"
		m.repathSessionID = ""
		m.mode = modeNormal
		return m
	}

	oldCWD := sessionCWD(sess)
	if err := loader.RepathSession(sess.FilePath, oldCWD, newCWD); err != nil {
		m.statusMsg = fmt.Sprintf("repath failed: %v", err)
		m.repathSessionID = ""
		m.mode = modeNormal
		return m
	}

	// Reload sessions so grouping reflects the new CWD.
	newSessions, err := loader.LoadAllSessions(m.currentRoot())
	if err != nil {
		m.statusMsg = fmt.Sprintf("repath ok but reload failed: %v", err)
		m.repathSessionID = ""
		m.mode = modeNormal
		return m
	}
	// Re-select the same session by ID.
	newIdx := 0
	for i, s := range newSessions {
		if s.ID == sess.ID {
			newIdx = i
			break
		}
	}
	m.sessions = newSessions
	m.sessionIdx = newIdx
	m.repathSessionID = ""
	m.mode = modeNormal
	m.rebuildItems()
	m.clampCursor()
	m.statusMsg = fmt.Sprintf("repathed to: %s", newCWD)
	return m
}

// recomputeMatches finds all items matching the current searchQuery.
func (m *model) recomputeMatches() {
	m.matches = nil
	if m.searchQuery == "" {
		m.matchCursor = 0
		return
	}
	q := strings.ToLower(m.searchQuery)
	for i, it := range m.items {
		if strings.Contains(strings.ToLower(it.plain), q) {
			m.matches = append(m.matches, i)
		}
	}
	if m.matchCursor >= len(m.matches) {
		m.matchCursor = 0
	}
}

// scrollToMatch sets cursor so the current match is visible.
func (m *model) scrollToMatch() {
	if len(m.matches) == 0 {
		return
	}
	idx := m.matches[m.matchCursor]
	// If idx is outside the current viewport, scroll to center it.
	vh := m.viewHeight()
	if idx < m.cursor || idx >= m.cursor+vh {
		m.cursor = idx - vh/2
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.clampCursor()
	}
}

func (m model) View() string {
	switch m.mode {
	case modePicker:
		if m.groupedPicker {
			return m.groupedPickerView()
		}
		return m.pickerView()
	case modeInfo:
		return m.infoModalView()
	case modeHelp:
		return m.helpModalView()
	default:
		return m.mainView()
	}
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

	currentMatch := -1
	if len(m.matches) > 0 && m.matchCursor < len(m.matches) {
		currentMatch = m.matches[m.matchCursor]
	}

	for i, it := range m.items[m.cursor:end] {
		absIdx := m.cursor + i
		line := it.text
		// Highlight matched lines.
		if m.searchQuery != "" && strings.Contains(strings.ToLower(it.plain), strings.ToLower(m.searchQuery)) {
			if absIdx == currentMatch {
				line = styleMatch.Render(it.plain)
			} else {
				line = styleDim.Render(it.plain)
			}
		}
		lines = append(lines, line)
	}

	// Pad to fill terminal.
	for len(lines) < vh {
		lines = append(lines, "")
	}

	statusBar := m.statusBar()
	return strings.Join(lines, "\n") + "\n" + statusBar
}

func (m model) statusBar() string {
	// Rename mode shows an inline input line.
	if m.mode == modeRename {
		cursor := "█"
		hint := styleHelp.Render(fmt.Sprintf("Rename: %s%s  Enter to save · Esc to cancel", m.renameInput, cursor))
		pos := styleDim.Render(fmt.Sprintf(" %d%%", m.scrollPct()))
		gap := m.width - lipgloss.Width(hint) - lipgloss.Width(pos)
		if gap < 0 {
			gap = 0
		}
		return hint + strings.Repeat(" ", gap) + pos
	}

	// Transient status message takes priority.
	if m.statusMsg != "" {
		pos := styleDim.Render(fmt.Sprintf(" %d%%", m.scrollPct()))
		gap := m.width - lipgloss.Width(m.statusMsg) - lipgloss.Width(pos)
		if gap < 0 {
			gap = 0
		}
		return m.statusMsg + strings.Repeat(" ", gap) + pos
	}

	// Confirm mode shows an inline y/n prompt.
	if m.mode == modeConfirm {
		cursor := styleOK.Render("▋")
		hint := styleHelp.Render(fmt.Sprintf("Duplicate session? y to confirm · n/Esc to cancel%s", cursor))
		return hint
	}

	var hint string
	if m.mode == modeRepath {
		cursor := styleOK.Render("▋")
		hint = styleHelp.Render(fmt.Sprintf("repath cwd: %s%s  Enter confirm · Esc cancel", m.repathInput, cursor))
	} else if m.mode == modeSearch {
		matchInfo := ""
		if m.searchQuery != "" {
			matchInfo = fmt.Sprintf(" [%d matches]", len(m.matches))
		}
		hint = styleHelp.Render(fmt.Sprintf("search: %s%s  ESC cancel · Enter confirm · n/N jump", m.searchQuery, matchInfo))
	} else if m.searchQuery != "" {
		hint = styleHelp.Render(fmt.Sprintf("/%s  [%d/%d]  n next · N prev · / new search", m.searchQuery, m.matchCursor+1, len(m.matches)))
	} else {
		viewLabel := ""
		if m.showArchived {
			viewLabel = styleArchive.Render("[ARCHIVE]") + " "
		}
		sessionInfo := ""
		if len(m.sessions) > 1 {
			sessionInfo = fmt.Sprintf(" s list · Tab grouped(%d)", len(m.sessions))
		}
		toolsHint := "t hide tools"
		if !m.showTools {
			toolsHint = "t show tools"
		}
		hint = viewLabel + styleHelp.Render(fmt.Sprintf("↑↓/jk scroll · %s · T expand · r resume · n rename · d dup · p repath · a archive · A toggle · u undo · / search · i info%s · ? help · q quit", toolsHint, sessionInfo))
	}

	pos := styleDim.Render(fmt.Sprintf(" %d%%", m.scrollPct()))
	gap := m.width - lipgloss.Width(hint) - lipgloss.Width(pos)
	if gap < 0 {
		gap = 0
	}
	return hint + strings.Repeat(" ", gap) + pos
}

func (m model) scrollPct() int {
	if len(m.items) == 0 {
		return 100
	}
	pct := (m.cursor + m.viewHeight()) * 100 / len(m.items)
	if pct > 100 {
		pct = 100
	}
	return pct
}

func (m model) pickerView() string {
	sess := m.sessions
	vh := m.viewHeight()

	// Build picker lines.
	lines := make([]string, 0, len(sess)+4)
	title := styleHeader.Render(fmt.Sprintf("Sessions (%d)  ↑↓/jk navigate · n rename · Enter switch · Esc cancel", len(sess)))
	lines = append(lines, title)
	lines = append(lines, styleBorder.Render(strings.Repeat("─", min(m.width-2, 60))))
	lines = append(lines, "")

	// Visible window for the list.
	visStart := 0
	if m.pickCursor >= vh-4 {
		visStart = m.pickCursor - (vh - 5)
	}
	if visStart < 0 {
		visStart = 0
	}

	for i := visStart; i < len(sess) && i < visStart+(vh-4); i++ {
		s := sess[i]
		ts := m.sessionLabel(s)
		label := fmt.Sprintf("  %2d  %s", i+1, ts)
		if i == m.sessionIdx {
			label = fmt.Sprintf("  %2d  %s  ←current", i+1, ts)
		}
		if i == m.pickCursor {
			lines = append(lines, stylePickSel.Render(label))
		} else {
			lines = append(lines, label)
		}
	}

	// Pad.
	for len(lines) < vh {
		lines = append(lines, "")
	}

	return strings.Join(lines[:vh], "\n") + "\n" + styleHelp.Render("press Esc to cancel")
}

func (m model) sessionLabel(s *parser.Session) string {
	id := s.ID
	if len(id) > 16 {
		id = id[:16]
	}
	// Find first user message for preview.
	preview := ""
	ts := ""
	for _, msg := range s.Messages {
		if !msg.Timestamp.IsZero() && ts == "" {
			ts = msg.Timestamp.Format("2006-01-02 15:04")
		}
		if msg.Role == "user" && preview == "" {
			preview = strings.TrimSpace(msg.Text)
			if len(preview) > 50 {
				preview = preview[:50] + "…"
			}
		}
		if ts != "" && preview != "" {
			break
		}
	}
	if ts == "" {
		ts = "unknown"
	}
	// Use custom name when available; fall back to first-message preview.
	if custom, ok := m.names[s.ID]; ok && custom != "" {
		name := custom
		if len([]rune(name)) > 50 {
			name = string([]rune(name)[:50]) + "…"
		}
		return fmt.Sprintf("[%s]  %s", ts, name)
	}
	if preview == "" {
		preview = id
	}
	return fmt.Sprintf("[%s]  %s", ts, preview)
}

// helpModalView renders a centered help overlay showing all keybindings.
// Content is context-aware: view-mode shortcuts are listed first, then
// session-list shortcuts are shown only when multiple sessions are loaded.
func (m model) helpModalView() string {
	sep := styleBorder.Render(strings.Repeat("─", 42))

	type entry struct{ key, desc string }
	viewKeys := []entry{
		{"↑ / k", "scroll up"},
		{"↓ / j", "scroll down"},
		{"PgUp / b / C-u", "scroll up one page"},
		{"PgDn / f / C-d", "scroll down one page"},
		{"g / Home", "go to top"},
		{"G / End", "go to bottom"},
		{"t", "hide / show all tool calls"},
		{"T", "expand / collapse tool details for focused message"},
		{"i", "show session info"},
		{"r", "resume session (claude --resume)"},
		{"n", "rename session (custom title)"},
		{"d", "duplicate session (copy with new UUID)"},
		{"p", "repath session CWD (change working directory metadata)"},
		{"a", "archive session (restore in archive view)"},
		{"A", "toggle live / archive view"},
		{"u", "undo last archive / restore"},
		{"s", "open flat session list"},
		{"Tab", "open grouped session list"},
		{"/", "enter search mode"},
		{"n / N", "next / previous search match (or rename when no search)"},
		{"?", "close this help"},
		{"q / Q", "quit"},
	}
	listKeys := []entry{
		{"↑ / k", "move cursor up"},
		{"↓ / j", "move cursor down"},
		{"n", "rename selected session"},
		{"Enter", "select session"},
		{"Esc / q", "close list"},
	}
	searchKeys := []entry{
		{"type", "add to query"},
		{"Backspace", "delete last char"},
		{"Enter", "confirm and return to view"},
		{"Esc", "cancel search"},
	}

	renderSection := func(title string, entries []entry) []string {
		out := []string{"  " + styleHeader.Render(title)}
		for _, e := range entries {
			line := fmt.Sprintf("  %-18s %s", e.key, e.desc)
			out = append(out, line)
		}
		return out
	}

	var modalLines []string
	modalLines = append(modalLines, "")
	modalLines = append(modalLines, "  "+styleHeader.Render("cc-history — keyboard shortcuts"))
	modalLines = append(modalLines, "  "+sep)
	modalLines = append(modalLines, "")
	modalLines = append(modalLines, renderSection("View mode", viewKeys)...)
	if len(m.sessions) > 1 {
		modalLines = append(modalLines, "")
		modalLines = append(modalLines, "  "+sep)
		modalLines = append(modalLines, renderSection("Session list  (s)", listKeys)...)
	}
	modalLines = append(modalLines, "")
	modalLines = append(modalLines, "  "+sep)
	modalLines = append(modalLines, renderSection("Search mode  (/)", searchKeys)...)
	modalLines = append(modalLines, "")

	// Overlay modal on top of main view.
	main := m.mainView()
	mainLines := strings.Split(main, "\n")
	vh := m.viewHeight()

	startRow := (vh - len(modalLines)) / 2
	if startRow < 0 {
		startRow = 0
	}

	for i, ml := range modalLines {
		row := startRow + i
		if row >= len(mainLines) {
			break
		}
		plain := stripANSI(ml)
		pad := m.width - len([]rune(plain))
		if pad < 0 {
			pad = 0
		}
		mainLines[row] = ml + strings.Repeat(" ", pad)
	}

	return strings.Join(mainLines, "\n")
}

// infoModalView renders a centered overlay with session metadata.
func (m model) infoModalView() string {
	sess := m.session()
	if sess == nil {
		return m.mainView()
	}

	// Compute session stats.
	var firstTs, lastTs time.Time
	msgCount := 0
	cwd := ""
	for _, msg := range sess.Messages {
		if msg.Role == "" {
			continue
		}
		msgCount++
		if !msg.Timestamp.IsZero() {
			if firstTs.IsZero() || msg.Timestamp.Before(firstTs) {
				firstTs = msg.Timestamp
			}
			if msg.Timestamp.After(lastTs) {
				lastTs = msg.Timestamp
			}
		}
		if cwd == "" && msg.CWD != "" {
			cwd = msg.CWD
		}
	}

	fmtTs := func(t time.Time) string {
		if t.IsZero() {
			return "(unknown)"
		}
		return t.Format("2006-01-02 15:04:05")
	}
	if cwd == "" {
		cwd = "(unknown)"
	}

	rows := []struct{ label, value string }{
		{"Session ID", sess.ID},
		{"Project path", abbrevPath(cwd)},
		{"File path", sess.FilePath},
		{"Created", fmtTs(firstTs)},
		{"Last message", fmtTs(lastTs)},
		{"Messages", fmt.Sprintf("%d", msgCount)},
	}

	// Calculate modal width.
	labelW := 0
	valueW := 0
	for _, r := range rows {
		if len(r.label) > labelW {
			labelW = len(r.label)
		}
		if len(r.value) > valueW {
			valueW = len(r.value)
		}
	}
	maxValue := m.width - labelW - 7 // padding + border
	if maxValue < 20 {
		maxValue = 20
	}

	var bodyLines []string
	for _, r := range rows {
		val := r.value
		if len(val) > maxValue {
			val = "…" + val[len(val)-maxValue+1:]
		}
		line := fmt.Sprintf("  %-*s  %s", labelW, r.label, styleUser.Render(val))
		bodyLines = append(bodyLines, line)
	}

	title := styleHeader.Render("Session Info")
	sep := styleBorder.Render(strings.Repeat("─", labelW+valueW+6))
	modalLines := []string{"", "  " + title, "  " + sep, ""}
	for _, l := range bodyLines {
		modalLines = append(modalLines, l)
	}
	modalLines = append(modalLines, "")

	// Render over main view — replace center lines.
	main := m.mainView()
	mainLines := strings.Split(main, "\n")
	vh := m.viewHeight()

	modalHeight := len(modalLines)
	startRow := (vh - modalHeight) / 2
	if startRow < 0 {
		startRow = 0
	}

	for i, ml := range modalLines {
		row := startRow + i
		if row >= len(mainLines) {
			break
		}
		// Pad modal line to full width for clean overlay.
		plain := stripANSI(ml)
		pad := m.width - len([]rune(plain))
		if pad < 0 {
			pad = 0
		}
		mainLines[row] = ml + strings.Repeat(" ", pad)
	}

	return strings.Join(mainLines, "\n")
}

// --------------------------------------------------------------------------
// Item builder
// --------------------------------------------------------------------------

func (m *model) rebuildItems() {
	m.items = nil
	sess := m.session()
	if sess == nil {
		m.totalLines = 0
		return
	}
	for i, msg := range sess.Messages {
		if msg.Role == "" {
			continue
		}
		m.items = append(m.items, m.renderMessage(i, msg)...)
	}
	m.totalLines = len(m.items)
	// Recompute matches if search is active.
	if m.searchQuery != "" {
		m.recomputeMatches()
	}
}

func (m *model) renderMessage(idx int, msg *parser.Message) []item {
	var items []item

	ts := msg.Timestamp.Format("15:04:05")
	role, roleStyle := roleLabel(msg.Role)

	// Header line.
	header := fmt.Sprintf("[%s]  %s", styleDim.Render(ts), roleStyle.Render(role))
	headerPlain := fmt.Sprintf("[%s]  %s", ts, role)

	// Text content.
	if text := strings.TrimSpace(msg.Text); text != "" {
		wrapped := wordWrap(text, m.width-12) // leave room for indent
		for j, line := range strings.Split(wrapped, "\n") {
			prefix := "        "
			prefixPlain := "        "
			if j == 0 {
				prefix = header + "  "
				prefixPlain = headerPlain + "  "
			}
			items = append(items, item{
				text:      prefix + line,
				plain:     prefixPlain + line,
				msgIndex:  idx,
				toolIndex: -1,
			})
		}
	} else {
		items = append(items, item{
			text:      header,
			plain:     headerPlain,
			msgIndex:  idx,
			toolIndex: -1,
		})
	}

	// Tool calls (only when visible).
	if m.showTools {
		for ti, tc := range msg.ToolCalls {
			items = append(items, m.renderToolCall(idx, ti, tc)...)
		}
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
	summaryPlain := fmt.Sprintf("  %s [tool] %s %s", sym, tc.Name, shortArgs(tc.Arguments))
	summary := foldStyle.Render(summaryPlain)
	items := []item{{text: summary, plain: summaryPlain, msgIndex: msgIdx, toolIndex: toolIdx}}

	if !expanded {
		return items
	}

	// Expanded: show arguments (diff format for Edit/MultiEdit, JSON otherwise).
	items = append(items, m.renderToolArgs(msgIdx, toolIdx, tc)...)

	// Expanded: show result (omit if empty).
	if tc.Result != "" {
		label := styleToolFold.Render("  result: ")
		labelPlain := "  result: "
		wrapped := wordWrap(tc.Result, m.width-14)
		for j, line := range strings.Split(wrapped, "\n") {
			prefix := "           "
			prefixPlain := "           "
			if j == 0 {
				prefix = label
				prefixPlain = labelPlain
			}
			items = append(items, item{
				text:      prefix + line,
				plain:     prefixPlain + line,
				msgIndex:  msgIdx,
				toolIndex: toolIdx,
			})
		}
	}

	return items
}

// renderToolArgs renders the argument section for an expanded tool call.
// For Edit/MultiEdit it shows a unified-diff-style view; otherwise pretty JSON.
func (m *model) renderToolArgs(msgIdx, toolIdx int, tc *parser.ToolCall) []item {
	if tc.Name == "Edit" || tc.Name == "MultiEdit" {
		if items := m.renderEditDiff(msgIdx, toolIdx, tc); len(items) > 0 {
			return items
		}
	}
	if tc.Arguments == "" || tc.Arguments == "{}" {
		return nil
	}
	pretty := prettyJSON(tc.Arguments)
	var items []item
	for _, line := range strings.Split(pretty, "\n") {
		items = append(items, item{
			text:      styleTool.Render("     │ ") + line,
			plain:     "     │ " + line,
			msgIndex:  msgIdx,
			toolIndex: toolIdx,
		})
	}
	return items
}

// renderEditDiff renders Edit/MultiEdit arguments as a unified-diff-style block.
func (m *model) renderEditDiff(msgIdx, toolIdx int, tc *parser.ToolCall) []item {
	pipe := styleDim.Render("     │ ")
	pipePlain := "     │ "

	addLine := func(items []item, rendered, plain string) []item {
		return append(items, item{
			text:      pipe + rendered,
			plain:     pipePlain + plain,
			msgIndex:  msgIdx,
			toolIndex: toolIdx,
		})
	}

	renderHunk := func(items []item, oldStr, newStr string) []item {
		for _, line := range strings.Split(oldStr, "\n") {
			items = addLine(items, styleRemoved.Render("-"+line), "-"+line)
		}
		for _, line := range strings.Split(newStr, "\n") {
			items = addLine(items, styleAdded.Render("+"+line), "+"+line)
		}
		return items
	}

	var items []item

	if tc.Name == "Edit" {
		var args struct {
			FilePath  string `json:"file_path"`
			OldString string `json:"old_string"`
			NewString string `json:"new_string"`
		}
		if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
			return nil
		}
		if args.FilePath != "" {
			items = addLine(items, styleDim.Render("file: "+args.FilePath), "file: "+args.FilePath)
		}
		items = renderHunk(items, args.OldString, args.NewString)
		return items
	}

	// MultiEdit
	var args struct {
		FilePath string `json:"file_path"`
		Edits    []struct {
			OldString string `json:"old_string"`
			NewString string `json:"new_string"`
		} `json:"edits"`
	}
	if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
		return nil
	}
	if args.FilePath != "" {
		items = addLine(items, styleDim.Render("file: "+args.FilePath), "file: "+args.FilePath)
	}
	for i, edit := range args.Edits {
		if len(args.Edits) > 1 {
			hunkHeader := styleDim.Render(fmt.Sprintf("@@ edit %d @@", i+1))
			hunkPlain := fmt.Sprintf("@@ edit %d @@", i+1)
			items = addLine(items, hunkHeader, hunkPlain)
		}
		items = renderHunk(items, edit.OldString, edit.NewString)
	}
	return items
}

// --------------------------------------------------------------------------
// Resume
// --------------------------------------------------------------------------

// doResume launches `claude --resume <session_id>` in the session's CWD,
// suspending the TUI while claude runs and resuming it when claude exits.
func (m model) doResume() (tea.Model, tea.Cmd) {
	sess := m.session()
	if sess == nil {
		m.statusMsg = "no session selected"
		return m, nil
	}
	sessionID := sess.ID
	if sessionID == "" {
		m.statusMsg = "session has no ID"
		return m, nil
	}

	cwd := sessionCWD(sess)
	bin := claudeCmd()

	m.statusMsg = fmt.Sprintf("resuming in %s …", cwd)

	c := exec.Command(bin, "--resume", sessionID)
	if cwd != "" {
		c.Dir = cwd
	}

	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return resumeFinishedMsg{err: err}
	})
}

// sessionCWD returns the working directory recorded in the session's first
// message that has a non-empty CWD field.
func sessionCWD(sess *parser.Session) string {
	for _, msg := range sess.Messages {
		if msg.CWD != "" {
			return msg.CWD
		}
	}
	return ""
}

// claudeCmd returns the claude binary name/path to use.
// Controlled by the CLAUDE_COMMAND environment variable; defaults to "claude".
func claudeCmd() string {
	if v := os.Getenv("CLAUDE_COMMAND"); v != "" {
		return v
	}
	return "claude"
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

// shortPath returns the last two path components for display.
func shortPath(p string) string {
	parts := strings.Split(p, "/")
	if len(parts) <= 2 {
		return p
	}
	return "…/" + strings.Join(parts[len(parts)-2:], "/")
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

// ansiEscape matches ANSI escape sequences for stripping.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI removes ANSI color codes from a string.
func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
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

// --------------------------------------------------------------------------
// Grouped session list helpers
// --------------------------------------------------------------------------

// abbrevPath replaces the home directory prefix with "~".
func abbrevPath(p string) string {
	if p == "" {
		return "(unknown)"
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}

// buildGroupRows rebuilds m.groupRows from m.sessions grouped by CWD.
// Stable insertion order is preserved.
func (m *model) buildGroupRows() {
	type groupEntry struct {
		cwd          string
		sessionIdxs  []int
		newestTs     string
		newestPreview string
	}

	var order []string
	groups := map[string]*groupEntry{}

	for i, s := range m.sessions {
		cwd := abbrevPath(sessionCWD(s))
		if _, ok := groups[cwd]; !ok {
			order = append(order, cwd)
			groups[cwd] = &groupEntry{cwd: cwd}
		}
		g := groups[cwd]
		g.sessionIdxs = append(g.sessionIdxs, i)
	}

	// Sort groups alphabetically for deterministic display.
	sort.Strings(order)

	// Collect newest timestamp + preview for each group.
	for _, cwd := range order {
		g := groups[cwd]
		var newestTs string
		var newestPreview string
		for _, idx := range g.sessionIdxs {
			s := m.sessions[idx]
			for _, msg := range s.Messages {
				if !msg.Timestamp.IsZero() {
					ts := msg.Timestamp.Format("2006-01-02 15:04")
					if ts > newestTs {
						newestTs = ts
					}
				}
				if msg.Role == "user" && msg.Text != "" && newestPreview == "" {
					p := strings.TrimSpace(msg.Text)
					if len([]rune(p)) > 50 {
						p = string([]rune(p)[:50]) + "…"
					}
					newestPreview = p
				}
			}
		}
		g.newestTs = newestTs
		g.newestPreview = newestPreview
	}

	m.groupRows = nil
	for _, cwd := range order {
		g := groups[cwd]
		expanded := m.expandedGroups[cwd]
		m.groupRows = append(m.groupRows, displayRow{
			kind: rowKindGroup, cwd: cwd, sessionIdx: -1, count: len(g.sessionIdxs),
		})
		if expanded {
			for _, idx := range g.sessionIdxs {
				m.groupRows = append(m.groupRows, displayRow{
					kind: rowKindSession, cwd: cwd, sessionIdx: idx,
				})
			}
		}
	}

	// Clamp cursor.
	if m.groupPickCursor >= len(m.groupRows) {
		m.groupPickCursor = len(m.groupRows) - 1
	}
	if m.groupPickCursor < 0 {
		m.groupPickCursor = 0
	}
}

// groupMeta scans all sessions belonging to cwd and returns the newest
// timestamp string and first user message preview.
func (m model) groupMeta(cwd string) (newestTs, newestPreview string) {
	for _, s := range m.sessions {
		if abbrevPath(sessionCWD(s)) != cwd {
			continue
		}
		for _, msg := range s.Messages {
			if !msg.Timestamp.IsZero() {
				ts := msg.Timestamp.Format("2006-01-02 15:04")
				if ts > newestTs {
					newestTs = ts
				}
			}
			if msg.Role == "user" && msg.Text != "" && newestPreview == "" {
				p := strings.TrimSpace(msg.Text)
				if len([]rune(p)) > 50 {
					p = string([]rune(p)[:50]) + "…"
				}
				newestPreview = p
			}
		}
	}
	if newestTs == "" {
		newestTs = "unknown"
	}
	return
}

// renderGroupRow renders a single displayRow into a styled string.
func (m model) renderGroupRow(i int) string {
	row := m.groupRows[i]
	selected := i == m.groupPickCursor

	switch row.kind {
	case rowKindGroup:
		sym := "▶"
		if m.expandedGroups[row.cwd] {
			sym = "▼"
		}
		ts, preview := m.groupMeta(row.cwd)
		label := fmt.Sprintf("%s %s  (%d)  %s  %s", sym, row.cwd, row.count, ts, preview)
		if selected {
			return stylePickSel.Render(label)
		}
		return styleHeader.Render(label)

	case rowKindSession:
		sl := m.sessionLabel(m.sessions[row.sessionIdx])
		label := fmt.Sprintf("    %s", sl)
		if row.sessionIdx == m.sessionIdx {
			label += "  ←current"
		}
		if selected {
			return stylePickSel.Render(label)
		}
		return label
	}
	return ""
}

// groupedPickerView renders the grouped session list overlay.
func (m model) groupedPickerView() string {
	vh := m.viewHeight()
	lines := make([]string, 0, vh)

	title := styleHeader.Render(fmt.Sprintf("Sessions grouped by directory (%d)  Tab=flat · ←/→/Enter=expand · n rename · Esc cancel", len(m.sessions)))
	lines = append(lines, title)
	lines = append(lines, styleBorder.Render(strings.Repeat("─", min(m.width-2, 70))))
	lines = append(lines, "")

	listHeight := vh - 4
	visStart := 0
	if m.groupPickCursor >= listHeight {
		visStart = m.groupPickCursor - listHeight + 1
	}
	if visStart < 0 {
		visStart = 0
	}

	for i := visStart; i < len(m.groupRows) && i < visStart+listHeight; i++ {
		lines = append(lines, m.renderGroupRow(i))
	}

	for len(lines) < vh {
		lines = append(lines, "")
	}

	return strings.Join(lines[:vh], "\n") + "\n" + styleHelp.Render("Tab flat view · Esc cancel")
}

// --------------------------------------------------------------------------
// Misc helpers
// --------------------------------------------------------------------------

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Ensure styleOK and styleArchive are used (they appear in the status bar logic).
var _ = styleOK
