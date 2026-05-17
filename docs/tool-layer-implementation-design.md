# Tool Layer 实现方式设计文档

## 1. 文档目的

本文档用于细化 AI Platform 中 `Tool Layer` 的实现方式，明确不同类型的 Tool 如何注册、管理、执行、治理和返回结果。

前期文档已经明确：

- `External API` 是外部业务系统原始接口
- `Connector Operation` 是 Connector Service 封装后的平台内部操作
- `Tool` 是面向 Agent/LLM 的能力抽象
- `Agent Runtime` 负责执行 Tool，核心运行时采用 Go 实现
- `Python Code Tool` 通过独立 Python Tool Runner 执行，不进入 Agent Runtime 主进程

本文档进一步回答：

- Tool 有哪些实现方式
- 不同实现方式应该由哪个模块执行
- Tool Registry 如何描述这些不同实现方式
- Tool 执行层如何统一调度
- Python 脚本、API、MCP、Knowledge、Workflow 等方式如何治理
- 在 Go First 技术栈下，如何保留 Python Tool 的灵活性但不影响核心运行时稳定性

---

## 2. 总体设计

## 2.1 Tool 层核心目标

Tool 层需要做到：

- 对 LLM 暴露统一能力描述
- 对 Agent Runtime 暴露统一执行接口
- 对不同底层实现提供 adapter
- 对安全、权限、审计、超时、幂等做统一治理

## 2.2 总体架构

```text
AI Orchestrator
    ->
Tool Plan
    ->
Agent Runtime
    ->
Tool Registry
    ->
Tool Executor
    ->
Tool Adapter
    ->
Underlying Implementation
```

底层实现可以是：

- Connector Operation
- Python Code / Script
- MCP Tool
- Knowledge Retrieval
- Workflow
- Internal Action

语言边界：

- Agent Runtime、Tool Registry、Tool Executor、Connector / Knowledge / Workflow Adapter 采用 Go
- Python 仅用于 Python Tool Runner 及其隔离沙箱
- MCP Gateway 可以优先用 Go 实现，必要时兼容外部 MCP Server 的运行语言

---

## 3. Tool 实现方式分类

## 3.1 Connector Operation Tool

通过 Connector Service 调用外部业务 API。

适合：

- CRM 查询
- 订单查询
- 工单创建
- 日历查询
- 邮件发送
- 企业内部 API 调用

执行链路：

```text
Tool
    ->
Agent Runtime
    ->
Connector Tool Adapter
    ->
Connector Service
    ->
External API
```

## 3.2 Python Code Tool

通过平台托管的 Python 函数或脚本执行逻辑。

适合：

- 数据转换
- 规则计算
- 文本清洗
- 简单算法处理
- LLM 输出后处理
- 离线分析类轻任务

执行链路：

```text
Tool
    ->
Agent Runtime
    ->
Python Tool Adapter
    ->
Python Tool Runner
    ->
Isolated Execution Sandbox
```

## 3.3 MCP Tool

通过 MCP Server 暴露的工具能力执行。

适合：

- 标准化外部工具生态
- 文件系统、数据库、第三方 SaaS 工具
- 已有 MCP Server 能力复用
- 开发者自定义工具接入

执行链路：

```text
Tool
    ->
Agent Runtime
    ->
MCP Tool Adapter
    ->
MCP Gateway / MCP Client
    ->
MCP Server
```

## 3.4 Knowledge Retrieval Tool

通过 Knowledge Service 执行知识库检索。

适合：

- 企业知识问答
- FAQ 检索
- 文档引用
- 规章制度查询

执行链路：

```text
Tool
    ->
Agent Runtime
    ->
Knowledge Tool Adapter
    ->
Knowledge Service
    ->
Vector DB / Rerank
```

## 3.5 Workflow Tool

由多个 step 组成的组合能力。

适合：

- 先查询再创建
- 先检索再调用 API
- 需要确认后继续执行
- 多系统协同任务

执行链路：

