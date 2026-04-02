import { api } from "./api";

export interface ProfileEntry {
  id: number;
  projectId: number;
  profileKey: string;
  profileValue: Record<string, unknown>;
  version: number;
  scannedAt: string;
}

interface ProfileListResult {
  profiles: ProfileEntry[];
}

export async function listProfiles(projectId: number | string): Promise<ProfileEntry[]> {
  const result = await api.get<ProfileListResult>(
    `/projects/${projectId}/profiles`
  );
  return result.profiles || [];
}

export async function triggerScan(
  projectId: number | string,
  keys?: string[]
): Promise<void> {
  await api.post(`/projects/${projectId}/profiles/scan`, keys ? { keys } : undefined);
}
