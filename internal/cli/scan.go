package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dorikin/internal/config"
	"github.com/indrasvat/dorikin/internal/drift"
	"github.com/indrasvat/dorikin/internal/k8s"
	"github.com/indrasvat/dorikin/internal/loader"
	"github.com/indrasvat/dorikin/pkg/api"
)

var scanCmd = &cobra.Command{
	Use:   "scan [paths...]",
	Short: "Scan for configuration drift",
	Long: `Scan Kubernetes manifests against cluster state to detect drift.

Examples:
  # Scan a single file
  dorikin scan deployment.yaml

  # Scan a directory
  dorikin scan ./manifests/

  # Scan with -f flag
  dorikin scan -f ./manifests/

  # Scan multiple paths
  dorikin scan ./base/ ./overlays/production/

  # Scan with namespace filter
  dorikin scan -n default ./manifests/`,
	RunE: runScan,
}

var (
	scanManifests []string
	scanNamespace string
	scanRecursive bool
	scanOutput    string
	scanIgnore    []string
	scanHPAAware  string

	// Helm flags
	scanHelm        bool
	scanHelmRelease string
	scanHelmValues  []string
	scanHelmSet     []string

	// Kustomize flags
	scanKustomize bool
)

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringArrayVarP(&scanManifests, "file", "f", nil, "manifest file or directory (can be repeated)")
	scanCmd.Flags().StringVarP(&scanNamespace, "namespace", "n", "", "filter by namespace")
	scanCmd.Flags().BoolVarP(&scanRecursive, "recursive", "R", true, "recursively scan directories")
	scanCmd.Flags().StringVarP(&scanOutput, "output", "o", "table", "output format: table, json, yaml, quiet")
	scanCmd.Flags().StringSliceVar(&scanIgnore, "ignore", nil, "field paths to ignore (overrides config file)")
	scanCmd.Flags().StringVar(&scanHPAAware, "hpa-aware", "", "HPA awareness mode: manifests (default), cluster, disabled")

	// Helm flags
	scanCmd.Flags().BoolVar(&scanHelm, "helm", false, "load manifests from Helm chart (run helm template)")
	scanCmd.Flags().StringVar(&scanHelmRelease, "helm-release", "release", "Helm release name for templating")
	scanCmd.Flags().StringSliceVar(&scanHelmValues, "helm-values", nil, "Helm values files (can be repeated)")
	scanCmd.Flags().StringSliceVar(&scanHelmSet, "helm-set", nil, "Helm --set values (can be repeated)")

	// Kustomize flags
	scanCmd.Flags().BoolVar(&scanKustomize, "kustomize", false, "load manifests from Kustomize directory (run kustomize build)")
}

func runScan(cmd *cobra.Command, args []string) error {
	// Combine -f flags and positional args
	paths := make([]string, 0, len(scanManifests)+len(args))
	paths = append(paths, scanManifests...)
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
		ignorePaths = scanIgnore
	}

	// Determine effective HPA mode: CLI flag overrides config
	hpaModeStr := cfg.EffectiveHPAAware()
	if cmd.Flags().Changed("hpa-aware") {
		hpaModeStr = scanHPAAware
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

	// Select loader based on flags
	ldr, err := buildLoader(scanHelm, scanKustomize, scanHelmRelease, scanNamespace, scanHelmValues, scanHelmSet)
	if err != nil {
		return err
	}

	// Create detector with config and loader
	detector := drift.NewDetector(client, ignorePaths, drift.WithConfig(cfg), drift.WithLoader(ldr))

	// Parse HPA awareness mode
	hpaMode, err := parseHPAAwareMode(hpaModeStr)
	if err != nil {
		return err
	}

	// Build scan options
	opts := api.ScanOptions{
		ManifestPaths: paths,
		Namespace:     scanNamespace,
		Recursive:     scanRecursive,
		HPAAware:      hpaMode,
	}

	// Run scan
	result, err := detector.Scan(cmd.Context(), opts)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Output results
	format := api.OutputFormat(scanOutput)
	reporter := drift.NewReporter(os.Stdout, format)
	if err := reporter.Report(result); err != nil {
		return fmt.Errorf("failed to report results: %w", err)
	}

	// Exit with error code if drift detected
	if result.Summary.HasIssues() {
		os.Exit(1)
	}

	return nil
}

// parseHPAAwareMode parses the HPA awareness mode from string.
func parseHPAAwareMode(mode string) (api.HPAAwareMode, error) {
	switch mode {
	case "manifests", "":
		return api.HPAAwareModeManifests, nil
	case "cluster":
		return api.HPAAwareModeCluster, nil
	case "disabled":
		return api.HPAAwareModeDisabled, nil
	default:
		return "", fmt.Errorf("invalid --hpa-aware mode: %q (valid: manifests, cluster, disabled)", mode)
	}
}

// buildLoader creates the appropriate loader based on flags.
func buildLoader(helm, kustomize bool, helmRelease, namespace string, helmValues, helmSet []string) (loader.Loader, error) {
	// Validate mutually exclusive flags
	if helm && kustomize {
		return nil, fmt.Errorf("--helm and --kustomize are mutually exclusive")
	}

	switch {
	case helm:
		opts := []loader.HelmOption{
			loader.WithRelease(helmRelease),
		}
		if namespace != "" {
			opts = append(opts, loader.WithNamespace(namespace))
		}
		if len(helmValues) > 0 {
			opts = append(opts, loader.WithValues(helmValues))
		}
		if len(helmSet) > 0 {
			opts = append(opts, loader.WithSet(helmSet))
		}
		return loader.NewHelmLoader(opts...), nil

	case kustomize:
		return loader.NewKustomizeLoader(), nil

	default:
		return loader.NewFileLoader(), nil
	}
}
