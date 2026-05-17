# Trace 功能调研与树形展示设计

更新时间：2026-05-08

## 1. 背景

当前平台已经具备基础 Trace 能力：

- Single Agent 调试时，可以查看一次请求中的 LLM、Skill、Tool、Connector 调用链路。
- Agent Flow 调试时，可以通过 Flow Test 发起全链路测试，并将各 Agent Node 的内部 Trace 合成到 Flow 级 Trace 中。
- 前端 Trace details 当前主要按时间顺序平铺展示 Trace Step。

平铺展示在 Single Agent 场景下问题不大，因为链路通常较短：

```text
event -> agent -> skill -> model -> tool -> connector -> model
```

但进入 Agent Flow / Multi-Agent 场景后，同一次用户请求会包含多个节点、多个 Agent，以及每个 Agent 内部的 Skill/Tool/Connector 调用。如果继续平铺展示，用户很难判断某个 model/tool/connector 属于哪个 Flow Node 或哪个 Agent。

理想的 Trace 结构应该是树形：

```text
Agent Flow Run
├─ Supervisor Planning Node
│  ├─ Agent: Personal Assistant
│  └─ Model: planning
├─ Sub-Agent: Weather Agent
│  ├─ Agent
│  ├─ Skill
│  ├─ Model
│  ├─ Tool
│  └─ Connector
└─ Supervisor Final Answer Node
   ├─ Agent
   └─ Model
```

## 2. 调研目标

本次调研关注两个问题：

- 是否应该引入外部 Trace / Observability 框架。
- 如果不立即引入，现有自研 Trace 如何演进为树状结构，并为未来接入标准观测系统预留空间。

## 3. 主流方案调研

### 3.1 OpenTelemetry

OpenTelemetry 是当前最主流的可观测性标准之一。它将一次 Trace 定义为一组 Span，Span 之间通过 parent-child 关系构成 DAG 或树。

核心概念：

- Trace：一次完整请求或业务操作的执行链路。
- Span：Trace 中的一个执行单元，例如一次 LLM 调用、一次 Tool 调用、一次 HTTP 调用。
- Parent Span：表示调用关系或包含关系。
- Attributes：Span 上的结构化元数据。
- Events：Span 内部发生的关键事件。
- Links：用于表达非严格父子关系，例如 fan-out / fan-in。

优点：

- 标准成熟，生态广。
- Go、Java、Node 等主流语言都有 SDK。
- 可以通过 OTLP 导出到 Jaeger、Tempo、Phoenix、Langfuse 等系统。
- Span 的 parent-child 模型天然适合树形 Trace 展示。

不足：

- OpenTelemetry 是通用观测标准，不是专门面向 LLM / Agent 的产品体验。
- Prompt、LLM response、tool call、retrieval document 等需要自定义 attributes 或采用 OpenInference 语义约定。
- 直接使用 OTel UI 无法天然和我们的 Agent Flow 画布、Agent/Skill/Tool/Connector 配置页面深度联动。

适配判断：

OpenTelemetry 适合作为中长期的数据模型和导出标准，但不建议现在直接用它替代当前平台内的调试 UI。

参考：

