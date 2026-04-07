import { api } from "./api"

// Mirrors forge-core/internal/module/agent/model.go AgentSession.
export interface AgentSession {
  id: string
  tenant_id: number
  project_id: number
  task_id?: number | null
  title?: string | null
  created_by: number
  created_at: string
  updated_at: string
  archived: boolean
  last_message_at?: string | null
}

// Mirrors forge-core AgentMessage. `data` is base64-encoded JSONB from
// pgx — the frontend decodes it when needed via JSON.parse.
export interface AgentMessageRow {
  id: number
  session_id: string
  redis_id?: string | null
  event_type: string
  role?: "user" | "assistant" | "system" | "tool" | null
  content?: string | null
  tool_name?: string | null
  data: Record<string, unknown> | string
  correlation_id?: string | null
  created_at: string
}

interface ListSessionsResponse {
  sessions: AgentSession[]
  total: number
}

interface ListMessagesResponse {
  messages: AgentMessageRow[]
  total: number
}

// ---- Session CRUD ---------------------------------------------------------

export async function listAgentSessions(
  projectId: number,
  limit = 50,
): Promise<ListSessionsResponse> {
  return api.get<ListSessionsResponse>(
    `/projects/${projectId}/agent/sessions?limit=${limit}`,
  )
}

export async function createAgentSession(
  projectId: number,
  opts: { title?: string; task_id?: number } = {},
): Promise<AgentSession> {
  return api.post<AgentSession>(
    `/projects/${projectId}/agent/sessions`,
    opts,
  )
}

export async function archiveAgentSession(
  projectId: number,
  sessionId: string,
): Promise<void> {
  await api.delete(`/projects/${projectId}/agent/sessions/${sessionId}`)
}

export async function renameAgentSession(
  projectId: number,
  sessionId: string,
  title: string,
): Promise<void> {
  await api.patch(`/projects/${projectId}/agent/sessions/${sessionId}`, { title })
}

// ---- Message history ------------------------------------------------------

/**
 * Fetch the durable message log for a session. Called on page load to
 * hydrate the chat UI before subscribing to Redis SSE for new events.
 */
export async function listSessionMessages(
  projectId: number,
  sessionId: string,
  limit = 500,
): Promise<ListMessagesResponse> {
  return api.get<ListMessagesResponse>(
    `/projects/${projectId}/agent/sessions/${sessionId}/messages?limit=${limit}`,
  )
}
