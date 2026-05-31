# S11' -- Code Generation Enhanced Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade CoderAgent with full context tool integration (reads existing code, queries schema, checks business rules), add inline Lint checking after generation with auto-fix loop, and enable parallel sub-task code generation for independent task nodes.

**Architecture:** CoderAgent uses all 5 project profile dimensions to generate context-aware code. After generation, an inline Lint step runs golangci-lint (Go) or eslint (JS/TS) on the output. If Lint fails, errors are fed back to CoderAgent for a single auto-fix attempt. For tasks with multiple independent sub-nodes (depends_on=[]), parallel CoderAgent instances run concurrently via Temporal, each scoped to relevant files. Results are merged and consistency-checked before Review.

**Tech Stack:** Python 3.12 (CoderAgent + parallel orchestration), Go 1.22 (workflow enhancement), golangci-lint / eslint (Lint tools), Docker (Lint execution environment)

**Dependencies:** S10' (test-first with approval), S9' (task nodes with touched_files), S16 (project profiles)

**Duration:** 2 days

---

## File Structure

### Python AI Worker

```
ai-worker/src/
+-- agents/coder.py                 # MODIFY: inject all 5 profile dimensions + lint feedback loop
+-- agents/lint_checker.py          # NEW: lightweight agent to fix lint errors
+-- activities/generate.py          # MODIFY: add lint step + parallel generation orchestration
+-- activities/lint.py              # NEW: lint execution activity
```

### Go Backend

```
forge-core/
+-- internal/temporal/
|   +-- workflow/task_workflow.go    # MODIFY: support parallel generate + lint loop
|   +-- activity/task_activities.go # MODIFY: add RunLint activity
```

### Frontend

```
forge-portal/
+-- components/tasks/
|   +-- lint-results-card.tsx       # NEW: lint results display
|   +-- task-workspace.tsx          # MODIFY: show lint results in GENERATE step
```

---

## Day 1: CoderAgent with Full Context Tools + Lint Integration

### Task 1: CoderAgent -- Full Profile Context Injection

**Files:**
- Modify: `ai-worker/src/agents/coder.py`
- Modify: `ai-worker/src/activities/generate.py`

**IMPORTANT**: Read `ai-worker/src/agents/coder.py`, `ai-worker/src/activities/generate.py`, `ai-worker/src/context/builder.py` first.

- [ ] **Step 1: Enhance CoderAgent with profile-aware prompting**

In `ai-worker/src/agents/coder.py`, update `_build_system_prompt` to leverage all profile dimensions:

```python
def _build_system_prompt(self, context: ProjectContext) -> str:
    base = CODER_SYSTEM_PROMPT
    project_context = context.to_system_prompt()
    if project_context:
        base += f"\n\n{project_context}"

    # Additional profile-specific guidance
    if context.project_profiles:
        # Business rules: explicit constraints
        rules = context.project_profiles.get("business_rules", {})
        if rules and rules.get("rules"):
            base += "\n\n## Business Rules (MUST follow)\n"
            for rule in rules["rules"][:10]:
                base += f"- [{rule.get('domain', '?')}] {rule.get('rule', '')}\n"

        # Coding habits: match existing style
        habits = context.project_profiles.get("coding_habits", {})
        if habits:
            base += f"\n\n## Existing Code Patterns (match this style)\n{json.dumps(habits, ensure_ascii=False, indent=2)}\n"

    return base
```

Add to CODER_SYSTEM_PROMPT:
```python
CODER_SYSTEM_PROMPT = """You are a senior software engineer. Your task is to generate production-ready code based on the task plan and coding standards.

## Critical Rules
1. STRICTLY follow the coding standards provided below
2. Generate complete, compilable code (no placeholders or TODOs)
3. Include proper error handling
4. Include necessary imports
5. Follow existing project patterns from the project profiles
6. Match naming conventions of existing code (check API catalog and module graph)
7. Reference correct table/column names from DB schema
8. Do NOT duplicate existing API endpoints
9. Follow business rules listed in project context

## Context Tool Usage
When project profiles are available, you MUST:
- Check api_catalog before creating new endpoints (avoid path conflicts)
- Check db_schema for correct column types and constraints
- Check module_graph to import from correct packages
- Check architecture to follow established patterns (e.g., Repository pattern, DI)
- Check business_rules for domain-specific constraints

## Lint Compliance
Your code MUST pass lint checks. Common issues to avoid:
- Go: unused imports, unused variables, missing error checks, shadowed variables
- JS/TS: missing semicolons (if enforced), unused variables, any-type usage
- General: consistent formatting, no magic numbers, proper naming

## Dockerfile Generation
... (existing dockerfile section unchanged) ...

## Output Format
IMPORTANT: You MUST respond with ONLY a JSON object. No explanations, no markdown.
{"files": [{"path": "...", "content": "...", "language": "...", "action": "create|modify"}], "summary": "...", "tests_addressed": [...]}
"""
```

