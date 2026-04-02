package task

import (
	"encoding/json"
	"time"
)

// Task status constants
const (
	StatusSubmitted    = "SUBMITTED"
	StatusAnalyzing    = "ANALYZING"
	StatusPlanning     = "PLANNING"
	StatusTestWriting  = "TEST_WRITING"
	StatusGenerating   = "GENERATING"
	StatusReviewing    = "REVIEWING"
	StatusTesting      = "TESTING"
	StatusDeploying    = "DEPLOYING"
	StatusCompleted    = "COMPLETED"
	StatusFailed       = "FAILED"
)

// Step status constants
const (
	StepPending   = "PENDING"
	StepRunning   = "RUNNING"
	StepCompleted = "COMPLETED"
	StepFailed    = "FAILED"
	StepSkipped   = "SKIPPED"
)

// Step type constants
const (
	StepTypeAnalyze      = "ANALYZE"
	StepTypePlan         = "PLAN"
	StepTypeTestWriting  = "TEST_WRITING"
	StepTypeGenerate     = "GENERATE"
	StepTypeReview       = "REVIEW"
	StepTypeTest         = "TEST"
	StepTypeDeploy       = "DEPLOY"
)

const SourceWeb = "WEB"

// AllSteps defines the default step sequence for a task workflow
var AllSteps = []struct {
	Name     string
	StepType string
}{
	{"需求分析", StepTypeAnalyze},
	{"方案规划", StepTypePlan},
	{"测试设计", StepTypeTestWriting},
	{"代码生成", StepTypeGenerate},
	{"代码审查", StepTypeReview},
	{"自动化测试", StepTypeTest},
	{"部署发布", StepTypeDeploy},
}

type Task struct {
	ID            int64      `json:"id"`
	TenantID      int64      `json:"tenant_id"`
	ProjectID     int64      `json:"project_id"`
	Title         *string    `json:"title,omitempty"`
	Requirement   string     `json:"requirement"`
	Source        string     `json:"source"`
	Status        string     `json:"status"`
	WorkflowID    *string    `json:"workflow_id,omitempty"`
	WorkflowRunID *string    `json:"workflow_run_id,omitempty"`
	RiskLevel     *string    `json:"risk_level,omitempty"`
	RiskScore     *int       `json:"risk_score,omitempty"`
	BranchName    *string    `json:"branch_name,omitempty"`
	FilesChanged  *int       `json:"files_changed,omitempty"`
	LinesAdded    *int       `json:"lines_added,omitempty"`
	LinesDeleted  *int       `json:"lines_deleted,omitempty"`
	PrNumber      *int       `json:"pr_number,omitempty" db:"pr_number"`
	MrUrl         *string    `json:"mr_url,omitempty" db:"mr_url"`
	ReviewScore   *int       `json:"review_score,omitempty" db:"review_score"`
	CreatedBy     int64      `json:"created_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

type TaskStep struct {
	ID          int64      `json:"id"`
	TaskID      int64      `json:"task_id"`
	Name        string     `json:"name"`
	StepType    string     `json:"step_type"`
	Status      string     `json:"status"`
	Input       *string    `json:"input,omitempty"`
	Output      *string    `json:"output,omitempty"`
	Error       *string    `json:"error,omitempty"`
	Attempt     int        `json:"attempt"`
	MaxAttempts int        `json:"max_attempts"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DurationMs  *int64     `json:"duration_ms,omitempty"`
}

type CreateTaskRequest struct {
	Title       string `json:"title" binding:"max=200"`
	Requirement string `json:"requirement" binding:"required,min=1,max=10000"`
}

type TaskResponse struct {
	Task  Task       `json:"task"`
	Steps []TaskStep `json:"steps,omitempty"`
}

type TaskListResponse struct {
	Tasks []Task `json:"tasks"`
	Total int64  `json:"total"`
}

// TaskNode represents a sub-task in the DAG decomposition
type TaskNode struct {
	ID             int64           `json:"id"`
	TaskID         int64           `json:"taskId"`
	NodeOrder      int             `json:"nodeOrder"`
	Title          string          `json:"title"`
	Description    *string         `json:"description,omitempty"`
	NodeType       string          `json:"nodeType"`
	Status         string          `json:"status"`
	DependsOn      json.RawMessage `json:"dependsOn"`
	Files          json.RawMessage `json:"files"`
	EstimateHours  *float64        `json:"estimateHours,omitempty"`
	RequirementRef *string         `json:"requirementRef,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type TaskNodeListResponse struct {
	Nodes []TaskNode `json:"nodes"`
}

type TaskProgressEvent struct {
	Type       string      `json:"type"`
	TaskID     int64       `json:"task_id"`
	Status     string      `json:"status"`
	StepType   string      `json:"step_type,omitempty"`
	StepName   string      `json:"step_name,omitempty"`
	StepStatus string      `json:"step_status,omitempty"`
	Progress   int         `json:"progress"`
	Data       interface{} `json:"data,omitempty"`
}
