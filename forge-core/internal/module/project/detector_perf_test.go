package project

import "testing"

func BenchmarkDetectProjectType_Small(b *testing.B) {
	files := []string{"go.mod", "cmd/main.go", "internal/handler.go"}
	langs := map[string]int{"Go": 5000}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DetectProjectType(files, langs)
	}
}

func BenchmarkParseOwnerRepo(b *testing.B) {
	url := "https://github.com/shulex/forge.git"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseOwnerRepo(url)
	}
}

func BenchmarkBuiltInTemplates(b *testing.B) {
	for i := 0; i < b.N; i++ {
		BuiltInTemplates()
	}
}