```text
Tool
    ->
Agent Runtime
    ->
Workflow Engine
    ->
Step Adapters
```

## 3.6 Internal Action Tool

平台内部动作，不访问外部系统。

适合：

- 取消任务
- 转人工
- 重新开始
- 更新会话状态
- 标记任务完成

执行链路：

```text
Tool
    ->
Agent Runtime
    ->
Internal Action Adapter
    ->
Platform Internal Service
```

---

## 4. Tool Registry 设计

## 4.1 Tool 基础字段

所有 Tool 都应具备统一元数据：

- `tool_id`
- `tool_name`
- `description_for_llm`
- `description_for_operator`
- `tool_type`
- `input_schema`
- `output_schema`
- `risk_level`
- `requires_confirmation`
- `permission_policy`
- `timeout_ms`
- `retry_policy`
- `idempotency_policy`
- `execution_binding`
- `version`
- `status`

## 4.2 tool_type 枚举

建议支持：

- `connector_operation`
- `python_code`
- `mcp`
- `knowledge_retrieval`
- `workflow`
- `internal_action`

## 4.3 execution_binding

`execution_binding` 用于描述 Tool 绑定到哪种底层实现。

### connector_operation 示例

```json
{
  "type": "connector_operation",
  "connector_name": "order_connector",
  "operation": "get_order_detail"
}
```

### python_code 示例

```json
{
  "type": "python_code",
  "runner": "python_tool_runner",
  "entrypoint": "normalize_order_status:run",
  "package_version": "1.0.3",
  "sandbox_profile": "default_no_network"
}
```

### mcp 示例

```json
{
  "type": "mcp",
  "server_id": "filesystem_mcp",
  "tool_name": "read_file",
  "transport": "stdio"
}
```

### knowledge_retrieval 示例

```json
{
  "type": "knowledge_retrieval",
  "kb_ids": ["kb_support"],
  "search_mode": "hybrid",
  "top_k": 5
}
```

### workflow 示例

```json
{
  "type": "workflow",
  "workflow_name": "create_support_ticket_flow"
}
```

---

## 5. Tool Executor 设计

## 5.1 职责

Tool Executor 位于 Agent Runtime 内部，负责：

- 根据 Tool Registry 读取 Tool 定义
- 校验 input_schema
- 校验权限和风险策略
- 选择对应 Tool Adapter
- 控制超时、重试、幂等
- 标准化 Tool Result
- 记录 trace 和 audit

## 5.2 Adapter 模型

建议实现以下 adapter：

- `ConnectorToolAdapter`
- `PythonCodeToolAdapter`
- `McpToolAdapter`
- `KnowledgeToolAdapter`
- `WorkflowToolAdapter`
- `InternalActionAdapter`

统一接口：

```text
execute(tool_definition, args, runtime_context) -> tool_result
```

## 5.3 标准执行流程

```text
1. 接收 Tool Plan
2. 查询 Tool Registry
3. 校验 Tool 状态和版本
4. 校验输入参数
5. 校验权限与风险策略
6. 根据 tool_type 选择 Adapter
7. 执行底层能力
8. 标准化 Tool Result
9. 写入执行记录和审计日志
10. 返回给 AI Orchestrator
```

---

## 6. Connector Operation Tool 实现

## 6.1 适用边界

只要 Tool 的底层能力是外部业务系统 API，就应通过 Connector Service，不应由 Tool 直接调用外部 API。

## 6.2 Adapter 职责

`ConnectorToolAdapter` 负责：

- 将 Tool args 映射为 Connector Request
- 调用 Connector Service
- 接收 Connector Result
- 转换为 Tool Result

## 6.3 设计原则

- Tool 不感知 URL、method、headers
- Agent Runtime 不管理外部系统凭证
- Connector Service 负责协议、鉴权、错误映射

---

## 7. Python Code Tool 实现

## 7.1 适用边界

Python Code Tool 适合轻量逻辑和 AI 相关处理，但不建议用它直接封装核心业务系统 API。

适合：

