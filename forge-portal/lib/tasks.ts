import { api } from "./api";

export interface Task {
  id: number;
  tenant_id: number;
  project_id: number;
  title?: string;
  requirement: string;
  source: string;
  status: string;
  workflow_id?: string;
  workflow_run_id?: string;
  created_by: number;
  created_at: string;
  updated_at: string;
  completed_at?: string;
}

export interface TaskStep {
  id: number;
  task_id: number;
  name: string;
  step_type: string;
  status: string;
  started_at?: string;
  completed_at?: string;
  duration_ms?: number;
}

export interface TaskDetail {
  task: Task;
  steps: TaskStep[];
}

export interface TaskListResult {
  tasks: Task[];
  total: number;
}

export const STATUS_LABELS: Record<string, string> = {
  SUBMITTED: "已提交",
  ANALYZING: "分析中",
  PLANNING: "规划中",
  GENERATING: "生成中",
  REVIEWING: "审查中",
  TESTING: "测试中",
  DEPLOYING: "部署中",
  COMPLETED: "已完成",
  FAILED: "失败",
};

export const STATUS_COLORS: Record<string, string> = {
  SUBMITTED: "#8888A0",
  ANALYZING: "#8B5CF6",
  PLANNING: "#8B5CF6",
  GENERATING: "#06B6D4",
  REVIEWING: "#F59E0B",
  TESTING: "#3B82F6",
  DEPLOYING: "#10B981",
  COMPLETED: "#10B981",
  FAILED: "#EF4444",
};

// Kanban columns: group active statuses
export const KANBAN_COLUMNS = [
  { key: "pending", label: "待处理", statuses: ["SUBMITTED"] },
  { key: "active", label: "执行中", statuses: ["ANALYZING", "PLANNING", "GENERATING", "REVIEWING", "TESTING", "DEPLOYING"] },
  { key: "done", label: "已完成", statuses: ["COMPLETED"] },
  { key: "failed", label: "失败", statuses: ["FAILED"] },
];

export async function listTasks(projectId: string | number): Promise<TaskListResult> {
  return api.get<TaskListResult>(`/projects/${projectId}/tasks?page=1&page_size=100`);
}

export async function createTask(projectId: string | number, requirement: string, title?: string): Promise<TaskDetail> {
  return api.post<TaskDetail>(`/projects/${projectId}/tasks`, { requirement, title });
}

export async function getTaskDetail(projectId: string | number, taskId: string | number): Promise<TaskDetail> {
  return api.get<TaskDetail>(`/projects/${projectId}/tasks/${taskId}`);
}
