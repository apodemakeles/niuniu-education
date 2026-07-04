import { execSync, spawn, ChildProcess } from 'node:child_process';
import { existsSync, mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { BACKEND_PORT, E2E_DATA_DIR, FRONTEND_PORT, ROOT } from './ports';

// 存放子进程引用，供 teardown 使用
const STATE_FILE = resolve(ROOT, 'e2e/.run-state.json');

let backendProc: ChildProcess | null = null;
let frontendProc: ChildProcess | null = null;

function ensureBackendBuilt() {
  const bin = resolve(ROOT, 'bin/niuniu-api');
  if (existsSync(bin)) return;
  console.log('[e2e] 后端二进制不存在，执行 go build...');
  execSync('CGO_ENABLED=0 go build -o ../bin/niuniu-api ./cmd/niuniu', {
    cwd: resolve(ROOT, 'backend'),
    stdio: 'inherit',
  });
}

function ensureFrontendBuilt(backendPort: number) {
  // e2e 用独立的 dist（.e2e-dist），API base 指向 e2e 后端端口。
  // 前端生产构建会把 VITE_API_BASE 固化进产物，故不能复用开发用的 dist。
  const e2eDist = resolve(ROOT, 'frontend/.e2e-dist');
  if (existsSync(e2eDist)) return;
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
  console.log('[e2e] 前后端均已就绪');

  // 持久化 PID 给 teardown（进程对象本身无法跨文件传递）
  writeFileSync(
    STATE_FILE,
    JSON.stringify({ backendPid: backendProc.pid, frontendPid: frontendProc.pid }),
  );
}
