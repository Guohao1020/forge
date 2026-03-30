import axios from 'axios'
import type { Result } from './types'

const request = axios.create({
  baseURL: '',
  timeout: 30000
})

request.interceptors.request.use((config) => {
  const token = localStorage.getItem('accessToken')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

request.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('accessToken')
      localStorage.removeItem('refreshToken')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export async function get<T>(url: string, params?: Record<string, unknown>): Promise<T> {
  const res = await request.get<Result<T>>(url, { params })
  return res.data.data
}

export async function post<T>(url: string, data?: unknown): Promise<T> {
  const res = await request.post<Result<T>>(url, data)
  return res.data.data
}

export async function del<T>(url: string): Promise<T> {
  const res = await request.delete<Result<T>>(url)
  return res.data.data
}

export default request
