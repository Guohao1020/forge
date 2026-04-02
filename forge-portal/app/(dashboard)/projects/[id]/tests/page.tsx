"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import {
  FlaskConical,
  Globe,
  Workflow,
  RotateCcw,
  Loader2,
  CheckCircle2,
  XCircle,
  Clock,
} from "lucide-react";
import { listTasks, getTaskDetail, getTestResults, Task, TaskStep, TestResult } from "@/lib/tasks";
import { TestLayerCard } from "@/components/tests/test-layer-card";

interface GeneratedFile {
  path: string;
  content: string;
  language?: string;
  action?: string;
}

const TEST_PATH_PATTERNS = [
  /\/tests?\//i,
  /\/spec\//i,
  /test_/i,
  /_test\./i,
  /\.test\./i,
  /\.spec\./i,
  /Test\./,
];

function isTestFile(file: GeneratedFile): boolean {
  if (file.action === "test") return true;
  return TEST_PATH_PATTERNS.some((pattern) => pattern.test(file.path));
}

function parseGenerateOutput(steps: TaskStep[]): GeneratedFile[] {
  const generateStep = steps.find(
    (s) => s.step_type === "GENERATE" && s.status === "COMPLETED" && s.output
  );
  if (!generateStep?.output) return [];

  try {
    const parsed = JSON.parse(generateStep.output);
    const files: GeneratedFile[] = Array.isArray(parsed)
      ? parsed
      : parsed.files ?? [];
    return files;
  } catch {
    return [];
  }
}

function TestResultSummary({ results }: { results: TestResult[] }) {
  if (results.length === 0) return null;

  const totalPassed = results.reduce((sum, r) => sum + r.passed, 0);
  const totalFailed = results.reduce((sum, r) => sum + r.failed, 0);
  const totalCases = results.reduce((sum, r) => sum + r.totalCases, 0);
  const totalDuration = results.reduce((sum, r) => sum + (r.durationMs || 0), 0);
  const avgCoverage = results.reduce((sum, r) => sum + (r.coveragePct || 0), 0) / results.length;
  const allPassed = results.every((r) => r.status === "PASSED");

  return (
    <div className="rounded-xl border border-white/10 bg-white/[0.02] p-5 mb-6">
      <div className="flex items-center gap-3 mb-4">
        {allPassed ? (
          <CheckCircle2 className="h-5 w-5 text-emerald-400" />
        ) : (
          <XCircle className="h-5 w-5 text-red-400" />
        )}
        <h2 className="text-lg font-semibold">
          {allPassed ? "所有测试通过" : "部分测试失败"}
        </h2>
      </div>
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <div className="rounded-lg bg-white/[0.03] p-3">
          <p className="text-xs text-white/40 mb-1">总用例</p>
          <p className="text-xl font-semibold">{totalCases}</p>
        </div>
        <div className="rounded-lg bg-white/[0.03] p-3">
          <p className="text-xs text-white/40 mb-1">通过</p>
          <p className="text-xl font-semibold text-emerald-400">{totalPassed}</p>
        </div>
        <div className="rounded-lg bg-white/[0.03] p-3">
          <p className="text-xs text-white/40 mb-1">失败</p>
          <p className="text-xl font-semibold text-red-400">{totalFailed}</p>
        </div>
        <div className="rounded-lg bg-white/[0.03] p-3">
          <p className="text-xs text-white/40 mb-1">覆盖率</p>
          <p className="text-xl font-semibold">{avgCoverage.toFixed(1)}%</p>
        </div>
      </div>
      <div className="flex items-center gap-2 mt-3 text-xs text-white/40">
        <Clock size={12} />
        <span>耗时 {(totalDuration / 1000).toFixed(1)}s</span>
        {results[0]?.framework && (
          <>
            <span className="text-white/20">|</span>
            <span>框架: {results[0].framework}</span>
          </>
        )}
      </div>
    </div>
  );
}

