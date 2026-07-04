#!/usr/bin/env bash
# 停止牛牛教辅系统的前后端进程。
#
# 优先级：PID 文件(.run/pids) → 按端口(lsof)兜底。
# 用法：./stop.sh
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

BACKEND_PORT="${PORT:-8787}"
FRONTEND_PORT="${FRONTEND_PORT:-5173}"
PID_FILE="$ROOT/.run/pids"

log()  { printf "\033[36m[stop]\033[0m %s\n" "$*"; }
warn() { printf "\033[33m[stop]\033[0m %s\n" "$*" >&2; }

# 杀单个 PID：先 SIGTERM，等 2s 仍存活则 SIGKILL
kill_pid() {
  local pid="$1"
  [[ -z "$pid" ]] && return 0
  if ! kill -0 "$pid" 2>/dev/null; then
    return 0 # 已退出
  fi
  kill -TERM "$pid" 2>/dev/null || true
  for _ in 1 2 3 4; do
    sleep 0.5
    kill -0 "$pid" 2>/dev/null || return 0
  done
  warn "PID $pid 未响应 SIGTERM，发送 SIGKILL"
  kill -KILL "$pid" 2>/dev/null || true
}

# 按端口找监听进程并杀掉（PID 文件失效时的兜底）
kill_port() {
  local port="$1"
  local label="$2"
  local pids
  if ! command -v lsof >/dev/null 2>&1; then
    return 0
  fi
  pids=$(lsof -ti tcp:"$port" -sTCP:LISTEN 2>/dev/null || true)
  if [[ -z "$pids" ]]; then
    return 0
  fi
  local pid_list
  pid_list=$(printf '%s' "$pids" | tr '\n' ' ')
  warn "通过端口 ${port} 兜底停止 ${label}（PID: ${pid_list}）"
  for pid in $pids; do
    kill_pid "$pid"
  done
}

stopped_any=false

# 1. 按 PID 文件停止
if [[ -f "$PID_FILE" ]]; then
  while IFS=: read -r name pid; do
    [[ -z "$pid" ]] && continue
    if kill -0 "$pid" 2>/dev/null; then
      log "停止 $name (PID $pid)"
      kill_pid "$pid"
      stopped_any=true
    fi
  done <"$PID_FILE"
  rm -f "$PID_FILE"
fi

# 2. 端口兜底（即使 PID 文件不存在或失效）
kill_port "$BACKEND_PORT" "后端"
kill_port "$FRONTEND_PORT" "前端"

if [[ "$stopped_any" == "false" ]]; then
  # 再确认端口是否真的已释放
  if command -v lsof >/dev/null 2>&1; then
    if [[ -z "$(lsof -ti tcp:"$BACKEND_PORT" -sTCP:LISTEN 2>/dev/null || true)"" ]] \
      && [[ -z "$(lsof -ti tcp:"$FRONTEND_PORT" -sTCP:LISTEN 2>/dev/null || true)"" ]]; then
      log "未发现运行中的实例（端口已空闲）"
    fi
  else
    log "未发现 PID 记录，且无 lsof 可用于端口检查"
  fi
else
  log "已停止"
fi
