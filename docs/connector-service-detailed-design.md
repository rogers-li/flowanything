# Connector Service 详细设计文档

## 1. 文档目的

本文档用于定义智能语音交互机器人项目中的 `Connector Service` 详细设计，明确该服务在 AI Platform 中的职责、边界、内部模块、接口协议、接入规范、容错机制、权限与审计协作方式以及推荐技术选型。

本文档重点回答以下问题：

- Connector Service 在系统中的定位是什么
- 它与 Agent Runtime、业务系统、Knowledge Service 的边界如何划分
- 不同业务系统如何以统一方式接入
- 如何处理鉴权、限流、超时、重试、幂等、错误映射
- 如何向 Agent Runtime 返回标准化结果

本文档面向：

- 服务设计评审
- 平台接入规范制定
- 后续具体业务系统 connector 开发
- 与 Agent Runtime 联调

---

## 2. 服务定位

## 2.1 服务角色

`Connector Service` 是 AI Platform 的系统集成层，负责把 Agent Runtime 发起的标准化 Connector Request 转发到具体业务系统、外部 API 或内部平台服务，并将其返回结果标准化后回传给 Agent Runtime。

更准确地说，Connector Service 负责封装外部业务系统 API，并对 Agent Runtime 暴露标准化 `Connector Operation`。它不负责把能力包装成 LLM 可见的 Tool。

它不是：

- 用户交互入口
- 对话决策服务
- Tool 计划生成服务
- Tool Registry 服务
- 业务真相存储服务

它是：

- 业务系统适配层
- 外部接口统一封装层
- 协议、鉴权、重试与错误映射层
- Connector Operation 提供层

## 2.2 核心职责

### 输入侧职责

- 接收 Agent Runtime 发起的标准化 connector request
- 根据 connector 配置选择具体目标系统
- 进行请求级参数校验和协议映射

### 执行侧职责

- 管理与外部系统的 HTTP / gRPC / SDK / DB 调用
- 处理鉴权、超时、重试、熔断、限流
- 处理幂等透传与请求去重

### 输出侧职责

- 输出标准化 connector result
- 统一错误结构
- 记录调用日志和审计摘要

---

## 3. 在整体架构中的位置

```text
AI Orchestrator
    ->
Agent Runtime
    ->
Connector Service
    ->
Business Systems / External APIs / Internal Services
    ->
Connector Result
    ->
Agent Runtime
    ->
AI Orchestrator
```

Connector Service 位于：

- Agent Runtime 之后
- 具体业务系统之前

一句话：

- `Agent Runtime` 决定“调用哪个能力”
- `Connector Service` 负责“怎么接这个能力”

---

## 4. 设计目标

## 4.1 核心目标

### 4.1.1 统一接入模型

不同业务系统无论协议和风格如何差异，都统一通过 Connector Service 提供受控访问。

### 4.1.2 平台与业务解耦

Agent Runtime 不直接感知每个业务系统的接口细节。

### 4.1.3 可治理

每个 connector 都应支持：

- 鉴权配置
- 超时配置
- 重试配置
- 错误映射
- 审计与日志

### 4.1.4 可扩展

支持：

- HTTP API
- gRPC
- SDK 调用
- 内部 RPC
- 后续异步消息型调用

### 4.1.5 可维护

每个 connector 应尽量独立、清晰、可测试，避免把业务系统差异污染到平台核心。

## 4.2 非目标

首期不追求：

- 所有业务系统自动生成 connector
- 复杂 ESB 式编排
- 跨系统事务补偿引擎

---

## 5. 服务边界

## 5.1 与 Agent Runtime 的边界

Agent Runtime 负责：

- 工具计划校验
- 权限判断
- 执行生命周期控制
- workflow 推进

Connector Service 负责：

- 将标准化请求翻译成具体系统调用
- 返回标准化 connector result

Agent Runtime 不应直接调用第三方系统或业务系统 API。

## 5.2 与业务系统的边界

业务系统负责：

- 提供实际业务能力
- 维持业务真相和主数据

Connector Service 负责：

- 访问适配
- 协议转换
- 鉴权透传
- 错误映射

## 5.3 与 Knowledge Service 的边界

Knowledge Service 不建议通过 Connector Service 访问。  
Knowledge Service 是 AI Platform 内部的一类能力服务，更适合直接由 Agent Runtime 以内部适配器方式接入。

换言之：

- 企业业务系统、第三方 API 走 Connector Service
- 检索、RAG 等内部平台能力不走 Connector Service

---

## 6. 适用场景

Connector Service 适用于以下类型的系统接入：

