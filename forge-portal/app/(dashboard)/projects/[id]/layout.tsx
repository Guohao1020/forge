"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { ProjectSidebar } from "@/components/project-sidebar";
import { Topbar } from "@/components/topbar";
import { api } from "@/lib/api";

interface Project {
  id: number;
  name: string;
}

export default function ProjectLayout({ children }: { children: React.ReactNode }) {
  const params = useParams();
  const projectId = params.id as string;
  const [project, setProject] = useState<Project | null>(null);

  useEffect(() => {
    api.get<Project>(`/projects/${projectId}`)
      .then(setProject)
      .catch(() => setProject({ id: Number(projectId), name: "项目" }));
  }, [projectId]);

  return (
    <div className="flex h-screen bg-background">
      <ProjectSidebar
        projectId={projectId}
        projectName={project?.name ?? "加载中..."}
      />
      <div className="flex-1 flex flex-col overflow-hidden">
        <Topbar />
        <main className="flex-1 overflow-auto p-6">
          {children}
        </main>
      </div>
    </div>
  );
}
