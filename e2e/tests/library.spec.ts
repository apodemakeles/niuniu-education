import { test, expect } from '@playwright/test';
import { resetLibrary, seedBaseline } from '../helpers';
import { API_BASE } from '../ports';

// 每个测试前重置并植入基线数据，保证断言稳定
test.beforeEach(async ({ request }) => {
  await resetLibrary(request, API_BASE);
  await seedBaseline(request, API_BASE);
});

test.describe('单词库总览与筛选', () => {
  test('打开页面显示预置单词与统计', async ({ page }) => {
    await page.goto('/');

    const strongs = await page.locator('.stats-grid strong').allTextContents();
    expect(strongs).toEqual(['6', '3', '3']);

    await expect(page.locator('tbody tr')).toHaveCount(6);
    await expect(page.getByText('apple')).toBeVisible();
  });

  test('点击"易错词"分类只显示易错词', async ({ page }) => {
    await page.goto('/');
    await page.locator('.library-card').nth(2).click();
    await expect(page.locator('tbody tr')).toHaveCount(3);
    await expect(page.getByText('read').first()).toBeVisible();
    await expect(page.getByText('climb').first()).toBeVisible();
  });

  test('状态筛选选"已掌握"只显示 water', async ({ page }) => {
    await page.goto('/');
    await page.locator('.table-toolbar select').selectOption('mastered');
    await expect(page.locator('tbody tr')).toHaveCount(1);
    await expect(page.getByText('water')).toBeVisible();
  });

  test('搜索框输入 apple 只剩匹配行', async ({ page }) => {
    await page.goto('/');
    await page.locator('.table-toolbar input[type="search"]').fill('apple');
    await expect(page.locator('tbody tr')).toHaveCount(1);
    await expect(page.getByText('apple')).toBeVisible();
  });

  test('搜索中文"书桌"能匹配 desk', async ({ page }) => {
    await page.goto('/');
    await page.locator('.table-toolbar input[type="search"]').fill('书桌');
    await expect(page.locator('tbody tr')).toHaveCount(1);
    await expect(page.locator('tbody tr').first()).toContainText('desk');
  });
});
