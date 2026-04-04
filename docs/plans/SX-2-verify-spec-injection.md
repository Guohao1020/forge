# SX-2 -- Verify and Fix Spec Injection

**Duration**: 1 day
**Priority**: P0 -- Specs are the soul of Harness Engineering; if they are not injected, generated code ignores all standards
**Dependencies**: SX-1 (need working analysis flow to test end-to-end)

---

## 1. Goal

Verify that coding standards created in the Specs Center actually reach the LLM system prompt and influence generated code. If broken, trace and fix the exact failure point. Add permanent verification: a spec compliance check in ReviewerAgent.

## 2. Current State Analysis

### 2.1 The Intended Chain

```
Specs Center (Web UI)
  -> POST /api/specs/standards (forge-core/internal/module/specs/handler.go:82)
  -> specs.Service.CreateStandard() (specs/service.go:43)
  -> engine.coding_standards table (PostgreSQL)
  -> Redis cache invalidation (specs/service.go:48)

AI Worker Context Build
  -> ContextBuilder.build() (ai-worker/src/context/builder.py:79)
  -> GET /api/specs/effective/{projectId} (builder.py:100)
  -> specs.Service.GetEffectiveSpecs() (specs/service.go:167)
  -> Three-level inheritance: COMPANY -> TEAM -> PROJECT (service.go:179-201)
  -> Returns merged standards + review rules (service.go:202-206)

Context Assembly
  -> ProjectContext.to_system_prompt() (builder.py:27)
  -> Layer 2: Standards injection (builder.py:34-37):
     if self.coding_standards:
         parts.append("\n## Coding Standards (MUST follow)\n")
         for std in self.coding_standards:
             parts.append(std)

Agent Execution
  -> AnalystAgent._build_system_prompt() (analyst.py:103-107)
  -> Calls context.to_system_prompt() to get project context
  -> Appends to ANALYST_SYSTEM_PROMPT
  -> Same pattern for CoderAgent, ReviewerAgent, etc.
```

### 2.2 Potential Failure Points

| # | Location | What Could Break | How to Check |
|---|----------|-----------------|--------------|
| 1 | `specs/service.go:167` GetEffectiveSpecs | Redis cache returns stale empty data | Check Redis key `specs:effective:{tenantId}:{projectId}` |
| 2 | `specs/service.go:180` GetStandardsByScope | Query uses COMPANY scope with scopeId=0; project standards need correct scopeId | SQL query with `scope='PROJECT' AND scope_id=$2` |
| 3 | `builder.py:100-107` HTTP call to `/api/specs/effective/{projectId}` | Auth token missing or expired; wrong base URL | Check `settings.forge_api_token` and `settings.forge_api_url` |
| 4 | `builder.py:104-107` Response parsing | API returns `{"data": {"standards": [...]}}` but builder expects specific shape | Log raw response |
| 5 | `builder.py:34-37` Standards injection | `coding_standards` list populated with `s.get("content", "")` -- if standard has no content field, empty strings | Check standard content is non-empty |
| 6 | Agent system prompt | Standards appended but total token count exceeds model context window | Check token budget (180k limit in builder.py:12) |

### 2.3 Current Standard Categories

From `specs/model.go` line 25:
```
Category: JAVA | SQL | REDIS | KAFKA | API | SECURITY | NAMING | GIT
Scope:    COMPANY | TEAM | PROJECT
```

Standards have `content` (string) which is the actual standard text injected into prompts.

## 3. Implementation Steps

### Step 3.1: Create a Test Standard (15 min)

Use curl or the Specs Center UI to create a standard that is easy to verify:

```bash
# Login first
TOKEN=$(curl -s -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.data.token')

# Create a COMPANY-level Go coding standard
curl -X POST http://localhost:8080/api/specs/standards \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Go Error Handling Standard",
    "category": "JAVA",
    "scope": "COMPANY",
    "scopeId": 0,
    "content": "## Go Error Handling Rules\n\n1. ALL functions that return error MUST have explicit error handling -- never use _ to discard errors\n2. Every error must be wrapped with context using fmt.Errorf(\"description: %w\", err)\n3. Never use panic() in library code -- only in main() for unrecoverable initialization errors\n4. Use errors.Is() and errors.As() for error comparison, never == comparison\n5. Custom error types must implement the error interface"
  }'

# Verify it was saved
curl -s http://localhost:8080/api/specs/standards \
  -H "Authorization: Bearer $TOKEN" | jq '.data.items[].name'
```

