package project

import (
	"testing"
)

func BenchmarkDetectProjectType_GoAPI(b *testing.B) {
	files := []string{
		"go.mod", "go.sum", "cmd/server/main.go", "internal/handler/user.go",
		"internal/service/user.go", "internal/repository/user.go",
		"Dockerfile", "docker-compose.yml", "Makefile", "README.md",
		"internal/middleware/auth.go", "internal/model/user.go",
		"migrations/001_init.sql", "configs/config.yaml",
	}
	languages := map[string]int{"Go": 15000}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DetectProjectType(files, languages)
	}
}

func BenchmarkDetectProjectType_NextJS(b *testing.B) {
	files := []string{
		"package.json", "next.config.js", "tsconfig.json", "tailwind.config.js",
		"src/app/page.tsx", "src/app/layout.tsx", "src/components/header.tsx",
		"public/favicon.ico", "postcss.config.js",
	}
	languages := map[string]int{"TypeScript": 10000, "JavaScript": 2000}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DetectProjectType(files, languages)
	}
}

func BenchmarkDetectProjectType_LargeRepo(b *testing.B) {
	// Simulate a large monorepo with 500 files
	files := make([]string, 500)
	for i := 0; i < 500; i++ {
		files[i] = "src/module" + string(rune('a'+i%26)) + "/file" + string(rune('0'+i%10)) + ".go"
	}
	files[0] = "go.mod"
	files[1] = "cmd/main.go"
	languages := map[string]int{"Go": 100000}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DetectProjectType(files, languages)
	}
}

