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

// ---- Empty-state suggestions (Stream 4c) ---------------------------------

export interface AgentSuggestion {
  text: string
  category?: "feature" | "fix" | "test" | "refactor"
}

interface SuggestionsResponse {
  suggestions: AgentSuggestion[]
  source: "heuristic" | "fallback"
  language?: string
}

/**
 * Fetch language-appropriate starter prompts for the Agent Terminal
 * empty state. Always succeeds — backend falls back to defaults on any
 * error so the empty state renders even without a tech stack hint.
 */
export async function getAgentSuggestions(
  projectId: number,
): Promise<SuggestionsResponse> {
  return api.get<SuggestionsResponse>(
    `/projects/${projectId}/agent/suggestions`,
  )
}

// ---- Clarification (Round 2, spec §2.9.2) -----------------------------------

/**
 * Submit a response to a pending `request_clarification` tool call.
 *
 * Spec §2.9.2.g: POST /api/sessions/{id}/clarify with JSON body
 * {tool_use_id, response}. Resolves the pending Future server-side
 * so the agent loop can continue.
 *
 * Returns on 204 No Content. Throws on 400 (bad input), 401
 * (unauthenticated), 403 (not your session), 404 (session not
 * found / no pending clarification matching tool_use_id), or 410
 * (clarification already resolved or timed out).
 */
export async function postClarification(
  sessionId: string,
  toolUseId: string,
  response: string,
): Promise<void> {
  const res = await fetch(
    `/api/sessions/${encodeURIComponent(sessionId)}/clarify`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({ tool_use_id: toolUseId, response }),
    },
  )
  if (!res.ok) {
    const body = await res.text().catch(() => "")
    throw new Error(`HTTP ${res.status}${body ? `: ${body}` : ""}`)
  }
}
