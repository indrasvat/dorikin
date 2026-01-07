package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.help.Width = msg.Width
		return m, nil
	}

	return m, nil
}

// handleKeyMsg processes keyboard input.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle quit globally
	if key.Matches(msg, m.keys.Quit) {
		m.quitting = true
		return m, tea.Quit
	}

	// Handle based on view mode
	switch m.viewMode {
	case ViewHelp:
		return m.handleHelpKeys(msg)
	case ViewDetail:
		return m.handleDetailKeys(msg)
	default:
		return m.handleListKeys(msg)
	}
}

// handleListKeys handles keys in list view.
func (m Model) handleListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}

	case key.Matches(msg, m.keys.Down):
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}

	case key.Matches(msg, m.keys.Enter):
		if m.selectedReport() != nil {
			m.viewMode = ViewDetail
		}

	case key.Matches(msg, m.keys.Filter):
		m.cycleFilter()

	case key.Matches(msg, m.keys.ToggleOK):
		m.showAll = !m.showAll
		m.applyFilter()

	case key.Matches(msg, m.keys.Help):
		m.viewMode = ViewHelp
	}

	return m, nil
}

// handleDetailKeys handles keys in detail view.
func (m Model) handleDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape):
		m.viewMode = ViewList

	case key.Matches(msg, m.keys.Up):
		// Navigate to previous resource
		if m.cursor > 0 {
			m.cursor--
		}

	case key.Matches(msg, m.keys.Down):
		// Navigate to next resource
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
		}

	case key.Matches(msg, m.keys.Help):
		m.viewMode = ViewHelp
	}

	return m, nil
}

// handleHelpKeys handles keys in help view.
func (m Model) handleHelpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Escape), key.Matches(msg, m.keys.Help):
		m.viewMode = ViewList
	}

	return m, nil
}
