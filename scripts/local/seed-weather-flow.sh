#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${FLOW_ANYTHING_ENV_FILE:-$ROOT_DIR/configs/local/services.env}"
PLATFORM_API_URL="${PLATFORM_API_URL:-http://localhost:8080}"

load_env_file() {
  if [[ ! -f "$ENV_FILE" ]]; then
    return
  fi

  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [[ -z "$line" || "$line" == \#* ]] && continue
    [[ "$line" != *=* ]] && continue
    export "$line"
  done < "$ENV_FILE"
}

post_json() {
  local path="$1"
  local payload="$2"
  curl -fsS -X POST "$PLATFORM_API_URL$path" \
    -H 'Content-Type: application/json' \
    -d "$payload" >/dev/null
}

resource_exists() {
  local path="$1"
  curl -fsS "$PLATFORM_API_URL$path" >/dev/null 2>&1
}

post_json_if_missing() {
  local label="$1"
  local get_path="$2"
  local post_path="$3"
  local payload="$4"

  if resource_exists "$get_path"; then
    echo "skip $label, already exists"
    return
  fi

  echo "seed $label"
  post_json "$post_path" "$payload"
}

weather_agent_provider_id() {
  local provider="${WEATHER_AGENT_PROVIDER:-${MODEL_GATEWAY_PROVIDER:-mock}}"
  case "$provider" in
    deepseek)
      echo "provider_deepseek"
      ;;
    openai-compatible)
      echo "provider_openai_compatible"
      ;;
    mock|"")
      echo "provider_mock"
      ;;
    provider_*)
      echo "$provider"
      ;;
    *)
      echo "provider_$provider"
      ;;
  esac
}

weather_agent_model() {
  local provider="${WEATHER_AGENT_PROVIDER:-${MODEL_GATEWAY_PROVIDER:-mock}}"
  case "$provider" in
    deepseek)
      echo "${WEATHER_AGENT_MODEL:-${DEEPSEEK_MODEL:-deepseek-v4-flash}}"
      ;;
    openai-compatible)
      echo "${WEATHER_AGENT_MODEL:-${OPENAI_COMPATIBLE_MODEL:-configured-by-env}}"
      ;;
    *)
      echo "${WEATHER_AGENT_MODEL:-${MODEL_GATEWAY_MOCK_MODEL:-mock-chat}}"
      ;;
  esac
}

main() {
  load_env_file
  local weather_agent_provider_id
  local weather_agent_model
  local weather_agent_payload
  weather_agent_provider_id="$(weather_agent_provider_id)"
  weather_agent_model="$(weather_agent_model)"
  weather_agent_payload='{"id":"agent_weather","tenant_id":"tenant_1","name":"Weather Agent","description":"面向用户的天气助手","business_domain":"Weather","owner_team":"AI Platform","status":"enabled","skill_ids":["skill_weather"],"default_lang":"zh-CN","supported_languages":["zh-CN","en-US"],"channels":["text","voice"],"system_prompt":"你是简洁的天气助手。用户询问天气时，调用 query_weather 并基于工具结果回答。","welcome_message":"您好，我可以帮您查询城市实时天气。","model_config":{"provider_id":"'"$weather_agent_provider_id"'","model":"'"$weather_agent_model"'","temperature":0.1},"runtime_policy":{"max_turns":6,"max_tool_calls":2,"response_timeout_ms":15000}}'

  post_json_if_missing \
    "weather connector operation" \
    "/v1/connector-operations/connop_query_weather?tenant_id=tenant_1" \
    "/v1/connector-operations" \
    '{"id":"connop_query_weather","tenant_id":"tenant_1","name":"query_weather","description":"查询城市实时天气","business_domain":"Weather","owner_team":"AI Platform","type":"http","status":"enabled","implementation_mode":"simple_http","base_url":"http://localhost:8090","method":"GET","path":"/weather/current","auth":{"type":"none"},"input_schema":{"type":"object","properties":{"city":{"type":"string","description":"城市名称"},"unit":{"type":"string","description":"单位，默认 metric"}},"required":["city"]},"output_schema":{"type":"object","properties":{"city":{"type":"string"},"condition":{"type":"string"},"temperature_c":{"type":"number"},"humidity":{"type":"integer"},"wind_kph":{"type":"number"}}},"timeout_ms":8000}'

  post_json_if_missing \
    "weather tool" \
    "/v1/tools/tool_query_weather?tenant_id=tenant_1" \
    "/v1/tools" \
    '{"id":"tool_query_weather","tenant_id":"tenant_1","name":"query_weather","description":"查询城市实时天气","business_domain":"Weather","owner_team":"AI Platform","status":"enabled","llm_description":"Use this tool when the user asks for current weather in a city.","implementation":"connector","binding":{"connector_operation_id":"connop_query_weather"},"input_schema":{"type":"object","properties":{"city":{"type":"string","description":"城市名称，例如深圳或Shanghai"},"unit":{"type":"string","description":"单位，默认 metric"}},"required":["city"]},"output_schema":{"type":"object","properties":{"city":{"type":"string"},"condition":{"type":"string"},"temperature_c":{"type":"number"},"humidity":{"type":"integer"},"wind_kph":{"type":"number"}}},"side_effect":"read","risk_level":"low","requires_confirmation":false,"timeout_ms":8000}'

  post_json_if_missing \
    "weather skill" \
    "/v1/skills/skill_weather?tenant_id=tenant_1" \
    "/v1/skills" \
    '{"id":"skill_weather","tenant_id":"tenant_1","name":"weather_service","description":"天气查询能力","business_domain":"Weather","owner_team":"AI Platform","status":"enabled","tool_ids":["tool_query_weather"],"system_prompt":"当用户询问天气时，调用 query_weather 查询城市实时天气。如果城市缺失，先追问用户。","risk_level":"low"}'

  post_json_if_missing \
    "weather agent" \
    "/v1/agents/agent_weather?tenant_id=tenant_1" \
    "/v1/agents" \
    "$weather_agent_payload"

  echo "weather flow seeded. Open Agent Debug and send: 帮我查一下深圳天气"
}

main "$@"
