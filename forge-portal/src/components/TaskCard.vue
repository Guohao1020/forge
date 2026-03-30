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
