package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/indrasvat/dorikin/internal/tui/styles"
	"github.com/indrasvat/dorikin/pkg/api"
)

// ViewMode represents the current view mode.
type ViewMode int

const (
	ViewList ViewMode = iota
	ViewDetail
	ViewHelp
)

// Model is the main TUI model.
type Model struct {
	// Data
	result   *api.ScanResult
	reports  []api.DriftReport
	filtered []int // indices into reports

	// UI state
	cursor   int
	viewMode ViewMode
	filter   api.DriftStatus // empty means show all
	showAll  bool            // show IN_SYNC too
	width    int
	height   int
	ready    bool
	quitting bool

	// Components
	help   help.Model
	keys   keyMap
	styles *styles.App
}

// keyMap defines keyboard shortcuts.
type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Enter    key.Binding
	Escape   key.Binding
	Tab      key.Binding
	Help     key.Binding
	Quit     key.Binding
	Filter   key.Binding
	Refresh  key.Binding
	ToggleOK key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Enter, k.Filter, k.Help, k.Quit}
}

// FullHelp returns keybindings for the expanded help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter, k.Escape},
		{k.Filter, k.ToggleOK, k.Refresh},
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
		Enter: key.NewBinding(
			key.WithKeys("enter", "l"),
			key.WithHelp("↵/l", "details"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc", "h"),
			key.WithHelp("esc/h", "back"),
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
		ToggleOK: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "toggle ok"),
		),
	}
}

// NewModel creates a new TUI model.
func NewModel(result *api.ScanResult) Model {
	m := Model{
		result:   result,
		reports:  result.Reports,
		viewMode: ViewList,
		showAll:  false,
		help:     help.New(),
		keys:     defaultKeyMap(),
		styles:   styles.New(),
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
