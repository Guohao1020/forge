package project

import (
	"path/filepath"
	"strings"
)

// ProjectTypeProfile holds the detected project type and configuration.
type ProjectTypeProfile struct {
	ProjectType    string   `json:"projectType"`    // web_app, mobile_app, desktop_app, backend_api, library, monorepo
	SubType        string   `json:"subType"`        // nextjs, flutter, tauri, go_api, npm_lib, etc.
	Languages      []string `json:"languages"`      // ["Go", "TypeScript"]
	Frameworks     []string `json:"frameworks"`     // ["Gin", "Next.js"]
	BuildTools     []string `json:"buildTools"`     // ["Docker", "npm"]
	TestFrameworks []string `json:"testFrameworks"` // ["go test", "Jest"]
	DeployTarget   string   `json:"deployTarget"`   // kubernetes, serverless, app_store, desktop_release, registry
	ArtifactType   string   `json:"artifactType"`   // docker_image, apk_ipa, exe_dmg, npm_package
	BranchStrategy string   `json:"branchStrategy"` // trunk_based, github_flow, release_train
	Confidence     string   `json:"confidence"`     // high, medium, low
}

// DetectProjectType analyzes the file tree to determine project type and configuration.
func DetectProjectType(files []string, repoLanguages map[string]int) *ProjectTypeProfile {
	p := &ProjectTypeProfile{
		Confidence: "low",
	}

	// Build lookup sets (normalize to forward slashes for cross-platform compatibility)
	fileSet := make(map[string]bool)
	baseSet := make(map[string]bool)
	dirSet := make(map[string]bool)
	for _, f := range files {
		f = strings.ReplaceAll(f, "\\", "/")
		fileSet[f] = true
		baseSet[filepath.Base(f)] = true
		// Add all parent directories (e.g., "cmd/server/main.go" → "cmd/server", "cmd")
		dir := strings.ReplaceAll(filepath.Dir(f), "\\", "/")
		for dir != "." && dir != "" && dir != "/" {
			dirSet[dir] = true
			parent := strings.ReplaceAll(filepath.Dir(dir), "\\", "/")
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// Extract languages
	for lang := range repoLanguages {
		p.Languages = append(p.Languages, lang)
	}

	// --- Tier 1: Project category detection (most specific first) ---

	// Monorepo detection
	if baseSet["turbo.json"] || baseSet["nx.json"] || baseSet["pnpm-workspace.yaml"] || baseSet["lerna.json"] {
		p.ProjectType = "monorepo"
		p.SubType = detectMonorepoTool(baseSet)
		p.DeployTarget = "kubernetes"
		p.ArtifactType = "docker_image"
		p.BranchStrategy = "trunk_based"
		p.Confidence = "high"
		p.BuildTools = append(p.BuildTools, "Docker")
		return p
	}

	// Mobile: Flutter
	if baseSet["pubspec.yaml"] && (dirSet["android"] || dirSet["ios"] || dirSet["lib"]) {
		p.ProjectType = "mobile_app"
		p.SubType = "flutter"
		p.Frameworks = append(p.Frameworks, "Flutter")
		p.DeployTarget = "app_store"
		p.ArtifactType = "apk_ipa"
		p.BranchStrategy = "release_train"
		p.TestFrameworks = append(p.TestFrameworks, "flutter_test")
		p.BuildTools = append(p.BuildTools, "Fastlane")
		p.Confidence = "high"
		return p
	}

	// Mobile: React Native
	if baseSet["package.json"] && (dirSet["android"] || dirSet["ios"]) && !baseSet["tauri.conf.json"] {
		p.ProjectType = "mobile_app"
		p.SubType = "react_native"
		p.Frameworks = append(p.Frameworks, "React Native")
		p.DeployTarget = "app_store"
		p.ArtifactType = "apk_ipa"
		p.BranchStrategy = "release_train"
		p.TestFrameworks = append(p.TestFrameworks, "Jest", "Detox")
		p.BuildTools = append(p.BuildTools, "Fastlane", "npm")
		p.Confidence = "high"
		return p
	}

	// Desktop: Tauri
	if baseSet["tauri.conf.json"] || dirSet["src-tauri"] {
		p.ProjectType = "desktop_app"
		p.SubType = "tauri"
		p.Frameworks = append(p.Frameworks, "Tauri")
		p.DeployTarget = "desktop_release"
		p.ArtifactType = "exe_dmg"
		p.BranchStrategy = "github_flow"
		p.TestFrameworks = append(p.TestFrameworks, "Playwright")
		p.BuildTools = append(p.BuildTools, "Tauri Action")
		p.Confidence = "high"
		return p
	}

	// Desktop: Electron
	if baseSet["electron-builder.yml"] || baseSet["electron-builder.json5"] {
		p.ProjectType = "desktop_app"
		p.SubType = "electron"
		p.Frameworks = append(p.Frameworks, "Electron")
		p.DeployTarget = "desktop_release"
		p.ArtifactType = "exe_dmg"
		p.BranchStrategy = "github_flow"
		p.TestFrameworks = append(p.TestFrameworks, "Playwright")
		p.BuildTools = append(p.BuildTools, "electron-builder")
		p.Confidence = "high"
		return p
	}

	// Web: Next.js
	if matchAnyBase(baseSet, "next.config.js", "next.config.ts", "next.config.mjs") {
		p.ProjectType = "web_app"
		p.SubType = "nextjs"
		p.Frameworks = append(p.Frameworks, "Next.js", "React")
		p.DeployTarget = "kubernetes"
		p.ArtifactType = "docker_image"
		p.BranchStrategy = "trunk_based"
		p.TestFrameworks = append(p.TestFrameworks, "Jest", "Playwright")
		p.BuildTools = append(p.BuildTools, "Docker", "npm")
		p.Confidence = "high"
		return p
	}

	// Web: Nuxt
	if matchAnyBase(baseSet, "nuxt.config.js", "nuxt.config.ts") {
		p.ProjectType = "web_app"
		p.SubType = "nuxt"
		p.Frameworks = append(p.Frameworks, "Nuxt", "Vue")
		p.DeployTarget = "kubernetes"
		p.ArtifactType = "docker_image"
		p.BranchStrategy = "trunk_based"
		p.TestFrameworks = append(p.TestFrameworks, "Vitest", "Playwright")
		p.BuildTools = append(p.BuildTools, "Docker", "npm")
		p.Confidence = "high"
		return p
	}

	// Backend: Go API
	if baseSet["go.mod"] && dirSet["cmd"] {
		p.ProjectType = "backend_api"
		p.SubType = "go_api"
		p.Frameworks = append(p.Frameworks, "Go")
		// Detect Gin/Echo/Fiber from file content (heuristic from dir structure)
		if dirSet["internal"] {
			p.Frameworks = append(p.Frameworks, "Go Modular")
		}
		p.DeployTarget = "kubernetes"
		p.ArtifactType = "docker_image"
		p.BranchStrategy = "trunk_based"
		p.TestFrameworks = append(p.TestFrameworks, "go test")
		p.BuildTools = append(p.BuildTools, "Docker")
		p.Confidence = "high"
		return p
	}

	// Backend: Java Spring
	if baseSet["pom.xml"] && (dirSet["src/main/java"] || dirSet["src/main/kotlin"]) {
		p.ProjectType = "backend_api"
		p.SubType = "spring_boot"
		p.Frameworks = append(p.Frameworks, "Spring Boot")
		p.DeployTarget = "kubernetes"
		p.ArtifactType = "docker_image"
		p.BranchStrategy = "trunk_based"
		p.TestFrameworks = append(p.TestFrameworks, "JUnit 5")
		p.BuildTools = append(p.BuildTools, "Maven", "Docker")
		p.Confidence = "high"
		return p
	}

	// Backend: Python (FastAPI/Django/Flask)
	if baseSet["pyproject.toml"] || baseSet["requirements.txt"] {
		p.ProjectType = "backend_api"
		p.SubType = "python_api"
		p.Frameworks = append(p.Frameworks, "Python")
		p.DeployTarget = "kubernetes"
		p.ArtifactType = "docker_image"
		p.BranchStrategy = "trunk_based"
		p.TestFrameworks = append(p.TestFrameworks, "pytest")
		p.BuildTools = append(p.BuildTools, "Docker")
		p.Confidence = "medium"
		return p
	}

	// Library: Go module (no cmd dir)
	if baseSet["go.mod"] && !dirSet["cmd"] {
		p.ProjectType = "library"
		p.SubType = "go_module"
		p.Frameworks = append(p.Frameworks, "Go")
		p.DeployTarget = "registry"
		p.ArtifactType = "go_module"
		p.BranchStrategy = "github_flow"
		p.TestFrameworks = append(p.TestFrameworks, "go test")
		p.BuildTools = append(p.BuildTools, "GoReleaser")
		p.Confidence = "medium"
		return p
	}

	// Library: npm package
	if baseSet["package.json"] && !matchAnyBase(baseSet, "next.config.js", "next.config.ts", "nuxt.config.ts") {
		p.ProjectType = "web_app" // default; could be library
		p.SubType = "node"
		p.Frameworks = append(p.Frameworks, "Node.js")
		p.DeployTarget = "kubernetes"
		p.ArtifactType = "docker_image"
		p.BranchStrategy = "trunk_based"
		p.TestFrameworks = append(p.TestFrameworks, "Jest")
		p.BuildTools = append(p.BuildTools, "npm", "Docker")
		p.Confidence = "low"
		return p
	}

	// Fallback
	p.ProjectType = "unknown"
	p.SubType = "unknown"
	p.DeployTarget = "kubernetes"
	p.ArtifactType = "docker_image"
	p.BranchStrategy = "trunk_based"
	p.Confidence = "low"
	return p
}

func matchAnyBase(baseSet map[string]bool, names ...string) bool {
	for _, n := range names {
		if baseSet[n] {
			return true
		}
	}
	return false
}

func detectMonorepoTool(baseSet map[string]bool) string {
	switch {
	case baseSet["turbo.json"]:
		return "turborepo"
	case baseSet["nx.json"]:
		return "nx"
	case baseSet["pnpm-workspace.yaml"]:
		return "pnpm_workspace"
	case baseSet["lerna.json"]:
		return "lerna"
	default:
		return "monorepo"
	}
}

// enhanceTechStack extends the existing DetectTechStack result with project type profile.
func enhanceTechStack(existingStack map[string]interface{}, profile *ProjectTypeProfile) map[string]interface{} {
	existingStack["projectType"] = profile.ProjectType
	existingStack["subType"] = profile.SubType
	existingStack["deployTarget"] = profile.DeployTarget
	existingStack["artifactType"] = profile.ArtifactType
	existingStack["branchStrategy"] = profile.BranchStrategy
	existingStack["testFrameworks"] = profile.TestFrameworks
	existingStack["buildTools"] = profile.BuildTools
	existingStack["confidence"] = profile.Confidence
	// Merge frameworks (keep existing + add detected)
	if existing, ok := existingStack["frameworks"].([]string); ok {
		merged := make(map[string]bool)
		for _, f := range existing {
			merged[strings.ToLower(f)] = true
		}
		for _, f := range profile.Frameworks {
			if !merged[strings.ToLower(f)] {
				existing = append(existing, f)
			}
		}
		existingStack["frameworks"] = existing
	} else {
		existingStack["frameworks"] = profile.Frameworks
	}
	return existingStack
}
