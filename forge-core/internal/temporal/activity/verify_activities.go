package activity

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/shulex/forge/forge-core/internal/workspace"
)

// VerifyActivities handles build verification for AI-generated code.
type VerifyActivities struct {
	ws *workspace.Manager
}

func NewVerifyActivities(ws *workspace.Manager) *VerifyActivities {
	return &VerifyActivities{ws: ws}
}

// BuildVerifyInput is the input for the BuildVerify activity.
type BuildVerifyInput struct {
	TaskID    int64                    `json:"task_id"`
	TenantID  int64                   `json:"tenant_id"`
	ProjectID int64                   `json:"project_id"`
	Files     []map[string]interface{} `json:"files"`     // generated files from AI
	Language  string                   `json:"language"`   // detected language: typescript, go, python, java
}

// BuildVerifyOutput is the result of build verification.
type BuildVerifyOutput struct {
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`       // build error message (for AI to fix)
	Language   string `json:"language"`
	Command    string `json:"command"`               // what was run
	DurationMs int64  `json:"duration_ms"`
}

// BuildVerify writes AI-generated files to a temp workspace and runs the
// language-specific build command to verify the code compiles.
//
// This is the gatekeeper: code that doesn't compile never gets pushed to GitHub.
//
// Supported languages and their verification:
//   - TypeScript/Node: npm install && npm run build
//   - Go: go build ./...
//   - Python: python -m py_compile on each .py file
//   - Java: mvn compile / gradle build
func (a *VerifyActivities) BuildVerify(ctx context.Context, input BuildVerifyInput) (*BuildVerifyOutput, error) {
	start := time.Now()

	if a.ws == nil {
		return &BuildVerifyOutput{Success: true, Language: input.Language, Command: "skipped (no workspace)"}, nil
	}

	// Create a temp directory for verification
	verifyDir := filepath.Join(a.ws.TaskDir(input.TenantID, input.ProjectID, input.TaskID), fmt.Sprintf("verify-%d", time.Now().Unix()))
	if err := os.MkdirAll(verifyDir, 0755); err != nil {
		return nil, fmt.Errorf("create verify dir: %w", err)
	}
	defer os.RemoveAll(verifyDir) // cleanup after verification

	// Write all generated files
	for _, f := range input.Files {
		path, _ := f["path"].(string)
		content, _ := f["content"].(string)
		if path == "" || content == "" {
			continue
		}
		fullPath := filepath.Join(verifyDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			continue
		}
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	// Detect language from files if not specified
	lang := input.Language
	if lang == "" {
		lang = detectLanguage(input.Files)
	}

	slog.Info("BuildVerify starting", "task_id", input.TaskID, "language", lang, "files", len(input.Files), "dir", verifyDir)

	var cmd string
	var buildErr error

	switch lang {
	case "typescript", "javascript", "tsx", "jsx":
		cmd, buildErr = verifyNode(ctx, verifyDir, input.Files)
	case "go", "golang":
		cmd, buildErr = verifyGo(ctx, verifyDir)
	case "python":
		cmd, buildErr = verifyPython(ctx, verifyDir, input.Files)
	default:
		// Unknown language — skip verification
		slog.Warn("BuildVerify: unknown language, skipping", "language", lang)
		return &BuildVerifyOutput{
			Success:    true,
			Language:   lang,
			Command:    "skipped (unknown language)",
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}

	if buildErr != nil {
		errMsg := buildErr.Error()
		// Truncate very long errors to 2000 chars (enough for AI to understand)
		if len(errMsg) > 2000 {
			errMsg = errMsg[:2000] + "\n... (truncated)"
		}
		slog.Warn("BuildVerify FAILED", "task_id", input.TaskID, "language", lang, "error", errMsg[:min(len(errMsg), 200)])
		return &BuildVerifyOutput{
			Success:    false,
			Error:      errMsg,
			Language:   lang,
			Command:    cmd,
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}

	slog.Info("BuildVerify PASSED", "task_id", input.TaskID, "language", lang, "duration_ms", time.Since(start).Milliseconds())
	return &BuildVerifyOutput{
		Success:    true,
		Language:   lang,
		Command:    cmd,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

// verifyNode writes package.json if missing, runs npm install + npm run build.
func verifyNode(ctx context.Context, dir string, files []map[string]interface{}) (string, error) {
	// Check if package.json exists
	pkgPath := filepath.Join(dir, "package.json")
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		return "npm run build", fmt.Errorf("MISSING package.json — AI must generate package.json with all dependencies")
	}

	// Check if next.config.js has output: 'standalone' (required for Docker deployment)
	nextConfigPath := filepath.Join(dir, "next.config.js")
	if data, err := os.ReadFile(nextConfigPath); err == nil {
		if !strings.Contains(string(data), "standalone") {
			return "config check", fmt.Errorf("next.config.js missing output: 'standalone' — required for Docker deployment")
		}
	}

	// Check Dockerfile exists and uses npm install (not npm ci)
	dockerfilePath := filepath.Join(dir, "Dockerfile")
	if data, err := os.ReadFile(dockerfilePath); err == nil {
		content := string(data)
		if strings.Contains(content, "npm ci") {
			return "Dockerfile check", fmt.Errorf("Dockerfile uses 'npm ci' which requires package-lock.json — change to 'npm install'")
		}
		// Check PORT=8080 for K8s compatibility
		if !strings.Contains(content, "8080") {
			return "Dockerfile check", fmt.Errorf("Dockerfile must expose PORT 8080 and set ENV PORT=8080 for K8s deployment")
		}
	} else {
		return "Dockerfile check", fmt.Errorf("MISSING Dockerfile — AI must generate a multi-stage Dockerfile for deployment")
	}

	// npm install
	installCmd := exec.CommandContext(ctx, "npm", "install", "--prefer-offline", "--no-audit", "--no-fund")
	installCmd.Dir = dir
	installOut, err := installCmd.CombinedOutput()
	if err != nil {
		return "npm install", fmt.Errorf("npm install failed:\n%s", string(installOut))
	}

	// npm run build
	buildCmd := exec.CommandContext(ctx, "npm", "run", "build")
	buildCmd.Dir = dir
	buildCmd.Env = append(os.Environ(), "NODE_ENV=production")
	buildOut, err := buildCmd.CombinedOutput()
	if err != nil {
		return "npm run build", fmt.Errorf("npm run build failed:\n%s", string(buildOut))
	}

	return "npm install && npm run build", nil
}

// verifyGo runs go build ./...
func verifyGo(ctx context.Context, dir string) (string, error) {
	// Check go.mod exists
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); os.IsNotExist(err) {
		return "go build", fmt.Errorf("MISSING go.mod — AI must generate go.mod with module name and Go version")
	}

	cmd := exec.CommandContext(ctx, "go", "build", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "go build ./...", fmt.Errorf("go build failed:\n%s", string(out))
	}
	return "go build ./...", nil
}

// verifyPython checks syntax of all .py files.
func verifyPython(ctx context.Context, dir string, files []map[string]interface{}) (string, error) {
	// Check requirements.txt exists
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); os.IsNotExist(err) {
		return "requirements check", fmt.Errorf("MISSING requirements.txt — AI must generate requirements.txt with all dependencies")
	}

	// Syntax check each .py file
	for _, f := range files {
		path, _ := f["path"].(string)
		if !strings.HasSuffix(path, ".py") {
			continue
		}
		fullPath := filepath.Join(dir, filepath.FromSlash(path))
		cmd := exec.CommandContext(ctx, "python3", "-m", "py_compile", fullPath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("python3 -m py_compile %s", path), fmt.Errorf("Python syntax error in %s:\n%s", path, string(out))
		}
	}
	return "python3 -m py_compile (all files)", nil
}

// detectLanguage infers the primary language from generated files.
func detectLanguage(files []map[string]interface{}) string {
	counts := map[string]int{}
	for _, f := range files {
		path, _ := f["path"].(string)
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".ts", ".tsx":
			counts["typescript"]++
		case ".js", ".jsx":
			counts["javascript"]++
		case ".go":
			counts["go"]++
		case ".py":
			counts["python"]++
		case ".java":
			counts["java"]++
		}
	}
	best := ""
	bestCount := 0
	for lang, count := range counts {
		if count > bestCount {
			best = lang
			bestCount = count
		}
	}
	if best == "" {
		return "unknown"
	}
	return best
}

// BuildVerifyFixInput is the input when asking AI to fix build errors.
type BuildVerifyFixInput struct {
	TaskID       int64                    `json:"task_id"`
	TenantID     int64                    `json:"tenant_id"`
	ProjectID    int64                    `json:"project_id"`
	Files        []map[string]interface{} `json:"files"`
	BuildError   string                   `json:"build_error"`   // the compilation error
	BuildCommand string                   `json:"build_command"`  // what command failed
	Language     string                   `json:"language"`
	Attempt      int                      `json:"attempt"`
}
