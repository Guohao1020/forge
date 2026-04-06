package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/activity"

	"github.com/shulex/forge/forge-core/internal/k8s"
	"github.com/shulex/forge/forge-core/internal/module/task"
)

// DeployActivities handles K8s deployment operations.
type DeployActivities struct {
	db  *pgxpool.Pool
	k8s *k8s.Client // may be nil
	sse *task.SSEHub
}

func NewDeployActivities(db *pgxpool.Pool, k8sClient *k8s.Client, sse *task.SSEHub) *DeployActivities {
	return &DeployActivities{db: db, k8s: k8sClient, sse: sse}
}

// GenerateManifestInput is the input for K8s manifest generation.
type GenerateManifestInput struct {
	TaskID      int64  `json:"task_id"`
	TenantID    int64  `json:"tenant_id"`
	ProjectID   int64  `json:"project_id"`
	ProjectName string `json:"project_name"`
	ImageURL    string `json:"image_url"`
	Environment string `json:"environment"` // dev, staging, prod
	Port        int    `json:"port"`        // container port (default 8080)
	Replicas    int    `json:"replicas"`    // default 1 for dev, 2 for staging, 3 for prod
}

// GenerateManifestOutput contains the generated K8s YAML manifests.
type GenerateManifestOutput struct {
	Namespace  string `json:"namespace"`
	Deployment string `json:"deployment_yaml"`
	Service    string `json:"service_yaml"`
	Ingress    string `json:"ingress_yaml"`
	ConfigMap  string `json:"configmap_yaml"`
	DeployID   int64  `json:"deploy_id"`
}

// GenerateK8sManifests creates K8s resource YAML files for deployment.
func (a *DeployActivities) GenerateK8sManifests(ctx context.Context, input GenerateManifestInput) (*GenerateManifestOutput, error) {
	info := activity.GetInfo(ctx)
	slog.Info("GenerateK8sManifests",
		"project", input.ProjectName,
		"env", input.Environment,
		"image", input.ImageURL,
		"workflow_id", info.WorkflowExecution.ID,
	)

	if input.Port == 0 {
		input.Port = 8080
	}
	if input.Replicas == 0 {
		switch input.Environment {
		case "prod":
			input.Replicas = 3
		case "staging":
			input.Replicas = 2
		default:
			input.Replicas = 1
		}
	}

	appName := sanitizeK8sName(input.ProjectName)
	namespace := fmt.Sprintf("tenant-%d-%s", input.TenantID, strings.ToLower(input.Environment))

	// Resource limits per environment
	cpuLimit, memLimit := "500m", "512Mi"
	cpuRequest, memRequest := "100m", "128Mi"
	if input.Environment == "staging" {
		cpuLimit, memLimit = "1000m", "1Gi"
		cpuRequest, memRequest = "250m", "256Mi"
	} else if input.Environment == "prod" {
		cpuLimit, memLimit = "2000m", "2Gi"
		cpuRequest, memRequest = "500m", "512Mi"
	}

	// --- Deployment ---
	deployment := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    forge.dev/project-id: "%d"
    forge.dev/task-id: "%d"
    forge.dev/env: "%s"
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    metadata:
      labels:
        app: %s
    spec:
      containers:
      - name: %s
        image: %s
        ports:
        - containerPort: %d
        resources:
          requests:
            cpu: %s
            memory: %s
          limits:
            cpu: %s
            memory: %s
        livenessProbe:
          httpGet:
            path: /healthz
            port: %d
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /readyz
            port: %d
          initialDelaySeconds: 5
          periodSeconds: 10
        env:
        - name: ENV
          value: "%s"
        - name: PORT
          value: "%d"`,
		appName, namespace, appName, input.ProjectID, input.TaskID, input.Environment,
		input.Replicas, appName, appName, appName, input.ImageURL, input.Port,
		cpuRequest, memRequest, cpuLimit, memLimit,
		input.Port, input.Port, input.Environment, input.Port,
	)

	// --- Service ---
	service := fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
spec:
  selector:
    app: %s
  ports:
  - port: 80
    targetPort: %d
    protocol: TCP
  type: ClusterIP`,
		appName, namespace, appName, input.Port,
	)

	// --- Ingress ---
	baseDomain := getEnvOrDefault("FORGE_BASE_DOMAIN", "shulex.com")
	host := fmt.Sprintf("forge-%s-%s.%s", appName, input.Environment, baseDomain)
	ingress := fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %s
  namespace: %s
  annotations:
    kubernetes.io/ingress.class: traefik
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  rules:
  - host: %s
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: %s
            port:
              number: 80
  tls:
  - hosts:
    - %s
    secretName: %s-tls`,
		appName, namespace, host, appName, host, appName,
	)

	// --- ConfigMap ---
	configMap := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s-config
  namespace: %s
data:
  APP_ENV: "%s"
  APP_PORT: "%d"
  LOG_LEVEL: "info"`,
		appName, namespace, input.Environment, input.Port,
	)

	// Save deploy record
	manifests := map[string]string{
		"deployment": deployment,
		"service":    service,
		"ingress":    ingress,
		"configmap":  configMap,
	}
	manifestJSON, _ := json.Marshal(manifests)

	var deployID int64
	err := a.db.QueryRow(ctx,
		`INSERT INTO pipeline.deploy_records (tenant_id, environment_id, version, status, metadata, deployed_at)
		 VALUES ($1, (SELECT id FROM pipeline.environments WHERE project_id = $2 AND env_type = $3 LIMIT 1),
		         $4, 'PENDING', $5::jsonb, NOW())
		 RETURNING id`,
		input.TenantID, input.ProjectID, strings.ToUpper(input.Environment),
		input.ImageURL, string(manifestJSON),
	).Scan(&deployID)
	if err != nil {
		// If no environment exists yet, create one
		slog.Warn("deploy record insert failed (may need environment), creating fallback",
			"error", err, "project_id", input.ProjectID,
		)
		// Create deploy record without environment_id
		err = a.db.QueryRow(ctx,
			`INSERT INTO pipeline.deploy_records (tenant_id, version, status, metadata, deployed_at)
			 VALUES ($1, $2, 'PENDING', $3::jsonb, NOW())
			 RETURNING id`,
			input.TenantID, input.ImageURL, string(manifestJSON),
		).Scan(&deployID)
		if err != nil {
			return nil, fmt.Errorf("save deploy record: %w", err)
		}
	}

	return &GenerateManifestOutput{
		Namespace:  namespace,
		Deployment: deployment,
		Service:    service,
		Ingress:    ingress,
		ConfigMap:  configMap,
		DeployID:   deployID,
	}, nil
}

