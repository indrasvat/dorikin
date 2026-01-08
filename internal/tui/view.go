package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/indrasvat/dorikin/internal/logcapture"
	"github.com/indrasvat/dorikin/internal/tui/styles"
	"github.com/indrasvat/dorikin/pkg/api"
)

// View renders the model.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	if !m.ready {
		return "Loading..."
	}

	var content string
	switch m.viewMode {
	case ViewHelp:
		content = m.renderHelp()
	case ViewDetail:
		content = m.renderDetail()
	case ViewLogs:
		content = m.renderLogs()
	default:
		content = m.renderList()
	}

	return m.styles.Container.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			m.renderHeader(),
			content,
			m.renderFooter(),
		),
	)
}

// renderHeader renders the header bar.
func (m Model) renderHeader() string {
	title := m.styles.Title.Render(" 🏎️  dorikin ")

	summary := fmt.Sprintf(
		"%s %d  %s %d  %s %d  %s %d  %s %d",
		styles.StatusIcon(api.StatusInSync), m.result.Summary.InSync,
		styles.StatusIcon(api.StatusDrifted), m.result.Summary.Drifted,
		styles.StatusIcon(api.StatusMissing), m.result.Summary.Missing,
		styles.StatusIcon(api.StatusExtra), m.result.Summary.Extra,
		styles.StatusIcon(api.StatusError), m.result.Summary.Errors,
	)

	// Auto-refresh indicator
	var autoIndicator string
	if m.autoRefresh {
		autoIndicator = m.styles.InSync.Render(" ⟳ AUTO ")
	}

	// Status indicator
	var status string
	if m.result.Summary.HasIssues() {
		status = m.styles.Drifted.Render("⚡ DRIFT DETECTED")
	} else {
		status = m.styles.InSync.Render("✓ ALL SYNCED")
	}

	gap := m.width - lipgloss.Width(title) - lipgloss.Width(summary) - lipgloss.Width(autoIndicator) - lipgloss.Width(status) - 10
	if gap < 0 {
		gap = 0
	}

	return m.styles.Header.Width(m.width - 4).Render(
		lipgloss.JoinHorizontal(
			lipgloss.Center,
			title,
			strings.Repeat(" ", gap/2),
			summary,
			strings.Repeat(" ", gap/2),
			autoIndicator,
			status,
		),
	)
}

// renderFooter renders the footer with help hints.
func (m Model) renderFooter() string {
	var hints []string

	// Filter indicator
	filterText := "all"
	if m.filter != "" {
		filterText = string(m.filter)
	}
	if !m.showAll {
		filterText += " (hiding ok)"
	}

	hints = append(hints, m.styles.HelpDesc.Render("filter: ")+m.styles.HelpKey.Render(filterText))

	// Navigation hints based on view
	switch m.viewMode {
	case ViewList:
		hints = append(hints,
			m.styles.HelpKey.Render("↑↓")+m.styles.HelpDesc.Render(" nav"),
			m.styles.HelpKey.Render("↵")+m.styles.HelpDesc.Render(" detail"),
			m.styles.HelpKey.Render("f")+m.styles.HelpDesc.Render(" filter"),
			m.styles.HelpKey.Render("o")+m.styles.HelpDesc.Render(" toggle ok"),
			m.styles.HelpKey.Render("r")+m.styles.HelpDesc.Render(" refresh"),
			m.styles.HelpKey.Render("a")+m.styles.HelpDesc.Render(" auto"),
			m.styles.HelpKey.Render("L")+m.styles.HelpDesc.Render(" logs"),
			m.styles.HelpKey.Render("q")+m.styles.HelpDesc.Render(" quit"),
		)
	case ViewDetail:
		hints = append(hints,
			m.styles.HelpKey.Render("↑↓")+m.styles.HelpDesc.Render(" prev/next"),
			m.styles.HelpKey.Render("esc")+m.styles.HelpDesc.Render(" back"),
			m.styles.HelpKey.Render("q")+m.styles.HelpDesc.Render(" quit"),
		)
	case ViewHelp:
		hints = append(hints,
			m.styles.HelpKey.Render("esc")+m.styles.HelpDesc.Render(" back"),
		)
	case ViewLogs:
		hints = append(hints,
			m.styles.HelpKey.Render("↑↓")+m.styles.HelpDesc.Render(" scroll"),
			m.styles.HelpKey.Render("f")+m.styles.HelpDesc.Render(" filter level"),
			m.styles.HelpKey.Render("c")+m.styles.HelpDesc.Render(" clear"),
			m.styles.HelpKey.Render("esc")+m.styles.HelpDesc.Render(" back"),
		)
	}

	sep := m.styles.HelpSep.Render(" │ ")
	return m.styles.Footer.Width(m.width - 4).Render(strings.Join(hints, sep))
}