- 纯计算
- 数据清洗
- 格式转换
- 文本规范化
- 小型算法
- RAG 后处理

不建议：

- 长时间运行任务
- 高风险写操作
- 绕过 Connector 直接访问业务系统
- 承载复杂企业鉴权

## 7.2 Python Tool Runner

建议单独建设 `Python Tool Runner`，不要让 Agent Runtime 直接执行用户脚本。

职责：

- 加载已发布 Python Tool Package
- 在隔离环境中运行 entrypoint
- 控制 timeout、memory、network
- 返回标准结果

## 7.3 安全隔离

至少需要：

- 进程级隔离
- 超时控制
- 内存限制
- 网络访问策略
- 文件系统访问策略
- 包依赖白名单

沙箱 profile 示例：

- `default_no_network`
- `network_allowlist`
- `readonly_filesystem`

## 7.4 Python Tool 包结构

建议规范：

```text
tool_package/
  tool.json
  main.py
  requirements.txt
  tests/
```

`tool.json` 描述：

- tool_name
- input_schema
- output_schema
- entrypoint
- dependencies
- sandbox_profile

## 7.5 Python 函数签名

建议统一：

```python
def run(args: dict, context: dict) -> dict:
    ...
```

返回值必须符合 output_schema。

## 7.6 发布流程

```text
1. 上传 Python Tool Package
2. 静态检查
3. 依赖检查
4. 单元测试
5. 沙箱执行测试
6. 提交审批
7. 发布版本
```

---

## 8. MCP Tool 实现

## 8.1 适用边界

MCP Tool 适合接入标准化工具生态，尤其是已有 MCP Server 能力。

适合：

- 文件系统工具
- 数据库工具
- 第三方 SaaS 工具
- 开发者工具
- 企业内部 MCP Server

## 8.2 MCP Gateway

建议引入 `MCP Gateway`，不要让 Agent Runtime 直接管理所有 MCP Server 连接细节。

职责：

- 管理 MCP Server 注册
- 发现 MCP Tools
- 管理连接、鉴权、transport
- 将 MCP Tool 调用结果标准化

## 8.3 MCP Registry

字段建议：

- `server_id`
- `server_name`
- `transport`
- `command` 或 `url`
- `auth_strategy`
- `allowed_tools`
- `status`
- `version`

## 8.4 MCP Tool 映射

MCP Server 暴露的工具不应默认全部暴露给 LLM。

需要通过 Tool Registry 显式创建 Tool：

```text
MCP Server Tool
    ->
Tool Registry
    ->
MCP Tool Adapter
```

这样可以补充：

- 风险等级
- 权限策略
- 参数约束
- 描述优化
- 确认策略

## 8.5 安全策略

MCP Tool 必须重点控制：

- 可访问路径
- 可执行命令
- 可连接网络
- 可访问数据库
- 租户隔离
- 审计记录

---

## 9. Knowledge Retrieval Tool 实现

## 9.1 适用边界

知识库检索作为 Tool 使用时，通常用于：

- FAQ 问答
- 企业文档检索
- 规章制度查询
- 答案引用来源

## 9.2 Adapter 职责

`KnowledgeToolAdapter` 负责：

- 将 Tool args 转换为 Retrieval Request
- 附加 tenant、kb、permission_context
- 调用 Knowledge Service
- 返回标准 Tool Result

## 9.3 设计原则

- Tool Registry 管理知识检索 Tool 的可见性
- Knowledge Service 负责真实 retrieval
- 权限过滤必须在 Knowledge Service 检索阶段完成

---

## 10. Workflow Tool 实现

## 10.1 适用边界

当一个能力需要多个步骤才能完成时，应使用 Workflow Tool。

典型场景：

- 先查客户，再创建工单
- 先检索知识，再生成邮件草稿
- 先确认，再提交审批

## 10.2 Workflow Engine 职责

- 管理 step 状态
- 调用不同 Tool Adapter
- 处理条件分支
- 处理确认节点
- 处理失败和部分结果

