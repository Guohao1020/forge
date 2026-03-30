# M6 — Web 工作台（轻量版）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建 forge-portal Vue 3 前端工作台，实现登录、需求输入、任务看板、代码浏览、AI Diff 预览、任务详情查看、紧急停止的完整 Web 交互闭环。

**Architecture:** forge-portal 是一个 Vue 3 SPA，使用 Ant Design Vue 作为 UI 组件库，Pinia 做状态管理，Vue Router 做路由，Axios 做 HTTP 请求。通过 APISIX 网关代理到后端三个服务（forge-identity:8082、forge-engine:8081、forge-pipeline:8083）。前端按页面组织，每页一个 `.vue` 文件 + 对应的 API 模块 + Pinia store。

**Tech Stack:** Vue 3.5, TypeScript, Ant Design Vue 4.x, Pinia 3.x, Vue Router 4.x, Axios, Vite 8.x

**M6 轻量版简化说明：**
- 不做 SSE 流式输出（用轮询替代）
- 不做 WebSocket 实时更新（用 5s 轮询替代）
- 代码浏览器仅展示文件列表和内容（不做完整的 Monaco Editor）
- AI Diff 预览用简单的 side-by-side 文本对比（不做语法高亮 diff 库）
- 不做多租户切换 UI（固定 tenantId=1）
- 不做国际化
- 不做响应式移动端适配

---

## 文件结构总览

```
forge-portal/src/
├── main.ts                          ← 修改：注册路由、Pinia、AntDesign
├── App.vue                          ← 修改：添加 RouterView + Layout
├── style.css                        ← 修改：全局样式重置
├── api/
│   ├── request.ts                   ← 新建：Axios 实例 + 拦截器
│   ├── auth.ts                      ← 新建：登录/登出 API
│   ├── task.ts                      ← 新建：任务 CRUD API
│   ├── pipeline.ts                  ← 新建：流水线/部署/环境 API
│   └── types.ts                     ← 新建：后端接口类型定义
├── stores/
│   ├── user.ts                      ← 新建：用户/Token 状态
│   └── task.ts                      ← 新建：任务列表状态
├── router/
│   └── index.ts                     ← 新建：路由配置 + 守卫
├── views/
│   ├── LoginView.vue                ← 新建：登录页
│   ├── DashboardView.vue            ← 新建：任务看板页
│   ├── TaskCreateView.vue           ← 新建：需求输入页
│   ├── TaskDetailView.vue           ← 新建：任务详情页
│   └── EnvironmentView.vue          ← 新建：环境管理页
├── components/
│   ├── AppLayout.vue                ← 新建：全局 Layout
│   ├── TaskCard.vue                 ← 新建：任务卡片
│   ├── StepTimeline.vue             ← 新建：步骤时间线
│   ├── CodeBrowser.vue              ← 新建：代码浏览器
│   ├── DiffViewer.vue               ← 新建：Diff 查看器
│   └── KillSwitch.vue               ← 新建：紧急停止按钮
└── vite-env.d.ts                    ← 已有
```

---

### Task 1: 项目基础设施（路由 + Axios + 全局样式 + Layout）

**前置修改（forge-engine 后端）：**

M6 的代码浏览器需要从 TaskStepVO 获取 `outputSnapshot` 字段，但 M4 实现时未暴露该字段。在开始前端工作之前，需要先修改 forge-engine 后端：

1. 修改 `forge-engine/src/main/java/com/shulex/forge/engine/entrance/vo/TaskStepVO.java`，添加 `private String outputSnapshot;` 字段
2. 修改 `forge-engine/src/main/java/com/shulex/forge/engine/entrance/controller/TaskController.java` 中构造 TaskStepVO 的代码，将 `step.getOutputSnapshot()` 映射到 VO
3. 安装 `@ant-design/icons-vue` 依赖：`cd forge-portal && npm install @ant-design/icons-vue`

**Files:**
- Modify: `forge-engine/src/main/java/com/shulex/forge/engine/entrance/vo/TaskStepVO.java`
- Modify: `forge-engine/src/main/java/com/shulex/forge/engine/entrance/controller/TaskController.java`
- Modify: `forge-portal/package.json`（npm install）
- Modify: `forge-portal/src/main.ts`
- Modify: `forge-portal/src/App.vue`
- Modify: `forge-portal/src/style.css`
- Modify: `forge-portal/vite.config.ts`
- Create: `forge-portal/src/api/request.ts`
- Create: `forge-portal/src/api/types.ts`
- Create: `forge-portal/src/router/index.ts`
- Create: `forge-portal/src/stores/user.ts`
- Create: `forge-portal/src/components/AppLayout.vue`
- Create: `forge-portal/src/views/LoginView.vue`（空占位）
- Create: `forge-portal/src/views/DashboardView.vue`（空占位）

- [ ] **Step 1: 配置 vite.config.ts — 添加代理和路径别名**

```typescript
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath } from 'node:url'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:9080',
        changeOrigin: true
      }
    }
  }
})
```

