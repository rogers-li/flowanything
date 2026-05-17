# Knowledge Service 详细设计文档

## 1. 文档目的

本文档用于定义智能语音交互机器人项目中的 `Knowledge Service` 详细设计，明确其在 AI Platform 中的职责、边界、内部模块、数据模型、检索与索引流程、权限过滤机制、技术选型以及与 AI Orchestrator、Agent Runtime 的协作方式。

本文档重点回答以下问题：

- Knowledge Service 在系统中的定位是什么
- 它与 AI Orchestrator、Agent Runtime、Connector Service 的边界如何划分
- 文档如何入库、切分、生成 embedding 并写入向量库
- 检索请求如何标准化，如何支持后续向量库迁移
- 如何做 metadata filter、租户隔离和权限控制
- 如何将检索结果标准化后返回给上层

本文档面向：

- 服务设计评审
- RAG 平台能力设计
- 向量检索与知识入库实施开发
- 与 AI Orchestrator、Agent Runtime 联调

---

## 2. 服务定位

## 2.1 服务角色

`Knowledge Service` 是 AI Platform 的知识检索与知识索引能力服务，负责将知识文档处理为可检索索引，并对外提供统一的 retrieval 能力。

它不是：

- 业务主数据真相源
- 用户对话决策层
- 通用业务系统接入层

它是：

- 文档知识入库服务
- embedding / chunk 索引服务
- retrieval / rerank 服务
- 知识权限过滤服务

## 2.2 核心职责

### 入库侧职责

- 管理知识库和文档元数据
- 执行文档切分
- 生成 embedding
- 将 chunk 与 metadata 写入向量库

### 检索侧职责

- 接收标准化检索请求
- 进行 metadata filter
- 调用向量检索
- 可选执行 rerank
- 返回标准化结果

### 治理侧职责

- 维护索引状态
- 维护文档与 chunk 映射
- 维护租户、知识库、权限标签过滤
- 支持向量库抽象和后续迁移

---

## 3. 在整体架构中的位置

```text
Document Sources / Admin Console
        ->
Knowledge Service
        ->
Embedding Pipeline
        ->
Vector DB (Qdrant)

AI Orchestrator / Agent Runtime
        ->
Knowledge Service Retrieval API
        ->
Vector DB + Rerank
        ->
Normalized Retrieval Result
        ->
AI Orchestrator / Agent Runtime
```

Knowledge Service 位于：

- 知识内容管理和向量索引之间
- AI Runtime 与向量数据库之间

一句话：

- `Knowledge Service` 负责“把知识变成可检索能力，并以统一方式提供给上层”

---

## 4. 设计目标

## 4.1 核心目标

### 4.1.1 统一知识检索能力

不让 AI Orchestrator 或 Agent Runtime 直接操作向量数据库，而是通过统一的 Knowledge Service 调用。

### 4.1.2 业务元数据与向量索引解耦

- MySQL 作为知识元数据真相源
- Qdrant 作为向量索引层

### 4.1.3 支持权限和多租户隔离

所有检索结果必须满足：

- tenant 过滤
- knowledge base 过滤
- metadata 过滤
- 权限标签过滤

### 4.1.4 可迁移

通过 Retrieval Abstraction Layer 降低未来从 Qdrant 迁移到 Milvus 等向量库的成本。

### 4.1.5 可评测

支持召回评测、检索回放、chunk 调优、embedding 策略优化。

## 4.2 非目标

首期不追求：

- 全自动知识治理
- 图谱级知识组织
- 跨模态统一检索
- 非结构化海量数据平台化治理全覆盖

---

## 5. 服务边界

## 5.1 与 AI Orchestrator 的边界

AI Orchestrator 负责：

- 判断是否需要知识检索
- 组织 retrieval request
- 将 retrieval result 融入回答和动作决策

Knowledge Service 负责：

- 检索
- rerank
- 结果标准化

## 5.2 与 Agent Runtime 的边界

Knowledge Service 可以被视为一种内部平台能力。

建议：