- [ ] **Step 2: Enhanced generate activity with scoped context**

In `ai-worker/src/activities/generate.py`, update user prompt to include specific profile slices relevant to the current task node:

```python
# When generating for a specific task node, scope the context
if input.task_plan:
    current_node = input.task_plan  # single node being generated
    touched = current_node.get("touched_files", {})
    modify_files = touched.get("modify", [])

    # If modifying existing files, fetch their content for reference
    if modify_files and ctx.project_profiles:
        user_prompt += "\n## Files to Modify (current content)\n"
        # The actual file content should be fetched via the code browsing API
        # For now, include the file paths so AI knows the context
        for f in modify_files[:5]:
            user_prompt += f"- {f} (existing file, modify in place)\n"
```

- [ ] **Step 3: Verify imports**

```bash
cd ai-worker && python -c "from src.activities.generate import generate_code_activity; print('OK')"
```

- [ ] **Step 4: Commit**

```bash
git add ai-worker/src/agents/coder.py ai-worker/src/activities/generate.py
git commit -m "feat(s11'): enhance CoderAgent with full profile context and lint-aware prompting"
```

---

### Task 2: Lint Check Activity + Auto-Fix Loop

**Files:**
- Create: `ai-worker/src/agents/lint_checker.py`
- Create: `ai-worker/src/activities/lint.py`
- Modify: `ai-worker/src/worker.py`

- [ ] **Step 1: Create lint_checker agent**

`ai-worker/src/agents/lint_checker.py`:

```python
LINT_FIX_SYSTEM_PROMPT = """You are a code linter assistant. You are given code files and lint error messages. Your task is to fix the lint errors WITHOUT changing the business logic.

## Rules
1. ONLY fix the specific lint errors listed
2. Do NOT change business logic, add features, or refactor
3. Do NOT remove code unless it is truly unused (lint says so)
4. Preserve all comments and documentation
5. If a fix is ambiguous, choose the most conservative option

## Output Format
IMPORTANT: You MUST respond with ONLY a JSON object.
{"files": [{"path": "...", "content": "...", "language": "..."}], "fixes_applied": ["description of each fix"]}
"""

class LintCheckerAgent(BaseAgent):
    purpose = Purpose.GENERATE  # reuses GENERATE model routing

    def _build_system_prompt(self, context: ProjectContext) -> str:
        return LINT_FIX_SYSTEM_PROMPT
```

- [ ] **Step 2: Create lint execution activity**

`ai-worker/src/activities/lint.py`:

```python
@dataclass
class LintInput:
    task_id: int
    files: List[Dict[str, Any]]  # [{path, content, language}]
    project_type: str  # "go", "javascript", "typescript", "python"

@dataclass
class LintOutput:
    passed: bool
    errors: List[Dict[str, Any]]  # [{file, line, column, message, rule}]
    warnings: List[Dict[str, Any]]
    error_count: int
    warning_count: int

@activity.defn(name="run_lint_check")
async def run_lint_check_activity(input: LintInput) -> LintOutput:
    """
    Run lint on generated code files.

    Strategy:
    1. Write files to a temp directory
    2. Run language-appropriate linter:
       - Go: golangci-lint run (if available, else go vet)
       - JS/TS: eslint --no-eslintrc --rule '...' (basic rules)
       - Python: ruff check (if available)
    3. Parse output into structured errors
    4. Return results

    If linter binary not available, fall back to basic regex checks:
    - Unused imports (Go: imported but not used)
    - Missing error checks (Go: err declared but not checked)
    - Console.log left in (JS/TS)
    """
    import tempfile
    import subprocess
    import os

    with tempfile.TemporaryDirectory() as tmpdir:
        # Write files
        for f in input.files:
            fpath = os.path.join(tmpdir, f["path"])
            os.makedirs(os.path.dirname(fpath), exist_ok=True)
            with open(fpath, "w") as fp:
                fp.write(f["content"])

        errors = []
        warnings = []

        if input.project_type == "go":
            errors, warnings = await _run_go_lint(tmpdir)
        elif input.project_type in ("javascript", "typescript"):
            errors, warnings = await _run_js_lint(tmpdir)
        elif input.project_type == "python":
            errors, warnings = await _run_python_lint(tmpdir)

        return LintOutput(
            passed=len(errors) == 0,
            errors=errors,
            warnings=warnings,
            error_count=len(errors),
            warning_count=len(warnings),
        )
```