Note: We use category "JAVA" because the current categories do not include "GO". This is a known limitation -- categories should be expanded. The content still works regardless of category label.

### Step 3.2: Verify Effective Specs API Returns the Standard (15 min)

```bash
# Replace {projectId} with your test project ID
curl -s http://localhost:8080/api/specs/effective/1 \
  -H "Authorization: Bearer $TOKEN" | jq '.data'
```

**Expected output**:
```json
{
  "standards": [
    {
      "id": 1,
      "name": "Go Error Handling Standard",
      "category": "JAVA",
      "scope": "COMPANY",
      "content": "## Go Error Handling Rules\n\n1. ALL functions that return error MUST have explicit error handling..."
    }
  ],
  "rules": []
}
```

**If empty**: Check Redis cache. Clear it:
```bash
redis-cli -a forge_redis_2026 KEYS "specs:effective:*"
redis-cli -a forge_redis_2026 DEL "specs:effective:1:1"
```

Then retry the API call.

### Step 3.3: Add Logging to ContextBuilder (30 min)

**File**: `ai-worker/src/context/builder.py`

Add detailed logging to trace what the builder fetches and assembles:

```python
# In build() method, after fetching effective specs (around line 100-107):
async def build(self, project_id: int, purpose: str, conversation_history=None):
    ctx = ProjectContext()
    ctx.conversation_history = conversation_history or []

    # ... (project fetch unchanged) ...

    # Fetch effective specs -- GET /api/specs/effective/{project_id}
    try:
        resp = await self._client.get(f"/api/specs/effective/{project_id}")
        logger.info(
            "Specs API response: status=%d, body_len=%d",
            resp.status_code,
            len(resp.text),
        )
        if resp.status_code == 200:
            data = resp.json().get("data", {})
            standards = data.get("standards", [])
            logger.info(
                "Fetched %d standards for project %d",
                len(standards),
                project_id,
            )
            for i, s in enumerate(standards):
                content = s.get("content", "")
                logger.info(
                    "  Standard[%d]: name=%s, category=%s, content_len=%d",
                    i,
                    s.get("name", "?"),
                    s.get("category", "?"),
                    len(content),
                )
            ctx.coding_standards = [
                s.get("content", "") for s in standards if s.get("content")
            ]
            ctx.review_rules = data.get("rules", [])
        else:
            logger.warning(
                "Specs API returned %d: %s",
                resp.status_code,
                resp.text[:200],
            )
    except Exception as e:
        logger.warning(f"Failed to fetch specs for project {project_id}: {e}")

    # ... rest unchanged ...

    return ctx
```

Also add logging at the system prompt assembly level:

```python
# In to_system_prompt() method (builder.py:27), add at the end:
def to_system_prompt(self) -> str:
    # ... existing code ...
    result = "\n\n".join(parts)
    logger.info(
        "System prompt assembled: len=%d, standards=%d, profiles=%d",
        len(result),
        len(self.coding_standards),
        len(self.project_profiles),
    )
    return result
```

### Step 3.4: Add Logging to Agent to Dump Full System Prompt (15 min)

**File**: `ai-worker/src/agents/base.py`

In the `run()` method (line 43-57), add logging before the LLM call:

```python
async def run(self, user_input: str, context: ProjectContext) -> AgentResult:
    system = self._build_system_prompt(context)
    messages = self._build_messages(user_input, context)

    # Debug: log the full system prompt (first 2000 chars) and message count
    logger.info(
        "Agent %s: system_prompt_len=%d, messages=%d, first_2000_chars:\n%s",
        self.__class__.__name__,
        len(system),
        len(messages),
        system[:2000],
    )

    response: LLMResponse = await self.router.chat(
        system=system, messages=messages, purpose=self.purpose
    )
    # ... rest unchanged ...
```

### Step 3.5: Trigger Code Generation and Inspect Logs (45 min)

