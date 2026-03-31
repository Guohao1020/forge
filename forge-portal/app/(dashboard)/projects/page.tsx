"use client";

import { useEffect, useState, useCallback } from "react";
import { useSearchParams } from "next/navigation";
import { FolderOpen, Plus, GitBranch, Search, Star } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ProjectCard } from "@/components/project-card";
import { CreateProjectDialog } from "@/components/create-project-dialog";
import { ConnectPlatformDialog } from "@/components/connect-platform-dialog";
import { ImportReposDialog } from "@/components/import-repos-dialog";
import { api } from "@/lib/api";

interface Project {
  id: number;
  name: string;
  description: string;
  status: string;
  defaultBranch: string;
  codePlatform: string;
  codeRepoUrl: string;
  starred: boolean;
  updatedAt: string;
}

interface ListResponse {
  projects: Project[];
  total: number;
  page: number;
  size: number;
}

export default function ProjectsPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [starredOnly, setStarredOnly] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [connectOpen, setConnectOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const searchParams = useSearchParams();

  // Auto-open import dialog after GitHub OAuth callback
  useEffect(() => {
    if (searchParams.get("github_connected") === "true") {
      setImportOpen(true);
      // Clean up URL param
      window.history.replaceState({}, "", "/projects");
    }
  }, [searchParams]);

  const fetchProjects = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (search) params.set("search", search);
      if (starredOnly) params.set("starred", "true");
      params.set("page", "1");
      params.set("size", "50");
      const data = await api.get<ListResponse>(`/projects?${params.toString()}`);
      setProjects(data.projects ?? []);
      setTotal(data.total);
    } catch {
      setProjects([]);
    } finally {
      setLoading(false);
    }
  }, [search, starredOnly]);

  useEffect(() => {
    const timer = setTimeout(fetchProjects, search ? 300 : 0);
    return () => clearTimeout(timer);
  }, [fetchProjects, search]);

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">项目大厅</h1>
          {total > 0 && (
            <p className="text-sm text-muted-foreground mt-0.5">共 {total} 个项目</p>
          )}
        </div>
        <div className="flex gap-3">
          <Button variant="outline" className="gap-2" onClick={() => setConnectOpen(true)}>
            <GitBranch size={16} />
            接入代码平台
          </Button>
          <Button
            className="gap-2"
            style={{ boxShadow: "0 0 20px rgba(139, 92, 246, 0.3)" }}
            onClick={() => setDialogOpen(true)}
          >
            <Plus size={16} />
            新建项目
          </Button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3 mb-6">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索项目..."
            className="pl-9 bg-input border-border"
          />
        </div>
        <Button
          variant={starredOnly ? "default" : "outline"}
          size="sm"
          className="gap-1.5"
          onClick={() => setStarredOnly(!starredOnly)}
        >
          <Star className={`h-3.5 w-3.5 ${starredOnly ? "fill-current" : ""}`} />
          收藏
        </Button>
      </div>

      {/* Content */}
      {loading ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <div
              key={i}
              className="h-[130px] rounded-xl border border-border bg-card animate-pulse"
            />
          ))}
        </div>
      ) : projects.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-24 rounded-xl border border-border bg-card">
          <div className="w-16 h-16 rounded-2xl flex items-center justify-center mb-4 bg-primary/10">
            <FolderOpen size={32} className="text-primary" />
          </div>
          <h3 className="text-lg font-medium mb-2">
            {starredOnly ? "没有收藏的项目" : search ? "没有匹配的项目" : "还没有项目"}
          </h3>
          <p className="text-sm text-muted-foreground mb-6">
            {starredOnly || search
              ? "换个条件试试"
              : "接入代码平台同步已有项目，或创建一个新项目开始"}
          </p>
          {!starredOnly && !search && (
            <div className="flex gap-3">
              <Button variant="outline" className="gap-2">
                <GitBranch size={16} />
                接入代码平台
              </Button>
              <Button className="gap-2" onClick={() => setDialogOpen(true)}>
                <Plus size={16} />
                新建项目
              </Button>
            </div>
          )}
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {projects.map((project) => (
            <ProjectCard
              key={project.id}
              project={project}
              onStarToggled={fetchProjects}
            />
          ))}
        </div>
      )}

      <CreateProjectDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onCreated={fetchProjects}
      />
      <ConnectPlatformDialog
        open={connectOpen}
        onOpenChange={setConnectOpen}
      />
      <ImportReposDialog
        open={importOpen}
        onOpenChange={setImportOpen}
        onImported={fetchProjects}
      />
    </div>
  );
}
