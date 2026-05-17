#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNTIME_DIR="${FLOW_ANYTHING_RUNTIME_DIR:-$ROOT_DIR/.runtime/local}"
PID_FILE="$RUNTIME_DIR/services.pid"
TIMEOUT_SECONDS="${FLOW_ANYTHING_STOP_TIMEOUT_SECONDS:-10}"

pid_is_running() {
  local pid="$1"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

stop_pid() {
  local service_name="$1"
  local pid="$2"

  if ! pid_is_running "$pid"; then
    echo "skip $service_name, pid $pid is not running"
    return
  fi

  echo "stop $service_name, pid $pid"
  kill "$pid" 2>/dev/null || true

  local elapsed=0
  while pid_is_running "$pid" && [[ "$elapsed" -lt "$TIMEOUT_SECONDS" ]]; do
    sleep 1
    elapsed=$((elapsed + 1))
  done

  if pid_is_running "$pid"; then
    echo "force stop $service_name, pid $pid"
    kill -9 "$pid" 2>/dev/null || true
  fi
}

main() {
  if [[ ! -f "$PID_FILE" ]]; then
    echo "pid file not found: $PID_FILE"
    echo "nothing to stop"
    return
  fi

  # Stop in reverse order so upstream entrypoints shut down before dependencies.
  awk '{ lines[NR] = $0 } END { for (i = NR; i >= 1; i--) print lines[i] }' "$PID_FILE" | while read -r service_name pid; do
    [[ -z "${service_name:-}" || -z "${pid:-}" ]] && continue
    stop_pid "$service_name" "$pid"
  done

  : > "$PID_FILE"
  echo "services stopped"
}

main "$@"
