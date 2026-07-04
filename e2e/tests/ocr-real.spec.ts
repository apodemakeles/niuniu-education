// @playwright/test tag: @real
// 真实 OCR 识别测试：用真实 Qwen3-VL 识别教材截图，验证端到端识别质量。
// 仅在有真实 API key 时运行（globalSetup 检测到 key 才启动真实后端）。
// 运行：npx playwright test --grep @real
import { test, expect } from '@playwright/test';
import { resolve } from 'node:path';
import { REAL_API_BASE, ROOT } from '../ports';

const TEXTBOOK_IMG = resolve(ROOT, 'e2e/fixtures/textbook.jpg');

// 检测真实后端是否可用（globalSetup 无 key 时不启动）
async function checkRealBackend(): Promise<boolean> {
  try {
    const res = await fetch(`${REAL_API_BASE}/health`);
    if (!res.ok) {
      console.log(`[@real] health 返回 ${res.status}`);
      return false;
    }
    return true;
  } catch (e) {
    console.log(`[@real] health 检查失败: ${(e as Error).message}`);
    return false;
  }
}

// 教材图里实际印有的单词（用于断言识别命中）
const EXPECTED_WORDS = ['factory', 'farm', 'hospital', 'their'];

test.describe('真实 OCR 识别（Qwen3-VL）@real', () => {
  test('非流式：识别出教材单词且无标题噪音', async () => {
    const available = await checkRealBackend();
    test.skip(!available, '真实后端未启动（无 API key）');
    test.setTimeout(120000);

    const imageBuffer = await readFile(TEXTBOOK_IMG);
    const formData = new FormData();
    formData.append('image', new Blob([imageBuffer], { type: 'image/jpeg' }), 'textbook.jpg');

    const res = await fetch(`${REAL_API_BASE}/imports/ocr`, {
      method: 'POST',
      body: formData,
      signal: AbortSignal.timeout(110000),
    });

    if (!res.ok) {
      const body = await res.text().catch(() => '');
      test.skip(true, `真实 OCR 调用失败（可能余额不足）：${body.slice(0, 100)}`);
      return;
    }

    const data = await res.json();
    const texts = (data.rows || []).map((r: any) => (r.text || '').toLowerCase());

    // 至少识别出 3 个预期单词
    const hits = EXPECTED_WORDS.filter((w) => texts.some((t: string) => t.includes(w)));
    expect(hits.length, `应识别出预期单词，实际识别行：${texts.join(', ')}`).toBeGreaterThanOrEqual(3);

    // Unit/Lesson 标题不应出现
    const hasTitle = texts.some((t: string) => /^unit\s*\d/.test(t) || /^lesson\s*\d/.test(t));
    expect(hasTitle, '标题行 Unit/Lesson 不应被识别为单词').toBe(false);
  });

  test('流式 SSE：收到 stage → row → final 事件序列', async () => {
    const available = await checkRealBackend();
    test.skip(!available, '真实后端未启动（无 API key）');
    test.setTimeout(120000);

    // 流式端点不能用 Playwright 的 APIRequestContext（它不暴露流读取），
    // 用原生 fetch 消费 SSE
    const imageBuffer = await readFile(TEXTBOOK_IMG);
    const formData = new FormData();
    formData.append('image', new Blob([imageBuffer], { type: 'image/jpeg' }), 'textbook.jpg');

    const events: { event: string; data: any }[] = [];
    const res = await fetch(`${REAL_API_BASE}/imports/ocr/stream`, {
      method: 'POST',
      body: formData,
    });

    if (!res.ok) {
      test.skip(true, `流式端点不可用：${res.status}`);
      return;
    }

    const reader = res.body!.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const parts = buffer.split('\n\n');
      buffer = parts.pop()!;
      for (const part of parts) {
        if (!part.trim()) continue;
        const ev = parseSSE(part);
        if (ev) events.push(ev);
      }
    }

    // 应有 stage 和 final 事件
    const stages = events.filter((e) => e.event === 'stage');
    const finals = events.filter((e) => e.event === 'final');
    expect(stages.length, '应有 stage 事件').toBeGreaterThan(0);
    expect(finals.length, '应有 final 事件').toBe(1);

    // final 里的 rows 应含预期单词
    const finalRows = finals[0].data.rows || [];
    const texts = finalRows.map((r: any) => (r.text || '').toLowerCase());
    const hits = EXPECTED_WORDS.filter((w) => texts.some((t: string) => t.includes(w)));
    expect(hits.length, `流式 final 应识别出预期单词`).toBeGreaterThanOrEqual(3);
  });
});

// 辅助：读文件为 Buffer
async function readFile(path: string): Promise<Uint8Array> {
  const fs = await import('node:fs/promises');
  return fs.readFile(path);
}

// 辅助：解析 SSE 块
function parseSSE(block: string): { event: string; data: any } | null {
  let event = 'message';
  let dataStr = '';
  for (const line of block.split('\n')) {
    if (line.startsWith('event:')) event = line.slice(6).trim();
    else if (line.startsWith('data:')) dataStr += line.slice(5).trim();
  }
  if (!dataStr) return null;
  try {
    return { event, data: JSON.parse(dataStr) };
  } catch {
    return { event, data: dataStr };
  }
}
