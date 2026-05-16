# Agent Runtime 详细设计文档

## 1. 文档目的

本文档用于定义智能语音交互机器人项目中的 `Agent Runtime` 服务详细设计，明确其在 AI Platform 中的职责、边界、内部模块、协议设计、执行生命周期、状态管理、错误处理和技术选型。

本文档重点回答以下问题：

- Agent Runtime 在系统中的定位是什么
- 它与 AI Orchestrator、Connector Service、Knowledge Service 的边界如何划分
- Tool 如何注册、选择、执行和回传结果
- 多步骤任务如何编排
- 如何处理确认、幂等、重试、超时和审计
- 如何支持后续扩展到更复杂的 Agent 工作流

本文档面向：

- 服务设计评审
- 接口与 Schema 定稿
- MVP 实施开发
- 与 AI Orchestrator 的联调

---

## 2. 服务定位

## 2.1 服务角色

`Agent Runtime` 是 AI Platform 的执行层，负责将 AI Orchestrator 生成的工具调用计划或任务执行计划，转化为受控、可追踪、可恢复的实际执行过程。

它不是：

- 用户事件入口
- 对话主决策层
- 直接生成最终用户回复的服务

它是：

- Tool 执行协调器
- Workflow 推进器
- 确认与重试控制器
- 执行结果标准化器

## 2.2 核心职责

### 输入侧职责

- 接收 AI Orchestrator 传入的 `tool plan` 或 `execution plan`
- 验证工具是否存在、参数是否合法、权限是否满足
- 判断执行前是否需要用户确认

### 执行侧职责

- 调用 Connector Service 或 Knowledge Service
- 执行单工具任务
- 执行简单多步骤工作流
- 进行超时、重试、幂等控制

### 输出侧职责

- 返回标准化的工具执行结果
- 在异步场景下回传 `tool_result_received`
- 记录审计与执行 trace

---

## 3. 在整体架构中的位置

```text
AI Orchestrator
      ->
Agent Runtime
      ->
Connector Service / Knowledge Service
      ->
Normalized Tool Result
      ->
AI Orchestrator
```

Agent Runtime 位于：

- AI Orchestrator 之后
- Connector / Knowledge 等能力层之前

一句话：

- `AI Orchestrator` 决定“做什么”
- `Agent Runtime` 负责“怎么做”

---

## 4. 设计目标

## 4.1 核心目标

### 4.1.1 标准化执行

无论是查询、创建、修改还是多步任务，都通过统一执行接口处理。

### 4.1.2 安全可控

所有工具调用必须满足：

- 参数校验
- 权限校验
- 高风险操作确认
- 审计留痕

### 4.1.3 失败可恢复

执行过程应支持：

- 重试
- 超时
- 幂等
- 部分失败回传

### 4.1.4 可扩展

首期支持单工具和简单工作流；后续可演进到复杂多步骤任务和更强 Agent 编排。

### 4.1.5 与底层能力解耦

Agent Runtime 不应深度绑定某个具体业务系统，也不应深度绑定某个具体知识检索实现。

---

## 5. 服务边界

## 5.1 与 AI Orchestrator 的边界

AI Orchestrator 负责：

- 理解用户意图
- 决定是否调用工具
- 生成 tool plan
- 生成用户可见动作

Agent Runtime 负责：

- 校验并执行 tool plan
- 推进 workflow
- 返回标准结果

AI Orchestrator 不应直接执行工具。  
Agent Runtime 不应直接决定用户对话策略。

## 5.2 与 Connector Service 的边界

Connector Service 负责：

- 与具体业务系统或外部 API 交互
- 屏蔽鉴权、协议、限流、重试差异
- 暴露标准化 Connector Operation

Agent Runtime 负责：

- 根据 Tool Registry 决定 Tool 绑定到哪个 Connector Operation 或 Workflow
- 将 Tool 参数转换为 Connector Operation 参数
- 控制执行生命周期
- 将 Connector Result 标准化为 Tool Result

## 5.3 与 Knowledge Service 的边界

Knowledge Service 可被看作一种特殊工具能力。

Agent Runtime 负责：

- 统一调用入口
- 统一结果协议

Knowledge Service 负责：

- retrieval / rerank / metadata filtering

---

## 6. 能力范围

