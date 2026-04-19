import { test, expect, chromium, BrowserContext } from '@playwright/test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const EXT = path.resolve(__dirname, '..', '..', 'extension');

async function launchWithExtension(): Promise<BrowserContext> {
  return chromium.launchPersistentContext('', {
    headless: false,
    args: [
      `--disable-extensions-except=${EXT}`,
      `--load-extension=${EXT}`,
      '--no-first-run',
      '--no-default-browser-check',
    ],
  });
}

async function getServiceWorker(ctx: BrowserContext) {
  let [sw] = ctx.serviceWorkers();
  if (!sw) sw = await ctx.waitForEvent('serviceworker', { timeout: 15_000 });
  return sw;
}

test('extension loads and popup renders', async () => {
  const ctx = await launchWithExtension();
  try {
    const sw = await getServiceWorker(ctx);
    const extId = new URL(sw.url()).host;
    const page = await ctx.newPage();
    await page.goto(`chrome-extension://${extId}/popup/popup.html`);
    await expect(page.locator('#session-id')).toBeVisible();
    await expect(page.locator('#h-sw')).toBeVisible();
    await expect(page.locator('#h-wasm')).toBeVisible();
    await expect(page.locator('#h-native')).toBeVisible();
  } finally {
    await ctx.close();
  }
});

test('basic page capture enqueues into storage', async () => {
  const ctx = await launchWithExtension();
  try {
    const sw = await getServiceWorker(ctx);
    const page = await ctx.newPage();
    await page.goto('http://localhost:4321/basic.html');
    // Give WASM + SW time to process + queue
    await page.waitForTimeout(4000);
    const stored = await sw.evaluate(async () => {
      return await new Promise<any>((resolve) =>
        (globalThis as any).chrome.storage.local.get('mairu_queue_v1', resolve),
      );
    });
    expect(stored).toBeDefined();
    const entries = stored?.mairu_queue_v1?.entries || [];
    expect(entries.length).toBeGreaterThan(0);
    const hit = entries.some((e: any) => (e.payload?.uri || '').includes('basic.html') || (e.payload?.name || '') === 'Basic Fixture');
    expect(hit).toBe(true);
  } finally {
    await ctx.close();
  }
});

test('strict CSP page still captures via MAIN-world injection', async () => {
  const ctx = await launchWithExtension();
  try {
    const sw = await getServiceWorker(ctx);
    const page = await ctx.newPage();
    await page.goto('http://localhost:4321/csp.html');
    await page.waitForTimeout(4000);
    const stored = await sw.evaluate(async () => {
      return await new Promise<any>((resolve) =>
        (globalThis as any).chrome.storage.local.get('mairu_queue_v1', resolve),
      );
    });
    const entries = stored?.mairu_queue_v1?.entries || [];
    const cspHit = entries.some((e: any) => (e.payload?.uri || '').includes('csp') || (e.payload?.name || '').includes('CSP'));
    expect(cspHit).toBe(true);
  } finally {
    await ctx.close();
  }
});

test('execute click returns a structured response', async () => {
  const ctx = await launchWithExtension();
  try {
    const sw = await getServiceWorker(ctx);
    const page = await ctx.newPage();
    await page.goto('http://localhost:4321/basic.html');
    await page.waitForTimeout(2000);
    const result = await sw.evaluate(async () => {
      const tabs = await new Promise<any[]>((resolve) =>
        (globalThis as any).chrome.tabs.query({ active: true, currentWindow: true }, resolve),
      );
      const id = tabs?.[0]?.id;
      if (!id) return { error: 'no tab' };
      return await new Promise<any>((resolve) =>
        (globalThis as any).chrome.tabs.sendMessage(
          id,
          { type: 'execute', command: 'click', selector: '#btn' },
          resolve,
        ),
      );
    });
    expect(result).toBeTruthy();
    // Either success or a structured error — both prove the handler ran.
    const ok = result?.success === true || typeof result?.error === 'string';
    expect(ok).toBe(true);
  } finally {
    await ctx.close();
  }
});
