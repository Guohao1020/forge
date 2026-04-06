package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/shulex/forge/forge-core/internal/module/task"
	"github.com/shulex/forge/forge-core/internal/temporal/activity"
	"go.temporal.io/sdk/client"
)

// TaskRepo defines the task repository methods needed by conversation service.
type TaskRepo interface {
	FindByID(ctx context.Context, id int64) (*task.Task, error)
	UpdateStatus(ctx context.Context, id int64, status string) error
	UpdateWorkflowIDs(ctx context.Context, taskID int64, workflowID, runID string) error
	UpdateAnalysis(ctx context.Context, taskID int64, analysis string) error
	UpdateStepStatus(ctx context.Context, taskID int64, stepType, status string) error
	GetStepsByTaskID(ctx context.Context, taskID int64) ([]task.TaskStep, error)
	SaveStepOutput(ctx context.Context, taskID int64, stepType string, output json.RawMessage) error
}

// AnalysisCompleteEvent is broadcast via SSE when AI analysis finishes.
type AnalysisCompleteEvent struct {
	Type   string `json:"type"`
	TaskID int64  `json:"task_id"`
	Status string `json:"status"`
	Data   string `json:"data,omitempty"`
}

// SSEBroadcaster broadcasts events to SSE clients watching a task.
type SSEBroadcaster interface {
	BroadcastRaw(taskID int64, data []byte)
}

type Service struct {
	repo           *Repository
	taskRepo       TaskRepo
	temporalClient client.Client // nil if Temporal unavailable
	sseBroadcaster SSEBroadcaster
}

func NewService(repo *Repository, taskRepo TaskRepo, tc client.Client, sse SSEBroadcaster) *Service {
	return &Service{
		repo:           repo,
		taskRepo:       taskRepo,
		temporalClient: tc,
		sseBroadcaster: sse,
	}
}

// SendMessage saves user message, triggers AI analysis asynchronously, returns immediately.
// The AI analysis result is delivered via SSE (analyze_token stream + ANALYSIS_COMPLETE event).
func (s *Service) SendMessage(ctx context.Context, projectID, taskID, tenantID, userID int64, content string) (*SendMessageResponse, error) {
	// Verify task exists
	t, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	if t.ProjectID != projectID {
		return nil, fmt.Errorf("task does not belong to project")
	}

	// Save user message
	userMsg := &Conversation{
		TaskID:  taskID,
		Role:    RoleUser,
		Content: content,
	}
	if err := s.repo.Create(ctx, userMsg); err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}

	// Update task status to ANALYZING if still SUBMITTED, and mark ANALYZE step as RUNNING
	if t.Status == task.StatusSubmitted {
		if err := s.taskRepo.UpdateStatus(ctx, taskID, task.StatusAnalyzing); err != nil {
			slog.Warn("failed to update task status to ANALYZING", "task_id", taskID, "error", err)
		}
		if err := s.taskRepo.UpdateStepStatus(ctx, taskID, task.StepTypeAnalyze, task.StepRunning); err != nil {
			slog.Warn("failed to mark ANALYZE step running", "task_id", taskID, "error", err)
		}
	}

	// Start AI analysis workflow asynchronously — result delivered via SSE
	if s.temporalClient != nil {
		history, err := s.repo.ListByTaskID(ctx, taskID)
		if err != nil {
			slog.Warn("failed to load conversation history", "task_id", taskID, "error", err)
		}
		messages := make([]map[string]interface{}, 0, len(history))
		for _, h := range history {
			msg := map[string]interface{}{
				"role":    h.Role,
				"content": h.Content,
			}
			// Include metadata for assistant messages so the AI worker can
			// reconstruct the expected conversation format
			if h.Role == RoleAssistant && h.Metadata != nil {
				var meta map[string]interface{}
				if err := json.Unmarshal(*h.Metadata, &meta); err == nil {
					msg["metadata"] = meta
				}
			}
			messages = append(messages, msg)
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
			// Wait for result in background goroutine, then broadcast via SSE
			go s.waitAndBroadcastAnalysis(taskID, we)
		}
	}

	// Return immediately — AI response will arrive via SSE
	return &SendMessageResponse{
		Conversation: userMsg,
		Status:       "analyzing",
		Metadata:     nil,
	}, nil
}

