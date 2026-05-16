# Flow Anything

Go-first AI platform scaffold.

## Services

- `platform-api`: resource registry and management APIs.
- `ai-orchestrator`: event handling, context assembly, planning, and response actions.
- `agent-runtime`: controlled tool execution runtime.
- `connector-service`: external API and business system integration layer.
- `knowledge-service`: online retrieval API.
- `model-gateway`: model provider gateway with mock and OpenAI-compatible providers.
- `mock-business-api`: local fake business API for connector debugging.

## Development

```bash
make fmt
make test
make run-platform-api
```

Admin console:

```bash
make web-install
make web-dev
```

The runtime services use Go standard-library HTTP primitives and small explicit adapters. Local registry storage uses SQLite through `modernc.org/sqlite`; production adapters such as MySQL, Redis, Qdrant, and message queues can be added behind ports without changing application services.

`platform-api` uses file-backed SQLite for local development by default, so Agent, Skill, Tool, Connector Operation, and MCP Tool configuration survives service restarts:

```bash
PLATFORM_DB_DSN='file:./flow-anything.db?cache=shared' make run-platform-api
```

Use an in-memory database only when you explicitly want a disposable registry:

```bash
PLATFORM_DB_DSN='file:flow_anything?mode=memory&cache=shared' make run-platform-api
```

Runtime state remains intentionally in-memory in the local scaffold: AI Orchestrator conversations and traces, Agent Runtime execution records, and Knowledge Service documents are reset when their processes restart.

## Current MVP APIs

Platform API:

- `POST /v1/agents`
- `GET /v1/agents?tenant_id=...`
- `GET /v1/agents/{agent_id}?tenant_id=...`
- `POST /v1/connector-operations`
- `GET /v1/connector-operations?tenant_id=...`
- `GET /v1/connector-operations/{operation_id}?tenant_id=...`
- `POST /v1/tools`
- `GET /v1/tools?tenant_id=...`
- `GET /v1/tools/{tool_id}?tenant_id=...`
- `POST /v1/skills`
- `GET /v1/skills?tenant_id=...`
- `GET /v1/skills/{skill_id}?tenant_id=...`

Runtime APIs:

- `POST /v1/events` on `ai-orchestrator`
- `POST /v1/tools/execute` on `agent-runtime`
- `GET /v1/tool-executions/{call_id}?tenant_id=...` on `agent-runtime`
- `POST /v1/connector/invoke` on `connector-service`
- `POST /v1/knowledge/documents` on `knowledge-service`
- `POST /v1/knowledge/search` on `knowledge-service`
- `POST /v1/chat/completions` on `model-gateway`

## Model Gateway Provider

`model-gateway` uses the local mock provider by default:

```bash
MODEL_GATEWAY_PROVIDER=mock make run-model-gateway
```

To use DeepSeek:

```bash
MODEL_GATEWAY_PROVIDER=deepseek \
DEEPSEEK_API_KEY='your-deepseek-api-key' \
DEEPSEEK_MODEL='deepseek-v4-flash' \
DEEPSEEK_THINKING='disabled' \
make run-model-gateway
```

Use `deepseek-v4-pro` plus `DEEPSEEK_THINKING=enabled` and `DEEPSEEK_REASONING_EFFORT=high` for heavier reasoning scenarios.

To use an OpenAI-compatible chat completions API:

```bash
MODEL_GATEWAY_PROVIDER=openai-compatible \
OPENAI_COMPATIBLE_BASE_URL='https://api.openai.com/v1' \
OPENAI_COMPATIBLE_API_KEY='your-api-key' \
OPENAI_COMPATIBLE_MODEL='your-model-name' \
make run-model-gateway
```

The provider calls:

```text
POST {OPENAI_COMPATIBLE_BASE_URL}/chat/completions
```

## Local Tool Execution Flow

Start services in separate terminals:

```bash
make run-platform-api
make run-connector-service
make run-knowledge-service
make run-agent-runtime
make run-model-gateway
make run-ai-orchestrator
make run-mock-business-api
```

Or start and stop all local backend services with scripts:

```bash
make start-services
make stop-services
```

The start script loads `configs/local/services.env`, writes PID records to `.runtime/local/services.pid`, writes logs to `log/local/*.log`, and seeds the local weather Agent/Skill/Tool/Connector flow by default. Set `FLOW_ANYTHING_AUTO_SEED=false` to skip automatic seed, or run `make seed-weather-flow` manually after services start.

