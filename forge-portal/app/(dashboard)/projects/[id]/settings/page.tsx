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
import Link from "next/link";
import { api } from "@/lib/api";
import { AlertTriangle, Lock, Globe, ExternalLink, CheckCircle2, Shield, Activity, TrendingUp, Webhook, BookOpen, Download } from "lucide-react";
import { GitHubIcon } from "@/components/icons";

interface TechStack {
  projectType?: string;
  subType?: string;
  branchStrategy?: string;
  deployTarget?: string;
  artifactType?: string;
  languages?: Record<string, number>;
  frameworks?: string[];
  testFrameworks?: string[];
  buildTools?: string[];
  confidence?: string;
}

interface Project {
  id: number;
  name: string;
  description: string;
  defaultBranch: string;
  codePlatform: string;
  codeRepoUrl: string;
  techStack?: TechStack;
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
  const [deleting, setDeleting] = useState(false);
  const [archiveConfirmName, setArchiveConfirmName] = useState("");
  const [deleteConfirmName, setDeleteConfirmName] = useState("");
  const [deleteRemoteRepo, setDeleteRemoteRepo] = useState(true);

  // Sync to remote state
  const [syncing, setSyncing] = useState(false);
  const [syncPrivate, setSyncPrivate] = useState(true);
  const [githubConnected, setGithubConnected] = useState<boolean | null>(null);

