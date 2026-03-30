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
