# AI Orchestrator 详细设计文档

## 1. 文档目的

本文档用于定义智能语音交互机器人项目中的 `AI Orchestrator` 服务详细设计，作为 AI Platform 的核心入口服务，重点说明：

- 服务职责与边界
- 在整体架构中的位置
- 内部模块拆分
- 输入输出协议
- 状态管理方式
- 与 Agent Runtime、Knowledge Service、Connector Service 的协作关系
- 技术选型与开发建议

本文档目标是支持后续：

- 服务设计评审
- Schema 定稿
- MVP 开发实现
- 与语音链路、文本渠道的对接

---

## 2. 服务定位

## 2.1 服务角色

`AI Orchestrator` 是整个 AI Platform 的“大脑入口层”，负责接收标准化用户事件，结合上下文、任务状态、路由策略和 LLM 推理结果，决定系统下一步应该执行哪些动作。

它不是：

- 语音接入服务
- 会话连接管理服务
- 具体工具执行服务
- 向量检索底层服务

它是：

- 统一的事件处理入口
- 对话和任务的决策层
- Agent 执行的发起方
- 结果呈现和动作规划的组织层

## 2.2 核心职责

### 输入侧职责

- 接收来自语音、文本、电话、IM 等渠道的标准化事件
- 加载会话上下文、任务上下文、用户信息和运行配置
- 识别当前请求属于：
  - 简单回答
  - 追问补槽
  - 确认
  - 工具调用
  - 多步任务
  - 知识检索

### 决策侧职责

- 执行规则路由、小模型路由和重模型决策
- 判断是否要：
  - 直接回复
  - 继续追问
  - 请求确认
  - 调用工具
  - 调用 Knowledge Service
  - 等待工具结果

### 输出侧职责

- 产出标准化 `Action List`
- 更新 `Task State`
- 生成 `summary / details / next_choices`
- 为语音和文本渠道提供不同的呈现提示

## 2.3 不负责的内容

以下职责不应放在 AI Orchestrator 中：

- WebSocket / WebRTC 长连接处理
- 实时音频流处理
- 原始 ASR / TTS 接入
- 具体业务 API 调用
- 向量数据库底层读写
- 用户认证与租户鉴权主逻辑

---

## 3. 在整体架构中的位置

```text
Channel Adapter / Session Service
        ->
AI Orchestrator
        ->
Agent Runtime
        ->
Connector Service / Knowledge Service
        ->
Tool Result / Retrieval Result
        ->
AI Orchestrator
        ->
Presentation Output Actions
```

它位于：

- Session Service 之后
- Agent Runtime 之前

它是：

- 上游事件的语义入口
- 下游执行能力的统一调度者

---

## 4. 设计目标

## 4.1 核心目标

### 4.1.1 渠道无关

AI Orchestrator 不依赖“语音”或“文本”实现细节，只处理标准化事件。

### 4.1.2 动作驱动

输出必须是动作，而不是只返回一段文本。

### 4.1.3 状态可追踪

每次事件处理后，都应有明确的：

- task state 更新
- action 输出
- decision trace

### 4.1.4 可扩展

未来支持：

- 更多模型
- 更多工具
- 更多渠道
- 更多任务类型

### 4.1.5 可治理

必须支持：

- 日志
- 审计
- 评测回放
- prompt / policy 版本管理

## 4.2 非目标

首期不追求：

- 完全自治多 Agent
- 动态自我改写 Workflow
- 所有场景复杂链式推理
- 极端低延迟优化

---

## 5. 服务边界

## 5.1 与 Session Service 的边界

Session Service 负责：

- 当前交互会话归属
- active task 决议
- 超时与恢复
- interaction state

AI Orchestrator 负责：

- 在既定 session / task 上做语义决策
- 更新 task state
- 生成下一步动作

一句话：

- `Session Service` 决定“这句话属于谁”
- `AI Orchestrator` 决定“接下来该做什么”

## 5.2 与 Agent Runtime 的边界

AI Orchestrator 负责：

- 判断是否需要调用工具
- 生成调用意图和参数草案
- 决定是否要求确认

Agent Runtime 负责：

- 真正执行工具
- 处理 workflow、重试、超时、幂等
- 规范化执行结果

一句话：

