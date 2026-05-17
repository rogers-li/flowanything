# Agent 推理与执行模式设计文档

## 1. 文档目的

本文档用于定义 AI 中台中不同 Agent 推理与执行模式的设计方式，重点说明：

- 常见推理与执行模式在本项目中的落地方式
- AI Orchestrator、Agent Runtime、Tool Layer、Knowledge Service、Connector Service 如何协作
- 不同推理与执行模式需要哪些内置提示词
- 如何通过 Skill Strategy 和 Runtime Decision 支持多种模式
- MVP 阶段应优先实现哪些模式

本文档面向：

- AI Orchestrator 设计
- Agent Runtime 设计
- Prompt / Policy 运营
- Tool / Skill / Agent Profile 产品设计

---

## 2. 总体设计思路

## 2.1 推理与执行模式不是平台架构

在本项目中，`Direct Answer`、`RAG`、`Tool Calling`、`ReAct`、`Plan-and-Execute`、`Workflow`、`Reflection`、`Multi-Agent` 不应被设计成完全独立的服务。

它们更准确地说是推理模式、执行模式或设计模式，运行在统一的平台架构之上。

真正的平台架构是：

- AI Orchestrator
- Agent Runtime
- Tool Layer
- Connector Service
- Knowledge Service
- Resource Registry
- Admin Console

这些模式不直接决定服务边界，而是在 AI Orchestrator、Agent Runtime 和 Tool Layer 内部被配置和调用。

统一协作关系如下：

```text
User Event
    ->
AI Orchestrator
    ->
Runtime Decision Router
    ->
Pattern Controller
    ->
Agent Runtime / Knowledge Service / Tool Layer
    ->
Observation / Tool Result / Retrieval Result
    ->
AI Orchestrator
    ->
Response Planner
```

## 2.2 各系统职责

### AI Orchestrator

负责：

- 加载 Skill 预设策略
- 在当前 Skill 约束内选择下一步动作
- 加载 Skill / Prompt / Policy
- 控制推理流程
- 生成 Tool Plan / Execution Plan
- 融合 Observation
- 生成最终用户响应

### Agent Runtime

负责：

- 执行 Tool
- 执行 Workflow
- 管理确认、幂等、重试、超时
- 返回 Tool Result / Observation

### Tool Layer

负责：

- 管理 Tool Registry
- 选择 Tool Adapter
- 将不同底层能力统一成 Tool Result

### Connector Service

负责：

- 封装外部业务系统 API
- 提供 Connector Operation
- 不直接暴露 Tool 给 LLM

### Knowledge Service

负责：

- 文档检索
- metadata filter
- rerank
- 返回 Retrieval Result

### Resource Registry

负责：

- 管理 Agent Profile
- 管理 Skill
- 管理 Tool Set
- 管理 Prompt / Policy / Model Route
- 管理 Skill Strategy、推理模式和执行模式配置

---

## 3. Prompt 分层设计

Agent 提示词不应只有一个大 prompt，而应分层管理。

## 3.1 Global System Prompt

所有推理与执行模式共用的基础约束。

示例：

```text
You are an enterprise AI agent operating inside a controlled AI platform.

Follow these rules:
- Use only the tools explicitly provided in the current context.
- Do not invent tool results or external facts.
- If required information is missing, ask a concise clarification question.
- If an operation is high risk, request confirmation before execution.
- Do not expose private reasoning. Provide only concise rationale or structured decisions when needed.
- Respect tenant, user, channel, locale, and policy constraints.
```

## 3.2 Skill Prompt

由 Skill 提供的场景指令。

示例：

```text
You are operating under the Customer Support skill.

Goal:
- Help users resolve order, logistics, refund, and support ticket issues.

Behavior:
- Prefer concise answers.
- Ask for missing order identifiers when needed.
- Confirm before creating or updating support tickets.
- Use the support knowledge base when answering policy questions.
- Escalate to human support when the user's issue cannot be resolved with available tools.
```

## 3.3 Pattern Prompt

由当前推理或执行模式决定的提示词，例如 RAG、Tool Calling、Plan-and-Execute。

## 3.4 Tool Prompt

由 Tool Registry 提供，包含 Tool 名称、描述、参数 schema、使用约束。

## 3.5 Output Schema Prompt

要求模型输出结构化结果，例如：

