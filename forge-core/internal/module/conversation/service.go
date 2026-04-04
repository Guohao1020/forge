package conversation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

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
}

type Service struct {
	repo           *Repository
	taskRepo       TaskRepo
	temporalClient client.Client // nil if Temporal unavailable
}

func NewService(repo *Repository, taskRepo TaskRepo, tc client.Client) *Service {
	return &Service{
		repo:           repo,
		taskRepo:       taskRepo,
		temporalClient: tc,
	}
}

// SendMessage saves user message, triggers AI analysis activity, saves AI response.
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

	// Call AI worker via Temporal workflow wrapping analyze_requirement activity
	aiResponse := "AI 服务当前不可用，请确认 Temporal 和 AI Worker 已启动运行。"
	aiStatus := "clarify"
	var aiMetadata map[string]interface{}
	if s.temporalClient != nil {
		// Load conversation history for context
		history, err := s.repo.ListByTaskID(ctx, taskID)
		if err != nil {
			slog.Warn("failed to load conversation history", "task_id", taskID, "error", err)
		}
		messages := make([]map[string]interface{}, 0, len(history))
		for _, h := range history {
			messages = append(messages, map[string]interface{}{
				"role":    h.Role,
				"content": h.Content,
			})
		}

		actInput := map[string]interface{}{
			"project_id":           projectID,
			"task_id":              taskID,
			"requirement":          content,
			"conversation_history": messages,
		}

		// Start a lightweight wrapper workflow that calls the Python activity
		workflowOpts := client.StartWorkflowOptions{
			ID:        fmt.Sprintf("analyze-%d-%d", taskID, userMsg.ID),
			TaskQueue: "forge-task-queue", // Go worker queue — runs the wrapper workflow
		}

		we, err := s.temporalClient.ExecuteWorkflow(ctx, workflowOpts, "AnalyzeRequirementWorkflow", actInput)
		if err != nil {
			slog.Warn("failed to trigger AI analysis", "task_id", taskID, "error", err)
		} else {
			// Wait for the result synchronously (with timeout)
			waitCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
			defer cancel()

			var result map[string]interface{}
			if err := we.Get(waitCtx, &result); err != nil {
				slog.Warn("AI analysis failed", "task_id", taskID, "error", err)
				aiResponse = "AI 分析遇到问题，请稍后重试。错误: " + err.Error()
			} else {
				// Extract AI response content
				if c, ok := result["content"].(string); ok && c != "" {
					aiResponse = c
				}
				// Extract status and metadata from AI result
				if status, ok := result["status"].(string); ok && status != "" {
					aiStatus = status
				}
				if md, ok := result["metadata"].(map[string]interface{}); ok {
					aiMetadata = md
				}
				// Extract risks from AI result
				if risks, ok := result["risks"]; ok {
					if aiMetadata == nil {
						aiMetadata = make(map[string]interface{})
					}
					aiMetadata["risks"] = risks
				}
				// If confirmed, update task analysis
				if aiStatus == "confirmed" {
					if metadata, err := json.Marshal(aiMetadata); err == nil {
						if err := s.taskRepo.UpdateAnalysis(ctx, taskID, string(metadata)); err != nil {
							slog.Warn("failed to update task analysis", "task_id", taskID, "error", err)
						}
					}
				}
			}
		}
	}

	// Save AI response (with metadata for options/phase/risks)
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
	if err := s.repo.Create(ctx, assistantMsg); err != nil {
		return nil, fmt.Errorf("save assistant message: %w", err)
	}

	return &SendMessageResponse{
		Conversation: assistantMsg,
		Status:       aiStatus,
		Metadata:     aiMetadata,
	}, nil
}

// ConfirmPlan confirms requirements, runs PlanOnlyWorkflow, and returns the plan for review.
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

	// Mark ANALYZE step as COMPLETED
	if err := s.taskRepo.UpdateStepStatus(ctx, taskID, task.StepTypeAnalyze, task.StepCompleted); err != nil {
		slog.Warn("failed to mark ANALYZE completed", "task_id", taskID, "error", err)
	}

	// Start PlanOnlyWorkflow — generates plan and returns it
	workflowID := fmt.Sprintf("plan-%d", taskID)
	we, err := s.temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "forge-task-queue",
	}, "PlanOnlyWorkflow", activity.TaskWorkflowInput{
		TaskID:      taskID,
		TenantID:    tenantID,
		ProjectID:   t.ProjectID,
		CreatedBy:   t.CreatedBy,
		Requirement: t.Requirement,
		Title:       derefStr(t.Title),
	})
	if err != nil {
		return nil, fmt.Errorf("start plan workflow: %w", err)
	}

	// Wait for plan result (up to 5 minutes)
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var planResult map[string]interface{}
	if err := we.Get(waitCtx, &planResult); err != nil {
		return nil, fmt.Errorf("plan generation failed: %w", err)
	}

	// Format plan as human-readable text for the conversation
	planText := formatPlanForConversation(planResult)
	planMeta, _ := json.Marshal(planResult)
	raw := json.RawMessage(planMeta)
	assistantMsg := &Conversation{
		TaskID:   taskID,
		Role:     RoleAssistant,
		Content:  planText,
		Metadata: &raw,
	}
	if err := s.repo.Create(ctx, assistantMsg); err != nil {
		slog.Warn("failed to save plan message", "task_id", taskID, "error", err)
	}

	return &PlanConfirmResponse{
		Conversation: assistantMsg,
		Status:       "plan_review",
		PlanData:     planResult,
	}, nil
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
		PlanResult:  planResult,
	})
	if err != nil {
		return fmt.Errorf("start execution workflow: %w", err)
	}

	if err := s.taskRepo.UpdateWorkflowIDs(ctx, taskID, we.GetID(), we.GetRunID()); err != nil {
		slog.Error("failed to save workflow IDs", "task_id", taskID, "error", err)
	}

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

	// Call AI worker — same logic as SendMessage but without saving user message
	aiResponse := "AI 服务当前不可用，请确认 Temporal 和 AI Worker 已启动运行。"
	aiStatus := "clarify"
	var aiMetadata map[string]interface{}
	if s.temporalClient != nil {
		history, err := s.repo.ListByTaskID(ctx, taskID)
		if err != nil {
			slog.Warn("failed to load conversation history", "task_id", taskID, "error", err)
		}
		messages := make([]map[string]interface{}, 0, len(history))
		for _, h := range history {
			messages = append(messages, map[string]interface{}{
				"role":    h.Role,
				"content": h.Content,
			})
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
			waitCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
			defer cancel()

			var result map[string]interface{}
			if err := we.Get(waitCtx, &result); err != nil {
				slog.Warn("AI analysis failed", "task_id", taskID, "error", err)
				aiResponse = "AI 分析遇到问题，请稍后重试。错误: " + err.Error()
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
		}
	}

	// Save AI response (with metadata for options/phase/risks)
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
	if err := s.repo.Create(ctx, assistantMsg); err != nil {
		return nil, fmt.Errorf("save assistant message: %w", err)
	}

	return &SendMessageResponse{
		Conversation: assistantMsg,
		Status:       aiStatus,
		Metadata:     aiMetadata,
	}, nil
}

// derefStr safely dereferences a *string, returning "" if nil.
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
