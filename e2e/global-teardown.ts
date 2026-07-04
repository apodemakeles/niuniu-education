import { readFileSync, rmSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';
import { ROOT } from './ports';

const STATE_FILE = resolve(ROOT, 'e2e/.run-state.json');

function killPid(pid: number | undefined) {
  if (!pid) return;
  try {
    process.kill(pid, 'SIGTERM');
  } catch {
    // 进程可能已退出
  }
}

export default async function globalTeardown() {
  console.log('[e2e] globalTeardown 开始');

  if (existsSync(STATE_FILE)) {
    const state = JSON.parse(readFileSync(STATE_FILE, 'utf-8'));
    killPid(state.backendPid);
    killPid(state.frontendPid);
    killPid(state.realBackendPid);
    // 给一点时间优雅退出
    await new Promise((r) => setTimeout(r, 500));
    // 兜底：SIGTERM 没杀掉的强杀
    try {
      if (state.backendPid) process.kill(state.backendPid, 'SIGKILL');
    } catch {
      /* noop */
    }
    try {
      if (state.frontendPid) process.kill(state.frontendPid, 'SIGKILL');
    } catch {
      /* noop */
    }
    try {
      if (state.realBackendPid) process.kill(state.realBackendPid, 'SIGKILL');
    } catch {
      /* noop */
    }
    rmSync(STATE_FILE, { force: true });
  }

  // 清理测试数据目录
  rmSync(resolve(ROOT, 'e2e/.data'), { recursive: true, force: true });
  console.log('[e2e] globalTeardown 完成');
}