- 对话问答类知识检索，可由 AI Orchestrator 直接调用
- 复杂任务工作流中的检索步骤，也可由 Agent Runtime 统一调用

无论哪种路径，都建议走同一套 retrieval API。

当知识检索需要作为 Agent/LLM 可调用能力时，应在 Tool Registry 中定义 Tool，例如 `search_knowledge_base`，并将其 execution binding 设置为 `knowledge_retrieval`。

此时：

- Tool Registry 负责定义 Tool 名称、描述、参数 schema、风险等级和绑定关系
- Agent Runtime 负责执行该 Tool
- Knowledge Service 负责实际 retrieval / rerank / metadata filtering

Knowledge Service 不负责生成 Tool，也不直接向 LLM 暴露内部检索 API。

## 5.3 与 Connector Service 的边界

Knowledge Service 不走 Connector Service。  
原因：

- 它是平台内部核心能力，不是外部业务系统适配
- 它需要更紧密地和 embedding、chunk、vector index 协同

## 5.4 与 MySQL 的边界

MySQL 中保存：

- knowledge base 元数据
- document 主记录
- chunk 主索引
- 权限和标签

Qdrant 中保存：

- embedding
- chunk payload metadata

Qdrant 是检索索引层，不是主数据真相源。

---

## 6. 能力范围

## 6.1 首期支持能力

1. 知识库创建与管理
2. 文档导入
3. 文档切分
4. embedding 生成
5. 写入 Qdrant
6. metadata filter 检索
7. 标准化 retrieval result
8. 简单 rerank

## 6.2 后续演进能力

1. 混合检索
2. 多 embedding 模型共存
3. 多向量字段检索
4. 多阶段召回与 rerank
5. 自动索引重建
6. 检索质量评测平台

---

## 7. 核心概念设计

## 7.1 Knowledge Base

表示一组具备统一用途和权限边界的知识集合。

字段建议包括：

- `kb_id`
- `tenant_id`
- `kb_name`
- `kb_type`
- `locale`
- `status`
- `permission_policy`

## 7.2 Document

表示一个知识原文档。

字段建议包括：

- `doc_id`
- `tenant_id`
- `kb_id`
- `title`
- `source_type`
- `source_uri`
- `language`
- `status`
- `checksum`
- `updated_at`

## 7.3 Chunk

表示文档切分后的最小检索单元。

字段建议包括：

- `chunk_id`
- `doc_id`
- `tenant_id`
- `kb_id`
- `chunk_text`
- `chunk_order`
- `token_count`
- `embedding_model`
- `permission_tags`
- `source_type`
- `language`

## 7.4 Retrieval Request

表示上层发起的统一检索请求。

示例：

```json
{
  "request_id": "ret_001",
  "tenant_id": "t_001",
  "kb_ids": ["kb_001"],
  "query_text": "如何重置密码",
  "top_k": 5,
  "filters": {
    "language": "zh-CN",
    "source_type": "faq"
  },
  "permission_context": {
    "user_id": "u_456",
    "allowed_tags": ["public", "internal_support"]
  },
  "search_mode": "vector"
}
```

## 7.5 Retrieval Result

表示返回给上层的标准化检索结果。

示例：

```json
{
  "request_id": "ret_001",
  "status": "success",
  "results": [
    {
      "chunk_id": "chunk_001",
      "doc_id": "doc_001",
      "score": 0.91,
      "text": "进入设置页面后点击重置密码。",
      "metadata": {
        "title": "账号帮助",
        "source_type": "faq",
        "language": "zh-CN"
      }
    }
  ],
  "error": null
}
```

---

## 8. 内部模块设计

## 8.1 模块总览

建议拆分为以下模块：

1. `Knowledge API`
2. `Knowledge Base Manager`
3. `Document Ingestion Manager`
4. `Chunker`
5. `Embedding Worker`
6. `Index Writer`
7. `Retrieval Engine`
8. `Rerank Module`
9. `Permission Filter`
10. `Vector Store Adapter`
11. `Index State Manager`
12. `Evaluation Hook`

