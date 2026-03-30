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
	"github.com/fsnotify/fsnotify"

	"github.com/a2d2-dev/cc-history/internal/config"
	"github.com/a2d2-dev/cc-history/internal/loader"
	"github.com/a2d2-dev/cc-history/internal/parser"
)

// spinnerFrames are the characters cycled for the search progress indicator.
var spinnerFrames = []rune(`⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`)

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
	modeSettings        // settings modal overlay
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
// Lazy session slot
// --------------------------------------------------------------------------

// sessionSlot holds lazily-loaded session data.
// On startup only the metadata is populated; the first-message preview and
// full session content are loaded on demand.
type sessionSlot struct {
	meta          *parser.SessionMeta
	preview       string          // first user message text; "" until loaded
	previewLoaded bool            // true after load attempt (even if preview is empty)
	full          *parser.Session // nil until session is selected for viewing
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
	slots      []*sessionSlot
	sessionIdx int
	items      []item
	expanded   map[int]bool // msgIndex -> expanded (tool call details)
	showTools  bool         // whether tool call lines are visible at all
	cursor     int          // viewport top line index
	height     int          // terminal height
	width      int          // terminal width
	totalLines int

	// search
	mode           tuiMode
	searchQuery    string
	matches        []int // item indices that match the search query
	matchCursor    int   // current position in matches
	searchSearching bool  // true while a background search is running
	searchVersion  int   // incremented on each new query; used to discard stale results
	spinnerFrame   int   // current spinner animation frame

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

	// live file watcher
	watchEnabled  bool         // true = fsnotify watcher is active
	watcherStop   chan struct{} // close to stop the watcher goroutine
	watcherNotify chan struct{} // receives a token on each file-change batch

	// settings modal
	settingsCursor int       // which field is focused (0–3)
	settingsValues [4]string // temp edit buffer: [claudePath, sortOrder, groupedMode, theme]
}

// resumeFinishedMsg is sent back by the ExecProcess callback after claude exits.
type resumeFinishedMsg struct{ err error }

// searchResultMsg carries results from the background search goroutine.
type searchResultMsg struct {
	query   string
	matches []int
	version int // matches model.searchVersion at dispatch time
}

// spinnerTickMsg advances the spinner animation frame.
type spinnerTickMsg struct{ frame int }

// previewLoadedMsg carries an asynchronously loaded first-message preview.
type previewLoadedMsg struct {
	slotIdx int
	preview string
}

// fileChangedMsg is sent when the file watcher detects a change in the
// sessions directory.
type fileChangedMsg struct{}

// currentSlot returns the currently active session slot, or nil if none.
func (m model) currentSlot() *sessionSlot {
	if len(m.slots) == 0 || m.sessionIdx < 0 || m.sessionIdx >= len(m.slots) {
		return nil
	}
	return m.slots[m.sessionIdx]
}

