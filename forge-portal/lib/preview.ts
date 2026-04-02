import { api } from "./api";

export interface PreviewEnvironment {
  id: number;
  tenantId: number;
  projectId: number;
  taskId?: number;
  branchName?: string;
  prNumber?: number;
  previewUrl?: string;
  status: string;
  namespace?: string;
  expiresAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface PreviewListResponse {
  previews: PreviewEnvironment[];
}

export async function listPreviews(projectId: string | number): Promise<PreviewEnvironment[]> {
  const res = await api.get<PreviewListResponse>(`/projects/${projectId}/previews`);
  return res.previews || [];
}

export async function getTaskPreview(
  projectId: string | number,
  taskId: string | number
): Promise<PreviewEnvironment | null> {
  try {
    return await api.get<PreviewEnvironment>(`/projects/${projectId}/tasks/${taskId}/preview`);
  } catch {
    return null;
  }
}

export async function createPreview(
  projectId: string | number,
  taskId: string | number,
  branchName?: string,
  prNumber?: number
): Promise<PreviewEnvironment> {
  return api.post<PreviewEnvironment>(`/projects/${projectId}/tasks/${taskId}/preview`, {
    branchName,
    prNumber,
  });
}

export async function destroyPreview(
  projectId: string | number,
  previewId: number
): Promise<void> {
  await api.delete(`/projects/${projectId}/previews/${previewId}`);
}