Run the full pipeline through the frontend:

1. Import/select a Go project
2. Create a task: "Add a health check endpoint that returns server status and database connectivity"
3. Go through the analysis conversation (2-3 rounds)
4. Confirm requirements
5. Approve plan

**While running, watch logs**:

```bash
# AI Worker logs (Python)
cd ai-worker && python -m src.main 2>&1 | tee /tmp/ai-worker.log

# After the run, search for spec injection:
grep "standards" /tmp/ai-worker.log
grep "Coding Standards" /tmp/ai-worker.log
grep "error handling" /tmp/ai-worker.log  # Our test standard content
```

**What to look for in logs**:

1. `Fetched N standards for project X` -- should be >= 1
2. `Standard[0]: name=Go Error Handling Standard, ...` -- our test standard
3. `System prompt assembled: len=XXXX, standards=1, profiles=N`
4. In the CoderAgent (generate step), the system prompt should contain "## Coding Standards (MUST follow)"

### Step 3.6: Verify Generated Code Follows the Standard (30 min)

After code generation completes:

1. Go to the task page in the frontend
2. Click on the GENERATE step
3. Inspect the generated files

**Check specifically**:
- Every function that returns error has explicit error handling
- Errors are wrapped with `fmt.Errorf("context: %w", err)`
- No `panic()` in library code
- Uses `errors.Is()` / `errors.As()` for comparison

If the standard is NOT followed, the spec injection is broken. Proceed to Step 3.7.

### Step 3.7: Fix Identified Issues

#### Issue A: forge_api_token is empty

**File**: `ai-worker/.env`

The `ContextBuilder` uses `settings.forge_api_token` (config.py line 22) to authenticate with forge-core. If this is empty, the `/api/specs/effective/{projectId}` call will return 401.

**Fix**: The AI Worker needs to call forge-core's internal API. Since they are co-located in dev, use the admin token or create a service token.

```env
# ai-worker/.env
FORGE_API_URL=http://localhost:8080
FORGE_API_TOKEN=<internal-service-token>
```

**Alternative**: Make the effective specs endpoint bypass auth for internal calls. Add the route to the public group or check for a service header.

**File**: `forge-core/internal/router/router.go`

Add an internal API group that does not require JWT:

```go
// Internal API for AI Worker (authenticated by service token header)
internal := api.Group("/internal")
internal.Use(middleware.ServiceTokenAuth("forge-internal-2026"))
{
    internal.GET("/specs/effective/:projectId", deps.SpecsHandler.GetEffectiveSpecs)
    internal.GET("/projects/:id", deps.ProjectHandler.GetByID)
    internal.GET("/projects/:id/profiles", deps.ProfileHandler.ListProfiles)
}
```

Or simpler: add these specific routes to the unprotected group since they return non-sensitive project metadata.

#### Issue B: Standard content is empty

**File**: `forge-core/internal/module/specs/repository.go`

Check the SQL query that fetches standards. The `content` column must be included in SELECT.

#### Issue C: ContextBuilder gets 404 on effective specs

The API path may be wrong. Check:
- forge-core router registers `/api/specs/effective/:projectId` (check router.go)
- ContextBuilder calls `/api/specs/effective/{project_id}` (builder.py line 100)

These must match exactly.

### Step 3.8: Add Spec Compliance Check to ReviewerAgent (1 hour)

**File**: `ai-worker/src/agents/reviewer.py`

The ReviewerAgent already receives the full context including `coding_standards` via `to_system_prompt()`. But it does not explicitly compare generated code against standards.

Add explicit spec compliance section to the reviewer system prompt:

```python
REVIEWER_SYSTEM_PROMPT = """You are a strict code reviewer...

## CRITICAL: Coding Standards Compliance
If coding standards are provided in the Project Context, you MUST:
1. Check EVERY standard rule against the generated code
2. For each standard rule violated, add a finding with:
   - severity: "ERROR"
   - rule: "STANDARD/{standard-name}"
   - message: Quote the specific standard rule and explain how the code violates it
3. Code that violates ANY coding standard MUST NOT pass review (passed=false)
4. In your summary, explicitly state which standards were checked and whether code complies

This is non-negotiable. Standards compliance is the primary purpose of Forge's Harness Engineering approach.

... (rest of existing prompt) ...
"""
```

