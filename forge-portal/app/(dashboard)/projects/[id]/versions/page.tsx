"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { Plus, Tag } from "lucide-react";
import {
  listVersions,
  createVersion,
  ProjectVersion,
  VERSION_STATUS_CONFIG,
} from "@/lib/versions";

export default function VersionListPage() {
  const params = useParams();
  const projectId = params.id as string;
  const router = useRouter();

  const [versions, setVersions] = useState<ProjectVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newVersion, setNewVersion] = useState("");
  const [newDesc, setNewDesc] = useState("");
  const [creating, setCreating] = useState(false);

  const fetchVersions = async () => {
    try {
      const res = await listVersions(Number(projectId));
      setVersions(res.versions || []);
    } catch {
      // empty
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchVersions();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId]);

  const handleCreate = async () => {
    if (!newVersion.trim()) return;
    setCreating(true);
    try {
      await createVersion(Number(projectId), newVersion.trim(), newDesc.trim());
      setShowCreate(false);
      setNewVersion("");
      setNewDesc("");
      await fetchVersions();
    } catch (e: unknown) {
      alert(e instanceof Error ? e.message : "创建失败");
    } finally {
      setCreating(false);
    }
  };

  if (loading) {
    return (
      <div className="space-y-3">
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-20 rounded-lg bg-white/5 animate-pulse" />
        ))}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold text-foreground">版本管理</h1>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 px-4 py-2 bg-primary text-white rounded-lg text-sm hover:bg-primary/90 transition-colors"
        >
          <Plus size={16} />
          创建版本
        </button>
      </div>

      {/* Create dialog */}
      {showCreate && (
        <div className="bg-surface-1 border border-border rounded-lg p-4 space-y-3">
          <div>
            <label className="block text-xs text-muted-foreground mb-1">版本号</label>
            <input
              type="text"
              value={newVersion}
              onChange={(e) => setNewVersion(e.target.value)}
              placeholder="v1.2.0"
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-foreground placeholder:text-white/30 focus:outline-none focus:border-primary/50"
              autoFocus
            />
          </div>
          <div>
            <label className="block text-xs text-muted-foreground mb-1">版本描述</label>
            <input
              type="text"
              value={newDesc}
              onChange={(e) => setNewDesc(e.target.value)}
              placeholder="用户积分功能 + 订单优化"
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-sm text-foreground placeholder:text-white/30 focus:outline-none focus:border-primary/50"
            />
          </div>
          <div className="flex gap-2 justify-end">
            <button
              onClick={() => setShowCreate(false)}
              className="px-3 py-1.5 text-sm text-muted-foreground hover:text-foreground"
            >
              取消
            </button>
            <button
              onClick={handleCreate}
              disabled={creating || !newVersion.trim()}
              className="px-4 py-1.5 bg-primary text-white rounded-lg text-sm hover:bg-primary/90 disabled:opacity-50"
            >
              {creating ? "创建中..." : "创建"}
            </button>
          </div>
        </div>
      )}

      {/* Version list */}
      {versions.length === 0 ? (
        <div className="text-center py-16 text-muted-foreground">
          <Tag size={48} className="mx-auto mb-4 opacity-30" />
          <p className="text-lg mb-1">暂无版本</p>
          <p className="text-sm">创建一个版本来组织你的需求迭代</p>
        </div>
      ) : (
        <div className="space-y-2">
          {versions.map((v) => {
            const config = VERSION_STATUS_CONFIG[v.status];
            const progress =
              v.taskCount > 0
                ? Math.round((v.completedCount / v.taskCount) * 100)
                : 0;

            return (
              <button
                key={v.id}
                onClick={() => router.push(`/projects/${projectId}/versions/${v.id}`)}
                className="w-full text-left bg-surface-1 border border-border rounded-lg p-4 hover:border-primary/30 transition-colors"
              >
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-3">
                    <span className="text-base font-mono font-semibold text-foreground">
                      {v.version}
                    </span>
                    <span
                      className={`px-2 py-0.5 rounded text-xs border ${config.bgColor} ${config.color}`}
                    >
                      {config.label}
                    </span>
                  </div>
                  <span className="text-xs text-muted-foreground">
                    {new Date(v.createdAt).toLocaleDateString("zh-CN")}
                  </span>
                </div>

                {v.description && (
                  <p className="text-sm text-muted-foreground mb-3 truncate">
                    {v.description}
                  </p>
                )}

                <div className="flex items-center gap-4">
                  <div className="flex-1">
                    <div className="h-1.5 bg-white/5 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-primary rounded-full transition-all duration-300"
                        style={{ width: `${progress}%` }}
                      />
                    </div>
                  </div>
                  <span className="text-xs text-muted-foreground whitespace-nowrap">
                    {v.completedCount}/{v.taskCount} 任务
                  </span>
                </div>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
