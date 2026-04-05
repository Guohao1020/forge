package temporal

import "testing"

func TestTaskQueueName(t *testing.T) {
	if TaskQueueName == "" {
		t.Error("TaskQueueName should not be empty")
	}
	if TaskQueueName != "forge-task-queue" {
		t.Errorf("expected 'forge-task-queue', got %s", TaskQueueName)
	}
}

func TestNamespace(t *testing.T) {
	if Namespace == "" {
		t.Error("Namespace should not be empty")
	}
	if Namespace != "default" {
		t.Errorf("expected 'default', got %s", Namespace)
	}
}