// RollbackInput is the input for rollback.
type RollbackInput struct {
	TenantID    int64 `json:"tenant_id"`
	ProjectID   int64 `json:"project_id"`
	DeployID    int64 `json:"deploy_id"` // The deploy to rollback TO (previous version)
}

// Rollback reverts to a previous deployment version.
func (a *DeployActivities) Rollback(ctx context.Context, input RollbackInput) error {
	slog.Info("Rollback", "deploy_id", input.DeployID, "project_id", input.ProjectID)

	// Get the previous deploy's manifests
	var metadata json.RawMessage
	err := a.db.QueryRow(ctx,
		`SELECT metadata FROM pipeline.deploy_records WHERE id = $1 AND tenant_id = $2`,
		input.DeployID, input.TenantID,
	).Scan(&metadata)
	if err != nil {
		return fmt.Errorf("get rollback target: %w", err)
	}

	// TODO: When K8s client is available:
	// 1. Parse manifests from metadata
	// 2. kubectl apply -f each manifest
	// 3. Wait for rollout status
	// 4. Update deploy record status

	// For now: create a new deploy record pointing to the rollback target
	_, err = a.db.Exec(ctx,
		`INSERT INTO pipeline.deploy_records (tenant_id, version, status, metadata, deployed_at)
		 VALUES ($1, (SELECT version FROM pipeline.deploy_records WHERE id = $2), 'ROLLED_BACK', $3::jsonb, NOW())`,
		input.TenantID, input.DeployID, string(metadata),
	)
	if err != nil {
		return fmt.Errorf("create rollback record: %w", err)
	}

	slog.Info("Rollback completed", "target_deploy_id", input.DeployID)
	return nil
}

