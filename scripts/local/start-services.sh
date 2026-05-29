#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${FLOW_ANYTHING_ENV_FILE:-$ROOT_DIR/configs/local/services.env}"
ENV_EXAMPLE_FILE="${FLOW_ANYTHING_ENV_EXAMPLE_FILE:-$ROOT_DIR/configs/local/services.env.example}"
LOCAL_ENV_FILE="${FLOW_ANYTHING_LOCAL_ENV_FILE:-$ROOT_DIR/configs/local/services.local.env}"
RUNTIME_DIR="${FLOW_ANYTHING_RUNTIME_DIR:-$ROOT_DIR/.runtime/local}"
LOG_DIR="${FLOW_ANYTHING_LOG_DIR:-$ROOT_DIR/log/local}"
BIN_DIR="${FLOW_ANYTHING_BIN_DIR:-$RUNTIME_DIR/bin}"
PID_FILE="$RUNTIME_DIR/services.pid"
FRONTEND_PID_FILE="$RUNTIME_DIR/admin-console.pid"
FRONTEND_LOG_FILE="$LOG_DIR/admin-console.log"
FRONTEND_ENABLED="${FLOW_ANYTHING_START_FRONTEND:-true}"
FRONTEND_URL="${FLOW_ANYTHING_FRONTEND_URL:-http://localhost:5173}"
GOCACHE="${GOCACHE:-/tmp/flow-anything-gocache}"
SERVICES=()

ensure_env_file() {
  if [[ -f "$ENV_FILE" ]]; then
    return
  fi
  if [[ ! -f "$ENV_EXAMPLE_FILE" ]]; then
    echo "env file not found: $ENV_FILE"
    echo "env example not found: $ENV_EXAMPLE_FILE"
    return 1
  fi
  mkdir -p "$(dirname "$ENV_FILE")"
  cp "$ENV_EXAMPLE_FILE" "$ENV_FILE"
  echo "created local env from example: $ENV_FILE"
}

load_env_file() {
  local file_path="$1"
  local required="${2:-false}"

  if [[ ! -f "$file_path" ]]; then
    if [[ "$required" == "true" ]]; then
      echo "env file not found: $file_path"
    fi
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
  done < "$file_path"
}

resolve_workspace_path() {
  local file_path="$1"
  if [[ "$file_path" == /* ]]; then
    echo "$file_path"
    return
  fi
  echo "$ROOT_DIR/${file_path#./}"
}

ensure_bundle_file() {
  local target_path="$1"
  local lifecycle="$2"
  local example_path="$ROOT_DIR/configs/examples/starter.draft.bundle.json"
  [[ -n "$target_path" ]] || return

  local resolved_target
  resolved_target="$(resolve_workspace_path "$target_path")"
  if [[ -f "$resolved_target" ]]; then
    return
  fi
  if [[ ! -f "$example_path" ]]; then
    echo "starter bundle example not found: $example_path"
    return 1
  fi

  mkdir -p "$(dirname "$resolved_target")"
  sed "s/\"lifecycle\": \"draft\"/\"lifecycle\": \"$lifecycle\"/" "$example_path" > "$resolved_target"
  echo "created local $lifecycle bundle: $resolved_target"
}

ensure_local_bundle_files() {
  ensure_bundle_file "${FLOW_ANYTHING_DRAFT_BUNDLE_PATH:-}" "draft"
  ensure_bundle_file "${FLOW_ANYTHING_PREVIEW_BUNDLE_PATH:-}" "preview"
  ensure_bundle_file "${FLOW_ANYTHING_RELEASE_BUNDLE_PATH:-}" "release"
}

configure_services() {
  SERVICES=("ai-platform-runtime:ai-platform-runtime")
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

wait_for_ai_platform_runtime() {
  local runtime_url="${FLOW_ANYTHING_RUNTIME_URL:-http://localhost:8081}"
  local attempts=30

  for _ in $(seq 1 "$attempts"); do
    if curl -fsS "$runtime_url/healthz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done

  echo "ai-platform-runtime is not ready at $runtime_url"
  return 1
}

start_frontend() {
  if [[ "$FRONTEND_ENABLED" != "true" ]]; then
    echo "skip admin-console frontend start, FLOW_ANYTHING_START_FRONTEND=$FRONTEND_ENABLED"
    return
  fi

  if [[ ! -d "$ROOT_DIR/web/admin-console/node_modules" ]]; then
    echo "admin-console dependencies are missing. Run: npm --prefix web/admin-console install"
    return 1
  fi

  local current_pid=""
  if [[ -f "$FRONTEND_PID_FILE" ]]; then
    current_pid="$(cat "$FRONTEND_PID_FILE" || true)"
  fi

  if pid_is_running "$current_pid"; then
    echo "skip admin-console frontend, already running with pid $current_pid"
    return
  fi

  if curl -fsS "$FRONTEND_URL" >/dev/null 2>&1; then
    echo "skip admin-console frontend, already reachable at $FRONTEND_URL"
    return
  fi

  : > "$FRONTEND_PID_FILE"

  echo "start admin-console frontend, log: $FRONTEND_LOG_FILE"
  (
    cd "$ROOT_DIR"
    exec nohup npm --prefix web/admin-console run dev
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

wait_for_frontend() {
  if [[ "$FRONTEND_ENABLED" != "true" ]]; then
    return
  fi

  local attempts=30
  for _ in $(seq 1 "$attempts"); do
    if curl -fsS "$FRONTEND_URL" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done

  echo "admin-console frontend is not ready at $FRONTEND_URL"
  return 1
}

main() {
  mkdir -p "$RUNTIME_DIR" "$LOG_DIR" "$BIN_DIR"
  touch "$PID_FILE"
  touch "$FRONTEND_PID_FILE"
  ensure_env_file
  load_env_file "$ENV_FILE" true
  load_env_file "$LOCAL_ENV_FILE" false
  ensure_local_bundle_files
  configure_services

  for item in "${SERVICES[@]}"; do
    IFS=: read -r service_name command_dir <<< "$item"
    start_service "$service_name" "$command_dir"
  done

  wait_for_ai_platform_runtime
  start_frontend
  wait_for_frontend

  echo
  echo "new runtime services started. pid file: $PID_FILE"
  echo "logs:"
  for item in "${SERVICES[@]}"; do
    IFS=: read -r service_name _ <<< "$item"
    echo "  $service_name -> $LOG_DIR/$service_name.log"
  done
  if [[ "$FRONTEND_ENABLED" == "true" ]]; then
    echo "  admin-console -> $FRONTEND_LOG_FILE"
    echo "frontend: $FRONTEND_URL"
  fi
}

main "$@"
