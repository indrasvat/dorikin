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
			m.styles.HelpKey.Render("1-4")+m.styles.HelpDesc.Render(" tabs"),
			m.styles.HelpKey.Render("↑↓")+m.styles.HelpDesc.Render(" scroll"),
			m.styles.HelpKey.Render("←→")+m.styles.HelpDesc.Render(" laps"),
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
	visibleHeight := max(5, m.height-10) // Account for header, footer, padding

	startIdx := 0
	if m.cursor >= visibleHeight {
		startIdx = m.cursor - visibleHeight + 1
	}

	endIdx := min(startIdx+visibleHeight, len(m.filtered))

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

	sections := []string{
		m.renderDetailHeader(report),
		m.renderDetailTabs(),
		m.renderDetailContent(report),
	}

	// Lap indicator (racing theme!)
	lapIndicator := m.styles.Subtle.Render(fmt.Sprintf(
		"Lap %d of %d  ◀ ▶",
		m.cursor+1,
		len(m.filtered),
	))
	sections = append(sections, "\n"+lipgloss.NewStyle().Padding(0, 2).Render(
		lipgloss.PlaceHorizontal(m.width-12, lipgloss.Right, lapIndicator),
	))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderDetailHeader renders the header section of detail view.
func (m Model) renderDetailHeader(report *api.DriftReport) string {
	icon := styles.StatusIcon(report.Status)
	statusStyle := m.styles.StatusStyle(report.Status)

	// Resource name: Kind/Name
	resourceName := fmt.Sprintf("%s/%s", report.Resource.Kind, report.Resource.Name)
	header := fmt.Sprintf("  %s  %s", icon, statusStyle.Render(resourceName))

	// Separator line
	sep := m.styles.Subtle.Render("  " + strings.Repeat("─", min(70, m.width-10)))

	// Metadata line: namespace • apiVersion • source file
	var metaParts []string
	if report.Resource.Namespace != "" {
		metaParts = append(metaParts, report.Resource.Namespace)
	}
	metaParts = append(metaParts, report.Resource.APIVersion)
	if report.SourceFile != "" {
		metaParts = append(metaParts, truncateSourcePath(report.SourceFile, 40))
	}
	metaLine := m.styles.Subtle.Render("  " + strings.Join(metaParts, "  •  "))

	return lipgloss.JoinVertical(lipgloss.Left, header, sep, metaLine, "")
}

// renderDetailTabs renders the tab bar for detail view.
func (m Model) renderDetailTabs() string {
	tabs := []struct {
		name   string
		active bool
	}{
		{"Diffs", m.detailTab == TabDiffs},
		{"Manifest", m.detailTab == TabManifest},
		{"Cluster", m.detailTab == TabCluster},
		{"Meta", m.detailTab == TabMeta},
	}

	var tabStrs []string
	for _, tab := range tabs {
		var style lipgloss.Style
		if tab.active {
			style = m.styles.TabActive
			tabStrs = append(tabStrs, style.Render("▸ "+tab.name))
		} else {
			style = m.styles.Tab
			tabStrs = append(tabStrs, style.Render("  "+tab.name))
		}
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabStrs...)
	underline := m.styles.Subtle.Render("  " + strings.Repeat("━", min(70, m.width-10)))

	return lipgloss.JoinVertical(lipgloss.Left, "  "+tabBar, underline, "")
}

// renderDetailContent renders the content for the active tab.
func (m Model) renderDetailContent(report *api.DriftReport) string {
	switch m.detailTab {
	case TabDiffs:
		return m.renderDiffsTab(report)
	case TabManifest:
		return m.renderManifestTab(report)
	case TabCluster:
		return m.renderClusterTab(report)
	case TabMeta:
		return m.renderMetaTab(report)
	default:
		return m.renderDiffsTab(report)
	}
}

