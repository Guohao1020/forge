"use client";

import { TaskStep } from "@/lib/tasks";
import { CodePreviewPanel } from "@/components/code-preview/code-preview-panel";
import { ShikiCodeViewer } from "@/components/code-preview/shiki-code-viewer";
import { PlanOutputCard } from "./plan-output-card";
import { ReviewReportCard } from "./review-report-card";
import { Loader2, FileText, Rocket, FlaskConical } from "lucide-react";
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
  commitMessage?: string;
  filesChanged?: number;
  linesAdded?: number;
  linesDeleted?: number;
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
    <div className="flex flex-col items-center justify-center py-20 text-white/40">
      <Loader2 className="h-8 w-8 animate-spin text-primary mb-4" />
      <p className="text-sm">{message}</p>
    </div>
  );
}

function EmptyState() {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-white/30">
      <FileText className="h-10 w-10 mb-3" />
      <p className="text-sm">选择左侧步骤查看详情</p>
    </div>
  );
}

function ComingSoonState() {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-white/30">
      <Rocket className="h-10 w-10 mb-3" />
      <p className="text-sm">即将上线</p>
    </div>
  );
}

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
        <div className="rounded-xl border border-white/10 bg-card p-5">
          <h3 className="text-sm font-medium mb-2">需求描述</h3>
          <p className="text-sm text-white/70 whitespace-pre-wrap">{requirement}</p>
        </div>
        {status === "RUNNING" && <RunningState message="AI 正在分析需求..." />}
        {status === "COMPLETED" && analysisOutput?.summary && (
          <div className="rounded-xl border border-white/10 bg-card p-5">
            <h3 className="text-sm font-medium mb-2">分析摘要</h3>
            <p className="text-sm text-white/60 whitespace-pre-wrap">{analysisOutput.summary}</p>
          </div>
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
                <div key={i} className="rounded-xl border border-white/10 bg-card overflow-hidden">
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
            <h3 className="text-sm font-medium mb-3">生成代码预览</h3>
            <CodePreviewPanel
              files={output.files}
              commitMessage={output.commitMessage}
              filesChanged={output.filesChanged}
              linesAdded={output.linesAdded}
              linesDeleted={output.linesDeleted}
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

  // TEST / DEPLOY / other future steps
  if (step_type === "TEST" || step_type === "DEPLOY") {
    if (status === "RUNNING") return <RunningState message="执行中..." />;
    return <ComingSoonState />;
  }

  return <EmptyState />;
}
