package temporal

import (
	"context"
	"fmt"
	"log/slog"

	"go.temporal.io/sdk/client"

	"github.com/shulex/forge/forge-core/internal/temporal/activity"
)

const (
	TaskQueueName = "forge-task-queue"
	Namespace     = "default"
)

type Client struct {
	inner client.Client
}

func NewClient(ctx context.Context, hostPort string) (*Client, error) {
	c, err := client.Dial(client.Options{
		HostPort:  hostPort,
		Namespace: Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("dial temporal: %w", err)
	}
	slog.Info("temporal connected", "host", hostPort)
	return &Client{inner: c}, nil
}

func (c *Client) Inner() client.Client {
	return c.inner
}

func (c *Client) Close() {
	c.inner.Close()
}

// StartTaskWorkflow implements task.WorkflowStarter interface.
func (c *Client) StartTaskWorkflow(ctx context.Context, taskID, tenantID, projectID int64) (string, string, error) {
	workflowID := fmt.Sprintf("task-%d", taskID)

	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: TaskQueueName,
	}

	we, err := c.inner.ExecuteWorkflow(ctx, options, "TaskWorkflow", activity.TaskWorkflowInput{
		TaskID:    taskID,
		TenantID:  tenantID,
		ProjectID: projectID,
	})
	if err != nil {
		return "", "", fmt.Errorf("start task workflow: %w", err)
	}

	slog.Info("workflow started", "workflow_id", we.GetID(), "run_id", we.GetRunID(), "task_id", taskID)
	return we.GetID(), we.GetRunID(), nil
}

// TaskWorkflowInput is defined in activity package to avoid circular imports.
// Re-exported here for convenience.