// renderList renders the resource list view.
func (m Model) renderList() string {
	if len(m.filtered) == 0 {
		return m.styles.Subtitle.Render("\n  No resources to display.\n  Press 'o' to show IN_SYNC resources.\n")
	}

	var rows []string

	// Calculate visible range
	visibleHeight := m.height - 10 // Account for header, footer, padding
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	startIdx := 0
	if m.cursor >= visibleHeight {
		startIdx = m.cursor - visibleHeight + 1
	}

	endIdx := startIdx + visibleHeight
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}

	for i := startIdx; i < endIdx; i++ {
		idx := m.filtered[i]
		report := m.reports[idx]

		// Build row
		icon := styles.StatusIcon(report.Status)
		statusStyle := m.styles.StatusStyle(report.Status)
		statusText := statusStyle.Render(fmt.Sprintf("%-8s", styles.StatusText(report.Status)))

		resourceName := fmt.Sprintf("%s/%s",
			report.Resource.Kind,
			report.Resource.Name,
		)
		if report.Resource.Namespace != "" {
			resourceName = fmt.Sprintf("%s (%s)",
				resourceName,
				report.Resource.Namespace,
			)
		}

		// Build row with selection indicator
		var prefix string
		if i == m.cursor {
			prefix = m.styles.SelectionCursor.Render("▶ ")
		} else {
			prefix = "  "
		}
		row := fmt.Sprintf("%s%s %s  %s", prefix, icon, statusText, resourceName)

		// Add diff count for drifted resources
		if report.Status == api.StatusDrifted && len(report.Diffs) > 0 {
			row += m.styles.Subtle.Render(fmt.Sprintf("  [%d diffs]", len(report.Diffs)))
		}

		// Highlight selected row
		if i == m.cursor {
			row = m.styles.TableSelected.Render(row)
		} else {
			row = m.styles.TableRow.Render(row)
		}

		rows = append(rows, row)
	}

	// Scroll indicator
	scrollInfo := ""
	if len(m.filtered) > visibleHeight {
		scrollInfo = m.styles.Subtitle.Render(fmt.Sprintf(
			"\n  Showing %d-%d of %d resources",
			startIdx+1, endIdx, len(m.filtered),
		))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		strings.Join(rows, "\n"),
		scrollInfo,
	)
}

