package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/indrasvat/dorikin/internal/logcapture"
	"github.com/indrasvat/dorikin/pkg/api"
)

// ScanFunc is a function that performs a scan and returns results.
type ScanFunc func() (*api.ScanResult, error)

// Run starts the TUI with the given scan result and optional refresh callback.
func Run(result *api.ScanResult, scanFunc ScanFunc) error {
	return RunWithInterval(result, scanFunc, DefaultAutoRefreshInterval)
}

// RunWithInterval starts the TUI with a custom auto-refresh interval.
func RunWithInterval(result *api.ScanResult, scanFunc ScanFunc, interval time.Duration) error {
	model := NewModelWithInterval(result, scanFunc, interval)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}

// RunWithCapture starts the TUI with log capture support.
func RunWithCapture(result *api.ScanResult, scanFunc ScanFunc, interval time.Duration, capture *logcapture.Capture, kubeContext string) error {
	model := NewModelWithCapture(result, scanFunc, interval, capture, kubeContext)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}
