#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DB_PATH="${FLOW_ANYTHING_DB_PATH:-$ROOT_DIR/flow-anything.db}"
TENANT_ID="${FLOW_ANYTHING_TENANT_ID:-tenant_1}"

main() {
  if [[ ! -f "$DB_PATH" ]]; then
    echo "database not found: $DB_PATH" >&2
    exit 1
  fi

  sqlite3 "$DB_PATH" <<SQL
INSERT OR REPLACE INTO connectors (
  tenant_id, id, name, description, business_domain, owner_team, type, status,
  base_url, headers_json, auth_json, timeout_ms, version
) VALUES (
  '$TENANT_ID',
  'conn_feishu_doc',
  '飞书文档',
  '飞书新版文档 Docx connector，支持创建文档、Markdown/HTML 转文档块以及创建嵌套块。',
  'Collaboration',
  'AI Platform',
  'http',
  'enabled',
  'https://open.feishu.cn',
  '{"Accept":"application/json"}',
  '{"type":"oauth2","provider":"feishu_tenant_access_token","client_id_ref":"env:FEISHU_APP_ID","client_secret_ref":"env:FEISHU_APP_SECRET","tenant_access_token_url":"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"}',
  30000,
  'v1'
);

INSERT OR REPLACE INTO connector_operations (
  tenant_id, id, connector_id, name, description, business_domain, owner_team,
  implementation_mode, type, status, base_url, method, path, headers_json,
  auth_json, input_schema_json, output_schema_json, timeout_ms, version
) VALUES (
  '$TENANT_ID',
  'connop_feishu_create_document',
  'conn_feishu_doc',
  'feishu_create_document',
  '创建飞书新版文档，可指定标题和目标文件夹 token。',
  '',
  '',
  'simple_http',
  'http',
  'enabled',
  '',
  'POST',
  '/open-apis/docx/v1/documents',
  '{}',
  '{}',
  '{"type":"object","properties":{"title":{"type":"string","description":"文档标题。建议使用用户可识别的标题，例如报告主题或任务名称。"},"folder_token":{"type":"string","description":"飞书文件夹 token。可选；为空时使用飞书默认位置或应用可访问的默认空间。"}}}',
  '{"type":"object","properties":{"code":{"type":"integer","description":"飞书错误码，0 表示成功。"},"msg":{"type":"string","description":"飞书返回消息。"},"data":{"type":"object","properties":{"document":{"type":"object","properties":{"document_id":{"type":"string","description":"新版文档 ID，也是根 block_id，可用于后续写入 blocks。"},"revision_id":{"type":"integer","description":"文档版本号。"},"title":{"type":"string","description":"文档标题。"},"url":{"type":"string","description":"文档访问 URL，若接口返回则可直接给用户。"}}}}},"status_code":{"type":"integer","description":"Connector runtime 捕获的 HTTP 状态码。"}}}',
  30000,
  'v1'
);

INSERT OR REPLACE INTO connector_operations (
  tenant_id, id, connector_id, name, description, business_domain, owner_team,
  implementation_mode, type, status, base_url, method, path, headers_json,
  auth_json, input_schema_json, output_schema_json, timeout_ms, version
) VALUES (
  '$TENANT_ID',
  'connop_feishu_convert_markdown',
  'conn_feishu_doc',
  'feishu_convert_markdown',
  '将 Markdown 或 HTML 内容转换为飞书新版文档 blocks，供后续创建嵌套块写入。',
  '',
  '',
  'simple_http',
  'http',
  'enabled',
  '',
  'POST',
  '/open-apis/docx/v1/documents/blocks/convert',
  '{}',
  '{}',
  '{"type":"object","required":["content_type","content"],"properties":{"content_type":{"type":"string","enum":["markdown","html"],"description":"内容类型。上传 Markdown 文档时固定传 markdown。"},"content":{"type":"string","description":"需要上传到飞书文档的 Markdown 或 HTML 内容。Markdown 表格、标题、列表会尽量转换为飞书 blocks。"}}}',
  '{"type":"object","properties":{"code":{"type":"integer","description":"飞书错误码，0 表示成功。"},"msg":{"type":"string","description":"飞书返回消息。"},"data":{"type":"object","properties":{"blocks":{"type":"array","description":"转换后的飞书 block 列表。作为 feishu_create_nested_blocks 的 descendants 输入。","items":{"type":"object"}},"first_level_block_ids":{"type":"array","description":"第一层 block_id 列表。作为 feishu_create_nested_blocks 的 children_id 输入。","items":{"type":"string"}}}},"status_code":{"type":"integer","description":"Connector runtime 捕获的 HTTP 状态码。"}}}',
  30000,
  'v1'
);

INSERT OR REPLACE INTO connector_operations (
  tenant_id, id, connector_id, name, description, business_domain, owner_team,
  implementation_mode, type, status, base_url, method, path, headers_json,
  auth_json, input_schema_json, output_schema_json, timeout_ms, version
) VALUES (
  '$TENANT_ID',
  'connop_feishu_create_nested_blocks',
  'conn_feishu_doc',
  'feishu_create_nested_blocks',
  '向飞书新版文档的指定 block 下创建一批嵌套 blocks，用于写入由 Markdown 转换得到的正文和表格。',
  '',
  '',
  'simple_http',
  'http',
  'enabled',
  '',
  'POST',
  '/open-apis/docx/v1/documents/{document_id}/blocks/{block_id}/descendant?document_revision_id=-1',
  '{}',
  '{}',
  '{"type":"object","required":["document_id","block_id","children_id","descendants"],"properties":{"document_id":{"type":"string","description":"飞书新版文档 ID。"},"block_id":{"type":"string","description":"父 block ID。写入文档根节点时通常等于 document_id。"},"children_id":{"type":"array","items":{"type":"string"},"description":"第一层子 block_id 列表，通常来自 feishu_convert_markdown 返回的 data.first_level_block_ids。"},"descendants":{"type":"array","items":{"type":"object"},"description":"完整嵌套 block 列表，通常来自 feishu_convert_markdown 返回的 data.blocks。"},"index":{"type":"integer","description":"插入位置。0 表示开头，-1 表示末尾；默认建议 0 或 -1。"}}}',
  '{"type":"object","properties":{"code":{"type":"integer","description":"飞书错误码，0 表示成功。"},"msg":{"type":"string","description":"飞书返回消息。"},"data":{"type":"object","description":"飞书创建嵌套 blocks 的返回数据，通常包含新建 block 信息或文档版本信息。"},"status_code":{"type":"integer","description":"Connector runtime 捕获的 HTTP 状态码。"}}}',
  30000,
  'v1'
);

INSERT OR REPLACE INTO tools (
  tenant_id, id, name, description, business_domain, owner_team, status,
  llm_description, implementation, binding_json, input_schema_json, output_schema_json,
  side_effect, risk_level, requires_confirmation, timeout_ms, retry_policy_json, version
) VALUES (
  '$TENANT_ID',
  'tool_feishu_create_document',
  'feishu_create_document',
  '创建飞书新版文档，可指定标题和文件夹。',
  'Collaboration',
  'AI Platform',
  'enabled',
  'Create a Feishu/Lark Docx document. Use this before writing markdown content. Return document_id and URL if available.',
  'connector',
  '{"connector_operation_id":"connop_feishu_create_document"}',
  (SELECT input_schema_json FROM connector_operations WHERE tenant_id='$TENANT_ID' AND id='connop_feishu_create_document'),
  (SELECT output_schema_json FROM connector_operations WHERE tenant_id='$TENANT_ID' AND id='connop_feishu_create_document'),
  'write',
  'medium',
  0,
  30000,
  '{"max_attempts":2,"backoff_ms":500}',
  'v1'
);

INSERT OR REPLACE INTO tools (
  tenant_id, id, name, description, business_domain, owner_team, status,
  llm_description, implementation, binding_json, input_schema_json, output_schema_json,
  side_effect, risk_level, requires_confirmation, timeout_ms, retry_policy_json, version
) VALUES (
  '$TENANT_ID',
  'tool_feishu_convert_markdown',
  'feishu_convert_markdown',
  '将 Markdown 或 HTML 转换为飞书文档 blocks。',
  'Collaboration',
  'AI Platform',
  'enabled',
  'Convert full Markdown or HTML content into Feishu Docx blocks. This API does not need document_id; pass content_type=markdown and content only. Feed blocks and first_level_block_ids to feishu_create_nested_blocks.',
  'connector',
  '{"connector_operation_id":"connop_feishu_convert_markdown"}',
  (SELECT input_schema_json FROM connector_operations WHERE tenant_id='$TENANT_ID' AND id='connop_feishu_convert_markdown'),
  (SELECT output_schema_json FROM connector_operations WHERE tenant_id='$TENANT_ID' AND id='connop_feishu_convert_markdown'),
  'write',
  'medium',
  0,
  30000,
  '{"max_attempts":2,"backoff_ms":500}',
  'v1'
);

INSERT OR REPLACE INTO tools (
  tenant_id, id, name, description, business_domain, owner_team, status,
  llm_description, implementation, binding_json, input_schema_json, output_schema_json,
  side_effect, risk_level, requires_confirmation, timeout_ms, retry_policy_json, version
) VALUES (
  '$TENANT_ID',
  'tool_feishu_create_nested_blocks',
  'feishu_create_nested_blocks',
  '向飞书文档指定 block 写入嵌套 blocks。',
  'Collaboration',
  'AI Platform',
  'enabled',
  'Write converted Feishu Docx blocks into a document. Use block_id=document_id for root insertion, children_id from first_level_block_ids, descendants from blocks, and index=0 or -1.',
  'connector',
  '{"connector_operation_id":"connop_feishu_create_nested_blocks"}',
  (SELECT input_schema_json FROM connector_operations WHERE tenant_id='$TENANT_ID' AND id='connop_feishu_create_nested_blocks'),
  (SELECT output_schema_json FROM connector_operations WHERE tenant_id='$TENANT_ID' AND id='connop_feishu_create_nested_blocks'),
  'write',
  'medium',
  0,
  30000,
  '{"max_attempts":2,"backoff_ms":500}',
  'v1'
);

INSERT OR REPLACE INTO workflows (
  tenant_id, id, name, description, business_domain, owner_team, status, profile,
  context_schema_json, input_schema_json, output_schema_json, graph_json, policy_json, ui_json, version
) VALUES (
  '$TENANT_ID',
  'wf_feishu_upload_markdown',
  'feishu_upload_markdown',
  '将标题、Markdown 内容和可选 folder_token 编排上传为飞书新版文档。',
  'Collaboration',
  'AI Platform',
  'enabled',
  'tool_workflow',
  '{"type":"object","properties":{"feishu":{"type":"object","properties":{"document_id":{"type":"string"},"document_url":{"type":"string"}}},"document":{"type":"object","properties":{"blocks":{"type":"array"},"first_level_block_ids":{"type":"array"}}},"result":{"type":"object","properties":{"document_id":{"type":"string"},"url":{"type":"string"},"success":{"type":"boolean"}}}}}',
  '{"type":"object","required":["title","markdown"],"properties":{"title":{"type":"string","description":"文档标题。"},"markdown":{"type":"string","description":"需要上传的 Markdown 内容。"},"folder_token":{"type":"string","description":"可选，目标飞书文件夹 token。"}}}',
  '{"type":"object","properties":{"document_id":{"type":"string"},"url":{"type":"string"},"success":{"type":"boolean"}}}',
  '{"entry_node_id":"start","nodes":{"start":{"id":"start","type":"start","name":"Start"},"create_doc":{"id":"create_doc","type":"connector_operation","name":"Create Feishu Doc","config":{"operation_id":"connop_feishu_create_document","input_mapping":{"title":"$input.title","folder_token":"$input.folder_token"},"output_mapping":{"document_id":"$.data.data.document.document_id","url":"$.data.data.document.url"},"write_context":{"ctx.feishu.document_id":"$output.document_id","ctx.feishu.document_url":"$output.url"}},"timeout_ms":30000},"convert_markdown":{"id":"convert_markdown","type":"connector_operation","name":"Convert Markdown","config":{"operation_id":"connop_feishu_convert_markdown","input_mapping":{"content_type":"markdown","content":"$input.markdown"},"output_mapping":{"blocks":"$.data.data.blocks","first_level_block_ids":"$.data.data.first_level_block_ids"},"write_context":{"ctx.document.blocks":"$output.blocks","ctx.document.first_level_block_ids":"$output.first_level_block_ids"}},"timeout_ms":30000},"write_blocks":{"id":"write_blocks","type":"connector_operation","name":"Write Feishu Blocks","config":{"operation_id":"connop_feishu_create_nested_blocks","input_mapping":{"document_id":"$ctx.feishu.document_id","block_id":"$ctx.feishu.document_id","children_id":"$ctx.document.first_level_block_ids","descendants":"$ctx.document.blocks","index":0},"output_mapping":{"success":"$.success","data":"$.data"},"write_context":{"ctx.result.success":"$output.success","ctx.result.document_id":"$ctx.feishu.document_id","ctx.result.url":"$ctx.feishu.document_url"}},"timeout_ms":30000},"end":{"id":"end","type":"end","name":"End","config":{"output_mapping":{"document_id":"$ctx.result.document_id","url":"$ctx.result.url","success":"$ctx.result.success"}}}},"edges":[{"id":"start-create","from_node_id":"start","to_node_id":"create_doc","type":"default"},{"id":"create-convert","from_node_id":"create_doc","to_node_id":"convert_markdown","type":"default"},{"id":"convert-write","from_node_id":"convert_markdown","to_node_id":"write_blocks","type":"default"},{"id":"write-end","from_node_id":"write_blocks","to_node_id":"end","type":"default"}]}',
  '{"max_steps":16,"max_parallelism":1,"timeout_ms":180000}',
  '{}',
  'v1'
);

INSERT OR REPLACE INTO tools (
  tenant_id, id, name, description, business_domain, owner_team, status,
  llm_description, implementation, binding_json, input_schema_json, output_schema_json,
  side_effect, risk_level, requires_confirmation, timeout_ms, retry_policy_json, version
) VALUES (
  '$TENANT_ID',
  'tool_feishu_upload_markdown',
  'feishu_upload_markdown',
  '通过 workflow 将 Markdown 文档上传为飞书新版文档。',
  'Collaboration',
  'AI Platform',
  'enabled',
  'Upload Markdown content to a Feishu/Lark Docx document using a deterministic workflow. Input title, markdown, and optional folder_token. Return document_id and url.',
  'workflow',
  '{"workflow_id":"wf_feishu_upload_markdown"}',
  '{"type":"object","required":["title","markdown"],"properties":{"title":{"type":"string","description":"文档标题。"},"markdown":{"type":"string","description":"完整 Markdown 内容。"},"folder_token":{"type":"string","description":"可选，目标飞书文件夹 token。"}}}',
  '{"type":"object","properties":{"document_id":{"type":"string"},"url":{"type":"string"},"success":{"type":"boolean"}}}',
  'write',
  'medium',
  0,
  180000,
  '{"max_attempts":1,"backoff_ms":0}',
  'v1'
);

INSERT INTO skills (
  tenant_id, id, name, description, business_domain, owner_team, status,
  tool_ids_json, knowledge_ids_json, system_prompt, use_cases_json, exclusions_json,
  output_format, risk_level, execution_policy_json, policy_version, version
) VALUES (
  '$TENANT_ID',
  'skill_feishu_doc_write',
  'feishu_doc_write',
  '将 Agent 产出的 Markdown 文档、报告和表格上传为飞书新版文档。',
  'Collaboration',
  'AI Platform',
  'enabled',
  json_array('tool_feishu_upload_markdown'),
  '[]',
  '你具备将 Markdown/HTML 内容上传到飞书新版文档的能力。请优先调用 feishu_upload_markdown 这个 workflow tool，一次性传入 title、markdown 和可选 folder_token。这个工具会确定性完成创建文档、Markdown 转 blocks、写入文档三步，不要再手动逐个调用底层飞书 API。成功后返回文档标题、document_id、URL（如果接口返回）以及写入摘要。涉及外部系统写入时，确认用户明确要求创建或上传文档后再执行。',
  json_array(
    '把分析报告、总结、会议纪要、技术方案等 Markdown 内容上传为飞书文档',
    '把包含 Markdown 表格的内容转为飞书文档',
    '创建空飞书文档并写入由 Agent 生成的结构化内容',
    '向指定飞书文件夹上传文档'
  ),
  json_array(
    '不要用于读取或导出用户无权限访问的飞书文档',
    '不要在用户没有明确要求创建/上传文档时主动写入飞书',
    '不要把敏感信息写入用户未指定或未授权的文件夹',
    '不要并发多次写入同一文档，避免块顺序错乱或触发限流'
  ),
  '请用简洁 Markdown 返回：文档标题、document_id、URL（如果有）、写入结果、注意事项。失败时说明失败阶段和可操作的修复建议。',
  'medium',
  '{"max_tool_calls":6,"timeout_ms":180000,"allow_write_tools":true,"require_confirmation":false}',
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

  echo "feishu doc connector seeded."
  echo "auth: oauth2 provider=feishu_tenant_access_token"
  echo "required env: FEISHU_APP_ID FEISHU_APP_SECRET"
  echo "optional env: FEISHU_DOC_FOLDER_TOKEN"
  echo "connector: conn_feishu_doc"
  echo "operations: connop_feishu_create_document connop_feishu_convert_markdown connop_feishu_create_nested_blocks"
  echo "workflow: wf_feishu_upload_markdown"
  echo "tools: tool_feishu_create_document tool_feishu_convert_markdown tool_feishu_create_nested_blocks tool_feishu_upload_markdown"
  echo "skill: skill_feishu_doc_write"
}

main "$@"
