# 智能语音交互机器人项目详细设计文档

## 1. 文档目的

本文档基于前期技术调研结论，输出项目级详细设计方案，面向以下目标：

- 明确整体架构设计
- 明确系统拆分方式
- 明确需要研发的服务与模块
- 明确各模块职责边界
- 明确推荐技术选型、开发语言和存储方案
- 为后续项目立项、团队拆分、技术评审和排期提供依据

本文档默认采用以下基础判断：

- 业务主数据使用 `MySQL`
- 缓存与短期状态使用 `Redis`
- 向量检索使用独立向量数据库，初期推荐 `Qdrant`
- 项目采用“文本智能核心可复用，语音运行时独立建设”的总体路线
- 开发语言采用 `Go First` 方案：核心在线服务和平台控制面优先使用 Go，Python 仅用于脚本工具执行、文档处理、离线评测等辅助 worker

---

## 2. 设计原则

### 2.1 渠道无关的智能核心

文本、语音、电话、IM、车机等不同渠道，共享同一个智能核心运行时。

核心系统不应设计为：

`Text -> LLM -> Text`

而应设计为：

`Event -> Runtime -> Action`

### 2.2 任务状态与交互状态分离

必须严格区分：

- `Task State`：业务任务推进状态
- `Interaction State`：交互态、说话态、播放态、等待态

前者跨渠道复用，后者由语音运行时主导。

### 2.3 内部执行状态与用户可见状态分离

系统内部可能有：

- plan
- tool trace
- reasoning state
- workflow step

但面向用户时应投影成：

- 文本端可展开状态
- 语音端可播报摘要

### 2.4 存储与检索解耦

- `MySQL` 作为业务真相源
- `Qdrant` 作为检索索引层
- 文档切片、embedding、metadata 是索引副本，不是主数据

### 2.5 小模型优先，低置信度自动升级

语音场景下不应所有请求都走重模型。  
推荐采用：

- 规则 / NLP / 小模型做快速路由
- 复杂请求走重模型
- 高风险请求附带确认机制

### 2.6 先可用、再低延迟、再极致拟人

第一阶段目标应是“可用且稳定”，不建议一开始追求：

- 全双工
- 完全自治多 Agent
- 全语言无差别支持
- 高拟人化连续对话

---

## 3. 总体架构

## 3.1 高层逻辑架构

```text
Client / Device
    ->
Realtime Gateway
    ->
Speech Processing Layer
    ->
Session Manager
    ->
Core Conversation Runtime
    ->
Agent / Tool / Workflow Layer
    ->
Presentation Layer
    ->
TTS / UI Rendering
    ->
Client / Device
```

## 3.2 高层服务架构

```text
[Web / App / Hardware Client]
        |
        v
[Realtime Gateway Service]
        |
        v
[Speech Service]
  - ASR
  - VAD fallback
  - endpointing
  - TTS
        |
        v
[Session Service]
  - conversation session
  - active task resolution
  - timeout / resume
        |
        v
[AI Orchestrator Service]
  - event normalization
  - policy / routing
  - action planning
  - response planning
        |
        +--------------------------+
        |                          |
        v                          v
[Agent Runtime Service]      [Knowledge Service]
  - tool call                  - retrieval
  - workflow                   - rerank
  - confirmation               - knowledge indexing metadata
        |                          |
        v                          v
[Connector Service]          [Vector DB: Qdrant]
        |
        v
[Business Systems / APIs]

[MySQL]
  - users
  - sessions metadata
  - tasks
  - tools metadata
  - audit logs index
  - knowledge metadata

[Redis]
  - active session state
  - short-term context
  - idempotency keys
  - transient routing state
```

---

## 4. 核心交互链路设计

## 4.1 语音问答链路

```text
用户说话
    ->
客户端 VAD / 音频采集
    ->
Realtime Gateway
    ->
Speech Service(ASR)
    ->
Session Service 归属会话 / 任务
    ->
AI Orchestrator
    ->
生成动作 actions
    ->
Presentation Layer 生成 spoken_text
    ->
Speech Service(TTS)
    ->
客户端播放
```

## 4.2 任务执行链路

```text
用户语音输入
    ->
ASR final transcript
    ->
Router 判断简单路径 / 复杂路径
    ->
AI Orchestrator 生成行动计划
    ->
先返回 speak 动作（占位反馈）
    ->
Agent Runtime 调用 Tools / Workflow
    ->
Connector Service 调业务系统
    ->
结果回传 AI Orchestrator
    ->
生成结果摘要 + details
    ->
TTS 播报 / UI 展示
```

