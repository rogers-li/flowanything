#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${FLOW_ANYTHING_ENV_FILE:-$ROOT_DIR/configs/local/services.env}"
RUNTIME_DIR="${FLOW_ANYTHING_RUNTIME_DIR:-$ROOT_DIR/.runtime/local}"
LOG_DIR="${FLOW_ANYTHING_LOG_DIR:-$ROOT_DIR/log/local}"
BIN_DIR="${FLOW_ANYTHING_BIN_DIR:-$RUNTIME_DIR/bin}"
PID_FILE="$RUNTIME_DIR/services.pid"
GOCACHE="${GOCACHE:-/tmp/flow-anything-gocache}"
AUTO_SEED="${FLOW_ANYTHING_AUTO_SEED:-true}"

SERVICES=(
  "platform-api:platform-api"
  "connector-service:connector-service"
  "knowledge-service:knowledge-service"
  "agent-runtime:agent-runtime"
  "model-gateway:model-gateway"
  "ai-orchestrator:ai-orchestrator"
  "agent-flow-runtime:agent-flow-runtime"
  "mock-business-api:mock-business-api"
)

load_env_file() {
  if [[ ! -f "$ENV_FILE" ]]; then
    echo "env file not found: $ENV_FILE"
    return
  fi

  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [[ -z "$line" || "$line" == \#* ]] && continue
    [[ "$line" != *=* ]] && continue

    # Use export "KEY=VALUE" instead of source so values can safely contain
    # spaces, Chinese text, and punctuation without shell escaping.
    export "$line"
  done < "$ENV_FILE"
}

existing_pid_for() {
  local service_name="$1"
  [[ -f "$PID_FILE" ]] || return 1
  awk -v name="$service_name" '$1 == name {print $2}' "$PID_FILE" | tail -n 1
}

pid_is_running() {
  local pid="$1"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

remove_stale_pid() {
  local service_name="$1"
  [[ -f "$PID_FILE" ]] || return 0
  local tmp_file
  tmp_file="$(mktemp "$RUNTIME_DIR/services.pid.XXXXXX")"
  awk -v name="$service_name" '$1 != name {print}' "$PID_FILE" > "$tmp_file"
  mv "$tmp_file" "$PID_FILE"
}

start_service() {
  local service_name="$1"
  local command_dir="$2"
  local current_pid
  current_pid="$(existing_pid_for "$service_name" || true)"

  if pid_is_running "$current_pid"; then
    echo "skip $service_name, already running with pid $current_pid"
    return
  fi

  if [[ -n "$current_pid" ]]; then
    echo "remove stale pid for $service_name: $current_pid"
    remove_stale_pid "$service_name"
  fi

  local log_file="$LOG_DIR/$service_name.log"
  local binary_file="$BIN_DIR/$service_name"
  echo "build $service_name"
  (
    cd "$ROOT_DIR"
    env GOCACHE="$GOCACHE" go build -o "$binary_file" "./cmd/$command_dir"
  )

  echo "start $service_name, log: $log_file"
  (
    cd "$ROOT_DIR"
    exec nohup "$binary_file"
  ) >> "$log_file" 2>&1 &

  local pid="$!"
  echo "$service_name $pid" >> "$PID_FILE"

  sleep 0.3
  if ! pid_is_running "$pid"; then
    echo "failed to start $service_name, last log lines:"
    tail -n 20 "$log_file" || true
    remove_stale_pid "$service_name"
    return 1
  fi
}

wait_for_platform_api() {
  local platform_url="${PLATFORM_API_URL:-http://localhost:8080}"
  local attempts=30

  for _ in $(seq 1 "$attempts"); do
    if curl -fsS "$platform_url/v1/agents?tenant_id=tenant_1" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done

  echo "platform-api is not ready at $platform_url"
  return 1
}

seed_local_flows() {
  if [[ "$AUTO_SEED" != "true" ]]; then
    return
  fi

  echo
  echo "seed local weather flow"
  wait_for_platform_api
  bash "$ROOT_DIR/scripts/local/seed-weather-flow.sh"
}

main() {
  mkdir -p "$RUNTIME_DIR" "$LOG_DIR" "$BIN_DIR"
  touch "$PID_FILE"
  load_env_file

  for item in "${SERVICES[@]}"; do
    IFS=: read -r service_name command_dir <<< "$item"
    start_service "$service_name" "$command_dir"
  done

  seed_local_flows

  echo
  echo "services started. pid file: $PID_FILE"
  echo "logs:"
  for item in "${SERVICES[@]}"; do
    IFS=: read -r service_name _ <<< "$item"
    echo "  $service_name -> $LOG_DIR/$service_name.log"
  done
}

main "$@"