export default function TestsPage() {
  const params = useParams();
  const projectId = params.id as string;
  const [loading, setLoading] = useState(true);
  const [testFiles, setTestFiles] = useState<GeneratedFile[]>([]);
  const [testResults, setTestResults] = useState<TestResult[]>([]);
  const [error, setError] = useState<string | null>(null);

  const fetchTestData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      // Fetch all tasks for this project
      const { tasks } = await listTasks(projectId);

      // Find the latest completed task
      const completedTasks = (tasks ?? [])
        .filter((t: Task) => t.status === "COMPLETED")
        .sort(
          (a: Task, b: Task) =>
            new Date(b.completed_at || b.updated_at).getTime() -
            new Date(a.completed_at || a.updated_at).getTime()
        );

      if (completedTasks.length === 0) {
        setTestFiles([]);
        setTestResults([]);
        setLoading(false);
        return;
      }

      const latestTask = completedTasks[0];

      // Fetch test results and task detail in parallel
      const [detail, results] = await Promise.all([
        getTaskDetail(projectId, latestTask.id),
        getTestResults(projectId, latestTask.id),
      ]);

      const allFiles = parseGenerateOutput(detail.steps);
      const filtered = allFiles.filter(isTestFile);
      setTestFiles(filtered);
      setTestResults(results);
    } catch {
      setError("加载测试数据失败");
      setTestFiles([]);
      setTestResults([]);
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    fetchTestData();
  }, [fetchTestData]);

  if (loading) {
    return (
      <div>
        <h1 className="text-2xl font-semibold tracking-tight mb-6">
          测试报告
        </h1>
        <div className="flex flex-col items-center justify-center py-20 rounded-xl border border-white/10 bg-white/[0.02]">
          <Loader2 className="h-6 w-6 text-white/30 animate-spin mb-3" />
          <p className="text-sm text-white/40">加载中...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div>
        <h1 className="text-2xl font-semibold tracking-tight mb-6">
          测试报告
        </h1>
        <div className="flex flex-col items-center justify-center py-20 rounded-xl border border-white/10 bg-white/[0.02]">
          <p className="text-sm text-red-400">{error}</p>
        </div>
      </div>
    );
  }

  // Build a map of test results by layer
  const resultsByLayer: Record<string, TestResult> = {};
  for (const r of testResults) {
    resultsByLayer[r.layer] = r;
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold tracking-tight mb-6">测试报告</h1>

      {testResults.length > 0 && <TestResultSummary results={testResults} />}

      <div className="space-y-3">
        <TestLayerCard
          title="单元测试"
          icon={<FlaskConical size={18} />}
          status="available"
          testFiles={testFiles}
          passCount={resultsByLayer["UNIT"]?.passed}
          failCount={resultsByLayer["UNIT"]?.failed}
          coverage={resultsByLayer["UNIT"]?.coveragePct}
          defaultOpen={testFiles.length > 0 || !!resultsByLayer["UNIT"]}
        />
        <TestLayerCard
          title="接口测试"
          icon={<Globe size={18} />}
          status={resultsByLayer["API"] ? "available" : "coming_soon"}
          passCount={resultsByLayer["API"]?.passed}
          failCount={resultsByLayer["API"]?.failed}
          coverage={resultsByLayer["API"]?.coveragePct}
        />
        <TestLayerCard
          title="集成测试"
          icon={<Workflow size={18} />}
          status={resultsByLayer["INTEGRATION"] ? "available" : "coming_soon"}
          passCount={resultsByLayer["INTEGRATION"]?.passed}
          failCount={resultsByLayer["INTEGRATION"]?.failed}
          coverage={resultsByLayer["INTEGRATION"]?.coveragePct}
        />
        <TestLayerCard
          title="回归测试"
          icon={<RotateCcw size={18} />}
          status={resultsByLayer["E2E"] ? "available" : "coming_soon"}
          passCount={resultsByLayer["E2E"]?.passed}
          failCount={resultsByLayer["E2E"]?.failed}
          coverage={resultsByLayer["E2E"]?.coveragePct}
        />
      </div>
    </div>
  );
}