## 4.3 用户打断链路

```text
TTS 正在播放
    ->
客户端检测到用户说话
    ->
上报 interrupt event
    ->
Realtime Gateway 通知停止播放
    ->
Session Service 更新 interaction state
    ->
重新进入 listening / understanding
```

---

## 5. 研发服务清单

建议首版按以下服务拆分。  
如果团队规模较小，可先模块化单体部署，再逐步独立。

## 5.1 Realtime Gateway Service

### 职责

- 提供 WebSocket / WebRTC 接入
- 管理音频流上传与下发
- 管理实时连接生命周期
- 支持鉴权、限流、设备标识、心跳
- 接收客户端中断事件
- 负责 TTS 音频流向客户端回传

### 输入

- 客户端音频流
- 客户端控制事件

### 输出

- 向 Speech Service 转发音频
- 向客户端回传音频流与控制事件

### 推荐语言

- `Go` 优先

### 选择原因

- 并发连接和低延迟音频流处理更适合 Go
- 对你现有技术栈学习成本最低

### 推荐技术

- WebSocket
- WebRTC（后续演进）
- gRPC / HTTP 内部调用
- OpenTelemetry

---

## 5.2 Speech Service

### 职责

- 封装 ASR
- 服务端兜底 VAD
- endpointing
- 语言识别
- TTS 合成与流式输出
- 语音文本预处理

### 子模块

- `asr-adapter`
- `tts-adapter`
- `vad-fallback`
- `endpoint-detector`
- `speech-normalizer`

### 输入

- 音频流
- spoken_text / display_text

### 输出

- partial transcript
- final transcript
- TTS 音频流

### 推荐语言

- `Go`

### 建议

- 外部云 ASR/TTS 接入、流式转发、endpointing、状态协调优先使用 Go
- 若后续加入复杂语音算法后处理、文本规范化实验，可作为独立 Python worker 或实验模块接入

### 当前推荐

- 首期可用 `Go` 实现适配层
- 复杂语音后处理脚本和实验不进入主链路，可由 Python worker 辅助

### 推荐技术

- 外部 ASR/TTS SDK
- 流式接口
- 文本规范化模块

---

## 5.3 Session Service

### 职责

- 管理连接会话、交互会话、任务会话
- 维护 active task
- 维护会话超时和恢复
- 处理 `conversation_reset_requested`
- 归并新 utterance 到现有上下文

### 核心状态

- `idle`
- `listening`
- `engaged`
- `waiting_user`
- `executing_task`
- `suspended`
- `closed`

### 核心能力

- 30 秒内默认续接当前交互会话
- 若存在 awaiting_confirmation / awaiting_slot 则优先续接
- 静默超时关闭交互会话
- 保留任务状态以支持恢复

### 推荐语言

- `Go`

### 原因

- 这是高可靠状态协调模块，Go 更适合和 Realtime Gateway 紧密协作
- 与 AI 推理逻辑解耦，便于长期稳定运行

### 依赖存储

- Redis：活跃交互态
- MySQL：会话元数据、任务索引

---

## 5.4 AI Orchestrator Service

### 职责

- 接收标准化事件
- 加载上下文和 task state
- 调用 Router / Policy / LLM
- 生成 actions
- 调用 Presentation Layer 生成渠道化表达
- 更新任务状态

### 这是整个系统的智能核心入口

其内部不直接面向“语音”或“文本”，而面向事件和动作。

### 内部模块

- `event-normalizer`
- `context-loader`
- `policy-engine`
- `llm-router`
- `action-planner`
- `response-planner`
- `state-updater`

### 推荐语言

- `Go`

### 原因

- AI Orchestrator 是在线主链路控制中心，需要稳定、低延迟、可观测、易部署
- Go 更符合团队现有工程经验，可以降低核心平台的学习成本和长期维护成本
- LLM、Prompt、Context、Tool 调度可以通过自研抽象和 Model Gateway 接入，不必依赖 Python 框架承载核心运行时
- Python 仍可用于离线 prompt 实验、评测脚本和数据分析，但不进入核心在线编排链路

### 推荐技术

- `chi` / `gin`
- `grpc-go` / `connect-go`
- `go-playground/validator`
- `santhosh-tekuri/jsonschema`
- `sqlc` / `ent`
- `go-redis`
- `OpenTelemetry Go`
- 配置中心 / prompt versioning