- `AI Orchestrator` 决定“要不要做”
- `Agent Runtime` 负责“怎么做成”

## 5.3 与 Knowledge Service 的边界

AI Orchestrator 负责：

- 判断是否需要知识检索
- 组织 retrieval request
- 将 retrieval result 融入回复和决策

Knowledge Service 负责：

- 检索
- rerank
- metadata filter
- 索引更新

## 5.4 与 Connector Service 的边界

AI Orchestrator 不应直接访问业务系统 API，也不应直接调用 Connector Service。

AI Orchestrator 只负责生成面向 Agent Runtime 的 Tool Plan 或 Execution Intent，由 Agent Runtime 再根据 Tool Registry 决定是否调用 Connector Service。

正确链路为：

```text
AI Orchestrator
    ->
Tool Plan / Execution Intent
    ->
Agent Runtime
    ->
Connector Service
    ->
External Business API
```

这样可以避免 AI Orchestrator 直接依赖底层业务 API 或 Connector Operation，保证对话决策层和系统集成层解耦。

## 5.5 与 Tool / Connector Operation 的边界

需要明确区分：

- `External API`：外部业务系统真实接口，例如 `GET /orders/{id}`
- `Connector Operation`：Connector Service 封装后的平台内部操作，例如 `order_connector.get_order_detail`
- `Tool`：面向 Agent/LLM 的能力抽象，例如 `query_order_status`

AI Orchestrator 只能感知 Tool，不应感知 External API 或 Connector Operation。

一个 Tool 可以绑定：

- 单个 Connector Operation
- 多个 Connector Operation 组成的 Workflow
- Knowledge Service 的 Retrieval API
- 平台内部能力

---

## 6. 核心能力范围

AI Orchestrator 首期应支持以下能力：

1. 单轮问答
2. 多轮补槽追问
3. 显式确认/否认处理
4. 单工具调用
5. 简单多步任务编排
6. 知识检索驱动问答
7. 面向语音的 summary 输出
8. 面向文本的 detail 输出

---

## 7. 内部模块设计

## 7.1 模块总览

建议拆分为以下内部模块：

1. `Event Normalizer`
2. `Context Loader`
3. `Policy Engine`
4. `Router`
5. `Reasoning Engine`
6. `Action Planner`
7. `Response Planner`
8. `State Manager`
9. `Trace Logger`

处理管道如下：

```text
Input Event
    ->
Event Normalizer
    ->
Context Loader
    ->
Policy Engine / Router
    ->
Reasoning Engine
    ->
Action Planner
    ->
Response Planner
    ->
State Manager
    ->
Trace Logger
    ->
Output Actions
```

## 7.2 Event Normalizer

### 职责

- 统一不同渠道的事件结构
- 清洗无关字段
- 规范 event type、locale、user_id、session_id、task_id

### 输入示例

- `user_message_committed`
- `user_utterance_committed`
- `tool_result_received`
- `tool_failed`
- `conversation_reset_requested`

### 输出

统一内部事件对象 `NormalizedEvent`

## 7.3 Context Loader

### 职责

- 加载当前任务状态
- 加载最近上下文
- 加载用户信息、租户信息、语言和偏好
- 加载当前 policy 配置和 prompt 版本

### 依赖

- Redis
- MySQL
- 配置中心

## 7.4 Policy Engine

### 职责

- 执行硬规则和安全策略
- 判断是否命中：
  - 会话控制
  - 高风险操作
  - 禁止场景
  - 强制确认场景

### 输出

- 是否可继续
- 是否绕过 LLM
- 是否要求强制确认

## 7.5 Router

### 职责

- 判断请求走哪条路径：
  - 规则路径
  - 小模型路径
  - 重模型路径
  - 工具路径
  - 知识检索路径

### 输入信号

- event type
- 当前 task state
- 用户话语复杂度
- 风险等级
- 历史上下文依赖程度
- 小模型置信度

### 输出

```json
{
  "route": "heavy_model_path",
  "requires_tool": true,
  "requires_knowledge": false,
  "requires_confirmation": false
}
```

## 7.6 Reasoning Engine

### 职责

- 调用对应模型执行理解和推理
- 输出结构化语义决策

### 主要能力

- 意图识别
- 槽位抽取
- 多轮上下文理解
- 判断是否缺槽位
- 判断是否需要确认
- 工具选择建议
- 回复语义草案

