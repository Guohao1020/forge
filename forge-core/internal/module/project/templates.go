package project

// ProjectTemplate represents a starter template for new projects.
type ProjectTemplate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Language    string   `json:"language"`
	Frameworks  []string `json:"frameworks"`
	Category    string   `json:"category"` // backend, frontend, fullstack, mobile
	Tags        []string `json:"tags"`
}

// BuiltInTemplates returns the available project starter templates.
func BuiltInTemplates() []ProjectTemplate {
	return []ProjectTemplate{
		{
			ID:          "go-api",
			Name:        "Go REST API",
			Description: "Go + Gin + PostgreSQL REST API with structured logging and health checks",
			Language:    "go",
			Frameworks:  []string{"Gin", "pgx", "slog"},
			Category:    "backend",
			Tags:        []string{"go", "api", "rest", "postgresql"},
		},
		{
			ID:          "nextjs-app",
			Name:        "Next.js Web App",
			Description: "Next.js 15 + TypeScript + Tailwind CSS + shadcn/ui web application",
			Language:    "typescript",
			Frameworks:  []string{"Next.js", "React", "Tailwind CSS", "shadcn/ui"},
			Category:    "frontend",
			Tags:        []string{"nextjs", "react", "typescript", "tailwind"},
		},
		{
			ID:          "python-service",
			Name:        "Python FastAPI Service",
			Description: "Python + FastAPI + SQLAlchemy + Alembic microservice",
			Language:    "python",
			Frameworks:  []string{"FastAPI", "SQLAlchemy", "Alembic", "Pydantic"},
			Category:    "backend",
			Tags:        []string{"python", "fastapi", "api"},
		},
		{
			ID:          "fullstack-go-next",
			Name:        "Go + Next.js Full Stack",
			Description: "Go API backend + Next.js frontend monorepo",
			Language:    "go",
			Frameworks:  []string{"Gin", "Next.js", "PostgreSQL", "Docker"},
			Category:    "fullstack",
			Tags:        []string{"go", "nextjs", "fullstack", "monorepo"},
		},
		{
			ID:          "python-ml",
			Name:        "Python ML Pipeline",
			Description: "Python + PyTorch/TensorFlow ML training and inference pipeline",
			Language:    "python",
			Frameworks:  []string{"PyTorch", "FastAPI", "MLflow"},
			Category:    "backend",
			Tags:        []string{"python", "ml", "pytorch", "ai"},
		},
		{
			ID:          "java-spring",
			Name:        "Java Spring Boot API",
			Description: "Java 21 + Spring Boot 3 + MyBatis + MySQL REST API",
			Language:    "java",
			Frameworks:  []string{"Spring Boot", "MyBatis", "MySQL"},
			Category:    "backend",
			Tags:        []string{"java", "spring", "api", "mysql"},
		},
		{
			ID:          "flutter-app",
			Name:        "Flutter Mobile App",
			Description: "Flutter cross-platform mobile app with Provider state management",
			Language:    "dart",
			Frameworks:  []string{"Flutter", "Provider", "Dio"},
			Category:    "mobile",
			Tags:        []string{"flutter", "dart", "mobile", "ios", "android"},
		},
	}
}
