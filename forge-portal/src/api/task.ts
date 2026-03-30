import { get, post } from './request'
import type { TaskVO, TaskStepVO, CreateTaskRequest, TokenUsageVO } from './types'

export function createTask(data: CreateTaskRequest): Promise<TaskVO> {
  return post<TaskVO>('/api/tasks', data)
}

export function listTasks(tenantId: number, userId: number): Promise<TaskVO[]> {
  return get<TaskVO[]>('/api/tasks', { tenantId, userId })
}

export function getTask(taskId: number): Promise<TaskVO> {
  return get<TaskVO>(`/api/tasks/${taskId}`)
}

export function getTaskSteps(taskId: number): Promise<TaskStepVO[]> {
  return get<TaskStepVO[]>(`/api/tasks/${taskId}/steps`)
}

export function cancelTask(taskId: number): Promise<void> {
  return post<void>(`/api/tasks/${taskId}/cancel`)
}

export function getTokenUsage(taskId: number): Promise<TokenUsageVO> {
  return get<TokenUsageVO>(`/api/token-usage/${taskId}`)
}

export async function getKillSwitchLevel(): Promise<string> {
  const result = await get<{ level: string }>('/api/killswitch')
  return result.level
}

export function activateKillSwitch(level: string): Promise<void> {
  return post<void>(`/api/killswitch/activate?level=${level}`)
}

export function deactivateKillSwitch(): Promise<void> {
  return post<void>('/api/killswitch/deactivate')
}
