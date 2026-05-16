# 技术栈与开源框架选型文档

## 1. 文档目的

本文档用于梳理 AI 中台项目的技术栈、开源框架选择，以及 LangChain / LangGraph / Dify 等框架在本项目中的使用边界。

本文档基于当前最新决策：

```text
AI 中台核心在线服务采用 Go First 技术栈。
Python 不作为核心在线运行时主语言，仅用于脚本工具、离线 AI worker 和实验分析。
```

重点回答：

- 各核心模块推荐使用什么语言和技术栈
- 为什么核心在线服务统一采用 Go
- Python 在项目中的保留边界是什么
- 哪些开源框架适合直接使用
- 为什么不建议深度绑定 LangChain / LangGraph / Dify 作为中台核心运行时
- MVP 阶段建议采用什么技术组合

---

## 2. 总体选型原则

## 2.1 Go First 原则

本项目推荐采用：

```text
Go 负责 AI 中台核心在线服务和平台控制面
TypeScript 负责管理后台
Python 负责隔离脚本工具和离线 AI/Data worker
```

核心原因：

- 团队已有 Go / Java 经验，采用 Go 可以显著降低核心平台建设的学习成本
- AI 中台的工程难点在状态机、工具治理、权限审计、灰度发布、回放评测和多租户，而不只是调用 LLM SDK
- Go 适合高并发、低延迟、强类型、易部署、易观测的在线服务
- LLM、Embedding、Rerank、ASR/TTS 等能力都可以通过 HTTP/gRPC adapter 接入，不要求核心服务跟随 Python 生态
- Python 仍然适合文档解析、离线评测、模型实验、脚本 Tool，但应和核心在线运行时解耦

## 2.2 平台核心抽象自研

以下平台核心抽象建议自研掌控：

- Agent Profile
- Skill
- Tool Registry
- Tool Set
- Connector Operation
- Workflow
- Context Builder
- Memory Service
- Resource Registry
- Release / Audit

这些对象直接关系到产品化、运营、权限、审计、灰度、回滚和多租户治理，不建议交给某个 Agent 框架决定。

## 2.3 开源框架局部使用

开源框架应作为工程加速器，而不是平台资源模型的来源。

建议方式：

```text
平台协议和资源模型自研
底层通用能力按需使用开源框架
Python 框架只进入隔离 worker、实验环境或 adapter 层
```

---

## 3. 模块技术栈总览

| 模块 | 推荐语言 | 推荐技术栈 | 说明 |
| --- | --- | --- | --- |
| AI Orchestrator | Go | chi/gin, grpc-go/connect-go, validator, JSON Schema, Redis, MySQL | LLM 编排、Context Builder、推理模式控制 |
| Agent Runtime | Go | Tool Executor, Workflow State Machine, Redis Lock, MySQL Audit, OpenTelemetry | Tool 执行、Workflow、确认、幂等、审计 |
| Connector Service | Go | net/http/resty, gRPC, resilience policy, schema mapping, SDK adapter | 外部业务系统 API 封装和 Connector Operation 暴露 |
| Knowledge Service | Go | Qdrant adapter, hybrid retrieval, rerank adapter, MySQL metadata | 在线 RAG 检索、权限过滤、检索结果标准化 |
| Memory Service | Go | MySQL, Redis, Qdrant optional, embedding adapter, policy engine | 长期记忆写入、检索、治理 |
| Platform Management API | Go | chi/gin, sqlc/ent, RBAC, release workflow, audit | 管理后台、Resource Registry、发布审批 |
| Realtime Gateway | Go | WebSocket/WebRTC, gRPC, go-redis, OpenTelemetry | 语音实时链路，可复用既有能力 |
| Speech Service | Go | ASR/TTS SDK adapter, streaming adapter | 语音能力封装，可复用既有 NLP 机器人经验 |
| Python Tool Runner | Python | sandbox, venv/uv, restricted packages, resource limits | 执行 Python 脚本类 Tool |
| Document / Eval Worker | Python 可选 | pypdf, python-docx, openpyxl, pandas, eval runner | 文档解析、embedding 批处理、离线评测 |
| Admin Console | TypeScript | React, Vite, Ant Design, TanStack Query | 中台运营管理后台 |

---

## 4. Go 核心服务技术栈