// renderDetail renders the detail view for a single resource.
func (m Model) renderDetail() string {
	report := m.selectedReport()
	if report == nil {
		return "No resource selected"
	}

	var sections []string

	// Resource header
	icon := styles.StatusIcon(report.Status)
	statusStyle := m.styles.StatusStyle(report.Status)

	header := fmt.Sprintf(
		"  %s %s\n  %s",
		icon,
		statusStyle.Render(styles.StatusText(report.Status)),
		m.styles.DiffPath.Render(report.Resource.String()),
	)
	sections = append(sections, header)

	// Source file
	if report.SourceFile != "" {
		sections = append(sections, m.styles.Subtitle.Render(
			fmt.Sprintf("\n  Source: %s", report.SourceFile),
		))
	}

	// Show diffs for drifted resources
	if report.Status == api.StatusDrifted && len(report.Diffs) > 0 {
		sections = append(sections, "\n"+m.styles.TableHeader.Render("  Differences:"))

		for i := range report.Diffs {
			diff := report.Diffs[i]
			diffLine := m.renderDiff(diff)
			sections = append(sections, diffLine)
		}
	}

	// Show error for error status
	if report.Status == api.StatusError && report.Error != "" {
		sections = append(sections, "\n"+m.styles.Error.Render(
			fmt.Sprintf("  Error: %s", report.Error),
		))
	}

	// Navigation hint
	navHint := m.styles.Subtitle.Render(fmt.Sprintf(
		"\n  Resource %d of %d",
		m.cursor+1,
		len(m.filtered),
	))
	sections = append(sections, navHint)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderDiff renders a single field diff.
func (m Model) renderDiff(diff api.FieldDiff) string {
	path := m.styles.DiffPath.Render(diff.Path)

	switch diff.Type {
	case api.DiffTypeModified:
		expected := m.styles.DiffRemove.Render(formatDiffValue(diff.Expected))
		actual := m.styles.DiffAdd.Render(formatDiffValue(diff.Actual))
		return fmt.Sprintf("    %s:\n      - %s\n      + %s", path, expected, actual)

	case api.DiffTypeAdded:
		actual := m.styles.DiffAdd.Render(formatDiffValue(diff.Actual))
		return fmt.Sprintf("    %s:\n      + %s (added)", path, actual)

	case api.DiffTypeRemoved:
		expected := m.styles.DiffRemove.Render(formatDiffValue(diff.Expected))
		return fmt.Sprintf("    %s:\n      - %s (removed)", path, expected)

	default:
		return fmt.Sprintf("    %s: (unknown diff type)", path)
	}
}

// renderHelp renders the help view.
func (m Model) renderHelp() string {
	return m.styles.Content.Render(m.help.View(m.keys))
}

// formatDiffValue formats a value for diff display.
func formatDiffValue(v any) string {
	if v == nil {
		return "<nil>"
	}
	s := fmt.Sprintf("%v", v)
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}

// renderLogs renders the logs view.
func (m Model) renderLogs() string {
	if m.logCapture == nil {
		return m.styles.Subtitle.Render("\n  Log capture not available.\n")
	}

	entries := m.logCapture.FilteredEntries(m.logsFilter)

	if len(entries) == 0 {
		filterName := logLevelName(m.logsFilter)
		return m.styles.Subtitle.Render(fmt.Sprintf("\n  No logs to display (filter: %s).\n", filterName))
	}

	var rows []string

	// Header with entry count and filter
	filterName := logLevelName(m.logsFilter)
	header := fmt.Sprintf("  📋 Logs (%d entries, filter: %s)", len(entries), filterName)
	rows = append(rows, m.styles.TableHeader.Render(header), "")

	// Calculate visible range
	visibleHeight := m.height - 12 // Account for header, footer, padding
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	startIdx := 0
	if m.logsCursor >= visibleHeight {
		startIdx = m.logsCursor - visibleHeight + 1
	}

	endIdx := startIdx + visibleHeight
	if endIdx > len(entries) {
		endIdx = len(entries)
	}

	for i := startIdx; i < endIdx; i++ {
		entry := entries[i]

		// Format timestamp
		timeStr := m.styles.LogTime.Render(entry.Time.Format("15:04:05.000"))

		// Format level with appropriate style
		var levelStr string
		switch entry.Level {
		case logcapture.LevelDebug:
			levelStr = m.styles.LogDebug.Render("DEBUG")
		case logcapture.LevelInfo:
			levelStr = m.styles.LogInfo.Render("INFO ")
		case logcapture.LevelWarn:
			levelStr = m.styles.LogWarn.Render("WARN ")
		case logcapture.LevelError:
			levelStr = m.styles.LogError.Render("ERROR")
		default:
			levelStr = "     "
		}

		// Truncate message if too long
		msg := entry.Message
		maxMsgLen := m.width - 30
		if maxMsgLen < 20 {
			maxMsgLen = 20
		}
		if len(msg) > maxMsgLen {
			msg = msg[:maxMsgLen-3] + "..."
		}

		// Build row
		row := fmt.Sprintf("  %s  %s  %s", timeStr, levelStr, msg)

		// Highlight selected row
		if i == m.logsCursor {
			row = m.styles.TableSelected.Render(row)
		}

		rows = append(rows, row)
	}

	// Scroll indicator
	if len(entries) > visibleHeight {
		scrollInfo := m.styles.Subtitle.Render(fmt.Sprintf(
			"\n  Showing %d-%d of %d entries",
			startIdx+1, endIdx, len(entries),
		))
		rows = append(rows, scrollInfo)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// logLevelName returns the display name for a log level.
func logLevelName(level logcapture.Level) string {
	switch level {
	case logcapture.LevelDebug:
		return "all"
	case logcapture.LevelInfo:
		return "info+"
	case logcapture.LevelWarn:
		return "warn+"
	case logcapture.LevelError:
		return "error"
	default:
		return "all"
	}
}