Helper functions for each linter:
```python
async def _run_go_lint(tmpdir: str) -> tuple[list, list]:
    """Run golangci-lint or fall back to go vet."""
    # Try golangci-lint first
    try:
        result = subprocess.run(
            ["golangci-lint", "run", "--out-format=json", "./..."],
            cwd=tmpdir, capture_output=True, text=True, timeout=60
        )
        if result.returncode != 0 and result.stdout:
            import json
            data = json.loads(result.stdout)
            issues = data.get("Issues", [])
            errors = [{"file": i["FromLinter"], "line": i["Pos"]["Line"],
                       "message": i["Text"], "rule": i["FromLinter"]} for i in issues]
            return errors, []
    except (FileNotFoundError, subprocess.TimeoutExpired):
        pass
    # Fallback: basic pattern matching
    return _basic_go_checks(tmpdir), []

async def _run_js_lint(tmpdir: str) -> tuple[list, list]:
    """Run eslint or basic checks."""
    # Similar pattern: try eslint, fallback to regex
    return _basic_js_checks(tmpdir), []

async def _run_python_lint(tmpdir: str) -> tuple[list, list]:
    """Run ruff or basic checks."""
    return _basic_python_checks(tmpdir), []
```

- [ ] **Step 3: Register activity in worker.py**

```python
# In worker.py activities list
from src.activities.lint import run_lint_check_activity
```

- [ ] **Step 4: Verify imports**

```bash
cd ai-worker && python -c "from src.activities.lint import run_lint_check_activity; print('OK')"
```

- [ ] **Step 5: Commit**

```bash
git add ai-worker/src/agents/lint_checker.py ai-worker/src/activities/lint.py ai-worker/src/worker.py
git commit -m "feat(s11'): add lint check activity with golangci-lint/eslint/ruff support"
```

---

### Task 3: Workflow Integration -- Lint Loop + Parallel Generation

**Files:**
- Modify: `forge-core/internal/temporal/workflow/task_workflow.go`

**IMPORTANT**: Read `task_workflow.go` fully to understand the current GENERATE step.

- [ ] **Step 1: Add lint-fix loop after GENERATE**

In `task_workflow.go`, after code generation completes, add a lint-fix cycle:

```go
// ---- GENERATE step ----
var generateResult map[string]interface{}
err = workflow.ExecuteActivity(aiCtx, "generate_code", generateInput).Get(ctx, &generateResult)
if err != nil {
    _ = workflow.ExecuteActivity(localCtx, "FailTask", input.TaskID, err.Error()).Get(ctx, nil)
    return err
}

// ---- Lint Check + Auto-Fix Loop (max 1 retry) ----
files, _ := generateResult["files"].([]interface{})
if len(files) > 0 {
    lintInput := map[string]interface{}{
        "task_id":      input.TaskID,
        "files":        files,
        "project_type": detectProjectType(input),  // "go", "javascript", etc.
    }

    var lintResult map[string]interface{}
    err = workflow.ExecuteActivity(aiCtx, "run_lint_check", lintInput).Get(ctx, &lintResult)

    if err == nil {
        passed, _ := lintResult["passed"].(bool)
        if !passed {
            errorCount, _ := lintResult["error_count"].(float64)
            slog.Info("lint check failed, attempting auto-fix",
                "task_id", input.TaskID, "errors", int(errorCount))

            // Auto-fix: feed lint errors back to LintCheckerAgent
            fixInput := map[string]interface{}{
                "task_id":     input.TaskID,
                "files":       files,
                "lint_errors": lintResult["errors"],
                "project_type": detectProjectType(input),
            }

            var fixResult map[string]interface{}
            err = workflow.ExecuteActivity(aiCtx, "fix_lint_errors", fixInput).Get(ctx, &fixResult)
            if err == nil {
                // Replace generated files with fixed versions
                if fixedFiles, ok := fixResult["files"].([]interface{}); ok && len(fixedFiles) > 0 {
                    generateResult["files"] = fixedFiles
                    generateResult["lint_fixed"] = true
                    generateResult["lint_fixes_applied"] = fixResult["fixes_applied"]
                }
            }
        } else {
            generateResult["lint_passed"] = true
        }
        generateResult["lint_result"] = lintResult
    }
}

_ = workflow.ExecuteActivity(localCtx, "SaveStepOutput", input.TaskID, "GENERATE", generateResult).Get(ctx, nil)
```

