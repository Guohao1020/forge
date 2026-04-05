package project

import (
	"testing"
)

func TestDetectProjectType(t *testing.T) {
	tests := []struct {
		name         string
		files        []string
		languages    map[string]int
		wantType     string
		wantSubType  string
		wantStrategy string
	}{
		{
			name:         "Go API with cmd directory",
			files:        []string{"go.mod", "cmd/server/main.go", "internal/handler/user.go", "Dockerfile"},
			languages:    map[string]int{"Go": 10000},
			wantType:     "backend_api",
			wantSubType:  "go_api",
			wantStrategy: "trunk_based",
		},
		{
			name:         "Next.js web app",
			files:        []string{"package.json", "next.config.js", "src/app/page.tsx", "tailwind.config.js"},
			languages:    map[string]int{"TypeScript": 8000, "JavaScript": 2000},
			wantType:     "web_app",
			wantSubType:  "nextjs",
			wantStrategy: "trunk_based",
		},
		{
			name:         "Flutter mobile app",
			files:        []string{"pubspec.yaml", "lib/main.dart", "android/app/build.gradle", "ios/Runner.xcworkspace"},
			languages:    map[string]int{"Dart": 15000},
			wantType:     "mobile_app",
			wantSubType:  "flutter",
			wantStrategy: "release_train",
		},
		{
			name:         "Tauri desktop app",
			files:        []string{"package.json", "src-tauri/tauri.conf.json", "src/App.tsx"},
			languages:    map[string]int{"TypeScript": 5000, "Rust": 3000},
			wantType:     "desktop_app",
			wantSubType:  "tauri",
			wantStrategy: "github_flow",
		},
		{
			name:         "Go library (no cmd dir)",
			files:        []string{"go.mod", "parser.go", "parser_test.go"},
			languages:    map[string]int{"Go": 5000},
			wantType:     "library",
			wantSubType:  "go_module",
			wantStrategy: "github_flow",
		},
		{
			name:         "Turborepo monorepo",
			files:        []string{"turbo.json", "package.json", "packages/web/package.json", "packages/api/package.json"},
			languages:    map[string]int{"TypeScript": 20000},
			wantType:     "monorepo",
			wantSubType:  "turborepo",
			wantStrategy: "trunk_based",
		},
		{
			name:         "Nuxt web app",
			files:        []string{"package.json", "nuxt.config.ts", "pages/index.vue"},
			languages:    map[string]int{"Vue": 8000, "TypeScript": 3000},
			wantType:     "web_app",
			wantSubType:  "nuxt",
			wantStrategy: "trunk_based",
		},
		{
			name:         "Java Spring Boot",
			files:        []string{"pom.xml", "src/main/java/com/example/App.java", "Dockerfile"},
			languages:    map[string]int{"Java": 20000},
			wantType:     "backend_api",
			wantSubType:  "spring_boot",
			wantStrategy: "trunk_based",
		},
		{
			name:         "React Native mobile",
			files:        []string{"package.json", "android/app/build.gradle", "ios/Podfile", "App.tsx"},
			languages:    map[string]int{"TypeScript": 10000, "Java": 500, "Objective-C": 300},
			wantType:     "mobile_app",
			wantSubType:  "react_native",
			wantStrategy: "release_train",
		},
		{
			name:         "Python API",
			files:        []string{"pyproject.toml", "app/main.py", "app/routes/user.py"},
			languages:    map[string]int{"Python": 8000},
			wantType:     "backend_api",
			wantSubType:  "python_api",
			wantStrategy: "trunk_based",
		},
		{
			name:         "Unknown project",
			files:        []string{"README.md", "LICENSE"},
			languages:    map[string]int{},
			wantType:     "unknown",
			wantSubType:  "unknown",
			wantStrategy: "trunk_based",
		},
		{
			name:         "Empty file list",
			files:        []string{},
			languages:    nil,
			wantType:     "unknown",
			wantSubType:  "unknown",
			wantStrategy: "trunk_based",
		},
		{
			name:         "Only README",
			files:        []string{"README.md", "LICENSE", ".gitignore"},
			languages:    map[string]int{},
			wantType:     "unknown",
			wantSubType:  "unknown",
			wantStrategy: "trunk_based",
		},
		{
			name:         "Windows backslash paths normalized",
			files:        []string{"go.mod", "cmd\\server\\main.go", "internal\\handler\\api.go"},
			languages:    map[string]int{"Go": 5000},
			wantType:     "backend_api",
			wantSubType:  "go_api",
			wantStrategy: "trunk_based",
		},
		{
			name:         "Nil languages map with Go files",
			files:        []string{"go.mod", "cmd/main.go", "internal/handler.go"},
			languages:    nil,
			wantType:     "backend_api",
			wantSubType:  "go_api",
			wantStrategy: "trunk_based",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := DetectProjectType(tt.files, tt.languages)

			if profile.ProjectType != tt.wantType {
				t.Errorf("ProjectType = %q, want %q", profile.ProjectType, tt.wantType)
			}
			if profile.SubType != tt.wantSubType {
				t.Errorf("SubType = %q, want %q", profile.SubType, tt.wantSubType)
			}
			if profile.BranchStrategy != tt.wantStrategy {
				t.Errorf("BranchStrategy = %q, want %q", profile.BranchStrategy, tt.wantStrategy)
			}
		})
	}
}
