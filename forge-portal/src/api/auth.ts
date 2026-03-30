import { post } from './request'
import type { LoginRequest, LoginResponse } from './types'

export function login(data: LoginRequest): Promise<LoginResponse> {
  return post<LoginResponse>('/api/auth/login', data)
}

export function logout(): Promise<void> {
  return post<void>('/api/auth/logout')
}
