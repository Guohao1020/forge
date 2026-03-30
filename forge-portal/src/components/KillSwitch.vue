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
