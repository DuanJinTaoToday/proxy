import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    proxy: {
      '/public/': process.env.PROXY_CONTROL_URL || 'http://127.0.0.1:8000',
      '/v1/': process.env.PROXY_CONTROL_URL || 'http://127.0.0.1:8000',
    },
  },
})