- [OpenTelemetry Trace API](https://opentelemetry.io/docs/specs/otel/trace/api)
- [OpenTelemetry Specification Overview](https://opentelemetry.io/docs/reference/specification/overview/)

### 3.2 Jaeger

Jaeger 是经典分布式追踪系统，支持 Trace 查询、Span Timeline、服务依赖图等能力。

优点：

- 部署简单，All-in-One 模式适合本地和测试环境。
- UI 成熟，可以查看 Span 树和时间轴。
- 支持通过 OTLP 接入 OpenTelemetry 数据。
- 对传统微服务链路排查非常成熟。

不足：

- 更偏传统 APM / 微服务 Trace。
- LLM request/response、Tool 参数、Agent 决策过程不是一等公民。
- UI 很难嵌入并适配我们的 Agent Flow 产品交互。

适配判断：

适合作为未来运维/研发排障的外部 Trace 后端，不适合作为当前平台内 Agent Debug 的主 UI。

参考：

- [Jaeger Architecture](https://www.jaegertracing.io/docs/1.45/architecture/)
- [Jaeger Getting Started](https://www.jaegertracing.io/docs/latest/getting-started/)

### 3.3 Grafana Tempo

Tempo 是 Grafana 生态中的 Trace 后端，通常和 Grafana、Loki、Prometheus 一起使用。

优点：

- 适合生产级 Trace 存储。
- Grafana Trace View 支持 Span Timeline、Span Details、Minimap 等。
- 可以和日志、指标打通。
- 适合统一观测体系。

不足：

- 产品形态偏运维观测，不是 Agent 平台内的调试体验。
- 引入后需要额外部署 Grafana/Tempo/Collector。
- 对 LLM/Agent 语义也需要额外规范。

适配判断：

适合中后期生产环境可观测性建设，不建议作为当前 Agent Flow Debug 的第一步。

参考：

- [Grafana Trace View](https://grafana.com/docs/grafana/latest/explore/trace-integration/)
- [Grafana Tempo Telemetry](https://grafana.com/docs/tempo/latest/introduction/telemetry)

### 3.4 Langfuse

Langfuse 是开源 LLM Engineering / Observability 平台，支持 Trace、Session、Prompt、Evaluation 等能力。

优点：

- 面向 LLM / Agent 应用，天然理解 prompt、completion、tool、retrieval、token、cost。
- 支持会话、多轮对话、Agent Graph。
- 开源，可自托管。
- 基于 OpenTelemetry，降低未来 vendor lock-in。

不足：

- 与我们正在建设的 AI 中台能力有重叠，例如 Prompt 管理、评估、观测、实验。
- 如果深度引入，可能出现“双平台”问题：用户在我们的平台配置 Agent，但要去 Langfuse 看 Trace。
- UI 和权限体系很难与当前平台完全一致。

适配判断：

Langfuse 适合作为后续可选的外部 LLM Observability 平台，尤其适合做生产分析、评估、成本追踪。但当前阶段更建议保留我们自己的产品内 Trace UI，同时预留导出到 Langfuse 的能力。

参考：

- [Langfuse Observability](https://langfuse.com/docs/observability/overview)
- [Langfuse Overview](https://langfuse.com/docs)

### 3.5 Arize Phoenix

Phoenix 是面向 AI/LLM 的开源 Observability 和 Evaluation 工具，支持 OpenTelemetry / OpenInference。

优点：

- 面向 LLM、RAG、Tool、Agent 调试。
- 支持 tracing、evaluation、prompt engineering、dataset、experiment。
- OpenInference 语义适合描述 LLM、Retriever、Tool 等 AI 场景。
- 适合调试 RAG、Agent、Prompt 质量。

不足：

- 更偏 AI Observability / Evaluation 平台，不是我们的主业务配置控制台。
- 当前项目以 Go 自研为主，需要手动接入 OTLP/OpenInference 语义。
- 如果直接引入 UI，仍会有平台割裂问题。

适配判断：

Phoenix 适合作为未来 AI 质量分析和评估平台候选，不建议短期替代现有 Trace UI。

参考：

- [Arize Phoenix](https://arize.com/docs/phoenix)
- [Phoenix First Traces](https://arize.com/docs/phoenix/tracing/tutorial/your-first-traces)

### 3.6 LangSmith

LangSmith 是 LangChain / LangGraph 生态中的 Observability、Evaluation、Debug 平台。

优点：

- 对 LangChain / LangGraph Agent trace 支持成熟。
- 可以查看 Agent 执行过程、Prompt、Tool、模型调用。
- 适合 LangChain/LangGraph 项目。

不足：

- 我们已经明确不深度使用 LangChain / LangGraph。
- 深度引入可能带来生态绑定。
- 企业私有化、权限、数据安全、成本等需要额外评估。

适配判断：

不建议作为当前平台的 Trace 主方案。

参考：

- [LangSmith Observability](https://docs.langchain.com/langsmith/observability-concepts)

## 4. 方案对比

| 方案 | 类型 | 树形 Trace | LLM/Agent 语义 | 可自托管 | 与当前平台融合 | 推荐程度 |
|---|---|---:|---:|---:|---:|---|
| 自研 Trace Viewer | 产品内调试 | 高 | 高 | 是 | 高 | 短期强推荐 |
| OpenTelemetry | 标准/SDK | 高 | 中 | 是 | 中 | 中长期强推荐 |
| Jaeger | 通用 Trace UI | 高 | 低 | 是 | 低 | 可选外部后端 |
| Grafana Tempo | Trace 存储/观测 | 高 | 低 | 是 | 低 | 生产观测可选 |
| Langfuse | LLM Observability | 高 | 高 | 是 | 中 | 中后期可选 |
| Phoenix | AI Observability | 高 | 高 | 是 | 中 | 中后期可选 |
| LangSmith | LangChain 生态 | 高 | 高 | 部分 | 低 | 不推荐当前阶段 |

## 5. 当前系统现状

### 5.1 后端模型

当前后端已有 Trace 数据结构：

```go
type TraceRecord struct {
    TraceID        string
    TenantID       tenant.ID
    AgentID        id.ID
    SessionID      id.ID
    EventID        id.ID
    Status         TraceStatus
    StartedAt      time.Time
    FinishedAt     time.Time
    DurationMillis int64
    Error          string
    Steps          []TraceStep
}

type TraceStep struct {
    ID             string
    ParentID       string
    Type           TraceStepType
    Name           string
    Status         TraceStepStatus
    StartedAt      time.Time
    FinishedAt     time.Time
    DurationMillis int64
    Metadata       map[string]any
    Error          string
}
```

这个模型已经具备树状结构的基础，因为 `TraceStep` 已经有 `ParentID` 字段。

但当前主要问题是：

- 多数 Step 没有明确设置 `ParentID`。
- Single Agent 内部 Step 大多按时间追加。
- Agent Flow 合成 Trace 时，虽然可以将 Flow Node 和内部 Agent Trace 拼在一起，但前端仍然平铺展示。
- TraceStep 类型还比较粗粒度，缺少 flow/node/span 语义。

### 5.2 前端展示

当前 `TraceInspector` 的核心逻辑类似：

```tsx
trace.steps.map((step, index) => (
  <TraceStepDetail key={step.id} index={index} step={step} />
))
```

这意味着即使后端提供 `ParentID`，前端也没有利用它构建树。

当前问题：

- Step 顺序展示，不能看出父子关系。
- Agent Flow 场景下，不容易判断某个 Tool/Connector 属于哪个 Node。
- 没有按 Node / Agent 聚合。
- 没有 timeline / duration 层级视觉。

## 6. 推荐方案

### 6.1 总体结论

短期不建议直接引入外部 Trace 框架替代当前功能。

推荐路线：

```text
阶段一：自研树形 Trace Viewer
阶段二：后端补充 ParentID 和 Span 语义
阶段三：内部模型对齐 OpenTelemetry / OpenInference
阶段四：可选导出到 Langfuse / Phoenix / Jaeger / Tempo
```

原因：

- 当前核心诉求是 Agent Flow 产品内调试体验。
- 外部 Trace 平台难以和 Agent Flow 画布、Agent 编辑器、Skill/Tool/Connector 配置深度融合。
- 当前已有 TraceRecord/TraceStep 基础，不需要推翻重做。
- 先自研树形 UI，成本低、收益快。
- 对齐 OpenTelemetry 后，不会堵死未来接入外部观测平台。

## 7. 树形 Trace 设计建议

### 7.1 Trace 树结构

Agent Flow 场景建议组织成：

```text
Trace Root: Agent Flow Run
├─ Flow Node: Supervisor Planning
│  ├─ Agent: Personal Assistant
│  ├─ Skill: xxx
│  ├─ Model: planning
│  ├─ Tool: xxx
│  └─ Connector: xxx
├─ Flow Node: Sub-Agent Weather
│  ├─ Agent: Weather Agent
│  ├─ Skill: weather_service
│  ├─ Model: tool_iteration
│  ├─ Tool: query_weather
│  ├─ Connector: connop_query_weather
│  └─ Model: final_answer
└─ Flow Node: Supervisor Final Answer
   ├─ Agent: Personal Assistant
   └─ Model: final_answer
```

Single Agent 场景建议组织成：

```text
Trace Root: Agent Turn
├─ Event: user_message_committed
├─ Agent: Weather Agent
│  ├─ Skill: weather_service
│  ├─ Model: tool_iteration
│  ├─ Tool: query_weather
│  ├─ Connector: connop_query_weather
│  └─ Model: final_answer
```

### 7.2 后端 ParentID 规范

建议定义统一 ParentID 规则：

| Step 类型 | Parent |
|---|---|
| flow run root | none |
| flow node | flow run root |
| agent | flow node 或 root |
| skill | agent |
| model | agent |
| tool | model 或 agent |
| connector | tool |

说明：

- 如果 tool 是由 LLM tool call 触发，tool 的 parent 可以是对应 model step。
- 如果暂时无法建立精确 model -> tool 关系，tool 可以挂到 agent 下。
- connector 应挂到 tool 下。
- Agent Flow 合成 Trace 时，内部 Agent Trace 的所有 Step 应挂到对应 Flow Node 下。

### 7.3 前端树构建逻辑

前端可以先兼容现有数据，在 `TraceInspector` 中构建树：

```ts
type TraceTreeNode = {
  step: TraceStep;
  children: TraceTreeNode[];
};

function buildTraceTree(steps: TraceStep[]): TraceTreeNode[] {
  const nodeById = new Map<string, TraceTreeNode>();
  const roots: TraceTreeNode[] = [];

  for (const step of steps) {
    nodeById.set(step.id, { step, children: [] });
  }

  for (const step of steps) {
    const node = nodeById.get(step.id)!;
    if (step.parentId && nodeById.has(step.parentId)) {
      nodeById.get(step.parentId)!.children.push(node);
    } else {
      roots.push(node);
    }
  }

  return roots;
}
```

兼容策略：

- 如果 Step 有 `parentId`，按 parentId 建树。
- 如果没有 `parentId`，挂到 synthetic root 下。
- 如果 parentId 缺失或找不到 parent，也挂到 root 下。
- 保留按时间排序，避免展示顺序混乱。

### 7.4 前端交互设计

树形 Trace UI 建议包含：

- 顶部概览：trace status、duration、trace id。
- 左侧/主体：树形链路。
- 每个节点展示：类型、名称、状态、耗时、关键摘要。
- 点击节点：展开/收起 Request、Response、Metadata。
- Agent Flow 节点高亮：Flow Node 名称、Agent 名称、phase。
- 错误节点突出：红色状态、默认展开错误路径。
- 搜索过滤：按 agent/tool/connector/model/provider/trace id 搜索。

可以先不做完整 timeline，但建议预留耗时条：

```text
[Agent Weather] succeeded 2300ms  ███████████
  [Model tool_iteration] 1800ms    ████████
  [Tool query_weather] 41ms        █
```

### 7.5 数据字段建议

当前 `Metadata` 建议逐步标准化：

Flow Node：

```json
{
  "scope": "agent_flow_node",
  "flow_id": "flow_xxx",
  "run_id": "flowrun_xxx",
  "node_id": "weather_node",
  "node_name": "Weather Agent",
  "node_type": "agent_node",
  "agent_id": "agent_weather",
  "phase": "sub_agent"
}
```

Agent：

```json
{
  "agent_id": "agent_weather",
  "agent_name": "Weather Agent",
  "model": "deepseek-v4-flash",
  "skill_count": 1,
  "tool_count": 3
}
```

Model：

```json
{
  "phase": "tool_iteration",
  "provider": "deepseek",
  "requested_model": "deepseek-v4-flash",
  "response_model": "deepseek-v4-flash",
  "input_tokens": 1234,
  "output_tokens": 321,
  "total_tokens": 1555,
  "request": {},
  "response": {}
}
```

Tool：

```json
{
  "tool_id": "tool_query_weather",
  "tool_name": "query_weather",
  "implementation": "connector",
  "connector_operation_id": "connop_query_weather",
  "success": true,
  "request": {},
  "response": {}
}
```

Connector：

```json
{
  "connector_operation_id": "connop_query_weather",
  "tool_id": "tool_query_weather",
  "success": true,
  "request": {},
  "response": {}
}
```

## 8. OpenTelemetry 对齐建议

当前系统可以不立刻引入 OTel SDK，但建议在内部命名上逐步向 OTel 靠齐。

建议映射：

| 当前字段 | OTel 概念 |
|---|---|
| TraceRecord.TraceID | trace_id |
| TraceStep.ID | span_id |
| TraceStep.ParentID | parent_span_id |
| TraceStep.Name | span name |
| TraceStep.Type | span kind / custom attribute |
| TraceStep.Metadata | span attributes |
| TraceStep.Error | span status / exception event |
| StartedAt / FinishedAt | span start/end time |

建议新增或标准化：

- `span_id`
- `parent_span_id`
- `service_name`
- `span_kind`
- `attributes`
- `events`
- `links`

可以保留现有 API 字段，对外兼容；内部逐步加字段。

## 9. OpenInference 对齐建议

如果未来希望接入 Phoenix / Langfuse 等 LLM Observability 平台，建议参考 OpenInference 的语义，把 AI 场景字段标准化。

建议 Span 类型：

- `AGENT`
- `CHAIN`
- `LLM`
- `TOOL`
- `RETRIEVER`
- `EMBEDDING`
- `RERANKER`

对应当前平台：

| 当前类型 | OpenInference 类似语义 |
---|---|
| agent | AGENT |
| skill | CHAIN |
| model | LLM |
| tool | TOOL |
| connector | TOOL / external call |
| knowledge | RETRIEVER |

这样未来导出到 Phoenix / Langfuse 时，数据会更自然。

## 10. 分阶段实施计划

### 阶段一：前端树形 Trace Viewer

目标：

- 不改后端 API。
- 前端基于 `parentId` 构建树。
- 无 parentId 的旧 Step 自动挂到 root。
- Agent Flow 合成 Trace 中显示 Flow Node 层级。

任务：

- 新增 `TraceTree` 构建函数。
- 重构 `TraceInspector`，从平铺列表改为递归树。
- 节点支持展开/收起。
- 保留 Request/Response/Metadata 展示。
- Agent Flow 节点增加明显视觉分组。

### 阶段二：后端补 ParentID

目标：

- Single Agent 内部 Trace 也能体现层级。
- Tool/Connector/Model 能挂到合理父节点。

任务：

- 在 `appendTraceStep` 支持传入 parentID。
- Agent step 作为 Single Agent root 下的主要 parent。
- Skill step 挂 agent。
- Model step 挂 agent。
- Tool step 挂 model 或 agent。
- Connector step 挂 tool。

### 阶段三：Agent Flow Trace 结构标准化

目标：

- Flow Run 是 root。
- 每个 Flow Node 是二级节点。
- 每个 Agent 内部 Trace 挂到对应 Flow Node 下。

任务：

- Flow Runtime 返回或聚合 trace tree。
- 前端当前合成逻辑可以先保留，但后续应由后端输出标准结构。
- NodeRun output 中稳定输出 `trace_id`、`agent_id`、`phase`。

### 阶段四：OTel / OpenInference 导出

目标：

- 保留产品内调试 UI。
- 可选导出到外部观测平台。

任务：

- 定义 Trace Exporter 接口。
- 实现 OTLP Exporter。
- 可选接入 Jaeger / Tempo。
- 可选接入 Langfuse / Phoenix。

## 11. 推荐决策

当前阶段推荐：

```text
继续沿用现有自研 Trace 体系
优先实现树形 Trace Viewer
后端逐步补 ParentID
内部模型向 OpenTelemetry / OpenInference 对齐
暂不直接引入外部 Trace UI
```

理由：

- 当前痛点是 Agent Flow 调试体验，不是生产级链路观测。
- 自研 Tree Viewer 能最快解决“看不出 Step 属于哪个 Node”的问题。
- 现有 TraceStep 已经有 ParentID，改造成本低。
- 外部平台更适合中长期生产观测、评估、成本分析。
- 先对齐标准，后续接入外部平台不会推倒重来。

## 12. 后续建议

下一步可以优先实现：

1. 前端 `TraceInspector` 树形展示。
2. Agent Flow 合成 Trace 时补齐 parentId。
3. Single Agent Trace 后端补 parentId。
4. Trace 节点搜索和错误路径高亮。
5. 设计 OTLP Exporter，但暂不落地部署。

## 13. 参考资料

- [OpenTelemetry Trace API](https://opentelemetry.io/docs/specs/otel/trace/api)
- [OpenTelemetry Specification Overview](https://opentelemetry.io/docs/reference/specification/overview/)
- [OpenTelemetry Collector Configuration](https://opentelemetry.io/docs/collector/configuration/)
- [Jaeger Architecture](https://www.jaegertracing.io/docs/1.45/architecture/)
- [Jaeger Getting Started](https://www.jaegertracing.io/docs/latest/getting-started/)
- [Grafana Trace View](https://grafana.com/docs/grafana/latest/explore/trace-integration/)
- [Grafana Tempo Telemetry](https://grafana.com/docs/tempo/latest/introduction/telemetry)
- [Langfuse Observability](https://langfuse.com/docs/observability/overview)
- [Langfuse Overview](https://langfuse.com/docs)
- [Arize Phoenix](https://arize.com/docs/phoenix)
- [Phoenix First Traces](https://arize.com/docs/phoenix/tracing/tutorial/your-first-traces)
- [LangSmith Observability Concepts](https://docs.langchain.com/langsmith/observability-concepts)
