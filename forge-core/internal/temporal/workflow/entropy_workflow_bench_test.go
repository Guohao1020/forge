package workflow

import (
	"testing"

	"github.com/shulex/forge/forge-core/internal/temporal/activity"
)

func BenchmarkComputeQualityScore_10Issues(b *testing.B) {
	issues := make([]activity.EntropyIssue, 10)
	for i := range issues {
		switch i % 4 {
		case 0:
			issues[i] = activity.EntropyIssue{Severity: "critical"}
		case 1:
			issues[i] = activity.EntropyIssue{Severity: "error"}
		case 2:
			issues[i] = activity.EntropyIssue{Severity: "warning"}
		case 3:
			issues[i] = activity.EntropyIssue{Severity: "info"}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		computeQualityScore(issues)
	}
}

func BenchmarkComputeQualityScore_100Issues(b *testing.B) {
	issues := make([]activity.EntropyIssue, 100)
	for i := range issues {
		issues[i] = activity.EntropyIssue{Severity: "warning"}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		computeQualityScore(issues)
	}
}