// renderManifestTab renders the full manifest YAML with drift highlighting.
func (m Model) renderManifestTab(report *api.DriftReport) string {
	if report.ManifestObject == nil {
		if report.Status == api.StatusInSync {
			return m.styles.Subtle.Render("  ✓ Resource in sync - manifest not stored for memory efficiency")
		}
		if report.Status == api.StatusExtra {
			return m.styles.Subtle.Render("  Resource exists only in cluster - no manifest available")
		}
		return m.styles.Subtle.Render("  No manifest data available")
	}

	// Build drift paths set for highlighting
	driftPaths := make(map[string]bool)
	for _, diff := range report.Diffs {
		driftPaths[diff.Path] = true
	}

	yaml := m.renderYAMLObject(report.ManifestObject, "", driftPaths, true)
	return m.renderScrollableYAML(yaml, "Manifest")
}

// renderClusterTab renders the full cluster YAML with drift highlighting.
func (m Model) renderClusterTab(report *api.DriftReport) string {
	if report.ClusterObject == nil {
		if report.Status == api.StatusInSync {
			return m.styles.Subtle.Render("  ✓ Resource in sync - cluster state not stored for memory efficiency")
		}
		if report.Status == api.StatusMissing {
			return m.styles.Subtle.Render("  Resource not found in cluster")
		}
		return m.styles.Subtle.Render("  No cluster data available")
	}

	// Build drift paths set for highlighting
	driftPaths := make(map[string]bool)
	for _, diff := range report.Diffs {
		driftPaths[diff.Path] = true
	}

	yaml := m.renderYAMLObject(report.ClusterObject, "", driftPaths, false)
	return m.renderScrollableYAML(yaml, "Cluster")
}

// renderMetaTab renders metadata information about the resource.
func (m Model) renderMetaTab(report *api.DriftReport) string {
	var lines []string

	// Resource identification
	lines = append(lines, m.styles.DiffPath.Render("  Resource"))
	lines = append(lines, fmt.Sprintf("    Kind:       %s", report.Resource.Kind))
	lines = append(lines, fmt.Sprintf("    Name:       %s", report.Resource.Name))
	lines = append(lines, fmt.Sprintf("    Namespace:  %s", valueOrDefault(report.Resource.Namespace, "(cluster-scoped)")))
	lines = append(lines, fmt.Sprintf("    APIVersion: %s", report.Resource.APIVersion))
	lines = append(lines, "")

	// Source file
	if report.SourceFile != "" {
		lines = append(lines, m.styles.DiffPath.Render("  Source"))
		lines = append(lines, fmt.Sprintf("    File: %s", report.SourceFile))
		lines = append(lines, "")
	}

	// Status
	lines = append(lines, m.styles.DiffPath.Render("  Status"))
	statusStyle := m.styles.StatusStyle(report.Status)
	lines = append(lines, fmt.Sprintf("    Status: %s %s",
		styles.StatusIcon(report.Status),
		statusStyle.Render(string(report.Status))))
	lines = append(lines, fmt.Sprintf("    Checked: %s", report.CheckedAt.Format("2006-01-02 15:04:05")))
	if len(report.Diffs) > 0 {
		lines = append(lines, fmt.Sprintf("    Diffs:   %d field%s", len(report.Diffs), pluralize(len(report.Diffs))))
	}
	if report.Error != "" {
		lines = append(lines, fmt.Sprintf("    Error:   %s", report.Error))
	}
	lines = append(lines, "")

	// Labels and annotations from manifest if available
	if report.ManifestObject != nil {
		if meta, ok := report.ManifestObject["metadata"].(map[string]any); ok {
			if labels, ok := meta["labels"].(map[string]any); ok && len(labels) > 0 {
				lines = append(lines, m.styles.DiffPath.Render("  Labels"))
				for k, v := range labels {
					lines = append(lines, fmt.Sprintf("    %s: %v", k, v))
				}
				lines = append(lines, "")
			}
			if annotations, ok := meta["annotations"].(map[string]any); ok && len(annotations) > 0 {
				lines = append(lines, m.styles.DiffPath.Render("  Annotations"))
				for k, v := range annotations {
					valStr := fmt.Sprintf("%v", v)
					if len(valStr) > 50 {
						valStr = valStr[:47] + "..."
					}
					lines = append(lines, fmt.Sprintf("    %s: %s", k, valStr))
				}
				lines = append(lines, "")
			}
		}
	}

	return strings.Join(lines, "\n")
}

