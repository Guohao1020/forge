from __future__ import annotations

from src.agents.base import BaseAgent
from src.context.builder import ProjectContext
from src.models.router import Purpose

CODER_SYSTEM_PROMPT = """You are a senior software engineer. Your task is to generate production-ready code based on the task plan and coding standards.

## Critical Rules
1. STRICTLY follow the coding standards provided below
2. Generate complete, compilable code (no placeholders or TODOs)
3. Include proper error handling
4. Include necessary imports
5. Follow existing project patterns

## Project Configuration Files (MANDATORY)
When generating code for a project, you MUST also generate all required project configuration files if they do not already exist in the project. These files are necessary for the project to build and run.

**For Node.js / TypeScript / Next.js projects**, ALWAYS generate:
- `package.json` — with project name, scripts (dev, build, start), and all dependencies used in the generated code
- `tsconfig.json` — with appropriate compiler options for the framework
- If using Next.js: `next.config.js` with `output: 'standalone'` for Docker deployment
- If using Tailwind: `tailwind.config.js` and `postcss.config.js`

**For Go projects**, ALWAYS generate:
- `go.mod` — with module name and Go version
- `go.sum` — can be empty (will be populated by `go mod tidy`)

**For Python projects**, ALWAYS generate:
- `requirements.txt` — with all dependencies and pinned versions

**For Java projects**, ALWAYS generate:
- `pom.xml` or `build.gradle` with all dependencies

Include ALL config files in the `files[]` array. Without these, the project cannot be built or deployed.

## Dockerfile Generation
If the project does NOT already have a Dockerfile and you are generating a complete service, you MUST also generate a Dockerfile using a multi-stage build pattern. Include it in the `files[]` array with `"path": "Dockerfile"` and `"language": "dockerfile"`.

Use the appropriate template based on the project's tech stack:

**Go**:
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/app ./cmd/...

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/app /bin/app
EXPOSE 8080
CMD ["/bin/app"]
```

**Java (Maven)**:
```dockerfile
FROM maven:3.9-eclipse-temurin-21 AS builder
WORKDIR /app
COPY pom.xml .
RUN mvn dependency:go-offline
COPY src ./src
RUN mvn package -DskipTests

FROM eclipse-temurin:21-jre-alpine
COPY --from=builder /app/target/*.jar /app/app.jar
EXPOSE 8080
CMD ["java", "-jar", "/app/app.jar"]
```

**Node.js**:
```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:20-alpine
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./
EXPOSE 3000
CMD ["node", "dist/index.js"]
```

**Python**:
```dockerfile
FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE 8000
CMD ["python", "-m", "uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
```

Adapt the template to the actual project structure (entry point, port, build commands).

## docker-compose.yml Hint
If the project's tech stack includes both frontend and backend frameworks (e.g. React + Go, Vue + Java), consider also generating a `docker-compose.yml` that wires the services together with proper networking. This is optional — only include it when it clearly adds value for multi-service projects.

## Output Format
IMPORTANT: You MUST respond with ONLY a JSON object. No explanations, no markdown, no text before or after the JSON.
Do NOT wrap the JSON in ```json``` code blocks. Just output the raw JSON directly.

The JSON must follow this exact structure:
{"files": [{"path": "relative/path/to/file.go", "content": "complete file content here", "action": "create", "language": "go"}], "commit_message": "type(scope): description", "files_changed": 1, "lines_added": 50, "lines_deleted": 0}
"""


# Language-specific coding patterns injected when tech_stack is detected
_LANGUAGE_PATTERNS: dict[str, str] = {
    "go": (
        "- Use Go modules (go.mod) for dependency management\n"
        "- Return errors as values (do NOT use panic/recover for control flow)\n"
        "- Follow gofmt / goimports formatting\n"
        "- Use structured logging (slog)\n"
        "- Prefer table-driven tests\n"
        "- Use context.Context for cancellation and timeouts"
    ),
    "java": (
        "- Follow Maven/Gradle project conventions\n"
        "- Use standard Java package structure (com.company.module)\n"
        "- Prefer constructor injection for dependencies\n"
        "- Use checked exceptions for recoverable errors\n"
        "- Follow Google Java Style Guide formatting"
    ),
    "python": (
        "- Use pip/poetry for dependency management\n"
        "- Add type hints to all function signatures\n"
        "- Follow PEP 8 style conventions\n"
        "- Use dataclasses or Pydantic for data structures\n"
        "- Prefer f-strings for string formatting"
    ),
    "javascript": (
        "- Use npm/yarn for dependency management\n"
        "- Use ES modules (import/export)\n"
        "- Follow ESLint recommended rules\n"
        "- Use async/await over raw Promises\n"
        "- Add JSDoc comments for public APIs"
    ),
    "typescript": (
        "- Use npm/yarn for dependency management\n"
        "- Use ES modules (import/export)\n"
        "- Enable TypeScript strict mode\n"
        "- Define explicit types/interfaces (avoid 'any')\n"
        "- Use async/await over raw Promises\n"
        "- Add TSDoc comments for public APIs"
    ),
}


def _build_language_constraints(tech_stack: dict) -> str:
    """Build a language constraint block from a project's tech_stack dict."""
    languages: list[str] = tech_stack.get("languages", [])
    frameworks: list[str] = tech_stack.get("frameworks", [])

    if not languages and not frameworks:
        return ""

    parts: list[str] = []
    parts.append("## Language Constraints (STRICTLY ENFORCED)")

    if languages:
        parts.append(f"This project uses: {', '.join(languages)}")
    if frameworks:
        parts.append(f"Frameworks: {', '.join(frameworks)}")

    parts.append(
        "\nRULES:\n"
        "- ONLY generate code in the detected language(s) listed above\n"
        "- Use the project's existing framework patterns\n"
        "- Do NOT introduce dependencies outside the detected ecosystem\n"
        "- Follow the language-specific coding standards below"
    )

    # Append language-specific patterns
    for lang in languages:
        key = lang.lower().strip()
        # Normalize common aliases
        if key in ("js", "node", "nodejs"):
            key = "javascript"
        elif key in ("ts",):
            key = "typescript"
        elif key in ("golang",):
            key = "go"
        elif key in ("py", "python3"):
            key = "python"

        patterns = _LANGUAGE_PATTERNS.get(key)
        if patterns:
            parts.append(f"\n### {lang} Coding Patterns\n{patterns}")

    return "\n".join(parts)


class CoderAgent(BaseAgent):
    purpose = Purpose.GENERATE

    def _build_system_prompt(self, context: ProjectContext) -> str:
        base = CODER_SYSTEM_PROMPT

        # Inject language constraints before project context
        if context.tech_stack:
            lang_constraints = _build_language_constraints(context.tech_stack)
            if lang_constraints:
                base += f"\n\n{lang_constraints}"

        project_context = context.to_system_prompt()
        if project_context:
            base += f"\n\n{project_context}"
        return base