## 6.1 首期支持能力

1. 单工具同步调用
2. 单工具异步调用
3. 基础重试
4. 基础超时
5. 幂等控制
6. 高风险操作确认
7. 简单多步骤任务
8. 标准化工具结果输出

## 6.2 后续演进能力

1. 条件分支 workflow
2. 多工具依赖编排
3. 补偿动作
4. 人工介入节点
5. 更复杂的长期任务执行

---

## 7. 核心概念设计

## 7.1 Tool

表示一个可被 AI 系统调用的受控能力单元。

Tool 是面向 Agent/LLM 的语义能力抽象，不等同于外部业务系统 API，也不等同于 Connector Operation。

一个 Tool 应包括：

- `tool_name`
- `description`
- `category`
- `input_schema`
- `output_schema`
- `risk_level`
- `requires_confirmation`
- `timeout_ms`
- `retry_policy`
- `idempotency_policy`
- `execution_binding`

## 7.2 Tool 与 Connector Operation 的关系

需要明确三层抽象：

- `External API`：外部业务系统原始接口，例如 `POST /tickets`
- `Connector Operation`：Connector Service 封装后的平台内部操作，例如 `ticket_connector.create_ticket`
- `Tool`：面向 Agent/LLM 的能力，例如 `create_support_ticket`

Agent Runtime 负责把 Tool 映射到可执行能力。

一个 Tool 的 `execution_binding` 可以是：

- `connector_operation`
- `workflow`
- `knowledge_retrieval`
- `internal_action`

示例：

```json
{
  "tool_name": "create_support_ticket",
  "description": "Create a support ticket after collecting required issue details.",
  "input_schema": {
    "type": "object",
    "properties": {
      "customer_phone": { "type": "string" },
      "issue_type": { "type": "string" },
      "description": { "type": "string" }
    },
    "required": ["customer_phone", "issue_type", "description"]
  },
  "risk_level": "medium",
  "requires_confirmation": true,
  "execution_binding": {
    "type": "workflow",
    "workflow_name": "support_ticket_create_flow"
  }
}
```

Tool Registry 负责保存这层绑定关系，Connector Service 不负责生成或管理 Tool。

## 7.3 Tool Plan

表示 AI Orchestrator 发给 Agent Runtime 的执行计划。

示例：

```json
{
  "plan_id": "plan_001",
  "task_id": "task_789",
  "tool": "search_flights",
  "args": {
    "from_city": "上海",
    "to_city": "东京",
    "date": "2026-04-27",
    "time_range": "afternoon"
  },
  "requires_confirmation": false,
  "idempotency_key": "task_789_search_flights_v1"
}
```

## 7.4 Execution

表示某次工具计划的具体执行实例。

其生命周期包括：

- `created`
- `validated`
- `waiting_confirmation`
- `running`
- `retrying`
- `succeeded`
- `failed`
- `cancelled`
- `timed_out`

## 7.5 Workflow

表示由多个 Tool Step 组成的任务执行流程。

首期建议支持：

- 线性步骤
- 简单条件继续
- 等待用户确认后再执行后续步骤

---

## 8. 内部模块设计

## 8.1 模块总览

建议拆分为以下内部模块：

1. `Execution API`
2. `Tool Registry`
3. `Plan Validator`
4. `Permission Guard`
5. `Confirmation Manager`
6. `Execution Engine`
7. `Workflow Engine`
8. `Connector Adapter`
9. `Result Normalizer`
10. `Execution Store`
11. `Trace & Audit Logger`

执行管道如下：

```text
Tool Plan Input
    ->
Plan Validator
    ->
Permission Guard
    ->
Confirmation Manager
    ->
Execution Engine / Workflow Engine
    ->
Connector Adapter / Knowledge Adapter
    ->
Result Normalizer
    ->
Execution Store
    ->
Trace & Audit Logger
    ->
Tool Result Output
```

## 8.2 Execution API

### 职责

- 接收 Orchestrator 的执行请求
- 提供同步执行和异步执行接口
- 提供确认继续接口
- 提供查询执行状态接口

### 推荐接口

- `POST /v1/agent/execute`
- `POST /v1/agent/confirm`
- `GET /v1/agent/executions/{execution_id}`

## 8.3 Tool Registry

### 职责