---

## 5.5 Router / Policy Module

### 职责

- 快速判断请求应该走哪条链路
- 区分：
  - 设备控制
  - 会话控制
  - 简单问答
  - 复杂任务
  - 高风险操作
  - 不确定请求

### 路由输出

- `rule_path`
- `small_model_path`
- `heavy_model_path`
- `agent_path`

### 推荐实现位置

- 作为 AI Orchestrator 内部模块首期实现
- 后续复杂后可独立服务化

### 推荐语言

- `Go`

### 原因

- Router / Policy 属于在线决策链路，应和 AI Orchestrator 共享 schema、trace、配置发布和灰度机制
- 规则、阈值、风险等级、模型路由策略都可以平台化配置，由 Go 运行时执行
- 如需小模型或 LLM 参与分类，通过 Model Gateway 调用，不要求 Router 本身使用 Python

---

## 5.6 Agent Runtime Service

### 职责

- 执行 tool call
- 多步骤工作流编排
- 参数校验
- 重试、超时、幂等控制
- 高风险操作确认
- 工具结果标准化

### 子模块

- `tool-executor`
- `workflow-engine`
- `confirmation-manager`
- `result-normalizer`

### 推荐语言

- `Go`

### 原因

- Agent Runtime 是工具执行、工作流状态、确认、幂等、超时、审计的强治理中心，工程稳定性比框架生态更重要
- Go 更适合承载高并发 Tool 调用、长任务状态机和企业级审计链路
- Python 脚本类 Tool 不由 Agent Runtime 直接内嵌执行，而是通过独立的 `Python Tool Runner` 沙箱执行

### 中长期演进

- 若工作流复杂度显著上升，可引入独立 Workflow Engine
- 若需要 Python / MCP / Shell 等异构 Tool，实现为 Tool Runner Adapter，而不是改变 Agent Runtime 主语言

---

## 5.7 Connector Service

### 职责

- 对接内部业务系统和外部 API
- 封装统一 connector 接口
- 屏蔽不同系统的鉴权、重试、协议差异

### 适配对象

- CRM / ERP / 工单
- 日历 / 邮件 / IM
- 订单 / 物流 / 支付
- 企业内部服务

### 推荐语言

- `Go`

### 原因

- Connector Service 在本项目中既要封装外部业务 API，也要暴露可工具化的 Connector Operation，和 Agent Runtime、Tool Registry 的协议一致性非常重要
- Go 可以统一 AI 中台在线服务的部署、观测、限流、熔断、审计和 schema 校验方式
- 如果个别外部系统只有成熟 Java SDK，可独立实现 Java Adapter / Sidecar，由 Go Connector Service 统一纳管

---

## 5.8 Knowledge Service

### 职责

- 文档切分
- embedding 生成
- metadata 管理
- 检索
- rerank
- 检索权限控制

### 子模块

- `document-ingestion`
- `chunker`
- `embedding-worker`
- `retrieval-api`
- `rerank-module`
- `metadata-sync`

### 推荐语言

- 在线检索服务：`Go`
- 离线文档处理 worker：`Python` 可选

### 原因

- Knowledge Service 的在线 retrieval API 位于主请求链路，建议用 Go 保证稳定性、低延迟和统一治理
- 文档解析、复杂格式抽取、embedding 批处理、rerank 评测等离线任务可以使用 Python worker，以利用 AI / 数据处理生态
- 在线服务与离线 worker 通过 MySQL、对象存储、消息队列和向量库解耦

### 存储依赖

- MySQL：知识库、文档、权限元数据
- Qdrant：chunk embedding、payload metadata

---

## 5.9 Evaluation & Replay Service

### 职责

- 对话日志聚合
- 错误归因
- 评测集管理
- 回放单次会话链路
- 线上/离线 AB 对比

### 推荐语言

- 回放 API 与 Trace 查询：`Go`
- 离线评测与数据分析：`Python` 可选

### 原因

- 回放和审计查询属于平台能力，建议和核心 trace schema 一起用 Go 实现
- 大规模评测、统计分析、报表生成可以由 Python worker 或 notebook 辅助完成

### 功能建议

- 可查看原始音频
- ASR partial/final
- Router 决策
- LLM 输入输出
- Tool trace
- TTS 文本
- 用户打断时序

---

## 5.10 Admin / Console Service

### 职责

