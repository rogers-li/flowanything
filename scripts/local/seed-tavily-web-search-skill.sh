#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DB_PATH="${FLOW_ANYTHING_DB_PATH:-$ROOT_DIR/flow-anything.db}"
TENANT_ID="${FLOW_ANYTHING_TENANT_ID:-tenant_1}"

tool_id_for_operation() {
  local operation_id="$1"
  sqlite3 "$DB_PATH" \
    "SELECT id FROM tools
     WHERE tenant_id = '$TENANT_ID'
       AND implementation = 'connector'
       AND binding_json LIKE '%\"connector_operation_id\":\"$operation_id\"%'
     ORDER BY CASE status WHEN 'enabled' THEN 0 ELSE 1 END, id
     LIMIT 1;"
}

require_tool() {
  local label="$1"
  local operation_id="$2"
  local tool_id
  tool_id="$(tool_id_for_operation "$operation_id")"
  if [[ -z "$tool_id" ]]; then
    echo "missing $label tool for operation $operation_id. Import Tavily connector tools first." >&2
    exit 1
  fi
  echo "$tool_id"
}

main() {
  if [[ ! -f "$DB_PATH" ]]; then
    echo "database not found: $DB_PATH" >&2
    exit 1
  fi

  local search_tool_id
  local extract_tool_id
  local crawl_tool_id
  local map_tool_id
  search_tool_id="$(require_tool tavily_search connop_tavily_search)"
  extract_tool_id="$(require_tool tavily_extract connop_tavily_extract)"
  crawl_tool_id="$(require_tool tavily_crawl connop_tavily_crawl)"
  map_tool_id="$(require_tool tavily_map connop_tavily_map)"

  sqlite3 "$DB_PATH" <<SQL
INSERT INTO skills (
  tenant_id, id, name, description, business_domain, owner_team, status,
  tool_ids_json, knowledge_ids_json, system_prompt, use_cases_json, exclusions_json,
  output_format, risk_level, execution_policy_json, policy_version, version
) VALUES (
  '$TENANT_ID',
  'skill_web_search',
  'web_search',
  '面向 Agent 的互联网搜索、网页阅读、站点发现与轻量网页采集能力。',
  'Search',
  'AI Platform',
  'enabled',
  json_array('$search_tool_id', '$extract_tool_id', '$map_tool_id', '$crawl_tool_id'),
  '[]',
  '你具备 Web Search 能力，用于获取实时互联网信息、读取网页正文、发现网站页面和有限范围采集网站内容。优先使用 tavily_search 查询实时信息、新闻、公开资料和候选来源；当搜索结果摘要不足以回答时，使用 tavily_extract 读取一个或多个 URL 的正文；当用户需要了解某个网站有哪些相关页面时，使用 tavily_map 发现 URL；只有在明确需要多页面内容采集时才使用 tavily_crawl，并主动限制 max_depth、max_breadth、limit 和 timeout。回答时优先给出结论，再列出关键依据和来源 URL；区分工具返回的事实与自己的推断；如果信息不足或来源冲突，说明不确定性并建议下一步验证。不要抓取需要登录、付费墙、隐私数据或用户未授权的内容。',
  json_array(
    '查询最新事件、新闻、政策、产品信息或公开网页资料',
    '根据搜索结果继续阅读指定网页并提炼结论',
    '发现某个网站的相关页面或文档结构',
    '为知识库构建、竞品调研、资料收集做小范围网页采集'
  ),
  json_array(
    '不要用于执行外部系统写操作或交易动作',
    '不要抓取需要登录、付费墙、隐私或未授权内容',
    '不要进行无限制、大范围或高成本网站爬取',
    '不要把未经验证的搜索摘要当作确定事实'
  ),
  '请用简洁 Markdown 输出：1. 结论；2. 关键依据；3. 来源链接；4. 不确定性或下一步建议。若用户只需要简短回答，可省略分节但仍保留必要来源。',
  'low',
  '{"max_tool_calls":6,"timeout_ms":180000,"allow_write_tools":false,"require_confirmation":false}',
  'v1',
  'v1'
)
ON CONFLICT(tenant_id, id) DO UPDATE SET
  name = excluded.name,
  description = excluded.description,
  business_domain = excluded.business_domain,
  owner_team = excluded.owner_team,
  status = excluded.status,
  tool_ids_json = excluded.tool_ids_json,
  knowledge_ids_json = excluded.knowledge_ids_json,
  system_prompt = excluded.system_prompt,
  use_cases_json = excluded.use_cases_json,
  exclusions_json = excluded.exclusions_json,
  output_format = excluded.output_format,
  risk_level = excluded.risk_level,
  execution_policy_json = excluded.execution_policy_json,
  policy_version = excluded.policy_version,
  version = excluded.version;
SQL

  echo "web_search skill seeded."
  echo "skill_id: skill_web_search"
  echo "tools: $search_tool_id $extract_tool_id $map_tool_id $crawl_tool_id"
}

main "$@"
