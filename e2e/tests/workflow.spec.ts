// The whole app, end to end, against the demo WhatsApp account: link,
// create a course group from a real-looking export, add late registrants,
// look at recent activity, and walk through the error states. With
// SCREENSHOTS=<dir> it also saves a screenshot of every screen and state.
import { readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { BrowserContext, Page } from '@playwright/test';
import { TESTDATA, apiPost, expect, shot, test } from './app';

test.describe.configure({ mode: 'serial' });

let context: BrowserContext;
let page: Page;

test.beforeAll(async ({ browser, app }) => {
  context = await browser.newContext({ permissions: ['clipboard-read', 'clipboard-write'] });
  page = await context.newPage();
  page.on('pageerror', (e) => console.error('page error:', e));
  // The UI must never reach the internet.
  page.on('request', (req) => {
    const url = new URL(req.url());
    if (url.hostname !== '127.0.0.1' && url.protocol !== 'data:' && url.protocol !== 'blob:') {
      throw new Error(`unexpected external request: ${req.url()}`);
    }
  });
  await page.goto(app.url);
});

test.afterAll(async () => {
  await context.close();
});

async function continueStep() {
  await page.getByRole('button', { name: 'Continue', exact: true }).click();
}

async function upload(file: string) {
  await page.locator('input[type=file]').setInputFiles(join(TESTDATA, file));
}

async function waitForReview() {
  await expect(page.getByRole('heading', { name: 'Review the list' })).toBeVisible();
  await expect(page.getByText(/Checking which numbers have WhatsApp/)).toHaveCount(0, { timeout: 60_000 });
  await expect(page.getByText('Checking every number…')).toHaveCount(0);
}

test('links WhatsApp with a QR code or a pairing code', async ({ app }) => {
  await expect(page.getByRole('heading', { name: 'Connect your WhatsApp' })).toBeVisible();
  await shot(page, '01-connect-loading');
  await expect(page.locator('.qr svg')).toBeVisible();
  await shot(page, '02-connect-qr');

  await page.getByRole('button', { name: 'Use a phone number instead' }).click();
  await page.getByLabel('WhatsApp phone number').fill('(212) 555-0100');
  await shot(page, '03-connect-phone-form');
  await page.getByRole('button', { name: 'Get my code' }).click();
  await expect(page.getByText('Type this code on the phone')).toBeVisible();
  await shot(page, '04-connect-pairing-code');

  await page.getByRole('button', { name: 'Use a QR code instead' }).click();
  await expect(page.getByRole('heading', { name: 'What would you like to do?' })).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText('Connected')).toBeVisible();
  await expect(page.getByText('Nothing here yet')).toBeVisible();
  await shot(page, '05-home-empty');
  expect(app.proc.exitCode).toBeNull();
});