// renderYAMLObject renders an object as YAML-like text with drift highlighting.
func (m Model) renderYAMLObject(obj map[string]any, path string, driftPaths map[string]bool, isManifest bool) string {
	var lines []string

	// Sort keys for consistent output
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sortStrings(keys)

	for _, k := range keys {
		v := obj[k]
		fullPath := path + "." + k

		// Check if this path or any child path has drift
		isDrifted := driftPaths[fullPath]
		for dp := range driftPaths {
			if strings.HasPrefix(dp, fullPath+".") || strings.HasPrefix(dp, fullPath+"[") {
				isDrifted = true
				break
			}
		}

		keyStyle := m.styles.Subtle
		if isDrifted {
			if isManifest {
				keyStyle = m.styles.DiffRemove
			} else {
				keyStyle = m.styles.DiffAdd
			}
		}

		switch val := v.(type) {
		case map[string]any:
			lines = append(lines, keyStyle.Render(k+":"))
			nested := m.renderYAMLObject(val, fullPath, driftPaths, isManifest)
			for _, line := range strings.Split(nested, "\n") {
				if line != "" {
					lines = append(lines, "  "+line)
				}
			}
		case []any:
			lines = append(lines, keyStyle.Render(k+":"))
			for i, item := range val {
				itemPath := fmt.Sprintf("%s[%d]", fullPath, i)
				itemDrifted := driftPaths[itemPath]
				for dp := range driftPaths {
					if strings.HasPrefix(dp, itemPath+".") {
						itemDrifted = true
						break
					}
				}

				itemStyle := m.styles.Subtle
				if itemDrifted {
					if isManifest {
						itemStyle = m.styles.DiffRemove
					} else {
						itemStyle = m.styles.DiffAdd
					}
				}

				switch itemVal := item.(type) {
				case map[string]any:
					nested := m.renderYAMLObject(itemVal, itemPath, driftPaths, isManifest)
					nestedLines := strings.Split(nested, "\n")
					if len(nestedLines) > 0 {
						lines = append(lines, itemStyle.Render("- ")+nestedLines[0])
						for _, nl := range nestedLines[1:] {
							if nl != "" {
								lines = append(lines, "  "+nl)
							}
						}
					}
				default:
					lines = append(lines, itemStyle.Render(fmt.Sprintf("- %v", item)))
				}
			}
		default:
			valStr := fmt.Sprintf("%v", v)
			if len(valStr) > 80 {
				valStr = valStr[:77] + "..."
			}
			lines = append(lines, keyStyle.Render(fmt.Sprintf("%s: %s", k, valStr)))
		}
	}

	return strings.Join(lines, "\n")
}

// renderScrollableYAML wraps YAML content with scroll info.
func (m Model) renderScrollableYAML(yaml string, title string) string {
	lines := strings.Split(yaml, "\n")

	// Calculate visible range
	visibleHeight := max(5, m.height-16)
	startIdx := m.detailScroll
	endIdx := min(startIdx+visibleHeight, len(lines))

	if startIdx >= len(lines) {
		startIdx = max(0, len(lines)-visibleHeight)
		endIdx = len(lines)
	}

	// Header
	header := fmt.Sprintf("  %s YAML (%d lines)", title, len(lines))
	if len(lines) > visibleHeight {
		header += fmt.Sprintf("  [%d-%d]", startIdx+1, endIdx)
	}

	var result []string
	result = append(result, m.styles.Subtle.Render(header))
	result = append(result, "")

	// Visible lines with indentation
	for i := startIdx; i < endIdx; i++ {
		result = append(result, "  "+lines[i])
	}

	// Scroll indicator
	if len(lines) > visibleHeight {
		scrollInfo := m.styles.Subtle.Render(fmt.Sprintf("\n  ↑↓ to scroll (%d more lines below)", max(0, len(lines)-endIdx)))
		result = append(result, scrollInfo)
	}

	return strings.Join(result, "\n")
}

