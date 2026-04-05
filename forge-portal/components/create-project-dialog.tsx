"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { api } from "@/lib/api";
import { AlertTriangle, Lock, Globe, Layout, Code2, Smartphone, Layers } from "lucide-react";
import { GitHubIcon } from "@/components/icons";

interface ProjectTemplate {
  id: string;
  name: string;
  description: string;
  language: string;
  frameworks: string[];
  category: string;
}

const CATEGORY_ICONS: Record<string, typeof Code2> = {
  backend: Code2,
  frontend: Layout,
  fullstack: Layers,
  mobile: Smartphone,
};

interface CreateProjectDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: () => void;
}

export function CreateProjectDialog({
  open,
  onOpenChange,
  onCreated,
}: CreateProjectDialogProps) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [syncToRemote, setSyncToRemote] = useState(true);
  const [repoPrivate, setRepoPrivate] = useState(true);
  const [repoName, setRepoName] = useState("");
  const [githubConnected, setGithubConnected] = useState<boolean | null>(null);
  const [showWarning, setShowWarning] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [step, setStep] = useState<"template" | "details">("template");
  const [templates, setTemplates] = useState<ProjectTemplate[]>([]);
  const [selectedTemplate, setSelectedTemplate] = useState<string | null>(null);

  // Check GitHub connection status and load templates when dialog opens
  useEffect(() => {
    if (open) {
      api.get<{ connected: boolean }>("/auth/github/status")
        .then((res) => setGithubConnected(res.connected))
        .catch(() => setGithubConnected(false));
      api.get<{ templates: ProjectTemplate[] }>("/projects/templates")
        .then((res) => setTemplates(res.templates))
        .catch(() => {});
    }
  }, [open]);

  function reset() {
    setName("");
    setDescription("");
    setSyncToRemote(true);
    setRepoPrivate(true);
    setRepoName("");
    setShowWarning(false);
    setError("");
    setStep("template");
    setSelectedTemplate(null);
  }

  async function doCreate() {
    setLoading(true);
    setError("");
    try {
      await api.post("/projects", {
        name: name.trim(),
        description,
        syncToRemote,
        repoPrivate,
        repoName: repoName.trim() || undefined,
      });
      reset();
      onOpenChange(false);
      onCreated();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "创建失败");
      setShowWarning(false); // back to form on error
    } finally {
      setLoading(false);
    }
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) {
      setError("项目名称不能为空");
      return;
    }
    // If not syncing and warning not shown yet, show it
    if (!syncToRemote && !showWarning) {
      setShowWarning(true);
      return;
    }
    doCreate();
  }

  // Template selection step
  if (step === "template") {
    return (
      <Dialog open={open} onOpenChange={(v) => { if (!v) reset(); onOpenChange(v); }}>
        <DialogContent className="bg-card border-border text-foreground max-w-lg">
          <DialogHeader>
            <DialogTitle>选择项目模板</DialogTitle>
          </DialogHeader>
          <div className="space-y-2 max-h-80 overflow-auto">
            <button
              onClick={() => { setSelectedTemplate(null); setStep("details"); }}
              className={`w-full text-left px-3 py-2.5 rounded-lg border transition-colors hover:border-primary/30 ${
                !selectedTemplate ? "border-primary/50 bg-primary/5" : "border-border bg-muted/50"
              }`}
            >
              <p className="text-sm font-medium text-foreground">空白项目</p>
              <p className="text-xs text-muted-foreground">从零开始创建项目</p>
            </button>
            {templates.map((tmpl) => {
              const Icon = CATEGORY_ICONS[tmpl.category] || Code2;
              return (
                <button
                  key={tmpl.id}
                  onClick={() => {
                    setSelectedTemplate(tmpl.id);
                    setDescription(tmpl.description);
                    setStep("details");
                  }}
                  className="w-full text-left px-3 py-2.5 rounded-lg border border-border bg-muted/50 hover:border-primary/30 transition-colors"
                >
                  <div className="flex items-center gap-2">
                    <Icon className="h-4 w-4 text-primary shrink-0" />
                    <p className="text-sm font-medium text-foreground">{tmpl.name}</p>
                    <span className="text-[9px] text-muted-foreground px-1.5 py-0.5 bg-muted rounded">{tmpl.language}</span>
                  </div>
                  <p className="text-xs text-muted-foreground mt-0.5 pl-6">{tmpl.description}</p>
                </button>
              );
            })}
          </div>
        </DialogContent>
      </Dialog>
    );
  }

  // Warning confirmation view
  if (showWarning) {
    return (
      <Dialog open={open} onOpenChange={(v) => { if (!v) { setShowWarning(false); } onOpenChange(v); }}>
        <DialogContent className="bg-card border-border text-foreground">
          <DialogHeader>
            <DialogTitle>未同步到云端</DialogTitle>
          </DialogHeader>
          <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 p-4 space-y-3">
            <div className="flex items-start gap-3">
              <AlertTriangle className="h-5 w-5 text-amber-500 shrink-0 mt-0.5" />
              <div className="space-y-1">
                <p className="text-sm font-medium text-amber-500">数据丢失风险</p>
                <p className="text-sm text-muted-foreground">
                  未同步到远程代码仓库，AI 生成的代码将仅存储在服务器本地。如果服务器发生故障，代码可能丢失且无法恢复。
                </p>
                <p className="text-sm text-muted-foreground">
                  你可以稍后在项目设置中同步到 GitHub。
                </p>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => setShowWarning(false)}
              disabled={loading}
            >
              返回修改
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={doCreate}
              disabled={loading}
            >
              {loading ? "创建中..." : "仍然创建"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    );
  }

  const canSync = githubConnected === true;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="bg-card border-border text-foreground">
        <DialogHeader>
          <DialogTitle>新建项目</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="project-name">项目名称 *</Label>
            <Input
              id="project-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="例：用户服务、订单系统"
              className="bg-input border-border"
              autoFocus
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="project-desc">描述（可选）</Label>
            <Textarea
              id="project-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="简要描述项目的业务用途..."
              className="bg-input border-border resize-none"
              rows={3}
            />
          </div>

          {/* Sync to GitHub toggle */}
          <div className="rounded-lg border border-border bg-card p-4 space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <GitHubIcon className="h-4 w-4" />
                <span className="text-sm font-medium">同步到 GitHub</span>
              </div>
              <Switch
                checked={syncToRemote}
                onCheckedChange={setSyncToRemote}
                disabled={!canSync}
              />
            </div>

            {!canSync && (
              <p className="text-xs text-muted-foreground">
                请先在项目大厅点击「接入代码平台」连接 GitHub 账户。
              </p>
            )}

            {syncToRemote && canSync && (
              <div className="space-y-3 pt-1">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    {repoPrivate ? (
                      <>
                        <Lock className="h-3.5 w-3.5" />
                        <span>私有仓库</span>
                      </>
                    ) : (
                      <>
                        <Globe className="h-3.5 w-3.5" />
                        <span>公开仓库</span>
                      </>
                    )}
                  </div>
                  <Switch
                    checked={repoPrivate}
                    onCheckedChange={setRepoPrivate}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="repo-name" className="text-xs text-muted-foreground">仓库名称</Label>
                  <Input
                    id="repo-name"
                    value={repoName}
                    onChange={(e) => setRepoName(e.target.value.replace(/[^a-zA-Z0-9-]/g, "-"))}
                    placeholder={name.trim() ? name.trim().replace(/[^a-zA-Z0-9-]/g, "-").replace(/-+/g, "-").replace(/^-|-$/g, "") || "my-project" : "my-project"}
                    className="bg-input border-border font-mono text-sm h-8"
                  />
                  <p className="text-xs text-muted-foreground">
                    仅支持英文字母、数字和连字符。留空则自动生成。
                  </p>
                </div>
              </div>
            )}

            {!syncToRemote && (
              <p className="text-xs text-amber-500/80">
                仅服务器本地存储，存在数据丢失风险。
              </p>
            )}
          </div>

          {error && (
            <p className="text-sm text-destructive">{error}</p>
          )}
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => onOpenChange(false)}
              disabled={loading}
            >
              取消
            </Button>
            <Button type="submit" disabled={loading}>
              {loading ? "创建中..." : "创建"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