- 运营后台
- 配置管理
- Prompt / 策略版本管理
- 知识库管理
- 工具管理
- 灰度与实验配置
- 监控入口

### 推荐语言

- 后端 `Go`
- 前端 `TypeScript + React`

---

## 5.11 Auth / Tenant / Audit Service

### 职责

- 统一身份认证
- 多租户隔离
- RBAC
- 工具级权限控制
- 高风险操作审计
- 会话与任务的审计索引

### 推荐语言

- `Go`

### 原因

- Auth / Tenant / Audit 是 AI 中台的基础治理能力，和 Tool 执行、Connector 调用、Knowledge 检索强相关
- 使用 Go 可以统一鉴权中间件、审计事件模型、租户隔离策略和服务治理方式
- 企业 SSO / IAM 可通过标准 OAuth2 / OIDC / SAML adapter 接入，不要求核心服务使用 Java

---

## 6. 模块级架构设计

## 6.1 文本智能核心 Runtime 模块

推荐输入输出模型：

### 输入 Event

- `user_message_committed`
- `user_utterance_committed`
- `user_interrupt_detected`
- `tool_result_received`
- `tool_failed`
- `timeout_occurred`
- `conversation_reset_requested`

### 输出 Action

- `speak`
- `display_text`
- `ask_question`
- `ask_confirmation`
- `call_tool`
- `wait`
- `handoff_human`
- `end_turn`

### Task State

- `idle`
- `understanding`
- `awaiting_slot`
- `awaiting_confirmation`
- `executing_tool`
- `awaiting_tool_result`
- `responding`
- `completed`
- `failed`

### 推荐内部结构

```text
Event Normalization
    ->
State Load
    ->
Policy / Routing
    ->
LLM Reasoning
    ->
Action Planning
    ->
State Commit
    ->
Action Output
```

---

## 6.2 Presentation Layer 模块

### 目标

解决“文本能展示很多信息、语音只能播摘要”的差异。

### 职责

- 为文本端生成细节丰富输出
- 为语音端生成口语化、短句化输出
- 将复杂结构化结果拆成：
  - `summary`
  - `details`
  - `next_choices`

### 推荐策略

- 文本端使用 `details`
- 语音端优先使用 `summary`
- 对长结果增加“要不要继续听详情”的追问

### 推荐语言

- `Go`

---

## 6.3 会话管理模块

### 三层会话模型

- `connection session`
- `interaction session`
- `task session`

### 核心规则

1. 连接断开不一定代表任务结束
2. 交互会话超时关闭，但任务会话可保留
3. 新 utterance 需要判断归属哪个 active task
4. 用户可通过“重新开始”“换个话题”显式切换

### 推荐存储

- Redis 存活跃态
- MySQL 存交互会话元数据和任务会话索引

---

## 6.4 混合模型模块

### 模型职责划分

- `Rules / NLP`
  - 设备控制
  - 会话控制
  - yes/no
  - 安全守卫

- `Small Model`
  - 快速路由
  - 首响应
  - 简单问答
  - 摘要改写

- `Heavy Model`
  - 复杂理解
  - 多轮推理
  - 多工具规划
  - 高风险决策

### 推荐策略

- 简单明确请求：小模型
- 模糊、多约束、高风险：重模型
- 小模型低置信度：升级
- 语音场景先快速反馈，再重模型后台处理

---

## 7. 数据架构设计

## 7.1 MySQL 使用范围

用于存储主业务数据：

- 用户与租户
- 会话元数据
- 任务状态索引
- 工具定义与配置
- 权限与审计索引
- 知识库元数据
- 文档主记录
- Prompt / 策略版本

## 7.2 Redis 使用范围

用于存储短期状态：

- 活跃交互会话
- 近期上下文摘要缓存
- 当前播放状态
- 幂等键
- 临时路由结果
- 限流计数

## 7.3 Qdrant 使用范围

用于存储检索索引数据：

- 文档切片向量
- embedding
- chunk payload metadata

建议 payload 字段至少包括：

- `chunk_id`
- `doc_id`
- `tenant_id`
- `kb_id`
- `lang`
- `source_type`
- `permission_tags`
- `updated_at`
- `embedding_model`

## 7.4 向量库抽象要求

为控制后续迁移成本，应用层必须引入 `Retrieval Abstraction Layer`，而不是直接在业务代码里绑定 Qdrant SDK。

建议定义统一请求结构：

