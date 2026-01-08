package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

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
	// Suppress klog output to prevent logs bleeding into TUI
	klog.SetOutput(io.Discard)

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
	result, err := scanFunc()
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Launch TUI with refresh capability and configurable interval
	interval := time.Duration(uiRefreshInterval) * time.Second
	return tui.RunWithInterval(result, scanFunc, interval)
}