test('creates a course group from an Eventbrite export', async ({ app }) => {
  await page.getByRole('button', { name: /Create a course group/ }).click();
  await expect(page.getByRole('heading', { name: 'Import the list of people' })).toBeVisible();
  await shot(page, '06-import-empty');

  // A workbook instead of a spreadsheet file gets a clear explanation.
  const xlsx = join(tmpdir(), 'registrations.xlsx');
  writeFileSync(xlsx, 'PK\x03\x04 not really a workbook');
  await page.locator('input[type=file]').setInputFiles(xlsx);
  await expect(page.getByText("That file didn't work")).toBeVisible();
  await shot(page, '07-import-error');

  await upload('eventbrite-attendees.csv');
  await expect(page.getByText('eventbrite-attendees.csv')).toBeVisible();
  await expect(page.getByText('18 rows of people')).toBeVisible();
  await shot(page, '08-import-loaded');

  await continueStep();
  await expect(page.getByRole('heading', { name: 'Match your columns' })).toBeVisible();
  await expect(page.getByLabel('Which column has the phone number?')).toHaveValue(/\d+/);
  await expect(page.locator('select#' + (await page.getByLabel('Which column has the phone number?').getAttribute('id')) + ' option:checked')).toHaveText('Cell Phone');
  await expect(page.getByText('Suggested').first()).toBeVisible();
  await shot(page, '09-match');

  await continueStep();
  await waitForReview();
  await shot(page, '10-review');

  await page.getByRole('button', { name: /Needs attention/ }).click();
  await shot(page, '11-review-needs-attention');
  // Fix Tariq's number, which is missing its area code.
  await page.getByRole('button', { name: 'Edit the number for Tariq Saleh' }).click();
  await page.getByLabel('Phone number for Tariq Saleh').fill('(202) 555-0117');
  await shot(page, '12-review-editing');
  await page.getByLabel('Phone number for Tariq Saleh').press('Enter');
  await expect(page.getByText('Number updated: Tariq Saleh is ready')).toBeVisible();
  await page.getByRole('button', { name: /^\d+ Everyone/ }).click();
  await waitForReview();
  await expect(page.getByRole('button', { name: 'Edit the number for Tariq Saleh' })).toBeVisible();
  await page.getByRole('button', { name: /Duplicates removed/ }).click();
  await expect(page.getByText(/Same number as row 2 \(Aisha Rahman\)/)).toBeVisible();
  await page.getByRole('button', { name: /^\d+ Everyone/ }).click();
  // Untick one person.
  await page.getByRole('checkbox', { name: 'Add Idris Mohamed' }).uncheck();
  await shot(page, '13-review-fixed', { toast: true });

  await continueStep();
  await expect(page.getByRole('heading', { name: 'Group details' })).toBeVisible();
  await expect(page.getByLabel('Community')).toBeVisible();
  await page.getByLabel('Community').selectOption({ label: 'Sabeel Institute' });
  await page.getByLabel('Group name').fill('Tafsir Circle Fall 2026 Evening Class');
  await expect(page.getByText(/too long\. WhatsApp allows up to 25/)).toBeVisible();
  await shot(page, '14-details-name-too-long');
  await page.getByLabel('Group name').fill('Tafsir Circle Fall 2026');
  await page.getByLabel('Description').fill('Weekly tafsir class, Thursdays 7:30 pm. Links and notes are shared here.');
  await page.getByText('More options').click();
  await shot(page, '15-details');

  await continueStep();
  await expect(page.getByRole('heading', { name: 'Ready to create the group' })).toBeVisible();
  await expect(page.getByText(/Create “Tafsir Circle Fall 2026” in “Sabeel Institute” and add \d+ people/)).toBeVisible();
  await shot(page, '16-confirm');

  await page.getByRole('button', { name: /Create group and add \d+ people/ }).click();
  await expect(page.getByText('Latest results')).toBeVisible();
  await expect(page.getByText(/Added \d+ of \d+/)).toBeVisible();
  await expect.poll(async () => Number((await page.locator('.big strong').textContent()) ?? 0), { timeout: 60_000 }).toBeGreaterThan(3);
  await shot(page, '17-progress');

  await page.getByRole('button', { name: 'Pause' }).click();
  await expect(page.getByText('Nothing more is being added')).toBeVisible();
  await shot(page, '18-paused');
  await page.getByRole('button', { name: 'Continue' }).click();

  // The connection drops in the middle of the run: the app waits, then carries on.
  await apiPost(app, '/api/demo/simulate', { what: 'offline' });
  await expect(page.getByText('Waiting for the internet connection to come back')).toBeVisible({ timeout: 30_000 });
  await shot(page, '19-waiting-offline');

  await expect(page.getByRole('heading', { name: '“Tafsir Circle Fall 2026” is ready' })).toBeVisible({ timeout: 120_000 });
  await shot(page, '20-results');

  // Invite link and message.
  await page.getByRole('button', { name: 'Copy invite link' }).first().click();
  await expect(page.getByText('Invite link copied')).toBeVisible();
  const clip = await page.evaluate(() => navigator.clipboard.readText());
  expect(clip).toMatch(/^https:\/\/chat\.whatsapp\.com\//);
  await expect(page.getByLabel(/A message you can paste/)).toHaveValue(/^Assalamu alaikum!/);

  // Results as a spreadsheet file.
  const [download] = await Promise.all([
    page.waitForEvent('download'),
    page.getByRole('button', { name: 'Save results as a spreadsheet' }).click(),
  ]);
  expect(download.suggestedFilename()).toBe('Tafsir Circle Fall 2026 results.csv');
  const csv = readFileSync((await download.path())!, 'utf8');
  expect(csv).toContain('Name,Phone,Result,Invite sent,Row in original file');
  expect(csv).toContain('Aisha Rahman');

  // Opt-in personal invites.
  await page.getByRole('button', { name: 'Send them an invite in WhatsApp' }).click();
  await expect(page.getByRole('dialog')).toBeVisible();
  await shot(page, '21-invite-confirm', { window: true });
  await page.getByRole('button', { name: /^Send \d+ invites?$/ }).click();
  await expect(page.getByText(/Sending invites one at a time/)).toBeVisible();
  await shot(page, '22-invites-sending');
  await expect(page.getByText('Everyone who needed an invite has been sent one.')).toBeVisible({ timeout: 90_000 });
  await page.getByRole('button', { name: /Need an invite/ }).click();
  await shot(page, '23-invites-sent');
});

test('suggests the right columns for every sample export', async () => {
  const expected: Record<string, string> = {
    'stripe-payments.csv': 'Customer Phone',
    'google-form-responses.csv': 'WhatsApp number (with country code, please)',
    'semicolon-windows1252.csv': 'Telefon',
    'no-header.csv': 'Column B',
    'excel-damaged.csv': 'Mobile',
    'excel-unicode-text.tsv': 'Phone Number',
    'quoted-utf8-bom.csv': 'Attendee Phone',
  };
  const importHeading = page.getByRole('heading', { name: 'Import the list of people' });
  await page.getByRole('button', { name: 'Back to home' }).click();
  await page.getByRole('button', { name: /Create a course group/ }).click();
  for (const [file, phoneColumn] of Object.entries(expected)) {
    await expect(importHeading).toBeVisible();
    await upload(file);
    await expect(page.getByText(file, { exact: true })).toBeVisible();
    if (file === 'semicolon-windows1252.csv') await expect(page.getByText('José Muñoz')).toBeVisible();
    if (file === 'no-header.csv') {
      await expect(page.getByText("doesn't have a row of column names")).toBeVisible();
      await shot(page, '24-import-no-header');
    }
    await continueStep();
    const select = page.getByLabel('Which column has the phone number?');
    await expect(select.locator('option:checked')).toHaveText(phoneColumn);
    let backs = 1;
    if (file === 'excel-damaged.csv' || file === 'google-form-responses.csv') {
      await continueStep();
      await waitForReview();
      if (file === 'excel-damaged.csv') {
        await expect(page.getByText('This number was damaged by Excel. Please fix it in the original file.').first()).toBeVisible();
        await shot(page, '25-review-excel-damaged');
      } else {
        await shot(page, '26-review-google-form');
      }
      backs = 2;
    }
    for (let i = 0; i < backs; i++) await page.getByRole('button', { name: 'Back', exact: true }).click();
  }
});

test('adds late registrants to an existing group', async () => {
  await page.getByRole('link', { name: 'Home' }).first().click();
  await page.getByRole('button', { name: /Add people to an existing group/ }).click();
  await upload('late-registrants.csv');
  await continueStep();
  await continueStep();
  await waitForReview();
  await continueStep();
  await expect(page.getByRole('heading', { name: 'Choose the group' })).toBeVisible();
  await page.getByLabel('Community').selectOption({ label: 'Sabeel Institute' });
  await page.getByLabel('Group', { exact: true }).selectOption({ label: 'Arabic Level 1 · 54 members' });
  await shot(page, '27-add-choose-group');
  await continueStep();
  await expect(page.getByText(/Add \d people to “Arabic Level 1” in “Sabeel Institute”/)).toBeVisible();
  await shot(page, '28-add-confirm');
  await page.getByRole('button', { name: /^Add \d people$/ }).click();
  await expect(page.getByRole('heading', { name: 'People added to “Arabic Level 1”' })).toBeVisible({ timeout: 60_000 });
  await shot(page, '29-add-results');
});

test('shows recent activity and reopens a run', async () => {
  await page.getByRole('button', { name: 'Back to home' }).click();
  await expect(page.getByRole('heading', { name: 'Recent activity' })).toBeVisible();
  await expect(page.getByRole('link', { name: /Tafsir Circle Fall 2026/ })).toBeVisible();
  await shot(page, '30-home-recent-activity');
  await page.getByRole('link', { name: /Tafsir Circle Fall 2026/ }).click();
  await expect(page.getByRole('heading', { name: '“Tafsir Circle Fall 2026” is ready' })).toBeVisible();
  await page.getByRole('button', { name: 'Back to home' }).click();
});

test('works at narrow widths', async () => {
  await page.setViewportSize({ width: 900, height: 860 });
  await shot(page, '31-home-900px');
  await page.getByRole('button', { name: /Create a course group/ }).click();
  await upload('stripe-payments.csv');
  await continueStep();
  await continueStep();
  await waitForReview();
  await shot(page, '32-review-900px');
  await page.setViewportSize({ width: 600, height: 900 });
  await shot(page, '33-review-600px');
  await page.setViewportSize({ width: 1280, height: 860 });
  await page.getByRole('link', { name: 'Home' }).first().click();
});

test('handles disconnects, logouts and other trouble', async ({ app }) => {
  // Account menu and the disconnect confirmation.
  await page.getByRole('button', { name: /Sabeel Front Desk/ }).click();
  await shot(page, '34-account-menu', { window: true });
  await page.getByRole('menuitem', { name: /Disconnect WhatsApp/ }).click();
  await expect(page.getByRole('dialog', { name: 'Disconnect WhatsApp?' })).toBeVisible();
  await shot(page, '35-disconnect-confirm', { window: true });
  await page.getByRole('button', { name: 'Cancel' }).click();

  // Unlinked from the phone: back to Connect, with an explanation.
  await apiPost(app, '/api/demo/simulate', { what: 'logout' });
  await expect(page.getByText('WhatsApp disconnected this computer')).toBeVisible();
  await expect(page.locator('.qr svg')).toBeVisible();
  await shot(page, '36-logged-out');
  await expect(page.getByRole('heading', { name: 'What would you like to do?' })).toBeVisible({ timeout: 30_000 });

  // Opened from a bookmark, without the session token.
  const other = await context.newPage();
  await other.goto(app.base + '/');
  await expect(other.getByRole('heading', { name: 'Please open the app from your apps menu' })).toBeVisible();
  await shot(other, '37-no-token');
  await other.close();

  await apiPost(app, '/api/demo/simulate', { what: 'ban' });
  await expect(page.getByRole('heading', { name: 'WhatsApp has paused this account' })).toBeVisible();
  await shot(page, '38-temporarily-banned');

  await apiPost(app, '/api/demo/simulate', { what: 'outdated' });
  await expect(page.getByRole('heading', { name: 'Please update the app' })).toBeVisible();
  await shot(page, '39-update-needed');
});

test('quits cleanly from the Quit button', async ({ app }) => {
  await page.getByRole('button', { name: 'Quit' }).click();
  await expect(page.getByRole('dialog', { name: 'Quit the app?' })).toBeVisible();
  await shot(page, '40-quit-confirm', { window: true });
  await page.getByRole('dialog').getByRole('button', { name: 'Quit' }).click();
  await expect(page.getByRole('heading', { name: 'The app has closed' })).toBeVisible();
  await shot(page, '41-closed');
  expect(await app.exited).toBe(0);
});
