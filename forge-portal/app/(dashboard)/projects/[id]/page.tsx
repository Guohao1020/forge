"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { Plus, Inbox } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { api } from "@/lib/api";

interface Project {
  id: number;
  name: string;
  description: string;
  defaultBranch: string;
  codePlatform: string;
  aiModel: string;
  riskThreshold: number;
  autoMerge: boolean;
}

export default function ProjectOverviewPage() {
  const params = useParams();
  const projectId = params.id as string;
  const [project, setProject] = useState<Project | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.get<Project>(`/projects/${projectId}`)
      .then(setProject)
      .finally(() => setLoading(false));
  }, [projectId]);

  if (loading) {
    return (
      <div className="space-y-4">
        <div className="h-8 w-48 rounded-lg bg-card animate-pulse" />
        <div className="h-32 rounded-xl bg-card animate-pulse" />
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">{project?.name}</h1>
          {project?.description && (
            <p className="text-sm text-muted-foreground mt-1">{project.description}</p>
          )}
        </div>
        <Button
          className="gap-2"
          style={{ boxShadow: "0 0 20px rgba(139, 92, 246, 0.3)" }}
        >
          <Plus size={16} />
          新建任务
        </Button>
      </div>

      {/* Stats row */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-8">
        {[
          { label: "进行中任务", value: "0" },
          { label: "待评审变更", value: "0" },
          { label: "测试通过率", value: "—" },
          { label: "最近部署", value: "—" },
        ].map((stat) => (
          <div key={stat.label} className="rounded-xl border border-border bg-card p-4">
            <p className="text-xs text-muted-foreground">{stat.label}</p>
            <p className="mt-1 text-2xl font-semibold">{stat.value}</p>
          </div>
        ))}
      </div>

      {/* Project info */}
      {project && (
        <div className="rounded-xl border border-border bg-card p-5 mb-6">
          <h2 className="text-sm font-medium mb-4">项目配置</h2>
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-muted-foreground">默认分支</span>
              <p className="mt-0.5 font-mono text-xs">{project.defaultBranch}</p>
            </div>
            <div>
              <span className="text-muted-foreground">AI 模型</span>
              <p className="mt-0.5">{project.aiModel || "默认（Claude）"}</p>
            </div>
            <div>
              <span className="text-muted-foreground">风险阈值</span>
              <p className="mt-0.5">{project.riskThreshold}%</p>
            </div>
            <div>
              <span className="text-muted-foreground">自动合并</span>
              <Badge variant={project.autoMerge ? "default" : "secondary"} className="mt-0.5">
                {project.autoMerge ? "开启" : "关闭"}
              </Badge>
            </div>
          </div>
        </div>
      )}

      {/* Empty tasks state */}
      <div className="flex flex-col items-center justify-center py-20 rounded-xl border border-border bg-card">
        <div className="w-12 h-12 rounded-xl flex items-center justify-center mb-3 bg-primary/10">
          <Inbox className="h-6 w-6 text-primary" />
        </div>
        <h3 className="text-base font-medium mb-1">还没有任务</h3>
        <p className="text-sm text-muted-foreground mb-4">
          用自然语言描述需求，AI 为你生成代码
        </p>
        <Button className="gap-2">
          <Plus size={16} />
          新建任务
        </Button>
      </div>
    </div>
  );
}
