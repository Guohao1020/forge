"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { Webhook, Plus, Trash2, CheckCircle2 } from "lucide-react";
import { api } from "@/lib/api";

interface WebhookItem {
  id: number;
  url: string;
  events: string;
  active: boolean;
  createdAt: string;
}

const EVENT_OPTIONS = [
  { value: "*", label: "所有事件" },
  { value: "task.completed", label: "任务完成" },
  { value: "task.failed", label: "任务失败" },
  { value: "pr.created", label: "PR 创建" },
  { value: "version.released", label: "版本发布" },
  { value: "entropy.scanned", label: "质量扫描完成" },
];

export default function WebhooksPage() {
  const params = useParams();
  const projectId = params.id as string;

  const [webhooks, setWebhooks] = useState<WebhookItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newUrl, setNewUrl] = useState("");
  const [newSecret, setNewSecret] = useState("");
  const [newEvents, setNewEvents] = useState("*");
  const [creating, setCreating] = useState(false);

  const fetchWebhooks = async () => {
    try {
      const res = await api.get<{ webhooks: WebhookItem[] }>(`/projects/${projectId}/webhooks`);
      setWebhooks(res.webhooks);
    } catch {
      setWebhooks([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchWebhooks(); }, [projectId]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleCreate = async () => {
    if (!newUrl.trim()) return;
    setCreating(true);
    try {
      await api.post(`/projects/${projectId}/webhooks`, {
        url: newUrl.trim(),
        secret: newSecret || undefined,
        events: newEvents,
      });
      setShowCreate(false);
      setNewUrl("");
      setNewSecret("");
      setNewEvents("*");
      await fetchWebhooks();
    } catch (e: unknown) {
      alert(e instanceof Error ? e.message : "创建失败");
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await api.delete(`/projects/${projectId}/webhooks/${id}`);
      await fetchWebhooks();
    } catch {
      // ignore
    }
  };

  if (loading) {
    return (
      <div className="space-y-3">
        {[1, 2].map(i => (
          <div key={i} className="h-16 rounded-lg bg-muted/50 animate-pulse" />
        ))}
      </div>
    );
  }

  return (
    <div className="max-w-2xl space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-foreground flex items-center gap-2">
            <Webhook className="h-5 w-5 text-primary" />
            Webhooks
          </h1>
          <p className="text-sm text-muted-foreground mt-1">接收任务事件的 HTTP 回调通知</p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-lg text-sm hover:bg-primary/90 transition-colors"
        >
          <Plus size={16} />
          添加 Webhook
        </button>
      </div>

      {showCreate && (
        <div className="bg-surface-1 border border-border rounded-lg p-4 space-y-3">
          <div>
            <label className="block text-xs text-muted-foreground mb-1">URL</label>
            <input
              type="url"
              value={newUrl}
              onChange={(e) => setNewUrl(e.target.value)}
              placeholder="https://example.com/webhook"
              className="w-full bg-muted/50 border border-border rounded-lg px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:border-primary/50"
              autoFocus
            />
          </div>
          <div>
            <label className="block text-xs text-muted-foreground mb-1">签名密钥（可选）</label>
            <input
              type="text"
              value={newSecret}
              onChange={(e) => setNewSecret(e.target.value)}
              placeholder="用于 HMAC-SHA256 签名验证"
              className="w-full bg-muted/50 border border-border rounded-lg px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:border-primary/50"
            />
          </div>
          <div>
            <label className="block text-xs text-muted-foreground mb-1">事件</label>
            <select
              value={newEvents}
              onChange={(e) => setNewEvents(e.target.value)}
              className="w-full bg-muted/50 border border-border rounded-lg px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary/50"
            >
              {EVENT_OPTIONS.map(opt => (
                <option key={opt.value} value={opt.value}>{opt.label}</option>
              ))}
            </select>
          </div>
          <div className="flex gap-2 justify-end">
            <button onClick={() => setShowCreate(false)} className="px-3 py-1.5 text-sm text-muted-foreground hover:text-foreground">取消</button>
            <button
              onClick={handleCreate}
              disabled={creating || !newUrl.trim()}
              className="px-4 py-1.5 bg-primary text-primary-foreground rounded-lg text-sm hover:bg-primary/90 disabled:opacity-50"
            >
              {creating ? "创建中..." : "创建"}
            </button>
          </div>
        </div>
      )}

      {webhooks.length === 0 && !showCreate ? (
        <div className="rounded-xl border border-border bg-card p-8 text-center">
          <Webhook className="h-10 w-10 text-muted-foreground mx-auto mb-3 opacity-30" />
          <p className="text-muted-foreground">暂无 Webhook</p>
          <p className="text-xs text-muted-foreground mt-1">添加 Webhook 以接收任务完成、PR 创建等事件通知</p>
        </div>
      ) : (
        <div className="space-y-2">
          {webhooks.map(wh => (
            <div key={wh.id} className="flex items-center justify-between bg-surface-1 border border-border rounded-lg px-4 py-3">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <CheckCircle2 className={`h-3.5 w-3.5 ${wh.active ? "text-green-400" : "text-muted-foreground"}`} />
                  <p className="text-sm font-mono text-foreground truncate">{wh.url}</p>
                </div>
                <div className="flex items-center gap-2 mt-1">
                  <span className="px-1.5 py-0.5 rounded text-[9px] bg-primary/10 text-primary border border-primary/20">
                    {wh.events === "*" ? "所有事件" : wh.events}
                  </span>
                  <span className="text-[10px] text-muted-foreground">
                    {new Date(wh.createdAt).toLocaleDateString("zh-CN")}
                  </span>
                </div>
              </div>
              <button
                onClick={() => handleDelete(wh.id)}
                className="p-1.5 text-muted-foreground hover:text-red-400 transition-colors"
              >
                <Trash2 size={14} />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