- [ ] **Step 2: 创建 api/types.ts — 后端接口通用类型**

```typescript
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
```

- [ ] **Step 3: 创建 api/request.ts — Axios 实例 + Token 拦截器**

```typescript
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
```

- [ ] **Step 4: 创建 stores/user.ts — 用户状态管理**

```typescript
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useUserStore = defineStore('user', () => {
  const accessToken = ref(localStorage.getItem('accessToken') || '')
  const username = ref(localStorage.getItem('username') || '')
  const userId = ref(Number(localStorage.getItem('userId')) || 0)

  const isLoggedIn = computed(() => !!accessToken.value)

  function setLoginInfo(token: string, name: string, id: number) {
    accessToken.value = token
    username.value = name
    userId.value = id
    localStorage.setItem('accessToken', token)
    localStorage.setItem('username', name)
    localStorage.setItem('userId', String(id))
  }

  function logout() {
    accessToken.value = ''
    username.value = ''
    userId.value = 0
    localStorage.removeItem('accessToken')
    localStorage.removeItem('refreshToken')
    localStorage.removeItem('username')
    localStorage.removeItem('userId')
  }

  return { accessToken, username, userId, isLoggedIn, setLoginInfo, logout }
})
```

- [ ] **Step 5: 创建 router/index.ts — 路由配置 + 登录守卫**

```typescript
import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/views/LoginView.vue'),
      meta: { requiresAuth: false }
    },
    {
      path: '/',
      redirect: '/dashboard'
    },
    {
      path: '/dashboard',
      name: 'Dashboard',
      component: () => import('@/views/DashboardView.vue')
    },
    {
      path: '/tasks/create',
      name: 'TaskCreate',
      component: () => import('@/views/TaskCreateView.vue')
    },
    {
      path: '/tasks/:id',
      name: 'TaskDetail',
      component: () => import('@/views/TaskDetailView.vue')
    },
    {
      path: '/environments',
      name: 'Environments',
      component: () => import('@/views/EnvironmentView.vue')
    }
  ]
})

router.beforeEach((to) => {
  const token = localStorage.getItem('accessToken')
  if (to.meta.requiresAuth !== false && !token) {
    return { name: 'Login' }
  }
})

export default router
```

- [ ] **Step 6: 创建 AppLayout.vue — 侧边栏导航 Layout**

```vue
<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import {
  DashboardOutlined,
  PlusOutlined,
  CloudServerOutlined,
  LogoutOutlined
} from '@ant-design/icons-vue'

const router = useRouter()
const userStore = useUserStore()

function handleLogout() {
  userStore.logout()
  router.push('/login')
}
</script>

<template>
  <a-layout style="min-height: 100vh">
    <a-layout-sider :width="200" theme="dark">
      <div style="height: 48px; line-height: 48px; text-align: center; color: #fff; font-size: 18px; font-weight: bold;">
        Forge
      </div>
      <a-menu theme="dark" mode="inline" @click="({ key }: { key: string }) => router.push(key)">
        <a-menu-item key="/dashboard">
          <DashboardOutlined />
          <span>任务看板</span>
        </a-menu-item>
        <a-menu-item key="/tasks/create">
          <PlusOutlined />
          <span>创建任务</span>
        </a-menu-item>
        <a-menu-item key="/environments">
          <CloudServerOutlined />
          <span>环境管理</span>
        </a-menu-item>
      </a-menu>
    </a-layout-sider>
    <a-layout>
      <a-layout-header style="background: #fff; padding: 0 24px; display: flex; justify-content: flex-end; align-items: center;">
        <span style="margin-right: 16px;">{{ userStore.username }}</span>
        <a-button type="text" @click="handleLogout">
          <LogoutOutlined /> 登出
        </a-button>
      </a-layout-header>
      <a-layout-content style="margin: 24px; padding: 24px; background: #fff; min-height: 280px;">
        <router-view />
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>
```

- [ ] **Step 7: 创建占位页面 LoginView.vue + DashboardView.vue**

```vue
<!-- LoginView.vue -->
<template>
  <div>登录页 - 待实现</div>
</template>
```

```vue
<!-- DashboardView.vue -->
<template>
  <div>看板页 - 待实现</div>
</template>
```

- [ ] **Step 8: 创建占位页面 TaskCreateView.vue + TaskDetailView.vue + EnvironmentView.vue**

```vue
<!-- TaskCreateView.vue -->
<template>
  <div>创建任务 - 待实现</div>
</template>
```

```vue
<!-- TaskDetailView.vue -->
<template>
  <div>任务详情 - 待实现</div>
</template>
```

```vue
<!-- EnvironmentView.vue -->
<template>
  <div>环境管理 - 待实现</div>
</template>
```

- [ ] **Step 9: 修改 main.ts — 注册路由、Pinia、AntDesign**

```typescript
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import Antd from 'ant-design-vue'
import 'ant-design-vue/dist/reset.css'
import App from './App.vue'
import router from './router'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(Antd)
app.mount('#app')
```