- CRM
- ERP
- 工单系统
- 订单系统
- 物流查询
- 日历 / 邮件 / IM
- 支付/审批系统
- 企业内部微服务
- 第三方 SaaS API

---

## 7. 核心概念设计

## 7.1 Connector

表示某一类目标系统的接入适配器。

一个 Connector 应包括：

- `connector_name`
- `connector_type`
- `target_system`
- `protocol`
- `auth_strategy`
- `timeout_ms`
- `retry_policy`
- `rate_limit_policy`
- `error_mapping`
- `enabled`

## 7.2 Operation

表示 Connector 暴露给 Agent Runtime 的单个可调用操作。Operation 是平台内部技术抽象，不直接暴露给 LLM。

一个 Operation 应包括：

- `operation_name`
- `method`
- `path` 或 `rpc_name`
- `input_schema`
- `output_schema`
- `idempotency_support`
- `timeout_ms`

## 7.3 API / Connector Operation / Tool 的关系

必须明确区分三层：

- `External API`：外部业务系统原始接口，例如 `GET /orders/{id}`、`POST /tickets`
- `Connector Operation`：Connector Service 封装后的标准操作，例如 `order_connector.get_order_detail`
- `Tool`：Tool Registry 定义、Agent Runtime 执行、可暴露给 LLM 的能力，例如 `query_order_status`

Connector Service 只负责前两层之间的适配：

```text
External API
    ->
Connector Operation
```

Tool 层由 Agent Runtime 和 Tool Registry 负责：

```text
Tool
    ->
Agent Runtime
    ->
Connector Operation
    ->
External API
```

一个 Tool 可以绑定一个 Connector Operation，也可以绑定多个 Connector Operation 组成的 Workflow。

## 7.4 Connector Request

表示 Agent Runtime 发起的标准化执行请求。

示例：

```json
{
  "request_id": "conn_req_001",
  "execution_id": "exec_001",
  "tenant_id": "t_001",
  "user_id": "u_456",
  "connector_name": "flight_api",
  "operation": "search_flights",
  "args": {
    "from_city": "上海",
    "to_city": "东京",
    "date": "2026-04-27"
  },
  "idempotency_key": "task_789_search_flights_v1"
}
```

## 7.5 Connector Result

表示 Connector Service 返回给 Agent Runtime 的标准化结果。

示例：

```json
{
  "request_id": "conn_req_001",
  "connector_name": "flight_api",
  "operation": "search_flights",
  "status": "success",
  "result": {
    "data": [
      {
        "flight_no": "MU123",
        "price": 2300
      }
    ]
  },
  "error": null,
  "metadata": {
    "latency_ms": 340,
    "target_status_code": 200
  }
}
```

---

## 8. 内部模块设计

## 8.1 模块总览

建议拆分为以下模块：

1. `Connector API`
2. `Connector Registry`
3. `Request Validator`
4. `Auth Manager`
5. `Protocol Adapter`
6. `Request Mapper`
7. `Response Mapper`
8. `Retry & Timeout Controller`
9. `Rate Limit / Circuit Breaker`
10. `Audit Logger`
11. `Connector Config Manager`

调用管道如下：

```text
Connector Request
    ->
Request Validator
    ->
Connector Registry
    ->
Auth Manager
    ->
Request Mapper
    ->
Protocol Adapter
    ->
Target System
    ->
Response Mapper
    ->
Error Mapping
    ->
Connector Result
```

## 8.2 Connector API

### 职责

- 接收 Agent Runtime 的调用请求
- 返回同步或异步执行结果

### 推荐接口

- `POST /v1/connectors/invoke`
- `GET /v1/connectors/registry`

## 8.3 Connector Registry

### 职责

- 管理 connector 元数据
- 提供 connector 与 operation 查询

### 存储内容

- connector 配置
- operation 配置
- auth 配置引用
- 超时、重试、限流策略

### 存储建议

- MySQL 持久化
- Redis 缓存热点配置

## 8.4 Request Validator

### 职责

- 校验 connector 是否存在
- 校验 operation 是否存在
- 校验参数结构和必填字段
- 校验租户和环境配置可用性

## 8.5 Auth Manager

### 职责

- 管理目标系统鉴权
- 支持不同认证策略：
  - API Key
  - OAuth2
  - JWT
  - Basic Auth
  - 服务账号
  - 租户级 token

### 设计原则

- 凭证不应由 Agent Runtime 直接传入
- 凭证应由 Connector Service 安全管理和注入

## 8.6 Protocol Adapter

### 职责

- 执行不同协议的底层调用

### 首期支持

- HTTP / HTTPS
- 内部 HTTP API
- gRPC

### 后续支持

- SDK 封装调用
- 消息队列异步触发

## 8.7 Request Mapper

### 职责

