package forgereview

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestIsReviewTask(t *testing.T) {
	marked, _ := json.Marshal(ForgeReviewContext{Type: ReviewContextType, ForgeReview: true})
	if !IsReviewTask(marked) {
		t.Fatal("marked context should be a review task")
	}
	if IsReviewTask(nil) || IsReviewTask([]byte(`{"type":"quick_create"}`)) {
		t.Fatal("non-review context must not be a review task")
	}
}

func TestShouldEnqueueReview(t *testing.T) {
	if !ShouldEnqueueReview(true, "/w/d", nil) {
		t.Fatal("issue-bound + workdir + non-review should enqueue")
	}
	if ShouldEnqueueReview(false, "/w/d", nil) {
		t.Fatal("no issue → skip")
	}
	if ShouldEnqueueReview(true, "", nil) {
		t.Fatal("no workdir → skip")
	}
	marked, _ := json.Marshal(ForgeReviewContext{ForgeReview: true})
	if ShouldEnqueueReview(true, "/w/d", marked) {
		t.Fatal("review task itself → skip (loop prevention)")
	}
}

type noRowErr struct{}

func (*noRowErr) Error() string { return "no row" }

var errNoRow = &noRowErr{}

type fakeReviewQ struct {
	ws, proj *db.ForgeReviewConfig
}

func (f fakeReviewQ) GetWorkspaceReviewConfig(_ context.Context, _ pgtype.UUID) (db.ForgeReviewConfig, error) {
	if f.ws == nil {
		return db.ForgeReviewConfig{}, errNoRow
	}
	return *f.ws, nil
}
func (f fakeReviewQ) GetProjectReviewConfig(_ context.Context, _ pgtype.UUID) (db.ForgeReviewConfig, error) {
	if f.proj == nil {
		return db.ForgeReviewConfig{}, errNoRow
	}
	return *f.proj, nil
}

var _ ReviewConfigQuerier = fakeReviewQ{}

func agentUUID(b byte) pgtype.UUID { return pgtype.UUID{Bytes: [16]byte{b}, Valid: true} }

func TestResolveReviewer_ProjectOverrides(t *testing.T) {
	wsCfg := &db.ForgeReviewConfig{ReviewerAgentID: agentUUID(1)}
	projCfg := &db.ForgeReviewConfig{ReviewerAgentID: agentUUID(2)}

	got, ok := ResolveReviewer(context.Background(), fakeReviewQ{ws: wsCfg, proj: projCfg}, pgtype.UUID{Valid: true}, pgtype.UUID{Valid: true})
	if !ok || got.Bytes[0] != 2 {
		t.Fatalf("project reviewer should win; got ok=%v id0=%d", ok, got.Bytes[0])
	}
	got, ok = ResolveReviewer(context.Background(), fakeReviewQ{ws: wsCfg}, pgtype.UUID{Valid: true}, pgtype.UUID{Valid: true})
	if !ok || got.Bytes[0] != 1 {
		t.Fatalf("workspace reviewer fallback; got ok=%v id0=%d", ok, got.Bytes[0])
	}
	if _, ok := ResolveReviewer(context.Background(), fakeReviewQ{}, pgtype.UUID{Valid: true}, pgtype.UUID{}); ok {
		t.Fatal("no config → ok=false")
	}
}
