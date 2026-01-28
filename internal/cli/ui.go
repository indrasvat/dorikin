package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dorikin/internal/logcapture"
	"github.com/indrasvat/dorikin/internal/tui"
	"github.com/indrasvat/dorikin/pkg/api"
)

var uiCmd = &cobra.Command{
	Use:   "ui [paths...]",
	Short: "Launch interactive TUI",
	Long: `Launch the interactive TUI to explore drift detection results.

The TUI provides:
  - Resource list with status indicators
  - Detailed diff view for drifted resources
  - Filtering by status
  - Keyboard navigation

Examples:
  # Launch TUI for manifests directory
  dorikin ui ./manifests/

  # Launch TUI with -f flag
  dorikin ui -f ./manifests/

  # Launch TUI with namespace filter
  dorikin ui -n production ./manifests/`,
	RunE: runUI,
}

var (
	uiFlags           ScanFlags
	uiRefreshInterval int
)

func init() {
	rootCmd.AddCommand(uiCmd)

	RegisterScanFlags(uiCmd, &uiFlags)
	uiCmd.Flags().IntVar(&uiRefreshInterval, "refresh-interval", 5, "auto-refresh interval in seconds (press 'a' to toggle)")
}

func runUI(cmd *cobra.Command, args []string) error {
	// Initialize log capture system
	capture, err := initLogCaptureForTUI(cmd)
	if err != nil {
		return fmt.Errorf("initializing log capture: %w", err)
	}

	// Start capturing stderr to prevent logs bleeding into TUI
	if err := capture.Start(); err != nil {
		return fmt.Errorf("starting log capture: %w", err)
	}
	defer capture.Stop()

	// Set as global for logging throughout the app
	logcapture.SetGlobal(capture)
	logcapture.Info("Starting dorikin TUI")

	// Build scan context from common flags
	ctx, err := BuildScanContext(cmd, args, &uiFlags)
	if err != nil {
		return err
	}

	// Create scan function for refresh
	scanFunc := func() (*api.ScanResult, error) {
		return ctx.Detector.Scan(cmd.Context(), ctx.Options)
	}

	// Get current kubectl context for display
	kubeContext := ctx.Detector.CurrentContext()

	// Launch TUI immediately with async initial scan (no blocking!)
	// The TUI will show a loading state and update when scan completes.
	interval := time.Duration(uiRefreshInterval) * time.Second
	logcapture.Info("Launching TUI (scan will run async)")
	return tui.RunWithCaptureAsync(scanFunc, interval, capture, kubeContext)
}

// initLogCaptureForTUI creates log capture for TUI (always captures stderr).
func initLogCaptureForTUI(cmd *cobra.Command) (*logcapture.Capture, error) {
	debug, _ := cmd.Flags().GetBool("debug")
	logFile, _ := cmd.Flags().GetString("log-file")

	opts := logcapture.Options{
		Debug: debug || logFile != "",
	}

	// Determine log file path
	if logFile != "" {
		opts.LogFile = logFile
	} else if debug {
		opts.LogFile = logcapture.DefaultLogFile()
	}

	return logcapture.New(opts), nil
}

// initLogCaptureForCLI creates log capture for CLI commands (only if logging enabled).
// Returns nil if logging is not enabled.
func initLogCaptureForCLI(cmd *cobra.Command) *logcapture.Capture {
	debug, _ := cmd.Flags().GetBool("debug")
	logFile, _ := cmd.Flags().GetString("log-file")

	// Return nil if logging is not enabled
	if !debug && logFile == "" {
		return nil
	}

	opts := logcapture.Options{
		Debug: true,
	}

	// Determine log file path
	if logFile != "" {
		opts.LogFile = logFile
	} else {
		opts.LogFile = logcapture.DefaultLogFile()
	}

	return logcapture.New(opts)
}
