<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useUserStore } from '@/stores/user'
import {
  DashboardOutlined,
  PlusOutlined,
  CloudServerOutlined,
  LogoutOutlined
} from '@ant-design/icons-vue'

const router = useRouter()
const userStore = useUserStore()

function handleLogout() {
  userStore.logout()
  router.push('/login')
}
</script>

<template>
  <a-layout style="min-height: 100vh">
    <a-layout-sider :width="200" theme="dark">
      <div style="height: 48px; line-height: 48px; text-align: center; color: #fff; font-size: 18px; font-weight: bold;">
        Forge
      </div>
      <a-menu theme="dark" mode="inline" @click="({ key }: { key: string }) => router.push(key)">
        <a-menu-item key="/dashboard">
          <DashboardOutlined />
          <span>任务看板</span>
        </a-menu-item>
        <a-menu-item key="/tasks/create">
          <PlusOutlined />
          <span>创建任务</span>
        </a-menu-item>
        <a-menu-item key="/environments">
          <CloudServerOutlined />
          <span>环境管理</span>
        </a-menu-item>
      </a-menu>
    </a-layout-sider>
    <a-layout>
      <a-layout-header style="background: #fff; padding: 0 24px; display: flex; justify-content: flex-end; align-items: center;">
        <span style="margin-right: 16px;">{{ userStore.username }}</span>
        <a-button type="text" @click="handleLogout">
          <LogoutOutlined /> 登出
        </a-button>
      </a-layout-header>
      <a-layout-content style="margin: 24px; padding: 24px; background: #fff; min-height: 280px;">
        <router-view />
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>