// waitAndBroadcastAnalysis waits for the Temporal workflow result in background,
// saves the AI response to DB, and broadcasts ANALYSIS_COMPLETE via SSE.
func (s *Service) waitAndBroadcastAnalysis(taskID int64, we client.WorkflowRun) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	aiResponse := "AI 分析遇到问题，请稍后重试。"
	aiStatus := "clarify"
	var aiMetadata map[string]interface{}

	var result map[string]interface{}
	if err := we.Get(ctx, &result); err != nil {
		slog.Warn("AI analysis failed", "task_id", taskID, "error", err)
		aiResponse = "AI 分析遇到问题，请稍后重试。错误: " + err.Error()
		aiStatus = "error"
	} else {
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
		if aiStatus == "confirmed" {
			if metadata, err := json.Marshal(aiMetadata); err == nil {
				if err := s.taskRepo.UpdateAnalysis(ctx, taskID, string(metadata)); err != nil {
					slog.Warn("failed to update task analysis", "task_id", taskID, "error", err)
				}
			}
		}
	}

	// Update ANALYZE step status to reflect progress
	if aiStatus != "error" {
		_ = s.taskRepo.UpdateStepStatus(ctx, taskID, "ANALYZE", "RUNNING")
	}

	// Save AI response to DB with message type for frontend routing
	if aiMetadata == nil {
		aiMetadata = make(map[string]interface{})
	}
	aiMetadata["message_type"] = "analysis"
	assistantMsg := &Conversation{
		TaskID:  taskID,
		Role:    RoleAssistant,
		Content: aiResponse,
	}
	metaJSON, _ := json.Marshal(aiMetadata)
	raw := json.RawMessage(metaJSON)
	assistantMsg.Metadata = &raw
	if err := s.repo.Create(ctx, assistantMsg); err != nil {
		slog.Error("failed to save AI response", "task_id", taskID, "error", err)
	}

	// Broadcast ANALYSIS_COMPLETE event via SSE — frontend will fetch new messages
	if s.sseBroadcaster != nil {
		eventData := map[string]interface{}{
			"status":   aiStatus,
			"metadata": aiMetadata,
		}
		dataJSON, _ := json.Marshal(eventData)
		evt := AnalysisCompleteEvent{
			Type:   "ANALYSIS_COMPLETE",
			TaskID: taskID,
			Status: aiStatus,
			Data:   string(dataJSON),
		}
		evtBytes, _ := json.Marshal(evt)
		s.sseBroadcaster.BroadcastRaw(taskID, evtBytes)
	}
}

