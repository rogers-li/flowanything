#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNTIME_DIR="${FLOW_ANYTHING_RUNTIME_DIR:-$ROOT_DIR/.runtime/local}"
LOG_DIR="${FLOW_ANYTHING_LOG_DIR:-$ROOT_DIR/log/local}"
FRONTEND_ENABLED="${FLOW_ANYTHING_RESTART_FRONTEND:-true}"

main() {
  mkdir -p "$RUNTIME_DIR" "$LOG_DIR"

  echo "restart local services"
  FLOW_ANYTHING_STOP_FRONTEND="$FRONTEND_ENABLED" bash "$ROOT_DIR/scripts/local/stop-services.sh"
  FLOW_ANYTHING_START_FRONTEND="$FRONTEND_ENABLED" bash "$ROOT_DIR/scripts/local/start-services.sh"

  echo
  echo "all services restarted"
  echo "logs: $LOG_DIR/*.log"
}

main "$@"
