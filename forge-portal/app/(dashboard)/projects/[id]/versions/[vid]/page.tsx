"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  ArrowLeft,
  AlertTriangle,
  CheckCircle2,
  Clock,
  GitBranch,
  Tag,
} from "lucide-react";
import {
  getVersion,
  releaseVersion,
  VersionDetailResponse,
  VersionTaskBrief,
  VERSION_STATUS_CONFIG,
  CONFLICT_STATUS_CONFIG,
} from "@/lib/versions";

export default function VersionDetailPage() {
  const params = useParams();
  const projectId = params.id as string;
  const versionId = params.vid as string;
  const router = useRouter();

  const [detail, setDetail] = useState<VersionDetailResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [releasing, setReleasing] = useState(false);

  const fetchDetail = async () => {
    try {
      const res = await getVersion(Number(projectId), Number(versionId));
      setDetail(res);
    } catch {
      // empty
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDetail();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId, versionId]);

  const handleRelease = async () => {
    if (!confirm("确认发布此版本？将创建 git tag 并触发部署。")) return;
    setReleasing(true);
    try {
      await releaseVersion(Number(projectId), Number(versionId));
      await fetchDetail();
    } catch (e: unknown) {
      alert(e instanceof Error ? e.message : "发布失败");
    } finally {
      setReleasing(false);
    }
  };

  if (loading || !detail) {
    return (
      <div className="space-y-4">
        <div className="h-24 rounded-lg bg-muted/50 animate-pulse" />
        <div className="h-48 rounded-lg bg-muted/50 animate-pulse" />
      </div>
    );
  }

  const { version: v, tasks } = detail;
  const config = VERSION_STATUS_CONFIG[v.status];
  const allCompleted = tasks.length > 0 && tasks.every((t) => t.status === "COMPLETED");
  const hasConflicts = tasks.some((t) => t.conflictStatus === "DETECTED" || t.conflictStatus === "WAITING");
  const canRelease = (v.status === "TESTING" || v.status === "IN_PROGRESS") && allCompleted;

  return (
    <div className="space-y-6">
      {/* Back + Header */}
      <div>
        <button
          onClick={() => router.push(`/projects/${projectId}/versions`)}
          className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground mb-3"
        >
          <ArrowLeft size={14} />
          返回版本列表
        </button>

        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-mono font-bold text-foreground">{v.version}</h1>
            <span className={`px-2.5 py-1 rounded text-xs border ${config.bgColor} ${config.color}`}>
              {config.label}
            </span>
          </div>

          {canRelease && (
            <button
              onClick={handleRelease}
              disabled={releasing}
              className="flex items-center gap-2 px-4 py-2 bg-success text-success-foreground rounded-lg text-sm hover:bg-green-500 transition-colors disabled:opacity-50"
            >
              <Tag size={16} />
              {releasing ? "发布中..." : "发布版本"}
            </button>
          )}
        </div>

        {v.description && (
          <p className="text-sm text-muted-foreground mt-2">{v.description}</p>
        )}

        {v.gitTag && (
          <div className="flex items-center gap-2 mt-2 text-xs text-green-400">
            <Tag size={12} />
            <span>git tag: {v.gitTag}</span>
            {v.releasedAt && (
              <span className="text-muted-foreground">
                {new Date(v.releasedAt).toLocaleString("zh-CN")}
              </span>
            )}
          </div>
        )}
      </div>

      {/* Conflict warning banner */}
      {hasConflicts && (
        <div className="flex items-start gap-3 bg-amber-500/10 border border-amber-500/20 rounded-lg p-4">
          <AlertTriangle size={20} className="text-amber-400 shrink-0 mt-0.5" />
          <div>
            <p className="text-sm font-medium text-amber-400">检测到文件冲突</p>
            <p className="text-xs text-amber-400/70 mt-1">
              部分任务修改了相同的文件，系统已自动排队等待。先完成的任务合并后，后续任务将自动解除阻塞。
            </p>
          </div>
        </div>
      )}

      {/* Task list */}
      <div>
        <h2 className="text-sm font-medium text-muted-foreground mb-3">
          关联任务 ({tasks.length})
        </h2>

        {tasks.length === 0 ? (
          <div className="text-center py-10 text-muted-foreground text-sm">
            暂无关联任务。在创建任务时选择此版本即可关联。
          </div>
        ) : (
          <div className="space-y-2">
            {tasks.map((t) => (
              <TaskRow
                key={t.id}
                task={t}
                projectId={projectId}
                allTasks={tasks}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function TaskRow({
  task,
  projectId,
  allTasks,
}: {
  task: VersionTaskBrief;
  projectId: string;
  allTasks: VersionTaskBrief[];
}) {
  const router = useRouter();
  const conflictConfig = CONFLICT_STATUS_CONFIG[task.conflictStatus];
  const isCompleted = task.status === "COMPLETED";
  const isWaiting = task.conflictStatus === "WAITING";

  // Find blocker names
  const blockerNames = (task.blockedBy || [])
    .map((id) => allTasks.find((t) => t.id === id)?.title || `#${id}`)
    .filter(Boolean);

  return (
    <button
      onClick={() => router.push(`/projects/${projectId}/tasks/${task.id}`)}
      className={`w-full text-left bg-surface-1 border rounded-lg p-3 transition-colors hover:border-primary/30 ${
        isWaiting
          ? "border-purple-500/30"
          : task.conflictStatus === "DETECTED"
          ? "border-amber-500/30"
          : "border-border"
      }`}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          {isCompleted ? (
            <CheckCircle2 size={16} className="text-green-400" />
          ) : isWaiting ? (
            <Clock size={16} className="text-purple-400" />
          ) : (
            <div className="w-4 h-4 rounded-full border-2 border-border" />
          )}
          <span className="text-sm text-foreground">{task.title}</span>

          {conflictConfig.label && (
            <span className={`text-xs ${conflictConfig.color}`}>
              {conflictConfig.label}
            </span>
          )}
        </div>

        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          {task.branchName && (
            <span className="flex items-center gap-1">
              <GitBranch size={12} />
              {task.branchName.split("/").pop()}
            </span>
          )}
          <span>{task.status}</span>
        </div>
      </div>

      {isWaiting && blockerNames.length > 0 && (
        <div className="mt-2 text-xs text-purple-400/70 pl-6">
          等待: {blockerNames.join(", ")}
        </div>
      )}

      {task.touchedFiles && (() => {
        const files: string[] = typeof task.touchedFiles === "string"
          ? JSON.parse(task.touchedFiles)
          : Array.isArray(task.touchedFiles) ? task.touchedFiles : [];
        return files.length > 0 ? (
          <div className="mt-2 text-xs text-muted-foreground/50 pl-6 truncate">
            文件: {files.slice(0, 3).join(", ")}
            {files.length > 3 && " ..."}
          </div>
        ) : null;
      })()}
    </button>
  );
}
