import process from 'node:process'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(() => {
  const backendTarget = process.env.API_MANAGER_DEV_TARGET || 'http://127.0.0.1:9000'

  return {
    plugins: [react()],
    server: {
      proxy: {
        '/api': backendTarget,
        '/health': backendTarget,
      },
    },
    build: {
      outDir: '../backend/dist',
      emptyOutDir: true,
    },
  }
})