适用模块：

- AI Orchestrator
- Agent Runtime
- Connector Service
- Knowledge Service
- Memory Service
- Platform Management API
- Auth / Tenant / Audit
- Realtime Gateway
- Speech Service

## 4.1 API 框架

推荐：

- `chi`
- `gin`

建议：

- 偏平台 API、清晰中间件、轻量路由时优先 `chi`
- 偏快速开发、生态插件、团队熟悉度时可选 `gin`
- 不建议为了框架能力引入过重 Web 框架，核心复杂度应在领域模型和运行时治理中

## 4.2 RPC 与内部通信

推荐：

- `grpc-go`
- `connect-go`

建议：

- 核心服务之间优先使用 gRPC / Connect，便于 schema 约束和跨语言 adapter
- 面向前端和外部系统提供 REST API
- 对需要流式能力的场景保留 streaming RPC 或 WebSocket

## 4.3 Schema 与参数校验

推荐：

- `go-playground/validator`
- `santhosh-tekuri/jsonschema`
- Protobuf / OpenAPI

用途：

- Tool input schema 校验
- Connector Operation request / response schema 校验
- Agent Profile、Skill、Workflow 配置校验
- 发布前静态校验

## 4.4 数据库访问

推荐：

- `sqlc`
- `ent`
- `go-sql-driver/mysql`

选择建议：

- 如果团队希望 SQL 可控、审计清晰，优先 `sqlc`
- 如果资源模型关系复杂、需要较多实体操作，可选 `ent`
- 不建议在核心服务里引入过厚 ORM，避免 SQL 行为不透明

## 4.5 Redis

推荐：

- `go-redis`

用途：

- session hot state
- recent turns
- context cache
- idempotency key
- execution lock
- config cache
- rate limit counter

## 4.6 HTTP Client 与外部调用

推荐：

- Go 标准库 `net/http`
- `resty`

用途：

- LLM provider API
- Embedding / Rerank API
- 外部业务系统 API
- 第三方 SaaS API

要求：

- 统一 timeout
- retry policy
- circuit breaker
- rate limit
- request / response audit
- sensitive data masking

## 4.7 日志与可观测性

推荐：

- `zap`
- `zerolog`
- `OpenTelemetry Go`
- Prometheus
- Grafana
- Loki / ELK / OpenSearch
- Jaeger / Tempo

必须打通 trace：

- user event
- orchestrator decision
- context budget
- model call
- tool plan
- tool execution
- connector invoke
- knowledge retrieval
- memory retrieval
- final response

## 4.8 测试

推荐：

- Go standard testing
- `testify`
- `gomock` / `mockery`
- `testcontainers-go` 可选

重点测试：

- Tool schema validation
- Tool execution idempotency
- Connector retry / timeout / circuit breaker
- Context budget control
- Knowledge retrieval permission filtering
- Memory write / merge / retrieve policy
- Release rollback compatibility

---

## 5. Python 的保留边界

Python 不作为 AI Orchestrator、Agent Runtime、Connector Service、Knowledge Service 的主实现语言。  
但 Python 仍然适合以下隔离场景。

## 5.1 Python Tool Runner

用途：

- 执行平台上配置的 Python Code Tool
- 支持数据处理、轻量计算、业务脚本、临时自动化
- 作为 Agent Runtime 的一种 Tool Runner Adapter

关键要求：

- 独立进程或容器沙箱
- CPU / memory / timeout 限制
- package allowlist
- 网络访问控制
- 文件系统隔离
- stdout / stderr / artifact 标准化
- 执行结果回传 Agent Runtime

## 5.2 Document Worker

用途：

- PDF、Word、Excel、HTML、Markdown 文档解析
- OCR 或复杂版面解析
- chunk 预处理
- metadata 抽取

可选库：

- `pypdf`
- `python-docx`
- `openpyxl`
- `beautifulsoup4`
- `markdown`
- `unstructured` 可评估

## 5.3 Embedding / Rerank / Eval Worker

用途：

- 批量 embedding
- rerank 离线评测
- RAG recall@k 评估
- memory extraction 质量评估
- 错误归因与统计分析

可选库：

- `pandas`
- `pytest`
- `numpy`
- 模型供应商 SDK

## 5.4 与 Go 主服务的边界

