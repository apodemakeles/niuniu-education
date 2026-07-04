import { execSync, spawn, ChildProcess } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { BACKEND_PORT, E2E_DATA_DIR, FRONTEND_PORT, REAL_BACKEND_PORT, ROOT } from './ports';

// 存放子进程引用，供 teardown 使用
const STATE_FILE = resolve(ROOT, 'e2e/.run-state.json');

let backendProc: ChildProcess | null = null;
let frontendProc: ChildProcess | null = null;

function ensureBackendBuilt() {
  const bin = resolve(ROOT, 'bin/niuniu-api');
  // 每次 e2e 前重建后端，确保 API 与源码一致
  console.log('[e2e] 构建后端...');
  execSync('CGO_ENABLED=0 go build -o ../bin/niuniu-api ./cmd/niuniu', {
    cwd: resolve(ROOT, 'backend'),
    stdio: 'inherit',
  });
}

function ensureFrontendBuilt(backendPort: number) {
  // e2e 每次重建，确保测到最新前端代码
  const e2eDist = resolve(ROOT, 'frontend/.e2e-dist');
  rmSync(e2eDist, { recursive: true, force: true });
  console.log('[e2e] 构建 e2e 专用前端（VITE_API_BASE 指向 :', backendPort, ')...');
  // 用环境变量覆盖 VITE_API_BASE，输出到 .e2e-dist 避免污染 dist
  // vite 不支持直接改 outDir via env，改用单独构建目录
  execSync(
    `VITE_API_BASE=http://localhost:${backendPort}/api/v1 npx vite build --outDir .e2e-dist --emptyOutDir`,
    {
      cwd: resolve(ROOT, 'frontend'),
      stdio: 'inherit',
      env: {
        ...process.env,
        VITE_API_BASE: `http://localhost:${backendPort}/api/v1`,
      },
    },
  );
}

async function waitFor(url: string, timeoutMs = 30000): Promise<void> {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    try {
      const res = await fetch(url);
      if (res.ok || res.status === 404) return; // 服务已响应
    } catch {
      // 还没起来，继续等
    }
    await new Promise((r) => setTimeout(r, 300));
  }
  throw new Error(`等待 ${url} 超时`);
}

export default async function globalSetup() {
  console.log('[e2e] globalSetup 开始');

  // 清理旧测试数据
  const dataDir = resolve(ROOT, E2E_DATA_DIR);
  rmSync(dataDir, { recursive: true, force: true });
  mkdirSync(dataDir, { recursive: true });

  ensureBackendBuilt();
  ensureFrontendBuilt(BACKEND_PORT);

  // 启动后端：独立数据目录 + mock OCR + 独立端口
  backendProc = spawn(
    resolve(ROOT, 'bin/niuniu-api'),
    ['-port', String(BACKEND_PORT), '-data-dir', dataDir],
    {
      env: {
        ...process.env,
        NIUNIU_OCR_PROVIDER: 'mock',
        NIUNIU_PORT: String(BACKEND_PORT),
        NIUNIU_CORS_ALLOWED_ORIGINS: `http://localhost:${FRONTEND_PORT}`,
        NIUNIU_ENABLE_TEST_ENDPOINTS: '1',
      },
      stdio: 'ignore',
      detached: false,
    },
  );
  backendProc.on('error', (e) => {
    console.error('[e2e] 后端启动失败:', e);
    throw e;
  });

  // 启动真实 provider 后端（@real 测试用）：读 data/config.yaml 的真实 key
  // 若无 key 则跳过，@real 测试会自动 skip
  let realBackendProc: ChildProcess | null = null;
  const realKey = readRealAPIKey();
  if (realKey) {
    console.log('[e2e] 启动真实 provider 后端 :', REAL_BACKEND_PORT);
    realBackendProc = spawn(
      resolve(ROOT, 'bin/niuniu-api'),
      ['-port', String(REAL_BACKEND_PORT), '-data-dir', resolve(dataDir, 'real')],
      {
        env: {
          ...process.env,
          NIUNIU_OCR_PROVIDER: 'siliconflow',
          NIUNIU_OCR_API_KEY: realKey,
          NIUNIU_OCR_ENDPOINT: 'https://api.siliconflow.cn/v1',
          NIUNIU_OCR_MODEL: 'Qwen/Qwen3-VL-32B-Instruct',
          NIUNIU_PORT: String(REAL_BACKEND_PORT),
          NIUNIU_CORS_ALLOWED_ORIGINS: `http://localhost:${FRONTEND_PORT}`,
          NIUNIU_ENABLE_TEST_ENDPOINTS: '1',
          https_proxy: 'http://127.0.0.1:7890',
          http_proxy: 'http://127.0.0.1:7890',
        },
        stdio: 'ignore',
        detached: false,
      },
    );
  } else {
    console.log('[e2e] 未检测到真实 API key，跳过真实 provider 后端（@real 测试将 skip）');
  }

  // 启动前端：vite preview，指向 .e2e-dist，--port 指定，--strictPort 占用即报错
  frontendProc = spawn(
    'npx',
    ['vite', 'preview', '--outDir', '.e2e-dist', '--port', String(FRONTEND_PORT), '--strictPort'],
    {
      cwd: resolve(ROOT, 'frontend'),
      env: process.env,
      stdio: 'ignore',
      detached: false,
      shell: true,
    },
  );
  frontendProc.on('error', (e) => {
    console.error('[e2e] 前端启动失败:', e);
    throw e;
  });

  // 等待服务就绪
  console.log('[e2e] 等待后端就绪...');
  await waitFor(`http://localhost:${BACKEND_PORT}/api/v1/health`);
  console.log('[e2e] 等待前端就绪...');
  await waitFor(`http://localhost:${FRONTEND_PORT}/`);
  if (realBackendProc) {
    console.log('[e2e] 等待真实 provider 后端就绪...');
    await waitFor(`http://localhost:${REAL_BACKEND_PORT}/api/v1/health`);
  }
  console.log('[e2e] 前后端均已就绪');

  // 持久化 PID 给 teardown（进程对象本身无法跨文件传递）
  writeFileSync(
    STATE_FILE,
    JSON.stringify({
      backendPid: backendProc.pid,
      frontendPid: frontendProc.pid,
      realBackendPid: realBackendProc?.pid,
    }),
  );
}

// 从 data/config.yaml 读真实 OCR apiKey（@real 测试用）。
// 优先读 data/config.yaml，其次读 ~/work/deepseek-ocr-demo/.env。
function readRealAPIKey(): string | null {
  // 1. data/config.yaml
  const cfgPath = resolve(ROOT, 'data/config.yaml');
  if (existsSync(cfgPath)) {
    const content = readFileSync(cfgPath, 'utf-8');
    const m = content.match(/apiKey:\s*(sk-\S+)/);
    if (m && m[1] && m[1].length > 10) return m[1];
  }
  // 2. demo .env
  const demoEnv = resolve(process.env.HOME || '', 'work/deepseek-ocr-demo/.env');
  if (existsSync(demoEnv)) {
    const content = readFileSync(demoEnv, 'utf-8');
    const m = content.match(/DEEPSEEK_OCR_API_KEY=(sk-\S+)/);
    if (m && m[1]) return m[1];
  }
  return null;
}
