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
  return api.post(`/projects/${projectId}/tasks/${taskId}/messages`, { content });
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

export async function confirmPlan(
  projectId: number,
  taskId: number
): Promise<void> {
  return api.post(`/projects/${projectId}/tasks/${taskId}/confirm`, {});
}