推荐交互方式：

```text
Go 主服务
  ->
消息队列 / 任务表
  ->
Python Worker
  ->
MySQL / Object Storage / Qdrant
  ->
Go 主服务读取结果
```

原则：

- Python worker 不直接控制核心会话状态
- Python worker 不直接执行高风险业务副作用
- Python worker 的输出必须经过 Go 主服务校验、审计和状态提交
- Python worker 可以失败重试，不应阻塞主链路可用性

---

## 6. Knowledge / RAG 技术栈

## 6.1 在线检索

推荐由 Go 实现：

- retrieval API
- permission filter
- hybrid search orchestration
- rerank adapter
- result normalization
- citation / source metadata
- context budget pre-filter

## 6.2 文档入库

推荐采用 Go 控制任务状态，Python worker 执行复杂解析：

```text
Upload Document
  ->
Go Knowledge Service 创建 ingestion job
  ->
Python Document Worker 解析文档
  ->
Go Knowledge Service 校验 chunk 与 metadata
  ->
Embedding Worker 生成向量
  ->
写入 Qdrant + MySQL metadata
```

## 6.3 向量库

初期推荐：

- Qdrant

原因：

- payload filter 直观
- 适合 MySQL + 独立向量库架构
- 初期接入和运维复杂度较低

后续可评估：

- Milvus

前提：

- 数据规模明显增大
- 检索基础设施团队成熟
- 需要更重型的向量检索能力

## 6.4 Rerank

建议保持 rerank adapter 抽象：

- bge-reranker
- Cohere rerank
- 模型供应商 rerank API
- 企业内部 rerank 模型

---

## 7. Memory Service 技术栈

推荐由 Go 实现在线服务。

核心模块建议自研：

- Memory Candidate Extractor
- Memory Classifier
- Memory Policy Engine
- Dedup / Merge
- Memory Retriever
- Memory Reranker
- Memory Audit

不建议直接使用现成 Agent 框架的 memory abstraction 作为核心长期记忆模型。

原因：

- 企业长期记忆需要用户可查看、可删除、可审计
- 需要 scope、sensitivity、confidence、source、ttl
- 需要多租户和合规治理
- 需要结构化存储为主、向量索引为辅

---

## 8. 前端管理后台技术栈

推荐：

- React
- TypeScript
- Vite
- Ant Design
- TanStack Query
- React Router
- Zustand

原因：

- 企业后台表单、表格、抽屉、弹窗、审批流组件成熟
- 适合 Agent / Skill / Tool / Connector / Knowledge 等资源管理

核心页面：

- Dashboard
- Agent 管理
- Skill 管理
- Tool 管理
- Connector 管理
- Knowledge 管理
- Memory 管理
- Prompt / Policy 管理
- Sandbox
- Release
- Observability
- Audit

---

## 9. 数据与基础设施

## 9.1 MySQL

主业务数据：

- Agent Profile
- Skill
- Tool Registry
- Connector Registry
- Workflow
- Prompt / Policy
- Knowledge metadata
- Memory
- Audit
- Release records

推荐：

- MySQL 8.x

## 9.2 Redis

短期状态：

- session hot state
- recent turns
- context cache
- idempotency key
- execution lock
- config cache

## 9.3 Qdrant

向量索引：

- Knowledge chunks
- Long-term memory embedding
- query cache embedding 可选

## 9.4 对象存储

推荐：

- MinIO / S3

存储：

- 文档原文件
- 音频回放
- trace 附件
- 导入导出包

## 9.5 消息队列 / 异步任务

MVP 可选：

- Redis Stream
- MySQL job table

企业化可选：

- Kafka
- RabbitMQ
- NATS

用途：

- 文档索引任务
- embedding 批处理
- memory extraction
- async tool execution
- audit log pipeline

---

## 10. LangChain / LangGraph 的定位

## 10.1 官方定位简述

根据官方文档：

- LangChain 提供预构建 agent 架构、模型集成和工具集成，适合快速开始构建 LLM agent 和应用。
- LangGraph 是更底层的 agent orchestration framework 和 runtime，面向 long-running、stateful agents，强调 durable execution、streaming、human-in-the-loop 等能力。
- LangChain agents 构建在 LangGraph 之上，以复用 durable execution、human-in-the-loop、persistence 等能力。

