package project_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/shulex/forge/forge-core/internal/module/project"
	"github.com/shulex/forge/forge-core/internal/testutil"
)

const (
	testTenantID int64 = 1
	testUserID   int64 = 1
)

// uid generates a unique suffix for test data to avoid conflicts across runs.
func uid() string {
	return fmt.Sprintf("%d_%d", time.Now().UnixMilli(), rand.Intn(10000))
}

func setupService(t *testing.T) (*project.Service, context.Context) {
	t.Helper()
	db := testutil.TestDB(t)
	repo := project.NewRepository(db)
	svc := project.NewService(repo, nil, nil)
	return svc, context.Background()
}

// --- CRUD Tests ---

func TestCreateProject(t *testing.T) {
	svc, ctx := setupService(t)
	name := "测试项目_" + uid()

	p, err := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
		Name:        name,
		Description: "集成测试用项目",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if p.Name != name {
		t.Fatalf("expected name '%s', got '%s'", name, p.Name)
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
	name := "唯一性测试_" + uid()

	_, err := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
		Name: name,
	})
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	_, err = svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
		Name: name,
	})
	if err == nil {
		t.Fatal("expected error for duplicate name within same tenant")
	}
}

func TestGetByID(t *testing.T) {
	svc, ctx := setupService(t)
	name := "查询测试_" + uid()

	created, _ := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
		Name: name,
	})

	got, err := svc.GetByID(ctx, created.ID, testTenantID, testUserID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Name != name {
		t.Fatalf("expected '%s', got '%s'", name, got.Name)
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
		Name:        "更新前_" + uid(),
		Description: "旧描述",
	})

	newName := "更新后_" + uid()
	newDesc := "新描述"
	updated, err := svc.Update(ctx, created.ID, testTenantID, &project.UpdateProjectRequest{
		Name:        &newName,
		Description: &newDesc,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Name != newName {
		t.Fatalf("expected name '%s', got '%s'", newName, updated.Name)
	}
	if updated.Description != "新描述" {
		t.Fatalf("expected description '新描述', got '%s'", updated.Description)
	}
}

func TestArchiveProject(t *testing.T) {
	svc, ctx := setupService(t)

	created, _ := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
		Name: "归档测试_" + uid(),
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
		Name: "收藏测试_" + uid(),
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
		Name: "幂等收藏_" + uid(),
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

	u := uid()
	svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: "列表项目A_" + u})
	svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: "列表项目B_" + u})
	svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: "列表项目C_" + u})

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

	u2 := uid()
	searchTerm := "搜索目标_" + u2
	svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: searchTerm + "_Alpha"})
	svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: searchTerm + "_Beta"})
	svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: "无关项目_" + u2})

	result, err := svc.List(ctx, testTenantID, testUserID, &project.ListProjectsQuery{
		Search: searchTerm,
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

	u3 := uid()
	p1, _ := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: "收藏筛选A_" + u3})
	svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: "收藏筛选B_" + u3})

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
	if result.Projects[0].Name != "收藏筛选A_"+u3 {
		t.Fatalf("expected '收藏筛选A_%s', got '%s'", u3, result.Projects[0].Name)
	}
}

func TestListProjects_Pagination(t *testing.T) {
	svc, ctx := setupService(t)

	for i := 0; i < 5; i++ {
		svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{
			Name: fmt.Sprintf("分页测试_%s_%c", uid(), rune('A'+i)),
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

	archName := "归档不可见_" + uid()
	p, _ := svc.Create(ctx, testTenantID, testUserID, &project.CreateProjectRequest{Name: archName})
	svc.Archive(ctx, p.ID, testTenantID)

	result, _ := svc.List(ctx, testTenantID, testUserID, &project.ListProjectsQuery{
		Search: archName,
		Page:   1,
		Size:   10,
	})
	if result.Total != 0 {
		t.Fatalf("archived project should not appear in list, got total=%d", result.Total)
	}
}