// ConfirmPlan confirms requirements, starts PlanOnlyWorkflow asynchronously,
// and returns immediately. The plan result is delivered via SSE (PLAN_COMPLETE event).
// This mirrors the async pattern used by SendMessage/waitAndBroadcastAnalysis.
func (s *Service) ConfirmPlan(ctx context.Context, taskID, tenantID int64) (*PlanConfirmResponse, error) {
	t, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	if t.Status == task.StatusSubmitted {
		return nil, fmt.Errorf("task has no analysis yet — send a message first")
	}

	if s.temporalClient == nil {
		return nil, fmt.Errorf("temporal not available")
	}

	// Mark ANALYZE step as COMPLETED and save confirmed requirements as step output
	if err := s.taskRepo.UpdateStepStatus(ctx, taskID, task.StepTypeAnalyze, task.StepCompleted); err != nil {
		slog.Warn("failed to mark ANALYZE completed", "task_id", taskID, "error", err)
	}
	history, _ := s.repo.ListByTaskID(ctx, taskID)
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.Role == RoleAssistant && msg.Metadata != nil {
			var meta map[string]interface{}
			if err := json.Unmarshal(*msg.Metadata, &meta); err == nil {
				if status, ok := meta["status"].(string); ok && status == "confirmed" {
					_ = s.taskRepo.SaveStepOutput(ctx, taskID, task.StepTypeAnalyze, *msg.Metadata)
					break
				}
			}
		}
	}

	// Extract confirmed requirements for planner context
	var confirmedReqs map[string]interface{}
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		if msg.Role == RoleAssistant && msg.Metadata != nil {
			var meta map[string]interface{}
			if err := json.Unmarshal(*msg.Metadata, &meta); err == nil {
				if status, ok := meta["status"].(string); ok && status == "confirmed" {
					confirmedReqs = meta
					break
				}
			}
		}
	}

	// Fetch project metadata for workflow
	var projectName, repoURL string
	row := s.repo.DB().QueryRow(ctx,
		`SELECT name, code_repo_url FROM engine.projects WHERE id = $1`, t.ProjectID)
	_ = row.Scan(&projectName, &repoURL)

	// Start PlanOnlyWorkflow asynchronously — result delivered via SSE
	workflowID := fmt.Sprintf("plan-%d", taskID)
	we, err := s.temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "forge-task-queue",
	}, "PlanOnlyWorkflow", activity.TaskWorkflowInput{
		TaskID:                taskID,
		TenantID:              tenantID,
		ProjectID:             t.ProjectID,
		CreatedBy:             t.CreatedBy,
		Requirement:           t.Requirement,
		Title:                 derefStr(t.Title),
		ProjectName:           projectName,
		RepoURL:               repoURL,
		ConfirmedRequirements: confirmedReqs,
	})
	if err != nil {
		return nil, fmt.Errorf("start plan workflow: %w", err)
	}

	// Wait for result in background goroutine, then broadcast via SSE
	go s.waitAndBroadcastPlan(taskID, we)

	// Return immediately — plan result will arrive via SSE PLAN_COMPLETE event
	return &PlanConfirmResponse{
		Status: "planning",
	}, nil
}

// waitAndBroadcastPlan waits for the PlanOnlyWorkflow result in background,
// saves the plan to DB, and broadcasts PLAN_COMPLETE via SSE.
func (s *Service) waitAndBroadcastPlan(taskID int64, we client.WorkflowRun) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var planResult map[string]interface{}
	if err := we.Get(ctx, &planResult); err != nil {
		slog.Error("plan generation failed", "task_id", taskID, "error", err)

		// Save error message to conversation
		errContent := "方案生成失败，请重试。错误: " + err.Error()
		errMeta := map[string]interface{}{
			"message_type": "plan_error",
			"error":        err.Error(),
		}
		metaJSON, _ := json.Marshal(errMeta)
		raw := json.RawMessage(metaJSON)
		errMsg := &Conversation{
			TaskID:   taskID,
			Role:     RoleAssistant,
			Content:  errContent,
			Metadata: &raw,
		}
		if err := s.repo.Create(ctx, errMsg); err != nil {
			slog.Error("failed to save plan error message", "task_id", taskID, "error", err)
		}

		// Broadcast error event via SSE
		if s.sseBroadcaster != nil {
			evt := map[string]interface{}{
				"type":    "PLAN_COMPLETE",
				"task_id": taskID,
				"status":  "error",
				"data":    errContent,
			}
			evtBytes, _ := json.Marshal(evt)
			s.sseBroadcaster.BroadcastRaw(taskID, evtBytes)
		}
		return
	}

	// Format plan as human-readable text for the conversation
	planText := formatPlanForConversation(planResult)
	planResult["message_type"] = "plan"
	planMeta, _ := json.Marshal(planResult)
	raw := json.RawMessage(planMeta)
	assistantMsg := &Conversation{
		TaskID:   taskID,
		Role:     RoleAssistant,
		Content:  planText,
		Metadata: &raw,
	}
	if err := s.repo.Create(ctx, assistantMsg); err != nil {
		slog.Error("failed to save plan message", "task_id", taskID, "error", err)
	}

	// Broadcast PLAN_COMPLETE event via SSE — frontend will fetch plan data
	if s.sseBroadcaster != nil {
		evt := map[string]interface{}{
			"type":    "PLAN_COMPLETE",
			"task_id": taskID,
			"status":  "plan_review",
		}
		evtBytes, _ := json.Marshal(evt)
		s.sseBroadcaster.BroadcastRaw(taskID, evtBytes)
	}
}