参考：

- [LangChain overview](https://docs.langchain.com/oss/python/langchain/overview)
- [LangGraph overview](https://docs.langchain.com/oss/python/langgraph/overview)
- [LangGraph durable execution](https://docs.langchain.com/oss/python/langgraph/durable-execution)

## 10.2 我们的总体态度

不建议把 LangChain / LangGraph 作为 AI 中台的核心平台架构。

建议采用：

```text
核心平台抽象自研
LangChain / LangGraph 仅用于实验、参考或隔离 adapter
```

这不是否定 LangChain / LangGraph，而是因为本项目目标是企业级 AI 中台，不是单一 Agent 应用。

---

## 11. 不建议深度使用 LangChain 的原因

## 11.1 与 Go First 决策不匹配

LangChain 主要优势在 Python / JS 生态。  
在本项目中，核心在线服务已经明确采用 Go，因此深度使用 LangChain 会带来：

- 主链路跨语言调用复杂度
- Python 运行时运维成本
- 框架对象和平台资源模型之间的映射成本
- 线上问题排查和 trace 归因复杂度

## 11.2 平台资源模型需要自主管理

本项目核心产品对象包括：

- Agent Profile
- Skill
- Tool Set
- Tool
- Connector Operation
- Workflow
- Knowledge Base
- Prompt / Policy
- Release / Audit

这些对象需要：

- 页面配置
- 版本管理
- 审批发布
- 灰度
- 回滚
- 多租户
- 审计
- 运行时追溯

LangChain 的 chain、agent、tool、retriever 抽象更偏代码级开发，不应成为我们平台资源模型的来源。

## 11.3 我们需要严格区分 API、Connector Operation、Tool

本项目定义：

```text
External API
  ->
Connector Operation
  ->
Tool
  ->
Tool Set
  ->
Skill
  ->
Agent Profile
```

LangChain Tool 更接近直接可调用函数或能力。  
但在我们的平台里：

- Connector Service 负责 API 封装
- Tool Registry 负责 Tool 定义
- Agent Runtime 负责 Tool 执行治理
- AI Orchestrator 负责对话和推理控制

如果直接采用 LangChain Tool 作为核心抽象，容易把 API 封装、Tool 定义、执行治理混在一起。

## 11.4 企业治理能力必须由平台掌控

Tool 执行需要：

- 权限校验
- 风险等级
- 用户确认
- 幂等
- 超时
- 重试
- 审计
- 多租户隔离
- 发布版本追溯
- Tool Result 标准化

这些是 Agent Runtime、Tool Registry、Connector Service 的核心职责，不应由框架内部 Tool 调用逻辑隐式承担。

## 11.5 Context Engineering 和 Memory 需要强控制

本项目需要：

- Context Builder
- Token Budget
- Short-term Memory
- Long-term Memory
- Memory Candidate Extraction
- Memory Policy
- Memory Retrieval
- Context Injection

长期记忆需要：

- 用户可查看
- 用户可删除
- 来源可追溯
- scope 控制
- sensitivity
- confidence
- ttl
- 审计

框架内置 memory abstraction 可以参考，但不适合作为企业长期记忆核心模型。

---

## 12. 不建议深度使用 LangGraph 的原因

## 12.1 LangGraph 可以参考，但不适合作为 Go 主链路运行时

LangGraph 的优势在于：

- graph orchestration
- stateful execution
- durable execution
- human-in-the-loop
- streaming

这些能力与后期复杂 Workflow 需求有重叠。  
但本项目的核心运行时采用 Go，如果深度使用 LangGraph，就意味着核心任务执行链路需要长期依赖 Python Runtime。

这会削弱 Go First 带来的统一工程收益。

## 12.2 副作用控制仍然要靠平台

企业任务中大量步骤会产生副作用：

- 发邮件
- 创建工单
- 提交审批
- 修改业务数据
- 下单

即使底层流程引擎支持持久化和恢复，也不能替代业务幂等和审计设计。

这些必须由 Agent Runtime、Tool Executor、Connector Service、业务系统共同保证。

## 12.3 Workflow 定义需要产品化管理

我们的 Workflow 需要：

- 可视化管理
- 版本化
- 审批发布
- 与 Skill / Tool Set 绑定
- 与 Agent Profile 绑定
- 运行时 trace

LangGraph 的 graph 定义可以作为实现参考，但不能直接成为产品配置模型。

建议方式：

```text
Platform Workflow Definition
  ->
Go Workflow Engine Interface
  ->
Self-developed Engine / Temporal optional
```

如果后期需要成熟 durable workflow，更建议评估 `Temporal` 这类通用工作流基础设施，而不是把 Agent 框架直接放入平台核心。

---

## 13. Dify 的定位

Dify 可以作为产品形态和运营能力参考，但不建议直接作为本项目的核心底座。

适合参考：

- 应用配置体验
- Prompt 编排体验
- 知识库管理体验
- 工作流可视化体验
- 运营后台的信息架构

不建议直接采用为核心底座的原因：

- 平台资源模型、权限、审计、发布流程需要更贴合企业内部体系
- 与 Go First 自研运行时方向不一致
- 深度二次开发后，升级和维护成本可能高于自研关键内核
- 语音实时交互、会话状态机、企业 Connector 治理需要更强的定制能力

---

## 14. 推荐使用边界

## 14.1 可以使用 LangChain 的场景

- PoC
- 快速验证 Prompt 和 Tool Calling
- 简单 Agent demo
- 模型 adapter 参考
- 文档 loader / splitter 参考
- RAG 实验

## 14.2 可以参考 LangGraph 的场景

- 复杂 Workflow Engine 设计
- long-running task 状态模型
- human-in-the-loop 设计
- graph-based execution 设计
- 多节点执行状态管理

## 14.3 不建议使用的核心区域

- Tool Registry 核心模型
- Connector Operation 核心抽象
- Agent Runtime 治理主链路
- Memory Service 长期记忆模型
- Resource Registry
- 发布审批
- 审计回放
- Context Builder 核心逻辑
- AI Orchestrator 在线主链路

---

## 15. MVP 推荐技术组合

## 15.1 Go 服务

服务：

- AI Orchestrator
- Agent Runtime
- Connector Service
- Knowledge Service
- Memory Service
- Platform Management API
- Auth / Tenant / Audit

技术栈：

- `chi` / `gin`
- `grpc-go` / `connect-go`
- `go-playground/validator`
- `santhosh-tekuri/jsonschema`
- `sqlc` / `ent`
- `go-sql-driver/mysql`
- `go-redis`
- `net/http` / `resty`
- `zap` / `zerolog`
- `OpenTelemetry Go`
- `testify`

## 15.2 Python 辅助服务

服务：

- Python Tool Runner
- Document Worker
- Embedding Worker
- Eval Worker

技术栈：

- `uv` / `venv`
- `pypdf`
- `python-docx`
- `openpyxl`
- `beautifulsoup4`
- `pandas`
- `pytest`

说明：

- Python 服务必须以 worker / runner 形态存在
- 不直接承担核心会话状态和工具副作用提交
- 输出结果由 Go 主服务校验、审计和提交

## 15.3 前端

技术栈：

- React
- TypeScript
- Vite
- Ant Design
- TanStack Query
- React Router

## 15.4 存储与基础设施

推荐：

- MySQL 8
- Redis
- Qdrant
- MinIO / S3
- Docker
- Kubernetes
- OpenTelemetry
- Prometheus / Grafana

---

## 16. 总结

本项目的技术选型建议是：

```text
Go 承担 AI 中台核心在线服务和平台控制面
TypeScript / React 承担管理后台
Python 承担隔离脚本工具、文档处理、embedding 批处理和离线评测
MySQL / Redis / Qdrant / MinIO 承担数据底座
```

对 LangChain / LangGraph / Dify 的使用建议是：

```text
可以参考和局部使用
不作为 AI 中台核心平台架构
不让平台资源模型依赖框架对象
不让 Tool / Memory / Context / Release / Audit 被框架抽象主导
不让核心在线运行时依赖 Python Agent 框架
```

更稳妥的落地方式：

```text
自研 Go 平台协议和核心 Runtime
开源框架作为局部 adapter、离线实验工具或产品体验参考
```

这样既能利用开源生态的效率，又能保证企业级 AI 中台所需要的可控性、可运营性、可审计性和长期可维护性。
