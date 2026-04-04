import { api } from "./api";

export interface Conversation {
  id: number;
  taskId: number;
  role: string;          // "user" | "assistant" | "system"
  content: string;
  metadata?: Record<string, unknown>;
  tokensUsed?: number;
  createdAt: string;
}

export interface SendMessageResponse {
  conversation: Conversation;
  status: string;        // "clarify" | "confirmed"
  metadata: Record<string, unknown>;
}

export async function sendMessage(
  projectId: number,
  taskId: number,
  content: string
): Promise<SendMessageResponse> {
  return api.post(`/projects/${projectId}/tasks/${taskId}/messages`, { content }, { timeout: 180000 });
}

export async function getHistory(
  projectId: number,
  taskId: number
): Promise<Conversation[]> {
  const res = await api.get<{ messages: Conversation[] }>(
    `/projects/${projectId}/tasks/${taskId}/messages`
  );
  return res.messages || [];
}

export async function triggerAnalysis(
  projectId: number,
  taskId: number
): Promise<SendMessageResponse> {
  return api.post(`/projects/${projectId}/tasks/${taskId}/analyze`, {}, { timeout: 180000 });
}

export interface PlanConfirmResponse {
  conversation: Conversation;
  status: string;        // "plan_review"
  planData: {
    title?: string;
    tasks?: Array<{
      order: number;
      title: string;
      description: string;
      type: string;
      files: string[];
      depends_on: number[];
      estimate_hours: number;
      requirement_ref?: string;
    }>;
    risk_level?: string;
    risk_factors?: string[];
    total_estimate_hours?: number;
    parallel_tracks?: number;
  };
}

export async function confirmPlan(
  projectId: number,
  taskId: number
): Promise<PlanConfirmResponse> {
  return api.post(`/projects/${projectId}/tasks/${taskId}/confirm`, {});
}

export async function approvePlan(
  projectId: number,
  taskId: number
): Promise<void> {
  return api.post(`/projects/${projectId}/tasks/${taskId}/approve-plan`, {});
}

export async function cancelTask(
  projectId: number,
  taskId: number
): Promise<void> {
  return api.post(`/projects/${projectId}/tasks/${taskId}/cancel`, {});
}
