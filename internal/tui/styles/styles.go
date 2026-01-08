package styles

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/indrasvat/dorikin/pkg/api"
)

// AE86 Panda Trueno Color Palette
// Inspired by the legendary Toyota AE86 Sprinter Trueno
var (
	// Primary colors
	PandaWhite = lipgloss.Color("#FFFFFF")
	PandaBlack = lipgloss.Color("#1C1C1C")

	// Accent colors
	TruenoRed    = lipgloss.Color("#FF4444")
	DriftYellow  = lipgloss.Color("#FFD700")
	TokyoNeon    = lipgloss.Color("#00FF88")
	AkinaBlue    = lipgloss.Color("#4A9EFF")
	MidnightGray = lipgloss.Color("#3D3D3D")

	// Status colors
	StatusInSync  = TokyoNeon
	StatusDrifted = DriftYellow
	StatusMissing = TruenoRed
	StatusExtra   = AkinaBlue
	StatusError   = lipgloss.Color("#FF6B6B")

	// Subtle colors
	Subtle = lipgloss.Color("#626262")
	Dim    = lipgloss.Color("#4A4A4A")

	// Selection colors
	SelectionGreen = lipgloss.Color("#2A3D2A") // Dark green tint for selected row background

	// Log level colors
	LogDebugColor = Subtle
	LogInfoColor  = AkinaBlue
	LogWarnColor  = DriftYellow
	LogErrorColor = TruenoRed
)

// App is the main application style container.
type App struct {
	// Layout
	Container lipgloss.Style
	Header    lipgloss.Style
	Footer    lipgloss.Style
	Content   lipgloss.Style

	// Components
	Title      lipgloss.Style
	Subtitle   lipgloss.Style
	StatusBar  lipgloss.Style
	HelpKey    lipgloss.Style
	HelpDesc   lipgloss.Style
	HelpSep    lipgloss.Style
	Tab        lipgloss.Style
	TabActive  lipgloss.Style
	TabContent lipgloss.Style

	// Table
	TableHeader     lipgloss.Style
	TableRow        lipgloss.Style
	TableSelected   lipgloss.Style
	TableCell       lipgloss.Style
	SelectionCursor lipgloss.Style

	// Status indicators
	InSync  lipgloss.Style
	Drifted lipgloss.Style
	Missing lipgloss.Style
	Extra   lipgloss.Style
	Error   lipgloss.Style

	// Diff view
	DiffAdd    lipgloss.Style
	DiffRemove lipgloss.Style
	DiffPath   lipgloss.Style

	// Text styles
	Subtle lipgloss.Style

	// Log level styles
	LogDebug lipgloss.Style
	LogInfo  lipgloss.Style
	LogWarn  lipgloss.Style
	LogError lipgloss.Style
	LogTime  lipgloss.Style
}

// New creates a new App style set.
func New() *App {
	return &App{
		// Layout
		Container: lipgloss.NewStyle().
			Padding(1, 2),

		Header: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(Subtle).
			Padding(0, 1).
			MarginBottom(1),

		Footer: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(Subtle).
			Padding(0, 1).
			MarginTop(1),

		Content: lipgloss.NewStyle().
			Padding(0, 1),

		// Components
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(PandaWhite).
			Background(PandaBlack).
			Padding(0, 1),

		Subtitle: lipgloss.NewStyle().
			Foreground(Subtle),

		StatusBar: lipgloss.NewStyle().
			Foreground(PandaWhite).
			Background(MidnightGray).
			Padding(0, 1),

		HelpKey: lipgloss.NewStyle().
			Foreground(DriftYellow).
			Bold(true),

		HelpDesc: lipgloss.NewStyle().
			Foreground(Subtle),

		HelpSep: lipgloss.NewStyle().
			Foreground(Dim),

		Tab: lipgloss.NewStyle().
			Foreground(Subtle).
			Padding(0, 2),

		TabActive: lipgloss.NewStyle().
			Foreground(PandaWhite).
			Background(MidnightGray).
			Bold(true).
			Padding(0, 2),

		TabContent: lipgloss.NewStyle().
			Padding(1, 0),

		// Table styles
		TableHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(PandaWhite).
			Background(MidnightGray).
			Padding(0, 1),

		TableRow: lipgloss.NewStyle().
			Padding(0, 1),

		TableSelected: lipgloss.NewStyle().
			Background(SelectionGreen).
			Foreground(PandaWhite).
			Bold(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderForeground(TokyoNeon).
			Padding(0, 1),

		TableCell: lipgloss.NewStyle().
			Padding(0, 1),

		SelectionCursor: lipgloss.NewStyle().
			Foreground(TokyoNeon).
			Bold(true),

		// Status indicators
		InSync: lipgloss.NewStyle().
			Foreground(StatusInSync).
			Bold(true),

		Drifted: lipgloss.NewStyle().
			Foreground(StatusDrifted).
			Bold(true),

		Missing: lipgloss.NewStyle().
			Foreground(StatusMissing).
			Bold(true),

		Extra: lipgloss.NewStyle().
			Foreground(StatusExtra).
			Bold(true),

		Error: lipgloss.NewStyle().
			Foreground(StatusError).
			Bold(true),

		// Diff view
		DiffAdd: lipgloss.NewStyle().
			Foreground(TokyoNeon),

		DiffRemove: lipgloss.NewStyle().
			Foreground(TruenoRed),

		DiffPath: lipgloss.NewStyle().
			Foreground(AkinaBlue).
			Bold(true),

		// Text styles
		Subtle: lipgloss.NewStyle().
			Foreground(Subtle),

		// Log level styles
		LogDebug: lipgloss.NewStyle().
			Foreground(LogDebugColor),

		LogInfo: lipgloss.NewStyle().
			Foreground(LogInfoColor),

		LogWarn: lipgloss.NewStyle().
			Foreground(LogWarnColor).
			Bold(true),

		LogError: lipgloss.NewStyle().
			Foreground(LogErrorColor).
			Bold(true),

		LogTime: lipgloss.NewStyle().
			Foreground(Subtle),
	}
}

// StatusStyle returns the appropriate style for a drift status.
func (a *App) StatusStyle(status api.DriftStatus) lipgloss.Style {
	switch status {
	case api.StatusInSync:
		return a.InSync
	case api.StatusDrifted:
		return a.Drifted
	case api.StatusMissing:
		return a.Missing
	case api.StatusExtra:
		return a.Extra
	case api.StatusError:
		return a.Error
	default:
		return lipgloss.NewStyle()
	}
}

// StatusIcon returns the icon for a drift status.
func StatusIcon(status api.DriftStatus) string {
	switch status {
	case api.StatusInSync:
		return "🏁"
	case api.StatusDrifted:
		return "🚨"
	case api.StatusMissing:
		return "🛑"
	case api.StatusExtra:
		return "➕"
	case api.StatusError:
		return "💥"
	default:
		return "❓"
	}
}

// StatusText returns the display text for a drift status.
func StatusText(status api.DriftStatus) string {
	switch status {
	case api.StatusInSync:
		return "IN_SYNC"
	case api.StatusDrifted:
		return "DRIFTED"
	case api.StatusMissing:
		return "MISSING"
	case api.StatusExtra:
		return "EXTRA"
	case api.StatusError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}