// AutoDeployInput is the input for the AutoDeployToDev activity.
type AutoDeployInput struct {
	TenantID   int64  `json:"tenant_id"`
	ProjectID  int64  `json:"project_id"`
	TaskID     int64  `json:"task_id"`
	ArtifactID int64  `json:"artifact_id"`
	ImageURL   string `json:"image_url"`
	Version    string `json:"version"`
	CreatedBy  int64  `json:"created_by"`
}

// AutoDeployOutput is the result of the AutoDeployToDev activity.
type AutoDeployOutput struct {
	DeployRecordID int64  `json:"deploy_record_id"`
	EnvironmentID  int64  `json:"environment_id"`
	Status         string `json:"status"`
}

// AutoDeployToDev automatically deploys to the DEV environment after task completion.
func (a *DeployActivities) AutoDeployToDev(ctx context.Context, input AutoDeployInput) (*AutoDeployOutput, error) {
	info := activity.GetInfo(ctx)
	slog.Info("AutoDeployToDev", "task_id", input.TaskID, "project_id", input.ProjectID, "workflow_id", info.WorkflowExecution.ID)

	// Find DEV environment for this project
	var envID int64
	err := a.db.QueryRow(ctx,
		`SELECT id FROM pipeline.environments WHERE project_id = $1 AND tenant_id = $2 AND env_type = 'DEV'`,
		input.ProjectID, input.TenantID,
	).Scan(&envID)
	if err != nil {
		slog.Warn("no DEV environment found, skipping auto-deploy", "project_id", input.ProjectID)
		return &AutoDeployOutput{Status: "SKIPPED"}, nil
	}

	// Create deploy record
	version := input.Version
	if version == "" {
		version = input.ImageURL
	}

	var deployID int64
	err = a.db.QueryRow(ctx,
		`INSERT INTO pipeline.deploy_records (tenant_id, project_id, environment_id, artifact_id, version, status, deployed_by)
		 VALUES ($1, $2, $3, $4, $5, 'PENDING', $6)
		 RETURNING id`,
		input.TenantID, input.ProjectID, envID, input.ArtifactID, version, input.CreatedBy,
	).Scan(&deployID)
	if err != nil {
		return nil, fmt.Errorf("create deploy record: %w", err)
	}

	// Real K8s deployment when client is available
	if a.k8s != nil && input.ImageURL != "" {
		namespace := fmt.Sprintf("tenant-%d-dev", input.TenantID)
		appName := fmt.Sprintf("project-%d", input.ProjectID)

		// Ensure namespace exists
		if err := a.k8s.EnsureNamespace(ctx, namespace, map[string]string{
			"app":        "forge",
			"component":  "dev-env",
			"managed-by": "forge",
		}); err != nil {
			slog.Warn("failed to ensure namespace", "namespace", namespace, "error", err)
		}

		// Create ACR pull secret
		dockerConfigJSON := []byte(`{"auths":{"repo-voc-registry-vpc.cn-hangzhou.cr.aliyuncs.com":{"username":"1652058863700531@shulex","password":"shulex123123","auth":"MTY1MjA1ODg2MzcwMDUzMUBzaHVsZXg6c2h1bGV4MTIzMTIz"}}}`)
		if err := a.k8s.EnsureImagePullSecret(ctx, namespace, "acr-secret", dockerConfigJSON); err != nil {
			slog.Warn("failed to create image pull secret", "error", err)
		}

		// Apply deployment
		if err := a.k8s.ApplyDeployment(ctx, namespace, appName, input.ImageURL,
			8080, // port
			1,    // replicas
			map[string]string{"APP_ENV": "dev", "APP_PORT": "8080"},
		); err != nil {
			slog.Warn("failed to apply deployment", "error", err)
		} else {
			// Apply service
			_ = a.k8s.ApplyService(ctx, namespace, appName, 80, 8080)

			// Ensure TLS secret for HTTPS
			tlsCertPath := getEnvOrDefault("FORGE_TLS_CERT", "k8s/tls/shulex.com.crt")
			tlsKeyPath := getEnvOrDefault("FORGE_TLS_KEY", "k8s/tls/shulex.com.key")
			certData, certErr := os.ReadFile(tlsCertPath)
			keyData, keyErr := os.ReadFile(tlsKeyPath)
			if certErr == nil && keyErr == nil {
				_ = a.k8s.EnsureTLSSecret(ctx, namespace, "forge-tls-secret", certData, keyData)
			}

			// Add path rule to shared Ingress: forge-dev.shulex.com/{project-slug}/
			baseDomain := getEnvOrDefault("FORGE_BASE_DOMAIN", "shulex.com")
			host := fmt.Sprintf("forge-dev.%s", baseDomain)
			var projectSlug string
			var projectName string
			_ = a.db.QueryRow(ctx,
				`SELECT name FROM engine.projects WHERE id = $1`, input.ProjectID,
			).Scan(&projectName)
			// Generate slug: prefer ASCII chars from name, fallback to "p{id}"
			projectSlug = sanitizeK8sName(projectName)
			if projectSlug == "" || projectSlug == "-" {
				projectSlug = fmt.Sprintf("p%d", input.ProjectID)
			}
			pathPrefix := "/" + projectSlug
			if err := a.k8s.AddIngressPathRule(ctx, namespace, "forge-dev-ingress", host, pathPrefix, appName, 8080); err != nil {
				slog.Warn("failed to add ingress path rule", "error", err)
			} else {
				slog.Info("ingress path added", "host", host, "path", pathPrefix)
			}

			// Mark as DEPLOYED (not SIMULATED)
			_, _ = a.db.Exec(ctx,
				`UPDATE pipeline.deploy_records SET status = 'DEPLOYED', completed_at = NOW() WHERE id = $1`,
				deployID,
			)

			// Update environment
			_, _ = a.db.Exec(ctx,
				`UPDATE pipeline.environments SET current_version = $1, last_deploy_at = NOW(), status = 'ACTIVE' WHERE id = $2`,
				version, envID,
			)

			// Broadcast
			if a.sse != nil {
				a.sse.Broadcast(input.TaskID, task.TaskProgressEvent{
					Type:   "deploy_completed",
					TaskID: input.TaskID,
					Status: "DEPLOYED",
					Data: map[string]string{
						"environment": "DEV",
						"version":     version,
						"deploy_id":   fmt.Sprintf("%d", deployID),
					},
				})
			}

			slog.Info("AutoDeployToDev completed (real K8s)", "deploy_id", deployID, "namespace", namespace)
			return &AutoDeployOutput{
				DeployRecordID: deployID,
				EnvironmentID:  envID,
				Status:         "DEPLOYED",
			}, nil
		}
	}

	// Fallback: Mark as SIMULATED (K8s unavailable or deployment failed)
	_, err = a.db.Exec(ctx,
		`UPDATE pipeline.deploy_records SET status = 'SIMULATED', completed_at = NOW() WHERE id = $1`,
		deployID,
	)
	if err != nil {
		slog.Warn("failed to update deploy status", "deploy_id", deployID, "error", err)
	}

	// Update environment current_version
	_, _ = a.db.Exec(ctx,
		`UPDATE pipeline.environments SET current_version = $1, last_deploy_at = NOW(), status = 'ACTIVE' WHERE id = $2`,
		version, envID,
	)

	// Broadcast SSE event
	if a.sse != nil {
		a.sse.Broadcast(input.TaskID, task.TaskProgressEvent{
			Type:   "deploy_completed",
			TaskID: input.TaskID,
			Status: "SIMULATED",
			Data: map[string]string{
				"environment": "DEV",
				"version":     version,
				"deploy_id":   fmt.Sprintf("%d", deployID),
			},
		})
	}

	slog.Info("AutoDeployToDev completed (simulated)", "deploy_id", deployID, "env_id", envID, "version", version)
	return &AutoDeployOutput{
		DeployRecordID: deployID,
		EnvironmentID:  envID,
		Status:         "SIMULATED",
	}, nil
}

func sanitizeK8sName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	// K8s names must be DNS-compatible
	var result []byte
	for _, c := range []byte(name) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result = append(result, c)
		}
	}
	if len(result) > 63 {
		result = result[:63]
	}
	return string(result)
}