- runtime decision
- tool plan
- execution plan
- retrieval answer
- verification result
- final response

---

## 4. Skill Strategy 与 Pattern 配置

不建议根据用户每一句话动态选择全局推理模式。更稳妥的方式是由 Agent Profile / Skill 在设计期定义默认模式和允许模式，运行时只在这些约束内选择下一步动作。

建议在 Resource Registry 中定义 `Skill Strategy`，由 AI Orchestrator 加载并执行。

示例：

```json
{
  "skill_id": "customer_support",
  "default_pattern": "workflow",
  "allowed_patterns": ["workflow", "rag", "tool_calling"],
  "allowed_tool_sets": ["support_tools"],
  "allowed_tools": ["query_order_status", "create_support_ticket"],
  "knowledge_scopes": ["kb_support"],
  "max_steps": 5,
  "requires_verification": true,
  "requires_confirmation_for_high_risk": true,
  "model_route": {
    "default": "fast_model",
    "complex_task": "reasoning_model"
  }
}
```

支持的 `pattern`：

- `direct_answer`
- `rag`
- `tool_calling`
- `react`
- `plan_and_execute`
- `workflow`
- `reflection`
- `multi_agent`

---

## 5. Runtime Decision Router

## 5.1 职责

Runtime Decision Router 位于 AI Orchestrator 内部，负责在当前 Skill Strategy 的约束内决定“下一步做什么”。

它不负责在每轮对话中自由选择 ReAct、RAG、Workflow 或 Plan-and-Execute 这类大模式。

更准确的职责是：

- 是否直接回答
- 是否补槽
- 是否检索知识库
- 是否调用 Tool
- 是否进入 Workflow 的下一节点
- 是否请求确认
- 是否转人工
- 是否触发 fallback

## 5.2 输入信号

- 用户输入
- 当前 Skill
- 当前任务状态
- 可用 Tool Set
- Skill 预设的 default_pattern
- Skill 允许的 allowed_patterns
- 是否需要知识检索
- 是否需要外部 API
- 是否多步骤
- 是否高风险
- 是否需要强流程
- 渠道类型
- 延迟预算

## 5.3 内置提示词

```text
Decide the next safe runtime action under the current skill strategy.

Allowed runtime actions:
- direct_answer
- ask_clarification
- retrieve_knowledge
- call_tool
- continue_workflow
- ask_confirmation
- handoff_human
- fallback

Rules:
- Stay within the current skill's allowed patterns and tools.
- Prefer task state and workflow state over free-form classification.
- If required context is missing, choose ask_clarification or retrieve_knowledge.
- Do not select a broad pattern just because the request is ambiguous.
- Do not expose private reasoning.

Return JSON only:
{
  "next_action": "...",
  "candidate_pattern": "...",
  "confidence": 0.0,
  "requires_tool": true,
  "requires_knowledge": false,
  "requires_confirmation": false,
  "missing_context": [],
  "reason_summary": "short explanation without private reasoning"
}
```

---

## 6. Direct Answer 模式

## 6.1 适用场景

- 简单问答
- 闲聊
- 无需工具的说明类问题
- 不依赖企业知识库的通用解释

## 6.2 模块协作

```text
User Event
    ->
AI Orchestrator
    ->
Runtime Decision Router: direct_answer
    ->
LLM Response Generation
    ->
Response Planner
    ->
speak / display_text
```

Agent Runtime、Tool Layer、Connector Service、Knowledge Service 通常不参与。

## 6.3 内置提示词

```text
Answer the user's question directly.

Rules:
- Do not call tools.
- Do not claim access to external systems.
- If the question requires private enterprise data, say that retrieval or tools are needed.
- Keep the answer concise for voice channel.
- If channel is text, include useful detail when appropriate.

Return JSON only:
{
  "answer": "...",
  "needs_tool": false,
  "needs_knowledge": false,
  "follow_up_question": null
}
```

---

## 7. RAG 模式

## 7.1 适用场景

- 企业知识问答
- FAQ
- 政策制度查询
- 产品文档问答

## 7.2 模块协作

简单知识问答：

```text
AI Orchestrator
    ->
Query Rewrite
    ->
Knowledge Service
    ->
Retrieval Result
    ->
Grounded Answer Generation
    ->
Response Planner
```

复杂任务中的知识检索：

