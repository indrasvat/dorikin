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
	capture, err := initLogCapture(cmd)
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

	// Run initial scan
	logcapture.Info("Running initial scan")
	result, err := scanFunc()
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}
	logcapture.Info("Initial scan complete: %d resources", len(result.Reports))

	// Launch TUI with refresh capability and configurable interval
	interval := time.Duration(uiRefreshInterval) * time.Second
	return tui.RunWithCapture(result, scanFunc, interval, capture)
}

// initLogCapture creates and configures the log capture system based on CLI flags.
func initLogCapture(cmd *cobra.Command) (*logcapture.Capture, error) {
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
