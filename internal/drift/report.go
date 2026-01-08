package drift

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/indrasvat/dorikin/pkg/api"
)

// Reporter formats and outputs scan results.
type Reporter struct {
	writer io.Writer
	format api.OutputFormat
}

// NewReporter creates a new Reporter.
func NewReporter(w io.Writer, format api.OutputFormat) *Reporter {
	return &Reporter{
		writer: w,
		format: format,
	}
}

// Report outputs the scan result in the configured format.
func (r *Reporter) Report(result *api.ScanResult) error {
	switch r.format {
	case api.OutputFormatJSON:
		return r.reportJSON(result)
	case api.OutputFormatYAML:
		return r.reportYAML(result)
	case api.OutputFormatQuiet:
		return r.reportQuiet(result)
	default:
		return r.reportTable(result)
	}
}

// reportJSON outputs results as JSON.
func (r *Reporter) reportJSON(result *api.ScanResult) error {
	encoder := json.NewEncoder(r.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// reportYAML outputs results as YAML.
func (r *Reporter) reportYAML(result *api.ScanResult) error {
	data, err := yaml.Marshal(result)
	if err != nil {
		return err
	}
	_, err = r.writer.Write(data)
	return err
}

// reportQuiet outputs only exit status info.
func (r *Reporter) reportQuiet(result *api.ScanResult) error {
	if result.Summary.HasIssues() {
		fmt.Fprintf(r.writer, "DRIFT_DETECTED\n")
	} else {
		fmt.Fprintf(r.writer, "IN_SYNC\n")
	}
	return nil
}

// reportTable outputs results as a human-readable table.
func (r *Reporter) reportTable(result *api.ScanResult) error {
	// Header
	fmt.Fprintf(r.writer, "\n")
	fmt.Fprintf(r.writer, "  🏎️  dorikin scan results\n")
	fmt.Fprintf(r.writer, "  ────────────────────────────────────────\n")

	// Summary line
	fmt.Fprintf(r.writer, "  Resources: %d | Duration: %s\n",
		result.Summary.TotalResources,
		result.Duration.Round(1000000).String())

	fmt.Fprintf(r.writer, "  🏁 %d in sync | 🚨 %d drifted | 🛑 %d missing | ➕ %d extra | 💥 %d errors\n\n",
		result.Summary.InSync,
		result.Summary.Drifted,
		result.Summary.Missing,
		result.Summary.Extra,
		result.Summary.Errors)

	// Group by status
	if result.Summary.HasIssues() {
		r.printResourcesByStatus(result.Reports, api.StatusDrifted, "🚨 DRIFTED")
		r.printResourcesByStatus(result.Reports, api.StatusMissing, "🛑 MISSING")
		r.printResourcesByStatus(result.Reports, api.StatusExtra, "➕ EXTRA")
		r.printResourcesByStatus(result.Reports, api.StatusError, "💥 ERROR")
	}

	// Final status
	fmt.Fprintf(r.writer, "  ────────────────────────────────────────\n")
	if result.Summary.HasIssues() {
		fmt.Fprintf(r.writer, "  ⚡ DRIFT DETECTED ⚡\n")
	} else {
		fmt.Fprintf(r.writer, "  🏁 Tsuchiya approves! All resources in sync.\n")
	}
	fmt.Fprintf(r.writer, "\n")

	return nil
}

// printResourcesByStatus prints resources with a specific status.
func (r *Reporter) printResourcesByStatus(reports []api.DriftReport, status api.DriftStatus, header string) {
	var matchingIdxs []int
	for i := range reports {
		if reports[i].Status == status {
			matchingIdxs = append(matchingIdxs, i)
		}
	}

	if len(matchingIdxs) == 0 {
		return
	}

	fmt.Fprintf(r.writer, "  %s (%d)\n", header, len(matchingIdxs))
	for _, idx := range matchingIdxs {
		fmt.Fprintf(r.writer, "    • %s\n", reports[idx].Resource.String())

		// Show diffs for drifted resources
		if status == api.StatusDrifted && len(reports[idx].Diffs) > 0 {
			for j := range reports[idx].Diffs {
				r.printDiff(reports[idx].Diffs[j])
			}
		}

		// Show error for error status
		if status == api.StatusError && reports[idx].Error != "" {
			fmt.Fprintf(r.writer, "      Error: %s\n", reports[idx].Error)
		}
	}
	fmt.Fprintf(r.writer, "\n")
}

// printDiff prints a single field diff.
func (r *Reporter) printDiff(diff api.FieldDiff) {
	switch diff.Type {
	case api.DiffTypeModified:
		fmt.Fprintf(r.writer, "      %s: %v → %v\n",
			diff.Path, formatValue(diff.Expected), formatValue(diff.Actual))
	case api.DiffTypeAdded:
		fmt.Fprintf(r.writer, "      %s: (added) %v\n",
			diff.Path, formatValue(diff.Actual))
	case api.DiffTypeRemoved:
		fmt.Fprintf(r.writer, "      %s: (removed) %v\n",
			diff.Path, formatValue(diff.Expected))
	}
}

// formatValue formats a value for display.
func formatValue(v any) string {
	if v == nil {
		return "<nil>"
	}
	s := fmt.Sprintf("%v", v)
	if len(s) > 50 {
		return s[:47] + "..."
	}
	// Handle multi-line or complex values
	if strings.Contains(s, "\n") {
		return strings.ReplaceAll(s, "\n", "\\n")
	}
	return s
}