```text
AI Orchestrator
    ->
Tool Plan: search_knowledge_base
    ->
Agent Runtime
    ->
KnowledgeToolAdapter
    ->
Knowledge Service
    ->
Tool Result
```

## 7.3 内置提示词

### Query Rewrite Prompt

```text
Rewrite the user request into one or more concise retrieval queries.

Rules:
- Preserve the user's intent.
- Include important entities, product names, dates, and constraints.
- Do not add assumptions that are not present.
- Use the user's locale unless the knowledge base requires another language.

Return JSON only:
{
  "queries": ["..."],
  "filters": {
    "language": "...",
    "source_type": null
  }
}
```

### Grounded Answer Prompt

```text
Answer the user using only the provided retrieval results.

Rules:
- If the retrieval results are insufficient, say what is missing.
- Do not invent facts beyond the retrieved content.
- For voice channel, produce a short spoken summary.
- For text channel, include concise details and source references when available.

Return JSON only:
{
  "answer_summary": "...",
  "answer_details": "...",
  "used_chunk_ids": ["..."],
  "confidence": 0.0,
  "needs_clarification": false,
  "clarification_question": null
}
```

---

## 8. Tool Calling 模式

## 8.1 适用场景

- 查询订单
- 查询物流
- 创建简单工单
- 查询日历
- 调用单个明确 Tool

## 8.2 模块协作

```text
AI Orchestrator
    ->
Tool Selection / Tool Plan
    ->
Agent Runtime
    ->
Tool Registry
    ->
Tool Executor
    ->
Tool Adapter
    ->
Connector / Knowledge / Python / MCP
    ->
Tool Result
    ->
AI Orchestrator
    ->
Response Planner
```

## 8.3 内置提示词

### Tool Selection Prompt

```text
Select the best tool for the user's request from the available tools.

Rules:
- Use only tools listed in available_tools.
- If no tool fits, return no_tool.
- If required arguments are missing, ask for clarification instead of guessing.
- If the selected tool is high risk, mark requires_confirmation as true.
- Do not expose private reasoning.

Return JSON only:
{
  "selected_tool": "...",
  "confidence": 0.0,
  "args": {},
  "missing_args": [],
  "requires_confirmation": false,
  "reason_summary": "short explanation"
}
```

### Tool Result Response Prompt

```text
Generate a user-facing response from the tool result.

Rules:
- Do not add facts not present in the tool result.
- For voice channel, summarize only the most important result.
- For text channel, include structured details when useful.
- If the tool failed, explain the failure simply and suggest the next action.

Return JSON only:
{
  "summary": "...",
  "details": "...",
  "next_choices": [],
  "task_completed": true
}
```

---

## 9. ReAct 模式

## 9.1 适用场景

- 工具路径不确定
- 需要边观察边决定下一步
- 开放式复杂任务

## 9.2 模块协作

```text
AI Orchestrator
    ->
ReAct Controller
    ->
LLM Next Action
    ->
Agent Runtime executes Tool
    ->
Observation
    ->
AI Orchestrator continues loop
    ->
Final Response
```

## 9.3 控制约束

必须配置：

- `max_steps`
- `allowed_tools`
- `max_tool_calls`
- `budget_limit`
- `loop_detection`
- `confirmation_gate`

## 9.4 内置提示词

```text
You are controlling an iterative tool-use process.

At each step, choose exactly one of:
- call_tool
- ask_clarification
- final_answer

Rules:
- Use only available tools.
- Do not repeat the same failed tool call with the same arguments.
- Stop when enough information is available.
- Ask for clarification if required information is missing.
- High risk actions require confirmation before execution.
- Do not reveal private reasoning. Output a short action rationale only.

Return JSON only:
{
  "next_step": "call_tool | ask_clarification | final_answer",
  "tool_name": null,
  "args": {},
  "clarification_question": null,
  "final_answer": null,
  "rationale_summary": "short explanation",
  "should_stop": false
}
```

---

## 10. Plan-and-Execute 模式

## 10.1 适用场景

- 多步骤任务
- 企业流程
- 需要可审计计划
- 需要先计划再执行

## 10.2 模块协作

```text
AI Orchestrator
    ->
Planner generates Execution Plan
    ->
Plan Validator
    ->
Agent Runtime
    ->
Step Execution
    ->
Observation Summary
    ->
AI Orchestrator final response
```

