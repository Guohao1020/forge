# Plan: Project Code Repository Sync Option

## Goal

1. **Create project dialog** — Add option to associate a remote Git repo (GitHub), with a warning if user skips
2. **Project settings page** — Add "code repository" section, allowing users to associate/change repo URL after creation

## Changes

### Backend (forge-core)

**File: `internal/module/project/model.go`**
- Add `CodePlatform *string` and `CodeRepoURL *string` to `UpdateProjectRequest`

**File: `internal/module/project/repository.go`**
- Update `Repository.Update()` SQL to include `code_platform` and `code_repo_url` in the SET clause (same COALESCE pattern)

No new endpoints needed — the existing `PUT /api/projects/:id` and `POST /api/projects` already handle these fields.

### Frontend (forge-portal)

**File: `components/create-project-dialog.tsx`**
- Add collapsible "代码仓库（可选）" section below description:
  - Code platform select (GitHub only for now, disabled Codeup placeholder)
  - Repo URL input (placeholder: `https://github.com/owner/repo`)
- If user submits without repo URL, show an amber warning banner:
  - "未关联代码仓库，项目代码将仅存储在服务器本地，存在丢失风险。建议稍后在项目设置中关联远程仓库。"
  - Two buttons: "仍然创建" (proceed) and "返回填写" (go back to fill)
- If user fills repo URL, submit `codePlatform` + `codeRepoUrl` in the POST body

**File: `app/(dashboard)/projects/[id]/settings/page.tsx`**
- Add "代码仓库" card section between "基本信息" and save button:
  - Show current `codePlatform` and `codeRepoUrl` (read from project GET response)
  - Code platform display (GitHub icon + label, or "未关联")
  - Repo URL input field (editable)
  - If no repo connected, show amber warning: "未关联远程仓库，代码仅在服务器本地，存在丢失风险"
- Update the `Project` interface to include `codePlatform` and `codeRepoUrl`
- Include `codePlatform` and `codeRepoUrl` in the PUT request body

## Task Breakdown

1. Backend: Add `CodePlatform`/`CodeRepoURL` to `UpdateProjectRequest` + update SQL
2. Frontend: Enhance create-project-dialog with repo URL fields + warning
3. Frontend: Add code repository section to settings page

## Not in Scope

- GitHub OAuth auto-fill in create dialog (existing import flow handles this)
- Repo URL validation (URL format check or connectivity test)
- Auto-clone workspace on repo URL change
