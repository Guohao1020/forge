"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { KanbanBoard } from "@/components/tasks/kanban-board";
import { CreateTaskDialog } from "@/components/tasks/create-task-dialog";
import { listTasks, Task } from "@/lib/tasks";

export default function ProjectTasksPage() {
  const params = useParams();
  const projectId = params.id as string;
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);

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
    // Poll every 5 seconds for task status updates
    const interval = setInterval(fetchTasks, 5000);
    return () => clearInterval(interval);
  }, [fetchTasks]);

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
