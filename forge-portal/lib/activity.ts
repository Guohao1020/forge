import { api } from "./api";

export interface ActivityItem {
  type: "task_created" | "task_completed" | "task_failed" | "task_running";
  projectId: number;
  projectName: string;
  title: string;
  status?: string;
  taskId?: number;
  timestamp: string;
}

export async function getRecentActivity(limit = 15): Promise<ActivityItem[]> {
  const res = await api.get<{ activity: ActivityItem[] }>(`/activity?limit=${limit}`);
  return res.activity;
}

export async function searchGlobal(query: string): Promise<{
  results: { type: string; id: number; projectId?: number; title: string; url: string }[];
  total: number;
}> {
  return api.get(`/search?q=${encodeURIComponent(query)}`);
}