- 注册和管理可调用工具元数据
- 向执行引擎提供 Tool 定义
- 管理 Tool 与 Connector Operation / Workflow / Knowledge Retrieval 的绑定关系

### Tool Registry 中应保存

- 名称
- 版本
- 输入输出 schema
- 风险级别
- 超时设置
- 重试策略
- execution binding

### 存储位置建议

- MySQL 持久化
- Redis 缓存

## 8.4 Plan Validator

### 职责

- 校验 tool 是否存在
- 校验参数类型、必填项、枚举值
- 校验 plan 结构完整性

### 输出

- `validated plan`
- 或结构化错误

## 8.5 Permission Guard

### 职责

- 校验租户权限
- 校验用户是否有权调用该工具
- 校验是否命中风险策略

### 输入

- user_id
- tenant_id
- tool_name
- args

### 输出

- `allowed`
- `rejected`
- `requires_confirmation`

## 8.6 Confirmation Manager

### 职责

- 判断当前 plan 是否必须确认
- 生成等待确认的执行状态
- 接收后续确认/否认结果

### 适用场景

- 发邮件
- 删除数据
- 提交审批
- 下单/支付

## 8.7 Execution Engine

### 职责

- 执行单工具计划
- 控制重试、超时、幂等

### 主要流程

1. 接收 validated plan
2. 生成 execution_id
3. 检查 idempotency key
4. 调用 Connector Adapter
5. 接收原始结果
6. 标准化结果
7. 保存 execution record
8. 返回结果

## 8.8 Workflow Engine

### 职责

- 执行多步骤计划
- 维护 step 状态
- 根据条件推进后续步骤

### 首期支持的工作流结构

- 线性步骤
- 单次确认节点
- 单次知识检索后继续执行

### 示例流程

```text
Step1: search_customer
Step2: ask_confirmation
Step3: create_ticket
Step4: send_notification
```

## 8.9 Connector Adapter

### 职责

- 根据 tool 定义找到对应 connector
- 适配统一执行接口

### 适配类型

- business connector
- knowledge connector
- internal service connector

### 注意

Agent Runtime 只感知统一 adapter 接口，不感知底层系统差异。

## 8.10 Result Normalizer

### 职责

- 将不同 connector 返回值转换成统一 Tool Result
- 统一 success / partial / failure 表达

### 输出结构建议

```json
{
  "execution_id": "exec_001",
  "tool": "search_flights",
  "status": "success",
  "result": {
    "summary": "找到 3 个航班",
    "data": [
      {
        "flight_no": "MU123",
        "price": 2300
      }
    ]
  },
  "error": null
}
```

## 8.11 Execution Store

### 职责

- 保存 execution record
- 保存 workflow step 状态
- 保存确认状态
- 保存重试记录

### 存储建议

- MySQL：执行主记录、审计记录、workflow 状态
- Redis：执行中的热状态和幂等键

## 8.12 Trace & Audit Logger

### 职责

- 记录执行 trace
- 记录工具调用参数摘要
- 记录风险动作审计信息

---

## 9. 输入输出协议设计

## 9.1 execute 接口

### 请求示例

```json
{
  "plan_id": "plan_001",
  "task_id": "task_789",
  "session_id": "s_123",
  "user_id": "u_456",
  "tenant_id": "t_001",
  "tool_plan": {
    "tool": "search_flights",
    "args": {
      "from_city": "上海",
      "to_city": "东京",
      "date": "2026-04-27",
      "time_range": "afternoon"
    },
    "requires_confirmation": false,
    "idempotency_key": "task_789_search_flights_v1"
  },
  "runtime_context": {
    "channel": "voice",
    "locale": "zh-CN"
  }
}
```

### 响应示例

```json
{
  "execution_id": "exec_001",
  "task_id": "task_789",
  "status": "succeeded",
  "tool_result": {
    "tool": "search_flights",
    "status": "success",
    "result": {
      "summary": "找到 3 个明天下午从上海到东京的航班",
      "data": [
        {
          "flight_no": "MU123",
          "departure_time": "13:20",
          "price": 2300
        }
      ]
    },
    "error": null
  },
  "trace_id": "trace_exec_001"
}
```

## 9.2 confirm 接口

### 请求示例

