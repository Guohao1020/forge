import { api } from "./api";

export interface DeployRecord {
  id: number;
  tenantId: number;
  projectId: number;
  environmentId: number;
  artifactId?: number;
  version: string;
  status: string; // PENDING / DEPLOYING / DEPLOYED / FAILED / ROLLED_BACK
  deployedBy: number;
  startedAt: string;
  completedAt?: string;
  k8sManifest?: string;
  errorMessage?: string;
  createdAt: string;
}

interface DeployRecordListResult {
  records: DeployRecord[];
}

export async function listDeployRecords(
  projectId: string,
  envId: number
): Promise<DeployRecord[]> {
  const result = await api.get<DeployRecordListResult>(
    `/projects/${projectId}/environments/${envId}/deploys`
  );
  return result.records || [];
}

export async function triggerDeploy(
  projectId: string,
  envId: number,
  version: string,
  artifactId?: number
): Promise<DeployRecord> {
  return api.post<DeployRecord>(
    `/projects/${projectId}/environments/${envId}/deploy`,
    { version, artifactId }
  );
}

export async function rollbackDeploy(
  projectId: string,
  envId: number
): Promise<DeployRecord> {
  return api.post<DeployRecord>(
    `/projects/${projectId}/environments/${envId}/rollback`
  );
}