Register a connector operation:

```bash
curl -s -X POST http://localhost:8080/v1/connector-operations \
  -H 'Content-Type: application/json' \
  -d '{"id":"connop_query_order","tenant_id":"tenant_1","name":"query_order","type":"http","base_url":"http://localhost:8090","method":"GET","path":"/orders/{order_id}"}'
```

Register a tool bound to that connector operation:

```bash
curl -s -X POST http://localhost:8080/v1/tools \
  -H 'Content-Type: application/json' \
  -d '{"id":"tool_query_order","tenant_id":"tenant_1","name":"query_order","description":"查询订单信息","implementation":"connector","binding":{"connector_operation_id":"connop_query_order"},"input_schema":{"type":"object","properties":{"order_id":{"type":"string","description":"订单 ID"}},"required":["order_id"]}}'
```

Register a skill and an agent:

```bash
curl -s -X POST http://localhost:8080/v1/skills \
  -H 'Content-Type: application/json' \
  -d '{"id":"skill_order","tenant_id":"tenant_1","name":"order_service","description":"订单查询能力","tool_ids":["tool_query_order"],"system_prompt":"当用户询问订单时，优先使用 query_order 工具查询订单。"}'
```

```bash
curl -s -X POST http://localhost:8080/v1/agents \
  -H 'Content-Type: application/json' \
  -d '{"id":"agent_order","tenant_id":"tenant_1","name":"Order Agent","description":"面向用户的订单助手","skill_ids":["skill_order"],"default_lang":"zh-CN"}'
```

Execute the tool through Agent Runtime:

```bash
curl -s -X POST http://localhost:8082/v1/tools/execute \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"tenant_1","tool_id":"tool_query_order","args":{"order_id":"o_123"}}'
```

Agent Runtime applies governance before invoking adapters:

- Tool arguments are validated against `input_schema` for required fields and primitive types.
- Tool execution is bounded by `timeout_ms` when configured on the Tool.
- Tools with `risk_level=high` or `requires_confirmation=true` require `"confirmed":true` in the tool call.
- Every execution writes an audit record with tool metadata, status, timing, error code, trace id, confirmation state, and a redacted argument summary.

Query an execution audit record by `call_id`:

```bash
curl -s 'http://localhost:8082/v1/tool-executions/{call_id}?tenant_id=tenant_1'
```

For high-risk tools, AI Orchestrator returns an `ask_confirmation` action instead of executing immediately. After the user confirms, call the same tool with `confirmed=true`:

```bash
curl -s -X POST http://localhost:8081/v1/events \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"tenant_1","type":"user_message_committed","channel":"text","payload":{"tool_id":"tool_query_order","tool_args":{"order_id":"o_123"},"confirmed":true}}'
```

Trigger the same tool through AI Orchestrator tool calling:

```bash
curl -s -X POST http://localhost:8081/v1/events \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"tenant_1","agent_id":"agent_order","type":"user_message_committed","channel":"text","payload":{"text":"帮我查订单 o_123"}}'
```

## Local Weather Tool Flow

`mock-business-api` also exposes a deterministic current weather endpoint:

```bash
curl -s 'http://localhost:8090/weather/current?city=深圳'
```

Register the weather connector operation:

```bash
curl -s -X POST http://localhost:8080/v1/connector-operations \
  -H 'Content-Type: application/json' \
  -d '{"id":"connop_query_weather","tenant_id":"tenant_1","name":"query_weather","description":"查询城市实时天气","type":"http","status":"enabled","implementation_mode":"simple_http","base_url":"http://localhost:8090","method":"GET","path":"/weather/current","input_schema":{"type":"object","properties":{"city":{"type":"string","description":"城市名称"},"unit":{"type":"string","description":"单位，默认 metric"}},"required":["city"]}}'
```

Register a weather tool, skill, and agent:

```bash
curl -s -X POST http://localhost:8080/v1/tools \
  -H 'Content-Type: application/json' \
  -d '{"id":"tool_query_weather","tenant_id":"tenant_1","name":"query_weather","description":"查询城市实时天气","business_domain":"Weather","owner_team":"AI Platform","status":"enabled","implementation":"connector","binding":{"connector_operation_id":"connop_query_weather"},"input_schema":{"type":"object","properties":{"city":{"type":"string","description":"城市名称，例如深圳或Shanghai"},"unit":{"type":"string","description":"单位，默认 metric"}},"required":["city"]},"side_effect":"read","risk_level":"low","requires_confirmation":false,"timeout_ms":8000}'
```