// session returns the full session for the active slot, or nil if not loaded.
func (m model) session() *parser.Session {
	s := m.currentSlot()
	if s == nil {
		return nil
	}
	return s.full
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

// RunTUI starts the TUI with session metadata, loading full content lazily.
// currentIdx is the index of the session to open first.
// sessionsRoot is the live sessions directory (used for archive operations).
// It blocks until the user quits.
func RunTUI(metas []*parser.SessionMeta, currentIdx int, sessionsRoot string) error {
	if len(metas) == 0 {
		return fmt.Errorf("no sessions to display")
	}
	if currentIdx < 0 || currentIdx >= len(metas) {
		currentIdx = 0
	}
	slots := make([]*sessionSlot, len(metas))
	for i, meta := range metas {
		slots[i] = &sessionSlot{meta: meta}
	}
	m := newModelFromSlots(slots, currentIdx, sessionsRoot)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// RunSession starts the TUI for a single session and blocks until the user quits.
func RunSession(session *parser.Session) error {
	slot := slotFromSession(session)
	m := newModelFromSlots([]*sessionSlot{slot}, 0, "")
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// slotFromSession wraps a fully-loaded session in a sessionSlot, extracting
// metadata from message content so no extra file reads are needed.
func slotFromSession(s *parser.Session) *sessionSlot {
	meta := &parser.SessionMeta{
		ID:       s.ID,
		FilePath: s.FilePath,
	}
	for _, msg := range s.Messages {
		if !msg.Timestamp.IsZero() {
			if meta.StartTime.IsZero() {
				meta.StartTime = msg.Timestamp
			}
			meta.EndTime = msg.Timestamp
		}
		if meta.CWD == "" && msg.CWD != "" {
			meta.CWD = msg.CWD
		}
	}
	preview := ""
	for _, msg := range s.Messages {
		if msg.Role == "user" && msg.Text != "" {
			p := strings.TrimSpace(msg.Text)
			runes := []rune(p)
			if len(runes) > 50 {
				preview = string(runes[:50]) + "…"
			} else {
				preview = p
			}
			break
		}
	}
	return &sessionSlot{
		meta:          meta,
		preview:       preview,
		previewLoaded: true,
		full:          s,
	}
}

// newModelFromSlots creates a model from a slice of sessionSlots (internal).
func newModelFromSlots(slots []*sessionSlot, idx int, sessionsRoot string) model {
	cfg := config.Load()
	m := model{
		slots:          slots,
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
	if cfg.WatcherEnabled && sessionsRoot != "" {
		m.watchEnabled = true
		m.watcherStop = make(chan struct{})
		m.watcherNotify = make(chan struct{}, 1)
		go runWatcher(m.currentRoot(), m.watcherNotify, m.watcherStop)
	}
	return m
}

// newModelMulti creates a model from fully-loaded sessions (used by tests).
func newModelMulti(sessions []*parser.Session, idx int, sessionsRoot string) model {
	slots := make([]*sessionSlot, len(sessions))
	for i, s := range sessions {
		slots[i] = slotFromSession(s)
	}
	return newModelFromSlots(slots, idx, sessionsRoot)
}

// newModel creates a model from a single session (used by tests).
func newModel(session *parser.Session) model {
	return newModelMulti([]*parser.Session{session}, 0, "")
}

// --------------------------------------------------------------------------
// bubbletea interface
// --------------------------------------------------------------------------

func (m model) Init() tea.Cmd {
	if m.watchEnabled && m.watcherNotify != nil {
		return waitForWatcherEvent(m.watcherNotify)
	}
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

	case searchResultMsg:
		// Discard stale results from a superseded query.
		if msg.version != m.searchVersion {
			return m, nil
		}
		m.matches = msg.matches
		m.searchSearching = false
		if m.matchCursor >= len(m.matches) {
			m.matchCursor = 0
		}
		if len(m.matches) > 0 {
			m.scrollToMatch()
		}
		return m, nil

	case spinnerTickMsg:
		if !m.searchSearching {
			return m, nil // search finished; stop ticking
		}
		m.spinnerFrame = msg.frame
		return m, spinnerTickCmd(m.spinnerFrame)

	case previewLoadedMsg:
		if msg.slotIdx >= 0 && msg.slotIdx < len(m.slots) {
			m.slots[msg.slotIdx].preview = msg.preview
			m.slots[msg.slotIdx].previewLoaded = true
		}
		return m, nil

	case fileChangedMsg:
		if m.watchEnabled && m.sessionsRoot != "" {
			m = m.doReloadSessions()
		}
		if m.watchEnabled && m.watcherNotify != nil {
			return m, waitForWatcherEvent(m.watcherNotify)
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
		case modeSettings:
			return m.updateSettings(msg)
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
		if len(m.slots) > 1 {
			m.mode = modePicker
			m.groupedPicker = false
			m.pickCursor = m.sessionIdx
			return m, m.launchPreviewLoads()
		}

	case "tab":
		if len(m.slots) > 1 {
			m.mode = modePicker
			m.groupedPicker = true
			m.buildGroupRows()
			return m, m.launchPreviewLoads()
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
		if slot := m.currentSlot(); slot != nil {
			m.repathSessionID = slot.meta.ID
			m.repathInput = slot.meta.CWD
			m.mode = modeRepath
		}

	case "w":
		return m.doToggleWatcher()

	case ",":
		m = m.openSettings()
	}
	return m, nil
}

// openSettings initialises the settings modal state from the current config.
func (m model) openSettings() model {
	cfg := config.Load()
	groupedStr := "off"
	if cfg.GroupedMode {
		groupedStr = "on"
	}
	m.settingsCursor = 0
	m.settingsValues = [4]string{
		cfg.ClaudePath,
		cfg.SortOrder,
		groupedStr,
		cfg.Theme,
	}
	m.mode = modeSettings
	return m
}

// doArchive archives (or restores) the currently selected session.
func (m model) doArchive() model {
	if m.sessionsRoot == "" {
		m.statusMsg = "archive unavailable: no session root"
		return m
	}
	slot := m.currentSlot()
	if slot == nil {
		return m
	}
	filePath := slot.meta.FilePath

	var dstPath string
	var err error
	var action archiveAction

	if m.showArchived {
		// Currently in archive view → restore to live.
		dstPath, err = loader.RestoreSession(m.sessionsRoot, filePath)
		if err != nil {
			m.statusMsg = fmt.Sprintf("restore failed: %v", err)
			return m
		}
		action = archiveAction{
			fromPath:    filePath,
			toPath:      dstPath,
			wasArchived: false,
		}
		m.statusMsg = fmt.Sprintf("restored: %s", shortPath(dstPath))
	} else {
		// Currently in live view → archive.
		dstPath, err = loader.ArchiveSession(m.sessionsRoot, filePath)
		if err != nil {
			m.statusMsg = fmt.Sprintf("archive failed: %v", err)
			return m
		}
		action = archiveAction{
			fromPath:    filePath,
			toPath:      dstPath,
			wasArchived: true,
		}
		m.statusMsg = fmt.Sprintf("archived: %s", shortPath(dstPath))
	}

	m.lastAction = &action
	// Remove the slot from the current list and update the index.
	m.slots = append(m.slots[:m.sessionIdx], m.slots[m.sessionIdx+1:]...)
	if m.sessionIdx >= len(m.slots) && m.sessionIdx > 0 {
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
	newMetas, err := loader.LoadAllSessionsMeta(newRoot, "")
	if err != nil || len(newMetas) == 0 {
		// Toggle back on empty archive; show message.
		m.showArchived = !m.showArchived
		if m.showArchived {
			m.statusMsg = "no archived sessions found"
		} else {
			m.statusMsg = fmt.Sprintf("load error: %v", err)
		}
		return m
	}
	newSlots := make([]*sessionSlot, len(newMetas))
	for i, meta := range newMetas {
		newSlots[i] = &sessionSlot{meta: meta}
	}
	m.slots = newSlots
	m.sessionIdx = len(newSlots) - 1 // most recent
	m.expanded = make(map[int]bool)
	m.cursor = 0
	m.searchQuery = ""
	m.matches = nil
	m.matchCursor = 0
	m.rebuildItems()
	if m.showArchived {
		m.statusMsg = fmt.Sprintf("archive view: %d sessions", len(newSlots))
	} else {
		m.statusMsg = fmt.Sprintf("live view: %d sessions", len(newSlots))
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
	newMetas, err := loader.LoadAllSessionsMeta(m.currentRoot(), "")
	if err != nil {
		m.statusMsg = fmt.Sprintf("undo ok, reload error: %v", err)
		return m
	}
	newSlots := make([]*sessionSlot, len(newMetas))
	for i, meta := range newMetas {
		newSlots[i] = &sessionSlot{meta: meta}
	}
	m.slots = newSlots
	// Try to restore the cursor to the affected session.
	m.sessionIdx = len(newSlots) - 1
	for i, slot := range newSlots {
		if slot.meta.FilePath == restoredPath {
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
	var filePath string
	var origID string
	slot := m.currentSlot()
	if slot != nil && slot.meta.ID == m.duplicatingSessionID {
		filePath = slot.meta.FilePath
		origID = slot.meta.ID
	} else {
		for _, s := range m.slots {
			if s.meta.ID == m.duplicatingSessionID {
				filePath = s.meta.FilePath
				origID = s.meta.ID
				break
			}
		}
	}
	if filePath == "" {
		m.statusMsg = "duplicate failed: session not found"
		m.mode = m.confirmReturnMode
		return m
	}

	_, newID, err := loader.DuplicateSession(filePath)
	if err != nil {
		m.statusMsg = fmt.Sprintf("duplicate failed: %v", err)
		m.mode = m.confirmReturnMode
		return m
	}

	// Reload sessions from disk.
	newMetas, err := loader.LoadAllSessionsMeta(m.currentRoot(), "")
	if err != nil {
		m.statusMsg = fmt.Sprintf("duplicate ok (%s) but reload failed: %v", newID[:8], err)
		m.mode = m.confirmReturnMode
		return m
	}
	newSlots := make([]*sessionSlot, len(newMetas))
	for i, meta := range newMetas {
		newSlots[i] = &sessionSlot{meta: meta}
	}
	m.slots = newSlots
	// Keep current session selected (find by original ID).
	for i, s := range m.slots {
		if s.meta.ID == origID {
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

// doReloadSessions reloads the session list from disk, preserving the current
// session selection by file path when possible.
func (m model) doReloadSessions() model {
	currentPath := ""
	if slot := m.currentSlot(); slot != nil {
		currentPath = slot.meta.FilePath
	}
	newMetas, err := loader.LoadAllSessionsMeta(m.currentRoot(), "")
	if err != nil || len(newMetas) == 0 {
		return m
	}
	newSlots := make([]*sessionSlot, len(newMetas))
	for i, meta := range newMetas {
		newSlots[i] = &sessionSlot{meta: meta}
	}
	m.slots = newSlots
	m.sessionIdx = len(newSlots) - 1
	if currentPath != "" {
		for i, s := range newSlots {
			if s.meta.FilePath == currentPath {
				m.sessionIdx = i
				break
			}
		}
	}
	m.expanded = make(map[int]bool)
	m.cursor = 0
	m.matches = nil
	m.matchCursor = 0
	m.rebuildItems()
	return m
}

// doToggleWatcher toggles the live file watcher on/off.
func (m model) doToggleWatcher() (tea.Model, tea.Cmd) {
	if m.sessionsRoot == "" {
		m.statusMsg = "watcher unavailable: no session root"
		return m, nil
	}
	cfg := config.Load()
	if m.watchEnabled {
		// Stop the watcher goroutine.
		if m.watcherStop != nil {
			close(m.watcherStop)
		}
		m.watchEnabled = false
		m.watcherStop = nil
		m.watcherNotify = nil
		m.statusMsg = "live watcher off"
		cfg.WatcherEnabled = false
		config.Save(cfg) //nolint:errcheck
		return m, nil
	}
	// Start the watcher.
	m.watcherStop = make(chan struct{})
	m.watcherNotify = make(chan struct{}, 1)
	m.watchEnabled = true
	m.statusMsg = "live watcher on"
	cfg.WatcherEnabled = true
	config.Save(cfg) //nolint:errcheck
	go runWatcher(m.currentRoot(), m.watcherNotify, m.watcherStop)
	return m, waitForWatcherEvent(m.watcherNotify)
}

// waitForWatcherEvent returns a tea.Cmd that blocks until the next file-change
// notification arrives on notify, then sends a fileChangedMsg to the program.
func waitForWatcherEvent(notify <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-notify
		return fileChangedMsg{}
	}
}

// runWatcher watches dir with fsnotify, forwarding Create/Write/Remove events
// as a single token on notify. Stops when stop is closed.
func runWatcher(dir string, notify chan<- struct{}, stop <-chan struct{}) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer watcher.Close()
	_ = watcher.Add(dir)
	for {
		select {
		case <-stop:
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Remove) {
				// Non-blocking send: drop the token if one is already queued.
				select {
				case notify <- struct{}{}:
				default:
				}
			}
		case _, ok := <-watcher.Errors:
			if !ok {
				return
			}
		}
	}
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
			return m, m.launchPreviewLoads()
		}

	case "down", "j":
		if m.pickCursor < len(m.slots)-1 {
			m.pickCursor++
			return m, m.launchPreviewLoads()
		}

	case "n":
		if m.pickCursor < len(m.slots) {
			m = m.startRename(m.slots[m.pickCursor].meta.ID, modePicker)
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
		if m.pickCursor < len(m.slots) {
			m = m.startConfirmDuplicate(m.slots[m.pickCursor].meta.ID, modePicker)
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
		return m, m.launchPreviewLoads()

	case "up", "k":
		if m.groupPickCursor > 0 {
			m.groupPickCursor--
			return m, m.launchPreviewLoads()
		}

	case "down", "j":
		if m.groupPickCursor < len(m.groupRows)-1 {
			m.groupPickCursor++
			return m, m.launchPreviewLoads()
		}

	case "n":
		if len(m.groupRows) > 0 {
			row := m.groupRows[m.groupPickCursor]
			if row.kind == rowKindSession && row.sessionIdx < len(m.slots) {
				m = m.startRename(m.slots[row.sessionIdx].meta.ID, modePicker)
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
			if row.kind == rowKindSession && row.sessionIdx < len(m.slots) {
				m = m.startConfirmDuplicate(m.slots[row.sessionIdx].meta.ID, modePicker)
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
		m.searchSearching = false

	case "enter":
		// Confirm search, stay in search mode but allow n/N navigation.
		m.mode = modeNormal

	case "backspace", "ctrl+h":
		if len(m.searchQuery) > 0 {
			runes := []rune(m.searchQuery)
			m.searchQuery = string(runes[:len(runes)-1])
			return m, m.launchSearch()
		}

	default:
		if len(msg.Runes) > 0 {
			m.searchQuery += string(msg.Runes)
			return m, m.launchSearch()
		}
	}
	return m, nil
}

// launchSearch starts an async background search for the current searchQuery.
// It returns a tea.Cmd that will send a searchResultMsg when done.
// If searchQuery is empty the matches are cleared immediately.
func (m *model) launchSearch() tea.Cmd {
	if m.searchQuery == "" {
		m.matches = nil
		m.matchCursor = 0
		m.searchSearching = false
		return nil
	}
	m.searchVersion++
	m.searchSearching = true
	m.spinnerFrame = 0

	// Snapshot the items and version to avoid data races.
	items := m.items
	version := m.searchVersion
	query := m.searchQuery

	searchCmd := func() tea.Msg {
		q := strings.ToLower(query)
		var matches []int
		for i, it := range items {
			if strings.Contains(strings.ToLower(it.plain), q) {
				matches = append(matches, i)
			}
		}
		return searchResultMsg{query: query, matches: matches, version: version}
	}

	return tea.Batch(searchCmd, spinnerTickCmd(0))
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

// --------------------------------------------------------------------------
// Settings constants
// --------------------------------------------------------------------------

const (
	settingsFieldClaudePath  = 0
	settingsFieldSortOrder   = 1
	settingsFieldGroupedMode = 2
	settingsFieldTheme       = 3
	settingsFieldCount       = 4
)

var settingsFieldNames = [settingsFieldCount]string{
	"Claude path",
	"Sort order",
	"Grouped mode",
	"Theme",
}

// updateSettings handles keypresses inside the settings modal.
func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		// Discard changes.
		m.mode = modeNormal

	case "enter":
		// Save and close.
		m = m.saveSettings()
		m.mode = modeNormal

	case "tab", "down", "j":
		m.settingsCursor = (m.settingsCursor + 1) % settingsFieldCount

	case "shift+tab", "up", "k":
		m.settingsCursor = (m.settingsCursor - 1 + settingsFieldCount) % settingsFieldCount

	case "left", "right", " ":
		// Cycle toggle fields.
		switch m.settingsCursor {
		case settingsFieldSortOrder:
			if m.settingsValues[settingsFieldSortOrder] == "asc" {
				m.settingsValues[settingsFieldSortOrder] = "desc"
			} else {
				m.settingsValues[settingsFieldSortOrder] = "asc"
			}
		case settingsFieldGroupedMode:
			if m.settingsValues[settingsFieldGroupedMode] == "on" {
				m.settingsValues[settingsFieldGroupedMode] = "off"
			} else {
				m.settingsValues[settingsFieldGroupedMode] = "on"
			}
		case settingsFieldTheme:
			themes := []string{"default", "dark"}
			cur := m.settingsValues[settingsFieldTheme]
			next := "default"
			for i, t := range themes {
				if t == cur {
					next = themes[(i+1)%len(themes)]
					break
				}
			}
			m.settingsValues[settingsFieldTheme] = next
		}

	case "backspace", "ctrl+h":
		// Delete last rune in text fields.
		switch m.settingsCursor {
		case settingsFieldClaudePath, settingsFieldTheme:
			v := m.settingsValues[m.settingsCursor]
			runes := []rune(v)
			if len(runes) > 0 {
				m.settingsValues[m.settingsCursor] = string(runes[:len(runes)-1])
			}
		}

	default:
		// Append printable chars to text fields.
		if len(msg.Runes) > 0 {
			switch m.settingsCursor {
			case settingsFieldClaudePath, settingsFieldTheme:
				m.settingsValues[m.settingsCursor] += string(msg.Runes)
			}
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

	var slot *sessionSlot
	for _, s := range m.slots {
		if s.meta.ID == m.repathSessionID {
			slot = s
			break
		}
	}
	if slot == nil {
		m.statusMsg = "repath failed: session not found"
		m.repathSessionID = ""
		m.mode = modeNormal
		return m
	}

	oldCWD := slot.meta.CWD
	if err := loader.RepathSession(slot.meta.FilePath, oldCWD, newCWD); err != nil {
		m.statusMsg = fmt.Sprintf("repath failed: %v", err)
		m.repathSessionID = ""
		m.mode = modeNormal
		return m
	}

	// Reload sessions so grouping reflects the new CWD.
	newMetas, err := loader.LoadAllSessionsMeta(m.currentRoot(), "")
	if err != nil {
		m.statusMsg = fmt.Sprintf("repath ok but reload failed: %v", err)
		m.repathSessionID = ""
		m.mode = modeNormal
		return m
	}
	newSlots := make([]*sessionSlot, len(newMetas))
	for i, meta := range newMetas {
		newSlots[i] = &sessionSlot{meta: meta}
	}
	// Re-select the same session by ID.
	newIdx := 0
	for i, s := range newSlots {
		if s.meta.ID == slot.meta.ID {
			newIdx = i
			break
		}
	}
	m.slots = newSlots
	m.sessionIdx = newIdx
	m.repathSessionID = ""
	m.mode = modeNormal
	m.rebuildItems()
	m.clampCursor()
	m.statusMsg = fmt.Sprintf("repathed to: %s", newCWD)
	return m
}

// saveSettings writes the edited values back to config.
func (m model) saveSettings() model {
	cfg := config.Load()
	cfg.ClaudePath = strings.TrimSpace(m.settingsValues[settingsFieldClaudePath])
	cfg.SortOrder = m.settingsValues[settingsFieldSortOrder]
	cfg.GroupedMode = m.settingsValues[settingsFieldGroupedMode] == "on"
	cfg.Theme = strings.TrimSpace(m.settingsValues[settingsFieldTheme])
	config.Save(cfg) //nolint:errcheck
	m.statusMsg = "settings saved"
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

// spinnerTickCmd schedules the next spinner animation frame (80ms interval).
func spinnerTickCmd(frame int) tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(_ time.Time) tea.Msg {
		return spinnerTickMsg{frame: (frame + 1) % len(spinnerFrames)}
	})
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
	case modeSettings:
		return m.settingsModalView()
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
		var matchInfo string
		if m.searchSearching {
			spinner := string(spinnerFrames[m.spinnerFrame%len(spinnerFrames)])
			matchInfo = fmt.Sprintf(" %s searching…", spinner)
		} else if m.searchQuery != "" {
			matchInfo = fmt.Sprintf(" [%d matches]", len(m.matches))
		}
		hint = styleHelp.Render(fmt.Sprintf("search: %s%s  ESC cancel · Enter confirm · n/N jump", m.searchQuery, matchInfo))
	} else if m.searchQuery != "" {
		statusPart := fmt.Sprintf("[%d/%d]", m.matchCursor+1, len(m.matches))
		if m.searchSearching {
			spinner := string(spinnerFrames[m.spinnerFrame%len(spinnerFrames)])
			statusPart = fmt.Sprintf("%s …", spinner)
		}
		hint = styleHelp.Render(fmt.Sprintf("/%s  %s  n next · N prev · / new search", m.searchQuery, statusPart))
	} else {
		viewLabel := ""
		if m.showArchived {
			viewLabel = styleArchive.Render("[ARCHIVE]") + " "
		}
		if m.watchEnabled {
			viewLabel += styleOK.Render("[LIVE]") + " "
		}
		sessionInfo := ""
		if len(m.slots) > 1 {
			sessionInfo = fmt.Sprintf(" s list · Tab grouped(%d)", len(m.slots))
		}
		toolsHint := "t hide tools"
		if !m.showTools {
			toolsHint = "t show tools"
		}
		hint = viewLabel + styleHelp.Render(fmt.Sprintf("↑↓/jk scroll · %s · T expand · r resume · n rename · d dup · p repath · a archive · A toggle · u undo · w watch · / search · i info · , settings%s · ? help · q quit", toolsHint, sessionInfo))
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
	slots := m.slots
	vh := m.viewHeight()

	// Build picker lines.
	lines := make([]string, 0, len(slots)+4)
	title := styleHeader.Render(fmt.Sprintf("Sessions (%d)  ↑↓/jk navigate · n rename · Enter switch · Esc cancel", len(slots)))
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

	for i := visStart; i < len(slots) && i < visStart+(vh-4); i++ {
		sl := m.slotLabel(slots[i])
		label := fmt.Sprintf("  %2d  %s", i+1, sl)
		if i == m.sessionIdx {
			label = fmt.Sprintf("  %2d  %s  ←current", i+1, sl)
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

// slotLabel returns a display string for the session picker using metadata
// and the lazily-loaded preview. Shows "…" while the preview is loading.
// Custom names from m.names take priority over the preview.
func (m model) slotLabel(slot *sessionSlot) string {
	id := slot.meta.ID
	if len(id) > 16 {
		id = id[:16]
	}
	ts := ""
	if !slot.meta.StartTime.IsZero() {
		ts = slot.meta.StartTime.Format("2006-01-02 15:04")
	}
	if ts == "" {
		ts = "unknown"
	}
	// Use custom name when available; fall back to lazy preview.
	if custom, ok := m.names[slot.meta.ID]; ok && custom != "" {
		name := custom
		if len([]rune(name)) > 50 {
			name = string([]rune(name)[:50]) + "…"
		}
		return fmt.Sprintf("[%s]  %s", ts, name)
	}
	preview := slot.preview
	if !slot.previewLoaded {
		preview = "…" // loading placeholder
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
		{",", "open settings modal"},
		{"r", "resume session (claude --resume)"},
		{"n", "rename session (custom title)"},
		{"d", "duplicate session (copy with new UUID)"},
		{"p", "repath session CWD (change working directory metadata)"},
		{"a", "archive session (restore in archive view)"},
		{"A", "toggle live / archive view"},
		{"u", "undo last archive / restore"},
		{"w", "toggle live file watcher (auto-refresh)"},
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
	if len(m.slots) > 1 {
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

// settingsModalView renders the settings form as a centered overlay.
func (m model) settingsModalView() string {
	labels := settingsFieldNames
	values := m.settingsValues

	// Build modal lines.
	title := styleHeader.Render("Settings")
	sep := styleBorder.Render(strings.Repeat("─", 46))

	var bodyLines []string
	for i := 0; i < settingsFieldCount; i++ {
		focused := i == m.settingsCursor
		label := fmt.Sprintf("%-14s", labels[i])
		val := values[i]

		var fieldStr string
		switch i {
		case settingsFieldSortOrder, settingsFieldGroupedMode, settingsFieldTheme:
			// Cycle field — show value in brackets.
			if focused {
				fieldStr = stylePickSel.Render(fmt.Sprintf(" ◀ %s ▶ ", val))
			} else {
				fieldStr = styleDim.Render(fmt.Sprintf("   %s   ", val))
			}
		default:
			// Text field — show value with cursor indicator.
			display := val
			if display == "" {
				display = "(empty)"
			}
			if focused {
				fieldStr = styleUser.Render("[" + display + "_]")
			} else {
				fieldStr = "[" + display + "]"
			}
		}

		prefix := "  "
		if focused {
			prefix = styleOK.Render("▶ ")
		}
		bodyLines = append(bodyLines, fmt.Sprintf("%s%-14s  %s", prefix, label, fieldStr))
	}

	modalLines := []string{
		"",
		"  " + title,
		"  " + sep,
		"",
	}
	modalLines = append(modalLines, bodyLines...)
	modalLines = append(modalLines, "")
	modalLines = append(modalLines, styleHelp.Render("  Tab/↑↓ navigate · ←/→/Space cycle · type to edit · Enter save · Esc cancel"))
	modalLines = append(modalLines, "")

	// Overlay on main view.
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

// --------------------------------------------------------------------------
// Item builder
// --------------------------------------------------------------------------

func (m *model) rebuildItems() {
	m.items = nil
	slot := m.currentSlot()
	if slot == nil {
		m.totalLines = 0
		return
	}
	// Load full session synchronously if the active slot has not been loaded yet.
	if slot.full == nil && slot.meta.FilePath != "" {
		sess, err := parser.ParseFile(slot.meta.FilePath)
		if err == nil {
			slot.full = sess
			if slot.meta.ID == "" {
				slot.meta.ID = sess.ID
			}
		}
	}
	sess := slot.full
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
	slot := m.currentSlot()
	if slot == nil {
		m.statusMsg = "no session selected"
		return m, nil
	}
	sessionID := slot.meta.ID
	if sessionID == "" {
		m.statusMsg = "session has no ID"
		return m, nil
	}

	cwd := slot.meta.CWD
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

// launchPreviewLoads returns a tea.Cmd that asynchronously loads first-message
// previews for session slots visible in the current picker viewport.
// Slots already loaded or with no file path are skipped.
func (m model) launchPreviewLoads() tea.Cmd {
	vh := m.viewHeight() - 4 // approximate visible picker rows
	if vh <= 0 {
		vh = 20
	}
	start := m.pickCursor - vh/2
	if start < 0 {
		start = 0
	}
	end := start + vh
	if end > len(m.slots) {
		end = len(m.slots)
	}
	var cmds []tea.Cmd
	for i := start; i < end; i++ {
		slot := m.slots[i]
		if slot.previewLoaded || slot.meta.FilePath == "" {
			continue
		}
		// Mark as loading immediately (slots are pointers; this persists).
		slot.previewLoaded = true
		idx := i
		filePath := slot.meta.FilePath
		cmds = append(cmds, func() tea.Msg {
			preview := parser.ParseFirstMsgPreview(filePath)
			return previewLoadedMsg{slotIdx: idx, preview: preview}
		})
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// claudeCmd returns the claude binary name/path to use.
// Controlled by the CLAUDE_COMMAND environment variable, then by config ClaudePath;
// defaults to "claude".
func claudeCmd() string {
	if v := os.Getenv("CLAUDE_COMMAND"); v != "" {
		return v
	}
	if p := config.Load().ClaudePath; p != "" {
		return p
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

// buildGroupRows rebuilds m.groupRows from m.slots grouped by CWD.
// Stable insertion order is preserved.
func (m *model) buildGroupRows() {
	type groupEntry struct {
		cwd           string
		sessionIdxs   []int
		newestTs      string
		newestPreview string
	}

	var order []string
	groups := map[string]*groupEntry{}

	for i, slot := range m.slots {
		cwd := abbrevPath(slot.meta.CWD)
		if _, ok := groups[cwd]; !ok {
			order = append(order, cwd)
			groups[cwd] = &groupEntry{cwd: cwd}
		}
		g := groups[cwd]
		g.sessionIdxs = append(g.sessionIdxs, i)
	}

	// Sort groups alphabetically for deterministic display.
	sort.Strings(order)

	// Collect newest timestamp + preview for each group from metadata.
	for _, cwd := range order {
		g := groups[cwd]
		var newestTs string
		var newestPreview string
		for _, idx := range g.sessionIdxs {
			slot := m.slots[idx]
			if !slot.meta.EndTime.IsZero() {
				ts := slot.meta.EndTime.Format("2006-01-02 15:04")
				if ts > newestTs {
					newestTs = ts
				}
			}
			if slot.previewLoaded && slot.preview != "" && newestPreview == "" {
				newestPreview = slot.preview
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

// groupMeta returns the newest timestamp string and first loaded preview
// for all slots belonging to cwd.
func (m model) groupMeta(cwd string) (newestTs, newestPreview string) {
	for _, slot := range m.slots {
		if abbrevPath(slot.meta.CWD) != cwd {
			continue
		}
		if !slot.meta.EndTime.IsZero() {
			ts := slot.meta.EndTime.Format("2006-01-02 15:04")
			if ts > newestTs {
				newestTs = ts
			}
		}
		if slot.previewLoaded && slot.preview != "" && newestPreview == "" {
			newestPreview = slot.preview
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
		sl := m.slotLabel(m.slots[row.sessionIdx])
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

	title := styleHeader.Render(fmt.Sprintf("Sessions grouped by directory (%d)  Tab=flat · ←/→/Enter=expand · n rename · Esc cancel", len(m.slots)))
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