- 将标准化 args 转换为目标系统请求格式
- 支持：
  - path params
  - query params
  - headers
  - body

### 目标

让 Agent Runtime 只关心统一参数结构，不关心底层协议细节。

## 8.8 Response Mapper

### 职责

- 将目标系统原始返回结果转换成标准化结构
- 提取关键字段、错误码、业务状态

## 8.9 Retry & Timeout Controller

### 职责

- 根据 connector / operation 配置控制超时
- 按策略控制是否重试

### 推荐策略

- 查询型 operation 可配置 1 到 2 次重试
- 写操作默认不自动重试，除非明确幂等安全

## 8.10 Rate Limit / Circuit Breaker

### 职责

- 防止下游系统被压垮
- 保护平台自身稳定性

### 建议支持

- connector 级限流
- operation 级限流
- 基础熔断

## 8.11 Audit Logger

### 职责

- 记录敏感调用摘要
- 记录目标系统调用结果
- 支持后续审计追踪

## 8.12 Connector Config Manager

### 职责

- 动态加载 connector 配置
- 配置版本控制
- 灰度开关

---

## 9. 输入输出协议设计

## 9.1 invoke 接口

### 请求示例

```json
{
  "request_id": "conn_req_001",
  "execution_id": "exec_001",
  "task_id": "task_789",
  "tenant_id": "t_001",
  "user_id": "u_456",
  "connector_name": "flight_api",
  "operation": "search_flights",
  "args": {
    "from_city": "上海",
    "to_city": "东京",
    "date": "2026-04-27",
    "time_range": "afternoon"
  },
  "runtime_context": {
    "channel": "voice",
    "locale": "zh-CN"
  },
  "idempotency_key": "task_789_search_flights_v1"
}
```

### 响应示例

```json
{
  "request_id": "conn_req_001",
  "execution_id": "exec_001",
  "connector_name": "flight_api",
  "operation": "search_flights",
  "status": "success",
  "result": {
    "data": [
      {
        "flight_no": "MU123",
        "departure_time": "13:20",
        "price": 2300
      }
    ]
  },
  "error": null,
  "metadata": {
    "latency_ms": 358,
    "target_status_code": 200
  }
}
```

---

## 10. Connector Registry 设计

## 10.1 Connector 元数据字段

建议字段包括：

- `connector_name`
- `connector_type`
- `target_system`
- `protocol`
- `base_url`
- `auth_strategy`
- `timeout_ms`
- `max_retries`
- `enabled`
- `config_version`

## 10.2 Operation 元数据字段

建议字段包括：

- `connector_name`
- `operation`
- `method`
- `path`
- `input_schema`
- `output_schema`
- `retryable`
- `idempotent`
- `timeout_ms`
- `error_mapping_profile`

## 10.3 示例

```json
{
  "connector_name": "ticket_system",
  "connector_type": "business_api",
  "protocol": "http",
  "base_url": "https://ticket.example.com",
  "auth_strategy": "service_oauth2",
  "operations": [
    {
      "operation": "create_ticket",
      "method": "POST",
      "path": "/api/tickets",
      "retryable": false,
      "idempotent": true
    }
  ]
}
```

---

## 11. 接入规范设计

## 11.1 标准接入流程

新业务系统接入建议流程：

1. 定义 connector
2. 定义 operations
3. 定义参数 schema
4. 定义 request mapper
5. 定义 response mapper
6. 定义 auth 策略
7. 定义错误映射
8. 完成测试和回放样例

完成 Connector 接入后，若该能力需要被 Agent/LLM 使用，还需要在 Tool Registry 中单独定义 Tool，并配置 Tool 到 Connector Operation 的绑定关系。

## 11.2 新 Connector 的最低要求

必须具备：

- 操作说明文档
- 参数 schema
- 错误映射规则
- 超时与重试配置
- 鉴权配置
- 失败样例

## 11.3 请求映射规范

建议所有 connector 都通过统一 mapper 实现：

- `map_request(args, context) -> target_request`
- `map_response(raw_response) -> normalized_result`
- `map_error(raw_error) -> normalized_error`

---

## 12. 鉴权与安全设计

## 12.1 鉴权策略

支持：

- 平台级服务账号
- 租户级 token
- 用户级 token 透传

## 12.2 推荐原则

- 优先使用平台级或租户级受控凭证
- 只有确有需要时才透传用户凭证
- 所有凭证通过安全配置中心管理

## 12.3 风险控制

- 敏感 connector 单独限权
- 高风险 operation 调用必须记录审计
- connector 级访问白名单

---

## 13. 错误处理设计

## 13.1 错误分类

