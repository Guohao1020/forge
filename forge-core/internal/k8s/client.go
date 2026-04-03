package k8s

import (
	"context"
	"fmt"
	"log/slog"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps a Kubernetes clientset for job management.
type Client struct {
	clientset *kubernetes.Clientset
}

// NewClient creates a K8s client from the given kubeconfig path.
// It verifies the connection by fetching the server version.
func NewClient(kubeconfigPath string) (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("build k8s config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create k8s clientset: %w", err)
	}
	// Verify connection
	ver, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("k8s connection failed: %w", err)
	}
	slog.Info("k8s connected", "server", config.Host, "version", ver.GitVersion)
	return &Client{clientset: clientset}, nil
}

// CreateJob creates a K8s Job in the forge-jobs namespace.
func (c *Client) CreateJob(ctx context.Context, name string, image string, command []string, env map[string]string, timeoutSeconds int64) error {
	envVars := make([]corev1.EnvVar, 0, len(env))
	for k, v := range env {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	backoffLimit := int32(0)
	ttl := int32(300) // cleanup after 5 min

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "forge-jobs",
			Labels: map[string]string{
				"app":       "forge",
				"component": "task-job",
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			ActiveDeadlineSeconds:   &timeoutSeconds,
			BackoffLimit:            &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "worker",
							Image:   image,
							Command: command,
							Env:     envVars,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2"),
									corev1.ResourceMemory: resource.MustParse("4Gi"),
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := c.clientset.BatchV1().Jobs("forge-jobs").Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	slog.Info("k8s job created", "name", name, "namespace", "forge-jobs")
	return nil
}

// GetJobStatus returns the status of a K8s Job as one of:
// SUCCEEDED, FAILED, RUNNING, PENDING.
func (c *Client) GetJobStatus(ctx context.Context, name string) (string, error) {
	job, err := c.clientset.BatchV1().Jobs("forge-jobs").Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get job: %w", err)
	}
	if job.Status.Succeeded > 0 {
		return "SUCCEEDED", nil
	}
	if job.Status.Failed > 0 {
		return "FAILED", nil
	}
	if job.Status.Active > 0 {
		return "RUNNING", nil
	}
	return "PENDING", nil
}

// GetJobLogs returns the logs from the first pod of a K8s Job.
func (c *Client) GetJobLogs(ctx context.Context, name string) (string, error) {
	pods, err := c.clientset.CoreV1().Pods("forge-jobs").List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", name),
	})
	if err != nil {
		return "", fmt.Errorf("list job pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for job %s", name)
	}
	logBytes, err := c.clientset.CoreV1().Pods("forge-jobs").
		GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{}).
		Do(ctx).Raw()
	if err != nil {
		return "", fmt.Errorf("get pod logs: %w", err)
	}
	return string(logBytes), nil
}

// DeleteJob deletes a K8s Job and its pods.
func (c *Client) DeleteJob(ctx context.Context, name string) error {
	propagation := metav1.DeletePropagationForeground
	return c.clientset.BatchV1().Jobs("forge-jobs").Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
}

// CreateNamespace creates a namespace with the given labels (for tenant environments).
func (c *Client) CreateNamespace(ctx context.Context, name string, labels map[string]string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
	_, err := c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create namespace: %w", err)
	}
	return nil
}

// DeleteNamespace deletes a namespace.
func (c *Client) DeleteNamespace(ctx context.Context, name string) error {
	return c.clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
}
