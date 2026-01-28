package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/indrasvat/dorikin/internal/logcapture"
	"github.com/indrasvat/dorikin/pkg/api"
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
		return m.handleRefreshMsg(msg)

	case TickMsg:
		// Auto-refresh tick - trigger a scan if not already refreshing
		if m.autoRefresh && !m.refreshing && m.scanFunc != nil {
			logcapture.Debug("Auto-refresh triggered")
			m.refreshing = true
			cmd := m.doRefresh()
			return m, cmd
		}
		return m, nil

	case SpinnerTickMsg:
		// Spinner animation tick - continue while loading
		if m.refreshing && len(m.reports) == 0 {
			m.spinnerFrame = (m.spinnerFrame + 1) % 10
			return m, spinnerTickCmd()
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

// handleRefreshMsg handles refresh completion messages.
func (m Model) handleRefreshMsg(msg RefreshMsg) (tea.Model, tea.Cmd) {
	m.refreshing = false
	if msg.Err != nil {
		logcapture.Error("Refresh failed: %v", msg.Err)
		m.scanError = msg.Err // Store error for display in UI
	} else if msg.Result != nil {
		m.scanError = nil // Clear any previous error on success
		m.updateFromResult(msg.Result)
		m.logScanSummary(msg.Result.Summary)
	}
	// Continue auto-refresh cycle if enabled
	if m.autoRefresh {
		cmd := m.tickCmd()
		return m, cmd
	}
	return m, nil
}

// logScanSummary logs the scan summary at the appropriate level.
func (m Model) logScanSummary(s api.ScanSummary) {
	switch {
	case s.Errors > 0:
		logcapture.Error("Scan complete: %d drifted, %d missing, %d extra, %d errors",
			s.Drifted, s.Missing, s.Extra, s.Errors)
	case s.HasIssues():
		logcapture.Warn("Scan complete: %d drifted, %d missing, %d extra",
			s.Drifted, s.Missing, s.Extra)
	default:
		logcapture.Info("Scan complete: %d resources in sync", s.InSync)
	}
}

// handleDetailKeys handles keys in detail view.
func (m Model) handleDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	report := m.selectedReport()

	switch {
	case key.Matches(msg, m.keys.Escape):
		m.viewMode = ViewList
		m.detailScroll = 0
		m.diffCursor = 0

	case key.Matches(msg, m.keys.Up):
		m.handleDetailUp(report)

	case key.Matches(msg, m.keys.Down):
		m.handleDetailDown(report)

	case key.Matches(msg, m.keys.Left):
		// Navigate to previous resource (lap)
		if m.cursor > 0 {
			m.cursor--
			m.resetDetailState()
		}

	case key.Matches(msg, m.keys.Right):
		// Navigate to next resource (lap)
		if m.cursor < len(m.filtered)-1 {
			m.cursor++
			m.resetDetailState()
		}

	case key.Matches(msg, m.keys.Tab):
		// Cycle through tabs
		m.detailTab = (m.detailTab + 1) % 4

	case key.Matches(msg, m.keys.Help):
		m.viewMode = ViewHelp

	default:
		// Handle number keys for direct tab selection
		m.handleDetailTabKeys(msg)
	}

	return m, nil
}

// handleDetailTabKeys handles number keys 1-4 for direct tab selection.
func (m *Model) handleDetailTabKeys(msg tea.KeyMsg) {
	switch msg.String() {
	case "1":
		m.detailTab = TabDiffs
	case "2":
		m.detailTab = TabManifest
	case "3":
		m.detailTab = TabCluster
	case "4":
		m.detailTab = TabMeta
	}
}

// handleDetailUp handles up navigation in detail view.
func (m *Model) handleDetailUp(report *api.DriftReport) {
	// For Manifest/Cluster tabs, scroll the YAML view
	if m.detailTab == TabManifest || m.detailTab == TabCluster {
		if m.detailScroll > 0 {
			m.detailScroll--
		}
		return
	}
	// Scroll up within diffs
	if m.diffCursor > 0 {
		m.diffCursor--
		m.adjustDetailScroll()
	}
}

// handleDetailDown handles down navigation in detail view.
func (m *Model) handleDetailDown(report *api.DriftReport) {
	// For Manifest/Cluster tabs, scroll the YAML view
	if m.detailTab == TabManifest || m.detailTab == TabCluster {
		m.detailScroll++
		return
	}
	// Scroll down within diffs
	if report != nil && m.diffCursor < len(report.Diffs)-1 {
		m.diffCursor++
		m.adjustDetailScroll()
	}
}

// resetDetailState resets detail view state when switching resources.
func (m *Model) resetDetailState() {
	m.detailScroll = 0
	m.diffCursor = 0
	m.detailTab = TabDiffs
}

// adjustDetailScroll ensures the selected diff is visible.
func (m *Model) adjustDetailScroll() {
	// Calculate how many diffs fit on screen (rough estimate).
	// Each diff box is about 6 lines (path + box with 3 lines + blank).
	// The header, tabs, and footer use approximately 14 lines.
	visibleDiffs := max(1, (m.height-14)/6)

	// Scroll up if cursor is above viewport
	if m.diffCursor < m.detailScroll {
		m.detailScroll = m.diffCursor
	}

	// Scroll down if cursor is below viewport
	if m.diffCursor >= m.detailScroll+visibleDiffs {
		m.detailScroll = m.diffCursor - visibleDiffs + 1
	}
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