// sortStrings sorts a slice of strings in place.
func sortStrings(s []string) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// valueOrDefault returns the value if non-empty, otherwise the default.
func valueOrDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

// renderDiffsTab renders the diffs tab content with side-by-side boxes.
func (m Model) renderDiffsTab(report *api.DriftReport) string {
	var sections []string

	// Handle non-drifted statuses
	if report.Status == api.StatusError && report.Error != "" {
		return m.styles.Error.Render(fmt.Sprintf("  Error: %s", report.Error))
	}

	if report.Status == api.StatusMissing {
		return m.styles.Missing.Render("  Resource defined in manifest but not found in cluster")
	}

	if report.Status == api.StatusExtra {
		return m.styles.Extra.Render("  Resource exists in cluster but not in manifests")
	}

	if report.Status == api.StatusInSync {
		return m.styles.InSync.Render("  ✓ Resource is in sync with manifest")
	}

	if len(report.Diffs) == 0 {
		return m.styles.Subtle.Render("  No differences to display")
	}

	// Diffs header with count and scroll position
	diffCount := len(report.Diffs)
	scrollIndicator := fmt.Sprintf("[%d/%d]", m.diffCursor+1, diffCount)
	header := fmt.Sprintf("  %d diff%s", diffCount, pluralize(diffCount))

	headerLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.styles.Subtitle.Render(header),
		strings.Repeat(" ", max(0, m.width-30-len(header)-len(scrollIndicator))),
		m.styles.Subtle.Render(scrollIndicator+" ▲▼"),
	)
	sections = append(sections, headerLine, "")

	// Calculate visible range for scrolling
	visibleDiffs := max(1, (m.height-18)/7) // Each diff box is ~7 lines
	startIdx := m.detailScroll
	endIdx := min(startIdx+visibleDiffs, diffCount)

	// Render visible diffs
	for i := startIdx; i < endIdx; i++ {
		diff := report.Diffs[i]
		isSelected := i == m.diffCursor
		diffBox := m.renderDiffBox(diff, isSelected)
		sections = append(sections, diffBox)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderDiffBox renders a single diff with smart render mode detection.
func (m Model) renderDiffBox(diff api.FieldDiff, selected bool) string {
	// Detect optimal render mode for this diff
	renderMode := DetectRenderMode(diff.Expected, diff.Actual)

	switch renderMode {
	case api.RenderModeUnifiedDiff:
		return m.renderUnifiedDiffBox(diff, selected)
	case api.RenderModeStructuralAdd:
		return m.renderStructuralAddBox(diff, selected)
	case api.RenderModeStructuralDel:
		return m.renderStructuralDelBox(diff, selected)
	case api.RenderModeReplacement:
		return m.renderReplacementBox(diff, selected)
	default:
		return m.renderSideBySideBox(diff, selected)
	}
}

// renderSideBySideBox renders a simple side-by-side comparison box.
func (m Model) renderSideBySideBox(diff api.FieldDiff, selected bool) string {
	// Calculate box width (half of available width minus padding)
	boxWidth := max(20, (m.width-16)/2)

	// Path line with selection indicator
	var pathPrefix string
	if selected {
		pathPrefix = m.styles.SelectionCursor.Render("▸ ")
	} else {
		pathPrefix = "  "
	}

	path := m.styles.DiffPath.Render(truncatePath(diff.Path, m.width-10))
	pathLine := pathPrefix + path

	// Format values
	var expectedVal, actualVal string
	switch diff.Type {
	case api.DiffTypeModified:
		expectedVal = formatDiffValue(diff.Expected)
		actualVal = formatDiffValue(diff.Actual)
	case api.DiffTypeAdded:
		expectedVal = "<not set>"
		actualVal = formatDiffValue(diff.Actual)
	case api.DiffTypeRemoved:
		expectedVal = formatDiffValue(diff.Expected)
		actualVal = "<removed>"
	}

	// Create side-by-side box
	// ╭────────────────────────┬────────────────────────╮
	// │  Manifest              │  Cluster               │
	// │  value1                │  value2                │
	// ╰────────────────────────┴────────────────────────╯

	topBorder := fmt.Sprintf("  ╭%s┬%s╮",
		strings.Repeat("─", boxWidth),
		strings.Repeat("─", boxWidth))

	headerRow := fmt.Sprintf("  │  %s│  %s│",
		padRight(m.styles.DiffRemove.Render("Manifest"), boxWidth-2),
		padRight(m.styles.DiffAdd.Render("Cluster"), boxWidth-2))

	// Truncate values to fit in box
	truncatedExpected := truncateValue(expectedVal, boxWidth-4)
	truncatedActual := truncateValue(actualVal, boxWidth-4)

	valueRow := fmt.Sprintf("  │  %s│  %s│",
		padRight(truncatedExpected, boxWidth-2),
		padRight(truncatedActual, boxWidth-2))

	bottomBorder := fmt.Sprintf("  ╰%s┴%s╯",
		strings.Repeat("─", boxWidth),
		strings.Repeat("─", boxWidth))

	box := lipgloss.JoinVertical(lipgloss.Left,
		pathLine,
		topBorder,
		headerRow,
		valueRow,
		bottomBorder,
		"",
	)

	return box
}

// renderUnifiedDiffBox renders a unified diff for multi-line text changes.
func (m Model) renderUnifiedDiffBox(diff api.FieldDiff, selected bool) string {
	// Path line with selection indicator
	var pathPrefix string
	if selected {
		pathPrefix = m.styles.SelectionCursor.Render("▸ ")
	} else {
		pathPrefix = "  "
	}

	// Convert values to strings
	expectedStr := fmt.Sprintf("%v", diff.Expected)
	actualStr := fmt.Sprintf("%v", diff.Actual)

	// Get diff stats
	added, removed := DiffStats(expectedStr, actualStr)
	statsText := fmt.Sprintf("[+%d -%d lines]", added, removed)

	path := m.styles.DiffPath.Render(truncatePath(diff.Path, m.width-len(statsText)-15))
	pathLine := pathPrefix + path + "  " + m.styles.Subtle.Render(statsText)

	// Separator line
	sepLine := "  " + strings.Repeat("━", min(m.width-8, 70))

	// Generate unified diff
	renderer := NewDiffRenderer(m.styles, m.width-8)
	unifiedDiff := renderer.RenderUnifiedDiff(expectedStr, actualStr, 3)

	// Limit lines shown to prevent overflow
	lines := strings.Split(unifiedDiff, "\n")
	maxLines := max(5, (m.height-20)/2)
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, m.styles.Subtle.Render(fmt.Sprintf("  ... %d more lines", len(lines)-maxLines)))
	}

	// Indent each line
	for i, line := range lines {
		lines[i] = "  " + line
	}

	box := lipgloss.JoinVertical(lipgloss.Left,
		pathLine,
		m.styles.DiffHunk.Render(sepLine),
		"",
		strings.Join(lines, "\n"),
		"",
	)

	return box
}

