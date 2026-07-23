#!/usr/bin/env node
// 下载 Cambridge + TFD 英式真人发音，构建离线语料库。
//
// 数据源：thousandlemons/English-words-pronunciation-mp3-audio-download 的 ultimate.json
//   - Cambridge 英式：URL 含 dictionary.cambridge.org 且路径含 /uk_pron/
//   - TFD 英式：URL 含 img2.tfd.com 且路径含 /UK/
//
// 产物目录结构（对齐 backend/internal/platform/pronunciation/local_corpus.go 约定）：
//   {outputDir}/cambridge/{word小写}.mp3
//   {outputDir}/tfd/{word小写}.mp3
//
// 用法：
//   export HTTPS_PROXY=http://127.0.0.1:7890   # 国内必填；Node 24+ 内置 fetch 认该变量
//   node download-pronunciation-corpus.mjs --output /path/to/corpus --index ultimate.json
//
// 特性：
//   - 每站独立 8 并发，失败重试 3 次 + 指数退避
//   - 断点续传：跳过已存在且 >1KB 的文件（local_corpus.go 同样以 1KB 为最小有效阈值）
//   - 进度每 500 词打印一次；结束输出 failures.log
//   - 全程不覆盖已有文件

import { readFileSync, existsSync, mkdirSync, writeFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

// ---------- 参数 ----------
const args = parseArgs(process.argv.slice(2));
const OUTPUT_DIR = args.output || '/Users/caozheng/workspace/niuniu-ed-pronuncation';
const INDEX_FILE = args.index || join(OUTPUT_DIR, 'ultimate.json');
const CONCURRENCY = Number(args.concurrency) || 8;
const LIMIT = args.limit ? Number(args.limit) : 0; // >0 时仅下载每库前 N 词，用于冒烟验证
const MAX_RETRIES = 3;
const MIN_BYTES = 1024; // 与 local_corpus.go 的最小有效文件阈值一致

if (!existsSync(INDEX_FILE)) {
  console.error(`索引文件不存在：${INDEX_FILE}`);
  console.error('请先下载：curl -L -o ultimate.json https://raw.githubusercontent.com/thousandlemons/English-words-pronunciation-mp3-audio-download/master/ultimate.json');
  process.exit(1);
}

const proxy = process.env.HTTPS_PROXY || process.env.https_proxy || process.env.HTTP_PROXY || process.env.http_proxy;
if (proxy) {
  console.log(`代理：${proxy}（Node 内置 fetch 自动读取 HTTPS_PROXY）`);
} else {
  console.log('未检测到 HTTPS_PROXY 环境变量，直连下载。国内网络建议先 export HTTPS_PROXY=http://127.0.0.1:7890');
}

// ---------- 筛选 ----------
console.log(`读取索引：${INDEX_FILE}`);
const index = JSON.parse(readFileSync(INDEX_FILE, 'utf8'));
console.log(`索引共 ${Object.keys(index).length.toLocaleString()} 词`);

function selectBritish(index, matchHost, matchPath) {
  // 每词取第一个匹配的英式 URL，避免重复下载。
  const picked = {};
  for (const [word, urls] of Object.entries(index)) {
    if (!Array.isArray(urls)) continue;
    for (const u of urls) {
      const lower = u.toLowerCase();
      if (lower.includes(matchHost) && lower.includes(matchPath)) {
        picked[word] = u;
        break;
      }
    }
  }
  return picked;
}

const cambridge = selectBritish(index, 'dictionary.cambridge.org', '/uk_pron/');
const tfd = selectBritish(index, 'img2.tfd.com', '/uk/');

function maybeLimit(entries) {
  const arr = Object.entries(entries);
  return LIMIT > 0 ? arr.slice(0, LIMIT) : arr;
}

const cambridgeEntries = maybeLimit(cambridge);
const tfdEntries = maybeLimit(tfd);
if (LIMIT > 0) console.log(`⚠️  限制模式：每库仅下载前 ${LIMIT} 词（冒烟验证用）`);
console.log(`筛选：Cambridge 英式 ${Object.keys(cambridge).length.toLocaleString()} 词，TFD 英式 ${Object.keys(tfd).length.toLocaleString()} 词`);

// ---------- 文件名编码 ----------
// local_corpus.go 约定文件名为 {word小写}.mp3；单词原文含 '/' 或空格时无法直接做文件名，
// 这里用百分号编码后小写，解码时 local_corpus 同样 toLowerCase(query.Text)，但磁盘文件名
// 必须与 Lookup 时 strings.ToLower(word) 拼接的结果一致。
// 对含特殊字符的词，Lookup 端 strings.ToLower 不会编码，因此这类词即便下载也无法命中。
// 保持与原行为一致：直接用小写原文做文件名，不安全字符替换为下划线（覆盖率影响极小）。
function fileNameFor(word) {
  let name = word.toLowerCase();
  // 替换文件系统不安全字符；这些词极少，且 local_corpus.go 的 Lookup 也是直接 toLowerCase 拼接，
  // 严格保持一致：含 '/' 的词两端都无法处理，这里只做兜底避免写文件报错。
  name = name.replace(/[/\\]/g, '_');
  return name + '.mp3';
}

// ---------- 下载器 ----------
async function downloadOne(word, url, dir, failures) {
  const file = join(dir, fileNameFor(word));
  // 断点续传：已存在且足够大则跳过
  if (existsSync(file)) {
    try {
      if (statSync(file).size >= MIN_BYTES) return 'skip';
    } catch { /* 文件可能在 stat 时被删，继续重下 */ }
  }

  for (let attempt = 0; attempt <= MAX_RETRIES; attempt++) {
    try {
      const resp = await fetch(url, {
        redirect: 'follow',
        signal: AbortSignal.timeout(10000),
        headers: { 'User-Agent': 'niuniu-education-corpus-builder/1.0' },
      });
      if (resp.status === 429 || resp.status >= 500) {
        // 退避后重试
        await sleep(Math.min(1000 * 2 ** attempt, 8000));
        continue;
      }
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      const buf = new Uint8Array(await resp.arrayBuffer());
      if (buf.byteLength < MIN_BYTES) throw new Error(`文件过小 ${buf.byteLength}B`);
      writeFileSync(file, buf);
      return 'ok';
    } catch (e) {
      if (attempt === MAX_RETRIES) {
        failures.push({ word, url, error: e.message });
        return 'fail';
      }
      await sleep(500 * 2 ** attempt);
    }
  }
  return 'fail';
}

function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

async function runPool(name, entries, concurrency, failures, counters) {
  const dir = join(OUTPUT_DIR, name);
  mkdirSync(dir, { recursive: true });
  const total = entries.length;
  let done = 0;
  let idx = 0;

  async function worker() {
    while (idx < total) {
      const i = idx++;
      const [word, url] = entries[i];
      const result = await downloadOne(word, url, dir, failures);
      done++;
      counters[result]++;
      if (done % 500 === 0 || done === total) {
        const pct = ((done / total) * 100).toFixed(1);
        console.log(`[${name}] ${done.toLocaleString()}/${total.toLocaleString()} (${pct}%) ok=${counters.ok} skip=${counters.skip} fail=${counters.fail}`);
      }
    }
  }

  await Promise.all(Array.from({ length: concurrency }, worker));
}

// ---------- 执行 ----------
mkdirSync(OUTPUT_DIR, { recursive: true });
const camFailures = [];
const tfdFailures = [];
const camCounters = { ok: 0, skip: 0, fail: 0 };
const tfdCounters = { ok: 0, skip: 0, fail: 0 };

const t0 = Date.now();
console.log(`\n开始下载：并发每站 ${CONCURRENCY}，输出目录 ${OUTPUT_DIR}\n`);

// Cambridge 与 TFD 是不同源站，可以并行跑；但为避免日志交叉与便于观察，顺序执行。
// 如需更快可改成 Promise.all，但总并发会翻倍到 16。
await runPool('cambridge', cambridgeEntries, CONCURRENCY, camFailures, camCounters);
console.log('');
await runPool('tfd', tfdEntries, CONCURRENCY, tfdFailures, tfdCounters);

// ---------- 失败清单 ----------
if (camFailures.length) writeFileSync(join(OUTPUT_DIR, 'cambridge-failures.log'), camFailures.map(f => `${f.word}\t${f.url}\t${f.error}`).join('\n'));
if (tfdFailures.length) writeFileSync(join(OUTPUT_DIR, 'tfd-failures.log'), tfdFailures.map(f => `${f.word}\t${f.url}\t${f.error}`).join('\n'));

// ---------- 汇总 ----------
const elapsed = ((Date.now() - t0) / 1000).toFixed(0);
console.log(`\n========== 完成（耗时 ${elapsed}s）==========`);
console.log(`Cambridge: ok=${camCounters.ok} skip=${camCounters.skip} fail=${camCounters.fail}（共 ${Object.keys(cambridge).length}）`);
console.log(`TFD:       ok=${tfdCounters.ok} skip=${tfdCounters.skip} fail=${tfdCounters.fail}（共 ${Object.keys(tfd).length}）`);
if (camFailures.length + tfdFailures.length > 0) {
  console.log(`失败清单已写入 ${OUTPUT_DIR}/{cambridge,tfd}-failures.log`);
  console.log(`提示：可重新运行本脚本，已下载文件会自动跳过，只重试失败项。`);
}

function parseArgs(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a.startsWith('--')) {
      const key = a.slice(2);
      const val = argv[i + 1];
      if (val && !val.startsWith('--')) { out[key] = val; i++; }
      else out[key] = true;
    }
  }
  return out;
}