**File**: `ai-worker/src/agents/reviewer.py`

Override `_build_system_prompt` to inject standards more prominently:

```python
class ReviewerAgent(BaseAgent):
    purpose = Purpose.REVIEW

    def _build_system_prompt(self, context: ProjectContext) -> str:
        base = REVIEWER_SYSTEM_PROMPT

        # Inject coding standards explicitly for compliance checking
        if context.coding_standards:
            base += "\n\n## Coding Standards to Enforce\n"
            base += "The following standards MUST be checked against every generated file:\n\n"
            for i, std in enumerate(context.coding_standards, 1):
                base += f"### Standard {i}\n{std}\n\n"

        if context.review_rules:
            base += "\n\n## Review Rules to Check\n"
            for rule in context.review_rules:
                name = rule.get("name", "")
                category = rule.get("category", "")
                severity = rule.get("severity", "")
                base += f"- [{severity}] {category}: {name}\n"

        project_context = context.to_system_prompt()
        if project_context:
            base += f"\n\n{project_context}"
        return base
```

### Step 3.9: Add Standard Category for Go/Python/TypeScript (15 min)

**File**: `forge-core/internal/module/specs/model.go`

The current categories are limited: `JAVA | SQL | REDIS | KAFKA | API | SECURITY | NAMING | GIT`

Add more categories:

```go
type CreateStandardReq struct {
    Name     string `json:"name" binding:"required,max=200"`
    Category string `json:"category" binding:"required,oneof=JAVA GO PYTHON TYPESCRIPT SQL REDIS KAFKA API SECURITY NAMING GIT TESTING ARCHITECTURE"`
    Scope    string `json:"scope" binding:"required,oneof=COMPANY TEAM PROJECT"`
    ScopeID  int64  `json:"scopeId"`
    ParentID *int64 `json:"parentId"`
    Content  string `json:"content" binding:"required"`
}
```

This is a simple validation change -- no DB migration needed since category is stored as a text column.

## 4. Files Modified

| File | Change |
|------|--------|
| `ai-worker/src/context/builder.py` | Add detailed logging for spec fetch and prompt assembly |
| `ai-worker/src/agents/base.py` | Add logging to dump system prompt before LLM call |
| `ai-worker/src/agents/reviewer.py` | Add explicit spec compliance checking section to system prompt |
| `ai-worker/.env` | Set `FORGE_API_TOKEN` for internal API auth |
| `forge-core/internal/module/specs/model.go` | Expand standard categories to include GO, PYTHON, TYPESCRIPT |

## 5. Acceptance Criteria

- [ ] Create a coding standard via API or UI -- verify it appears in GET `/api/specs/effective/{projectId}`
- [ ] AI Worker logs show "Fetched N standards" with N >= 1 during any agent execution
- [ ] AI Worker logs show system prompt contains "## Coding Standards (MUST follow)"
- [ ] Generated code demonstrably follows the injected standard (manual inspection)
- [ ] ReviewerAgent findings include "STANDARD/" rule type when code violates a standard
- [ ] ReviewerAgent fails code that violates a coding standard (passed=false, score < 80)
- [ ] Logging can be reduced to WARNING level after verification (remove DEBUG-level prompt dumps)

## 6. Risks and Rollback

| Risk | Mitigation |
|------|------------|
| AI Worker cannot reach forge-core API (network/auth) | Add service token; fallback: empty standards list means code generated without constraints (existing behavior) |
| Standards too long, exceed token budget | `TOKEN_BUDGET = 180_000` (builder.py:12) gives plenty of room; individual standards should be < 5000 chars |
| Reviewer becomes too strict, fails everything | Score threshold is 80 (reviewer.py line 68); standard violations are ERROR severity; can adjust threshold if needed |
| LLM ignores standards in system prompt | This is a model quality issue; qwen3-max and claude-sonnet-4 both follow system prompts well; add "CRITICAL" and "MUST" emphasis |
| Adding logging slows down agent execution | Logger calls are negligible; remove after verification if desired |

---

**Estimated actual work**: 3-4 hours (half is investigation, half is enhancement)