## 10.3 内置提示词

### Planner Prompt

```text
Create a structured execution plan for the user's task.

Rules:
- Use only available tools.
- Keep the plan minimal.
- Identify missing information before execution.
- Mark steps that require user confirmation.
- Do not include hidden reasoning. Include only concise step purpose.

Return JSON only:
{
  "can_execute": true,
  "missing_information": [],
  "requires_user_confirmation": false,
  "plan": [
    {
      "step_id": "s1",
      "tool_name": "...",
      "args": {},
      "depends_on": [],
      "requires_confirmation": false,
      "purpose": "short purpose"
    }
  ]
}
```

### Plan Repair Prompt

```text
Repair the execution plan based on the failed step and observation.

Rules:
- Do not retry non-retryable operations.
- Preserve completed steps.
- If the failure requires user input, ask for clarification.
- If the task cannot continue, return cannot_continue.

Return JSON only:
{
  "action": "continue_with_repaired_plan | ask_clarification | cannot_continue",
  "repaired_steps": [],
  "clarification_question": null,
  "failure_summary": "short summary"
}
```

---

## 11. Workflow 模式

## 11.1 适用场景

- 固定业务流程
- 高风险流程
- 工单、审批、订单、客服流程
- 状态机明确的任务

## 11.2 模块协作

```text
AI Orchestrator
    ->
Workflow Match / Slot Fill
    ->
Agent Runtime Workflow Engine
    ->
Step Execution
    ->
ask_question / ask_confirmation / continue
    ->
Workflow Complete
```

## 11.3 内置提示词

### Workflow Match Prompt

```text
Determine whether the user's request matches one of the available workflows.

Rules:
- Select a workflow only if the user intent clearly matches it.
- If no workflow matches, return no_match.
- Extract known slots.
- List missing slots.

Return JSON only:
{
  "matched_workflow": "...",
  "confidence": 0.0,
  "slots": {},
  "missing_slots": [],
  "needs_clarification": false,
  "clarification_question": null
}
```

### Workflow Step Prompt

```text
Given the workflow definition, current step, known slots, and previous step results, decide the next workflow action.

Allowed actions:
- execute_step
- ask_slot
- ask_confirmation
- complete
- fail

Return JSON only:
{
  "action": "...",
  "step_id": "...",
  "args": {},
  "question": null,
  "confirmation_message": null,
  "completion_summary": null
}
```

---

## 12. Reflection / Verification 模式

## 12.1 适用场景

- 高风险操作前检查
- Tool 参数检查
- RAG 答案事实性检查
- 最终回复质量检查

## 12.2 模块协作

```text
Draft Plan / Draft Answer / Tool Args
    ->
Verification Module
    ->
pass / revise / reject
```

AI Orchestrator 负责调用 Verifier。Agent Runtime 也会执行确定性的 schema、权限、风险校验。

## 12.3 内置提示词

### Tool Argument Verification Prompt

```text
Verify whether the proposed tool call is safe and valid.

Check:
- Required arguments are present.
- Arguments match the schema.
- The operation matches the user's intent.
- High risk actions are marked for confirmation.
- No unauthorized data access is requested.

Return JSON only:
{
  "verdict": "pass | revise | reject",
  "issues": [],
  "revised_args": {},
  "requires_confirmation": false,
  "summary": "short verification summary"
}
```

### RAG Answer Verification Prompt

```text
Verify whether the answer is supported by the retrieval results.

Rules:
- Mark unsupported claims.
- Do not introduce new facts.
- If the answer is not sufficiently grounded, return revise or reject.

Return JSON only:
{
  "verdict": "pass | revise | reject",
  "unsupported_claims": [],
  "revision_instruction": null,
  "confidence": 0.0
}
```

---

## 13. Multi-Agent 协作模式

## 13.1 适用场景

- 复杂研究
- 长任务
- 多角色协作
- 多领域专家组合

## 13.2 模块协作

```text
AI Orchestrator
    ->
Coordinator
    ->
Sub Agent Profiles
    ->
Each Agent uses Skill / Tool Set
    ->
Agent Runtime executes tools
    ->
Coordinator aggregates result
```

## 13.3 内置提示词

### Coordinator Prompt

