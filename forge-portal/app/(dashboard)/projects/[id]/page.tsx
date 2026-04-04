"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import { Plus, CheckCircle2, Clock, AlertTriangle, Shield } from "lucide-react";
import { Button } from "@/components/ui/button";
import { KanbanBoard } from "@/components/tasks/kanban-board";
import { CreateTaskDialog } from "@/components/tasks/create-task-dialog";
import { listTasks, Task } from "@/lib/tasks";
import { api } from "@/lib/api";

export default function ProjectTasksPage() {
  const params = useParams();
  const projectId = params.id as string;
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [stats, setStats] = useState<{
    totalTasks: number;
    completedTasks: number;
    tasksByStatus: Record<string, number>;
    activeVersions: number;
    qualityScore?: number;
  } | null>(null);

  const fetchTasks = useCallback(async () => {
    try {
      const data = await listTasks(projectId);
      setTasks(data.tasks ?? []);
    } catch {
      setTasks([]);
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    fetchTasks();
    api.get<{ totalTasks: number; completedTasks: number; tasksByStatus: Record<string, number>; activeVersions: number; qualityScore?: number }>(`/projects/${projectId}/stats`)
      .then(setStats)
      .catch(() => {});
    // Poll every 5 seconds for task status updates
    const interval = setInterval(fetchTasks, 5000);
    return () => clearInterval(interval);
  }, [fetchTasks, projectId]);

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-semibold tracking-tight">任务</h1>
        <Button
          className="gap-2"
          style={{ boxShadow: "0 0 20px rgba(139, 92, 246, 0.3)" }}
          onClick={() => setDialogOpen(true)}
        >
          <Plus size={16} />
          新建任务
        </Button>
      </div>

      {/* Project Stats Bar */}
      {stats && stats.totalTasks > 0 && (
        <div className="grid grid-cols-4 gap-3 mb-6">
          <div className="bg-card border border-border rounded-lg px-4 py-2.5 flex items-center gap-3">
            <Clock className="h-4 w-4 text-muted-foreground" />
            <div>
              <p className="text-[10px] text-muted-foreground uppercase">总任务</p>
              <p className="text-lg font-bold text-foreground">{stats.totalTasks}</p>
            </div>
          </div>
          <div className="bg-card border border-border rounded-lg px-4 py-2.5 flex items-center gap-3">
            <CheckCircle2 className="h-4 w-4 text-green-400" />
            <div>
              <p className="text-[10px] text-muted-foreground uppercase">已完成</p>
              <p className="text-lg font-bold text-green-400">{stats.completedTasks}</p>
            </div>
          </div>
          <div className="bg-card border border-border rounded-lg px-4 py-2.5 flex items-center gap-3">
            <AlertTriangle className="h-4 w-4 text-yellow-400" />
            <div>
              <p className="text-[10px] text-muted-foreground uppercase">进行中</p>
              <p className="text-lg font-bold text-yellow-400">
                {(stats.tasksByStatus["RUNNING"] || 0) + (stats.tasksByStatus["SUBMITTED"] || 0)}
              </p>
            </div>
          </div>
          {stats.qualityScore != null && (
            <div className="bg-card border border-border rounded-lg px-4 py-2.5 flex items-center gap-3">
              <Shield className="h-4 w-4 text-primary" />
              <div>
                <p className="text-[10px] text-muted-foreground uppercase">质量分数</p>
                <p className={`text-lg font-bold ${
                  stats.qualityScore >= 80 ? "text-green-400" :
                  stats.qualityScore >= 60 ? "text-yellow-400" : "text-red-400"
                }`}>{stats.qualityScore}</p>
              </div>
            </div>
          )}
        </div>
      )}

      {loading ? (
        <div className="grid grid-cols-4 gap-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="h-[300px] rounded-lg bg-card animate-pulse" />
          ))}
        </div>
      ) : (
        <KanbanBoard tasks={tasks} projectId={projectId} />
      )}

      <CreateTaskDialog
        projectId={projectId}
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onCreated={fetchTasks}
      />
    </div>
  );
}
