# SX-1 -- Wire Requirement Analysis (AnalystAgent -> Temporal -> Frontend)

**Duration**: 1 day
**Priority**: P0 -- This is the first thing users experience; currently faked
**Dependencies**: None (all infrastructure already exists)

---

## 1. Goal

Make the chat panel truly call the AI AnalystAgent via Temporal when a user types a requirement. Currently the system calls Temporal synchronously and it works, but the 3-minute synchronous wait blocks the HTTP response. Convert to an async pattern: return immediately with "analyzing" status, let the Temporal workflow complete in background, poll or use SSE to push the result back to the frontend.

## 2. Current State Analysis

### 2.1 What works

The full chain **already exists and runs end-to-end**:

1. **Frontend** `forge-portal/components/chat/chat-panel.tsx` (line 63-68): `handleSubmit` calls `onSend(content)` which calls `sendMessage()` from `forge-portal/lib/conversation.ts` (line 19-25) -- POST `/api/projects/${projectId}/tasks/${taskId}/messages`

2. **Go handler** `forge-core/internal/module/conversation/handler.go` (line 20-49): `SendMessage` handler extracts params, calls `service.SendMessage()`

3. **Go service** `forge-core/internal/module/conversation/service.go` (line 41-165): `SendMessage()` saves user message, starts `AnalyzeRequirementWorkflow` via Temporal, **waits synchronously up to 3 minutes** (line 107), extracts result, saves assistant message

4. **Temporal workflow** `forge-core/internal/temporal/workflow/task_workflow.go` (line 682-705): `AnalyzeRequirementWorkflow` dispatches `analyze_requirement` activity to `ai-worker` queue

5. **Python activity** `ai-worker/src/activities/analyze.py` (line 229-260): `analyze_requirement_activity` builds context via `ContextBuilder`, runs `AnalystAgent`, normalizes response, returns `AnalyzeOutput`

6. **AnalystAgent** `ai-worker/src/agents/analyst.py` (line 100-108): builds system prompt with project context, calls LLM via ModelRouter

### 2.2 What is broken

**Nothing is actually broken** -- the chain works. The real problem is **UX**:

- The HTTP request blocks for 20-60 seconds while the LLM thinks
- During this time, the frontend shows a spinner with no feedback
- If the LLM takes >60s (common with qwen3-max for complex requirements), the user sees nothing
- The conversation response includes structured data (options, risks, phase) that the frontend already handles correctly via `OptionButtons`, `RiskAlert`, `ConfirmationCard`

### 2.3 Specific issues to fix

| Issue | Location | Severity |
|-------|----------|----------|
| Synchronous 3-min wait blocks HTTP | `conversation/service.go` line 106-113 | HIGH -- causes timeouts on slow models |
| No thinking/progress indicator | `chat-panel.tsx` line 126-131 | MEDIUM -- just shows spinner |
| Fallback placeholder still in code | `conversation/service.go` line 72 | LOW -- only used when Temporal is nil |

## 3. Implementation Steps

### Step 3.1: Verify Current Sync Path Still Works (15 min)

Before changing anything, confirm the current synchronous path works:

```bash
# Terminal 1: Start forge-core
cd forge-core && go run ./cmd/forge-core

# Terminal 2: Start ai-worker
cd ai-worker && python -m src.main

# Terminal 3: Start frontend
cd forge-portal && npm run dev

# Test: Create a task, type a requirement, verify AI responds
```

Expected: AI responds with a clarify JSON (question + options) within 30-60 seconds.

### Step 3.2: Add SSE Channel for Analysis Streaming (30 min)

Currently SSE is only used for task step progress (`task.SSEHub`). Add analysis-specific events.

**File**: `forge-core/internal/module/conversation/service.go`

Add a new method that fires the workflow and returns immediately:

