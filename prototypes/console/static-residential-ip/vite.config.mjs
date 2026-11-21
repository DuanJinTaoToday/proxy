import vue from '@vitejs/plugin-vue'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'

const page = name => fileURLToPath(new URL(name, import.meta.url))

export default defineConfig({
  plugins: [vue()],
  build: { rollupOptions: { input: { console: page('index.html'), operator: page('operator.html') } } },
})
