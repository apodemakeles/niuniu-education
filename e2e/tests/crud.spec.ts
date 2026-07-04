import { test, expect } from '@playwright/test';
import { resetLibrary } from '../helpers';
import { API_BASE } from '../ports';

// 断言单词总数（统计卡第一个数字），避免被"没有匹配的单词"提示行干扰
async function expectTotal(page: import('@playwright/test').Page, n: number) {
  await expect(page.locator('.stats-grid strong').first()).toHaveText(String(n));
}

test.beforeEach(async ({ request }) => {
  await resetLibrary(request, API_BASE);
});

test.describe('手动录入', () => {
  test('逐个录入单词并出现在列表', async ({ page }) => {
    await page.goto('/');
    await expectTotal(page, 0);

    await page.getByRole('button', { name: '逐个录入' }).click();
    await page.getByTestId('create-text').fill('jump');
    await page.getByTestId('create-meaning').fill('跳跃');
    await page.getByRole('button', { name: '保存并继续' }).click();

    await expect(page.locator('.toast')).toContainText('已保存');
    await expect(page.getByText('jump')).toBeVisible();
    await expectTotal(page, 1);
  });

  test('录入同形异义词（text 相同释义不同）成功', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: '逐个录入' }).click();
    await page.getByTestId('create-text').fill('light');
    await page.getByTestId('create-meaning').fill('轻的');
    await page.getByRole('button', { name: '保存并继续' }).click();
    await expect(page.locator('.toast')).toContainText('已保存');

    // 同 text 不同释义（同形词），去重按 text+meaningZh 不拦截
    await page.getByTestId('create-text').fill('light');
    await page.getByTestId('create-meaning').fill('光');
    await page.getByRole('button', { name: '保存并继续' }).click();
    await expect(page.locator('.toast')).toContainText('已保存');
    await expectTotal(page, 2);
  });
});

test.describe('编辑', () => {
  test('编辑单词中文并更新列表', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: '逐个录入' }).click();
    await page.getByTestId('create-text').fill('run');
    await page.getByTestId('create-meaning').fill('跑');
    await page.getByRole('button', { name: '保存并继续' }).click();
    await page.getByRole('button', { name: '完成' }).click();

    await page.getByRole('button', { name: '编辑' }).last().click();
    await page.getByTestId('edit-meaning').fill('奔跑');
    await page.getByRole('button', { name: '保存修改' }).click();

    await expect(page.locator('.toast')).toContainText('已更新');
    await expect(page.getByText('奔跑')).toBeVisible();
  });
});

test.describe('删除', () => {
  test('删除未学单词（物理删除）后总数减少', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: '逐个录入' }).click();
    await page.getByTestId('create-text').fill('deltest');
    await page.getByTestId('create-meaning').fill('删测');
    await page.getByRole('button', { name: '保存并继续' }).click();
    await page.getByRole('button', { name: '完成' }).click();
    await expectTotal(page, 1);

    page.on('dialog', (d) => d.accept());
    await page.locator('tbody tr', { hasText: 'deltest' }).getByRole('button', { name: '删除' }).click();

    await expect(page.locator('.toast')).toContainText('已直接删除');
    await expectTotal(page, 0);
  });

  test('删除学习中单词提示保留学习记录', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: '逐个录入' }).click();
    await page.getByTestId('create-text').fill('dellearned');
    await page.getByTestId('create-meaning').fill('删学');
    await page.getByRole('button', { name: '保存并继续' }).click();
    await page.getByRole('button', { name: '完成' }).click();

    // 改为学习中
    await page.locator('tbody tr', { hasText: 'dellearned' }).getByRole('button', { name: '编辑' }).click();
    await page.locator('.modal select').nth(1).selectOption('learning');
    await page.getByRole('button', { name: '保存修改' }).click();
    await expectTotal(page, 1);

    page.on('dialog', (d) => d.accept());
    await page.locator('tbody tr', { hasText: 'dellearned' }).getByRole('button', { name: '删除' }).click();

    await expect(page.locator('.toast')).toContainText('学习记录保留');
    await expectTotal(page, 0);
  });
});