```go
// SendMessageAsync saves user message, triggers AI analysis via Temporal,
// returns immediately with "analyzing" status. Result arrives via SSE.
func (s *Service) SendMessageAsync(ctx context.Context, projectID, taskID, tenantID, userID int64, content string) (*SendMessageResponse, error) {
    // 1. Verify task exists (same as current)
    t, err := s.taskRepo.FindByID(ctx, taskID)
    if err != nil {
        return nil, fmt.Errorf("task not found: %w", err)
    }
    if t.ProjectID != projectID {
        return nil, fmt.Errorf("task does not belong to project")
    }

    // 2. Save user message (same as current)
    userMsg := &Conversation{
        TaskID:  taskID,
        Role:    RoleUser,
        Content: content,
    }
    if err := s.repo.Create(ctx, userMsg); err != nil {
        return nil, fmt.Errorf("save user message: %w", err)
    }

    // 3. Update task status to ANALYZING
    if t.Status == task.StatusSubmitted {
        _ = s.taskRepo.UpdateStatus(ctx, taskID, task.StatusAnalyzing)
        _ = s.taskRepo.UpdateStepStatus(ctx, taskID, task.StepTypeAnalyze, task.StepRunning)
    }

    // 4. Fire and forget -- start Temporal workflow in background
    if s.temporalClient != nil {
        history, _ := s.repo.ListByTaskID(ctx, taskID)
        messages := make([]map[string]interface{}, 0, len(history))
        for _, h := range history {
            messages = append(messages, map[string]interface{}{
                "role": h.Role, "content": h.Content,
            })
        }
        actInput := map[string]interface{}{
            "project_id":           projectID,
            "task_id":              taskID,
            "requirement":          content,
            "conversation_history": messages,
        }
        workflowOpts := client.StartWorkflowOptions{
            ID:        fmt.Sprintf("analyze-%d-%d", taskID, userMsg.ID),
            TaskQueue: "forge-task-queue",
        }
        we, err := s.temporalClient.ExecuteWorkflow(ctx, workflowOpts, "AnalyzeRequirementWorkflow", actInput)
        if err != nil {
            slog.Warn("failed to trigger AI analysis", "task_id", taskID, "error", err)
        } else {
            // Start goroutine to wait for result and save it
            go s.waitAndSaveAnalysis(taskID, we)
        }
    }

    // 5. Return immediately with "analyzing" status
    placeholderMsg := &Conversation{
        TaskID:  taskID,
        Role:    RoleAssistant,
        Content: "AI is analyzing your requirement...",
    }
    return &SendMessageResponse{
        Conversation: placeholderMsg,
        Status:       "analyzing",
    }, nil
}
```

Add the background waiter:

```go
func (s *Service) waitAndSaveAnalysis(taskID int64, we client.WorkflowRun) {
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
    defer cancel()

    var result map[string]interface{}
    if err := we.Get(ctx, &result); err != nil {
        slog.Warn("AI analysis failed (async)", "task_id", taskID, "error", err)
        // Save error message
        errMsg := &Conversation{
            TaskID:  taskID,
            Role:    RoleAssistant,
            Content: "AI analysis encountered an error. Please try again.",
        }
        _ = s.repo.Create(ctx, errMsg)
        // Push SSE event so frontend updates
        // (SSE hub access needed -- add to Service struct)
        return
    }

    // Extract and save (same logic as current synchronous path)
    aiResponse := ""
    aiStatus := "clarify"
    var aiMetadata map[string]interface{}

    if c, ok := result["content"].(string); ok && c != "" {
        aiResponse = c
    }
    if status, ok := result["status"].(string); ok && status != "" {
        aiStatus = status
    }
    if md, ok := result["metadata"].(map[string]interface{}); ok {
        aiMetadata = md
    }
    if risks, ok := result["risks"]; ok {
        if aiMetadata == nil {
            aiMetadata = make(map[string]interface{})
        }
        aiMetadata["risks"] = risks
    }

    // Save assistant message
    assistantMsg := &Conversation{
        TaskID:  taskID,
        Role:    RoleAssistant,
        Content: aiResponse,
    }
    if aiMetadata != nil {
        metaJSON, _ := json.Marshal(aiMetadata)
        raw := json.RawMessage(metaJSON)
        assistantMsg.Metadata = &raw
    }
    _ = s.repo.Create(ctx, assistantMsg)

    // Push SSE event with the full analysis result
    // Frontend will receive this and append the message
}
```

### Step 3.3: DECISION -- Keep Synchronous, Add Frontend Polling Instead

**After analysis, the simpler approach is better**: Keep the current synchronous pattern but add **frontend polling** as a resilience layer.

Rationale:
- The synchronous path works and is simpler to reason about
- 20-60 second wait is acceptable for requirement analysis (users expect AI to think)
- The frontend already shows a spinner; we just need to improve it
- Adding SSE for analysis adds complexity that is not needed yet when the sync path works

**Revised plan**: Keep `SendMessage` synchronous. Fix the frontend to show better progress feedback.

### Step 3.4: Improve Frontend Loading State (45 min)

**File**: `forge-portal/components/chat/chat-panel.tsx`

Replace the simple spinner (line 126-131) with a thinking animation:

```tsx
{isLoading && (
  <div className="flex justify-start mb-4">
    <div className="bg-white/5 rounded-2xl rounded-bl-md px-4 py-3 max-w-[80%]">
      <div className="flex items-center gap-2 text-white/50 text-sm">
        <Loader2 className="h-4 w-4 animate-spin text-[#8B5CF6]" />
        <span className="animate-pulse">AI is analyzing your requirement...</span>
      </div>
      <div className="mt-2 text-white/30 text-xs">
        Typical analysis takes 15-45 seconds
      </div>
    </div>
  </div>
)}
```