// renderStructuralAddBox renders a preview of structurally added content.
func (m Model) renderStructuralAddBox(diff api.FieldDiff, selected bool) string {
	// Path line with selection indicator
	var pathPrefix string
	if selected {
		pathPrefix = m.styles.SelectionCursor.Render("▸ ")
	} else {
		pathPrefix = "  "
	}

	path := m.styles.DiffPath.Render(truncatePath(diff.Path, m.width-20))
	badge := m.styles.DiffAdd.Render("[ADDED]")
	pathLine := pathPrefix + path + "  " + badge

	// Separator line
	sepLine := "  " + strings.Repeat("━", min(m.width-8, 70))

	// Format the added value
	formattedValue := FormatValue(diff.Actual, 0) // No truncation for preview

	// For complex objects, show a preview
	var preview string
	switch v := diff.Actual.(type) {
	case map[string]any:
		preview = m.formatMapPreview(v, "  ")
	case []any:
		preview = m.formatSlicePreview(v, "  ")
	default:
		preview = "  " + m.styles.DiffAdd.Render(formattedValue)
	}

	// Limit preview lines
	lines := strings.Split(preview, "\n")
	maxLines := max(5, (m.height-20)/2)
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, m.styles.Subtle.Render(fmt.Sprintf("  ... %d more lines", len(lines)-maxLines)))
	}

	box := lipgloss.JoinVertical(lipgloss.Left,
		pathLine,
		m.styles.DiffHunk.Render(sepLine),
		"",
		m.styles.Subtle.Render("  Content exists in cluster but not in manifest:"),
		"",
		strings.Join(lines, "\n"),
		"",
	)

	return box
}

