// Package testinit provides test initialization for consistent TUI output.
// Import with _ "github.com/indrasvat/dorikin/internal/testinit" in test files
// that require deterministic styling output (e.g., golden file tests).
package testinit

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() {
	// Force ASCII color profile for CI consistency.
	// This ensures TUI tests produce identical output across environments.
	lipgloss.SetColorProfile(termenv.Ascii)
}
