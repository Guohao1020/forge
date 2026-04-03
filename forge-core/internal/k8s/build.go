package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildAndPushImage creates a K8s Job that builds a Docker image using kaniko
// and pushes it to a container registry (e.g. ACR).
// Returns the full image reference (registry/imageName:tag).
func (c *Client) BuildAndPushImage(ctx context.Context, taskID int64, repoURL, branch, registry, imageName, tag string, registryCreds map[string]string) (string, error) {
	jobName := fmt.Sprintf("build-%d", taskID)
	fullImage := fmt.Sprintf("%s/%s:%s", registry, imageName, tag)
	namespace := "forge-jobs"

	// Build docker config JSON for registry auth
	dockerConfig, err := buildDockerConfigJSON(registry, registryCreds)
	if err != nil {
		return "", fmt.Errorf("build docker config: %w", err)
	}

	// Create a Secret with registry credentials
	secretName := fmt.Sprintf("regcred-%d", taskID)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":       "forge",
				"component": "build-job",
				"task-id":   fmt.Sprintf("%d", taskID),
			},
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: dockerConfig,
		},
	}

	_, err = c.clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("create registry secret: %w", err)
	}

	backoffLimit := int32(0)
	ttl := int32(600) // cleanup after 10 min
	timeout := int64(1800) // 30 min build timeout

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":       "forge",
				"component": "build-job",
				"task-id":   fmt.Sprintf("%d", taskID),
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			ActiveDeadlineSeconds:   &timeout,
			BackoffLimit:            &backoffLimit,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "kaniko",
							Image: "gcr.io/kaniko-project/executor:latest",
							Args: []string{
								fmt.Sprintf("--dockerfile=Dockerfile"),
								fmt.Sprintf("--context=git://%s#refs/heads/%s", repoURL, branch),
								fmt.Sprintf("--destination=%s", fullImage),
								"--cache=true",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "docker-config",
									MountPath: "/kaniko/.docker",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1"),
									corev1.ResourceMemory: resource.MustParse("2Gi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2"),
									corev1.ResourceMemory: resource.MustParse("4Gi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "docker-config",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretName,
									Items: []corev1.KeyToPath{
										{
											Key:  corev1.DockerConfigJsonKey,
											Path: "config.json",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	_, err = c.clientset.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		// Cleanup secret on job creation failure
		_ = c.clientset.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		return "", fmt.Errorf("create build job: %w", err)
	}

	slog.Info("k8s build job created",
		"job", jobName,
		"task_id", taskID,
		"image", fullImage,
		"repo", repoURL,
		"branch", branch,
	)
	return fullImage, nil
}

// buildDockerConfigJSON creates a Docker config.json for registry authentication.
func buildDockerConfigJSON(registry string, creds map[string]string) ([]byte, error) {
	username := creds["username"]
	password := creds["password"]

	if username == "" || password == "" {
		return nil, fmt.Errorf("registry credentials missing username or password")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))

	dockerConfig := map[string]interface{}{
		"auths": map[string]interface{}{
			registry: map[string]string{
				"auth": auth,
			},
		},
	}

	return json.Marshal(dockerConfig)
}