处理链路如下：

```text
Document Input
    ->
Document Ingestion Manager
    ->
Chunker
    ->
Embedding Worker
    ->
Index Writer
    ->
Vector Store Adapter (Qdrant)

Retrieval Request
    ->
Permission Filter
    ->
Retrieval Engine
    ->
Rerank Module
    ->
Result Normalization
    ->
Retrieval Result
```

## 8.2 Knowledge API

### 职责

- 提供文档入库接口
- 提供检索接口
- 提供索引状态查询接口

### 推荐接口

- `POST /v1/knowledge/documents`
- `POST /v1/knowledge/retrieve`
- `GET /v1/knowledge/index-jobs/{job_id}`

## 8.3 Knowledge Base Manager

### 职责

- 管理知识库元数据
- 校验租户归属
- 管理知识库状态

## 8.4 Document Ingestion Manager

### 职责

- 接收文档原文或文档引用
- 计算 checksum
- 判断是否需要重新入库
- 生成 indexing job

## 8.5 Chunker

### 职责

- 对文档做切分
- 保持 chunk 语义完整性
- 控制 chunk 大小和重叠策略

### 推荐策略

- 先按结构切分
- 再按 token 长度切分
- 必要时带 overlap

### 可配置参数

- `max_tokens_per_chunk`
- `overlap_tokens`
- `split_strategy`

## 8.6 Embedding Worker

### 职责

- 调用 embedding 模型
- 生成向量
- 为 chunk 添加 embedding_model 标识

### 设计建议

- 独立 worker 化
- 支持批量处理
- 支持重试和失败回补

## 8.7 Index Writer

### 职责

- 将 chunk + embedding + payload 写入向量库
- 维护写入状态
- 支持更新和删除

## 8.8 Retrieval Engine

### 职责

- 接收标准化 retrieval request
- 生成向量检索请求
- 调用 Vector Store Adapter

### 搜索模式建议

- `vector`
- `hybrid`
- `keyword_only`（后续）

## 8.9 Rerank Module

### 职责

- 对初步召回结果进行 rerank
- 提升精度

### 首期建议

- 可选开关
- 先支持 topN rerank

## 8.10 Permission Filter

### 职责

- 基于 tenant、kb_id、language、source_type、permission_tags 做过滤
- 确保不召回越权内容

### 原则

- 过滤尽量前置到检索阶段
- 不应只在生成阶段做权限兜底

## 8.11 Vector Store Adapter

### 职责

- 隔离 Qdrant 具体 SDK 和检索 DSL
- 提供统一检索和索引接口

### 这是降低未来迁移成本的关键模块

上层只依赖：

- `upsert_chunks`
- `delete_chunks`
- `search`
- `health_check`

不直接依赖 Qdrant SDK。

## 8.12 Index State Manager

### 职责

- 管理 indexing job 状态
- 支持失败重试
- 管理全量重建和增量更新状态

## 8.13 Evaluation Hook

### 职责

- 为后续离线评测、召回分析、回放提供埋点

---

## 9. 输入输出协议设计

## 9.1 文档入库接口

### 请求示例

```json
{
  "tenant_id": "t_001",
  "kb_id": "kb_001",
  "document": {
    "doc_id": "doc_001",
    "title": "账号帮助",
    "source_type": "faq",
    "language": "zh-CN",
    "content": "用户可在设置页点击重置密码。",
    "permission_tags": ["public"]
  }
}
```

### 响应示例

```json
{
  "job_id": "idx_job_001",
  "status": "queued"
}
```

## 9.2 检索接口

### 请求示例

```json
{
  "request_id": "ret_001",
  "tenant_id": "t_001",
  "kb_ids": ["kb_001"],
  "query_text": "如何重置密码",
  "top_k": 5,
  "filters": {
    "language": "zh-CN",
    "source_type": "faq"
  },
  "permission_context": {
    "user_id": "u_456",
    "allowed_tags": ["public"]
  },
  "search_mode": "vector"
}
```

### 响应示例

