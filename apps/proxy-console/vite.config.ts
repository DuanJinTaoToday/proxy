import vue from '@vitejs/plugin-vue'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'

const control = process.env.PROXY_CONTROL_URL || 'http://127.0.0.1:8000'

export default defineConfig({
  plugins: [vue()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: {
    proxy: {
      '/v1/': control,
      '/public/': control,
    },
  },
})
