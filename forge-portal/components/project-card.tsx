"use client";

import Link from "next/link";
import { Star, GitBranch, FolderGit2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";

interface Project {
  id: number;
  name: string;
  description: string;
  status: string;
  defaultBranch: string;
  codePlatform: string;
  starred: boolean;
  updatedAt: string;
}

interface ProjectCardProps {
  project: Project;
  onStarToggled: () => void;
}

export function ProjectCard({ project, onStarToggled }: ProjectCardProps) {
  async function toggleStar(e: React.MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    try {
      if (project.starred) {
        await api.delete(`/projects/${project.id}/star`);
      } else {
        await api.post(`/projects/${project.id}/star`);
      }
      onStarToggled();
    } catch {
      // ignore
    }
  }

  const updatedDate = new Date(project.updatedAt).toLocaleDateString("zh-CN", {
    month: "short",
    day: "numeric",
  });

  return (
    <Link href={`/projects/${project.id}`} className="block group">
      <div className="relative rounded-xl border border-border bg-card p-5 transition-all duration-200 hover:border-primary/50 hover:shadow-lg hover:shadow-primary/5">
        {/* Star button */}
        <Button
          variant="ghost"
          size="icon"
          className="absolute top-4 right-4 h-7 w-7 opacity-0 group-hover:opacity-100 transition-opacity"
          onClick={toggleStar}
        >
          <Star
            className={`h-4 w-4 transition-colors ${
              project.starred
                ? "fill-yellow-400 text-yellow-400"
                : "text-muted-foreground"
            }`}
          />
        </Button>
        {project.starred && (
          <Star className="absolute top-4 right-4 h-4 w-4 fill-yellow-400 text-yellow-400 group-hover:hidden" />
        )}

        {/* Header */}
        <div className="flex items-start gap-3 mb-3">
          <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10">
            <FolderGit2 className="h-4 w-4 text-primary" />
          </div>
          <div className="min-w-0">
            <h3 className="font-semibold text-foreground truncate pr-8 group-hover:text-primary transition-colors">
              {project.name}
            </h3>
            {project.description && (
              <p className="mt-0.5 text-sm text-muted-foreground line-clamp-2">
                {project.description}
              </p>
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center gap-2 mt-4 text-xs text-muted-foreground">
          <GitBranch className="h-3 w-3 shrink-0" />
          <span className="truncate">{project.defaultBranch}</span>
          {project.codePlatform && (
            <Badge variant="secondary" className="ml-auto text-xs py-0">
              {project.codePlatform}
            </Badge>
          )}
          <span className={project.codePlatform ? "" : "ml-auto"}>
            {updatedDate}
          </span>
        </div>
      </div>
    </Link>
  );
}