```json
{
  "request_id": "ret_001",
  "status": "success",
  "results": [
    {
      "chunk_id": "chunk_001",
      "doc_id": "doc_001",
      "score": 0.91,
      "text": "用户可在设置页点击重置密码。",
      "metadata": {
        "title": "账号帮助",
        "source_type": "faq",
        "language": "zh-CN"
      }
    }
  ],
  "error": null
}
```

---

## 10. 数据模型设计

## 10.1 MySQL 数据范围

建议在 MySQL 中保存：

- knowledge base
- document
- chunk 主索引
- permission tags
- indexing job
- embedding model metadata

## 10.2 Qdrant 数据范围

建议在 Qdrant 中保存：

- chunk vector
- chunk payload metadata

### 建议 payload 字段

- `chunk_id`
- `doc_id`
- `tenant_id`
- `kb_id`
- `language`
- `source_type`
- `permission_tags`
- `updated_at`
- `embedding_model`

## 10.3 一致性原则

- MySQL 是真相源
- Qdrant 是可重建索引

因此：

- 文档修改 -> 触发重新切分/重建索引
- 文档删除 -> 删除索引并更新 MySQL 状态
- 向量库损坏 -> 可通过 MySQL 全量重建

---

## 11. 检索抽象层设计

## 11.1 目标

防止业务代码和 AI Runtime 直接依赖 Qdrant DSL。

## 11.2 抽象接口建议

```text
retrieve(request) -> retrieval_result
upsert_chunks(chunks)
delete_chunks(chunk_ids)
rebuild_index(kb_id)
```

## 11.3 检索请求抽象

统一字段包括：

- `tenant_id`
- `kb_ids`
- `query_text`
- `top_k`
- `filters`
- `permission_context`
- `search_mode`

这样未来迁移到 Milvus 时，主要改动集中在 Adapter 层。

---

## 12. 索引流程设计

## 12.1 入库流程

```text
1. 接收文档
2. 校验 tenant/kb/document
3. 计算 checksum
4. 生成 indexing job
5. 文档切分
6. 生成 embedding
7. 写入 MySQL chunk 索引
8. 写入 Qdrant
9. 更新 indexing job 状态
```

## 12.2 更新流程

```text
1. 文档内容变更
2. 标记旧 chunk 失效
3. 重新切分与 embedding
4. upsert 新 chunk
5. 清理旧 chunk
```

## 12.3 删除流程

```text
1. 标记文档删除
2. 删除 Qdrant 对应 chunk
3. 更新 MySQL 状态
```

---

## 13. 检索流程设计

## 13.1 标准检索流程

```text
1. 接收 retrieval request
2. 校验 tenant/kb/filter
3. 生成 query embedding
4. 构造 metadata filter
5. 调用 vector adapter 检索
6. 可选 rerank
7. 结果标准化
8. 返回 retrieval result
```

## 13.2 权限过滤流程

必须至少过滤：

- tenant_id
- kb_id
- language（如有）
- source_type（如有）
- permission_tags

## 13.3 结果返回建议

除文本内容外，建议同时返回：

- title
- source_type
- doc_id
- chunk_id
- score
- 关键 metadata

便于上层：

- 做引用
- 做可追踪回答
- 做后续 UI 展示

---

## 14. 权限与多租户设计

## 14.1 多租户原则

不同 tenant 的数据必须强隔离。

建议：

- MySQL 记录 tenant 归属
- Qdrant payload 必带 tenant_id
- 检索时必须强制 tenant filter

## 14.2 权限标签

建议文档或 chunk 具备：

- `public`
- `internal`
- `restricted`

上层传入 `allowed_tags`，Knowledge Service 在检索阶段过滤。

## 14.3 为什么检索阶段必须过滤

因为如果先召回、后过滤，会带来：

- 性能浪费
- 结果集失真
- 潜在越权风险

---

## 15. 错误处理设计

## 15.1 错误类型

