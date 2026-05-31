package forge

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/forge/standards"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type fakeQ struct {
	ws, proj []db.ForgeStandard
	tags     []string
}

func (f fakeQ) ListWorkspaceStandards(_ context.Context, _ pgtype.UUID) ([]db.ForgeStandard, error) {
	return f.ws, nil
}
func (f fakeQ) ListProjectStandards(_ context.Context, _ pgtype.UUID) ([]db.ForgeStandard, error) {
	return f.proj, nil
}
func (f fakeQ) GetForgeProjectProfile(_ context.Context, _ pgtype.UUID) (db.ForgeProjectProfile, error) {
	return db.ForgeProjectProfile{Tags: f.tags}, nil
}

var _ standards.Querier = fakeQ{}

func validUUID() pgtype.UUID { return pgtype.UUID{Valid: true} }

func TestInjectStandards_AppendsCoreAndSkill(t *testing.T) {
	q := fakeQ{ws: []db.ForgeStandard{
		{Category: "api", Name: "rest", CoreContent: "CORE-RULES", DetailContent: "DETAILED", Enabled: true},
	}}
	instr := "You are a code agent."
	var skills []service.AgentSkillData
	InjectStandards(context.Background(), q, &instr, &skills, validUUID(), pgtype.UUID{})

	if !strings.Contains(instr, "CORE-RULES") || !strings.Contains(instr, "You are a code agent.") {
		t.Fatalf("instructions should keep base + append core; got %q", instr)
	}
	if len(skills) != 1 || skills[0].Name != standards.SkillName || !strings.Contains(skills[0].Content, "DETAILED") {
		t.Fatalf("expected one forge-standards skill with detail; got %+v", skills)
	}
}

func TestInjectStandards_EmptyNoop(t *testing.T) {
	q := fakeQ{}
	instr := "base"
	var skills []service.AgentSkillData
	InjectStandards(context.Background(), q, &instr, &skills, validUUID(), pgtype.UUID{})
	if instr != "base" || len(skills) != 0 {
		t.Fatalf("no standards must leave payload unchanged; got instr=%q skills=%d", instr, len(skills))
	}
}
