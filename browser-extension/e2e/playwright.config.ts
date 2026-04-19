import { defineConfig } from '@playwright/test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const EXT = path.resolve(__dirname, '..', 'extension');

export default defineConfig({
  testDir: './tests',
  timeout: 60_000,
  retries: 0,
  reporter: [['list']],
  use: {
    channel: 'chromium',
    headless: false,
    launchOptions: {
      args: [
        `--disable-extensions-except=${EXT}`,
        `--load-extension=${EXT}`,
        '--no-first-run',
        '--no-default-browser-check',
      ],
    },
  },
  webServer: {
    command: 'bun run serve',
    port: 4321,
    reuseExistingServer: true,
    cwd: __dirname,
    timeout: 10_000,
  },
});
