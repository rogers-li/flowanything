#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNTIME_DIR="${FLOW_ANYTHING_RUNTIME_DIR:-$ROOT_DIR/.runtime/local}"
LOG_DIR="${FLOW_ANYTHING_LOG_DIR:-$ROOT_DIR/log/local}"
FRONTEND_PID_FILE="$RUNTIME_DIR/admin-console.pid"
FRONTEND_LOG_FILE="$LOG_DIR/admin-console.log"
FRONTEND_ENABLED="${FLOW_ANYTHING_RESTART_FRONTEND:-true}"

pid_is_running() {
  local pid="$1"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

stop_frontend() {
  if [[ "$FRONTEND_ENABLED" != "true" ]]; then
    echo "skip admin-console frontend stop, FLOW_ANYTHING_RESTART_FRONTEND=$FRONTEND_ENABLED"
    return
  fi
  if [[ ! -f "$FRONTEND_PID_FILE" ]]; then
    echo "skip admin-console frontend stop, pid file not found"
    return
  fi

  local pid
  pid="$(cat "$FRONTEND_PID_FILE")"
  if ! pid_is_running "$pid"; then
    echo "skip admin-console frontend stop, pid $pid is not running"
    : > "$FRONTEND_PID_FILE"
    return
  fi

  echo "stop admin-console frontend, pid $pid"
  kill "$pid" 2>/dev/null || true
  for _ in $(seq 1 10); do
    if ! pid_is_running "$pid"; then
      : > "$FRONTEND_PID_FILE"
      return
    fi
    sleep 1
  done

  echo "force stop admin-console frontend, pid $pid"
  kill -9 "$pid" 2>/dev/null || true
  : > "$FRONTEND_PID_FILE"
}

start_frontend() {
  if [[ "$FRONTEND_ENABLED" != "true" ]]; then
    echo "skip admin-console frontend start, FLOW_ANYTHING_RESTART_FRONTEND=$FRONTEND_ENABLED"
    return
  fi

  if [[ ! -d "$ROOT_DIR/web/admin-console/node_modules" ]]; then
    echo "admin-console dependencies are missing. Run: npm --prefix web/admin-console install"
    return 1
  fi

  echo "start admin-console frontend, log: $FRONTEND_LOG_FILE"
  (
    cd "$ROOT_DIR"
    exec npm --prefix web/admin-console run dev
  ) >> "$FRONTEND_LOG_FILE" 2>&1 &

  local pid="$!"
  echo "$pid" > "$FRONTEND_PID_FILE"

  sleep 1
  if ! pid_is_running "$pid"; then
    echo "failed to start admin-console frontend, last log lines:"
    tail -n 20 "$FRONTEND_LOG_FILE" || true
    : > "$FRONTEND_PID_FILE"
    return 1
  fi
}

main() {
  mkdir -p "$RUNTIME_DIR" "$LOG_DIR"

  echo "stop frontend service"
  stop_frontend

  echo
  echo "restart backend services"
  bash "$ROOT_DIR/scripts/local/stop-services.sh"
  bash "$ROOT_DIR/scripts/local/start-services.sh"

  echo
  echo "start frontend service"
  start_frontend

  echo
  echo "all services restarted"
  echo "backend logs: $LOG_DIR/*.log"
  echo "frontend log: $FRONTEND_LOG_FILE"
}

main "$@"
