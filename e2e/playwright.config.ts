import { defineConfig } from '@playwright/test';
import { BASE_URL } from './ports';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false, // 共享同一个后端实例，串行避免数据竞争
  retries: 0,
  workers: 1,
  reporter: [['list']],
  use: {
    baseURL: BASE_URL,
    trace: 'on-first-retry',
    actionTimeout: 8000,
  },
  // globalSetup/Teardown 用相对路径，Playwright 会自行解析
  globalSetup: './global-setup.ts',
  globalTeardown: './global-teardown.ts',
});
