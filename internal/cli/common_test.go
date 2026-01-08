package cli

import (
	"testing"

	"github.com/indrasvat/dorikin/internal/loader"
	"github.com/indrasvat/dorikin/pkg/api"
)

func TestParseHPAAwareMode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    api.HPAAwareMode
		wantErr bool
	}{
		{
			name:    "empty defaults to manifests",
			input:   "",
			want:    api.HPAAwareModeManifests,
			wantErr: false,
		},
		{
			name:    "explicit manifests",
			input:   "manifests",
			want:    api.HPAAwareModeManifests,
			wantErr: false,
		},
		{
			name:    "cluster mode",
			input:   "cluster",
			want:    api.HPAAwareModeCluster,
			wantErr: false,
		},
		{
			name:    "disabled mode",
			input:   "disabled",
			want:    api.HPAAwareModeDisabled,
			wantErr: false,
		},
		{
			name:    "invalid mode",
			input:   "invalid",
			want:    "",
			wantErr: true,
		},
		{
			name:    "typo in mode",
			input:   "manifest", // missing 's'
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHPAAwareMode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseHPAAwareMode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseHPAAwareMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildLoader(t *testing.T) {
	tests := []struct {
		name        string
		helm        bool
		kustomize   bool
		helmRelease string
		namespace   string
		helmValues  []string
		helmSet     []string
		wantType    string
		wantErr     bool
	}{
		{
			name:     "default returns file loader",
			wantType: "*loader.FileLoader",
			wantErr:  false,
		},
		{
			name:        "helm flag returns helm loader",
			helm:        true,
			helmRelease: "myrelease",
			wantType:    "*loader.HelmLoader",
			wantErr:     false,
		},
		{
			name:      "kustomize flag returns kustomize loader",
			kustomize: true,
			wantType:  "*loader.KustomizeLoader",
			wantErr:   false,
		},
		{
			name:      "helm and kustomize are mutually exclusive",
			helm:      true,
			kustomize: true,
			wantErr:   true,
		},
		{
			name:        "helm with namespace",
			helm:        true,
			helmRelease: "release",
			namespace:   "production",
			wantType:    "*loader.HelmLoader",
			wantErr:     false,
		},
		{
			name:        "helm with values files",
			helm:        true,
			helmRelease: "release",
			helmValues:  []string{"values.yaml", "values-prod.yaml"},
			wantType:    "*loader.HelmLoader",
			wantErr:     false,
		},
		{
			name:        "helm with set values",
			helm:        true,
			helmRelease: "release",
			helmSet:     []string{"image.tag=v1.0.0", "replicas=3"},
			wantType:    "*loader.HelmLoader",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildLoader(tt.helm, tt.kustomize, tt.helmRelease, tt.namespace, tt.helmValues, tt.helmSet)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildLoader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			// Check loader type
			var gotType string
			switch got.(type) {
			case *loader.FileLoader:
				gotType = "*loader.FileLoader"
			case *loader.HelmLoader:
				gotType = "*loader.HelmLoader"
			case *loader.KustomizeLoader:
				gotType = "*loader.KustomizeLoader"
			default:
				gotType = "unknown"
			}

			if gotType != tt.wantType {
				t.Errorf("BuildLoader() returned %v, want %v", gotType, tt.wantType)
			}
		})
	}
}

func TestBuildLoader_MutualExclusion(t *testing.T) {
	_, err := BuildLoader(true, true, "release", "", nil, nil)
	if err == nil {
		t.Error("BuildLoader() should return error when both helm and kustomize are true")
	}

	expectedMsg := "--helm and --kustomize are mutually exclusive"
	if err.Error() != expectedMsg {
		t.Errorf("BuildLoader() error = %q, want %q", err.Error(), expectedMsg)
	}
}
