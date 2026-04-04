package activity

import (
	"testing"
)

func TestSanitizeK8sName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"my-project", "my-project"},
		{"My Project", "my-project"},
		{"user_service", "user-service"},
		{"Project 123!", "project-123"},
		{"forge-core", "forge-core"},
		{"Test--Double", "test--double"},
		{"", ""},
		{"UPPER", "upper"},
		// Test length limit (63 chars max for K8s)
		{"a-very-long-project-name-that-exceeds-the-kubernetes-dns-label-maximum-of-sixty-three-characters", "a-very-long-project-name-that-exceeds-the-kubernetes-dns-label-"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeK8sName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeK8sName(%q) = %q, want %q", tt.input, got, tt.want)
			}
			// Verify length constraint
			if len(got) > 63 {
				t.Errorf("sanitizeK8sName(%q) length = %d, exceeds 63", tt.input, len(got))
			}
		})
	}
}

func TestGenerateManifestInput_Defaults(t *testing.T) {
	input := GenerateManifestInput{
		ProjectName: "test",
		Environment: "dev",
	}

	// Default port should be 0 (set in activity)
	if input.Port != 0 {
		t.Errorf("default Port = %d, want 0", input.Port)
	}
	if input.Replicas != 0 {
		t.Errorf("default Replicas = %d, want 0", input.Replicas)
	}
}
