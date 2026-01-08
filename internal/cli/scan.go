package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/indrasvat/dorikin/internal/drift"
	"github.com/indrasvat/dorikin/internal/logcapture"
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
	// Initialize log capture for file logging (if enabled)
	capture := initLogCaptureForCLI(cmd)
	if capture != nil {
		logcapture.SetGlobal(capture)
		logcapture.Info("Starting dorikin scan")
	}

	// Build scan context from common flags
	ctx, err := BuildScanContext(cmd, args, &scanFlags)
	if err != nil {
		stopCapture(capture)
		return err
	}

	// Run scan
	if capture != nil {
		logcapture.Debug("Scanning %d manifest paths", len(ctx.Options.ManifestPaths))
	}
	result, err := ctx.Detector.Scan(cmd.Context(), ctx.Options)
	if err != nil {
		if capture != nil {
			logcapture.Error("Scan failed: %v", err)
		}
		stopCapture(capture)
		return fmt.Errorf("scan failed: %w", err)
	}

	// Log results summary
	if capture != nil {
		s := result.Summary
		if s.HasIssues() {
			logcapture.Warn("Scan complete: %d drifted, %d missing, %d extra, %d errors",
				s.Drifted, s.Missing, s.Extra, s.Errors)
		} else {
			logcapture.Info("Scan complete: %d resources in sync", s.InSync)
		}
	}

	// Output results
	format := api.OutputFormat(scanOutput)
	reporter := drift.NewReporter(os.Stdout, format)
	if err := reporter.Report(result); err != nil {
		stopCapture(capture)
		return fmt.Errorf("failed to report results: %w", err)
	}

	// Stop capture before exit
	stopCapture(capture)

	// Exit with error code if drift detected
	if result.Summary.HasIssues() {
		os.Exit(1)
	}

	return nil
}

// stopCapture safely stops the capture if it exists.
func stopCapture(capture *logcapture.Capture) {
	if capture != nil {
		capture.Stop()
	}
}