### Step 3.5: Add Timeout Handling in Frontend (30 min)

**File**: `forge-portal/lib/conversation.ts`

The current `sendMessage` has no timeout. Add one:

```typescript
export async function sendMessage(
  projectId: number,
  taskId: number,
  content: string
): Promise<SendMessageResponse> {
  return api.post(`/projects/${projectId}/tasks/${taskId}/messages`, { content }, {
    timeout: 180000, // 3 minutes to match Temporal timeout
  });
}
```

**File**: The parent component that calls `onSend` (find via searching for `sendMessage` usage) -- add error handling for timeouts:

```typescript
const handleSend = async (content: string) => {
  try {
    setIsLoading(true);
    const result = await sendMessage(projectId, taskId, content);
    // ... handle result
  } catch (error) {
    if (error instanceof Error && error.message.includes('timeout')) {
      toast.error("Analysis is taking longer than expected. Please refresh and check the conversation history.");
    } else {
      toast.error("Failed to send message");
    }
  } finally {
    setIsLoading(false);
  }
};
```

### Step 3.6: Remove Dead Placeholder Text (10 min)

**File**: `forge-core/internal/module/conversation/service.go`

Line 72: Change the placeholder text that is only used when Temporal is nil:

```go
// Before:
aiResponse := "AI analysis coming soon, placeholder response."
// After:
aiResponse := "AI service is currently unavailable. Please ensure Temporal and AI Worker are running."
```

Same for `TriggerAnalysis` at line 438.

### Step 3.7: Verify Multi-Round Conversation (30 min)

The critical behavior to verify is multi-round clarification:

1. User types "give users a points feature" (Chinese: "give users a points feature")
2. AI should respond with Phase 1 (understanding) -- a single question with options
3. User clicks an option or types a response
4. AI should respond with Phase 2 (scenario) -- another question
5. After 3+ rounds, AI should respond with `status: "confirmed"` and full requirements

**Test script**:

```bash
# 1. Create a task
curl -X POST http://localhost:8080/api/projects/1/tasks \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"requirement": "give users a points feature"}'

# 2. Send first message (triggers analysis)
curl -X POST http://localhost:8080/api/projects/1/tasks/<taskId>/messages \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"content": "give users a points feature"}'

# 3. Check response -- should be status=clarify with question+options

# 4. Reply with selection
curl -X POST http://localhost:8080/api/projects/1/tasks/<taskId>/messages \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"content": "Daily points calculation, automatic distribution"}'

# 5. Repeat until status=confirmed
```

**What to verify at each step**:
- `status` field is `"clarify"` or `"confirmed"`
- `metadata` contains `phase`, `question`, `options`, `risks`
- `content` is human-readable Chinese markdown
- `conversation_history` is correctly accumulated (check Python activity logs)
- The model used is from the routing chain (check `ai-worker` logs for `provider=` and `model=`)

## 4. Files Modified

| File | Change |
|------|--------|
| `forge-core/internal/module/conversation/service.go` | Update placeholder text (line 72, 438); potentially add async path |
| `forge-portal/components/chat/chat-panel.tsx` | Improve loading state UI (line 126-131) |
| `forge-portal/lib/conversation.ts` | Add timeout to `sendMessage` |

## 5. Acceptance Criteria

- [ ] User types a Chinese requirement in chat -- AI responds with structured clarification question within 60 seconds
- [ ] AI response includes `status: "clarify"`, `phase`, `question`, `options` in metadata
- [ ] Frontend renders clickable option buttons from metadata
- [ ] Multi-round conversation works: at least 3 rounds before confirmation
- [ ] Final `status: "confirmed"` response shows ConfirmationCard with full requirements summary
- [ ] Risks are displayed when present
- [ ] Loading state shows descriptive text ("AI is analyzing...") not just a spinner
- [ ] If AI fails, error message is shown (not silent failure)

## 6. Risks and Rollback

| Risk | Mitigation |
|------|------------|
| qwen3-max takes >60s for first response | ModelRouter fallback chain already handles this: qwen3-max -> claude-sonnet-4 -> gpt-4o -> deepseek |
| Temporal not running | Service falls back to static placeholder message (line 72) |
| Python ai-worker not running | Temporal activity will timeout after 3 min; Go service returns error message |
| JSON parse failure from LLM | `BaseAgent._parse_json` (base.py line 59-88) has robust fallback: tries direct parse, markdown block, balanced braces |
| LLM returns `questions` (plural) instead of `question` | `normalize_clarify_response` (analyze.py line 151-187) handles legacy format conversion |

---

**Estimated actual work**: 2-3 hours (most of it is verification, not code changes)