### 输出应是结构化对象，而不是纯文本

示例：

```json
{
  "intent": "flight_search",
  "confidence": 0.92,
  "slots": {
    "from_city": "上海",
    "to_city": "东京",
    "date": "2026-04-27"
  },
  "missing_slots": ["time_range"],
  "requires_tool": true,
  "tool_candidates": ["search_flights"],
  "requires_confirmation": false,
  "response_semantic": {
    "summary": "我可以帮你查从上海到东京的航班。",
    "next_question": "你想查明天上午、下午还是晚上出发的？"
  }
}
```

## 7.7 Action Planner

### 职责

- 将 Reasoning Engine 的结构化输出转成标准动作

### 动作类型

- `speak`
- `display_text`
- `ask_question`
- `ask_confirmation`
- `call_tool`
- `wait`
- `handoff_human`
- `end_turn`

### 示例

```json
{
  "actions": [
    {
      "type": "ask_question",
      "text": "你想查明天上午、下午还是晚上出发的？"
    }
  ]
}
```

或：

```json
{
  "actions": [
    {
      "type": "speak",
      "text": "好的，我先帮你查一下。"
    },
    {
      "type": "call_tool",
      "tool": "search_flights",
      "args": {
        "from_city": "上海",
        "to_city": "东京",
        "date": "2026-04-27",
        "time_range": "afternoon"
      }
    }
  ]
}
```

## 7.8 Response Planner

### 职责

- 处理文本端和语音端的表达差异
- 将结构化结果转成：
  - `spoken_text`
  - `display_text`
  - `summary`
  - `details`
  - `next_choices`

### 关键原则

- 语音优先短句、摘要、口语化
- 文本可以更详细、结构化

## 7.9 State Manager

### 职责

- 计算 task state patch
- 更新任务状态
- 记录 waiting 条件
- 记录最近一次 action 输出

### 任务状态示例

- `understanding`
- `awaiting_slot`
- `awaiting_confirmation`
- `awaiting_tool_result`
- `responding`
- `completed`
- `failed`

## 7.10 Trace Logger

### 职责

- 记录本次处理链路
- 支持后续回放、评测、错误归因

### 应记录内容

- 输入事件
- 路由决策
- policy 命中
- 模型版本
- prompt 版本
- 工具调用计划
- 输出 actions
- 状态 patch

---

## 8. 输入输出协议设计

## 8.1 输入协议

AI Orchestrator 对外暴露统一事件处理接口：

### 接口建议

- `POST /v1/orchestrator/handle-event`

### 请求体示例

```json
{
  "event": {
    "event_id": "evt_001",
    "type": "user_utterance_committed",
    "session_id": "s_123",
    "task_id": "task_789",
    "user_id": "u_456",
    "tenant_id": "t_001",
    "channel": "voice",
    "locale": "zh-CN",
    "timestamp": "2026-04-26T10:00:00+08:00",
    "payload": {
      "text": "帮我查一下明天下午去东京的航班",
      "language": "zh-CN",
      "confidence": 0.95
    }
  },
  "runtime_context": {
    "device_id": "dev_001",
    "interaction_mode": "voice",
    "requires_fast_ack": true
  }
}
```

## 8.2 输出协议

### 响应体示例

```json
{
  "session_id": "s_123",
  "task_id": "task_789",
  "decision": {
    "route": "heavy_model_path",
    "intent": "flight_search",
    "requires_tool": true,
    "requires_confirmation": false
  },
  "actions": [
    {
      "type": "speak",
      "text": "好的，我先帮你查一下。",
      "presentation": {
        "voice": {
          "interruptible": true,
          "priority": "high"
        }
      }
    },
    {
      "type": "call_tool",
      "tool": "search_flights",
      "args": {
        "from_city": "上海",
        "to_city": "东京",
        "date": "2026-04-27",
        "time_range": "afternoon"
      }
    }
  ],
  "task_state_patch": {
    "status": "awaiting_tool_result",
    "intent": "flight_search"
  },
  "response_content": {
    "summary": "好的，我先帮你查一下。",
    "details": null,
    "next_choices": []
  },
  "trace_id": "trace_xxx"
}
```

---

## 9. 核心 Schema 设计

## 9.1 Event Schema

关键字段：

