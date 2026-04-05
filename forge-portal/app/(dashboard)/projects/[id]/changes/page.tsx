"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import { GitCommit, ChevronDown, ChevronRight } from "lucide-react";
import { listTasks, getTaskDetail } from "@/lib/tasks";
import type { Task, TaskDetail } from "@/lib/tasks";
import { ChangeSummary } from "@/components/changes/change-summary";
import { ChangeFileList } from "@/components/changes/change-file-list";
import type { ChangeFile } from "@/components/changes/change-file-list";
import { ChangeDiffView } from "@/components/changes/change-diff-view";

interface GenerateOutput {
  commit_message?: string;
  files?: Array<{
    path: string;
    content: string;
    action: string;
    language?: string;
  }>;
  files_changed?: number;
  lines_added?: number;
  lines_deleted?: number;
}

interface ReviewOutput {
  passed?: boolean;
  score?: number;
  summary?: string;
}

interface PlanOutput {
  risk_level?: string;
}

function parseStepOutput<T>(steps: TaskDetail["steps"], stepType: string): T | null {
  const step = steps.find((s) => s.step_type === stepType);
  if (!step?.output) return null;
  try {
    return JSON.parse(step.output) as T;
  } catch {
    return null;
  }
}

function countLines(content: string): number {
  if (!content) return 0;
  return content.split("\n").length;
}

function LoadingSkeleton() {
  return (
    <div className="space-y-4 animate-pulse">
      <div className="rounded-xl border border-border bg-card p-5">
        <div className="h-5 w-3/4 bg-muted/50 rounded" />
      </div>
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="rounded-xl border border-border bg-card p-4">
            <div className="h-3 w-16 bg-muted/50 rounded mb-3" />
            <div className="h-7 w-12 bg-muted/50 rounded" />
          </div>
        ))}
      </div>
      <div className="rounded-xl border border-border bg-card p-4 space-y-3">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="h-10 bg-muted/50 rounded" />
        ))}
      </div>
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-20 rounded-xl border border-border bg-card">
      <div className="w-12 h-12 rounded-xl flex items-center justify-center mb-3 bg-primary/10">
        <GitCommit className="h-6 w-6 text-primary" />
      </div>
      <h3 className="text-base font-medium mb-1">暂无变更记录</h3>
      <p className="text-sm text-muted-foreground">
        完成 AI 任务后变更将在此展示
      </p>
    </div>
  );
}

export default function ChangesPage() {
  const params = useParams();
  const projectId = params.id as string;

  const [loading, setLoading] = useState(true);
  const [generateOutput, setGenerateOutput] = useState<GenerateOutput | null>(null);
  const [reviewOutput, setReviewOutput] = useState<ReviewOutput | null>(null);
  const [planOutput, setPlanOutput] = useState<PlanOutput | null>(null);
  const [mrUrl, setMrUrl] = useState<string | undefined>();
  const [selectedPath, setSelectedPath] = useState<string | undefined>();
  const [filesExpanded, setFilesExpanded] = useState(true);

  const fetchLatestCompletedTask = useCallback(async () => {
    try {
      setLoading(true);
      const result = await listTasks(projectId);
      const completedTasks = result.tasks
        .filter((t: Task) => t.status === "COMPLETED")
        .sort(
          (a: Task, b: Task) =>
            new Date(b.completed_at || b.updated_at).getTime() -
            new Date(a.completed_at || a.updated_at).getTime()
        );

      if (completedTasks.length === 0) {
        setGenerateOutput(null);
        return;
      }

      const detail = await getTaskDetail(projectId, completedTasks[0].id);
      const genOut = parseStepOutput<GenerateOutput>(detail.steps, "GENERATE");
      const revOut = parseStepOutput<ReviewOutput>(detail.steps, "REVIEW");
      const planOut = parseStepOutput<PlanOutput>(detail.steps, "PLAN");

      setGenerateOutput(genOut);
      setReviewOutput(revOut);
      setPlanOutput(planOut);
      setMrUrl(completedTasks[0].mr_url);
    } catch (err) {
      console.error("Failed to fetch changes:", err);
      setGenerateOutput(null);
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    fetchLatestCompletedTask();
  }, [fetchLatestCompletedTask]);

  // Derive file list from generate output
  const files: ChangeFile[] =
    generateOutput?.files?.map((f) => ({
      path: f.path,
      action: f.action,
      language: f.language,
      content: f.content,
      linesCount: countLines(f.content),
    })) || [];

  const totalLinesAdded =
    generateOutput?.lines_added ??
    files.reduce((sum, f) => sum + f.linesCount, 0);
  const totalLinesDeleted = generateOutput?.lines_deleted ?? 0;

  const selectedFile = files.find((f) => f.path === selectedPath);

  if (loading) {
    return (
      <div>
        <h1 className="text-2xl font-semibold tracking-tight mb-6">变更</h1>
        <LoadingSkeleton />
      </div>
    );
  }

  if (!generateOutput || files.length === 0) {
    return (
      <div>
        <h1 className="text-2xl font-semibold tracking-tight mb-6">变更</h1>
        <EmptyState />
      </div>
    );
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold tracking-tight mb-6">变更</h1>

      <div className="space-y-4">
        {/* Layer 1: Summary */}
        <ChangeSummary
          commitMessage={generateOutput.commit_message || "AI 生成代码变更"}
          reviewScore={reviewOutput?.score}
          reviewPassed={reviewOutput?.passed}
          riskLevel={planOutput?.risk_level}
          filesChanged={generateOutput.files_changed ?? files.length}
          linesAdded={totalLinesAdded}
          linesDeleted={totalLinesDeleted}
          summary={reviewOutput?.summary}
          mrUrl={mrUrl}
        />

        {/* Layer 2: File list */}
        <div className="rounded-xl border border-border bg-card overflow-hidden">
          <button
            onClick={() => setFilesExpanded(!filesExpanded)}
            className="w-full flex items-center justify-between px-4 py-3 hover:bg-muted/20 transition-colors"
          >
            <div className="flex items-center gap-2">
              {filesExpanded ? (
                <ChevronDown className="h-4 w-4 text-muted-foreground/60" />
              ) : (
                <ChevronRight className="h-4 w-4 text-muted-foreground/60" />
              )}
              <span className="text-sm font-medium text-muted-foreground">变更文件</span>
              <span className="text-xs text-muted-foreground/60">({files.length})</span>
            </div>
          </button>
          {filesExpanded && (
            <div className="border-t border-border/50">
              <ChangeFileList
                files={files}
                selectedPath={selectedPath}
                onSelectFile={(path) =>
                  setSelectedPath(path === selectedPath ? undefined : path)
                }
              />
            </div>
          )}
        </div>

        {/* Layer 3: Diff view */}
        {selectedFile && (
          <ChangeDiffView
            fileName={selectedFile.path}
            language={selectedFile.language || "text"}
            originalContent=""
            modifiedContent={selectedFile.content}
            isNewFile={selectedFile.action === "create"}
            onClose={() => setSelectedPath(undefined)}
          />
        )}
      </div>
    </div>
  );
}