```text
Break the user task into sub-tasks for available specialist agents.

Rules:
- Use only available specialist agents.
- Assign clear, non-overlapping tasks.
- Define expected output for each sub-agent.
- Avoid unnecessary delegation.
- Track dependencies between sub-tasks.

Return JSON only:
{
  "sub_tasks": [
    {
      "agent_profile": "...",
      "task": "...",
      "expected_output": "...",
      "depends_on": []
    }
  ],
  "aggregation_plan": "short plan"
}
```

### Aggregation Prompt

```text
Combine specialist agent outputs into a final response.

Rules:
- Resolve conflicts explicitly.
- Do not invent missing results.
- If outputs are incomplete, state what remains unresolved.
- Produce a concise final answer.

Return JSON only:
{
  "final_summary": "...",
  "details": "...",
  "unresolved_items": [],
  "confidence": 0.0
}
```

---

## 14. 推理与执行模式的系统模块映射

| 推理/执行模式 | AI Orchestrator | Agent Runtime | Tool Layer | Knowledge Service | Connector Service |
| --- | --- | --- | --- | --- | --- |
| Direct Answer | 生成回答 | 不参与 | 不参与 | 不参与 | 不参与 |
| RAG | 控制检索与答案生成 | 可选参与 | Knowledge Tool | 检索/rerank | 不参与 |
| Tool Calling | 生成 Tool Plan | 执行 Tool | 选择 Adapter | 可作为 Tool | API Tool 底层 |
| ReAct | 控制循环 | 执行每步 Tool | 提供 Tool | 可作为 Tool | 可作为 Tool 底层 |
| Plan-and-Execute | 生成计划 | 执行计划 | 执行步骤 | 可作为步骤 | 可作为步骤 |
| Workflow | 匹配流程/补槽 | 执行 Workflow | 执行节点 | 可作为节点 | 可作为节点 |
| Reflection | 验证计划/答案 | 确定性校验 | 校验 Tool | 检查引用 | 不参与 |
| Multi-Agent | 协调子 Agent | 统一执行工具 | 共享 Tool | 共享知识 | 共享 API |

---

## 15. 推荐落地优先级

## 15.1 第一阶段

实现：

- `direct_answer`
- `rag`
- `tool_calling`

原因：

- 覆盖简单问答、知识问答、单工具调用
- 是 AI 中台最基础的能力闭环

## 15.2 第二阶段

实现：

- `plan_and_execute`
- `workflow`
- `reflection`

原因：

- 支持企业多步骤任务
- 支持高风险流程
- 提升稳定性和可审计性

## 15.3 第三阶段

实现：

- `react`
- `multi_agent`

原因：

- 更适合开放复杂任务
- 对治理、成本、观测要求更高

---

## 16. 落地示例：售后客服 Workflow 模式

本节以“售后客服创建工单”为例，说明推理与执行模式如何通过平台配置和运行时服务真正落地。

## 16.1 场景定义

用户说：

```text
我的订单还没收到，帮我处理一下。
```

目标流程：

1. 识别用户要处理售后问题
2. 补齐订单号或用户身份
3. 查询订单和物流
4. 判断是否需要创建工单
5. 创建工单前请求确认
6. 调用工单系统
7. 给用户播报结果

该场景适合使用 `Workflow` 模式，因为流程明确、风险可控、结果可审计。

## 16.2 产品配置层

### Connector 配置

先接入外部业务系统 API。

示例 Connector：

- `order_connector.get_order_detail`
- `order_connector.get_logistics_status`
- `ticket_connector.create_ticket`

Connector 管理中需要配置：

- base_url
- auth
- request mapping
- response mapping
- timeout
- retry
- error mapping

此时这些能力只是 `Connector Operation`，还不是 LLM/Agent 可见的 Tool。

### Tool 配置

将 Connector Operation 包装成 Tool。

`query_order_status`：

```json
{
  "tool_type": "connector_operation",
  "execution_binding": {
    "connector_name": "order_connector",
    "operation": "get_order_detail"
  },
  "risk_level": "low",
  "requires_confirmation": false
}
```

`query_logistics_status`：

```json
{
  "tool_type": "connector_operation",
  "execution_binding": {
    "connector_name": "order_connector",
    "operation": "get_logistics_status"
  },
  "risk_level": "low",
  "requires_confirmation": false
}
```

`create_support_ticket`：