// ApprovePlan approves the generated plan and starts the execution workflow.
func (s *Service) ApprovePlan(ctx context.Context, taskID, tenantID int64) error {
	t, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	if s.temporalClient == nil {
		return fmt.Errorf("temporal not available")
	}

	// Load plan result from PLAN step output
	var planResult map[string]interface{}
	steps, err := s.taskRepo.GetStepsByTaskID(ctx, taskID)
	if err == nil {
		for _, step := range steps {
			if step.StepType == task.StepTypePlan && step.Output != nil {
				if err := json.Unmarshal([]byte(*step.Output), &planResult); err != nil {
					slog.Warn("failed to parse plan output", "task_id", taskID, "error", err)
				}
				break
			}
		}
	}

	// Fetch project metadata for workflow
	var projectName, repoURL string
	row := s.repo.DB().QueryRow(ctx,
		`SELECT name, code_repo_url FROM engine.projects WHERE id = $1`, t.ProjectID)
	_ = row.Scan(&projectName, &repoURL)

	// Start TaskExecutionWorkflow (TEST_WRITING → GENERATE → REVIEW → TEST → DEPLOY)
	workflowID := fmt.Sprintf("task-%d", taskID)
	we, err := s.temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "forge-task-queue",
	}, "TaskExecutionWorkflow", activity.TaskWorkflowInput{
		TaskID:      taskID,
		TenantID:    tenantID,
		ProjectID:   t.ProjectID,
		CreatedBy:   t.CreatedBy,
		Requirement: t.Requirement,
		Title:       derefStr(t.Title),
		ProjectName: projectName,
		RepoURL:     repoURL,
		PlanResult:  planResult,
	})
	if err != nil {
		return fmt.Errorf("start execution workflow: %w", err)
	}

	if err := s.taskRepo.UpdateWorkflowIDs(ctx, taskID, we.GetID(), we.GetRunID()); err != nil {
		slog.Error("failed to save workflow IDs", "task_id", taskID, "error", err)
	}

	// Signal VersionOrchestrator with new task (non-fatal)
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var versionID int64
		var touchedFiles []string
		row := s.repo.DB().QueryRow(bgCtx,
			`SELECT COALESCE(version_id, 0) FROM engine.tasks WHERE id = $1`, taskID)
		_ = row.Scan(&versionID)

		if versionID > 0 {
			rows, _ := s.repo.DB().Query(bgCtx,
				`SELECT COALESCE(touched_files, '[]'::jsonb) FROM engine.tasks WHERE id = $1`, taskID)
			if rows != nil {
				defer rows.Close()
				if rows.Next() {
					var filesJSON json.RawMessage
					_ = rows.Scan(&filesJSON)
					_ = json.Unmarshal(filesJSON, &touchedFiles)
				}
			}

			orchestratorID := fmt.Sprintf("version-orchestrator-%d", versionID)
			signal := map[string]interface{}{
				"task_id":       taskID,
				"touched_files": touchedFiles,
			}
			if err := s.temporalClient.SignalWorkflow(bgCtx, orchestratorID, "", "new_task", signal); err != nil {
				slog.Info("VersionOrchestrator not running yet, will be started when version is created",
					"version_id", versionID, "task_id", taskID)
			}
		}
	}()

	// Save system message
	meta, _ := json.Marshal(map[string]string{
		"workflow_id": we.GetID(),
		"run_id":      we.GetRunID(),
	})
	rawMeta := json.RawMessage(meta)
	sysMsg := &Conversation{
		TaskID:   taskID,
		Role:     RoleSystem,
		Content:  "方案已批准，代码生成流程已启动。",
		Metadata: &rawMeta,
	}
	if err := s.repo.Create(ctx, sysMsg); err != nil {
		slog.Warn("failed to save system message", "task_id", taskID, "error", err)
	}

	return nil
}

