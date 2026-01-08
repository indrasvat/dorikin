package drift

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/dorikin/pkg/api"
)

func testScanResult() *api.ScanResult {
	return &api.ScanResult{
		Duration: 1234 * time.Millisecond,
		Summary: api.ScanSummary{
			TotalResources: 5,
			InSync:         2,
			Drifted:        1,
			Missing:        1,
			Extra:          0,
			Errors:         1,
		},
		Reports: []api.DriftReport{
			{
				Resource: api.ResourceRef{
					Kind:      "Deployment",
					Name:      "nginx",
					Namespace: "default",
				},
				Status: api.StatusDrifted,
				Diffs: []api.FieldDiff{
					{
						Path:     ".spec.replicas",
						Expected: float64(3),
						Actual:   float64(5),
						Type:     api.DiffTypeModified,
					},
				},
			},
			{
				Resource: api.ResourceRef{
					Kind:      "ConfigMap",
					Name:      "app-config",
					Namespace: "default",
				},
				Status: api.StatusInSync,
			},
			{
				Resource: api.ResourceRef{
					Kind:      "Service",
					Name:      "web",
					Namespace: "default",
				},
				Status: api.StatusInSync,
			},
			{
				Resource: api.ResourceRef{
					Kind:      "Secret",
					Name:      "db-creds",
					Namespace: "default",
				},
				Status: api.StatusMissing,
			},
			{
				Resource: api.ResourceRef{
					Kind:      "Deployment",
					Name:      "broken",
					Namespace: "default",
				},
				Status: api.StatusError,
				Error:  "failed to fetch: connection refused",
			},
		},
	}
}

func testScanResultNoIssues() *api.ScanResult {
	return &api.ScanResult{
		Duration: 500 * time.Millisecond,
		Summary: api.ScanSummary{
			TotalResources: 3,
			InSync:         3,
			Drifted:        0,
			Missing:        0,
			Extra:          0,
			Errors:         0,
		},
		Reports: []api.DriftReport{
			{
				Resource: api.ResourceRef{Kind: "Deployment", Name: "app1", Namespace: "default"},
				Status:   api.StatusInSync,
			},
			{
				Resource: api.ResourceRef{Kind: "Service", Name: "app1-svc", Namespace: "default"},
				Status:   api.StatusInSync,
			},
			{
				Resource: api.ResourceRef{Kind: "ConfigMap", Name: "app1-config", Namespace: "default"},
				Status:   api.StatusInSync,
			},
		},
	}
}

func TestReportJSON(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf, api.OutputFormatJSON)
	result := testScanResult()

	err := r.Report(result)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	// Verify it's valid JSON
	var parsed api.ScanResult
	err = json.Unmarshal(buf.Bytes(), &parsed)
	if err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}

	// Verify key fields are present
	if parsed.Summary.TotalResources != 5 {
		t.Errorf("TotalResources = %d, want 5", parsed.Summary.TotalResources)
	}
	if len(parsed.Reports) != 5 {
		t.Errorf("len(Reports) = %d, want 5", len(parsed.Reports))
	}
}

func TestReportJSON_Formatting(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf, api.OutputFormatJSON)
	result := testScanResult()

	err := r.Report(result)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	output := buf.String()
	// Should be pretty-printed (contains newlines and indentation)
	if !strings.Contains(output, "\n") {
		t.Error("JSON output should be pretty-printed with newlines")
	}
	if !strings.Contains(output, "  ") {
		t.Error("JSON output should have indentation")
	}
}

func TestReportYAML(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf, api.OutputFormatYAML)
	result := testScanResult()

	err := r.Report(result)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	output := buf.String()

	// Verify it looks like YAML (contains key: patterns)
	if !strings.Contains(output, "summary:") {
		t.Error("YAML output missing 'summary:' key")
	}
	if !strings.Contains(output, "reports:") {
		t.Error("YAML output missing 'reports:' key")
	}
	if !strings.Contains(output, "totalResources:") {
		t.Error("YAML output missing 'totalResources:' key")
	}
}

func TestReportQuiet_WithIssues(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf, api.OutputFormatQuiet)
	result := testScanResult()

	err := r.Report(result)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "DRIFT_DETECTED" {
		t.Errorf("output = %q, want %q", output, "DRIFT_DETECTED")
	}
}

