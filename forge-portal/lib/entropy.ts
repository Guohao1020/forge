import { api } from "./api";

export interface EntropyIssue {
  file: string;
  line: number;
  rule: string;
  message: string;
  severity: "critical" | "error" | "warning" | "info";
  category: "naming" | "dead_code" | "error_handling" | "complexity" | "style";
  suggestion?: string;
  autoFixable: boolean;
}

export interface EntropyScan {
  id: number;
  projectId: number;
  score: number;
  issueCount: number;
  issues: string; // JSON string
  scannedAt: string;
}

export interface QualityTrend {
  date: string;
  score: number;
  issueCount: number;
}

export interface EntropyConfig {
  projectId: number;
  enabled: boolean;
  schedule: "daily" | "weekly" | "monthly";
  autoFix: boolean;
  rules: string;
}

export async function getLatestScan(projectId: number): Promise<EntropyScan | null> {
  const res = await api.get<{ scan: EntropyScan | null }>(`/projects/${projectId}/entropy/latest`);
  return res.scan;
}

export async function listScans(projectId: number, limit = 20): Promise<EntropyScan[]> {
  const res = await api.get<{ scans: EntropyScan[] }>(`/projects/${projectId}/entropy/scans?limit=${limit}`);
  return res.scans;
}

export async function getQualityTrends(projectId: number, days = 30): Promise<QualityTrend[]> {
  const res = await api.get<{ trends: QualityTrend[] }>(`/projects/${projectId}/entropy/trends?days=${days}`);
  return res.trends;
}

export async function getEntropyConfig(projectId: number): Promise<EntropyConfig> {
  const res = await api.get<{ config: EntropyConfig }>(`/projects/${projectId}/entropy/config`);
  return res.config;
}

export async function updateEntropyConfig(
  projectId: number,
  config: Partial<{ enabled: boolean; schedule: string; autoFix: boolean; rules: string[] }>
): Promise<void> {
  await api.put(`/projects/${projectId}/entropy/config`, config);
}

export async function triggerScan(projectId: number): Promise<string> {
  const res = await api.post<{ workflow_id: string }>(`/projects/${projectId}/entropy/scan`, {});
  return res.workflow_id;
}

export const SEVERITY_CONFIG: Record<string, { label: string; color: string }> = {
  critical: { label: "严重", color: "text-red-400 bg-red-500/10 border-red-500/20" },
  error: { label: "错误", color: "text-orange-400 bg-orange-500/10 border-orange-500/20" },
  warning: { label: "警告", color: "text-yellow-400 bg-yellow-500/10 border-yellow-500/20" },
  info: { label: "提示", color: "text-blue-400 bg-blue-500/10 border-blue-500/20" },
};

export const CATEGORY_CONFIG: Record<string, { label: string; icon: string }> = {
  naming: { label: "命名规范", icon: "Aa" },
  dead_code: { label: "死代码", icon: "X" },
  error_handling: { label: "错误处理", icon: "!" },
  complexity: { label: "复杂度", icon: "~" },
  style: { label: "代码风格", icon: "#" },
};
