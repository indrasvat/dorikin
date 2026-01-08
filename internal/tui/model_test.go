package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/indrasvat/dorikin/pkg/api"
)

func testScanResult() *api.ScanResult {
	return &api.ScanResult{
		Duration: time.Second,
		Summary: api.ScanSummary{
			TotalResources: 5,
			InSync:         2,
			Drifted:        1,
			Missing:        1,
			Errors:         1,
		},
		Reports: []api.DriftReport{
			{Resource: api.ResourceRef{Kind: "Deployment", Name: "nginx", Namespace: "default"}, Status: api.StatusDrifted},
			{Resource: api.ResourceRef{Kind: "Service", Name: "web", Namespace: "default"}, Status: api.StatusInSync},
			{Resource: api.ResourceRef{Kind: "ConfigMap", Name: "config", Namespace: "default"}, Status: api.StatusInSync},
			{Resource: api.ResourceRef{Kind: "Secret", Name: "creds", Namespace: "default"}, Status: api.StatusMissing},
			{Resource: api.ResourceRef{Kind: "Deployment", Name: "broken", Namespace: "default"}, Status: api.StatusError},
		},
	}
}

func TestNewModel(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)

	if m.result != result {
		t.Error("result not set")
	}
	if len(m.reports) != 5 {
		t.Errorf("reports = %d, want 5", len(m.reports))
	}
	if m.viewMode != ViewList {
		t.Errorf("viewMode = %v, want ViewList", m.viewMode)
	}
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
	if m.showAll {
		t.Error("showAll should be false by default")
	}
}

func TestNewModel_InitialFilter(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)

	// By default, showAll=false, so IN_SYNC resources should be filtered out
	// We have 5 resources: 2 IN_SYNC, 1 DRIFTED, 1 MISSING, 1 ERROR
	// So filtered should have 3 resources
	if len(m.filtered) != 3 {
		t.Errorf("filtered = %d, want 3 (excluding IN_SYNC)", len(m.filtered))
	}
}

func TestModel_KeyNavigation_Down(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)

	// Should start at cursor 0
	if m.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", m.cursor)
	}

	// Press down
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(Model)

	if m.cursor != 1 {
		t.Errorf("cursor after down = %d, want 1", m.cursor)
	}
}

func TestModel_KeyNavigation_Up(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)
	m.cursor = 2

	// Press up
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(Model)

	if m.cursor != 1 {
		t.Errorf("cursor after up = %d, want 1", m.cursor)
	}
}

func TestModel_KeyNavigation_UpAtZero(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)

	// Press up at cursor 0 - should stay at 0
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(Model)

	if m.cursor != 0 {
		t.Errorf("cursor after up at 0 = %d, want 0", m.cursor)
	}
}

func TestModel_KeyNavigation_DownAtEnd(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)
	m.cursor = len(m.filtered) - 1

	// Press down at end - should stay at end
	lastPos := m.cursor
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(Model)

	if m.cursor != lastPos {
		t.Errorf("cursor after down at end = %d, want %d", m.cursor, lastPos)
	}
}

func TestModel_ViewToggle(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)

	if m.viewMode != ViewList {
		t.Fatal("should start in list view")
	}

	// Press enter to go to detail view
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(Model)

	if m.viewMode != ViewDetail {
		t.Errorf("viewMode after enter = %v, want ViewDetail", m.viewMode)
	}

	// Press escape to go back to list view
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = newModel.(Model)

	if m.viewMode != ViewList {
		t.Errorf("viewMode after escape = %v, want ViewList", m.viewMode)
	}
}

func TestModel_HelpView(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)

	// Press ? to open help
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = newModel.(Model)

	if m.viewMode != ViewHelp {
		t.Errorf("viewMode after ? = %v, want ViewHelp", m.viewMode)
	}

	// Press ? again to close help
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = newModel.(Model)

	if m.viewMode != ViewList {
		t.Errorf("viewMode after second ? = %v, want ViewList", m.viewMode)
	}
}

