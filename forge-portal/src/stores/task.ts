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