```json
{
  "tenant_id": "t1",
  "kb_id": "kb1",
  "query_text": "如何重置密码",
  "top_k": 10,
  "filters": {
    "lang": "zh-CN",
    "source_type": "faq"
  },
  "search_mode": "hybrid"
}
```

这样未来迁移到 Milvus 的成本可控。

---

## 8. 技术选型总表

| 层级 | 服务/模块 | 推荐技术 | 推荐语言 | 说明 |
| --- | --- | --- | --- | --- |
| 客户端 | Web/App/Hardware | React / Flutter / Native | TypeScript / Dart / Native | 负责采集和播放 |
| 接入层 | Realtime Gateway | WebSocket / WebRTC | Go | 高并发、低延迟 |
| 语音层 | Speech Service | 流式 ASR/TTS SDK | Go | 首期 Go，复杂语音实验可用 Python worker |
| 会话层 | Session Service | Redis + MySQL | Go | 交互状态与任务归属 |
| 智能核心 | AI Orchestrator | chi/gin + gRPC + Model Gateway | Go | LLM/Prompt/Action 核心 |
| 路由层 | Router / Policy | Rule Engine + Small Model Adapter | Go | 小模型优先，模型调用走网关 |
| 执行层 | Agent Runtime | Tool Executor / Workflow | Go | Tool 执行治理主战场 |
| 集成层 | Connector Service | HTTP/gRPC/SDK Adapter | Go | 接企业系统，Java SDK 可做 sidecar |
| 检索层 | Knowledge Service | Retrieval API + Qdrant Adapter | Go | 在线 RAG 检索，离线处理可用 Python worker |
| 向量层 | Vector DB | Qdrant | N/A | 初期首选 |
| 主存储 | Business DB | MySQL | N/A | 业务真相源 |
| 缓存 | State Cache | Redis | N/A | 短期状态 |
| 评测层 | Eval/Replay | Trace API + Eval Worker | Go + Python 可选 | 回放用 Go，离线分析可用 Python |
| 后台 | Admin Console | React + Go API | TS + Go | 配置、管理、运营 |
| 安全 | Auth/Tenant/Audit | OAuth2/JWT/RBAC | Go | 企业治理 |

---

## 9. Go First 开发语言分工建议

## 9.1 总体分工

推荐采用：

- `Go`：核心在线服务、平台控制面、实时链路、Agent Runtime、Connector、Knowledge 在线检索、Auth/Audit
- `TypeScript`：管理后台、配置运营界面、可视化调试台
- `Python`：Python Code Tool、沙箱脚本执行、文档解析、embedding 批处理、离线评测、数据分析

## 9.2 为什么核心服务采用 Go

原因：

- 你在 Go / Java 上已有成熟经验
- AI 中台的核心难点不是“调用 LLM SDK”，而是资源模型、状态机、权限、审计、灰度、回放、幂等和可观测性
- Go 更适合承载高并发、低延迟、强治理的在线运行时
- 统一 Go 技术栈可以降低服务治理、部署运维、代码规范和招聘协作成本
- LLM、Embedding、Rerank、ASR/TTS 都可以通过标准 HTTP/gRPC adapter 接入，不需要核心服务跟随 Python 框架生态

## 9.3 Python 的保留边界

Python 仍然有价值，但应放在边界清晰的位置：

- `Python Tool Runner`：执行平台上配置的 Python 脚本类 Tool
- `Document Worker`：处理 PDF、Word、Excel、HTML 等复杂文档解析
- `Embedding / Rerank Worker`：批量生成向量、离线质量评估
- `Eval Worker`：评测集运行、统计分析、错误归因
- `Notebook / Lab`：prompt、RAG、memory 策略实验

这些模块可以通过消息队列、对象存储、MySQL、Qdrant 与 Go 主服务解耦。  
这样既能利用 Python AI 生态，又不会让核心平台运行时被 Python 工程体系绑定。

---

## 10. 观测、评测与运维设计

## 10.1 可观测性

建议统一接入：

- `Prometheus`
- `Grafana`
- `OpenTelemetry`
- `Loki / ELK / OpenSearch`

## 10.2 关键埋点

- `speech_started`
- `speech_ended`
- `vad_triggered`
- `asr_partial_received`
- `asr_final_received`
- `router_decided`
- `llm_started`
- `first_token_received`
- `tool_called`
- `tool_finished`
- `tts_started`
- `tts_first_audio`
- `tts_interrupted`
- `user_interrupted`
- `task_completed`
- `task_failed`

