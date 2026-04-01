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

	// Update task status to ANALYZING if still SUBMITTED
	if t.Status == task.StatusSubmitted {
		_ = s.taskRepo.UpdateStatus(ctx, taskID, task.StatusAnalyzing)
	}

	// Call AI worker via Temporal workflow wrapping analyze_requirement activity
	aiResponse := "AI 分析功能即将上线，当前为占位响应。您的需求已记录。"
	aiStatus := "clarify"
	var aiMetadata map[string]interface{}
	if s.temporalClient != nil {
		// Load conversation history for context
		history, _ := s.repo.ListByTaskID(ctx, taskID)
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
				// If confirmed, update task analysis
				if aiStatus == "confirmed" {
					if metadata, err := json.Marshal(aiMetadata); err == nil {
						_ = s.taskRepo.UpdateAnalysis(ctx, taskID, string(metadata))
					}
				}
			}
		}
	}

	// Save AI response
	assistantMsg := &Conversation{
		TaskID:  taskID,
		Role:    RoleAssistant,
		Content: aiResponse,
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

// ConfirmPlan confirms the AI analysis and starts the task generation workflow.
func (s *Service) ConfirmPlan(ctx context.Context, taskID, tenantID int64) error {
	t, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// Check that analysis has been done (task has moved past SUBMITTED)
	if t.Status == task.StatusSubmitted {
		return fmt.Errorf("task has no analysis yet — send a message first")
	}

	if s.temporalClient == nil {
		return fmt.Errorf("temporal not available")
	}

	// Start the full task generation workflow
	workflowID := fmt.Sprintf("task-%d", taskID)
	we, err := s.temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "forge-task-queue",
	}, "TaskWorkflow", activity.TaskWorkflowInput{
		TaskID:      taskID,
		TenantID:    tenantID,
		ProjectID:   t.ProjectID,
		Requirement: t.Requirement,
	})
	if err != nil {
		return fmt.Errorf("start generation workflow: %w", err)
	}

	if err := s.taskRepo.UpdateWorkflowIDs(ctx, taskID, we.GetID(), we.GetRunID()); err != nil {
		slog.Error("failed to save workflow IDs", "task_id", taskID, "error", err)
	}

	// Save system message about plan confirmation
	meta, _ := json.Marshal(map[string]string{
		"workflow_id": we.GetID(),
		"run_id":      we.GetRunID(),
	})
	raw := json.RawMessage(meta)
	sysMsg := &Conversation{
		TaskID:   taskID,
		Role:     RoleSystem,
		Content:  "方案已确认，代码生成流程已启动。",
		Metadata: &raw,
	}
	_ = s.repo.Create(ctx, sysMsg)

	return nil
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
