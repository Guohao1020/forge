import { api } from "./api";

export interface UserItem {
  id: number;
  username: string;
  displayName: string;
  roles: string[];
  status: string;
  lastLoginAt?: string;
}

export const ROLE_CONFIG: Record<string, { label: string; color: string }> = {
  PLATFORM_ADMIN: { label: "平台管理员", color: "text-red-400 bg-red-500/10 border-red-500/20" },
  ORG_ADMIN: { label: "组织管理员", color: "text-orange-400 bg-orange-500/10 border-orange-500/20" },
  PROJECT_ADMIN: { label: "项目管理员", color: "text-amber-400 bg-amber-500/10 border-amber-500/20" },
  DEVELOPER: { label: "开发者", color: "text-blue-400 bg-blue-500/10 border-blue-500/20" },
  VIEWER: { label: "查看者", color: "text-gray-400 bg-gray-500/10 border-gray-500/20" },
  TECH_LEAD: { label: "技术管理者", color: "text-purple-400 bg-purple-500/10 border-purple-500/20" },
  PM: { label: "产品经理", color: "text-green-400 bg-green-500/10 border-green-500/20" },
};

export async function listUsers(): Promise<{ users: UserItem[] }> {
  return api.get("/admin/users");
}

export async function createUser(data: {
  username: string;
  password: string;
  displayName?: string;
  role?: string;
}): Promise<{ id: number }> {
  return api.post("/admin/users", data);
}

export async function updateUserRole(userId: number, role: string): Promise<void> {
  return api.put(`/admin/users/${userId}/role`, { role });
}
