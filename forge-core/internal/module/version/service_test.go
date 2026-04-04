package version

import (
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
