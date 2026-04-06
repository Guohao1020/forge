"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import { Package, Hash, HardDrive, ExternalLink, ListTodo, Tag } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { listArtifacts, type Artifact } from "@/lib/artifact";
import Link from "next/link";

const TYPE_LABELS: Record<string, string> = {
  DOCKER_IMAGE: "Docker 镜像",
  JAR: "JAR 包",
  BINARY: "二进制",
  ARCHIVE: "归档包",
};

const STATUS_STYLES: Record<string, string> = {
  BUILDING: "bg-yellow-500/10 text-yellow-400 border-yellow-500/20",
  READY: "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
  FAILED: "bg-red-500/10 text-red-400 border-red-500/20",
};

const STATUS_LABELS: Record<string, string> = {
  BUILDING: "构建中",
  READY: "就绪",
  FAILED: "失败",
};

function formatBytes(bytes?: number): string {
  if (!bytes) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024)
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function formatTime(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function LoadingSkeleton() {
  return (
    <div className="space-y-3 animate-pulse">
      {Array.from({ length: 3 }).map((_, i) => (
        <div
          key={i}
          className="rounded-xl border border-border bg-muted/50 p-5"
        >
          <div className="h-4 w-40 bg-muted rounded mb-3" />
          <div className="h-3 w-60 bg-muted rounded" />
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-20 rounded-xl border border-border bg-card">
      <div className="w-12 h-12 rounded-xl flex items-center justify-center mb-3 bg-primary/10">
        <Package className="h-6 w-6 text-primary" />
      </div>
      <h3 className="text-base font-medium mb-1">暂无制品</h3>
      <p className="text-sm text-muted-foreground">
        任务完成后自动生成
      </p>
    </div>
  );
}

export default function ArtifactsPage() {
  const params = useParams();
  const projectId = params.id as string;
  const [loading, setLoading] = useState(true);
  const [artifacts, setArtifacts] = useState<Artifact[]>([]);

  const fetchArtifacts = useCallback(async () => {
    try {
      setLoading(true);
      const arts = await listArtifacts(projectId);
      setArtifacts(arts);
    } catch (err) {
      console.error("Failed to fetch artifacts:", err);
      setArtifacts([]);
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    fetchArtifacts();
  }, [fetchArtifacts]);

  if (loading) {
    return (
      <div>
        <h1 className="text-2xl font-semibold tracking-tight mb-6">制品</h1>
        <LoadingSkeleton />
      </div>
    );
  }

  if (artifacts.length === 0) {
    return (
      <div>
        <h1 className="text-2xl font-semibold tracking-tight mb-6">制品</h1>
        <EmptyState />
      </div>
    );
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold tracking-tight mb-6">制品</h1>

      <div className="rounded-xl border border-border overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border bg-muted/50">
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">
                名称
              </th>
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">
                版本
              </th>
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">
                类型
              </th>
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">
                大小
              </th>
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">
                状态
              </th>
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">
                关联任务
              </th>
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">
                Registry
              </th>
              <th className="text-left px-4 py-3 font-medium text-muted-foreground">
                时间
              </th>
            </tr>
          </thead>
          <tbody>
            {artifacts.map((art) => (
              <tr
                key={art.id}
                className="border-b border-border last:border-0 hover:bg-muted/30 transition-colors"
              >
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2">
                    <Package className="h-4 w-4 text-muted-foreground/60 shrink-0" />
                    <span className="font-medium text-foreground">
                      {art.name}
                    </span>
                  </div>
                </td>
                <td className="px-4 py-3">
                  <code className="text-xs font-mono text-muted-foreground bg-muted/50 px-1.5 py-0.5 rounded">
                    {art.version}
                  </code>
                </td>
                <td className="px-4 py-3 text-muted-foreground">
                  {TYPE_LABELS[art.artifactType] || art.artifactType}
                </td>
                <td className="px-4 py-3">
                  <div className="flex items-center gap-1.5 text-muted-foreground">
                    <HardDrive className="h-3 w-3" />
                    {formatBytes(art.sizeBytes)}
                  </div>
                </td>
                <td className="px-4 py-3">
                  <Badge
                    variant="secondary"
                    className={`text-[10px] ${STATUS_STYLES[art.status] || "bg-muted text-muted-foreground"}`}
                  >
                    {STATUS_LABELS[art.status] || art.status}
                  </Badge>
                </td>
                <td className="px-4 py-3">
                  {art.taskId ? (
                    <Link
                      href={`/projects/${projectId}/tasks/${art.taskId}`}
                      className="flex items-center gap-1.5 text-xs text-primary hover:text-primary/80 transition-colors"
                    >
                      <ListTodo className="h-3 w-3 shrink-0" />
                      <span>任务 #{art.taskId}</span>
                    </Link>
                  ) : (
                    <span className="text-muted-foreground/40 text-xs">—</span>
                  )}
                </td>
                <td className="px-4 py-3">
                  {art.registryUrl ? (
                    <div className="flex items-center gap-1.5 text-xs text-muted-foreground/60 max-w-[200px]">
                      <ExternalLink className="h-3 w-3 shrink-0" />
                      <span className="truncate font-mono">
                        {art.registryUrl}
                      </span>
                    </div>
                  ) : (
                    <span className="text-muted-foreground/40">—</span>
                  )}
                </td>
                <td className="px-4 py-3 text-muted-foreground text-xs">
                  {formatTime(art.createdAt)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Checksum info */}
      {artifacts.some((a) => a.checksum) && (
        <div className="mt-4 space-y-1">
          {artifacts
            .filter((a) => a.checksum)
            .map((a) => (
              <div
                key={a.id}
                className="flex items-center gap-2 text-xs text-muted-foreground/40"
              >
                <Hash className="h-3 w-3" />
                <span className="font-mono">
                  {a.name}: SHA256={a.checksum}
                </span>
              </div>
            ))}
        </div>
      )}
    </div>
  );
}
