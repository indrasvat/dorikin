package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/indrasvat/dorikin/internal/logcapture"
	"github.com/indrasvat/dorikin/internal/tui/styles"
	"github.com/indrasvat/dorikin/pkg/api"
)

// DefaultAutoRefreshInterval is the default interval for auto-refresh.
const DefaultAutoRefreshInterval = 5 * time.Second

// ViewMode represents the current view mode.
type ViewMode int

const (
	ViewList ViewMode = iota
	ViewDetail
	ViewHelp
	ViewLogs
)

// DetailTab represents the active tab in detail view.
type DetailTab int

const (
	TabDiffs DetailTab = iota
	TabManifest
	TabCluster
	TabMeta
)

// Model is the main TUI model.
type Model struct {
	// Data
	result      *api.ScanResult
	reports     []api.DriftReport
	filtered    []int // indices into reports
	kubeContext string

	// Refresh callback
	scanFunc        ScanFunc
	refreshing      bool
	autoRefresh     bool
	refreshInterval time.Duration

	// UI state
	cursor   int
	viewMode ViewMode
	filter   api.DriftStatus // empty means show all
	showAll  bool            // show IN_SYNC too
	width    int
	height   int
	ready    bool
	quitting bool

	// Detail view state
	detailTab    DetailTab // Current tab in detail view
	detailScroll int       // Scroll position within detail content
	diffCursor   int       // Selected diff index

	// Log capture
	logCapture *logcapture.Capture
	logsCursor int
	logsFilter logcapture.Level

	// Components
	help   help.Model
	keys   keyMap
	styles *styles.App
}

// RefreshMsg is sent when a refresh completes.
type RefreshMsg struct {
	Result *api.ScanResult
	Err    error
}

// TickMsg is sent on each auto-refresh interval.
type TickMsg time.Time

// keyMap defines keyboard shortcuts.
type keyMap struct {
	Up          key.Binding
	Down        key.Binding
	Left        key.Binding
	Right       key.Binding
	Enter       key.Binding
	Escape      key.Binding
	Tab         key.Binding
	Help        key.Binding
	Quit        key.Binding
	Filter      key.Binding
	Refresh     key.Binding
	AutoRefresh key.Binding
	ToggleOK    key.Binding
	Logs        key.Binding
	ClearLogs   key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Filter, k.Help, k.Quit}
}

// FullHelp returns keybindings for the expanded help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter, k.Escape},
		{k.Filter, k.ToggleOK, k.Refresh, k.AutoRefresh},
		{k.Help, k.Quit},
	}
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "prev"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "next"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("↵", "details"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next tab"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		AutoRefresh: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "auto-refresh"),
		),
		ToggleOK: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "toggle ok"),
		),
		Logs: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "logs"),
		),
		ClearLogs: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "clear logs"),
		),
	}
}

// NewModel creates a new TUI model.
func NewModel(result *api.ScanResult, scanFunc ScanFunc) Model {
	return NewModelWithInterval(result, scanFunc, DefaultAutoRefreshInterval)
}

// NewModelWithInterval creates a new TUI model with a custom refresh interval.
func NewModelWithInterval(result *api.ScanResult, scanFunc ScanFunc, interval time.Duration) Model {
	m := Model{
		result:          result,
		reports:         result.Reports,
		scanFunc:        scanFunc,
		refreshInterval: interval,
		viewMode:        ViewList,
		showAll:         false,
		help:            help.New(),
		keys:            defaultKeyMap(),
		styles:          styles.New(),
	}

	m.applyFilter()
	return m
}

// Init initializes the model.
func (m Model) Init() tea.Cmd {
	return nil
}

// applyFilter updates the filtered indices based on current filter settings.
func (m *Model) applyFilter() {
	m.filtered = m.filtered[:0]

	for i := range m.reports {
		status := m.reports[i].Status

		// Skip IN_SYNC unless showAll is true
		if !m.showAll && status == api.StatusInSync {
			continue
		}

		// Apply status filter if set
		if m.filter != "" && status != m.filter {
			continue
		}

		m.filtered = append(m.filtered, i)
	}

	// Reset cursor if out of bounds
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

// selectedReport returns the currently selected report, or nil.
func (m *Model) selectedReport() *api.DriftReport {
	if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
		return nil
	}
	return &m.reports[m.filtered[m.cursor]]
}

// cycleFilter cycles through status filters.
func (m *Model) cycleFilter() {
	filters := []api.DriftStatus{
		"",                // All
		api.StatusDrifted, // Drifted only
		api.StatusMissing, // Missing only
		api.StatusExtra,   // Extra only
		api.StatusError,   // Error only
	}

	current := 0
	for i, f := range filters {
		if f == m.filter {
			current = i
			break
		}
	}

	m.filter = filters[(current+1)%len(filters)]
	m.applyFilter()
}

// doRefresh performs an async refresh scan.
func (m *Model) doRefresh() tea.Cmd {
	if m.scanFunc == nil {
		return nil
	}
	return func() tea.Msg {
		result, err := m.scanFunc()
		return RefreshMsg{Result: result, Err: err}
	}
}

// tickCmd returns a command that sends a TickMsg after the refresh interval.
func (m *Model) tickCmd() tea.Cmd {
	return tea.Tick(m.refreshInterval, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// updateFromResult updates the model with new scan results.
func (m *Model) updateFromResult(result *api.ScanResult) {
	m.result = result
	m.reports = result.Reports
	m.applyFilter()
}

// NewModelWithCapture creates a new TUI model with log capture support.
func NewModelWithCapture(result *api.ScanResult, scanFunc ScanFunc, interval time.Duration, capture *logcapture.Capture, kubeContext string) Model {
	m := NewModelWithInterval(result, scanFunc, interval)
	m.logCapture = capture
	m.logsFilter = logcapture.LevelDebug // Show all levels by default
	m.kubeContext = kubeContext
	return m
}
