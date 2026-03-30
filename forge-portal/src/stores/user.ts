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
