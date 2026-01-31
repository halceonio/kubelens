import { defineConfig } from '@playwright/test';

const baseURL = process.env.E2E_BASE_URL || 'http://localhost:3000';

export default defineConfig({
  testDir: './e2e',
  outputDir: 'test-results',
  timeout: 60000,
  expect: {
    timeout: 15000
  },
  reporter: [['list'], ['html', { outputFolder: 'test-results/html', open: 'never' }]],
  use: {
    baseURL,
    screenshot: 'off',
    video: 'off',
    trace: 'off'
  }
});
