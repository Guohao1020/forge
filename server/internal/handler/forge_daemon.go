package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/multica-ai/multica/server/internal/forge/checks"
)

// ForgeChecksResponse is returned by the daemon forge-checks endpoint.
type ForgeChecksResponse struct {
	Checks []checks.Check `json:"checks"`
}

// GetTaskForgeChecks resolves verification checks for a claimed task (daemon-auth).
// The daemon runs these in the workdir after the agent session ends (F2 gate).
func (h *Handler) GetTaskForgeChecks(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskId")
	task, workspaceID, ok := h.requireDaemonTaskAccessWithWorkspace(w, r, taskID)
	if !ok {
		return
	}
	projID := h.taskProjectID(r.Context(), task.IssueID) // F1 helper in forge_hook.go
	cs, err := checks.ResolveChecks(r.Context(), h.Queries, parseUUID(workspaceID), projID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve checks")
		return
	}
	writeJSON(w, http.StatusOK, ForgeChecksResponse{Checks: cs})
}