## 10.3 Step 可用类型

- `connector_operation`
- `tool`
- `knowledge_retrieval`
- `python_code`
- `mcp`
- `condition`
- `confirmation`
- `human_handoff`

---

## 11. Internal Action Tool 实现

## 11.1 适用边界

Internal Action Tool 用于平台内部动作，不应被当作外部 API 能力。

典型场景：

- 取消当前任务
- 转人工
- 清空当前任务状态
- 结束会话
- 标记用户确认

## 11.2 设计原则

- 由平台内部服务实现
- 参数应非常收敛
- 风险策略明确
- 不依赖外部系统协议

---

## 12. Tool Result 统一协议

所有实现方式都必须返回统一 Tool Result：

```json
{
  "tool_name": "query_order_status",
  "execution_id": "exec_001",
  "status": "success",
  "result_type": "structured_data",
  "result": {
    "summary": "订单已发货，预计明天送达。",
    "data": {}
  },
  "error": null,
  "metadata": {
    "adapter": "connector_operation",
    "latency_ms": 320
  }
}
```

status 枚举：

- `success`
- `partial_success`
- `failed`
- `cancelled`
- `timed_out`
- `awaiting_confirmation`

---

## 13. 运营管理设计

## 13.1 Tool 创建流程

```text
1. 选择 tool_type
2. 填写 Tool 描述
3. 定义 input_schema / output_schema
4. 配置 execution_binding
5. 配置权限、风险、确认策略
6. 配置测试用例
7. 沙箱测试
8. 提交审批
9. 发布
```

## 13.2 不同类型的测试方式

### connector_operation

- 测试 Connector Operation
- 验证请求/响应映射
- 验证错误映射

### python_code

- 静态检查
- 单元测试
- 沙箱执行
- 依赖检查

### mcp

- 测试 MCP Server 连接
- 发现可用工具
- 执行样例调用
- 校验权限边界

### knowledge_retrieval

- 测试召回结果
- 查看 chunk、score、metadata
- 检查权限过滤

### workflow

- 单步测试
- 全流程测试
- 模拟确认和失败

---

## 14. 安全与治理

## 14.1 通用治理

所有 Tool 必须支持：

- 输入 schema 校验
- 输出 schema 校验
- 权限校验
- 风险等级
- 超时控制
- 审计日志

## 14.2 高风险 Tool

高风险 Tool 包括：

- 发邮件
- 删除数据
- 提交审批
- 下单/支付
- 修改关键业务数据

必须要求：

- 用户确认
- 审批发布
- 执行审计
- 幂等控制

## 14.3 Python / MCP 特殊治理

Python 和 MCP Tool 风险更高，需要额外控制：

- 禁止默认访问内网
- 禁止默认访问文件系统敏感路径
- 禁止未审批依赖
- 禁止未登记 MCP Server
- 所有执行必须可追踪

---

## 15. 推荐实施顺序

建议按以下顺序建设：

1. Tool Registry 基础模型
2. ConnectorToolAdapter
3. KnowledgeToolAdapter
4. WorkflowToolAdapter
5. Python Tool Runner
6. PythonCodeToolAdapter
7. MCP Gateway
8. McpToolAdapter
9. Tool 沙箱测试能力
10. Tool 发布审批和审计

---

## 16. 总结

Tool 层的关键不是把所有能力都做成同一种实现，而是保持上层抽象统一、底层实现可插拔。

建议最终形成：

```text
Tool Registry
    ->
Tool Executor
    ->
Tool Adapter
    ->
Connector / Python Runner / MCP Gateway / Knowledge Service / Workflow Engine
```

其中：

- API 类能力走 Connector Service
- Python 逻辑走 Python Tool Runner
- MCP 能力走 MCP Gateway
- 知识检索走 Knowledge Service
- 多步骤任务走 Workflow Engine
- 平台内部动作走 Internal Action Adapter

这样既能支持快速扩展工具能力，也能保持权限、审计、发布和运行时治理的一致性。
