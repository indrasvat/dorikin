package cli

import (
	"fmt"

	"github.com/spf13/cobra"

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
	uiManifests []string
	uiNamespace string
	uiRecursive bool
	uiIgnore    []string
)

func init() {
	rootCmd.AddCommand(uiCmd)

	uiCmd.Flags().StringArrayVarP(&uiManifests, "file", "f", nil, "manifest file or directory (can be repeated)")
	uiCmd.Flags().StringVarP(&uiNamespace, "namespace", "n", "", "filter by namespace")
	uiCmd.Flags().BoolVarP(&uiRecursive, "recursive", "R", true, "recursively scan directories")
	uiCmd.Flags().StringSliceVar(&uiIgnore, "ignore", defaultIgnorePaths(), "field paths to ignore")
}

func runUI(cmd *cobra.Command, args []string) error {
	// Combine -f flags and positional args
	paths := make([]string, 0, len(uiManifests)+len(args))
	paths = append(paths, uiManifests...)
	paths = append(paths, args...)
	if len(paths) == 0 {
		return fmt.Errorf("at least one manifest path required (use -f or positional args)")
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

	// Create detector
	detector := drift.NewDetector(client, uiIgnore)

	// Build scan options
	opts := api.ScanOptions{
		ManifestPaths: paths,
		Namespace:     uiNamespace,
		Recursive:     uiRecursive,
	}

	// Run scan
	result, err := detector.Scan(cmd.Context(), opts)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Launch TUI
	return tui.Run(result)
}
