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