- [ ] **Step 2: Add parallel generation support**

For tasks where PlannerAgent marked multiple nodes as independent (depends_on=[]), launch parallel CoderAgent instances:

```go
// Check if plan has independent nodes that can run in parallel
independentNodes := getIndependentNodeGroups(planResult)

if len(independentNodes) > 1 && len(independentNodes) <= 4 {
    // Parallel generation: one CoderAgent per independent group
    var futures []workflow.Future
    for _, nodeGroup := range independentNodes {
        scopedInput := buildScopedGenerateInput(input, nodeGroup, planResult, testResult)
        future := workflow.ExecuteActivity(aiCtx, "generate_code", scopedInput)
        futures = append(futures, future)
    }

    // Collect results
    var allFiles []interface{}
    for _, future := range futures {
        var result map[string]interface{}
        if err := future.Get(ctx, &result); err != nil {
            slog.Warn("parallel generation failed for a group, falling back", "error", err)
            // Fallback to serial generation
            goto serialGeneration
        }
        if files, ok := result["files"].([]interface{}); ok {
            allFiles = append(allFiles, files...)
        }
    }

    // Consistency check: look for naming conflicts in merged output
    generateResult = map[string]interface{}{
        "files":    allFiles,
        "parallel": true,
        "groups":   len(independentNodes),
    }
} else {
serialGeneration:
    // Serial generation (existing path)
    // ... existing code ...
}
```

Helper:
```go
func getIndependentNodeGroups(plan map[string]interface{}) [][]map[string]interface{} {
    tasks, ok := plan["tasks"].([]interface{})
    if !ok {
        return nil
    }
    // Group nodes by dependency layer
    // Layer 0: nodes with no dependencies
    // For simplicity, just return nodes with empty depends_on as one group per node
    var independent [][]map[string]interface{}
    for _, t := range tasks {
        node, ok := t.(map[string]interface{})
        if !ok { continue }
        deps, _ := node["depends_on"].([]interface{})
        if len(deps) == 0 {
            independent = append(independent, []map[string]interface{}{node})
        }
    }
    return independent
}
```

- [ ] **Step 3: Verify build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 4: Commit**

```bash
git add forge-core/
git commit -m "feat(s11'): add lint-fix loop and parallel sub-task generation in workflow"
```

---

## Day 2: Frontend Lint Results + Polish

### Task 4: Frontend -- Lint Results Display

**Files:**
- Create: `forge-portal/components/tasks/lint-results-card.tsx`
- Modify: `forge-portal/components/tasks/task-workspace.tsx`

- [ ] **Step 1: Create LintResultsCard component**

`forge-portal/components/tasks/lint-results-card.tsx`:

```
+-------------------------------------------------------------+
| Lint Results                                                  |
| [PASSED] 0 errors, 3 warnings          golangci-lint         |
+-------------------------------------------------------------+
| OR                                                           |
+-------------------------------------------------------------+
| Lint Results                                                  |
| [FIXED] 2 errors auto-fixed, 1 warning                      |
|                                                               |
| Fixes applied:                                                |
| - Added error check for db.Query return value (user.go:45)   |
| - Removed unused import "fmt" (handler.go:3)                  |
+-------------------------------------------------------------+
```

Props: `lintResult: LintResult`, `lintFixed: boolean`, `fixesApplied: string[]`

States:
- `passed=true`: green CheckCircle icon, "0 errors, N warnings"
- `passed=false, lintFixed=true`: amber Wrench icon, "N errors auto-fixed"
  - Show fixes_applied list
