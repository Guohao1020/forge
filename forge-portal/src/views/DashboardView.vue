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
