import { defineConfig } from '@playwright/test'
import path from 'node:path'

if (!process.env.VERIFY_OUTPUT) throw new Error('Run npm test from tests/verify to deploy an isolated test environment.')

export default defineConfig({
  testDir: './scenarios',
  outputDir: path.join(process.env.VERIFY_OUTPUT, 'results'),
  workers: 1,
  retries: 0,
  timeout: 60_000,
  reporter: [
    ['list'],
    ['html', { outputFolder: path.join(process.env.VERIFY_OUTPUT, 'report'), open: 'never' }],
  ],
  use: {
    baseURL: process.env.VERIFY_URL,
    ignoreHTTPSErrors: true,
    viewport: { width: 1440, height: 1000 },
    screenshot: 'only-on-failure',
  },
})
