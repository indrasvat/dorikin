package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/indrasvat/dorikin/internal/logcapture"
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

	case RefreshMsg:
		m.refreshing = false
		if msg.Err != nil {
			logcapture.Error("Refresh failed: %v", msg.Err)
		} else if msg.Result != nil {
			m.updateFromResult(msg.Result)
			// Log summary of results
			s := msg.Result.Summary
			if s.HasIssues() {
				logcapture.Warn("Scan complete: %d drifted, %d missing, %d extra, %d errors",
					s.Drifted, s.Missing, s.Extra, s.Errors)
			} else {
				logcapture.Info("Scan complete: %d resources in sync", s.InSync)
			}
		}
		// Continue auto-refresh cycle if enabled
		if m.autoRefresh {
			cmd := m.tickCmd()
			return m, cmd
		}
		return m, nil

	case TickMsg:
		// Auto-refresh tick - trigger a scan if not already refreshing
		if m.autoRefresh && !m.refreshing && m.scanFunc != nil {
			logcapture.Debug("Auto-refresh triggered")
			m.refreshing = true
			cmd := m.doRefresh()
			return m, cmd
		}
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
	case ViewLogs:
		return m.handleLogsKeys(msg)
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

	case key.Matches(msg, m.keys.Refresh):
		return m.handleRefresh()

	case key.Matches(msg, m.keys.AutoRefresh):
		return m.handleAutoRefresh()

	case key.Matches(msg, m.keys.Help):
		m.viewMode = ViewHelp

	case key.Matches(msg, m.keys.Logs):
		m.viewMode = ViewLogs
		m.logsCursor = 0
	}

	return m, nil
}

// handleRefresh handles manual refresh trigger.
func (m Model) handleRefresh() (tea.Model, tea.Cmd) {
	if !m.refreshing && m.scanFunc != nil {
		logcapture.Info("Manual refresh triggered")
		m.refreshing = true
		cmd := m.doRefresh()
		return m, cmd
	}
	return m, nil
}

// handleAutoRefresh handles auto-refresh toggle.
func (m Model) handleAutoRefresh() (tea.Model, tea.Cmd) {
	m.autoRefresh = !m.autoRefresh
	if m.autoRefresh {
		cmd := m.tickCmd()
		return m, cmd
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

// handleLogsKeys handles keys in logs view.
func (m Model) handleLogsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.logCapture == nil {
		// No log capture available, go back to list
		if key.Matches(msg, m.keys.Escape) {
			m.viewMode = ViewList
		}
		return m, nil
	}

	entries := m.logCapture.FilteredEntries(m.logsFilter)

	switch {
	case key.Matches(msg, m.keys.Escape):
		m.viewMode = ViewList

	case key.Matches(msg, m.keys.Up):
		if m.logsCursor > 0 {
			m.logsCursor--
		}

	case key.Matches(msg, m.keys.Down):
		if m.logsCursor < len(entries)-1 {
			m.logsCursor++
		}

	case key.Matches(msg, m.keys.Filter):
		// Cycle through log level filters
		m.cycleLogsFilter()
		// Reset cursor if it's now out of bounds
		newEntries := m.logCapture.FilteredEntries(m.logsFilter)
		if m.logsCursor >= len(newEntries) {
			m.logsCursor = max(0, len(newEntries)-1)
		}

	case key.Matches(msg, m.keys.ClearLogs):
		m.logCapture.Clear()
		m.logsCursor = 0
	}

	return m, nil
}

// cycleLogsFilter cycles through log level filters.
func (m *Model) cycleLogsFilter() {
	levels := []logcapture.Level{
		logcapture.LevelDebug, // All
		logcapture.LevelInfo,  // Info+
		logcapture.LevelWarn,  // Warn+
		logcapture.LevelError, // Error only
	}

	current := 0
	for i, l := range levels {
		if l == m.logsFilter {
			current = i
			break
		}
	}

	m.logsFilter = levels[(current+1)%len(levels)]
}