- [ ] **Step 10: 修改 App.vue — 条件 Layout**

```vue
<script setup lang="ts">
import { useRoute } from 'vue-router'
import AppLayout from '@/components/AppLayout.vue'
import { computed } from 'vue'

const route = useRoute()
const showLayout = computed(() => route.name !== 'Login')
</script>

<template>
  <AppLayout v-if="showLayout" />
  <router-view v-else />
</template>
```

- [ ] **Step 11: 修改 style.css — 最小样式重置**

```css
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}
```

- [ ] **Step 12: 删除不再需要的文件**

删除 `src/components/HelloWorld.vue`（已被替代）。

- [ ] **Step 13: 编译验证**

Run: `cd forge-portal && npm run build 2>&1 | tail -20`
Expected: 编译通过（可能有 unused 变量 warning，不影响）

注意：如果 TypeScript 报 `@` 别名找不到，需要在 `tsconfig.app.json` 中添加 paths 配置：
```json
"paths": {
  "@/*": ["./src/*"]
}
```

- [ ] **Step 14: Commit**

```bash
git add forge-portal/src/ forge-portal/vite.config.ts
git commit -m "feat(m6): add project infrastructure with routing, axios, layout, and stores"
```

---

### Task 2: 登录页 + Auth API

**Files:**
- Create: `forge-portal/src/api/auth.ts`
- Modify: `forge-portal/src/views/LoginView.vue`

- [ ] **Step 1: 创建 api/auth.ts**

```typescript
import { post } from './request'
import type { LoginRequest, LoginResponse } from './types'

export function login(data: LoginRequest): Promise<LoginResponse> {
  return post<LoginResponse>('/api/auth/login', data)
}

export function logout(): Promise<void> {
  return post<void>('/api/auth/logout')
}
```

- [ ] **Step 2: 实现 LoginView.vue**

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { useUserStore } from '@/stores/user'
import { login } from '@/api/auth'

const router = useRouter()
const userStore = useUserStore()
const loading = ref(false)
const form = ref({
  username: '',
  password: ''
})