func TestReportQuiet_NoIssues(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf, api.OutputFormatQuiet)
	result := testScanResultNoIssues()

	err := r.Report(result)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "IN_SYNC" {
		t.Errorf("output = %q, want %q", output, "IN_SYNC")
	}
}

func TestReportTable_WithIssues(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf, api.OutputFormatTable)
	result := testScanResult()

	err := r.Report(result)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	output := buf.String()

	// Check header elements
	if !strings.Contains(output, "dorikin scan results") {
		t.Error("missing header")
	}

	// Check summary stats
	if !strings.Contains(output, "Resources: 5") {
		t.Error("missing resource count")
	}
	if !strings.Contains(output, "2 in sync") {
		t.Error("missing in sync count")
	}
	if !strings.Contains(output, "1 drifted") {
		t.Error("missing drifted count")
	}

	// Check status sections
	if !strings.Contains(output, "DRIFTED") {
		t.Error("missing DRIFTED section")
	}
	if !strings.Contains(output, "MISSING") {
		t.Error("missing MISSING section")
	}
	if !strings.Contains(output, "ERROR") {
		t.Error("missing ERROR section")
	}

	// Check resource details
	if !strings.Contains(output, "Deployment/nginx") {
		t.Error("missing drifted deployment")
	}
	if !strings.Contains(output, ".spec.replicas") {
		t.Error("missing drift path")
	}

	// Check final status
	if !strings.Contains(output, "DRIFT DETECTED") {
		t.Error("missing drift detected message")
	}
}

func TestReportTable_NoIssues(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf, api.OutputFormatTable)
	result := testScanResultNoIssues()

	err := r.Report(result)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	output := buf.String()

	// Check header
	if !strings.Contains(output, "dorikin scan results") {
		t.Error("missing header")
	}

	// Should have Tsuchiya approval message
	if !strings.Contains(output, "Tsuchiya approves") {
		t.Error("missing approval message for in-sync result")
	}

	// Should NOT have drift sections
	if strings.Contains(output, "DRIFTED") {
		t.Error("should not have DRIFTED section")
	}
	if strings.Contains(output, "MISSING") {
		t.Error("should not have MISSING section")
	}
}

func TestReportTable_ShowsErrorMessage(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf, api.OutputFormatTable)
	result := testScanResult()

	err := r.Report(result)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "connection refused") {
		t.Error("error message not shown in output")
	}
}

func TestReportTable_DefaultFormat(t *testing.T) {
	var buf bytes.Buffer
	// Empty format should default to table
	r := NewReporter(&buf, "")
	result := testScanResultNoIssues()

	err := r.Report(result)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	output := buf.String()
	// Should behave like table format
	if !strings.Contains(output, "dorikin scan results") {
		t.Error("default format should be table")
	}
}

func TestFormatValue_Truncation(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			name:  "short string",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "exactly 50 chars",
			input: strings.Repeat("a", 50),
			want:  strings.Repeat("a", 50),
		},
		{
			name:  "51 chars truncated",
			input: strings.Repeat("a", 51),
			want:  strings.Repeat("a", 47) + "...",
		},
		{
			name:  "long string",
			input: strings.Repeat("x", 100),
			want:  strings.Repeat("x", 47) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatValue(tt.input)
			if got != tt.want {
				t.Errorf("formatValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatValue_Multiline(t *testing.T) {
	input := "line1\nline2\nline3"
	got := formatValue(input)
	want := "line1\\nline2\\nline3"

	if got != want {
		t.Errorf("formatValue() = %q, want %q", got, want)
	}
}

func TestFormatValue_Nil(t *testing.T) {
	got := formatValue(nil)
	want := "<nil>"

	if got != want {
		t.Errorf("formatValue(nil) = %q, want %q", got, want)
	}
}

func TestFormatValue_Types(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"int", 42, "42"},
		{"float", 3.14, "3.14"},
		{"bool", true, "true"},
		{"map", map[string]any{"k": "v"}, "map[k:v]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatValue(tt.input)
			if got != tt.want {
				t.Errorf("formatValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewReporter(t *testing.T) {
	var buf bytes.Buffer
	r := NewReporter(&buf, api.OutputFormatJSON)

	if r == nil {
		t.Fatal("NewReporter() returned nil")
	}
	if r.writer != &buf {
		t.Error("writer not set correctly")
	}
	if r.format != api.OutputFormatJSON {
		t.Errorf("format = %q, want %q", r.format, api.OutputFormatJSON)
	}
}
