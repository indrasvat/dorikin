package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/indrasvat/dorikin/pkg/api"
)

// Run starts the TUI with the given scan result.
func Run(result *api.ScanResult) error {
	model := NewModel(result)

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
