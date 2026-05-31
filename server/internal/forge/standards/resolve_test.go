package standards

import (
	"strings"
	"testing"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func std(category, name string, tags []string, core, detail string) db.ForgeStandard {
	return db.ForgeStandard{Category: category, Name: name, ProfileTags: tags, CoreContent: core, DetailContent: detail, Enabled: true}
}

func TestResolve_ProjectOverridesWorkspace(t *testing.T) {
	ws := []db.ForgeStandard{std("naming", "go", nil, "ws-core", "ws-detail")}
	proj := []db.ForgeStandard{std("naming", "go", nil, "proj-core", "proj-detail")}
	got := resolveStandards(ws, proj, nil)
	if !strings.Contains(got.Core, "proj-core") || strings.Contains(got.Core, "ws-core") {
		t.Fatalf("project core should override workspace; got %q", got.Core)
	}
}

func TestResolve_ProfileFilter(t *testing.T) {
	ws := []db.ForgeStandard{
		std("lang", "go", []string{"go"}, "go-core", "go-detail"),
		std("lang", "java", []string{"java"}, "java-core", "java-detail"),
		std("general", "naming", nil, "naming-core", "naming-detail"), // empty tags = always
	}
	got := resolveStandards(ws, nil, []string{"go"})
	if !strings.Contains(got.Core, "go-core") || !strings.Contains(got.Core, "naming-core") {
		t.Fatalf("go + empty-tag standards should apply; got %q", got.Core)
	}
	if strings.Contains(got.Core, "java-core") {
		t.Fatalf("java standard should be filtered out for go project; got %q", got.Core)
	}
}

func TestResolve_CoreDetailSplit(t *testing.T) {
	ws := []db.ForgeStandard{std("api", "rest", nil, "CORE-RULES", "DETAILED-GUIDANCE")}
	got := resolveStandards(ws, nil, nil)
	if !strings.Contains(got.Core, "CORE-RULES") {
		t.Fatalf("core missing core_content; got %q", got.Core)
	}
	if !strings.Contains(got.Detail, "DETAILED-GUIDANCE") || !strings.Contains(got.Detail, "name: forge-standards") {
		t.Fatalf("detail skill must contain detail_content + frontmatter; got %q", got.Detail)
	}
}

func TestResolve_EmptyDowngrade(t *testing.T) {
	got := resolveStandards(nil, nil, nil)
	if got.Core != "" || got.Detail != "" {
		t.Fatalf("empty standards must yield empty Resolved; got %+v", got)
	}
}
