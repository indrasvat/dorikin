package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dorikin/internal/config"
	"github.com/indrasvat/dorikin/internal/drift"
	"github.com/indrasvat/dorikin/internal/k8s"
	"github.com/indrasvat/dorikin/internal/loader"
	"github.com/indrasvat/dorikin/pkg/api"
)

// ScanFlags holds the common flags shared between scan and ui commands.
type ScanFlags struct {
	Manifests []string
	Namespace string
	Recursive bool
	Ignore    []string
	HPAAware  string

	// Helm flags
	Helm        bool
	HelmRelease string
	HelmValues  []string
	HelmSet     []string

	// Kustomize flags
	Kustomize bool
}

// RegisterScanFlags registers the common scan flags on a cobra command.
func RegisterScanFlags(cmd *cobra.Command, f *ScanFlags) {
	cmd.Flags().StringArrayVarP(&f.Manifests, "file", "f", nil, "manifest file or directory (can be repeated)")
	cmd.Flags().StringVarP(&f.Namespace, "namespace", "n", "", "filter by namespace")
	cmd.Flags().BoolVarP(&f.Recursive, "recursive", "R", true, "recursively scan directories")
	cmd.Flags().StringSliceVar(&f.Ignore, "ignore", nil, "field paths to ignore (overrides config file)")
	cmd.Flags().StringVar(&f.HPAAware, "hpa-aware", "", "HPA awareness mode: manifests (default), cluster, disabled")

	// Helm flags
	cmd.Flags().BoolVar(&f.Helm, "helm", false, "load manifests from Helm chart (run helm template)")
	cmd.Flags().StringVar(&f.HelmRelease, "helm-release", "release", "Helm release name for templating")
	cmd.Flags().StringSliceVar(&f.HelmValues, "helm-values", nil, "Helm values files (can be repeated)")
	cmd.Flags().StringSliceVar(&f.HelmSet, "helm-set", nil, "Helm --set values (can be repeated)")

	// Kustomize flags
	cmd.Flags().BoolVar(&f.Kustomize, "kustomize", false, "load manifests from Kustomize directory (run kustomize build)")
}

// ScanContext holds the initialized components needed for scanning.
type ScanContext struct {
	Detector *drift.Detector
	Options  api.ScanOptions
	Config   *config.Config
}

// BuildScanContext creates a ScanContext from flags and command args.
func BuildScanContext(cmd *cobra.Command, args []string, f *ScanFlags) (*ScanContext, error) {
	// Load configuration from .dorikin.yaml (if present)
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Combine -f flags and positional args
	paths := make([]string, 0, len(f.Manifests)+len(args))
	paths = append(paths, f.Manifests...)
	paths = append(paths, args...)

	// Fall back to config paths if no CLI paths provided
	if len(paths) == 0 {
		paths = cfg.EffectivePaths()
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no manifest paths (use -f, positional args, or paths in .dorikin.yaml)")
	}

	// Determine effective ignore paths: CLI flag overrides config
	ignorePaths := cfg.EffectiveIgnorePaths()
	if cmd.Flags().Changed("ignore") {
		ignorePaths = f.Ignore
	}

	// Determine effective HPA mode: CLI flag overrides config
	hpaModeStr := cfg.EffectiveHPAAware()
	if cmd.Flags().Changed("hpa-aware") {
		hpaModeStr = f.HPAAware
	}

	// Get kubeconfig options from persistent flags
	kubeconfig, _ := cmd.Flags().GetString("kubeconfig")
	kubeContext, _ := cmd.Flags().GetString("context")

	// Create Kubernetes client
	client, err := k8s.NewClient(k8s.ClientOptions{
		KubeConfig: kubeconfig,
		Context:    kubeContext,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Select loader based on flags
	ldr, err := BuildLoader(f.Helm, f.Kustomize, f.HelmRelease, f.Namespace, f.HelmValues, f.HelmSet)
	if err != nil {
		return nil, err
	}

	// Create detector with config and loader
	detector := drift.NewDetector(client, ignorePaths, drift.WithConfig(cfg), drift.WithLoader(ldr))

	// Parse HPA awareness mode
	hpaMode, err := ParseHPAAwareMode(hpaModeStr)
	if err != nil {
		return nil, err
	}

	// Build scan options
	opts := api.ScanOptions{
		ManifestPaths: paths,
		Namespace:     f.Namespace,
		Recursive:     f.Recursive,
		HPAAware:      hpaMode,
	}

	return &ScanContext{
		Detector: detector,
		Options:  opts,
		Config:   cfg,
	}, nil
}

// ParseHPAAwareMode parses the HPA awareness mode from string.
func ParseHPAAwareMode(mode string) (api.HPAAwareMode, error) {
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

// BuildLoader creates the appropriate loader based on flags.
func BuildLoader(helm, kustomize bool, helmRelease, namespace string, helmValues, helmSet []string) (loader.Loader, error) {
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
