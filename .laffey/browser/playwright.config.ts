import { defineConfig } from '@playwright/test'

const executablePath = process.env.PLAYWRIGHT_EXECUTABLE_PATH

export default defineConfig({
  outputDir: 'test-results',
  testDir: '.',
  use: {
    launchOptions: executablePath ? { executablePath } : {},
  },
})
