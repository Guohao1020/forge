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
