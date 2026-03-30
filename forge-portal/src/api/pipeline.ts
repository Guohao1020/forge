import { get, post, del } from './request'
import type { PipelineExecutionVO, EnvironmentVO } from './types'

export function triggerPipeline(tenantId: number, repoId: string, branch: string): Promise<void> {
  return post<void>('/api/pipelines/trigger', { tenantId, repoId, branch })
}

export function getPipelineExecution(id: number): Promise<PipelineExecutionVO> {
  return get<PipelineExecutionVO>(`/api/pipelines/${id}`)
}

export function listEnvironments(tenantId: number): Promise<EnvironmentVO[]> {
  return get<EnvironmentVO[]>('/api/environments', { tenantId })
}

export function createTemporaryEnvironment(tenantId: number, repoId: string, branch: string, taskId?: number): Promise<EnvironmentVO> {
  return post<EnvironmentVO>('/api/environments/temporary', { tenantId, repoId, branch, taskId })
}

export function destroyEnvironment(id: number): Promise<void> {
  return del<void>(`/api/environments/${id}`)
}