```bash
curl -s -X POST http://localhost:8080/v1/skills \
  -H 'Content-Type: application/json' \
  -d '{"id":"skill_weather","tenant_id":"tenant_1","name":"weather_service","description":"天气查询能力","status":"enabled","tool_ids":["tool_query_weather"],"system_prompt":"当用户询问天气时，调用 query_weather 查询城市实时天气。如果城市缺失，先追问用户。","risk_level":"low"}'
```

```bash
curl -s -X POST http://localhost:8080/v1/agents \
  -H 'Content-Type: application/json' \
  -d '{"id":"agent_weather","tenant_id":"tenant_1","name":"Weather Agent","description":"面向用户的天气助手","status":"enabled","skill_ids":["skill_weather"],"default_lang":"zh-CN","system_prompt":"你是简洁的天气助手。用户询问天气时，调用 query_weather 并基于工具结果回答。"}'
```

Execute the weather tool through Agent Runtime:

```bash
curl -s -X POST http://localhost:8082/v1/tools/execute \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"tenant_1","tool_id":"tool_query_weather","args":{"city":"深圳"}}'
```

Trigger weather lookup through AI Orchestrator tool calling:

```bash
curl -s -X POST http://localhost:8081/v1/events \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"tenant_1","agent_id":"agent_weather","type":"user_message_committed","channel":"text","payload":{"text":"帮我查一下深圳天气"}}'
```

You can still bypass the model and trigger a tool explicitly:

```bash
curl -s -X POST http://localhost:8081/v1/events \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"tenant_1","type":"user_message_committed","channel":"text","payload":{"text":"帮我查订单","tool_id":"tool_query_order","tool_args":{"order_id":"o_123"}}}'
```

Send a plain message through AI Orchestrator and Model Gateway:

```bash
curl -s -X POST http://localhost:8081/v1/events \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"tenant_1","type":"user_message_committed","channel":"text","payload":{"text":"你好，介绍一下你自己"}}'
```

## Local Knowledge Tool Flow

Index a local document:

```bash
curl -s -X POST http://localhost:8084/v1/knowledge/documents \
  -H 'Content-Type: application/json' \
  -d '{"id":"doc_refund","tenant_id":"tenant_1","kb_id":"kb_help","title":"退款规则","text":"用户可以在订单支付后七天内申请退款。退款一般会在三个工作日内到账。","metadata":{"locale":"zh-CN"}}'
```

Search it directly:

```bash
curl -s -X POST http://localhost:8084/v1/knowledge/search \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"tenant_1","kb_ids":["kb_help"],"text":"退款多久到账","top_k":3}'
```

Register a knowledge tool, skill, and agent:

```bash
curl -s -X POST http://localhost:8080/v1/tools \
  -H 'Content-Type: application/json' \
  -d '{"id":"tool_search_help","tenant_id":"tenant_1","name":"search_help_center","description":"检索帮助中心知识库","implementation":"knowledge","binding":{"knowledge_base_ids":["kb_help"]},"input_schema":{"type":"object","properties":{"query":{"type":"string","description":"用户问题或检索关键词"},"top_k":{"type":"integer","description":"返回片段数量"}},"required":["query"]}}'
```

```bash
curl -s -X POST http://localhost:8080/v1/skills \
  -H 'Content-Type: application/json' \
  -d '{"id":"skill_help","tenant_id":"tenant_1","name":"help_center","description":"帮助中心问答能力","tool_ids":["tool_search_help"],"system_prompt":"当用户询问产品规则、售后、退款、帮助中心问题时，优先使用 search_help_center 检索知识库。"}'
```

```bash
curl -s -X POST http://localhost:8080/v1/agents \
  -H 'Content-Type: application/json' \
  -d '{"id":"agent_help","tenant_id":"tenant_1","name":"Help Agent","description":"帮助中心问答助手","skill_ids":["skill_help"],"default_lang":"zh-CN"}'
```

Trigger knowledge retrieval through AI Orchestrator tool calling:

```bash
curl -s -X POST http://localhost:8081/v1/events \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"tenant_1","agent_id":"agent_help","type":"user_message_committed","channel":"text","payload":{"text":"退款多久到账"}}'
```