```json
{
  "execution_id": "exec_002",
  "task_id": "task_789",
  "user_id": "u_456",
  "tenant_id": "t_001",
  "decision": "approved"
}
```

### 响应示例

```json
{
  "execution_id": "exec_002",
  "status": "running"
}
```

## 9.3 查询执行状态接口

返回：

- 当前状态
- 当前 step
- 是否等待确认
- 最近错误
- 最近结果摘要

---

## 10. Tool Registry 设计

## 10.1 Tool 元数据结构

建议字段包括：

- `tool_name`
- `tool_version`
- `description`
- `category`
- `input_schema`
- `output_schema`
- `risk_level`
- `requires_confirmation`
- `timeout_ms`
- `max_retries`
- `idempotency_strategy`
- `connector_name`
- `enabled`

## 10.2 风险等级建议

- `low`
- `medium`
- `high`
- `critical`

### 默认策略

- `low`：可直接执行
- `medium`：按策略决定是否确认
- `high`：默认确认
- `critical`：必须双重确认或人工介入

## 10.3 工具分类建议

- `knowledge_query`
- `business_query`
- `business_action`
- `notification`
- `system_control`

---

## 11. 执行生命周期设计

## 11.1 单工具执行生命周期

```text
created
  ->
validated
  ->
(waiting_confirmation)?
  ->
running
  ->
(retrying)?
  ->
succeeded / failed / timed_out / cancelled
```

## 11.2 幂等处理

### 原则

- 写操作必须支持幂等
- 查询操作可弱化幂等要求

### 实现建议

- 由 Orchestrator 传入 `idempotency_key`
- Agent Runtime 生成执行签名
- Redis 存短期幂等锁
- MySQL 存最终执行结果索引

## 11.3 超时与重试

### 建议策略

- 每个 tool 配置独立超时
- 查询类可重试 1 到 2 次
- 写操作默认不自动重试，除非 connector 明确支持安全重试

## 11.4 取消

支持后续扩展：

- 用户取消
- 会话关闭
- 系统超时取消

---

## 12. Workflow 设计

## 12.1 首期工作流模型

首期建议使用轻量自研 workflow，而不是引入复杂工作流引擎。

支持结构：

- `steps[]`
- `next_on_success`
- `next_on_failure`
- `requires_confirmation_before_step`

## 12.2 Workflow Step 结构

建议字段：

- `step_id`
- `tool`
- `args_template`
- `requires_confirmation`
- `timeout_ms`
- `retry_policy`

## 12.3 示例

```json
{
  "workflow_id": "wf_001",
  "steps": [
    {
      "step_id": "step1",
      "tool": "search_customer",
      "args_template": {
        "phone": "${slots.phone}"
      }
    },
    {
      "step_id": "step2",
      "tool": "create_ticket",
      "requires_confirmation": true,
      "args_template": {
        "customer_id": "${step1.result.customer_id}",
        "issue_type": "${slots.issue_type}"
      }
    }
  ]
}
```

## 12.4 何时需要升级到更重的 Workflow Engine

当出现以下情况可考虑后续升级：

- 长时任务非常多
- 分支和补偿逻辑很复杂
- 需要跨天状态恢复
- 需要大量异步任务协调

---

## 13. 结果标准化设计

## 13.1 Tool Result 结构

建议统一为：

```json
{
  "execution_id": "exec_001",
  "tool": "search_flights",
  "status": "success",
  "result_type": "structured_data",
  "result": {
    "summary": "找到 3 个结果",
    "data": []
  },
  "error": null,
  "metadata": {
    "latency_ms": 420,
    "connector": "flight_api"
  }
}
```

## 13.2 status 枚举

- `success`
- `partial_success`
- `failed`
- `cancelled`
- `timed_out`
- `awaiting_confirmation`

## 13.3 error 结构

建议字段：

- `code`
- `message`
- `retryable`
- `details`

---

## 14. 错误处理设计

## 14.1 错误类型

- `tool_not_found`
- `invalid_args`
- `permission_denied`
- `confirmation_required`
- `execution_timeout`
- `connector_error`
- `workflow_error`
- `idempotency_conflict`
- `internal_error`

## 14.2 错误处理原则

- 所有错误结构化
- 尽量区分可重试与不可重试
- 向 Orchestrator 返回标准错误对象
- 对高风险失败动作记录审计日志

