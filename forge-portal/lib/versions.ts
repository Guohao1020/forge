import { api } from "./api";

// --- Types ---

export interface ProjectVersion {
  id: number;
  tenantId: number;
  projectId: number;
  version: string;
  status: "PLANNING" | "IN_PROGRESS" | "TESTING" | "RELEASED" | "CANCELLED";
  description: string;
  gitTag?: string;
  releasedAt?: string;
  createdBy: number;
  createdAt: string;
  updatedAt: string;
  taskCount: number;
  completedCount: number;
}

export interface VersionTaskBrief {
  id: number;
  title: string;
  status: string;
  conflictStatus: "NONE" | "DETECTED" | "WAITING" | "RESOLVED";
  blockedBy: number[];
  touchedFiles: string[];
  branchName?: string;
  prNumber?: number;
  createdAt: string;
  completedAt?: string;
}

export interface VersionDetailResponse {
  version: ProjectVersion;
  tasks: VersionTaskBrief[];
}

export interface VersionListResponse {
  versions: ProjectVersion[];
}

// --- Status display helpers ---

export const VERSION_STATUS_CONFIG: Record<
  ProjectVersion["status"],
  { label: string; color: string; bgColor: string }
> = {
  PLANNING: {
    label: "规划中",
    color: "text-blue-400",
    bgColor: "bg-blue-500/10 border-blue-500/20",
  },
  IN_PROGRESS: {
    label: "开发中",
    color: "text-purple-400",
    bgColor: "bg-purple-500/10 border-purple-500/20",
  },
  TESTING: {
    label: "待发布",
    color: "text-amber-400",
    bgColor: "bg-amber-500/10 border-amber-500/20",
  },
  RELEASED: {
    label: "已发布",
    color: "text-green-400",
    bgColor: "bg-green-500/10 border-green-500/20",
  },
  CANCELLED: {
    label: "已取消",
    color: "text-white/40",
    bgColor: "bg-white/5 border-white/10",
  },
};

export const CONFLICT_STATUS_CONFIG: Record<
  VersionTaskBrief["conflictStatus"],
  { label: string; color: string }
> = {
  NONE: { label: "", color: "" },
  DETECTED: { label: "文件冲突", color: "text-amber-400" },
  WAITING: { label: "等待中", color: "text-purple-400" },
  RESOLVED: { label: "已解决", color: "text-green-400" },
};

// --- API calls ---

export async function createVersion(
  projectId: number,
  version: string,
  description: string
): Promise<ProjectVersion> {
  return api.post(`/projects/${projectId}/versions`, { version, description });
}

export async function listVersions(
  projectId: number
): Promise<VersionListResponse> {
  return api.get(`/projects/${projectId}/versions`);
}

export async function getVersion(
  projectId: number,
  versionId: number
): Promise<VersionDetailResponse> {
  return api.get(`/projects/${projectId}/versions/${versionId}`);
}

export async function updateVersion(
  projectId: number,
  versionId: number,
  data: { description?: string; status?: string }
): Promise<void> {
  return api.put(`/projects/${projectId}/versions/${versionId}`, data);
}

export async function releaseVersion(
  projectId: number,
  versionId: number
): Promise<ProjectVersion> {
  return api.post(`/projects/${projectId}/versions/${versionId}/release`);
}
