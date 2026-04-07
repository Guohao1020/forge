package agent

import (
	"context"
	"encoding/json"
	"strings"
)

// Suggestion is one entry in the Agent Terminal empty-state list.
type Suggestion struct {
	Text     string `json:"text"`
	Category string `json:"category,omitempty"` // "feature" | "fix" | "test" | "refactor"
}

// SuggestionsResponse is GET /projects/:id/agent/suggestions.
type SuggestionsResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
	Source      string       `json:"source"` // "heuristic" | "fallback"
	Language    string       `json:"language,omitempty"`
}

// defaultSuggestions is the language-agnostic fallback. Matches the
// frontend's pre-Stream-4c hardcoded list so the UX degrades gracefully
// when project language can't be detected.
var defaultSuggestions = []Suggestion{
	{Text: "Add user registration with JWT auth", Category: "feature"},
	{Text: "Fix the login bug in feat/auth", Category: "fix"},
	{Text: "Write tests for the API", Category: "test"},
}

// languageSuggestions maps a normalized language name to three
// language-appropriate starter prompts. Keys come from the detected
// language profile in skills/languages (via ai-worker) OR from the
// project's tech_stack.languages field.
var languageSuggestions = map[string][]Suggestion{
	"java": {
		{Text: "Add a REST endpoint with JPA persistence", Category: "feature"},
		{Text: "Fix the NullPointerException in OrderService", Category: "fix"},
		{Text: "Write JUnit 5 tests for the repository layer", Category: "test"},
	},
	"python": {
		{Text: "Add a FastAPI endpoint with Pydantic validation", Category: "feature"},
		{Text: "Fix the import cycle in src/models/", Category: "fix"},
		{Text: "Write pytest fixtures for the database layer", Category: "test"},
	},
	"go": {
		{Text: "Add a gin handler with struct validation", Category: "feature"},
		{Text: "Fix the race condition in the worker pool", Category: "fix"},
		{Text: "Write table-driven tests for the service layer", Category: "test"},
	},
	"typescript": {
		{Text: "Add a React component with form validation", Category: "feature"},
		{Text: "Fix the type error in the reducer", Category: "fix"},
		{Text: "Write vitest unit tests for the API client", Category: "test"},
	},
	"javascript": {
		{Text: "Add an Express route with middleware", Category: "feature"},
		{Text: "Fix the async/await error handling", Category: "fix"},
		{Text: "Write Jest tests for the utility functions", Category: "test"},
	},
	"rust": {
		{Text: "Add a new Cargo workspace member", Category: "feature"},
		{Text: "Fix the lifetime error in the parser", Category: "fix"},
		{Text: "Write integration tests in tests/", Category: "test"},
	},
}

// normalizeLanguage maps various tech_stack.languages values to a
// stable key in languageSuggestions. Accepts common aliases.
func normalizeLanguage(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	switch l {
	case "java":
		return "java"
	case "python", "py":
		return "python"
	case "go", "golang":
		return "go"
	case "typescript", "ts":
		return "typescript"
	case "javascript", "js", "node", "nodejs":
		return "javascript"
	case "rust", "rs":
		return "rust"
	}
	return ""
}

// techStack is the minimal subset of the project.tech_stack JSONB
// payload the suggestions heuristic cares about.
type techStack struct {
	Languages []string `json:"languages"`
	// Accept single-string variants for legacy rows.
	Language string `json:"language"`
}

// generateSuggestions picks 3 starter prompts based on the project's
// primary language. Falls back to the language-agnostic defaults when
// no match is found.
func (h *Handler) generateSuggestions(
	ctx context.Context,
	projectID int64,
) (*SuggestionsResponse, error) {
	if h.repo == nil {
		return &SuggestionsResponse{
			Suggestions: defaultSuggestions,
			Source:      "fallback",
		}, nil
	}

	raw, err := h.repo.GetProjectTechStack(ctx, projectID)
	if err != nil {
		return &SuggestionsResponse{
			Suggestions: defaultSuggestions,
			Source:      "fallback",
		}, nil
	}

	var stack techStack
	_ = json.Unmarshal(raw, &stack)

	candidates := stack.Languages
	if len(candidates) == 0 && stack.Language != "" {
		candidates = []string{stack.Language}
	}
	for _, lang := range candidates {
		key := normalizeLanguage(lang)
		if key == "" {
			continue
		}
		if suggestions, ok := languageSuggestions[key]; ok {
			return &SuggestionsResponse{
				Suggestions: suggestions,
				Source:      "heuristic",
				Language:    key,
			}, nil
		}
	}

	return &SuggestionsResponse{
		Suggestions: defaultSuggestions,
		Source:      "fallback",
	}, nil
}
