// e2e 测试用的端口与路径常量。
// 故意避开开发端口(8787/5173)，防止 e2e 与开发服务互相干扰。
export const BACKEND_PORT = 18787; // mock provider 后端（现有 e2e 用）
export const REAL_BACKEND_PORT = 18788; // 真实 provider 后端（@real 测试用）
export const FRONTEND_PORT = 14173;
export const BASE_URL = `http://localhost:${FRONTEND_PORT}`;

// 后端 API 地址（直连后端端口；前端生产构建用绝对地址，不经 preview proxy）
export const API_BASE = `http://localhost:${BACKEND_PORT}/api/v1`;
export const REAL_API_BASE = `http://localhost:${REAL_BACKEND_PORT}/api/v1`;

// 测试用独立数据目录（避免污染开发数据）
export const E2E_DATA_DIR = './e2e/.data';

// 项目根目录（e2e/ 的上一级）
export const ROOT = new URL('../', import.meta.url).pathname;
