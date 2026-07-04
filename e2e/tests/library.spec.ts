import { test, expect } from '@playwright/test';
import { resetLibrary, seedBaseline, seedManyWords } from '../helpers';
import { API_BASE } from '../ports';

test.beforeEach(async ({ request }) => {
  await resetLibrary(request, API_BASE);
  await seedBaseline(request, API_BASE);
});

test.describe('单词库总览与筛选', () => {
  test('页面加载完成后不卡在加载中', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText('加载中…')).toBeHidden({ timeout: 10000 });
    await expect(page.locator('.stats-grid strong').first()).toHaveText('6');
    await expect(page.locator('tbody tr')).toHaveCount(6);
  });

  test('打开页面显示预置单词与统计', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText('加载中…')).toBeHidden({ timeout: 10000 });

    const strongs = await page.locator('.stats-grid strong').allTextContents();
    expect(strongs).toEqual(['6', '3', '3']);

    await expect(page.locator('tbody tr')).toHaveCount(6);
    await expect(page.getByText('apple')).toBeVisible();
  });

  test('点击"易错词"分类触发服务端查询', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText('加载中…')).toBeHidden({ timeout: 10000 });

    const respPromise = page.waitForResponse(
      (r) => r.url().includes('/api/v1/words') && r.url().includes('type=mistake'),
    );
    await page.locator('.library-card').nth(2).click();
    const resp = await respPromise;
    expect(resp.ok()).toBeTruthy();

    await expect(page.locator('tbody tr')).toHaveCount(3);
    await expect(page.getByText('read').first()).toBeVisible();
    await expect(page.getByText('climb').first()).toBeVisible();
  });

  test('点击"新词"分类触发服务端查询', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText('加载中…')).toBeHidden({ timeout: 10000 });

    const respPromise = page.waitForResponse(
      (r) => r.url().includes('/api/v1/words') && r.url().includes('type=new'),
    );
    await page.locator('.library-card').nth(1).click();
    const resp = await respPromise;
    expect(resp.ok()).toBeTruthy();

    await expect(page.locator('tbody tr')).toHaveCount(3);
    await expect(page.getByText('apple')).toBeVisible();
  });

  test('状态筛选选"已掌握"只显示 water', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText('加载中…')).toBeHidden({ timeout: 10000 });

    const respPromise = page.waitForResponse(
      (r) => r.url().includes('/api/v1/words') && r.url().includes('status=mastered'),
    );
    await page.locator('.table-toolbar select').selectOption('mastered');
    await respPromise;

    await expect(page.locator('tbody tr')).toHaveCount(1);
    await expect(page.getByText('water')).toBeVisible();
  });

  test('搜索框输入 apple 只剩匹配行', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText('加载中…')).toBeHidden({ timeout: 10000 });

    const respPromise = page.waitForResponse(
      (r) => r.url().includes('/api/v1/words') && r.url().includes('q=apple'),
    );
    await page.locator('.table-toolbar input[type="search"]').fill('apple');
    await respPromise;

    await expect(page.locator('tbody tr')).toHaveCount(1);
    await expect(page.getByText('apple')).toBeVisible();
  });

  test('搜索中文"书桌"能匹配 desk', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText('加载中…')).toBeHidden({ timeout: 10000 });

    await page.locator('.table-toolbar input[type="search"]').fill('书桌');
    await page.waitForResponse((r) => r.url().includes('/api/v1/words') && r.url().includes('q='));

    await expect(page.locator('tbody tr')).toHaveCount(1);
    await expect(page.locator('tbody tr').first()).toContainText('desk');
  });
});

test.describe('单词列表分页', () => {
  test.beforeEach(async ({ request }) => {
    await seedManyWords(request, API_BASE, 25);
  });

  test('超过一页时显示分页并可翻页', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByText('加载中…')).toBeHidden({ timeout: 10000 });

    await expect(page.locator('.pagination-bar')).toBeVisible();
    await expect(page.locator('tbody tr')).toHaveCount(20);
    await expect(page.locator('.pagination-info')).toContainText('1–20 / 共 31 条');

    const respPromise = page.waitForResponse(
      (r) => r.url().includes('/api/v1/words') && r.url().includes('page=2'),
    );
    await page.getByRole('button', { name: '下一页' }).click();
    await respPromise;

    await expect(page.locator('tbody tr')).toHaveCount(11);
    await expect(page.locator('.pagination-info')).toContainText('21–31 / 共 31 条');
  });
});
