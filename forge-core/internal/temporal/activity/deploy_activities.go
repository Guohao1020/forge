package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/activity"
)

// DeployActivities handles K8s deployment operations.
type DeployActivities struct {
	db *pgxpool.Pool
}

func NewDeployActivities(db *pgxpool.Pool) *DeployActivities {
	return &DeployActivities{db: db}
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
	host := fmt.Sprintf("%s-%s.forge.example.com", appName, input.Environment)
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

