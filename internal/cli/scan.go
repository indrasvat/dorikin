package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dorikin/internal/drift"
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
	scanFlags  ScanFlags
	scanOutput string
)

func init() {
	rootCmd.AddCommand(scanCmd)

	RegisterScanFlags(scanCmd, &scanFlags)
	scanCmd.Flags().StringVarP(&scanOutput, "output", "o", "table", "output format: table, json, yaml, quiet")
}

func runScan(cmd *cobra.Command, args []string) error {
	// Build scan context from common flags
	ctx, err := BuildScanContext(cmd, args, &scanFlags)
	if err != nil {
		return err
	}

	// Run scan
	result, err := ctx.Detector.Scan(cmd.Context(), ctx.Options)
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
