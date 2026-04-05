import { api } from "./api";

export interface Webhook {
  id: number;
  projectId: number;
  url: string;
  events: string;
  active: boolean;
  createdAt: string;
}

export async function listWebhooks(projectId: number): Promise<Webhook[]> {
  const res = await api.get<{ webhooks: Webhook[] }>(`/projects/${projectId}/webhooks`);
  return res.webhooks;
}

export async function createWebhook(
  projectId: number,
  url: string,
  events: string,
  secret?: string
): Promise<number> {
  const res = await api.post<{ id: number }>(`/projects/${projectId}/webhooks`, {
    url,
    events,
    secret: secret || undefined,
  });
  return res.id;
}

export async function deleteWebhook(projectId: number, webhookId: number): Promise<void> {
  await api.delete(`/projects/${projectId}/webhooks/${webhookId}`);
}

export const WEBHOOK_EVENTS = [
  { value: "*", label: "All Events" },
  { value: "task.completed", label: "Task Completed" },
  { value: "task.failed", label: "Task Failed" },
  { value: "pr.created", label: "PR Created" },
  { value: "version.released", label: "Version Released" },
  { value: "entropy.scanned", label: "Quality Scan Done" },
] as const;