async function handleLogin() {
  if (!form.value.username || !form.value.password) {
    message.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    const res = await login({
      tenantId: 1,
      username: form.value.username,
      password: form.value.password
    })
    userStore.setLoginInfo(res.accessToken, res.username, res.userId)
    localStorage.setItem('refreshToken', res.refreshToken)
    message.success('登录成功')
    router.push('/dashboard')
  } catch {
    message.error('登录失败，请检查用户名和密码')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div style="display: flex; justify-content: center; align-items: center; min-height: 100vh; background: #f0f2f5;">
    <a-card title="Forge 工作台" style="width: 400px;">
      <a-form layout="vertical">
        <a-form-item label="用户名">
          <a-input v-model:value="form.username" placeholder="请输入用户名" @pressEnter="handleLogin" />
        </a-form-item>
        <a-form-item label="密码">
          <a-input-password v-model:value="form.password" placeholder="请输入密码" @pressEnter="handleLogin" />
        </a-form-item>
        <a-form-item>
          <a-button type="primary" block :loading="loading" @click="handleLogin">
            登 录
          </a-button>
        </a-form-item>
      </a-form>
    </a-card>
  </div>
</template>
```

- [ ] **Step 3: 编译验证**

Run: `cd forge-portal && npm run build 2>&1 | tail -10`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
git add forge-portal/src/
git commit -m "feat(m6): add login page and auth API"
```

---

### Task 3: 任务看板页（Dashboard）+ Task API + Store

**Files:**
- Create: `forge-portal/src/api/task.ts`
- Create: `forge-portal/src/stores/task.ts`
- Create: `forge-portal/src/components/TaskCard.vue`
- Modify: `forge-portal/src/views/DashboardView.vue`

- [ ] **Step 1: 创建 api/task.ts**

```typescript
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
```

- [ ] **Step 2: 创建 stores/task.ts**

```typescript
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { listTasks } from '@/api/task'
import type { TaskVO } from '@/api/types'

export const useTaskStore = defineStore('task', () => {
  const tasks = ref<TaskVO[]>([])
  const loading = ref(false)

  async function fetchTasks() {
    loading.value = true
    try {
      const userId = Number(localStorage.getItem('userId')) || 0
      tasks.value = await listTasks(1, userId)
    } finally {
      loading.value = false
    }
  }

  return { tasks, loading, fetchTasks }
})
```

- [ ] **Step 3: 创建 components/TaskCard.vue**

```vue
<script setup lang="ts">
import type { TaskVO } from '@/api/types'

defineProps<{ task: TaskVO }>()
defineEmits<{ click: [id: number] }>()

function statusColor(status: string): string {
  const map: Record<string, string> = {
    PENDING: 'default',
    RISK_ASSESSING: 'processing',
    DISPATCHING: 'processing',
    EXECUTING: 'processing',
    REVIEWING: 'processing',
    COMMITTING: 'processing',
    COMPLETED: 'success',
    FAILED: 'error',
    CANCELLED: 'warning'
  }
  return map[status] || 'default'
}

function riskColor(level: string | null): string {
  if (!level) return 'default'
  const map: Record<string, string> = { LOW: 'green', MEDIUM: 'orange', HIGH: 'red' }
  return map[level] || 'default'
}
</script>

<template>
  <a-card hoverable size="small" style="margin-bottom: 12px;" @click="$emit('click', task.id)">
    <template #title>
      <span style="font-size: 14px;">#{{ task.id }}</span>
      <a-tag :color="statusColor(task.status)" style="margin-left: 8px;">{{ task.status }}</a-tag>
      <a-tag v-if="task.riskLevel" :color="riskColor(task.riskLevel)" style="margin-left: 4px;">
        {{ task.riskLevel }}
      </a-tag>
    </template>
    <p style="margin: 0; color: #666; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
      {{ task.requirement }}
    </p>
    <div style="margin-top: 8px; font-size: 12px; color: #999;">
      <span>{{ task.taskType }}</span>
      <span style="margin-left: 12px;">Token: {{ task.totalInputTokens + task.totalOutputTokens }}</span>
      <span style="margin-left: 12px;">{{ task.gmtCreate }}</span>
    </div>
  </a-card>
</template>
```

- [ ] **Step 4: 实现 DashboardView.vue — 任务看板**

```vue
<script setup lang="ts">
import { onMounted, onUnmounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useTaskStore } from '@/stores/task'
import TaskCard from '@/components/TaskCard.vue'
import KillSwitch from '@/components/KillSwitch.vue'

const router = useRouter()
const taskStore = useTaskStore()

const activeTasks = computed(() =>
  taskStore.tasks.filter(t => !['COMPLETED', 'FAILED', 'CANCELLED'].includes(t.status))
)
const completedTasks = computed(() =>
  taskStore.tasks.filter(t => ['COMPLETED', 'FAILED', 'CANCELLED'].includes(t.status))
)

let timer: ReturnType<typeof setInterval>

onMounted(() => {
  taskStore.fetchTasks()
  timer = setInterval(() => taskStore.fetchTasks(), 5000)
})

onUnmounted(() => clearInterval(timer))

function goToDetail(id: number) {
  router.push(`/tasks/${id}`)
}
</script>

<template>
  <div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px;">
      <h2 style="margin: 0;">任务看板</h2>
      <div style="display: flex; gap: 12px; align-items: center;">
        <KillSwitch />
        <a-button type="primary" @click="router.push('/tasks/create')">创建任务</a-button>
      </div>
    </div>

    <a-spin :spinning="taskStore.loading">
      <a-row :gutter="24">
        <a-col :span="12">
          <h3>进行中 ({{ activeTasks.length }})</h3>
          <TaskCard v-for="t in activeTasks" :key="t.id" :task="t" @click="goToDetail" />
          <a-empty v-if="activeTasks.length === 0" description="暂无进行中的任务" />
        </a-col>
        <a-col :span="12">
          <h3>已完成 ({{ completedTasks.length }})</h3>
          <TaskCard v-for="t in completedTasks" :key="t.id" :task="t" @click="goToDetail" />
          <a-empty v-if="completedTasks.length === 0" description="暂无已完成的任务" />
        </a-col>
      </a-row>
    </a-spin>
  </div>
</template>
```

- [ ] **Step 5: 创建 components/KillSwitch.vue — 紧急停止按钮**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { getKillSwitchLevel, activateKillSwitch, deactivateKillSwitch } from '@/api/task'

const level = ref('NONE')
const loading = ref(false)

async function fetchLevel() {
  try {
    level.value = await getKillSwitchLevel()
  } catch {
    // ignore
  }
}

function handleToggle() {
  if (level.value === 'NONE') {
    Modal.confirm({
      title: '确认激活紧急停止？',
      content: '激活后所有正在执行的 AI 任务将被暂停。',
      okText: '确认激活',
      okType: 'danger',
      async onOk() {
        loading.value = true
        try {
          await activateKillSwitch('GLOBAL')
          level.value = 'GLOBAL'
          message.success('紧急停止已激活')
        } catch {
          message.error('操作失败')
        } finally {
          loading.value = false
        }
      }
    })
  } else {
    Modal.confirm({
      title: '确认解除紧急停止？',
      content: '解除后 AI 任务将恢复执行。',
      okText: '确认解除',
      async onOk() {
        loading.value = true
        try {
          await deactivateKillSwitch()
          level.value = 'NONE'
          message.success('紧急停止已解除')
        } catch {
          message.error('操作失败')
        } finally {
          loading.value = false
        }
      }
    })
  }
}

onMounted(fetchLevel)
</script>

<template>
  <a-button
    :type="level === 'NONE' ? 'default' : 'primary'"
    :danger="level !== 'NONE'"
    :loading="loading"
    @click="handleToggle"
  >
    {{ level === 'NONE' ? '紧急停止' : '解除停止 (' + level + ')' }}
  </a-button>
</template>
```

- [ ] **Step 6: 编译验证**

Run: `cd forge-portal && npm run build 2>&1 | tail -10`
Expected: 编译通过

- [ ] **Step 7: Commit**

```bash
git add forge-portal/src/
git commit -m "feat(m6): add dashboard with task kanban, kill switch, and task API"
```

---

### Task 4: 需求输入页（TaskCreate）

**Files:**
- Modify: `forge-portal/src/views/TaskCreateView.vue`

- [ ] **Step 1: 实现 TaskCreateView.vue**

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { createTask } from '@/api/task'

const router = useRouter()
const loading = ref(false)
const form = ref({
  requirement: '',
  taskType: 'CREATE',
  repoId: ''
})

const taskTypes = [
  { label: '创建项目', value: 'CREATE' },
  { label: '迭代功能', value: 'ITERATE' },
  { label: '修复缺陷', value: 'FIX' }
]

async function handleSubmit() {
  if (!form.value.requirement.trim()) {
    message.warning('请输入需求描述')
    return
  }
  if (!form.value.repoId.trim()) {
    message.warning('请输入仓库 ID')
    return
  }
  loading.value = true
  try {
    const userId = Number(localStorage.getItem('userId')) || 1
    const task = await createTask({
      tenantId: 1,
      userId,
      requirement: form.value.requirement,
      taskType: form.value.taskType,
      repoId: form.value.repoId
    })
    message.success('任务已创建')
    router.push(`/tasks/${task.id}`)
  } catch {
    message.error('创建任务失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div style="max-width: 800px; margin: 0 auto;">
    <h2>创建 AI 任务</h2>
    <a-form layout="vertical" style="margin-top: 24px;">
      <a-form-item label="任务类型">
        <a-radio-group v-model:value="form.taskType" :options="taskTypes" option-type="button" />
      </a-form-item>
      <a-form-item label="仓库 ID">
        <a-input v-model:value="form.repoId" placeholder="Codeup 仓库 ID" />
      </a-form-item>
      <a-form-item label="需求描述">
        <a-textarea
          v-model:value="form.requirement"
          placeholder="请用自然语言描述你的需求，例如：创建一个用户管理服务，包含用户注册、登录、信息修改功能"
          :rows="8"
          show-count
          :maxlength="5000"
        />
      </a-form-item>
      <a-form-item>
        <a-space>
          <a-button type="primary" :loading="loading" @click="handleSubmit">提交任务</a-button>
          <a-button @click="router.push('/dashboard')">取消</a-button>
        </a-space>
      </a-form-item>
    </a-form>
  </div>
</template>
```

- [ ] **Step 2: 编译验证**

Run: `cd forge-portal && npm run build 2>&1 | tail -10`
Expected: 编译通过

- [ ] **Step 3: Commit**

```bash
git add forge-portal/src/
git commit -m "feat(m6): add task creation page with requirement input"
```

---

### Task 5: 任务详情页 + 步骤时间线 + Token 消耗

**Files:**
- Create: `forge-portal/src/components/StepTimeline.vue`
- Modify: `forge-portal/src/views/TaskDetailView.vue`

- [ ] **Step 1: 创建 StepTimeline.vue**

```vue
<script setup lang="ts">
import type { TaskStepVO } from '@/api/types'
import { CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined, LoadingOutlined } from '@ant-design/icons-vue'

defineProps<{ steps: TaskStepVO[] }>()

function stepIcon(status: string) {
  switch (status) {
    case 'COMPLETED': return CheckCircleOutlined
    case 'EXECUTING': return LoadingOutlined
    case 'FAILED': return CloseCircleOutlined
    default: return ClockCircleOutlined
  }
}

function stepColor(status: string): string {
  switch (status) {
    case 'COMPLETED': return 'green'
    case 'EXECUTING': return 'blue'
    case 'FAILED': return 'red'
    default: return 'gray'
  }
}
</script>

<template>
  <a-timeline>
    <a-timeline-item v-for="step in steps" :key="step.id" :color="stepColor(step.status)">
      <template #dot>
        <component :is="stepIcon(step.status)" />
      </template>
      <div>
        <strong>{{ step.stepType }}</strong>
        <a-tag :color="stepColor(step.status)" style="margin-left: 8px;">{{ step.status }}</a-tag>
      </div>
      <div style="font-size: 12px; color: #999; margin-top: 4px;">
        Token: {{ step.inputTokens + step.outputTokens }}
        <span style="margin-left: 8px;">{{ step.gmtCreate }}</span>
      </div>
      <div v-if="step.errorMessage" style="color: red; font-size: 12px; margin-top: 4px;">
        {{ step.errorMessage }}
      </div>
    </a-timeline-item>
  </a-timeline>
</template>
```

- [ ] **Step 2: 实现 TaskDetailView.vue**

```vue
<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { getTask, getTaskSteps, cancelTask, getTokenUsage } from '@/api/task'
import type { TaskVO, TaskStepVO, TokenUsageVO } from '@/api/types'
import StepTimeline from '@/components/StepTimeline.vue'

const route = useRoute()
const router = useRouter()
const taskId = Number(route.params.id)

const task = ref<TaskVO | null>(null)
const steps = ref<TaskStepVO[]>([])
const tokenUsage = ref<TokenUsageVO | null>(null)
const loading = ref(true)
const cancelling = ref(false)

async function fetchData() {
  try {
    const [t, s, u] = await Promise.all([
      getTask(taskId),
      getTaskSteps(taskId),
      getTokenUsage(taskId)
    ])
    task.value = t
    steps.value = s
    tokenUsage.value = u
  } catch {
    message.error('加载任务详情失败')
  } finally {
    loading.value = false
  }
}

async function handleCancel() {
  cancelling.value = true
  try {
    await cancelTask(taskId)
    message.success('任务已取消')
    await fetchData()
  } catch {
    message.error('取消失败')
  } finally {
    cancelling.value = false
  }
}

const isRunning = (status: string) =>
  !['COMPLETED', 'FAILED', 'CANCELLED'].includes(status)

let timer: ReturnType<typeof setInterval>

onMounted(() => {
  fetchData()
  timer = setInterval(fetchData, 5000)
})

onUnmounted(() => clearInterval(timer))
</script>

<template>
  <a-spin :spinning="loading">
    <div v-if="task">
      <div style="display: flex; justify-content: space-between; align-items: center;">
        <h2>任务 #{{ task.id }}</h2>
        <a-space>
          <a-button v-if="isRunning(task.status)" danger :loading="cancelling" @click="handleCancel">
            取消任务
          </a-button>
          <a-button @click="router.push('/dashboard')">返回看板</a-button>
        </a-space>
      </div>

      <a-descriptions bordered :column="2" style="margin-top: 16px;">
        <a-descriptions-item label="状态">
          <a-tag>{{ task.status }}</a-tag>
        </a-descriptions-item>
        <a-descriptions-item label="类型">{{ task.taskType }}</a-descriptions-item>
        <a-descriptions-item label="仓库">{{ task.repoId }}</a-descriptions-item>
        <a-descriptions-item label="分支">{{ task.branchName || '-' }}</a-descriptions-item>
        <a-descriptions-item label="风险等级">
          <a-tag v-if="task.riskLevel" :color="task.riskLevel === 'HIGH' ? 'red' : task.riskLevel === 'MEDIUM' ? 'orange' : 'green'">
            {{ task.riskLevel }}
          </a-tag>
          <span v-else>-</span>
        </a-descriptions-item>
        <a-descriptions-item label="Review 评分">
          <span v-if="task.reviewScore !== null">{{ task.reviewScore }}</span>
          <span v-else>-</span>
        </a-descriptions-item>
        <a-descriptions-item label="MR ID">{{ task.mrId || '-' }}</a-descriptions-item>
        <a-descriptions-item label="创建时间">{{ task.gmtCreate }}</a-descriptions-item>
      </a-descriptions>

      <a-card title="Token 消耗" style="margin-top: 16px;" v-if="tokenUsage">
        <a-row :gutter="16">
          <a-col :span="8">
            <a-statistic title="输入 Token" :value="tokenUsage.totalInputTokens" />
          </a-col>
          <a-col :span="8">
            <a-statistic title="输出 Token" :value="tokenUsage.totalOutputTokens" />
          </a-col>
          <a-col :span="8">
            <a-statistic title="总计" :value="tokenUsage.totalInputTokens + tokenUsage.totalOutputTokens" />
          </a-col>
        </a-row>
      </a-card>

      <a-card title="需求描述" style="margin-top: 16px;">
        <pre style="white-space: pre-wrap; word-break: break-word; margin: 0;">{{ task.requirement }}</pre>
      </a-card>

      <a-card title="执行步骤" style="margin-top: 16px;">
        <StepTimeline :steps="steps" />
        <a-empty v-if="steps.length === 0" description="暂无步骤" />
      </a-card>
    </div>
  </a-spin>
</template>
```

- [ ] **Step 3: 编译验证**

Run: `cd forge-portal && npm run build 2>&1 | tail -10`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
git add forge-portal/src/
git commit -m "feat(m6): add task detail page with step timeline and token usage"
```

---

### Task 6: 环境管理页 + Pipeline API

**Files:**
- Create: `forge-portal/src/api/pipeline.ts`
- Modify: `forge-portal/src/views/EnvironmentView.vue`

- [ ] **Step 1: 创建 api/pipeline.ts**

```typescript
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
```

- [ ] **Step 2: 实现 EnvironmentView.vue**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { listEnvironments, destroyEnvironment } from '@/api/pipeline'
import type { EnvironmentVO } from '@/api/types'

const environments = ref<EnvironmentVO[]>([])
const loading = ref(true)

async function fetchEnvironments() {
  loading.value = true
  try {
    environments.value = await listEnvironments(1)
  } finally {
    loading.value = false
  }
}

function handleDestroy(env: EnvironmentVO) {
  Modal.confirm({
    title: `确认销毁环境 "${env.name}"？`,
    content: `Namespace: ${env.namespace}`,
    okText: '确认销毁',
    okType: 'danger',
    async onOk() {
      try {
        await destroyEnvironment(env.id)
        message.success('环境已销毁')
        await fetchEnvironments()
      } catch {
        message.error('销毁失败')
      }
    }
  })
}

function envTypeTag(type: string) {
  return type === 'FIXED' ? 'blue' : 'orange'
}

function statusTag(status: string) {
  const map: Record<string, string> = { ACTIVE: 'green', DESTROYING: 'orange', DESTROYED: 'default' }
  return map[status] || 'default'
}

const columns = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '类型', dataIndex: 'envType', key: 'envType' },
  { title: 'Namespace', dataIndex: 'namespace', key: 'namespace' },
  { title: '绑定分支', dataIndex: 'boundBranch', key: 'boundBranch' },
  { title: '状态', dataIndex: 'status', key: 'status' },
  { title: '自动销毁时间', dataIndex: 'autoDestroyAt', key: 'autoDestroyAt' },
  { title: '操作', key: 'action' }
]

onMounted(fetchEnvironments)
</script>

<template>
  <div>
    <h2>环境管理</h2>
    <a-table
      :columns="columns"
      :data-source="environments"
      :loading="loading"
      row-key="id"
      :pagination="false"
      style="margin-top: 16px;"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'envType'">
          <a-tag :color="envTypeTag(record.envType)">{{ record.envType }}</a-tag>
        </template>
        <template v-else-if="column.key === 'status'">
          <a-tag :color="statusTag(record.status)">{{ record.status }}</a-tag>
        </template>
        <template v-else-if="column.key === 'boundBranch'">
          {{ record.boundBranch || '-' }}
        </template>
        <template v-else-if="column.key === 'autoDestroyAt'">
          {{ record.autoDestroyAt || '-' }}
        </template>
        <template v-else-if="column.key === 'action'">
          <a-button
            v-if="record.status === 'ACTIVE'"
            type="link"
            danger
            @click="handleDestroy(record)"
          >
            销毁
          </a-button>
        </template>
      </template>
    </a-table>
  </div>
</template>
```

- [ ] **Step 3: 编译验证**

Run: `cd forge-portal && npm run build 2>&1 | tail -10`
Expected: 编译通过

- [ ] **Step 4: Commit**

```bash
git add forge-portal/src/
git commit -m "feat(m6): add environment management page and pipeline API"
```

---

### Task 7: 代码浏览器 + Diff 查看器组件

**Files:**
- Create: `forge-portal/src/components/CodeBrowser.vue`
- Create: `forge-portal/src/components/DiffViewer.vue`
- Modify: `forge-portal/src/views/TaskDetailView.vue`（添加代码浏览 Tab）

- [ ] **Step 1: 创建 CodeBrowser.vue — 文件列表 + 内容显示**

```vue
<script setup lang="ts">
import { ref } from 'vue'
import type { TaskStepVO } from '@/api/types'

const props = defineProps<{ steps: TaskStepVO[] }>()

interface FileEntry {
  path: string
  content: string
  action: string
}

const selectedFile = ref<FileEntry | null>(null)

function extractFiles(): FileEntry[] {
  const files: FileEntry[] = []
  for (const step of props.steps) {
    if (step.outputSnapshot && step.stepType === 'CODE_GENERATE') {
      try {
        const parsed = JSON.parse(step.outputSnapshot)
        if (Array.isArray(parsed)) {
          for (const f of parsed) {
            files.push({ path: f.filePath, content: f.content, action: f.action || 'CREATE' })
          }
        }
      } catch {
        // not JSON, skip
      }
    }
  }
  return files
}
</script>

<template>
  <div style="display: flex; gap: 16px;">
    <div style="width: 300px; border-right: 1px solid #f0f0f0; padding-right: 16px;">
      <h4>文件列表</h4>
      <a-list size="small" :data-source="extractFiles()">
        <template #renderItem="{ item }">
          <a-list-item
            style="cursor: pointer; padding: 4px 8px;"
            :style="{ background: selectedFile?.path === item.path ? '#e6f7ff' : 'transparent' }"
            @click="selectedFile = item"
          >
            <a-tag :color="item.action === 'CREATE' ? 'green' : 'blue'" size="small">
              {{ item.action }}
            </a-tag>
            <span style="font-size: 13px; margin-left: 4px;">{{ item.path }}</span>
          </a-list-item>
        </template>
      </a-list>
      <a-empty v-if="extractFiles().length === 0" description="暂无生成文件" />
    </div>
    <div style="flex: 1; overflow: auto;">
      <template v-if="selectedFile">
        <h4>{{ selectedFile.path }}</h4>
        <pre style="background: #fafafa; padding: 16px; border-radius: 4px; font-size: 13px; line-height: 1.6; overflow: auto; max-height: 600px;">{{ selectedFile.content }}</pre>
      </template>
      <a-empty v-else description="请选择文件查看" />
    </div>
  </div>
</template>
```

- [ ] **Step 2: 创建 DiffViewer.vue — 简单 Diff 展示**

```vue
<script setup lang="ts">
import type { TaskStepVO } from '@/api/types'

const props = defineProps<{ steps: TaskStepVO[] }>()

interface DiffEntry {
  filePath: string
  original: string
  modified: string
}

function extractDiffs(): DiffEntry[] {
  const diffs: DiffEntry[] = []
  for (const step of props.steps) {
    if (step.outputSnapshot && (step.stepType === 'CODE_FIX' || step.stepType === 'CODE_REVIEW')) {
      try {
        const parsed = JSON.parse(step.outputSnapshot)
        if (Array.isArray(parsed)) {
          for (const f of parsed) {
            diffs.push({
              filePath: f.filePath,
              original: f.originalContent || '(new file)',
              modified: f.content || ''
            })
          }
        }
      } catch {
        // not JSON
      }
    }
  }
  return diffs
}
</script>

<template>
  <div>
    <div v-for="diff in extractDiffs()" :key="diff.filePath" style="margin-bottom: 24px;">
      <h4>{{ diff.filePath }}</h4>
      <div style="display: flex; gap: 8px;">
        <div style="flex: 1;">
          <div style="background: #fff1f0; padding: 4px 8px; font-size: 12px; font-weight: bold;">原始</div>
          <pre style="background: #fff1f0; padding: 12px; font-size: 12px; line-height: 1.6; overflow: auto; max-height: 400px;">{{ diff.original }}</pre>
        </div>
        <div style="flex: 1;">
          <div style="background: #f6ffed; padding: 4px 8px; font-size: 12px; font-weight: bold;">修改后</div>
          <pre style="background: #f6ffed; padding: 12px; font-size: 12px; line-height: 1.6; overflow: auto; max-height: 400px;">{{ diff.modified }}</pre>
        </div>
      </div>
    </div>
    <a-empty v-if="extractDiffs().length === 0" description="暂无 Diff 数据" />
  </div>
</template>
```

- [ ] **Step 3: 修改 TaskDetailView.vue — 添加代码浏览和 Diff Tab**

在 TaskDetailView.vue 的"执行步骤" card 下方添加 Tabs：

在 `</a-card>` (执行步骤) 后面添加：

```vue
      <a-card style="margin-top: 16px;">
        <a-tabs>
          <a-tab-pane key="code" tab="生成代码">
            <CodeBrowser :steps="steps" />
          </a-tab-pane>
          <a-tab-pane key="diff" tab="AI Diff">
            <DiffViewer :steps="steps" />
          </a-tab-pane>
        </a-tabs>
      </a-card>
```

并在 `<script setup>` 的 import 中添加：

```typescript
import CodeBrowser from '@/components/CodeBrowser.vue'
import DiffViewer from '@/components/DiffViewer.vue'
```

- [ ] **Step 4: 编译验证**

Run: `cd forge-portal && npm run build 2>&1 | tail -10`
Expected: 编译通过

- [ ] **Step 5: Commit**

```bash
git add forge-portal/src/
git commit -m "feat(m6): add code browser and diff viewer components"
```

---

### Task 8: 最终验证 + 构建 + 清理

**Files:**
- 无新文件

- [ ] **Step 1: 完整构建验证**

Run: `cd forge-portal && npm run build 2>&1`
Expected: 无错误，`dist/` 目录生成

- [ ] **Step 2: 检查 dist 输出**

Run: `ls forge-portal/dist/`
Expected: `index.html`, `assets/` 目录存在

- [ ] **Step 3: Commit（如有遗留修改）**

```bash
git add forge-portal/
git commit -m "feat(m6): finalize web console build"
```

---

## M6 完成标准

- [ ] forge-portal 构建通过，`dist/` 正常产出
- [ ] 登录页：用户名 + 密码表单 → 调用 forge-identity 登录 API → 存储 JWT
- [ ] 路由守卫：未登录自动跳转 /login
- [ ] 任务看板：分"进行中"和"已完成"两列展示，5s 自动轮询刷新
- [ ] 创建任务：需求描述文本框 + 任务类型 + 仓库 ID → 调用 forge-engine 创建任务
- [ ] 任务详情：状态、风险等级、步骤时间线、Token 消耗统计
- [ ] 代码浏览器：从步骤 outputSnapshot 提取文件列表，点击查看内容
- [ ] AI Diff 预览：side-by-side 原始 vs 修改后对比
- [ ] 环境管理：列表展示、销毁操作
- [ ] 紧急停止按钮：确认弹窗 → 激活/解除 kill switch
- [ ] 侧边栏导航：看板 / 创建任务 / 环境管理 + 登出