- `event_id`
- `type`
- `session_id`
- `task_id`
- `user_id`
- `tenant_id`
- `channel`
- `locale`
- `timestamp`
- `payload`

### 推荐事件类型

- `user_message_committed`
- `user_utterance_committed`
- `user_interrupt_detected`
- `tool_result_received`
- `tool_failed`
- `timeout_occurred`
- `conversation_reset_requested`

## 9.2 Action Schema

关键字段：

- `type`
- `text`
- `tool`
- `args`
- `presentation`
- `metadata`

### 推荐动作类型

- `speak`
- `display_text`
- `ask_question`
- `ask_confirmation`
- `call_tool`
- `wait`
- `handoff_human`
- `end_turn`

## 9.3 Task State Schema

关键字段：

- `task_id`
- `intent`
- `status`
- `slots`
- `missing_slots`
- `confirmation_required`
- `tool_execution`
- `last_actions`

### 推荐状态

- `idle`
- `understanding`
- `awaiting_slot`
- `awaiting_confirmation`
- `executing_tool`
- `awaiting_tool_result`
- `responding`
- `completed`
- `failed`

## 9.4 Decision Trace Schema

关键字段：

- `trace_id`
- `route`
- `policy_hits`
- `model_name`
- `model_class`
- `prompt_version`
- `reasoning_summary`
- `tool_plan`

---

## 10. 处理流程设计

## 10.1 用户消息事件处理流程

```text
1. 接收 event
2. Event Normalizer 标准化
3. Context Loader 加载 task/context
4. Policy Engine 执行规则检查
5. Router 判断路径
6. Reasoning Engine 生成结构化决策
7. Action Planner 生成 actions
8. Response Planner 生成 summary/details/spoken_text
9. State Manager 计算并提交 task_state_patch
10. Trace Logger 记录 trace
11. 返回 response
```

## 10.2 工具结果事件处理流程

输入类型为：

- `tool_result_received`
- `tool_failed`

处理逻辑：

1. 读取当前 `awaiting_tool_result` task
2. 解析工具结果
3. 判断是否需要：
   - 继续下一步工具
   - 请求用户确认
   - 汇总结果并回复
   - 标记任务失败
4. 生成动作并更新状态

## 10.3 中断事件处理流程

输入类型为：

- `user_interrupt_detected`

处理逻辑：

1. 根据 Session Service 的 interaction state 判断是否正在 speaking
2. 若需要，生成停止当前播报的控制动作
3. 进入 listening / understanding
4. 等待新的 committed utterance

---

## 11. 路由与模型策略设计

## 11.1 路由层级

建议按以下顺序路由：

1. 规则路径
2. 小模型路径
3. 重模型路径

## 11.2 规则直接命中场景

- 取消
- 重复
- 重新开始
- yes / no
- 简单设备控制
- 禁止场景

## 11.3 小模型适用场景

- 简单问答
- 简单分类
- 简短摘要
- 简单口语改写
- 单工具高置信度调用

## 11.4 重模型适用场景

- 多约束请求
- 多轮上下文依赖
- 高风险操作
- 多工具规划
- 模糊需求
- 知识检索与复杂整合

## 11.5 路由升级条件

若任一命中，则从小模型升级到重模型：

- 置信度低
- 多目标表达
- 高风险动作词
- 强历史上下文依赖
- 当前 task state 复杂

---

## 12. 状态管理设计

## 12.1 Task State 与 Interaction State 分离

AI Orchestrator 只直接维护 Task State。

Interaction State 由 Session Service 维护，例如：

- listening
- speaking
- interrupted

## 12.2 Task State 转换示例

### 例 1：缺槽位

```text
understanding -> awaiting_slot
```

### 例 2：需要确认

```text
understanding -> awaiting_confirmation
```

### 例 3：等待工具结果

```text
understanding -> awaiting_tool_result
```

### 例 4：结果完成

```text
awaiting_tool_result -> completed
```

## 12.3 持久化策略

建议：

- Redis：热状态缓存
- MySQL：任务主状态、历史 patch、action 索引

---

## 13. 错误处理设计

## 13.1 错误类型

- `validation_error`
- `policy_rejected`
- `routing_error`
- `model_timeout`
- `model_output_invalid`
- `tool_plan_invalid`
- `state_conflict`
- `internal_error`

