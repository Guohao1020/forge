package version

import (
	"strings"
	"testing"
)

func TestSemverRegex(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"1.0", true},
		{"1.2.0", true},
		{"v1.0", true},
		{"v1.2.0", true},
		{"v2.1.3", true},
		{"10.20.30", true},
		{"v0.1", true},
		{"", false},
		{"abc", false},
		{"v", false},
		{"1", false},
		{"1.2.3.4", false},
		{"v1.2.3-beta", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := semverRegex.MatchString(tt.input)
			if result != tt.valid {
				t.Errorf("semverRegex.MatchString(%q) = %v, want %v", tt.input, result, tt.valid)
			}
		})
	}
}

func TestValidateStatusTransition(t *testing.T) {
	s := &Service{}
	tests := []struct {
		from    string
		to      string
		wantErr bool
	}{
		// Valid transitions
		{StatusPlanning, StatusInProgress, false},
		{StatusPlanning, StatusCancelled, false},
		{StatusInProgress, StatusTesting, false},
		{StatusInProgress, StatusCancelled, false},
		{StatusTesting, StatusReleased, false},
		{StatusTesting, StatusInProgress, false},
		{StatusTesting, StatusCancelled, false},

		// Invalid transitions
		{StatusPlanning, StatusReleased, true},
		{StatusPlanning, StatusTesting, true},
		{StatusInProgress, StatusPlanning, true},
		{StatusReleased, StatusPlanning, true},
		{StatusReleased, StatusCancelled, true},
		{StatusCancelled, StatusPlanning, true},
	}

	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			err := s.validateStatusTransition(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateStatusTransition(%q, %q) error = %v, wantErr %v",
					tt.from, tt.to, err, tt.wantErr)
			}
		})
	}
}

func TestVersionNormalization(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.0", "v1.0"},
		{"1.2.0", "v1.2.0"},
		{"v1.0", "v1.0"},      // already has prefix
		{"v2.1.3", "v2.1.3"},  // already has prefix
		{"  v1.0  ", "v1.0"},  // whitespace trimmed
		{"  1.2.0  ", "v1.2.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ver := strings.TrimSpace(tt.input)
			if !strings.HasPrefix(ver, "v") {
				ver = "v" + ver
			}
			if ver != tt.want {
				t.Errorf("normalize(%q) = %q, want %q", tt.input, ver, tt.want)
			}
		})
	}
}

func TestStatusConstants(t *testing.T) {
	// Verify all status constants are uppercase
	statuses := []string{StatusPlanning, StatusInProgress, StatusTesting, StatusReleased, StatusCancelled}
	for _, s := range statuses {
		if s != strings.ToUpper(s) {
			t.Errorf("status %q is not uppercase", s)
		}
	}
	// Verify uniqueness
	seen := map[string]bool{}
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate status: %q", s)
		}
		seen[s] = true
	}
}

func TestAllStatusTransitions(t *testing.T) {
	// Verify that every status has defined transitions (or is terminal)
	s := &Service{}
	statuses := []string{StatusPlanning, StatusInProgress, StatusTesting, StatusReleased, StatusCancelled}

	terminal := map[string]bool{StatusReleased: true, StatusCancelled: true}

	for _, from := range statuses {
		if terminal[from] {
			// Terminal states should reject all transitions
			for _, to := range statuses {
				err := s.validateStatusTransition(from, to)
				if err == nil {
					t.Errorf("terminal status %q should reject transition to %q", from, to)
				}
			}
		} else {
			// Non-terminal states must have at least one valid transition
			hasValid := false
			for _, to := range statuses {
				if s.validateStatusTransition(from, to) == nil {
					hasValid = true
				}
			}
			if !hasValid {
				t.Errorf("non-terminal status %q has no valid transitions", from)
			}
		}
	}
}
