import type { APIRequestContext } from '@playwright/test';

// 用 `import type` 避免 @playwright/test 在运行时重复加载。
// request context 由调用方（spec 的 fixture）传入。

// 物理清空测试库的全部单词（含软删行）。
// 用专用测试端点 POST /_test/reset（仅 e2e 环境启用，生产路由表不含）。
export async function resetLibrary(ctx: APIRequestContext, apiBase: string) {
  await ctx.post(`${apiBase}/_test/reset`);
}

// 插入预置的 6 个基线单词（与迁移脚本预置一致）。
export async function seedBaseline(ctx: APIRequestContext, apiBase: string) {
  const baseline = [
    { text: 'apple', meaningZh: '苹果', phonetic: '/ˈæpl/', wordType: 'new', status: 'unlearned' },
    { text: 'read', meaningZh: '阅读', phonetic: '/riːd/', wordType: 'mistake', status: 'reinforce' },
    { text: 'desk', meaningZh: '书桌', phonetic: '/desk/', wordType: 'new', status: 'learning' },
    { text: 'climb', meaningZh: '攀爬', phonetic: '/klaɪm/', wordType: 'mistake', status: 'reinforce' },
    { text: 'water', meaningZh: '水', phonetic: '/ˈwɔːtər/', wordType: 'new', status: 'mastered' },
    { text: 'their', meaningZh: '他们的', phonetic: '/ðer/', wordType: 'mistake', status: 'reinforce' },
  ];
  for (const w of baseline) {
    const r = await ctx.post(`${apiBase}/words`, { data: w });
    if (r.ok()) {
      const created = await r.json();
      if (created.status !== w.status) {
        await ctx.put(`${apiBase}/words/${created.id}`, {
          data: { meaningZh: w.meaningZh, phonetic: w.phonetic, wordType: w.wordType, status: w.status },
        });
      }
    }
  }
}

// 批量插入测试单词，用于分页 e2e。
export async function seedManyWords(ctx: APIRequestContext, apiBase: string, count: number) {
  for (let i = 1; i <= count; i++) {
    await ctx.post(`${apiBase}/words`, {
      data: {
        text: `word${i}`,
        meaningZh: `词${i}`,
        phonetic: '',
        wordType: 'new',
      },
    });
  }
}

export { API_BASE } from './ports';
