package project_test

import (
	"context"
	"testing"

	"github.com/shulex/forge/forge-core/internal/module/project"
	"github.com/shulex/forge/forge-core/internal/testutil"
)

const (
	testTenantID int64 = 1
	testUserID   int64 = 1
)

func setupService(t *testing.T) (*project.Service, context.Context) {
	t.Helper()
	db := testutil.TestDB(t)
	repo := project.NewRepository(db)
	svc := project.NewService(repo)
	return svc, context.Background()
}

// --- CRUD Tests ---

func TestCreateProject(t *testing.T) {
	svc, ctx := setupService(t)

	p, err := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
		Name:        "测试项目",
		Description: "集成测试用项目",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if p.Name != "测试项目" {
		t.Fatalf("expected name '测试项目', got '%s'", p.Name)
	}
	if p.Status != "ACTIVE" {
		t.Fatalf("expected status ACTIVE, got '%s'", p.Status)
	}
	if p.DefaultBranch != "main" {
		t.Fatalf("expected default branch 'main', got '%s'", p.DefaultBranch)
	}
	if p.RiskThreshold != 90 {
		t.Fatalf("expected risk threshold 90, got %d", p.RiskThreshold)
	}
	if !p.AutoMerge {
		t.Fatal("expected auto_merge true")
	}
}

func TestCreateProject_EmptyName(t *testing.T) {
	svc, ctx := setupService(t)

	_, err := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
		Name: "  ",
	})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCreateProject_DuplicateName(t *testing.T) {
	svc, ctx := setupService(t)

	_, err := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
		Name: "唯一性测试",
	})
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	_, err = svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
		Name: "唯一性测试",
	})
	if err == nil {
		t.Fatal("expected error for duplicate name within same tenant")
	}
}

func TestGetByID(t *testing.T) {
	svc, ctx := setupService(t)

	created, _ := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
		Name: "查询测试",
	})

	got, err := svc.GetByID(ctx, created.ID, testTenantID, testUserID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Name != "查询测试" {
		t.Fatalf("expected '查询测试', got '%s'", got.Name)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	svc, ctx := setupService(t)

	_, err := svc.GetByID(ctx, 99999, testTenantID, testUserID)
	if err == nil {
		t.Fatal("expected error for non-existent project")
	}
}

func TestUpdateProject(t *testing.T) {
	svc, ctx := setupService(t)

	created, _ := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
		Name:        "更新前",
		Description: "旧描述",
	})

	newName := "更新后"
	newDesc := "新描述"
	updated, err := svc.Update(ctx, created.ID, testTenantID, &project.UpdateProjectRequest{
		Name:        &newName,
		Description: &newDesc,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Name != "更新后" {
		t.Fatalf("expected name '更新后', got '%s'", updated.Name)
	}
	if updated.Description != "新描述" {
		t.Fatalf("expected description '新描述', got '%s'", updated.Description)
	}
}

func TestArchiveProject(t *testing.T) {
	svc, ctx := setupService(t)

	created, _ := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
		Name: "归档测试",
	})

	err := svc.Archive(ctx, created.ID, testTenantID)
	if err != nil {
		t.Fatalf("Archive failed: %v", err)
	}

	// Archived project should not be found via GetByID
	_, err = svc.GetByID(ctx, created.ID, testTenantID, testUserID)
	if err == nil {
		t.Fatal("expected error: archived project should not be retrievable")
	}
}

// --- Star Tests ---

func TestStarAndUnstar(t *testing.T) {
	svc, ctx := setupService(t)

	created, _ := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
		Name: "收藏测试",
	})

	// Star
	if err := svc.Star(ctx, created.ID, testTenantID, testUserID); err != nil {
		t.Fatalf("Star failed: %v", err)
	}

	// Verify starred flag
	got, _ := svc.GetByID(ctx, created.ID, testTenantID, testUserID)
	if !got.Starred {
		t.Fatal("expected starred=true after starring")
	}

	// Unstar
	if err := svc.Unstar(ctx, created.ID, testTenantID, testUserID); err != nil {
		t.Fatalf("Unstar failed: %v", err)
	}

	got, _ = svc.GetByID(ctx, created.ID, testTenantID, testUserID)
	if got.Starred {
		t.Fatal("expected starred=false after unstarring")
	}
}

func TestStarIdempotent(t *testing.T) {
	svc, ctx := setupService(t)

	created, _ := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
		Name: "幂等收藏",
	})

	// Star twice should not error
	svc.Star(ctx, created.ID, testTenantID, testUserID)
	if err := svc.Star(ctx, created.ID, testTenantID, testUserID); err != nil {
		t.Fatalf("second Star should be idempotent, got: %v", err)
	}
}

// --- List Tests ---

func TestListProjects(t *testing.T) {
	svc, ctx := setupService(t)

	svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: "列表项目A"})
	svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: "列表项目B"})
	svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: "列表项目C"})

	result, err := svc.List(ctx, testTenantID, testUserID, &project.ListProjectsQuery{
		Page: 1,
		Size: 10,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if result.Total < 3 {
		t.Fatalf("expected at least 3 projects, got %d", result.Total)
	}
	if len(result.Projects) < 3 {
		t.Fatalf("expected at least 3 projects in slice, got %d", len(result.Projects))
	}
}

func TestListProjects_Search(t *testing.T) {
	svc, ctx := setupService(t)

	svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: "搜索目标Alpha"})
	svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: "搜索目标Beta"})
	svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: "无关项目"})

	result, err := svc.List(ctx, testTenantID, testUserID, &project.ListProjectsQuery{
		Search: "搜索目标",
		Page:   1,
		Size:   10,
	})
	if err != nil {
		t.Fatalf("List with search failed: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("expected 2 search results, got %d", result.Total)
	}
}

func TestListProjects_StarredFilter(t *testing.T) {
	svc, ctx := setupService(t)

	p1, _ := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: "收藏筛选A"})
	svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: "收藏筛选B"})

	svc.Star(ctx, p1.ID, testTenantID, testUserID)

	result, err := svc.List(ctx, testTenantID, testUserID, &project.ListProjectsQuery{
		Starred: true,
		Page:    1,
		Size:    10,
	})
	if err != nil {
		t.Fatalf("List with starred filter failed: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 starred project, got %d", result.Total)
	}
	if result.Projects[0].Name != "收藏筛选A" {
		t.Fatalf("expected '收藏筛选A', got '%s'", result.Projects[0].Name)
	}
}

func TestListProjects_Pagination(t *testing.T) {
	svc, ctx := setupService(t)

	for i := 0; i < 5; i++ {
		svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
			Name: "分页测试" + string(rune('A'+i)),
		})
	}

	result, err := svc.List(ctx, testTenantID, testUserID, &project.ListProjectsQuery{
		Page: 1,
		Size: 2,
	})
	if err != nil {
		t.Fatalf("List page 1 failed: %v", err)
	}
	if len(result.Projects) != 2 {
		t.Fatalf("expected 2 projects on page 1, got %d", len(result.Projects))
	}
	if result.Total < 5 {
		t.Fatalf("expected total >= 5, got %d", result.Total)
	}
}

func TestListProjects_ArchivedNotShown(t *testing.T) {
	svc, ctx := setupService(t)

	p, _ := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: "归档不可见"})
	svc.Archive(ctx, p.ID, testTenantID)

	result, _ := svc.List(ctx, testTenantID, testUserID, &project.ListProjectsQuery{
		Search: "归档不可见",
		Page:   1,
		Size:   10,
	})
	if result.Total != 0 {
		t.Fatalf("archived project should not appear in list, got total=%d", result.Total)
	}
}