## 10.3 关键评测指标

### 语音层

- VAD 准确率
- 截断率
- ASR 词错率
- TTS 首包时延

### 对话层

- 首响应时间
- 打断成功率
- 上下文续接成功率

### 任务层

- 意图识别准确率
- 工具调用成功率
- 任务完成率

### 体验层

- 重复表达率
- 放弃率
- 满意度

---

## 11. 安全与权限设计

## 11.1 必须具备的能力

- OAuth2 / JWT
- 多租户隔离
- RBAC
- Tool 级权限控制
- 高风险动作确认
- 审计日志

## 11.2 高风险操作处理

例如：

- 发邮件
- 提交审批
- 删除数据
- 下单 / 支付

必须要求：

- 明确确认
- 记录操作者
- 记录工具参数
- 记录执行结果

---

## 12. 部署与环境建议

## 12.1 首期部署

- Docker
- Kubernetes
- 内部服务优先 HTTP/gRPC

## 12.2 环境划分

- dev
- test
- staging
- prod

## 12.3 多区域演进

若后续支持多国家，建议按区域部署：

- 亚太
- 北美
- 欧洲

主要原因：

- 降低语音实时延迟
- 满足数据合规和驻留要求

---

## 13. 分阶段实施建议

## 13.1 Phase 1：MVP

目标：

- 单语言或双语言
- 基础语音问答
- 3 到 5 个高频工具
- 打通事件、状态、动作链路

建议建设：

- Realtime Gateway
- Speech Service
- Session Service
- AI Orchestrator
- Agent Runtime
- Connector Service
- Knowledge Service（最小版）
- Admin Console（最小版）

## 13.2 Phase 2：可用性强化

目标：

- 多轮任务能力增强
- 小模型/重模型路由
- 语音摘要与增量 TTS
- 回放与评测系统完善

## 13.3 Phase 3：企业化与全球化

目标：

- 多租户
- 多区域部署
- 权限和合规强化
- 检索层和评测层成熟化

---

## 14. 推荐的首版研发拆分

## 14.1 团队 A：语音与实时交互

负责：

- Realtime Gateway
- Speech Service
- Session Service
- 客户端语音交互 SDK

主语言：

- `Go`

## 14.2 团队 B：智能核心与 Agent

负责：

- AI Orchestrator
- Router / Policy
- Agent Runtime
- Presentation Layer
- Knowledge Service
- Evaluation / Replay

主语言：

- `Go`

辅助能力：

- Python Tool Runner
- 文档处理 worker
- 离线评测 worker

## 14.3 团队 C：业务平台与治理

负责：

- Connector Service
- Auth / Tenant / Audit
- Admin Console
- MySQL 主数据管理

主语言：

- 后端 `Go`
- 前端 `TypeScript`

若团队规模较小，可先由两队承担：

- Go 团队：语音与实时链路
- AI 平台团队：Go 核心服务 + TypeScript 管理后台 + 少量 Python worker

---

## 15. 风险与注意事项

### 15.1 架构风险

- 将核心设计成“文本输入、文本输出”的聊天后端，后续语音难复用
- 不区分任务状态和交互状态
- 将权限过滤只放在生成阶段，不放在检索阶段

### 15.2 技术风险

- 所有请求都走重模型，导致延迟和成本失控
- 向量库与 MySQL 元数据不同步
- 没有评测集和回放系统，导致优化靠感觉

### 15.3 组织风险

- 两个团队缺少统一 schema，造成大量返工
- 过早微服务化，导致联调成本过高

---

## 16. 总结

本项目推荐采用 `Go First` AI 中台架构：

- `Go` 负责核心在线服务、实时链路、AI Orchestrator、Agent Runtime、Connector、Knowledge 在线检索、权限治理和审计
- `TypeScript + React` 负责平台管理后台
- `Python` 仅作为辅助运行时，用于 Python Code Tool、离线文档处理、embedding 批处理和评测分析

数据层采用：

- `MySQL` 作为业务真相源
- `Redis` 作为短期状态与缓存层
- `Qdrant` 作为初期向量检索引擎

系统层采用：

- 可复用的 `Core Conversation Runtime`
- 独立的 `Voice Runtime`
- 独立的 `Session Manager`
- 独立的 `Agent / Knowledge / Connector` 分层

这套设计既适合当前阶段快速落地，也为后续多语言、多国家、多渠道和企业级扩展预留了空间。