// renderStructuralDelBox renders a preview of structurally deleted content.
func (m Model) renderStructuralDelBox(diff api.FieldDiff, selected bool) string {
	// Path line with selection indicator
	var pathPrefix string
	if selected {
		pathPrefix = m.styles.SelectionCursor.Render("▸ ")
	} else {
		pathPrefix = "  "
	}

	path := m.styles.DiffPath.Render(truncatePath(diff.Path, m.width-20))
	badge := m.styles.DiffRemove.Render("[REMOVED]")
	pathLine := pathPrefix + path + "  " + badge

	// Separator line
	sepLine := "  " + strings.Repeat("━", min(m.width-8, 70))

	// Format the removed value
	formattedValue := FormatValue(diff.Expected, 0) // No truncation for preview

	// For complex objects, show a preview
	var preview string
	switch v := diff.Expected.(type) {
	case map[string]any:
		preview = m.formatMapPreview(v, "  ")
	case []any:
		preview = m.formatSlicePreview(v, "  ")
	default:
		preview = "  " + m.styles.DiffRemove.Render(formattedValue)
	}

	// Limit preview lines
	lines := strings.Split(preview, "\n")
	maxLines := max(5, (m.height-20)/2)
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, m.styles.Subtle.Render(fmt.Sprintf("  ... %d more lines", len(lines)-maxLines)))
	}

	box := lipgloss.JoinVertical(lipgloss.Left,
		pathLine,
		m.styles.DiffHunk.Render(sepLine),
		"",
		m.styles.Subtle.Render("  Content exists in manifest but removed from cluster:"),
		"",
		strings.Join(lines, "\n"),
		"",
	)

	return box
}

// renderReplacementBox renders a toggle view for substantially different content.
func (m Model) renderReplacementBox(diff api.FieldDiff, selected bool) string {
	// Path line with selection indicator
	var pathPrefix string
	if selected {
		pathPrefix = m.styles.SelectionCursor.Render("▸ ")
	} else {
		pathPrefix = "  "
	}

	// Calculate similarity for display
	expStr := fmt.Sprintf("%v", diff.Expected)
	actStr := fmt.Sprintf("%v", diff.Actual)
	sim := stringSimilarity(expStr, actStr)
	simPercent := int((1 - sim) * 100)

	path := m.styles.DiffPath.Render(truncatePath(diff.Path, m.width-30))
	badge := m.styles.Drifted.Render(fmt.Sprintf("[REPLACED - %d%% different]", simPercent))
	pathLine := pathPrefix + path + "  " + badge

	// Separator line
	sepLine := "  " + strings.Repeat("━", min(m.width-8, 70))

	// Show summary of changes
	expLines := strings.Count(expStr, "\n") + 1
	actLines := strings.Count(actStr, "\n") + 1

	summary := fmt.Sprintf("  Object substantially rewritten (%d → %d lines, %d%% different)",
		expLines, actLines, simPercent)

	// Toggle hint
	toggleHint := "  " + m.styles.Subtle.Render("Showing unified diff would be unhelpful.")

	// Show abbreviated version of both
	// Calculate box width
	boxWidth := max(20, (m.width-16)/2)

	topBorder := fmt.Sprintf("  ╭%s┬%s╮",
		strings.Repeat("─", boxWidth),
		strings.Repeat("─", boxWidth))

	headerRow := fmt.Sprintf("  │  %s│  %s│",
		padRight(m.styles.DiffRemove.Render("Manifest"), boxWidth-2),
		padRight(m.styles.DiffAdd.Render("Cluster"), boxWidth-2))

	// Show first few chars of each
	truncExpected := truncateValue(expStr, boxWidth-4)
	truncActual := truncateValue(actStr, boxWidth-4)

	valueRow := fmt.Sprintf("  │  %s│  %s│",
		padRight(truncExpected, boxWidth-2),
		padRight(truncActual, boxWidth-2))

	bottomBorder := fmt.Sprintf("  ╰%s┴%s╯",
		strings.Repeat("─", boxWidth),
		strings.Repeat("─", boxWidth))

	box := lipgloss.JoinVertical(lipgloss.Left,
		pathLine,
		m.styles.DiffHunk.Render(sepLine),
		"",
		m.styles.Subtle.Render(summary),
		toggleHint,
		"",
		topBorder,
		headerRow,
		valueRow,
		bottomBorder,
		"",
	)

	return box
}

