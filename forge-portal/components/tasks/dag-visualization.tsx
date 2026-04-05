"use client";

/**
 * DAG Visualization — Shows task dependency graph as a vertical tree.
 *
 * Renders task nodes with colored type badges, effort hours, and
 * dependency arrows. Tasks at the same level (no mutual dependencies)
 * are shown side by side.
 */

interface DagTask {
  order: number;
  title: string;
  description?: string;
  type: string;
  files?: string[];
  depends_on?: number[];
  estimate_hours?: number;
  requirement_ref?: string;
}

interface DagVisualizationProps {
  tasks: DagTask[];
  touchedFiles?: { create?: string[]; modify?: string[] };
  onTaskClick?: (task: DagTask) => void;
}

const TYPE_COLORS: Record<string, { bg: string; text: string; border: string }> = {
  BACKEND:  { bg: "bg-blue-500/10",    text: "text-blue-400",    border: "border-blue-500/30" },
  FRONTEND: { bg: "bg-purple-500/10",  text: "text-purple-400",  border: "border-purple-500/30" },
  SCHEMA:   { bg: "bg-amber-500/10",   text: "text-amber-400",   border: "border-amber-500/30" },
  CONFIG:   { bg: "bg-gray-500/10",    text: "text-gray-400",    border: "border-gray-500/30" },
  TEST:     { bg: "bg-emerald-500/10", text: "text-emerald-400", border: "border-emerald-500/30" },
};

/**
 * Topological sort into levels. Tasks at the same level have no
 * mutual dependencies and can execute in parallel.
 */
function computeLevels(tasks: DagTask[]): DagTask[][] {
  const taskMap = new Map(tasks.map((t) => [t.order, t]));
  const inDegree = new Map<number, number>();
  const children = new Map<number, number[]>();

  for (const t of tasks) {
    inDegree.set(t.order, (t.depends_on || []).length);
    for (const dep of t.depends_on || []) {
      const existing = children.get(dep) || [];
      existing.push(t.order);
      children.set(dep, existing);
    }
  }

  const levels: DagTask[][] = [];
  let queue = tasks.filter((t) => (inDegree.get(t.order) || 0) === 0);

  while (queue.length > 0) {
    levels.push(queue);
    const nextQueue: DagTask[] = [];
    for (const t of queue) {
      for (const childOrder of children.get(t.order) || []) {
        const deg = (inDegree.get(childOrder) || 1) - 1;
        inDegree.set(childOrder, deg);
        if (deg === 0) {
          const child = taskMap.get(childOrder);
          if (child) nextQueue.push(child);
        }
      }
    }
    queue = nextQueue;
  }

  return levels;
}

export function DagVisualization({ tasks, touchedFiles, onTaskClick }: DagVisualizationProps) {
  if (!tasks || tasks.length === 0) {
    return (
      <div className="text-center py-8 text-muted-foreground text-sm">
        暂无任务分解数据
      </div>
    );
  }

  const levels = computeLevels(tasks);

  return (
    <div className="space-y-1">
      {levels.map((level, levelIdx) => (
        <div key={levelIdx}>
          {/* Level indicator */}
          {levelIdx > 0 && (
            <div className="flex justify-center py-1">
              <div className="w-px h-4 bg-border" />
            </div>
          )}

          {/* Tasks at this level */}
          <div className="flex gap-2 flex-wrap justify-center">
            {level.map((task) => {
              const colors = TYPE_COLORS[task.type] || TYPE_COLORS.CONFIG;
              return (
                <button
                  key={task.order}
                  onClick={() => onTaskClick?.(task)}
                  className={`
                    ${colors.bg} border ${colors.border}
                    rounded-lg px-3 py-2 text-left min-w-[180px] max-w-[260px]
                    hover:brightness-125 transition-all cursor-pointer
                  `}
                >
                  <div className="flex items-center justify-between mb-1">
                    <span className={`text-[10px] font-mono ${colors.text} uppercase`}>
                      {task.type}
                    </span>
                    {task.estimate_hours !== undefined && (
                      <span className="text-[10px] text-muted-foreground">
                        {task.estimate_hours}h
                      </span>
                    )}
                  </div>
                  <p className="text-xs text-foreground font-medium leading-tight truncate">
                    {task.order}. {task.title}
                  </p>
                  {task.files && task.files.length > 0 && (
                    <p className="text-[10px] text-muted-foreground/60 mt-1 truncate">
                      {task.files[0]}
                      {task.files.length > 1 && ` +${task.files.length - 1}`}
                    </p>
                  )}
                  {task.depends_on && task.depends_on.length > 0 && (
                    <p className="text-[10px] text-muted-foreground/40 mt-0.5">
                      deps: {task.depends_on.join(", ")}
                    </p>
                  )}
                </button>
              );
            })}
          </div>
        </div>
      ))}

      {/* Touched files summary */}
      {touchedFiles && (
        <div className="mt-4 pt-3 border-t border-border/50">
          <p className="text-[10px] text-muted-foreground mb-1.5 uppercase tracking-wider">
            涉及文件
          </p>
          <div className="flex flex-wrap gap-1">
            {(touchedFiles.create || []).map((f) => (
              <span
                key={`c-${f}`}
                className="text-[10px] px-1.5 py-0.5 rounded bg-green-500/10 text-green-400 border border-green-500/20"
              >
                + {f.split("/").pop()}
              </span>
            ))}
            {(touchedFiles.modify || []).map((f) => (
              <span
                key={`m-${f}`}
                className="text-[10px] px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20"
              >
                ~ {f.split("/").pop()}
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
