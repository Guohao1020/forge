import { api } from "./api";

export interface Branch {
  name: string;
  sha: string;
  protected: boolean;
}

export interface PRSummary {
  number: number;
  title: string;
  state: string;
  html_url: string;
  head: string;
  base: string;
  created_at: string;
  user: string;
}

export interface PRFile {
  filename: string;
  status: string;
  additions: number;
  deletions: number;
  patch: string;
}

export async function getCodeTree(
  projectId: number | string,
  ref?: string
): Promise<{ files: string[]; ref: string }> {
  const params = ref ? `?ref=${encodeURIComponent(ref)}` : "";
  return api.get(`/projects/${projectId}/code/tree${params}`);
}

export async function getCodeFile(
  projectId: number | string,
  path: string,
  ref?: string
): Promise<{ path: string; content: string; ref: string }> {
  const params = new URLSearchParams({ path });
  if (ref) params.set("ref", ref);
  return api.get(`/projects/${projectId}/code/file?${params}`);
}

export async function listBranches(
  projectId: number | string
): Promise<Branch[]> {
  const res = await api.get<{ branches: Branch[] }>(
    `/projects/${projectId}/code/branches`
  );
  return res.branches || [];
}

export async function listPRs(
  projectId: number | string,
  state?: string
): Promise<PRSummary[]> {
  const params = state ? `?state=${state}` : "";
  const res = await api.get<{ prs: PRSummary[] }>(
    `/projects/${projectId}/code/prs${params}`
  );
  return res.prs || [];
}

export async function getPRDetail(
  projectId: number | string,
  prNumber: number
): Promise<PRFile[]> {
  const res = await api.get<{ files: PRFile[] }>(
    `/projects/${projectId}/code/prs/${prNumber}`
  );
  return res.files || [];
}
