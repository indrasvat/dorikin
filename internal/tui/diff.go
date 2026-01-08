package tui

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/indrasvat/dorikin/internal/tui/styles"
	"github.com/indrasvat/dorikin/pkg/api"
)

// DiffRenderer renders diffs in various formats.
type DiffRenderer struct {
	styles *styles.App
	width  int
}

// NewDiffRenderer creates a new DiffRenderer.
func NewDiffRenderer(s *styles.App, width int) *DiffRenderer {
	return &DiffRenderer{
		styles: s,
		width:  width,
	}
}

// DetectRenderMode determines the optimal render mode for a diff.
func DetectRenderMode(expected, actual any) api.RenderMode {
	// Nil checks → structural add/del
	if mode, handled := detectNilMode(expected, actual); handled {
		return mode
	}

	// Type mismatch → replacement
	if reflect.TypeOf(expected) != reflect.TypeOf(actual) {
		return api.RenderModeReplacement
	}

	// String comparison
	if expStr, actStr, ok := extractStrings(expected, actual); ok {
		return detectStringMode(expStr, actStr)
	}

	// Maps and slices - check structural similarity
	if isComplexObject(expected) && isComplexObject(actual) {
		if objectSimilarity(expected, actual) < 0.25 {
			return api.RenderModeReplacement
		}
	}

	return api.RenderModeSideBySide
}

// detectNilMode handles nil value scenarios.
func detectNilMode(expected, actual any) (api.RenderMode, bool) {
	if expected == nil && actual != nil {
		return api.RenderModeStructuralAdd, true
	}
	if expected != nil && actual == nil {
		return api.RenderModeStructuralDel, true
	}
	if expected == nil && actual == nil {
		return api.RenderModeSideBySide, true
	}
	return api.RenderModeSideBySide, false
}

// extractStrings attempts to extract strings from both values.
func extractStrings(expected, actual any) (expStr, actStr string, ok bool) {
	expStr, expIsStr := expected.(string)
	actStr, actIsStr := actual.(string)
	return expStr, actStr, expIsStr && actIsStr
}

// detectStringMode determines render mode for string comparisons.
func detectStringMode(expStr, actStr string) api.RenderMode {
	expLines := strings.Count(expStr, "\n")
	actLines := strings.Count(actStr, "\n")

	// Multi-line or long strings → unified diff
	if expLines > 2 || actLines > 2 || len(expStr) > 80 || len(actStr) > 80 {
		if stringSimilarity(expStr, actStr) < 0.25 {
			return api.RenderModeReplacement
		}
		return api.RenderModeUnifiedDiff
	}
	return api.RenderModeSideBySide
}

// RenderUnifiedDiff renders a unified diff for multi-line text changes.
func (r *DiffRenderer) RenderUnifiedDiff(expected, actual string, contextLines int) string {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(expected),
		B:        difflib.SplitLines(actual),
		FromFile: "Manifest",
		ToFile:   "Cluster",
		Context:  contextLines,
	}

	result, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return r.styles.Error.Render("Error generating diff: " + err.Error())
	}

	return r.colorizeDiff(result)
}

// colorizeDiff applies lipgloss styles to unified diff output.
func (r *DiffRenderer) colorizeDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	var styled []string

	for _, line := range lines {
		if line == "" {
			styled = append(styled, "")
			continue
		}

		switch {
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			// File headers - skip (we show our own header)
			continue
		case strings.HasPrefix(line, "@@"):
			// Hunk header
			styled = append(styled, r.styles.DiffHunk.Render(line))
		case strings.HasPrefix(line, "-"):
			// Removed line
			styled = append(styled, r.styles.DiffRemove.Render(line))
		case strings.HasPrefix(line, "+"):
			// Added line
			styled = append(styled, r.styles.DiffAdd.Render(line))
		default:
			// Context line
			styled = append(styled, r.styles.DiffContext.Render(line))
		}
	}

	return strings.Join(styled, "\n")
}

// DiffStats returns addition/deletion counts for a unified diff.
func DiffStats(expected, actual string) (added, removed int) {
	diff := difflib.UnifiedDiff{
		A:       difflib.SplitLines(expected),
		B:       difflib.SplitLines(actual),
		Context: 0, // No context for counting
	}

	result, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return 0, 0
	}

	for _, line := range strings.Split(result, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			removed++
		}
	}

	return added, removed
}

// stringSimilarity calculates similarity ratio between two strings.
// Returns 0.0 (completely different) to 1.0 (identical).
func stringSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}

	// Use sequence matcher for similarity
	matcher := difflib.NewMatcher(
		difflib.SplitLines(a),
		difflib.SplitLines(b),
	)
	return matcher.Ratio()
}

// objectSimilarity calculates similarity between two objects.
// Uses a simple key-based comparison for maps.
func objectSimilarity(a, b any) float64 {
	aMap, aIsMap := a.(map[string]any)
	bMap, bIsMap := b.(map[string]any)

	if aIsMap && bIsMap {
		return mapSimilarity(aMap, bMap)
	}

	aSlice, aIsSlice := a.([]any)
	bSlice, bIsSlice := b.([]any)

	if aIsSlice && bIsSlice {
		return sliceSimilarity(aSlice, bSlice)
	}

	// For other types, compare as strings
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return stringSimilarity(aStr, bStr)
}

// mapSimilarity calculates similarity between two maps.
func mapSimilarity(a, b map[string]any) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}

	// Count common keys
	allKeys := make(map[string]bool)
	for k := range a {
		allKeys[k] = true
	}
	for k := range b {
		allKeys[k] = true
	}

	commonKeys := 0
	for k := range allKeys {
		_, inA := a[k]
		_, inB := b[k]
		if inA && inB {
			commonKeys++
		}
	}

	return float64(commonKeys) / float64(len(allKeys))
}

// sliceSimilarity calculates similarity between two slices.
func sliceSimilarity(a, b []any) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}

	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}

	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	return float64(minLen) / float64(maxLen)
}

// isComplexObject returns true if the value is a map or slice.
func isComplexObject(v any) bool {
	if v == nil {
		return false
	}
	switch v.(type) {
	case map[string]any, []any:
		return true
	default:
		return false
	}
}

// FormatValue formats a value for display, with optional truncation.
func FormatValue(v any, maxLen int) string {
	if v == nil {
		return "<nil>"
	}

	str := fmt.Sprintf("%v", v)

	// For maps and slices, try to format more nicely
	switch val := v.(type) {
	case map[string]any:
		if len(val) == 0 {
			return "{}"
		}
		str = fmt.Sprintf("{...%d keys}", len(val))
	case []any:
		if len(val) == 0 {
			return "[]"
		}
		str = fmt.Sprintf("[...%d items]", len(val))
	}

	if maxLen > 0 && len(str) > maxLen {
		return str[:maxLen-3] + "..."
	}

	return str
}
