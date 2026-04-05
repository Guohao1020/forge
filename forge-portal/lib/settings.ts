import { api } from "./api";

export interface Setting {
  key: string;
  value: string;
  category: string;
  updatedAt?: string;
}

export async function listSettings(): Promise<Setting[]> {
  const res = await api.get<{ settings: Setting[] }>("/settings");
  return res.settings;
}

export async function getSetting(key: string): Promise<string> {
  const res = await api.get<{ key: string; value: string }>(`/settings/${key}`);
  return res.value;
}

export async function setSetting(key: string, value: string, category?: string): Promise<void> {
  await api.put(`/settings/${key}`, { value, category });
}

export async function bulkSetSettings(settings: Record<string, string>): Promise<void> {
  await api.put("/settings", settings);
}
