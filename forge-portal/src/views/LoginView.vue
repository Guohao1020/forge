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
