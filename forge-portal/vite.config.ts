import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath } from 'node:url'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  server: {
    port: 5173,
    proxy: {
      '/api/auth': {
        target: 'http://localhost:8082',
        changeOrigin: true
      },
      '/api/users': {
        target: 'http://localhost:8082',
        changeOrigin: true
      },
      '/api/roles': {
        target: 'http://localhost:8082',
        changeOrigin: true
      },
      '/api/tasks': {
        target: 'http://localhost:8081',
        changeOrigin: true
      },
      '/api/killswitch': {
        target: 'http://localhost:8081',
        changeOrigin: true
      },
      '/api/token-usage': {
        target: 'http://localhost:8081',
        changeOrigin: true
      },
      '/api/standards': {
        target: 'http://localhost:8084',
        changeOrigin: true
      },
      '/api/prompts': {
        target: 'http://localhost:8084',
        changeOrigin: true
      },
      '/api/review-rules': {
        target: 'http://localhost:8084',
        changeOrigin: true
      },
      '/api/pipelines': {
        target: 'http://localhost:8083',
        changeOrigin: true
      },
      '/api/deployments': {
        target: 'http://localhost:8083',
        changeOrigin: true
      },
      '/api/environments': {
        target: 'http://localhost:8083',
        changeOrigin: true
      },
      '/api/webhooks': {
        target: 'http://localhost:8083',
        changeOrigin: true
      }
    }
  }
})
