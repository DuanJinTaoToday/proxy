import { fileURLToPath } from 'node:url'
import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

const root = fileURLToPath(new URL('../../prototypes/console', import.meta.url))
const consoleSource = fileURLToPath(new URL('./src', import.meta.url))
const nodeModules = new URL('./node_modules/', import.meta.url)
const dependencies = ['vue', 'vue-router', 'lucide-vue-next', 'reka-ui']

export default defineConfig({
  root,
  cacheDir: fileURLToPath(new URL('.vite-prototypes', nodeModules)),
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@console': consoleSource,
      ...Object.fromEntries(dependencies.map(name => [
        name, fileURLToPath(new URL(name, nodeModules)),
      ])),
    },
  },
  server: {
    fs: { allow: [root, consoleSource, fileURLToPath(nodeModules)] },
  },
})