## 14.3 降级策略

### connector 失败

- 查询类：可按策略重试
- 写操作：默认不自动重试

### workflow 中途失败

- 返回部分结果
- 告知失败 step
- 由 Orchestrator 决定是否提示用户重试或改为人工介入

---

## 15. 状态持久化设计

## 15.1 MySQL 存储内容

- tool metadata
- execution 主记录
- workflow 记录
- confirmation 记录
- 审计索引

## 15.2 Redis 存储内容

- 执行中状态缓存
- 幂等锁
- 短期确认等待态

## 15.3 对账与恢复

建议支持：

- 定时扫描长时间 running 状态
- 异常 execution 自动回收或标记
- 失败 execution 可回放分析

---

## 16. 可观测性与审计设计

## 16.1 关键日志

- input plan
- validation result
- permission result
- confirmation result
- execution state transition
- connector request summary
- connector response summary
- normalized result
- error detail

## 16.2 核心指标

- execution count
- success rate
- average latency
- retry count
- timeout rate
- confirmation rate
- permission denied rate
- per-tool failure rate
- per-connector latency

## 16.3 审计要求

对以下场景必须审计：

- 发信
- 删除
- 提交审批
- 下单/支付
- 修改关键数据

审计内容至少包括：

- who
- when
- which tool
- args summary
- decision
- result

---

## 17. 技术选型与实现建议

## 17.1 推荐语言

- `Go`

## 17.2 原因

- Agent Runtime 是受控执行中心，核心职责是 Tool 生命周期、Workflow 状态、确认、幂等、超时、重试和审计
- 这些能力更偏工程运行时而不是模型实验，Go 的并发、类型系统和单 binary 部署更适合长期稳定运行
- Python 脚本类 Tool 不由 Agent Runtime 直接执行，而是通过独立 `Python Tool Runner` 沙箱执行
- 采用 Go 可以让在线主链路保持稳定，同时保留 Python Tool 的灵活性

## 17.3 推荐技术栈

- `chi` 或 `gin`
- `grpc-go` / `connect-go`
- `go-playground/validator`
- `santhosh-tekuri/jsonschema` 或同类 JSON Schema 校验库
- `sqlc` 或 `ent`
- `go-sql-driver/mysql`
- `go-redis`
- `zap` 或 `zerolog`
- `OpenTelemetry Go`
- `testify`
- 可选：复杂长任务阶段评估 `Temporal`

## 17.4 代码组织建议

```text
agent_runtime/
  api/
  schemas/
  registry/
  validator/
  permission/
  confirmation/
  execution/
  workflow/
  adapters/
  storage/
  tracing/
  tests/
```

---

## 18. MVP 范围

首期建议实现：

1. `execute` 接口
2. `confirm` 接口
3. Tool Registry
4. Plan Validator
5. Permission Guard
6. Confirmation Manager
7. 单工具执行引擎
8. 基础幂等
9. 基础重试和超时
10. 结果标准化
11. 执行日志与审计日志

MVP 可暂缓：

- 复杂分支 workflow
- 补偿事务
- 多 Agent 协同
- 跨天长时任务恢复

---

## 19. 实施顺序建议

建议按以下顺序实现：

1. 定义 Tool Schema / Tool Result Schema
2. 实现 Tool Registry
3. 实现 execute API
4. 实现 Plan Validator / Permission Guard
5. 实现单工具 Execution Engine
6. 实现 Result Normalizer
7. 接入 Connector Adapter
8. 实现 confirm API
9. 实现基础 workflow
10. 补充审计与回放能力

---

## 20. 总结

Agent Runtime 是 AI Platform 的受控执行中心，它将 AI Orchestrator 的决策结果转化为真实、可审计、可恢复的任务执行过程。

其设计关键在于：

- 工具定义标准化
- 执行生命周期清晰
- 确认、权限、幂等、重试机制完备
- 结果标准化后回流给 Orchestrator

在项目推进上，Agent Runtime 应作为 AI Orchestrator 之后的第二优先级详细设计和开发对象，因为它决定了：

- AI Agent 是否真正具备稳定执行能力
- 业务系统接入是否可控
- RAG、Connector、Workflow 是否能够被统一纳入平台能力