// formatPlanForConversation converts plan result to human-readable Chinese text.
func formatPlanForConversation(plan map[string]interface{}) string {
	var parts []string

	if title, ok := plan["title"].(string); ok && title != "" {
		parts = append(parts, fmt.Sprintf("## 方案规划：%s\n", title))
	}

	if tasks, ok := plan["tasks"].([]interface{}); ok && len(tasks) > 0 {
		parts = append(parts, "**实施任务：**\n")
		for _, t := range tasks {
			if node, ok := t.(map[string]interface{}); ok {
				order, _ := node["order"].(float64)
				title, _ := node["title"].(string)
				nodeType, _ := node["type"].(string)
				hours, _ := node["estimate_hours"].(float64)
				desc, _ := node["description"].(string)

				line := fmt.Sprintf("%d. **%s** `%s` (%.1fh)", int(order), title, nodeType, hours)
				if desc != "" {
					line += "\n   " + desc
				}

				if files, ok := node["files"].([]interface{}); ok && len(files) > 0 {
					fileStrs := make([]string, 0, len(files))
					for _, f := range files {
						if s, ok := f.(string); ok {
							fileStrs = append(fileStrs, "`"+s+"`")
						}
					}
					if len(fileStrs) > 0 {
						line += "\n   文件: " + fmt.Sprintf("%s", fileStrs)
					}
				}

				if deps, ok := node["depends_on"].([]interface{}); ok && len(deps) > 0 {
					depStrs := make([]string, 0, len(deps))
					for _, d := range deps {
						depStrs = append(depStrs, fmt.Sprintf("%v", d))
					}
					line += fmt.Sprintf("\n   依赖: [%s]", fmt.Sprintf("%s", depStrs))
				}

				parts = append(parts, line)
			}
		}
	}

	var summary []string
	if riskLevel, ok := plan["risk_level"].(string); ok {
		summary = append(summary, fmt.Sprintf("**风险等级：** %s", riskLevel))
	}
	if total, ok := plan["total_estimate_hours"].(float64); ok {
		summary = append(summary, fmt.Sprintf("**总预估工时：** %.1f 小时", total))
	}
	if tracks, ok := plan["parallel_tracks"].(float64); ok {
		summary = append(summary, fmt.Sprintf("**并行通道：** %d", int(tracks)))
	}
	if len(summary) > 0 {
		parts = append(parts, "\n"+fmt.Sprintf("%s", summary[0]))
		for _, s := range summary[1:] {
			parts = append(parts, s)
		}
	}

	if factors, ok := plan["risk_factors"].([]interface{}); ok && len(factors) > 0 {
		factorLines := []string{"**风险因素：**"}
		for _, f := range factors {
			if s, ok := f.(string); ok {
				factorLines = append(factorLines, "- "+s)
			}
		}
		parts = append(parts, fmt.Sprintf("%s", factorLines[0]))
		for _, l := range factorLines[1:] {
			parts = append(parts, l)
		}
	}

	parts = append(parts, "\n请审查方案，确认后将开始代码生成。")

	return fmt.Sprintf("%s", joinParts(parts))
}

func joinParts(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "\n"
		}
		result += p
	}
	return result
}

// GetHistory returns the full conversation history for a task.
func (s *Service) GetHistory(ctx context.Context, taskID int64) ([]*Conversation, error) {
	convs, err := s.repo.ListByTaskID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if convs == nil {
		convs = []*Conversation{}
	}
	return convs, nil
}

// CreateInitialMessage saves the task requirement as the first user message.
func (s *Service) CreateInitialMessage(ctx context.Context, taskID int64, content string) error {
	msg := &Conversation{
		TaskID:  taskID,
		Role:    RoleUser,
		Content: content,
	}
	return s.repo.Create(ctx, msg)
}

