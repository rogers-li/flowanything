#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${FLOW_ANYTHING_ENV_FILE:-$ROOT_DIR/configs/local/services.env}"
PLATFORM_API_URL="${PLATFORM_API_URL:-http://localhost:8080}"
TENANT_ID="${FLOW_ANYTHING_TENANT_ID:-tenant_1}"

load_env_file() {
  if [[ ! -f "$ENV_FILE" ]]; then
    return
  fi

  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [[ -z "$line" || "$line" == \#* ]] && continue
    [[ "$line" != *=* ]] && continue
    local key="${line%%=*}"
    if printenv "$key" >/dev/null 2>&1; then
      continue
    fi
    export "$line"
  done < "$ENV_FILE"
}

json_request() {
  local method="$1"
  local path="$2"
  local payload="$3"

  curl -fsS -X "$method" "$PLATFORM_API_URL$path" \
    -H 'Content-Type: application/json' \
    -d "$payload" >/dev/null
}

resource_exists() {
  local path="$1"
  curl -fsS "$PLATFORM_API_URL$path" >/dev/null 2>&1
}

upsert_json() {
  local label="$1"
  local get_path="$2"
  local post_path="$3"
  local put_path="$4"
  local payload="$5"

  if resource_exists "$get_path"; then
    echo "update $label"
    json_request PUT "$put_path" "$payload"
    return
  fi

  echo "create $label"
  json_request POST "$post_path" "$payload"
}

main() {
  load_env_file

  local connector_payload
  connector_payload=$(cat <<'JSON'
{
  "id": "conn_juhe_news",
  "tenant_id": "tenant_1",
  "name": "新闻资讯",
  "description": "聚合数据新闻头条 connector，提供新闻列表和新闻详情查询能力。",
  "business_domain": "News",
  "owner_team": "AI Platform",
  "type": "http",
  "status": "enabled",
  "base_url": "https://v.juhe.cn",
  "headers": {
    "Accept": "application/json"
  },
  "auth": {
    "type": "api_key",
    "header_name": "query:key",
    "secret_ref": "env:JUHE_NEWS_API_KEY"
  },
  "timeout_ms": 15000,
  "version": "v1"
}
JSON
)
  connector_payload="${connector_payload//\"tenant_1\"/\"$TENANT_ID\"}"

  local list_operation_payload
  list_operation_payload=$(cat <<'JSON'
{
  "id": "connop_juhe_news_list",
  "tenant_id": "tenant_1",
  "connector_id": "conn_juhe_news",
  "name": "juhe_news_list",
  "description": "查询聚合数据新闻头条列表，可按新闻分类分页获取最新资讯。",
  "type": "http",
  "status": "enabled",
  "implementation_mode": "simple_http",
  "method": "GET",
  "path": "/toutiao/index",
  "input_schema": {
    "type": "object",
    "properties": {
      "type": {
        "type": "string",
        "enum": ["top", "guonei", "guoji", "yule", "tiyu", "junshi", "keji", "caijing", "youxi", "qiche", "jiankang"],
        "description": "新闻分类。top=推荐/头条，guonei=国内，guoji=国际，yule=娱乐，tiyu=体育，junshi=军事，keji=科技，caijing=财经，youxi=游戏，qiche=汽车，jiankang=健康。默认 top。"
      },
      "page": {
        "type": "integer",
        "minimum": 1,
        "maximum": 50,
        "description": "当前页数，默认 1，最大 50。"
      },
      "page_size": {
        "type": "integer",
        "minimum": 1,
        "maximum": 30,
        "description": "每页返回条数，默认 30，最大 30。"
      },
      "is_filter": {
        "type": "integer",
        "enum": [0, 1],
        "description": "是否只返回有正文详情的新闻。1=是，0=否，默认 0。"
      }
    }
  },
  "output_schema": {
    "type": "object",
    "properties": {
      "reason": { "type": "string", "description": "接口返回说明。" },
      "error_code": { "type": "integer", "description": "聚合数据错误码，0 表示成功。" },
      "result": {
        "type": "object",
        "properties": {
          "stat": { "type": "string", "description": "接口状态标识。" },
          "page": { "type": "string", "description": "当前页。" },
          "pageSize": { "type": "string", "description": "每页数量。" },
          "data": {
            "type": ["array", "null"],
            "description": "新闻列表，无数据时为 null。",
            "items": {
              "type": "object",
              "properties": {
                "uniquekey": { "type": "string", "description": "新闻 ID，可用于查询新闻详情。" },
                "title": { "type": "string", "description": "新闻标题。" },
                "date": { "type": "string", "description": "新闻时间，格式通常为 YYYY-MM-DD HH:mm:ss。" },
                "category": { "type": "string", "description": "新闻分类。" },
                "author_name": { "type": "string", "description": "新闻来源或作者。" },
                "url": { "type": "string", "description": "新闻原文访问链接。" },
                "thumbnail_pic_s": { "type": "string", "description": "新闻图片链接。" },
                "thumbnail_pic_s02": { "type": "string", "description": "新闻图片链接 2。" },
                "thumbnail_pic_s03": { "type": "string", "description": "新闻图片链接 3。" },
                "is_content": { "type": "string", "description": "是否有详情正文，1 表示可调用 juhe_news_detail 获取正文。" }
              }
            }
          }
        }
      },
      "status_code": { "type": "integer", "description": "Connector runtime 捕获的 HTTP 状态码。" }
    }
  },
  "timeout_ms": 15000,
  "version": "v1"
}
JSON
)
  list_operation_payload="${list_operation_payload//\"tenant_1\"/\"$TENANT_ID\"}"

  local detail_operation_payload
  detail_operation_payload=$(cat <<'JSON'
{
  "id": "connop_juhe_news_detail",
  "tenant_id": "tenant_1",
  "connector_id": "conn_juhe_news",
  "name": "juhe_news_detail",
  "description": "根据新闻 uniquekey 查询聚合数据新闻详情正文。",
  "type": "http",
  "status": "enabled",
  "implementation_mode": "simple_http",
  "method": "GET",
  "path": "/toutiao/content",
  "input_schema": {
    "type": "object",
    "required": ["uniquekey"],
    "properties": {
      "uniquekey": {
        "type": "string",
        "description": "新闻 ID，来源于 juhe_news_list 返回的 result.data[].uniquekey。"
      }
    }
  },
  "output_schema": {
    "type": "object",
    "properties": {
      "reason": { "type": "string", "description": "接口返回说明。" },
      "error_code": { "type": "integer", "description": "聚合数据错误码，0 表示成功。" },
      "result": {
        "type": "object",
        "properties": {
          "uniquekey": { "type": "string", "description": "新闻 ID。" },
          "content": { "type": "string", "description": "新闻正文，通常为 HTML 片段，回答用户前需要提炼为可读文本。" },
          "detail": {
            "type": "object",
            "description": "新闻基础信息。",
            "properties": {
              "title": { "type": "string", "description": "新闻标题。" },
              "date": { "type": "string", "description": "新闻时间。" },
              "category": { "type": "string", "description": "新闻分类。" },
              "author_name": { "type": "string", "description": "新闻来源或作者。" },
              "url": { "type": "string", "description": "新闻原文访问链接。" },
              "thumbnail_pic_s": { "type": "string", "description": "新闻图片链接。" },
              "thumbnail_pic_s02": { "type": "string", "description": "新闻图片链接 2。" },
              "thumbnail_pic_s03": { "type": "string", "description": "新闻图片链接 3。" }
            }
          }
        }
      },
      "status_code": { "type": "integer", "description": "Connector runtime 捕获的 HTTP 状态码。" }
    }
  },
  "timeout_ms": 15000,
  "version": "v1"
}
JSON
)
  detail_operation_payload="${detail_operation_payload//\"tenant_1\"/\"$TENANT_ID\"}"

  upsert_json \
    "Juhe news connector" \
    "/v1/connectors/conn_juhe_news?tenant_id=$TENANT_ID" \
    "/v1/connectors" \
    "/v1/connectors/conn_juhe_news?tenant_id=$TENANT_ID" \
    "$connector_payload"

  upsert_json \
    "Juhe news list operation" \
    "/v1/connector-operations/connop_juhe_news_list?tenant_id=$TENANT_ID" \
    "/v1/connector-operations" \
    "/v1/connector-operations/connop_juhe_news_list?tenant_id=$TENANT_ID" \
    "$list_operation_payload"

  upsert_json \
    "Juhe news detail operation" \
    "/v1/connector-operations/connop_juhe_news_detail?tenant_id=$TENANT_ID" \
    "/v1/connector-operations" \
    "/v1/connector-operations/connop_juhe_news_detail?tenant_id=$TENANT_ID" \
    "$detail_operation_payload"

  echo "juhe news connector seeded."
  echo "secret_ref: env:JUHE_NEWS_API_KEY"
  echo "operations: connop_juhe_news_list connop_juhe_news_detail"
}

main "$@"
