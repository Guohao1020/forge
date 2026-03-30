export interface Result<T> {
  code: number
  message: string
  data: T
  timestamp: number
}

export interface LoginRequest {
  tenantId: number
  username: string
  password: string
}

export interface LoginResponse {
  accessToken: string
  refreshToken: string
  userId: number
  username: string
  roles: string[]
}

export interface TaskVO {
  id: number
  tenantId: number
  userId: number
  requirement: string
  taskType: string
  status: string
  repoId: string
  branchName: string | null
  mrId: number | null
  riskLevel: string | null
  reviewScore: number | null
  totalInputTokens: number
  totalOutputTokens: number
  gmtCreate: string
}

export interface TaskStepVO {
  id: number
  stepType: string
  stepOrder: number
  status: string
  inputTokens: number
  outputTokens: number
  retryCount: number
  outputSnapshot: string | null
  errorMessage: string | null
  gmtCreate: string
}

export interface CreateTaskRequest {
  tenantId: number
  userId: number
  requirement: string
  taskType: string
  repoId: string
}

export interface PipelineExecutionVO {
  id: number
  repoId: string
  branch: string
  projectType: string
  status: string
  compilePassed: boolean | null
  testPassed: boolean | null
  reviewPassed: boolean | null
  qualityGatePassed: boolean | null
  triggerType: string
  gmtCreate: string
}

export interface EnvironmentVO {
  id: number
  name: string
  envType: string
  namespace: string
  boundBranch: string | null
  status: string
  autoDestroyAt: string | null
  gmtCreate: string
}

export interface TokenUsageVO {
  taskId: number
  totalInputTokens: number
  totalOutputTokens: number
}