## 13.2 错误处理原则

- 错误对象结构化
- 尽量返回可恢复动作
- 记录完整 trace
- 对外隐藏内部实现细节

## 13.3 降级策略

### 路由失败

- 降级到重模型通用理解

### 重模型超时

- 返回兜底回复
- 若语音场景，优先播报短句占位

### 工具计划不合法

- 请求澄清或改为人工兜底

---

## 14. 可观测性设计

## 14.1 关键日志

- input event
- normalized event
- context summary
- route result
- model selected
- prompt version
- actions output
- task_state_patch
- latency breakdown

## 14.2 关键指标

- orchestrator request count
- success rate
- route distribution
- small model hit rate
- heavy model hit rate
- average latency
- tool invocation planning rate
- clarification rate
- confirmation rate
- fallback rate

## 14.3 Trace 维度

每次请求建议打出统一：

- `trace_id`
- `session_id`
- `task_id`
- `tenant_id`
- `user_id`
- `channel`

---

## 15. 技术选型与实现建议

## 15.1 推荐语言

- `Go`

## 15.2 选择原因

- AI Orchestrator 是在线主链路服务，需要稳定、低延迟、易部署和强类型协议约束
- Go 更适合实现 Context Builder、Runtime Decision、状态流转、并发调用和可观测性
- 团队 Go 经验较强，可以降低工程落地和长期维护成本
- Python 能力保留在离线实验、评测、文档处理、Python Tool Runner 等辅助 worker 中，不进入 Orchestrator 主链路

## 15.3 推荐技术栈

- `chi` 或 `gin`
- `grpc-go` / `connect-go`
- `go-playground/validator`
- `santhosh-tekuri/jsonschema` 或同类 JSON Schema 校验库
- `net/http` 或 `resty`
- `sqlc` 或 `ent`
- `go-sql-driver/mysql`
- `go-redis`
- `zap` 或 `zerolog`
- `OpenTelemetry Go`
- `testify`

## 15.4 工程组织建议

建议代码按以下目录组织：

```text
ai_orchestrator/
  api/
  schemas/
  services/
  router/
  policy/
  reasoning/
  planner/
  state/
  storage/
  tracing/
  tests/
```

---

## 16. MVP 范围

首版建议只实现以下能力：

1. `handle-event` 单入口
2. 支持 `user_utterance_committed`
3. 支持 `tool_result_received`
4. 支持 `tool_failed`
5. 支持基础路由：
   - 规则
   - 小模型
   - 重模型
6. 支持基础动作：
   - `speak`
   - `ask_question`
   - `call_tool`
   - `ask_confirmation`
7. 支持基础 task state patch
8. 支持 trace logging

MVP 暂不要求：

- 多 Agent 协同
- 动态工作流生成
- 复杂长期记忆
- 全量多语言策略差异化

---

## 17. 与后续服务的对接要求

## 17.1 对 Agent Runtime 的要求

- 提供统一 `execute-tool-plan` 接口
- 返回标准 `tool_result_received`
- 工具结果结构化

## 17.2 对 Knowledge Service 的要求

- 提供统一 retrieval API
- 支持 metadata filter
- 支持权限标签过滤

## 17.3 对 Session Service 的要求

- 保证 session_id / task_id 的有效性
- 提供 active task 归属
- 提供 interaction state

---

## 18. 实施建议

建议实施顺序如下：

1. 定义 Schema
2. 实现 `handle-event` 主流程
3. 实现 Router + Policy Engine
4. 实现 Reasoning Engine MVP
5. 实现 Action Planner / Response Planner
6. 接入 State Manager
7. 接入 Trace Logger
8. 再与 Agent Runtime 联调

---

## 19. 总结

AI Orchestrator 是整个 AI Platform 的中枢服务，其核心价值在于：

- 把不同渠道输入统一成事件
- 把 LLM 推理结果统一成动作
- 把任务推进、工具调用和用户表达组织在同一条可追踪链路上

在项目实施中，AI Orchestrator 应作为第一优先级详细设计和开发对象，因为它决定了：

- 智能核心的抽象边界
- 与 Agent Runtime 的接口关系
- 与 Knowledge Service、Connector Service 的集成方式
- 未来语音、文本、多渠道复用的基础能力
