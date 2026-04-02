import { api } from "./api";

export interface Artifact {
  id: number;
  tenantId: number;
  projectId: number;
  taskId?: number;
  name: string;
  version: string;
  artifactType: string; // DOCKER_IMAGE / JAR / BINARY / ARCHIVE
  registryUrl?: string;
  sizeBytes?: number;
  checksum?: string;
  metadata: Record<string, unknown>;
  status: string; // BUILDING / READY / FAILED
  createdAt: string;
}

interface ArtifactListResult {
  artifacts: Artifact[];
}

export async function listArtifacts(projectId: string): Promise<Artifact[]> {
  const result = await api.get<ArtifactListResult>(
    `/projects/${projectId}/artifacts`
  );
  return result.artifacts || [];
}

export async function getArtifact(
  projectId: string,
  artifactId: number
): Promise<Artifact> {
  return api.get<Artifact>(
    `/projects/${projectId}/artifacts/${artifactId}`
  );
}