// formatMapPreview formats a map for preview display.
func (m Model) formatMapPreview(obj map[string]any, indent string) string {
	var lines []string

	for k, v := range obj {
		switch val := v.(type) {
		case map[string]any:
			lines = append(lines, indent+m.styles.DiffAdd.Render(k+":"))
			// Only show first level for nested maps
			for nk := range val {
				lines = append(lines, indent+"  "+m.styles.Subtle.Render(nk+": ..."))
			}
		case []any:
			lines = append(lines, indent+m.styles.DiffAdd.Render(fmt.Sprintf("%s: [%d items]", k, len(val))))
		default:
			lines = append(lines, indent+m.styles.DiffAdd.Render(fmt.Sprintf("%s: %v", k, v)))
		}
	}

	return strings.Join(lines, "\n")
}

// formatSlicePreview formats a slice for preview display.
func (m Model) formatSlicePreview(arr []any, indent string) string {
	var lines []string

	for i, item := range arr {
		if i >= 5 { // Limit to first 5 items
			lines = append(lines, indent+m.styles.Subtle.Render(fmt.Sprintf("... and %d more items", len(arr)-5)))
			break
		}
		switch val := item.(type) {
		case map[string]any:
			// Try to find a name field for better display
			if name, ok := val["name"].(string); ok {
				lines = append(lines, indent+m.styles.DiffAdd.Render(fmt.Sprintf("- %s: {...}", name)))
			} else {
				lines = append(lines, indent+m.styles.DiffAdd.Render(fmt.Sprintf("- {...%d keys}", len(val))))
			}
		default:
			lines = append(lines, indent+m.styles.DiffAdd.Render(fmt.Sprintf("- %v", val)))
		}
	}

	return strings.Join(lines, "\n")
}

// truncateSourcePath truncates a source file path for display.
func truncateSourcePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	// Keep the filename and as much of the path as possible
	parts := strings.Split(path, "/")
	if len(parts) <= 1 {
		return path[:maxLen-3] + "..."
	}
	filename := parts[len(parts)-1]
	if len(filename) >= maxLen-3 {
		return "..." + filename[len(filename)-(maxLen-3):]
	}
	return ".../" + filename
}

// truncatePath truncates a JSON path for display.
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return path[:maxLen-3] + "..."
}

// truncateValue truncates a value string for display in diff box.
func truncateValue(val string, maxLen int) string {
	if len(val) <= maxLen {
		return val
	}
	return val[:maxLen-3] + "..."
}

// padRight pads a string to the specified width.
func padRight(s string, width int) string {
	visibleLen := lipgloss.Width(s)
	if visibleLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visibleLen)
}

// pluralize returns "s" if count != 1.
func pluralize(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
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
	visibleHeight := max(5, m.height-12) // Account for header, footer, padding

	startIdx := 0
	if m.logsCursor >= visibleHeight {
		startIdx = m.logsCursor - visibleHeight + 1
	}

	endIdx := min(startIdx+visibleHeight, len(entries))

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
		maxMsgLen := max(20, m.width-30)
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