- `connector_not_found`
- `operation_not_found`
- `invalid_request`
- `auth_failed`
- `permission_denied`
- `target_timeout`
- `target_unavailable`
- `rate_limited`
- `invalid_response`
- `mapping_error`
- `internal_error`

## 13.2 错误结构建议

```json
{
  "code": "target_timeout",
  "message": "Target system timed out",
  "retryable": true,
  "target_status": 504,
  "details": {}
}
```

## 13.3 错误映射原则

- 保留目标系统原始状态码作为 metadata
- 对外统一成平台错误码
- 标注是否可重试

---

## 14. 重试、超时与幂等设计

## 14.1 超时

支持：

- connector 默认超时
- operation 级覆盖超时

## 14.2 重试

建议策略：

- 查询型操作：可重试
- 幂等写操作：按配置重试
- 非幂等写操作：默认不重试

## 14.3 幂等协作

Agent Runtime 传入 `idempotency_key`，Connector Service 应：

- 尽量透传给下游系统
- 若下游不支持，则在平台侧尽可能做请求去重辅助

注意：

Connector Service 不能独立承担所有幂等保证，幂等主责任仍在 Agent Runtime 和业务系统共同承担。

---

## 15. 可观测性与审计设计

## 15.1 关键日志

- connector request summary
- request mapping result
- auth strategy selected
- target request metadata
- target response metadata
- normalized response
- error mapping result

## 15.2 核心指标

- connector invoke count
- per-connector success rate
- per-operation success rate
- average latency
- timeout rate
- retry count
- auth failure rate
- target 5xx rate

## 15.3 审计建议

对敏感操作记录：

- request_id
- execution_id
- tenant_id
- user_id
- connector_name
- operation
- args summary
- result status
- target status code

---

## 16. 数据存储设计

## 16.1 MySQL

用于存储：

- connector metadata
- operation metadata
- auth strategy 配置引用
- 错误映射配置
- connector 级审计索引

## 16.2 Redis

用于存储：

- 热配置缓存
- 短期限流状态
- 熔断状态
- 短期请求幂等辅助数据

---

## 17. 技术选型与语言建议

## 17.1 推荐语言

- `Go`

## 17.2 原因

- Connector Service 是 API 适配与治理层，主要职责是 HTTP/gRPC 调用、鉴权注入、mapping、限流、熔断、重试和错误映射
- Go 在网络调用、并发、部署简洁性、资源占用和可观测性方面非常适合作为 Connector 主语言
- 统一 Go 技术栈可以减少 AI 中台核心服务的跨语言复杂度
- 若个别企业系统只有成熟 Java SDK，可通过独立 adapter service 或 sidecar 接入，不改变 Connector Service 主体语言

## 17.3 推荐技术栈

- `chi` 或 `gin`
- `grpc-go` / `connect-go`
- `net/http` / `resty`
- `sony/gobreaker` 或同类 circuit breaker
- `go-redis`
- `sqlc` 或 `ent`
- `go-sql-driver/mysql`
- `zap` 或 `zerolog`
- `OpenTelemetry Go`
- `testify`

## 17.4 代码组织建议

```text
connector_service/
  api/
  registry/
  config/
  auth/
  adapters/
  mappers/
  resilience/
  storage/
  audit/
  tests/
```

---

## 18. MVP 范围

首期建议实现：

1. `invoke` 接口
2. Connector Registry
3. HTTP 协议适配
4. Request / Response Mapper
5. 基础鉴权支持
6. operation 级超时
7. 查询型重试
8. 标准错误映射
9. 审计日志摘要

MVP 暂不要求：

- gRPC 全面支持
- 异步消息型 connector
- 复杂熔断联动
- 自动 connector 生成

---

## 19. 实施顺序建议

建议按以下顺序实施：

1. 定义 Connector Request / Result Schema
2. 定义 Connector Registry 元数据结构
3. 实现 invoke API
4. 实现 HTTP Protocol Adapter
5. 实现 Request / Response Mapper
6. 实现基础 Auth Manager
7. 实现 Timeout / Retry Controller
8. 实现错误映射
9. 实现审计日志
10. 接入第一个真实业务系统

---

## 20. 总结

Connector Service 是 AI Platform 与真实业务世界之间的受控适配层，它的价值不在于“简单转发请求”，而在于：

- 隔离业务系统差异
- 统一鉴权、超时、重试、错误映射
- 为 Agent Runtime 提供标准化、可治理的调用接口

在项目推进上，Connector Service 应作为 Agent Runtime 之后的关键配套服务尽快落地，因为它直接决定了：

- AI Agent 是否能够稳定接入企业系统
- 复杂业务能力是否能被平台化管理
- 后续新增工具和业务系统的接入成本是否可控