func TestModel_StatusFilter(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)

	initialCount := len(m.filtered)

	// Press f to cycle filter
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = newModel.(Model)

	// First cycle should filter to DRIFTED only
	if m.filter != api.StatusDrifted {
		t.Errorf("filter after f = %v, want StatusDrifted", m.filter)
	}
	if len(m.filtered) >= initialCount {
		t.Errorf("filtered count should decrease after filtering to DRIFTED")
	}
}

func TestModel_ToggleShowAll(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)

	initialCount := len(m.filtered)

	// Press o to toggle showing IN_SYNC
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m = newModel.(Model)

	if !m.showAll {
		t.Error("showAll should be true after pressing o")
	}
	// Should now include IN_SYNC resources
	if len(m.filtered) <= initialCount {
		t.Errorf("filtered count should increase when showAll is true")
	}
}

func TestModel_Quit(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)

	// Press q to quit
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = newModel.(Model)

	if !m.quitting {
		t.Error("quitting should be true after pressing q")
	}
	if cmd == nil {
		t.Error("should return tea.Quit command")
	}
}

func TestModel_WindowResize(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)

	if m.ready {
		t.Error("should not be ready before WindowSizeMsg")
	}

	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newModel.(Model)

	if !m.ready {
		t.Error("should be ready after WindowSizeMsg")
	}
	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
	if m.height != 40 {
		t.Errorf("height = %d, want 40", m.height)
	}
}

func TestModel_SelectedReport(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)

	report := m.selectedReport()
	if report == nil {
		t.Fatal("selectedReport() returned nil")
	}

	// Move cursor and check selection changes
	m.cursor = 1
	report2 := m.selectedReport()
	if report2 == nil {
		t.Fatal("selectedReport() returned nil at cursor 1")
	}
	if report == report2 {
		t.Error("selected report should change when cursor moves")
	}
}

func TestModel_SelectedReport_Empty(t *testing.T) {
	// Create result with only IN_SYNC resources
	result := &api.ScanResult{
		Reports: []api.DriftReport{
			{Resource: api.ResourceRef{Kind: "Deployment", Name: "app"}, Status: api.StatusInSync},
		},
	}
	m := NewModel(result, nil)

	// With showAll=false, no resources should be filtered
	if len(m.filtered) != 0 {
		t.Fatalf("filtered = %d, want 0", len(m.filtered))
	}

	report := m.selectedReport()
	if report != nil {
		t.Error("selectedReport() should return nil when no filtered resources")
	}
}

func TestModel_CycleFilter(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)

	filters := []api.DriftStatus{
		api.StatusDrifted,
		api.StatusMissing,
		api.StatusExtra,
		api.StatusError,
		"", // back to all
	}

	for _, expectedFilter := range filters {
		m.cycleFilter()
		if m.filter != expectedFilter {
			t.Errorf("filter = %v, want %v", m.filter, expectedFilter)
		}
	}
}

func TestModel_ApplyFilter_CursorReset(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)
	m.showAll = true
	m.applyFilter()

	// Move cursor to end
	m.cursor = len(m.filtered) - 1

	// Apply a filter that reduces results
	m.filter = api.StatusDrifted
	m.applyFilter()

	// Cursor should be reset if it's out of bounds
	if m.cursor >= len(m.filtered) {
		t.Error("cursor should be reset when filter reduces results")
	}
}

func TestModel_RefreshCallback(t *testing.T) {
	result := testScanResult()
	scanFunc := func() (*api.ScanResult, error) {
		return result, nil
	}
	m := NewModel(result, scanFunc)

	// Press r to refresh
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = newModel.(Model)

	if !m.refreshing {
		t.Error("refreshing should be true after pressing r")
	}
	if cmd == nil {
		t.Error("should return refresh command")
	}
}

func TestModel_AutoRefresh(t *testing.T) {
	result := testScanResult()
	m := NewModel(result, nil)

	if m.autoRefresh {
		t.Error("autoRefresh should be false by default")
	}

	// Press a to toggle auto-refresh
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = newModel.(Model)

	if !m.autoRefresh {
		t.Error("autoRefresh should be true after pressing a")
	}

	// Press a again to disable
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = newModel.(Model)

	if m.autoRefresh {
		t.Error("autoRefresh should be false after pressing a again")
	}
}
