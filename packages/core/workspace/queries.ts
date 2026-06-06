import { queryOptions } from "@tanstack/react-query";
import { api } from "../api";
import type { Agent, Squad, Workspace } from "../types";

export const workspaceKeys = {
  all: (wsId: string) => ["workspaces", wsId] as const,
  list: () => ["workspaces", "list"] as const,
  members: (wsId: string) => ["workspaces", wsId, "members"] as const,
  invitations: (wsId: string) => ["workspaces", wsId, "invitations"] as const,
  myInvitations: () => ["invitations", "mine"] as const,
  agents: (wsId: string) => ["workspaces", wsId, "agents"] as const,
  squads: (wsId: string) => ["workspaces", wsId, "squads"] as const,
  // Per-squad member status. Lives under the workspace key tree so
  // workspace switches naturally drop the cache, and so a broad
  // `["workspaces", wsId, "squads"]` invalidation covers it.
  squadMemberStatus: (wsId: string, squadId: string) =>
    ["workspaces", wsId, "squads", squadId, "members-status"] as const,
  skills: (wsId: string) => ["workspaces", wsId, "skills"] as const,
  forgeStandards: (wsId: string) => ["workspaces", wsId, "forge-standards"] as const,
  forgeChecks: (wsId: string) => ["workspaces", wsId, "forge-checks"] as const,
  forgeReviewConfig: (wsId: string) =>
    ["workspaces", wsId, "forge-review-config"] as const,
  forgeEntropyScans: (wsId: string) =>
    ["workspaces", wsId, "forge-entropy-scans"] as const,
  forgeHealth: (wsId: string) => ["workspaces", wsId, "forge-health"] as const,
  forgeHealthTrends: (wsId: string) => ["workspaces", wsId, "forge-health-trends"] as const,
  mcpRegistry: (wsId: string) => ["workspaces", wsId, "mcp-registry"] as const,
  assigneeFrequency: (wsId: string) => ["workspaces", wsId, "assignee-frequency"] as const,
};

export function workspaceListOptions() {
  return queryOptions({
    queryKey: workspaceKeys.list(),
    queryFn: () => api.listWorkspaces(),
  });
}

/** Resolves the workspace whose slug matches, from the cached workspace list. */
export function workspaceBySlugOptions(slug: string) {
  return queryOptions({
    ...workspaceListOptions(),
    select: (list: Workspace[]) => list.find((w) => w.slug === slug) ?? null,
  });
}

export function memberListOptions(wsId: string) {
  return queryOptions({
    queryKey: workspaceKeys.members(wsId),
    queryFn: () => api.listMembers(wsId),
  });
}

export function agentListOptions(wsId: string) {
  return queryOptions({
    queryKey: workspaceKeys.agents(wsId),
    queryFn: () =>
      api.listAgents({ workspace_id: wsId, include_archived: true }),
  });
}

export function squadListOptions(wsId: string) {
  return queryOptions<Squad[]>({
    queryKey: workspaceKeys.squads(wsId),
    queryFn: () => api.listSquads(),
    enabled: !!wsId,
  });
}

// Per-squad members status snapshot. The freshness signal is the WS task /
// agent / runtime invalidation wired in use-realtime-sync (which broadly
// invalidates `["workspaces", wsId, "squads"]`); the staleTime is a
// tab-focus safety net.
export function squadMemberStatusOptions(wsId: string, squadId: string) {
  return queryOptions({
    queryKey: workspaceKeys.squadMemberStatus(wsId, squadId),
    queryFn: () => api.getSquadMemberStatus(squadId),
    enabled: !!wsId && !!squadId,
    staleTime: 30 * 1000,
    refetchOnWindowFocus: true,
  });
}

export function skillListOptions(wsId: string) {
  return queryOptions({
    queryKey: workspaceKeys.skills(wsId),
    queryFn: () => api.listSkills(),
  });
}

export function skillDetailOptions(wsId: string, skillId: string) {
  return queryOptions({
    queryKey: [...workspaceKeys.skills(wsId), skillId] as const,
    queryFn: () => api.getSkill(skillId),
    enabled: !!skillId,
  });
}

export function forgeStandardListOptions(wsId: string) {
  return queryOptions({
    queryKey: workspaceKeys.forgeStandards(wsId),
    queryFn: () => api.listForgeStandards(),
  });
}

export function forgeCheckListOptions(wsId: string) {
  return queryOptions({
    queryKey: workspaceKeys.forgeChecks(wsId),
    queryFn: () => api.listForgeChecks(),
  });
}

export function mcpServerListOptions(wsId: string) {
  return queryOptions({
    queryKey: workspaceKeys.mcpRegistry(wsId),
    queryFn: () => api.listMCPServers(),
  });
}

export function forgeReviewConfigOptions(wsId: string) {
  return queryOptions({
    queryKey: workspaceKeys.forgeReviewConfig(wsId),
    queryFn: () => api.getForgeReviewConfig(),
  });
}

export function forgeEntropyScanListOptions(wsId: string) {
  return queryOptions({
    queryKey: workspaceKeys.forgeEntropyScans(wsId),
    queryFn: () => api.listForgeEntropyScans(),
  });
}

export function forgeHealthOptions(wsId: string) {
  return queryOptions({
    queryKey: workspaceKeys.forgeHealth(wsId),
    queryFn: () => api.getForgeHealth(),
  });
}

export function forgeHealthTrendsOptions(wsId: string) {
  return queryOptions({
    queryKey: workspaceKeys.forgeHealthTrends(wsId),
    queryFn: () => api.getForgeHealthTrends(),
  });
}

/**
 * Builds a `Map<skillId, Agent[]>` from the cached agent list. The server
 * already returns each agent with its full skill list inline, so no extra
 * request is needed — "which agents use skill X" is pure client-side fold.
 *
 * Exposed as a plain helper rather than a `queryOptions` with `select` so
 * the Map's identity is stable across unrelated agent-cache rerenders —
 * callers wrap this in `useMemo(..., [agents])` and only re-fold when the
 * agent array identity actually changes. Previously this was `{ select }`,
 * which returned a new Map every subscription tick and triggered cascading
 * re-renders on every `agent:updated` WS event.
 */
export function selectSkillAssignments(
  agents: Agent[] | undefined,
): Map<string, Agent[]> {
  const map = new Map<string, Agent[]>();
  if (!agents) return map;
  for (const a of agents) {
    if (a.archived_at) continue;
    for (const s of a.skills ?? []) {
      const existing = map.get(s.id);
      if (existing) existing.push(a);
      else map.set(s.id, [a]);
    }
  }
  return map;
}

export function invitationListOptions(wsId: string) {
  return queryOptions({
    queryKey: workspaceKeys.invitations(wsId),
    queryFn: () => api.listWorkspaceInvitations(wsId),
  });
}

export function myInvitationListOptions() {
  return queryOptions({
    queryKey: workspaceKeys.myInvitations(),
    queryFn: () => api.listMyInvitations(),
  });
}

export function assigneeFrequencyOptions(wsId: string) {
  return queryOptions({
    queryKey: workspaceKeys.assigneeFrequency(wsId),
    queryFn: () => api.getAssigneeFrequency(),
  });
}