  // Entropy scan state
  const [entropyScan, setEntropyScan] = useState<{
    score: number;
    issueCount: number;
    scannedAt: string;
  } | null>(null);
  const [scanLoading, setScanLoading] = useState(false);

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
    api.get<{ scan: { score: number; issueCount: number; scannedAt: string } | null }>(`/projects/${projectId}/entropy/latest`)
      .then((res) => { if (res.scan) setEntropyScan(res.scan); })
      .catch(() => {});
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
      await api.post(`/projects/${projectId}/archive`, { confirmName: archiveConfirmName });
      router.push("/projects");
    } catch {
      setArchiving(false);
    }
  }

  async function handleDelete() {
    setDeleting(true);
    try {
      await api.delete(`/projects/${projectId}`, {
        confirmName: deleteConfirmName,
        deleteRemoteRepo,
      });
      router.push("/projects");
    } catch {
      setDeleting(false);
    }
  }

  if (!project) {
    return <div className="h-48 rounded-xl bg-card animate-pulse" />;
  }

  const hasRepo = !!project.codeRepoUrl;

  return (
    <div className="max-w-xl">
      <h1 className="text-2xl font-semibold tracking-tight mb-4">项目设置</h1>

      {/* Sub-page links */}
      <div className="flex gap-2 mb-6">
        <Link href={`/projects/${projectId}/settings/specs`}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-muted/50 border border-border text-muted-foreground hover:text-foreground hover:border-primary/30 transition-colors">
          <BookOpen size={12} /> 规范配置
        </Link>
        <Link href={`/projects/${projectId}/settings/webhooks`}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-muted/50 border border-border text-muted-foreground hover:text-foreground hover:border-primary/30 transition-colors">
          <Webhook size={12} /> Webhooks
        </Link>
        <a href={`/api/projects/${projectId}/export`}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-muted/50 border border-border text-muted-foreground hover:text-foreground hover:border-primary/30 transition-colors"
          download>
          <Download size={12} /> 导出备份
        </a>
      </div>

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

      {/* Project Type Detection (SP-1) */}
      {project.techStack && project.techStack.projectType && (
        <>
          <Separator className="my-8" />
          <div className="space-y-4">
            <h2 className="text-sm font-medium text-foreground">项目类型检测</h2>
            <div className="grid grid-cols-2 gap-3">
              <InfoCard label="项目类型" value={formatProjectType(project.techStack.projectType)} />
              <InfoCard label="子类型" value={project.techStack.subType || "—"} />
              <InfoCard label="分支策略" value={formatBranchStrategy(project.techStack.branchStrategy)} />
              <InfoCard label="部署目标" value={project.techStack.deployTarget || "—"} />
              <InfoCard label="制品类型" value={project.techStack.artifactType || "—"} />
              <InfoCard label="检测置信度" value={project.techStack.confidence || "—"} />
            </div>
            {project.techStack.frameworks && project.techStack.frameworks.length > 0 && (
              <div>
                <p className="text-xs text-muted-foreground mb-1.5">检测到的框架</p>
                <div className="flex flex-wrap gap-1">
                  {project.techStack.frameworks.map((f) => (
                    <span key={f} className="px-2 py-0.5 rounded text-xs bg-primary/10 text-primary border border-primary/20">
                      {f}
                    </span>
                  ))}
                </div>
              </div>
            )}
            {project.techStack.testFrameworks && project.techStack.testFrameworks.length > 0 && (
              <div>
                <p className="text-xs text-muted-foreground mb-1.5">测试框架</p>
                <div className="flex flex-wrap gap-1">
                  {project.techStack.testFrameworks.map((f) => (
                    <span key={f} className="px-2 py-0.5 rounded text-xs bg-green-500/10 text-green-400 border border-green-500/20">
                      {f}
                    </span>
                  ))}
                </div>
              </div>
            )}
          </div>
        </>
      )}

      {/* Code Quality / Entropy */}
      <Separator className="my-8" />
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-medium text-foreground flex items-center gap-2">
            <Shield className="h-4 w-4 text-primary" />
            代码质量
          </h2>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={scanLoading}
            onClick={async () => {
              setScanLoading(true);
              try {
                await api.post(`/projects/${projectId}/entropy/scan`, {});
                setSaveMsg("质量扫描已启动");
                setTimeout(() => setSaveMsg(""), 3000);
              } catch {
                setSaveMsg("扫描启动失败");
                setTimeout(() => setSaveMsg(""), 3000);
              } finally {
                setScanLoading(false);
              }
            }}
          >
            <Activity className="h-3.5 w-3.5 mr-1.5" />
            {scanLoading ? "启动中..." : "运行扫描"}
          </Button>
        </div>

        {entropyScan ? (
          <div className="grid grid-cols-3 gap-3">
            <div className="bg-muted/50 rounded-lg border border-border px-3 py-3 text-center">
              <p className="text-[10px] text-muted-foreground uppercase tracking-wider mb-1">质量分数</p>
              <p className={`text-2xl font-bold ${
                entropyScan.score >= 80 ? "text-green-400" :
                entropyScan.score >= 60 ? "text-yellow-400" : "text-red-400"
              }`}>
                {entropyScan.score}
              </p>
            </div>
            <div className="bg-muted/50 rounded-lg border border-border px-3 py-3 text-center">
              <p className="text-[10px] text-muted-foreground uppercase tracking-wider mb-1">问题数量</p>
              <p className="text-2xl font-bold text-foreground">{entropyScan.issueCount}</p>
            </div>
            <div className="bg-muted/50 rounded-lg border border-border px-3 py-3 text-center">
              <p className="text-[10px] text-muted-foreground uppercase tracking-wider mb-1">上次��描</p>
              <p className="text-sm text-foreground mt-1">
                {new Date(entropyScan.scannedAt).toLocaleDateString("zh-CN")}
              </p>
            </div>
          </div>
        ) : (
          <div className="rounded-lg border border-border bg-muted/50 p-4 text-center">
            <TrendingUp className="h-8 w-8 text-muted-foreground mx-auto mb-2 opacity-30" />
            <p className="text-sm text-muted-foreground">尚未运行过质量扫描</p>
            <p className="text-xs text-muted-foreground mt-1">点击「运行扫描」开始首次代码质量检测</p>
          </div>
        )}
      </div>

      <Separator className="my-8" />

      {/* Danger zone */}
      <div className="rounded-xl border border-destructive/30 bg-card p-5 space-y-6">
        <h2 className="text-sm font-medium text-destructive">危险区域</h2>

        {/* Archive */}
        <div className="space-y-2">
          <p className="text-sm text-muted-foreground">
            归档后项目将不再显示在项目大厅，数据保留但不可访问。
          </p>
          <AlertDialog onOpenChange={() => setArchiveConfirmName("")}>
            <AlertDialogTrigger
              disabled={archiving}
              className="inline-flex items-center justify-center rounded-md border border-destructive/50 px-4 py-2 text-sm font-medium text-destructive hover:bg-destructive/10 transition-colors disabled:opacity-50"
            >
              {archiving ? "归档中..." : "归档项目"}
            </AlertDialogTrigger>
            <AlertDialogContent className="bg-card border-border">
              <AlertDialogHeader>
                <AlertDialogTitle>确认归档项目？</AlertDialogTitle>
                <AlertDialogDescription>
                  项目 <strong>{project.name}</strong> 将被归档，从项目大厅移除。
                </AlertDialogDescription>
              </AlertDialogHeader>
              <div className="space-y-1.5 px-1">
                <Label htmlFor="archive-confirm" className="text-xs text-muted-foreground">
                  请输入项目名称 <strong className="text-foreground">{project.name}</strong> 以确认
                </Label>
                <Input
                  id="archive-confirm"
                  value={archiveConfirmName}
                  onChange={(e) => setArchiveConfirmName(e.target.value)}
                  placeholder={project.name}
                  className="bg-input border-border"
                />
              </div>
              <AlertDialogFooter>
                <AlertDialogCancel>取消</AlertDialogCancel>
                <AlertDialogAction
                  onClick={handleArchive}
                  disabled={archiveConfirmName !== project.name || archiving}
                  className="bg-destructive hover:bg-destructive/90 disabled:opacity-50"
                >
                  确认归档
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>

        <Separator />

        {/* Delete */}
        <div className="space-y-2">
          <p className="text-sm text-destructive font-medium">
            永久删除项目，此操作不可恢复。项目数据、任务记录、代码版本将被永久删除。
          </p>
          <AlertDialog onOpenChange={() => { setDeleteConfirmName(""); setDeleteRemoteRepo(true); }}>
            <AlertDialogTrigger
              disabled={deleting}
              className="inline-flex items-center justify-center rounded-md bg-destructive px-4 py-2 text-sm font-medium text-destructive-foreground hover:bg-destructive/90 transition-colors disabled:opacity-50"
            >
              {deleting ? "删除中..." : "永久删除项目"}
            </AlertDialogTrigger>
            <AlertDialogContent className="bg-card border-border">
              <AlertDialogHeader>
                <AlertDialogTitle className="text-destructive">永久删除项目</AlertDialogTitle>
                <AlertDialogDescription>
                  此操作不可恢复。项目的所有数据（任务、版本、规范、代码记录）将被永久删除。
                </AlertDialogDescription>
              </AlertDialogHeader>
              <div className="space-y-3 px-1">
                <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-3">
                  <div className="flex items-start gap-2.5">
                    <AlertTriangle className="h-4 w-4 text-destructive shrink-0 mt-0.5" />
                    <p className="text-sm text-destructive">
                      删除后无法恢复，请谨慎操作。
                    </p>
                  </div>
                </div>
                {hasRepo && (
                  <div className="flex items-center justify-between">
                    <label htmlFor="delete-remote" className="text-sm text-muted-foreground cursor-pointer">
                      同时删除 GitHub 远程仓库
                    </label>
                    <Switch
                      id="delete-remote"
                      checked={deleteRemoteRepo}
                      onCheckedChange={setDeleteRemoteRepo}
                    />
                  </div>
                )}
                <div className="space-y-1.5">
                  <Label htmlFor="delete-confirm" className="text-xs text-muted-foreground">
                    请输入项目名称 <strong className="text-foreground">{project.name}</strong> 以确认
                  </Label>
                  <Input
                    id="delete-confirm"
                    value={deleteConfirmName}
                    onChange={(e) => setDeleteConfirmName(e.target.value)}
                    placeholder={project.name}
                    className="bg-input border-border"
                  />
                </div>
              </div>
              <AlertDialogFooter>
                <AlertDialogCancel>取消</AlertDialogCancel>
                <AlertDialogAction
                  onClick={handleDelete}
                  disabled={deleteConfirmName !== project.name || deleting}
                  className="bg-destructive hover:bg-destructive/90 disabled:opacity-50"
                >
                  永久删除
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>
    </div>
  );
}

function InfoCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-muted/50 rounded-lg border border-border px-3 py-2">
      <p className="text-[10px] text-muted-foreground uppercase tracking-wider mb-0.5">{label}</p>
      <p className="text-sm text-foreground">{value}</p>
    </div>
  );
}

function formatProjectType(type: string): string {
  const map: Record<string, string> = {
    web_app: "Web 应用",
    mobile_app: "移动应用",
    desktop_app: "桌面应用",
    backend_api: "后端 API",
    library: "函数库",
    monorepo: "Monorepo",
    unknown: "未识别",
  };
  return map[type] || type;
}

function formatBranchStrategy(strategy?: string): string {
  const map: Record<string, string> = {
    trunk_based: "主干开发",
    github_flow: "GitHub Flow",
    release_train: "发布列车",
  };
  return strategy ? (map[strategy] || strategy) : "—";
}
