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
  test('Qwen 占位斜杠格式 → 无音标词条不以斜杠结尾', async ({ request }) => {
    const res = await request.post(`${API_BASE}/imports/parse`, {
      data: {
        text: `middle school / 中学\nfactory / 'fæktri/ / 工厂\ntake care of / 保管；照顾`,
      },
    });
    expect(res.ok()).toBeTruthy();
    const data = await res.json();
    const rows = data.rows || [];
    const byText = Object.fromEntries(rows.map((r: { text: string }) => [r.text, r]));

    expect(byText['middle school']?.text).toBe('middle school');
    expect(byText['middle school']?.phonetic).toBe('');
    expect(byText['take care of']?.text).not.toMatch(/\/\s*$/);
    expect(byText['factory']?.phonetic).not.toMatch(/ \/$/);
    expect(byText['factory']?.phonetic).toMatch(/^\/.*\/$/);
  });

  test('括号注记应归入中文释义而非英文', async ({ request }) => {
    const res = await request.post(`${API_BASE}/imports/parse`, {
      data: {
        text: `Ms /mɪz/ （用于女子的姓氏或姓名前，不指明婚否）女士\nNice to see you! （以前见过面的人之间用）见到你很高兴！`,
      },
    });
    expect(res.ok()).toBeTruthy();
    const rows = (await res.json()).rows || [];
    const byText = Object.fromEntries(rows.map((r: { text: string }) => [r.text, r]));

    expect(byText['Ms']?.text).toBe('Ms');
    expect(byText['Ms']?.meaningZh).toBe('（用于女子的姓氏或姓名前，不指明婚否）女士');
    expect(byText['Nice to see you!']?.text).toBe('Nice to see you!');
    expect(byText['Nice to see you!']?.meaningZh).toBe('（以前见过面的人之间用）见到你很高兴！');
  });

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
