package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dorikin/internal/drift"
	"github.com/indrasvat/dorikin/internal/k8s"
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
)

func init() {
	rootCmd.AddCommand(scanCmd)

	scanCmd.Flags().StringArrayVarP(&scanManifests, "file", "f", nil, "manifest file or directory (can be repeated)")
	scanCmd.Flags().StringVarP(&scanNamespace, "namespace", "n", "", "filter by namespace")
	scanCmd.Flags().BoolVarP(&scanRecursive, "recursive", "R", true, "recursively scan directories")
	scanCmd.Flags().StringVarP(&scanOutput, "output", "o", "table", "output format: table, json, yaml, quiet")
	scanCmd.Flags().StringSliceVar(&scanIgnore, "ignore", defaultIgnorePaths(), "field paths to ignore")
}

func runScan(cmd *cobra.Command, args []string) error {
	// Combine -f flags and positional args
	paths := make([]string, 0, len(scanManifests)+len(args))
	paths = append(paths, scanManifests...)
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
	detector := drift.NewDetector(client, scanIgnore)

	// Build scan options
	opts := api.ScanOptions{
		ManifestPaths: paths,
		Namespace:     scanNamespace,
		Recursive:     scanRecursive,
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

// defaultIgnorePaths returns the default paths to ignore during comparison.
func defaultIgnorePaths() []string {
	return []string{
		// Metadata fields
		"metadata.resourceVersion",
		"metadata.uid",
		"metadata.generation",
		"metadata.creationTimestamp",
		"metadata.managedFields",
		"metadata.annotations.kubectl.kubernetes.io/last-applied-configuration",
		"metadata.annotations.deployment.kubernetes.io/revision",
		"metadata.selfLink",

		// Status (always server-side)
		"status",

		// Deployment defaults
		"spec.progressDeadlineSeconds",
		"spec.revisionHistoryLimit",
		"spec.strategy",
		"spec.template.metadata.creationTimestamp",

		// Pod spec defaults
		"spec.template.spec.dnsPolicy",
		"spec.template.spec.restartPolicy",
		"spec.template.spec.schedulerName",
		"spec.template.spec.terminationGracePeriodSeconds",
		"spec.template.spec.securityContext",

		// Container defaults
		"spec.template.spec.containers.imagePullPolicy",
		"spec.template.spec.containers.terminationMessagePath",
		"spec.template.spec.containers.terminationMessagePolicy",
		"spec.template.spec.containers.ports.protocol",
		"spec.template.spec.containers.livenessProbe.failureThreshold",
		"spec.template.spec.containers.livenessProbe.successThreshold",
		"spec.template.spec.containers.livenessProbe.timeoutSeconds",
		"spec.template.spec.containers.livenessProbe.httpGet.scheme",
		"spec.template.spec.containers.readinessProbe.failureThreshold",
		"spec.template.spec.containers.readinessProbe.successThreshold",
		"spec.template.spec.containers.readinessProbe.timeoutSeconds",
		"spec.template.spec.containers.readinessProbe.httpGet.scheme",

		// Service defaults
		"spec.clusterIP",
		"spec.clusterIPs",
		"spec.internalTrafficPolicy",
		"spec.ipFamilies",
		"spec.ipFamilyPolicy",
		"spec.sessionAffinity",
		"spec.ports.protocol",
	}
}