// TriggerAnalysis triggers AI analysis without saving a new user message.
// Used when the initial message already exists and we just need to trigger AI.
// Returns immediately — result delivered via SSE.
func (s *Service) TriggerAnalysis(ctx context.Context, projectID, taskID, tenantID int64) (*SendMessageResponse, error) {
	t, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	if t.ProjectID != projectID {
		return nil, fmt.Errorf("task does not belong to project")
	}

	// Update task status to ANALYZING if still SUBMITTED
	if t.Status == task.StatusSubmitted {
		if err := s.taskRepo.UpdateStatus(ctx, taskID, task.StatusAnalyzing); err != nil {
			slog.Warn("failed to update task status to ANALYZING", "task_id", taskID, "error", err)
		}
		if err := s.taskRepo.UpdateStepStatus(ctx, taskID, task.StepTypeAnalyze, task.StepRunning); err != nil {
			slog.Warn("failed to mark ANALYZE step running", "task_id", taskID, "error", err)
		}
	}

	if s.temporalClient != nil {
		history, err := s.repo.ListByTaskID(ctx, taskID)
		if err != nil {
			slog.Warn("failed to load conversation history", "task_id", taskID, "error", err)
		}
		messages := make([]map[string]interface{}, 0, len(history))
		for _, h := range history {
			msg := map[string]interface{}{
				"role":    h.Role,
				"content": h.Content,
			}
			if h.Role == RoleAssistant && h.Metadata != nil {
				var meta map[string]interface{}
				if err := json.Unmarshal(*h.Metadata, &meta); err == nil {
					msg["metadata"] = meta
				}
			}
			messages = append(messages, msg)
		}

		actInput := map[string]interface{}{
			"project_id":           projectID,
			"task_id":              taskID,
			"requirement":          t.Requirement,
			"conversation_history": messages,
		}

		workflowOpts := client.StartWorkflowOptions{
			ID:        fmt.Sprintf("analyze-%d-init", taskID),
			TaskQueue: "forge-task-queue",
		}

		we, err := s.temporalClient.ExecuteWorkflow(ctx, workflowOpts, "AnalyzeRequirementWorkflow", actInput)
		if err != nil {
			slog.Warn("failed to trigger AI analysis", "task_id", taskID, "error", err)
		} else {
			go s.waitAndBroadcastAnalysis(taskID, we)
		}
	}

	return &SendMessageResponse{
		Status: "analyzing",
	}, nil
}

// derefStr safely dereferences a *string, returning "" if nil.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// buildPlanRequirement creates a rich, structured requirement text from confirmed
// analysis metadata. This gives the planner detailed context (functional requirements,
// acceptance criteria, non-functional constraints) instead of just raw user input.
func buildPlanRequirement(meta map[string]interface{}, fallback string) string {
	var b strings.Builder

	if summary, ok := meta["summary"].(string); ok && summary != "" {
		b.WriteString("## 需求概述\n")
		b.WriteString(summary)
		b.WriteString("\n\n")
	} else {
		b.WriteString("## 原始需求\n")
		b.WriteString(fallback)
		b.WriteString("\n\n")
	}

	if reqs, ok := meta["functional_requirements"].([]interface{}); ok && len(reqs) > 0 {
		b.WriteString("## 功能需求\n")
		for i, r := range reqs {
			if s, ok := r.(string); ok {
				fmt.Fprintf(&b, "%d. %s\n", i+1, s)
			}
		}
		b.WriteString("\n")
	}

	if nf, ok := meta["non_functional"].(map[string]interface{}); ok {
		b.WriteString("## 非功能需求\n")
		for k, v := range nf {
			if s, ok := v.(string); ok && s != "" {
				fmt.Fprintf(&b, "- %s: %s\n", k, s)
			}
		}
		b.WriteString("\n")
	}

	if ac, ok := meta["acceptance_criteria"].([]interface{}); ok && len(ac) > 0 {
		b.WriteString("## 验收标准\n")
		for i, a := range ac {
			if s, ok := a.(string); ok {
				fmt.Fprintf(&b, "%d. %s\n", i+1, s)
			}
		}
		b.WriteString("\n")
	}

	if oos, ok := meta["out_of_scope"].([]interface{}); ok && len(oos) > 0 {
		b.WriteString("## 不在范围内\n")
		for _, o := range oos {
			if s, ok := o.(string); ok {
				fmt.Fprintf(&b, "- %s\n", s)
			}
		}
		b.WriteString("\n")
	}

	if modules, ok := meta["affected_modules"].([]interface{}); ok && len(modules) > 0 {
		b.WriteString("## 影响模块\n")
		names := make([]string, 0, len(modules))
		for _, m := range modules {
			if s, ok := m.(string); ok {
				names = append(names, s)
			}
		}
		b.WriteString(strings.Join(names, ", "))
		b.WriteString("\n\n")
	}

	if complexity, ok := meta["estimated_complexity"].(string); ok && complexity != "" {
		fmt.Fprintf(&b, "预估复杂度: %s\n", complexity)
	}

	result := b.String()
	if result == "" {
		return fallback
	}
	return result
}
