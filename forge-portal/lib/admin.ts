import { api } from "./api";

export interface MetricsSnapshot {
  totalRequests: number;
  totalErrors: number;
  avgLatencyMs: number;
  uptime: string;
  requestsByPath: Record<string, number>;
  ai: {
    totalCalls: number;
    totalTokens: number;
    totalFallback: number;
    avgLatencyMs: number;
    callsByModel: Record<string, number>;
    tokensByModel: Record<string, number>;
  };
  tasks: {
    total: number;
    completed: number;
    failed: number;
    inProgress: number;
    byStatus: Record<string, number>;
    stageAvgMs: Record<string, number>;
  };
  sseActive: number;
}

export interface CostSummary {
  totalCalls: number;
  totalTokens: number;
  estimatedCost: number;
  models: { model: string; calls: number; tokens: number; cost: number }[];
}

export interface BudgetStatus {
  monthlyBudget: number;
  usedTokens: number;
  remainingPct: number;
  isExceeded: boolean;
}

export interface SystemInfo {
  version: string;
  go: string;
  platform: string;
  uptime: string;
}

export interface HealthStatus {
  status: string;
  database?: string;
  redis?: string;
  uptime?: string;
}

export async function getMetrics(): Promise<MetricsSnapshot> {
  const res = await fetch("/api/admin/metrics");
  return res.json();
}

export async function getMonthlyCosts(): Promise<CostSummary> {
  const res = await api.get<{ summary: CostSummary }>("/admin/costs");
  return res.summary;
}

export async function getBudgetStatus(): Promise<BudgetStatus> {
  const res = await api.get<{ budget: BudgetStatus }>("/admin/budget");
  return res.budget;
}

export async function getSystemInfo(): Promise<SystemInfo> {
  const res = await fetch("/api/system/info");
  return res.json();
}

export async function getHealth(): Promise<HealthStatus> {
  const res = await fetch("/api/health");
  return res.json();
}
