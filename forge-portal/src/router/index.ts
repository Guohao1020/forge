import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/views/LoginView.vue'),
      meta: { requiresAuth: false }
    },
    {
      path: '/',
      redirect: '/dashboard'
    },
    {
      path: '/dashboard',
      name: 'Dashboard',
      component: () => import('@/views/DashboardView.vue')
    },
    {
      path: '/tasks/create',
      name: 'TaskCreate',
      component: () => import('@/views/TaskCreateView.vue')
    },
    {
      path: '/tasks/:id',
      name: 'TaskDetail',
      component: () => import('@/views/TaskDetailView.vue')
    },
    {
      path: '/environments',
      name: 'Environments',
      component: () => import('@/views/EnvironmentView.vue')
    }
  ]
})

router.beforeEach((to) => {
  const token = localStorage.getItem('accessToken')
  if (to.meta.requiresAuth !== false && !token) {
    return { name: 'Login' }
  }
})

export default router
