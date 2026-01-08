// Package cli provides the command-line interface for dorikin.
package cli

import (
	"github.com/spf13/cobra"
)

// Version information set by ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
	GoVersion = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "dorikin",
	Short: "Kubernetes configuration drift detector",
	Long: `dorikin - Kubernetes Configuration Drift Detector

Catch your Kubernetes configs drifting before they Tokyo Drift into production chaos.

A beautiful TUI application that compares your desired Kubernetes manifests
against actual cluster state, highlighting configuration drift with style.`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringP("kubeconfig", "k", "", "path to kubeconfig file")
	rootCmd.PersistentFlags().StringP("context", "c", "", "kubernetes context to use")
	rootCmd.PersistentFlags().Bool("debug", false, "enable debug logging to file")
	rootCmd.PersistentFlags().String("log-file", "", "write logs to specified file (implies --debug)")
}
