// Package forge wires Forge's spec-center into Multica's task dispatch.
package forge

import (
	"context"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/forge/standards"
	"github.com/multica-ai/multica/server/internal/service"
)

// InjectStandards resolves Forge standards for (workspaceID, projectID) and
// merges them into the task's agent payload: Core → instructions (appended,
// mandatory), Detail → a forge-standards skill (appended). Best-effort: on any
// error it logs and leaves the payload unchanged — never blocks a task.
func InjectStandards(
	ctx context.Context,
	q standards.Querier,
	instructions *string,
	skills *[]service.AgentSkillData,
	workspaceID, projectID pgtype.UUID,
) {
	res, err := standards.Resolve(ctx, q, workspaceID, projectID)
	if err != nil {
		slog.Warn("forge: resolve standards failed", "error", err)
		return
	}
	if res.Core != "" {
		if strings.TrimSpace(*instructions) == "" {
			*instructions = res.Core
		} else {
			*instructions = *instructions + "\n\n## Forge Coding Standards (mandatory)\n\n" + res.Core
		}
	}
	if res.Detail != "" {
		*skills = append(*skills, service.AgentSkillData{
			Name:        standards.SkillName,
			Description: "Project coding standards resolved by Forge.",
			Content:     res.Detail,
		})
	}
}