```json
{
  "tool_type": "connector_operation",
  "execution_binding": {
    "connector_name": "ticket_connector",
    "operation": "create_ticket"
  },
  "risk_level": "medium",
  "requires_confirmation": true
}
```

### Workflow 配置

创建 `after_sales_ticket_workflow`：

```json
{
  "workflow_name": "after_sales_ticket_workflow",
  "input_schema": {
    "order_id": "string",
    "issue_type": "string",
    "description": "string"
  },
  "steps": [
    {
      "step_id": "collect_order_id",
      "type": "slot_fill",
      "slot": "order_id"
    },
    {
      "step_id": "query_order",
      "type": "tool",
      "tool_name": "query_order_status"
    },
    {
      "step_id": "query_logistics",
      "type": "tool",
      "tool_name": "query_logistics_status"
    },
    {
      "step_id": "confirm_create_ticket",
      "type": "confirmation",
      "message_template": "我可以帮你创建一个售后工单，是否继续？"
    },
    {
      "step_id": "create_ticket",
      "type": "tool",
      "tool_name": "create_support_ticket"
    }
  ]
}
```

### Skill 配置

创建 `after_sales_support_skill`：

```json
{
  "skill_name": "after_sales_support",
  "default_pattern": "workflow",
  "allowed_patterns": ["workflow", "rag", "tool_calling"],
  "workflow_candidates": ["after_sales_ticket_workflow"],
  "tool_sets": ["after_sales_tools"],
  "knowledge_scopes": ["kb_after_sales_policy"],
  "instructions": "Handle after-sales issues. Confirm before creating tickets."
}
```

该配置表示：

- 售后 Skill 默认使用 Workflow 模式
- 流程节点中可以调用 RAG
- 可使用售后 Tool Set
- 可访问售后政策知识库

### Agent Profile 配置

将 Skill 绑定到客服 Agent：

```json
{
  "agent_name": "customer_service_agent",
  "enabled_skills": ["after_sales_support"],
  "channel": ["voice", "text"],
  "locale": ["zh-CN"]
}
```

## 16.3 系统协作链路

```text
User Event
  ->
AI Orchestrator
  ->
Skill Resolver
  ->
Load after_sales_support_skill
  ->
Runtime Decision Router
  ->
Workflow Match / Slot Fill
  ->
Agent Runtime Workflow Engine
  ->
Tool Executor
  ->
Connector Service
  ->
External Business API
```

职责划分：

- AI Orchestrator：加载 Skill、匹配 Workflow、补槽、生成对话动作、组织最终回复
- Agent Runtime：执行 Workflow、维护 step state、处理确认、重试、超时
- Tool Layer：根据 tool_name 查询 Tool Registry，选择 Tool Adapter
- Connector Service：调用订单系统、物流系统、工单系统
- Knowledge Service：在用户询问售后政策时提供 RAG 能力

## 16.4 运行时流程

### 用户首次输入

```json
{
  "type": "user_utterance_committed",
  "text": "我的订单还没收到，帮我处理一下",
  "channel": "voice"
}
```

AI Orchestrator 加载 Skill 后识别：

```json
{
  "skill": "after_sales_support",
  "default_pattern": "workflow",
  "workflow_candidate": "after_sales_ticket_workflow"
}
```

Runtime Decision Router 发现缺少 `order_id`：

```json
{
  "next_action": "ask_clarification",
  "missing_context": ["order_id"],
  "question": "请告诉我订单号，或者说一下你要处理的是哪一笔订单。"
}
```

返回语音动作：

```json
{
  "type": "speak",
  "text": "请告诉我订单号，或者说一下你要处理的是哪一笔订单。"
}
```

### 用户补充订单号

用户说：

```text
订单号是 123456
```

更新 Task State：

```json
{
  "slots": {
    "order_id": "123456",
    "issue_type": "not_received"
  },
  "workflow_state": {
    "workflow_name": "after_sales_ticket_workflow",
    "current_step": "query_order"
  }
}
```

### 执行订单查询

Orchestrator 发起 workflow 执行：

```json
{
  "workflow_name": "after_sales_ticket_workflow",
  "task_id": "task_001",
  "slots": {
    "order_id": "123456"
  }
}
```

Agent Runtime 执行 `query_order_status`，Tool Executor 查询 Tool Registry：

