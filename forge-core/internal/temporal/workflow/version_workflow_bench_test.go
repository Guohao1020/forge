package workflow

import (
	"fmt"
	"testing"
)

func BenchmarkDetectConflicts_NoOverlap(b *testing.B) {
	active := make(map[int64]*taskState)
	for i := int64(1); i <= 10; i++ {
		active[i] = &taskState{
			TaskID:       i,
			Status:       "RUNNING",
			TouchedFiles: []string{fmt.Sprintf("module_%d/service.go", i), fmt.Sprintf("module_%d/handler.go", i)},
		}
	}
	newFiles := []string{"new_module/service.go", "new_module/handler.go"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detectConflicts(newFiles, active)
	}
}

func BenchmarkDetectConflicts_WithOverlap(b *testing.B) {
	active := make(map[int64]*taskState)
	for i := int64(1); i <= 10; i++ {
		active[i] = &taskState{
			TaskID:       i,
			Status:       "RUNNING",
			TouchedFiles: []string{fmt.Sprintf("module_%d/service.go", i), "shared/common.go"},
		}
	}
	newFiles := []string{"shared/common.go", "new/handler.go"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detectConflicts(newFiles, active)
	}
}

func BenchmarkDetectConflicts_50Tasks(b *testing.B) {
	active := make(map[int64]*taskState)
	for i := int64(1); i <= 50; i++ {
		files := make([]string, 5)
		for j := 0; j < 5; j++ {
			files[j] = fmt.Sprintf("pkg_%d/file_%d.go", i, j)
		}
		active[i] = &taskState{
			TaskID:       i,
			Status:       "RUNNING",
			TouchedFiles: files,
		}
	}
	newFiles := []string{"pkg_new/a.go", "pkg_new/b.go", "pkg_new/c.go"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detectConflicts(newFiles, active)
	}
}

func BenchmarkHasFileOverlap_Large(b *testing.B) {
	a := make([]string, 50)
	bFiles := make([]string, 50)
	for i := 0; i < 50; i++ {
		a[i] = fmt.Sprintf("module_a/file_%d.go", i)
		bFiles[i] = fmt.Sprintf("module_b/file_%d.go", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hasFileOverlap(a, bFiles)
	}
}
