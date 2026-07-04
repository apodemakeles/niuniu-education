#!/usr/bin/env bash
# 启动牛牛教辅系统：后端 + 前端(vite dev)，并在就绪后打开浏览器。
#
# 用法：
#   ./start.sh           # 默认 8787 / 5173
#   PORT=9000 ./start.sh # 自定义后端端口
#
# 停止：./stop.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

# ---- 配置（可由环境变量覆盖）----
BACKEND_PORT="${PORT:-8787}"
FRONTEND_PORT="${FRONTEND_PORT:-5173}"
export NIUNIU_PORT="$BACKEND_PORT"          # 供后端读取（config 默认值也是 8787）
RUN_DIR="$ROOT/.run"
LOG_DIR="$RUN_DIR/logs"
PID_FILE="$RUN_DIR/pids"

# ---- 工具 ----
log() { printf "\033[36m[start]\033[0m %s\n" "$*"; }
warn() { printf "\033[33m[start]\033[0m %s\n" "$*" >&2; }
die() { printf "\033[31m[start]\033[0m %s\n" "$*" >&2; exit 1; }

# 若已存在运行记录，提示先停止
if [[ -f "$PID_FILE" ]] && [[ -s "$PID_FILE" ]]; then
  die "检测到已有运行实例（$PID_FILE）。请先执行 ./stop.sh，或删除该文件后再启动。"
fi

mkdir -p "$LOG_DIR"
: >"$PID_FILE"

# ---- 1. 构建后端二进制（若缺失）----
BIN="$ROOT/bin/niuniu-api"
if [[ ! -x "$BIN" ]] || [[ "$BIN" -ot "$ROOT/backend/cmd/niuniu/main.go" ]] \
  || [[ "$BIN" -ot "$(ls -t "$ROOT"/backend/internal/**/*.go 2>/dev/null | head -1 || echo /dev/null)" ]]; then
  log "构建后端..."
  ( cd backend && CGO_ENABLED=0 go build -o "$BIN" ./cmd/niuniu )
fi

# ---- 2. 安装前端依赖（若缺失）----
if [[ ! -d "$ROOT/frontend/node_modules" ]]; then
  log "安装前端依赖..."
  ( cd frontend && npm install )
fi

# ---- 3. 启动后端 ----
log "启动后端 :$BACKEND_PORT"
"$BIN" -port "$BACKEND_PORT" >"$LOG_DIR/backend.log" 2>&1 &
echo "backend:$!" >>"$PID_FILE"

# ---- 4. 启动前端（vite dev，proxy 已将 /api 转发到后端）----
log "启动前端 :$FRONTEND_PORT"
( cd frontend && npm run dev -- --port "$FRONTEND_PORT" --strictPort ) >"$LOG_DIR/frontend.log" 2>&1 &
echo "frontend:$!" >>"$PID_FILE"

# ---- 5. 等待就绪 ----
wait_url() {
  local url="$1" name="$2" timeout="${3:-30}"
  local elapsed=0
  while (( elapsed < timeout )); do
    if curl -s --noproxy '*' -o /dev/null "$url" 2>/dev/null; then
      return 0
    fi
    sleep 0.5; elapsed=$((elapsed + 1))
  done
  return 1
}

log "等待后端就绪..."
if ! wait_url "http://127.0.0.1:$BACKEND_PORT/api/v1/health" "后端" 30; then
  warn "后端未在 30s 内就绪，查看日志：$LOG_DIR/backend.log"
fi

log "等待前端就绪..."
if ! wait_url "http://localhost:$FRONTEND_PORT/" "前端" 30; then
  warn "前端未在 30s 内就绪，查看日志：$LOG_DIR/frontend.log"
fi

# ---- 6. 打开浏览器 ----
URL="http://localhost:$FRONTEND_PORT/"
log "服务已就绪，打开浏览器：$URL"
if command -v open >/dev/null 2>&1; then
  ( open "$URL" & ) >/dev/null 2>&1
elif command -v xdg-open >/dev/null 2>&1; then
  ( xdg-open "$URL" & ) >/dev/null 2>&1
fi

log "完成。日志在 $LOG_DIR/，停止请执行 ./stop.sh"
