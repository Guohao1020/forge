"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { api } from "@/lib/api";
import { AlertTriangle, Lock, Globe, ExternalLink, CheckCircle2 } from "lucide-react";
import { GitHubIcon } from "@/components/icons";

interface Project {
  id: number;
  name: string;
  description: string;
  defaultBranch: string;
  codePlatform: string;
  codeRepoUrl: string;
}

export default function ProjectSettingsPage() {
  const params = useParams();
  const router = useRouter();
  const projectId = params.id as string;

  const [project, setProject] = useState<Project | null>(null);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [defaultBranch, setDefaultBranch] = useState("");
  const [saving, setSaving] = useState(false);
  const [saveMsg, setSaveMsg] = useState("");
  const [archiving, setArchiving] = useState(false);

  // Sync to remote state
  const [syncing, setSyncing] = useState(false);
  const [syncPrivate, setSyncPrivate] = useState(true);
  const [githubConnected, setGithubConnected] = useState<boolean | null>(null);

  useEffect(() => {
    api.get<Project>(`/projects/${projectId}`).then((p) => {
      setProject(p);
      setName(p.name);
      setDescription(p.description);
      setDefaultBranch(p.defaultBranch);
    });
    api.get<{ connected: boolean }>("/auth/github/status")
      .then((res) => setGithubConnected(res.connected))
      .catch(() => setGithubConnected(false));
  }, [projectId]);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setSaveMsg("");
    try {
      await api.put(`/projects/${projectId}`, { name, description, defaultBranch });
      setSaveMsg("保存成功");
      setTimeout(() => setSaveMsg(""), 3000);
    } catch (err: unknown) {
      setSaveMsg(err instanceof Error ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  async function handleSyncToRemote() {
    setSyncing(true);
    try {
      const updated = await api.post<Project>(`/projects/${projectId}/sync`, {
        private: syncPrivate,
      });
      setProject(updated);
      setDefaultBranch(updated.defaultBranch);
    } catch (err: unknown) {
      setSaveMsg(err instanceof Error ? err.message : "同步失败");
      setTimeout(() => setSaveMsg(""), 5000);
    } finally {
      setSyncing(false);
    }
  }

  async function handleArchive() {
    setArchiving(true);
    try {
      await api.delete(`/projects/${projectId}`);
      router.push("/projects");
    } catch {
      setArchiving(false);
    }
  }

  if (!project) {
    return <div className="h-48 rounded-xl bg-card animate-pulse" />;
  }

  const hasRepo = !!project.codeRepoUrl;

  return (
    <div className="max-w-xl">
      <h1 className="text-2xl font-semibold tracking-tight mb-6">项目设置</h1>

      <form onSubmit={handleSave} className="space-y-5">
        <div className="rounded-xl border border-border bg-card p-5 space-y-4">
          <h2 className="text-sm font-medium">基本信息</h2>
          <div className="space-y-2">
            <Label htmlFor="s-name">项目名称</Label>
            <Input
              id="s-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="bg-input border-border"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="s-desc">描述</Label>
            <Textarea
              id="s-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              className="bg-input border-border resize-none"
              rows={3}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="s-branch">默认分支</Label>
            <Input
              id="s-branch"
              value={defaultBranch}
              onChange={(e) => setDefaultBranch(e.target.value)}
              className="bg-input border-border font-mono text-sm"
            />
          </div>
        </div>

        <div className="flex items-center gap-3">
          <Button type="submit" disabled={saving}>
            {saving ? "保存中..." : "保存更改"}
          </Button>
          {saveMsg && (
            <span className={`text-sm ${saveMsg === "保存成功" ? "text-green-500" : "text-destructive"}`}>
              {saveMsg}
            </span>
          )}
        </div>
      </form>

      {/* Code repository section */}
      <div className="mt-5 rounded-xl border border-border bg-card p-5 space-y-4">
        <h2 className="text-sm font-medium">代码仓库</h2>

        {hasRepo ? (
          /* Already connected */
          <div className="space-y-3">
            <div className="flex items-center gap-2 text-sm">
              <CheckCircle2 className="h-4 w-4 text-green-500" />
              <span className="text-green-500">已同步到 GitHub</span>
            </div>
            <a
              href={project.codeRepoUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 text-sm text-primary hover:underline"
            >
              <GitHubIcon className="h-4 w-4" />
              <span className="font-mono">{project.codeRepoUrl}</span>
              <ExternalLink className="h-3 w-3" />
            </a>
          </div>
        ) : (
          /* Not connected — offer sync */
          <div className="space-y-4">
            <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 p-3">
              <div className="flex items-start gap-2.5">
                <AlertTriangle className="h-4 w-4 text-amber-500 shrink-0 mt-0.5" />
                <p className="text-sm text-muted-foreground">
                  未同步到远程仓库，代码仅在服务器本地，存在丢失风险。
                </p>
              </div>
            </div>

            {githubConnected ? (
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    {syncPrivate ? (
                      <>
                        <Lock className="h-3.5 w-3.5" />
                        <span>创建为私有仓库</span>
                      </>
                    ) : (
                      <>
                        <Globe className="h-3.5 w-3.5" />
                        <span>创建为公开仓库</span>
                      </>
                    )}
                  </div>
                  <Switch
                    checked={syncPrivate}
                    onCheckedChange={setSyncPrivate}
                  />
                </div>
                <Button
                  type="button"
                  onClick={handleSyncToRemote}
                  disabled={syncing}
                  className="w-full"
                >
                  <GitHubIcon className="h-4 w-4 mr-2" />
                  {syncing ? "正在创建仓库..." : "同步到 GitHub"}
                </Button>
              </div>
            ) : (
              <p className="text-xs text-muted-foreground">
                请先在项目大厅点击「接入代码平台」连接 GitHub 账户。
              </p>
            )}
          </div>
        )}
      </div>

      <Separator className="my-8" />

      {/* Danger zone */}
      <div className="rounded-xl border border-destructive/30 bg-card p-5">
        <h2 className="text-sm font-medium text-destructive mb-1">危险区域</h2>
        <p className="text-sm text-muted-foreground mb-4">
          归档后项目将不再显示在项目大厅，但数据不会删除。
        </p>
        <AlertDialog>
          <AlertDialogTrigger
            disabled={archiving}
            className="inline-flex items-center justify-center rounded-md bg-destructive px-4 py-2 text-sm font-medium text-white hover:bg-destructive/90 transition-colors disabled:opacity-50"
          >
            {archiving ? "归档中..." : "归档项目"}
          </AlertDialogTrigger>
          <AlertDialogContent className="bg-card border-border">
            <AlertDialogHeader>
              <AlertDialogTitle>确认归档项目？</AlertDialogTitle>
              <AlertDialogDescription>
                项目 <strong>{project.name}</strong> 将被归档，从项目大厅移除。此操作可以通过数据库恢复。
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>取消</AlertDialogCancel>
              <AlertDialogAction onClick={handleArchive} className="bg-destructive hover:bg-destructive/90">
                确认归档
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </div>
  );
}