```json
{
  "tool_name": "query_order_status",
  "tool_type": "connector_operation",
  "execution_binding": {
    "connector_name": "order_connector",
    "operation": "get_order_detail"
  }
}
```

然后调用 Connector Service：

```json
{
  "connector_name": "order_connector",
  "operation": "get_order_detail",
  "args": {
    "order_id": "123456"
  }
}
```

### 执行物流查询

Agent Runtime 继续执行：

```json
{
  "tool_name": "query_logistics_status",
  "args": {
    "order_id": "123456"
  }
}
```

得到：

```json
{
  "status": "delayed",
  "estimated_delivery": "2026-04-29"
}
```

### 创建工单前确认

因为 `create_support_ticket` 是中风险 Tool，Workflow 进入确认节点：

```json
{
  "workflow_status": "awaiting_confirmation",
  "message": "我查到这笔订单物流延迟，预计 4 月 29 日送达。我可以帮你创建一个售后工单，是否继续？"
}
```

语音输出：

```json
{
  "type": "speak",
  "text": "我查到这笔订单物流延迟，预计 4 月 29 日送达。我可以帮你创建一个售后工单，是否继续？"
}
```

### 用户确认并创建工单

用户说：

```text
可以
```

当前 Task State 为：

```json
{
  "status": "awaiting_confirmation",
  "pending_action": "create_support_ticket"
}
```

Runtime Decision Router 不重新选择全局模式，只判断这是确认，Agent Runtime 继续执行 `create_support_ticket`：

```json
{
  "tool_name": "create_support_ticket",
  "args": {
    "order_id": "123456",
    "issue_type": "not_received",
    "description": "User reports order not received. Logistics delayed."
  }
}
```

Connector Service 调用工单系统后返回：

```json
{
  "ticket_id": "T20260428001",
  "status": "created"
}
```

### 最终回复

文本端：

```text
已为你创建售后工单 T20260428001。订单当前物流延迟，预计 4 月 29 日送达。
```

语音端：

```text
已帮你创建售后工单，编号是 T20260428001。物流预计明天送达。
```

## 16.5 该例子用到的内置提示词

### Workflow Match Prompt

```text
Determine whether the user's request matches the after_sales_ticket_workflow.

Return JSON:
{
  "matched": true,
  "confidence": 0.0,
  "slots": {},
  "missing_slots": [],
  "issue_type": "..."
}
```

### Slot Fill Prompt

```text
Extract required workflow slots from the user message.

Required slots:
- order_id
- issue_type
- description

Return JSON:
{
  "slots": {},
  "missing_slots": []
}
```

### Confirmation Response Prompt

```text
Generate a concise confirmation message before executing the high-risk action.

Return JSON:
{
  "confirmation_message": "..."
}
```

### Tool Result Response Prompt

```text
Summarize the workflow result for the user.

Rules:
- For voice, be concise.
- For text, include ticket id and important details.
```

这里没有让 LLM 自由选择“使用哪种模式”。Workflow 是 Skill 预设的，LLM 只参与补槽、摘要、确认话术和必要判断。

## 16.6 关键结论

这个例子说明，推理与执行模式的落地不是运行时临场选择模式，而是由 Skill、Workflow、Tool、Connector、Knowledge 这些平台配置对象承载。

Workflow 模式的落地核心是：

```text
Skill 预设模式
  ->
Workflow 定义流程
  ->
Tool 定义能力
  ->
Connector 封装 API
  ->
Agent Runtime 执行
  ->
AI Orchestrator 负责对话和状态推进
```

---

## 17. 总结

本项目不应为每种推理与执行模式单独建设一套系统，而应通过 Skill Strategy、Runtime Decision 和 Pattern Controller，在同一套 AI Platform 上支持多种模式。

核心设计是：

```text
AI Orchestrator = Runtime Decision Router + Pattern Controller
Agent Runtime = Tool / Workflow Execution
Tool Layer = Capability Abstraction + Adapter
Knowledge Service = Retrieval Capability
Connector Service = External API Operation
Resource Registry = Skill Strategy / Pattern / Tool / Prompt / Policy Configuration
```

内置提示词应按层管理：

- Global System Prompt
- Skill Prompt
- Pattern Prompt
- Tool Prompt
- Output Schema Prompt
- Verification Prompt

这样既能支持多种推理与执行模式，又能保证企业场景下的可控性、可测试性、可审计性和可运营性。
