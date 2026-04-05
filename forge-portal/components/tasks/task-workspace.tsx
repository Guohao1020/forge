"use client";

import { useState } from "react";
import { TaskStep } from "@/lib/tasks";
import { CodePreviewPanel } from "@/components/code-preview/code-preview-panel";
import { ShikiCodeViewer } from "@/components/code-preview/shiki-code-viewer";
import { PlanOutputCard } from "./plan-output-card";
import { ReviewReportCard } from "./review-report-card";
import { Loader2, FileText, Rocket, FlaskConical, MessageSquareCheck, ChevronDown, ChevronRight, Terminal } from "lucide-react";
import { StreamingCodeView } from "./streaming-code-view";

interface TaskWorkspaceProps {
  selectedStep: TaskStep | null;
  steps: TaskStep[];
  requirement: string;
  streamingTokens?: string;
  isStreaming?: boolean;
}

interface PlanOutput {
  title?: string;
  tasks?: Array<{ order: number; title: string; files?: string[]; type?: string }>;
  risk_level?: string;
  risk_factors?: string[];
}

interface GenerateOutput {
  files: { path: string; content: string; action: string; language?: string }[];
  commit_message?: string;
  files_changed?: number;
  lines_added?: number;
  lines_deleted?: number;
}

interface ReviewOutput {
  passed: boolean;
  score: number;
  findings: Array<{ severity: string; file: string; line?: number; message: string; suggestion?: string }>;
  summary: string;
}

interface TestWritingOutput {
  test_files: Array<{ path: string; content: string; language?: string; framework?: string; covers_task?: number }>;
  test_count: number;
  framework: string;
  coverage_targets?: string[];
}

interface TestStepOutput {
  status: string;
  mock?: boolean;
  framework?: string;
  total?: number;
  passed?: number;
  failed?: number;
  coverage_pct?: number;
  duration_ms?: number;
  k8s_job?: string;
  logs?: string;
}

function tryParseOutput<T>(step: TaskStep): T | null {
  if (!step.output) return null;
  try {
    return JSON.parse(step.output) as T;
  } catch {
    return null;
  }
}

function RunningState({ message }: { message: string }) {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-muted-foreground/60">
      <Loader2 className="h-8 w-8 animate-spin text-primary mb-4" />
      <p className="text-sm">{message}</p>
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-muted-foreground/50">
      <FileText className="h-10 w-10 mb-3" />
      <p className="text-sm">选择左侧步骤查看详情</p>
    </div>
  );
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
function ComingSoonState() {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-muted-foreground/50">
      <Rocket className="h-10 w-10 mb-3" />
      <p className="text-sm">即将上线</p>
    </div>
  );
}

