#!/usr/bin/env bash
# 学生端完整流程的单入口测试工具：停止 → 恢复/初始化 → 启动。
#
# 首次使用：./reset-today.sh --init   保存当前状态作为测试起点后启动
# 日常重测：./reset-today.sh          停止、恢复到测试起点后重新启动
# 更新起点：./reset-today.sh --init   覆盖原测试起点后启动
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="${NIUNIU_DATA_DIR:-$ROOT/data}"
DB="$DATA_DIR/niuniu.db"
BASELINE_DIR="${NIUNIU_TEST_BASELINE_DIR:-$ROOT/.run/test-baseline}"
BASELINE_DB="$BASELINE_DIR/niuniu.db"
AUDIO_DIR="$DATA_DIR/audio/pronunciations"
BASELINE_AUDIO="$BASELINE_DIR/pronunciations"
PID_FILE="$ROOT/.run/pids"
BACKEND_PORT="${PORT:-8787}"
FRONTEND_PORT="${FRONTEND_PORT:-5173}"

log() { printf '\033[36m[reset-today]\033[0m %s\n' "$*"; }
die() { printf '\033[31m[reset-today]\033[0m %s\n' "$*" >&2; exit 1; }

command -v sqlite3 >/dev/null 2>&1 || die "需要先安装 sqlite3。"
[[ -f "$DB" ]] || die "找不到数据库：$DB"

# 恢复数据库时必须停止服务，避免覆盖正在使用的数据库文件。
# stop.sh 执行后仍检测到存活 PID 时中止，防止覆盖被占用的数据库文件。
ensure_stopped() {
  if [[ -s "$PID_FILE" ]]; then
    while IFS=: read -r _ pid; do
      if [[ -n "${pid:-}" ]] && kill -0 "$pid" 2>/dev/null; then
        die "停止系统失败，进程 $pid 仍在运行；未执行恢复。"
      fi
    done <"$PID_FILE"
  fi

  if command -v lsof >/dev/null 2>&1; then
    local port pids
    for port in "$BACKEND_PORT" "$FRONTEND_PORT"; do
      pids=$(lsof -ti tcp:"$port" -sTCP:LISTEN 2>/dev/null || true)
      [[ -z "$pids" ]] || die "停止系统失败，端口 $port 仍被 PID ${pids//$'\n'/ } 监听；未执行恢复。"
    done
  fi
}

init_baseline() {
  mkdir -p "$BASELINE_DIR"
  rm -f "$BASELINE_DB"
  sqlite3 "$DB" ".backup '$BASELINE_DB'"

  rm -rf "$BASELINE_AUDIO"
  if [[ -d "$AUDIO_DIR" ]]; then
    mkdir -p "$BASELINE_AUDIO"
    cp -R "$AUDIO_DIR"/. "$BASELINE_AUDIO"/
  fi
  log "测试起点已保存。"
  log "以后直接执行 ./reset-today.sh，即可自动停止、恢复并重新启动。"
}

restore_baseline() {
  [[ -f "$BASELINE_DB" ]] || die "还没有测试起点。请先执行：./reset-today.sh --init"

  # 只恢复学生学习流程和发音缓存；不覆盖词库、系统配置和复习策略。
  local escaped_baseline="${BASELINE_DB//\'/\'\'}"
  sqlite3 "$DB" <<SQL
PRAGMA foreign_keys = ON;
ATTACH DATABASE '$escaped_baseline' AS baseline;
BEGIN IMMEDIATE;
DELETE FROM daily_tasks;
DELETE FROM reading_passages;
DELETE FROM checkins;
DELETE FROM pronunciation_audio;
DELETE FROM word_learning;
INSERT INTO word_learning SELECT * FROM baseline.word_learning;
INSERT INTO daily_tasks SELECT * FROM baseline.daily_tasks;
INSERT INTO reading_passages SELECT * FROM baseline.reading_passages;
INSERT INTO checkins SELECT * FROM baseline.checkins;
INSERT INTO pronunciation_audio SELECT * FROM baseline.pronunciation_audio;
COMMIT;
DETACH DATABASE baseline;
SQL

  # 发音文件与 pronunciation_audio 记录必须一起恢复，防止数据库指向不存在的文件。
  case "$AUDIO_DIR" in
    "$DATA_DIR"/audio/pronunciations) ;;
    *) die "音频目录安全检查失败：$AUDIO_DIR" ;;
  esac
  rm -rf "$AUDIO_DIR"
  if [[ -d "$BASELINE_AUDIO" ]]; then
    mkdir -p "$AUDIO_DIR"
    cp -R "$BASELINE_AUDIO"/. "$AUDIO_DIR"/
  fi

  log "已恢复到测试起点。"
  log "现在将自动重新启动系统。"
}

case "${1:-}" in
  --init)
    log "停止当前系统..."
    "$ROOT/stop.sh"
    ensure_stopped
    init_baseline
    exec "$ROOT/start.sh"
    ;;
  "")
    [[ -f "$BASELINE_DB" ]] || die "还没有测试起点。请先执行：./reset-today.sh --init"
    log "停止当前系统..."
    "$ROOT/stop.sh"
    ensure_stopped
    restore_baseline
    exec "$ROOT/start.sh"
    ;;
  *) die "未知参数：$1。用法：./reset-today.sh [--init]" ;;
esac
