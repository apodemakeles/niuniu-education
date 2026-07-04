import { test, expect } from '@playwright/test';
import { resolve } from 'node:path';
import { resetLibrary } from '../helpers';
import { API_BASE, ROOT } from '../ports';

const TEST_IMAGE = resolve(ROOT, 'e2e/fixtures/test.png');

async function expectTotal(page: import('@playwright/test').Page, n: number) {
  await expect(page.locator('.stats-grid strong').first()).toHaveText(String(n));
}

test.beforeEach(async ({ request }) => {
  await resetLibrary(request, API_BASE);
});

test.describe('拍照导入（mock OCR）', () => {
  test('上传图片 → mock OCR 草稿 → 修正 → 确认入库', async ({ page }) => {
    await page.goto('/');
    await expectTotal(page, 0);

    await page.getByRole('button', { name: '拍照导入' }).click();
    await page.locator('input[type="file"]').setInputFiles(TEST_IMAGE);
    await page.getByRole('button', { name: '识别并生成草稿' }).click();

    // 草稿表出现：mock 返回 clirnb/window/chair 三行
    await expect(page.getByText('拍照识别草稿')).toBeVisible();
    await expect(page.locator('.draft-table tbody tr')).toHaveCount(3);

    // 修正 clirnb → climb（第一行第一个输入框）
    await page.locator('.draft-table tbody tr').first().locator('input').first().fill('climb');

    await page.getByRole('button', { name: '确认入库' }).click();

    await expect(page.locator('.toast')).toContainText('已入库');
    // mock 的 climb/window/chair 全部入库（之前是空库，无重复）
    await expectTotal(page, 3);
    await expect(page.getByText('climb').first()).toBeVisible();
  });
});

test.describe('粘贴导入', () => {
  test('粘贴文本 → 解析草稿 → 确认入库', async ({ page }) => {
    await page.goto('/');
    await expectTotal(page, 0);

    await page.getByRole('button', { name: '粘贴导入' }).click();
    await page.locator('textarea').fill('tiger 老虎 /ˈtaɪɡə/\nlion,狮子');
    await page.getByRole('button', { name: '生成草稿预览' }).click();

    await expect(page.getByText('粘贴导入草稿')).toBeVisible();
    await expect(page.locator('.draft-table tbody tr')).toHaveCount(2);

    await page.getByRole('button', { name: '确认入库' }).click();

    await expect(page.locator('.toast')).toContainText('已入库');
    await expectTotal(page, 2);
    await expect(page.getByText('tiger').first()).toBeVisible();
    await expect(page.getByText('lion').first()).toBeVisible();
  });
});
