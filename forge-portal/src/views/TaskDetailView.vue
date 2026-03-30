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
