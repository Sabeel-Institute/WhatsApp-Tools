import { test as base, expect, type Page } from '@playwright/test';
import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtempSync, mkdirSync, rmSync } from 'node:fs';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

export const TESTDATA = resolve(import.meta.dirname, '../../testdata');
const SHOTS = process.env.SCREENSHOTS ? resolve(process.env.SCREENSHOTS) : '';

export interface RunningApp {
  url: string;
  base: string;
  token: string;
  proc: ChildProcess;
  dataDir: string;
  exited: Promise<number | null>;
}

function freePort(): Promise<number> {
  return new Promise((ok, fail) => {
    const s = createServer();
    s.listen(0, '127.0.0.1', () => {
      const port = (s.address() as { port: number }).port;
      s.close(() => ok(port));
    });
    s.on('error', fail);
  });
}

/** Starts the app binary in demo mode with its own data folder. */
export async function startApp(): Promise<RunningApp> {
  const bin = resolve(process.env.APP_BIN ?? '../dist/linux/sabeel-whatsapp-tools');
  const port = await freePort();
  const token = 'e2e' + Math.random().toString(36).slice(2, 12);
  const dataDir = mkdtempSync(join(tmpdir(), 'swt-e2e-'));
  const proc = spawn(
    bin,
    ['--demo', '--no-browser', '--port', String(port), '--dev-token', token, '--data-dir', dataDir, '--demo-latency', '0.35'],
    { stdio: ['ignore', 'pipe', 'pipe'] },
  );
  let output = '';
  proc.stdout?.on('data', (d) => (output += d));
  proc.stderr?.on('data', (d) => (output += d));
  const exited = new Promise<number | null>((ok) => proc.on('exit', (code) => ok(code)));
  const base = `http://127.0.0.1:${port}`;
  for (let i = 0; i < 100; i++) {
    try {
      const res = await fetch(`${base}/api/app`, { headers: { 'X-App-Token': token } });
      if (res.ok) return { url: `${base}/#token=${token}`, base, token, proc, dataDir, exited };
    } catch {
      /* not up yet */
    }
    await new Promise((r) => setTimeout(r, 100));
  }
  proc.kill();
  throw new Error(`app did not start:\n${output}`);
}

export async function stopApp(app: RunningApp) {
  if (app.proc.exitCode === null) app.proc.kill('SIGTERM');
  await app.exited;
  rmSync(app.dataDir, { recursive: true, force: true });
}

/** Calls the app's API as the page would. */
export async function apiPost(app: RunningApp, path: string, body: unknown = {}) {
  const res = await fetch(app.base + path, {
    method: 'POST',
    headers: { 'X-App-Token': app.token, 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`${path}: ${res.status} ${await res.text()}`);
  return res.json();
}

/**
 * Saves a screenshot into docs/screenshots when SCREENSHOTS is set: the full
 * page by default, or just the window (for dialogs, whose backdrop covers
 * the window only). Toasts are hidden unless asked for.
 */
export async function shot(page: Page, name: string, opts: { window?: boolean; toast?: boolean } = {}) {
  if (!SHOTS) return;
  mkdirSync(SHOTS, { recursive: true });
  await page.evaluate((toast) => {
    document.documentElement.dataset.capture = toast ? 'toast' : '1';
    return document.fonts.ready;
  }, !!opts.toast);
  if (!opts.window) await page.evaluate(() => window.scrollTo(0, 0));
  await page.waitForTimeout(250);
  await page.screenshot({ path: join(SHOTS, `${name}.png`), fullPage: !opts.window, animations: 'disabled' });
  await page.evaluate(() => delete document.documentElement.dataset.capture);
}

export const test = base.extend<object, { app: RunningApp }>({
  app: [
    async ({}, use) => {
      const app = await startApp();
      await use(app);
      await stopApp(app);
    },
    { scope: 'worker' },
  ],
});

export { expect };
