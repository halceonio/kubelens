import { test, expect } from '@playwright/test';

const username = process.env.E2E_USERNAME || 'tester';
const password = process.env.E2E_PASSWORD || 'tester';

const ensureLogin = async (page: any) => {
  await page.goto('/');

  const appReady = page.getByRole('button', { name: 'Groups' });
  try {
    await appReady.waitFor({ timeout: 8000 });
    return;
  } catch {
    // continue to login
  }

  await page.waitForSelector('input#username', { timeout: 15000 });
  await page.fill('input#username', username);
  await page.fill('input#password', password);
  await page.click('#kc-login');
  await appReady.waitFor({ timeout: 20000 });
};

test('capture core views', async ({ page }, testInfo) => {
  await ensureLogin(page);

  await page.getByRole('button', { name: 'Groups' }).click();
  await page.getByRole('button', { name: /demo-suite/i }).click();
  await page.waitForSelector('text=apps/demo-api', { timeout: 15000 });
  await page.screenshot({ path: testInfo.outputPath('screenshots/groups.png'), fullPage: true });

  await page.getByRole('button', { name: /apps\/demo-api/ }).click();
  await page.waitForSelector('text=demo-api', { timeout: 15000 });
  await page.waitForTimeout(1500);
  await page.screenshot({ path: testInfo.outputPath('screenshots/logs.png'), fullPage: true });

  await page.getByRole('button', { name: 'Apps' }).click();
  await page.getByRole('button', { name: 'apps' }).click();
  await page.waitForTimeout(1000);
  await page.screenshot({ path: testInfo.outputPath('screenshots/apps.png'), fullPage: true });

  await page.getByRole('button', { name: 'Pods' }).click();
  await page.getByRole('button', { name: 'apps' }).click();
  await page.waitForTimeout(1000);
  await page.screenshot({ path: testInfo.outputPath('screenshots/pods.png'), fullPage: true });
});