function TestStepCard({ output }: { output: TestStepOutput }) {
  const [logsOpen, setLogsOpen] = useState(false);

  return (
    <div className="space-y-4">
      <div className="rounded-xl border border-border bg-card p-5">
        <div className="flex items-center gap-3 mb-4">
          <FlaskConical className="h-5 w-5 text-primary" />
          <h3 className="text-sm font-medium">测试执行结果</h3>
          {output.status === "PASSED" ? (
            <span className="inline-flex items-center rounded-md bg-emerald-500/15 px-2 py-0.5 text-xs font-medium text-emerald-400">
              通过
            </span>
          ) : (
            <span className="inline-flex items-center rounded-md bg-red-500/15 px-2 py-0.5 text-xs font-medium text-red-400">
              失败
            </span>
          )}
          {output.mock && (
            <span className="inline-flex items-center rounded-md bg-amber-500/10 px-2 py-0.5 text-xs font-medium text-amber-400">
              Mock
            </span>
          )}
          {output.k8s_job && (
            <span className="inline-flex items-center rounded-md bg-blue-500/10 px-2 py-0.5 text-xs font-medium text-blue-400">
              K8s
            </span>
          )}
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          {output.total !== undefined && (
            <div className="rounded-lg bg-muted/30 p-3">
              <p className="text-xs text-muted-foreground/60 mb-1">总用例</p>
              <p className="text-lg font-semibold">{output.total}</p>
            </div>
          )}
          {output.passed !== undefined && (
            <div className="rounded-lg bg-muted/30 p-3">
              <p className="text-xs text-muted-foreground/60 mb-1">通过</p>
              <p className="text-lg font-semibold text-emerald-400">{output.passed}</p>
            </div>
          )}
          {output.failed !== undefined && output.failed > 0 && (
            <div className="rounded-lg bg-muted/30 p-3">
              <p className="text-xs text-muted-foreground/60 mb-1">失败</p>
              <p className="text-lg font-semibold text-red-400">{output.failed}</p>
            </div>
          )}
          {output.framework && (
            <div className="rounded-lg bg-muted/30 p-3">
              <p className="text-xs text-muted-foreground/60 mb-1">框架</p>
              <p className="text-lg font-semibold">{output.framework}</p>
            </div>
          )}
          {output.coverage_pct !== undefined && output.coverage_pct > 0 && (
            <div className="rounded-lg bg-muted/30 p-3">
              <p className="text-xs text-muted-foreground/60 mb-1">覆盖率</p>
              <p className="text-lg font-semibold">{output.coverage_pct.toFixed(1)}%</p>
            </div>
          )}
          {output.duration_ms !== undefined && output.duration_ms > 0 && (
            <div className="rounded-lg bg-muted/30 p-3">
              <p className="text-xs text-muted-foreground/60 mb-1">耗时</p>
              <p className="text-lg font-semibold">{(output.duration_ms / 1000).toFixed(1)}s</p>
            </div>
          )}
        </div>
        {output.k8s_job && (
          <div className="mt-3 text-xs text-muted-foreground/60">
            <span className="text-muted-foreground/50">K8s Job:</span>{" "}
            <span className="font-mono text-muted-foreground">{output.k8s_job}</span>
          </div>
        )}
      </div>

      {/* Execution logs (collapsible) */}
      {output.logs && (
        <div className="rounded-xl border border-border bg-card overflow-hidden">
          <button
            onClick={() => setLogsOpen(!logsOpen)}
            className="flex items-center gap-2 w-full px-4 py-3 text-sm font-medium text-muted-foreground hover:text-foreground/90 hover:bg-muted/30 transition-colors"
          >
            {logsOpen ? (
              <ChevronDown className="h-4 w-4" />
            ) : (
              <ChevronRight className="h-4 w-4" />
            )}
            <Terminal className="h-4 w-4" />
            <span>执行日志</span>
          </button>
          {logsOpen && (
            <div className="border-t border-border/50 bg-muted/20 p-4 max-h-96 overflow-y-auto">
              <pre className="text-xs font-mono text-emerald-400/80 whitespace-pre-wrap break-all leading-relaxed">
                {output.logs}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
export function TaskWorkspace({ selectedStep, steps, requirement, streamingTokens, isStreaming }: TaskWorkspaceProps) {
  if (!selectedStep) {
    return <EmptyState />;
  }

  const { step_type, status } = selectedStep;

  // ANALYZE step
  if (step_type === "ANALYZE") {
    const analysisOutput = tryParseOutput<{ summary?: string }>(selectedStep);
    return (
      <div className="space-y-4">
        <div className="rounded-xl border border-border bg-card p-5">
          <h3 className="text-sm font-medium mb-2">需求描述</h3>
          <p className="text-sm text-muted-foreground whitespace-pre-wrap">{requirement}</p>
        </div>
        {status === "RUNNING" && <RunningState message="AI 正在分析需求..." />}
        {status === "COMPLETED" && (
          <>
            {analysisOutput?.summary && (
              <div className="rounded-xl border border-border bg-card p-5">
                <h3 className="text-sm font-medium mb-2">分析摘要</h3>
                <p className="text-sm text-muted-foreground whitespace-pre-wrap">{analysisOutput.summary}</p>
              </div>
            )}
            <div className="rounded-xl border border-emerald-500/20 bg-emerald-500/5 p-5">
              <div className="flex items-center gap-2 mb-2">
                <MessageSquareCheck className="h-4 w-4 text-emerald-400" />
                <h3 className="text-sm font-medium text-emerald-400">需求确认</h3>
              </div>
              <p className="text-sm text-muted-foreground">
                需求已通过对话确认，AI 已理解并准备执行后续步骤。
              </p>
            </div>
          </>
        )}
      </div>
    );
  }

  // PLAN step
  if (step_type === "PLAN") {
    if (status === "RUNNING") return <RunningState message="AI 正在规划..." />;
    if (status === "COMPLETED") {
      const planOutput = tryParseOutput<PlanOutput>(selectedStep);
      if (planOutput) return <PlanOutputCard planOutput={planOutput} />;
    }
    if (status === "PENDING") return <EmptyState />;
    return <EmptyState />;
  }

  // TEST_WRITING step
  if (step_type === "TEST_WRITING") {
    if (status === "RUNNING") return <RunningState message="AI 正在生成测试用例..." />;
    if (status === "COMPLETED") {
      const output = tryParseOutput<TestWritingOutput>(selectedStep);
      if (output?.test_files?.length) {
        return (
          <div className="space-y-4">
            <div className="flex items-center gap-3">
              <h3 className="text-sm font-medium">测试用例预览</h3>
              {output.framework && (
                <span className="inline-flex items-center gap-1 rounded-md bg-primary/15 px-2 py-0.5 text-xs font-medium text-primary">
                  <FlaskConical className="h-3 w-3" />
                  {output.framework}
                </span>
              )}
              {output.test_count > 0 && (
                <span className="inline-flex items-center rounded-md bg-green-500/15 px-2 py-0.5 text-xs font-medium text-green-400">
                  {output.test_count} 个测试
                </span>
              )}
            </div>
            <div className="space-y-3">
              {output.test_files.map((file, i) => (
                <div key={i} className="rounded-xl border border-border bg-card overflow-hidden">
                  <ShikiCodeViewer
                    content={file.content}
                    fileName={file.path}
                    language={file.language}
                  />
                </div>
              ))}
            </div>
          </div>
        );
      }
    }
    if (status === "PENDING") return <EmptyState />;
    return <EmptyState />;
  }

  // GENERATE step
  if (step_type === "GENERATE") {
    if (status === "RUNNING") {
      if (isStreaming && streamingTokens) {
        return <StreamingCodeView tokens={streamingTokens} isStreaming={isStreaming} />;
      }
      return <RunningState message="AI 正在生成代码..." />;
    }
    if (status === "COMPLETED") {
      const output = tryParseOutput<GenerateOutput>(selectedStep);
      if (output?.files?.length) {
        return (
          <div>
            <CodePreviewPanel
              files={output.files}
              commitMessage={output.commit_message}
              filesChanged={output.files_changed}
              linesAdded={output.lines_added}
              linesDeleted={output.lines_deleted}
            />
          </div>
        );
      }
    }
    if (status === "PENDING") return <EmptyState />;
    return <EmptyState />;
  }

  // REVIEW step
  if (step_type === "REVIEW") {
    if (status === "RUNNING") return <RunningState message="AI 正在审查代码..." />;
    if (status === "COMPLETED") {
      const reviewOutput = tryParseOutput<ReviewOutput>(selectedStep);
      if (reviewOutput) return <ReviewReportCard reviewOutput={reviewOutput} />;
    }
    if (status === "PENDING") return <EmptyState />;
    return <EmptyState />;
  }

  // TEST step
  if (step_type === "TEST") {
    if (status === "RUNNING") return <RunningState message="正在执行测试..." />;
    if (status === "COMPLETED") {
      const output = tryParseOutput<TestStepOutput>(selectedStep);
      if (output) {
        return <TestStepCard output={output} />;
      }
    }
    if (status === "PENDING") return <EmptyState />;
    return <EmptyState />;
  }

  // DEPLOY step
  if (step_type === "DEPLOY") {
    if (status === "RUNNING") return <RunningState message="正在推送代码到 GitHub..." />;
    if (status === "COMPLETED") {
      const output = tryParseOutput<{ branch_name?: string; pr_number?: number; pr_url?: string; preview_url?: string; skipped?: boolean; error?: string }>(selectedStep);
      if (output && !output.skipped) {
        return (
          <div className="rounded-xl border border-border bg-card p-5">
            <div className="flex items-center gap-3 mb-3">
              <Rocket className="h-5 w-5 text-emerald-400" />
              <h3 className="text-sm font-medium">部署完成</h3>
            </div>
            <div className="space-y-2 text-sm text-muted-foreground">
              {output.branch_name && <p>分支: <span className="text-foreground/80 font-mono">{output.branch_name}</span></p>}
              {output.pr_url && (
                <p>PR: <a href={output.pr_url} target="_blank" rel="noopener noreferrer" className="text-primary hover:underline">#{output.pr_number}</a></p>
              )}
              {output.preview_url && (
                <p>预览: <a href={output.preview_url} target="_blank" rel="noopener noreferrer" className="text-primary hover:underline">{output.preview_url}</a></p>
              )}
            </div>
          </div>
        );
      }
      if (output?.skipped) {
        return (
          <div className="rounded-xl border border-border bg-card p-5">
            <div className="flex items-center gap-3 mb-3">
              <Rocket className="h-5 w-5 text-muted-foreground/50" />
              <h3 className="text-sm font-medium text-muted-foreground">部署已跳过</h3>
            </div>
            {output.error && <p className="text-sm text-muted-foreground/60">{output.error}</p>}
          </div>
        );
      }
    }
    if (status === "PENDING") return <EmptyState />;
    return <EmptyState />;
  }

  return <EmptyState />;
}
