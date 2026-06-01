package forge

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func chk(name, cmd string) db.ForgeCheck {
	return db.ForgeCheck{Name: name, Command: cmd, Enabled: true}
}

func TestResolveChecks_Additive(t *testing.T) {
	ws := []db.ForgeCheck{chk("build", "go build ./..."), chk("lint", "ruff check")}
	proj := []db.ForgeCheck{chk("test", "go test ./...")}
	got := resolveChecks(ws, proj)
	if len(got) != 3 {
		t.Fatalf("expected workspace+project additive = 3 checks; got %d", len(got))
	}
	names := map[string]bool{}
	for _, c := range got {
		names[c.Name] = true
	}
	if !names["build"] || !names["lint"] || !names["test"] {
		t.Fatalf("missing checks; got %+v", got)
	}
}

type fakeCheckQ struct{ ws, proj []db.ForgeCheck }

func (f fakeCheckQ) ListWorkspaceChecks(_ context.Context, _ pgtype.UUID) ([]db.ForgeCheck, error) {
	return f.ws, nil
}
func (f fakeCheckQ) ListProjectChecks(_ context.Context, _ pgtype.UUID) ([]db.ForgeCheck, error) {
	return f.proj, nil
}

var _ CheckQuerier = fakeCheckQ{}

func TestResolveChecks_NoProject(t *testing.T) {
	q := fakeCheckQ{ws: []db.ForgeCheck{chk("build", "go build ./...")}}
	got, err := ResolveChecks(context.Background(), q, pgtype.UUID{Valid: true}, pgtype.UUID{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Command != "go build ./..." {
		t.Fatalf("no-project resolve should yield workspace checks only; got %+v", got)
	}
}