- `kb_not_found`
- `document_not_found`
- `invalid_request`
- `embedding_failed`
- `index_write_failed`
- `retrieval_failed`
- `permission_denied`
- `vector_store_unavailable`
- `rerank_failed`
- `internal_error`

## 15.2 错误结构建议

```json
{
  "code": "retrieval_failed",
  "message": "Vector search failed",
  "retryable": true,
  "details": {}
}
```

## 15.3 降级策略

### rerank 失败

- 可降级返回原始召回结果

### 向量检索失败

- 返回结构化错误
- 由上层决定是否回退到无检索回答或人工兜底

---

## 16. 可观测性与评测设计

## 16.1 关键日志

- indexing job created
- chunk count
- embedding latency
- vector write result
- retrieval request summary
- filter summary
- recall count
- rerank result
- retrieval latency

## 16.2 核心指标

- indexing job success rate
- average chunks per doc
- embedding latency
- vector upsert success rate
- retrieval latency
- rerank latency
- recall hit count
- permission filter reject rate

## 16.3 评测建议

后续应支持：

- Recall@K
- MRR
- NDCG
- 多语言召回效果
- 权限过滤准确率

---

## 17. 技术选型与语言建议

## 17.1 推荐语言

- `Go`

## 17.2 原因

- Knowledge Service 的在线检索链路需要稳定、低延迟、可观测和统一治理，适合使用 Go 实现
- Go 可以很好地承载 Retrieval API、metadata filter、Qdrant adapter、权限过滤、rerank adapter 和索引状态管理
- Python 生态仍可用于文档解析、embedding 批处理、离线评测、chunk 策略实验等异步 worker，但不作为在线 Knowledge Service 主语言
- 这样既能保持在线链路 Go First，又保留 Python 在 AI 数据处理上的效率

## 17.3 向量数据库

- 初期推荐 `Qdrant`

原因：

- payload filter 直观
- 多租户和 metadata filter 适合当前项目
- 与 MySQL 解耦清晰

## 17.4 推荐技术栈

- `chi` 或 `gin`
- `grpc-go` / `connect-go`
- `qdrant/go-client` 或自研 Qdrant HTTP adapter
- `sqlc` 或 `ent`
- `go-sql-driver/mysql`
- `go-redis`
- `go-playground/validator`
- `zap` 或 `zerolog`
- `OpenTelemetry Go`
- `testify`
- Python worker 可选：文档解析、embedding 批处理、离线 eval

## 17.5 代码组织建议

```text
knowledge_service/
  api/
  schemas/
  kb/
  ingestion/
  chunking/
  embedding/
  retrieval/
  rerank/
  vector_store/
  storage/
  evaluation/
  tests/
```

---

## 18. MVP 范围

首期建议实现：

1. Knowledge Base 管理最小能力
2. 单文档入库
3. 基础 chunking
4. embedding 生成
5. Qdrant upsert
6. vector 检索
7. metadata filter
8. retrieval result 标准化
9. indexing job 状态查询

MVP 暂不要求：

- 混合检索
- 多阶段 rerank
- 多 embedding 模型路由
- 大规模自动重建调度

---

## 19. 实施顺序建议

建议按以下顺序实施：

1. 定义 KB / Document / Chunk / Retrieval Schema
2. 实现 Knowledge API
3. 实现 Document Ingestion Manager
4. 实现 Chunker
5. 实现 Embedding Worker
6. 实现 Vector Store Adapter
7. 实现 Retrieval Engine
8. 实现 Permission Filter
9. 实现 Index State Manager
10. 接入第一个真实知识库

---

## 20. 总结

Knowledge Service 是 AI Platform 的知识能力底座，其核心价值在于：

- 让知识内容成为可控、可检索、可治理的能力
- 将业务元数据与向量索引合理解耦
- 为 AI Orchestrator 和 Agent Runtime 提供统一的 retrieval 能力

在项目推进上，Knowledge Service 应作为 AI Platform 第四个核心服务尽快落地，因为它直接决定了：

- 企业知识问答能力是否可用
- RAG 能力是否平台化
- 向量检索是否可治理、可迁移、可评测