- `passed=false, lintFixed=false`: red XCircle icon, "N errors remain"
  - Show error details (file, line, message)

Warnings displayed as collapsible section (not blocking).

- [ ] **Step 2: Integrate into task-workspace.tsx**

In the GENERATE step completed view, add lint results display:

```tsx
if (stepType === "GENERATE" && status === "COMPLETED" && output) {
  const lintResult = output.lint_result;
  const lintFixed = output.lint_fixed || false;
  const lintPassed = output.lint_passed || false;
  const fixesApplied = output.lint_fixes_applied || [];

  return (
    <div className="space-y-4">
      {/* Existing code file display */}
      <CodeOutputViewer files={output.files} ... />

      {/* NEW: Lint results */}
      {lintResult && (
        <LintResultsCard
          lintResult={lintResult}
          lintFixed={lintFixed}
          fixesApplied={fixesApplied}
        />
      )}

      {/* Parallel generation indicator */}
      {output.parallel && (
        <div className="text-sm text-muted-foreground">
          Generated in parallel across {output.groups} independent task groups
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Verify frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 4: Commit**

```bash
git add forge-portal/
git commit -m "feat(s11'): add lint results display and parallel generation indicator in UI"
```

---

### Task 5: Build Verification + End-to-End Testing

- [ ] **Step 1: Go build**

```bash
cd forge-core && go build ./cmd/forge-core
```

- [ ] **Step 2: Frontend build**

```bash
cd forge-portal && npm run build
```

- [ ] **Step 3: Rebuild AI Worker**

```bash
docker compose -f docker-compose.dev.yml up -d --build ai-worker
```

- [ ] **Step 4: End-to-end verification**

1. Restart all services
2. Create task with multi-part requirement (e.g., "add user CRUD + admin dashboard")
3. Verify workflow:
   - PLAN outputs tasks with `touched_files`
   - TEST_WRITING generates context-aware tests
   - GENERATE produces code that:
     a. References correct table names from db_schema
     b. Follows existing API patterns from api_catalog
     c. Respects business rules
4. Lint check runs after generation:
   - If lint passes: `lint_passed: true` in output
   - If lint fails: auto-fix runs, `lint_fixed: true` + `fixes_applied` in output
5. Frontend shows:
   - Code files with Shiki highlighting
   - Lint results card (green/amber/red)
   - Fixes applied list (if auto-fixed)
6. If plan has independent tasks: verify parallel generation occurs
   - Check workflow logs for concurrent activity execution

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat(s11'): complete code generation enhancement with context tools + lint + parallel generation"
```

---

## Data Structures

### New Activity Definitions

| Activity Name | Input | Output | Description |
|--------------|-------|--------|-------------|
| `run_lint_check` | LintInput (task_id, files, project_type) | LintOutput (passed, errors, warnings) | Run language-specific linter |
| `fix_lint_errors` | files + lint_errors | Fixed files + fixes_applied | AI fixes lint errors |

### Generate Output (enhanced)

```json
{
  "files": [...],
  "summary": "...",
  "lint_result": {
    "passed": false,
    "errors": [{"file": "user.go", "line": 45, "message": "error not checked", "rule": "errcheck"}],
    "warnings": [...],
    "error_count": 2,
    "warning_count": 1
  },
  "lint_passed": false,
  "lint_fixed": true,
  "lint_fixes_applied": ["Added error check for db.Query (user.go:45)", "Removed unused import (handler.go:3)"],
  "parallel": true,
  "groups": 3
}
```

---

## Acceptance Criteria

- [ ] CoderAgent uses all 5 profile dimensions (api_catalog, db_schema, module_graph, architecture, business_rules)
- [ ] Generated code references real table/column names, follows existing patterns
- [ ] Lint runs after generation (golangci-lint for Go, eslint for JS/TS)
- [ ] If lint fails: auto-fix attempt via LintCheckerAgent (1 retry)
- [ ] Frontend shows lint results card (passed/fixed/failed)
- [ ] Parallel generation works for independent task nodes (depends_on=[])
- [ ] Parallel results merged and consistency checked
- [ ] If parallel fails: graceful fallback to serial generation
- [ ] `go build` + `npm run build` + ai-worker rebuild pass
