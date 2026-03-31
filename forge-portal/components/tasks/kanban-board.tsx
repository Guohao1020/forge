"use client";

import { Task, KANBAN_COLUMNS } from "@/lib/tasks";
import { TaskCard } from "./task-card";

interface KanbanBoardProps {
  tasks: Task[];
  projectId: string;
}

export function KanbanBoard({ tasks, projectId }: KanbanBoardProps) {
  return (
    <div className="grid grid-cols-4 gap-4 min-h-[400px]">
      {KANBAN_COLUMNS.map((col) => {
        const columnTasks = tasks.filter((t) => col.statuses.includes(t.status));
        return (
          <div key={col.key} className="flex flex-col">
            <div className="flex items-center gap-2 mb-3">
              <h3 className="text-sm font-medium text-muted-foreground">{col.label}</h3>
              {columnTasks.length > 0 && (
                <span className="text-xs text-muted-foreground bg-secondary rounded-full px-2 py-0.5">
                  {columnTasks.length}
                </span>
              )}
            </div>
            <div className="flex-1 space-y-2 rounded-lg border border-border/50 bg-secondary/20 p-2 min-h-[200px]">
              {columnTasks.length === 0 ? (
                <p className="text-xs text-muted-foreground text-center py-8">暂无</p>
              ) : (
                columnTasks.map((task) => (
                  <TaskCard key={task.id} task={task} projectId={projectId} />
                ))
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
