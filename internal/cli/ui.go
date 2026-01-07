package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dorikin/internal/config"
	"github.com/indrasvat/dorikin/internal/drift"
	"github.com/indrasvat/dorikin/internal/k8s"
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
	uiManifests       []string
	uiNamespace       string
	uiRecursive       bool
	uiIgnore          []string
	uiRefreshInterval int
	uiHPAAware        string
)

func init() {
	rootCmd.AddCommand(uiCmd)

	uiCmd.Flags().StringArrayVarP(&uiManifests, "file", "f", nil, "manifest file or directory (can be repeated)")
	uiCmd.Flags().StringVarP(&uiNamespace, "namespace", "n", "", "filter by namespace")
	uiCmd.Flags().BoolVarP(&uiRecursive, "recursive", "R", true, "recursively scan directories")
	uiCmd.Flags().StringSliceVar(&uiIgnore, "ignore", nil, "field paths to ignore (overrides config file)")
	uiCmd.Flags().IntVar(&uiRefreshInterval, "refresh-interval", 5, "auto-refresh interval in seconds (press 'a' to toggle)")
	uiCmd.Flags().StringVar(&uiHPAAware, "hpa-aware", "", "HPA awareness mode: manifests (default), cluster, disabled")
}

func runUI(cmd *cobra.Command, args []string) error {
	// Combine -f flags and positional args
	paths := make([]string, 0, len(uiManifests)+len(args))
	paths = append(paths, uiManifests...)
	paths = append(paths, args...)
	if len(paths) == 0 {
		return fmt.Errorf("at least one manifest path required (use -f or positional args)")
	}

	// Load configuration from .dorikin.yaml (if present)
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Determine effective ignore paths: CLI flag overrides config
	ignorePaths := cfg.EffectiveIgnorePaths()
	if cmd.Flags().Changed("ignore") {
		ignorePaths = uiIgnore
	}

	// Determine effective HPA mode: CLI flag overrides config
	hpaModeStr := cfg.EffectiveHPAAware()
	if cmd.Flags().Changed("hpa-aware") {
		hpaModeStr = uiHPAAware
	}

	// Get kubeconfig options
	kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
	kubeContext, _ := cmd.Flags().GetString("context")

	// Create Kubernetes client
	client, err := k8s.NewClient(k8s.ClientOptions{
		KubeConfig: kubeconfig,
		Context:    kubeContext,
	})
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Create detector with config
	detector := drift.NewDetectorWithConfig(client, cfg, ignorePaths)

	// Parse HPA awareness mode
	hpaMode, err := parseHPAAwareMode(hpaModeStr)
	if err != nil {
		return err
	}

	// Build scan options
	opts := api.ScanOptions{
		ManifestPaths: paths,
		Namespace:     uiNamespace,
		Recursive:     uiRecursive,
		HPAAware:      hpaMode,
	}

	// Create scan function for refresh
	scanFunc := func() (*api.ScanResult, error) {
		return detector.Scan(cmd.Context(), opts)
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
