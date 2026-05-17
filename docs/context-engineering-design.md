# 上下文工程设计文档

## 1. 文档目的

本文档用于定义 AI 中台中的上下文工程设计，重点说明：

- Context、短期记忆、长期记忆、任务状态、知识库的区别
- AI Orchestrator 如何构建 LLM Runtime Context
- 如何控制上下文不超过模型上下文窗口
- 如何提炼有价值信息进入长期记忆
- 如何检索长期记忆并确保检索质量
- Context Builder、Memory Service、Knowledge Service 如何协作

本文档面向：

- AI Orchestrator 设计
- Memory Service 设计
- Knowledge Service 设计
- Agent Runtime 状态管理
- Prompt / Policy / Context 运营

---

## 2. 核心概念

## 2.1 Context

`Context` 是一次 LLM 调用前临时组装的输入材料集合。

它不是一个长期存储对象，而是由 Context Builder 在运行时根据当前任务动态生成。

典型组成：

```text
Context = System Prompt
        + Skill Prompt
        + Policy
        + Current Task State
        + Recent Turns
        + Relevant Short-term Memory
        + Relevant Long-term Memory
        + Retrieved Knowledge
        + Available Tools
        + Current User Input
        + Output Schema
```

## 2.2 Short-term Memory

短期记忆服务当前会话和当前任务。

包括：

- 最近几轮对话
- 当前 active task
- 当前槽位
- 当前 workflow step
- 最近 tool result summary
- 最近向用户展示的选项
- 当前 pending confirmation

生命周期：

- 当前 turn
- 当前 session
- 当前 task

推荐存储：

- Redis
- MySQL task state
- session summary table

## 2.3 Long-term Memory

长期记忆跨会话保留，用于个性化和跨任务连续性。

包括：

- 用户偏好
- 用户长期配置
- 业务关系
- 常用设置引用
- 用户明确纠正的信息
- 可复用任务摘要

生命周期：

- 天、周、月、年
- 需要授权、审计、可删除

推荐存储：

- MySQL 作为真相源
- 向量索引作为语义检索辅助

## 2.4 Task State

Task State 是任务状态机，不是记忆。

它描述当前任务推进到哪里：

```json
{
  "task_id": "task_001",
  "intent": "after_sales_ticket",
  "status": "awaiting_confirmation",
  "slots": {
    "order_id": "123456",
    "issue_type": "not_received"
  },
  "workflow_state": {
    "workflow_name": "after_sales_ticket_workflow",
    "current_step": "confirm_create_ticket"
  }
}
```

Task State 应由 Session Service、AI Orchestrator、Agent Runtime 严格管理，不应由 LLM 自由写入。

## 2.5 Knowledge

Knowledge 是企业知识，不是用户长期记忆。

包括：

- 企业文档
- FAQ
- 政策制度
- 产品文档
- 操作手册

由 Knowledge Service 管理。

---

## 3. 记忆分层设计

## 3.1 L0：Runtime Context

一次模型调用的最终上下文。

来源：

- prompt
- task state
- memory
- retrieval result
- tool schema

存储：

- 不长期保存完整上下文
- 记录 trace 和 budget report

## 3.2 L1：Turn Memory

当前轮次内的临时信息。

包括：

- 当前 ASR final text
- 本轮 runtime decision
- 本轮 tool result
- 本轮 response draft

生命周期：

- 当前 turn

## 3.3 L2：Session Memory

当前交互会话内上下文。

包括：

- 最近 N 轮对话
- rolling session summary
- entity cache
- last presented options
- current active task

生命周期：

- 当前 session
- 可在语音静默后保留一段时间

## 3.4 L3：Task Memory

当前任务内状态与历史摘要。

包括：

- slots
- workflow step
- tool execution summary
- confirmation state
- failure history

生命周期：

- task 生命周期
- 任务完成后可保留摘要

## 3.5 L4：Long-term Memory

跨会话保留的用户长期信息。

包括：

- 偏好
- 画像
- 历史任务摘要
- 常用配置
- 业务身份映射

生命周期：

- 长期
- 需要权限、审计、删除能力

---

## 4. 系统职责划分

## 4.1 AI Orchestrator

负责：

- 调用 Context Builder
- 根据当前模式选择 context policy
- 调用 Memory Service 检索长期记忆
- 调用 Knowledge Service 获取检索结果
- 将模型输出中的 memory candidate 交给 Memory Service 判断
- 不直接让 LLM 写长期记忆

## 4.2 Context Builder

建议作为 AI Orchestrator 的核心模块，后续可独立服务化。

负责：

- 收集候选上下文
- 估算 token
- 分配上下文预算
- 排序裁剪
- 摘要压缩
- 组装最终 LLM 输入
- 生成 budget report

## 4.3 Memory Service

负责：

- 长期记忆候选抽取
- 记忆分类
- 记忆策略判断
- 去重和合并
- 长期记忆存储
- 长期记忆检索
- 长期记忆审计
- 用户记忆管理

## 4.4 Session Service

负责：

- interaction session
- active task
- recent turns
- session summary
- 会话挂起/恢复

## 4.5 Agent Runtime

负责：

- task execution state
- workflow step state
- tool result summary
- confirmation state

不负责长期用户记忆。

## 4.6 Knowledge Service

负责：

- 企业知识检索
- metadata filter
- rerank
- retrieval result

不负责用户个人长期记忆。

---

## 5. Context Builder 设计

## 5.1 核心目标

Context Builder 要确保：

- LLM 输入不超过上下文窗口
- 必要信息优先进入上下文
- 可选信息按相关性和预算裁剪
- 每次上下文构建可追踪、可审计、可调优

## 5.2 Token Budget

每次 LLM 调用必须计算可用输入预算。

示例：

```text
model_context_window = 128000
reserved_output = 8000
safety_buffer = 4000
available_input_budget = 116000
```

公式：

```text
available_input_budget = context_window - reserved_output - safety_buffer
```

## 5.3 按调用类型设置预算

不同调用类型应有不同预算：

| 调用类型 | 输入预算策略 |
| --- | --- |
| runtime decision | 小预算 |
| slot filling | 小预算 |
| tool selection | 中预算 |
| tool args generation | 中预算 |
| RAG answer | 中高预算 |
| planning | 高预算 |
| final summary | 中预算 |

## 5.4 按上下文槽位分配预算

示例：

| Context Section | 预算比例 |
| --- | --- |
| System Prompt | 5% |
| Skill Prompt | 5% |
| Policy | 5% |
| Task State | 10% |
| Recent Turns | 15% |
| Long-term Memory | 10% |
| RAG Results | 35% |
| Tool Schema | 10% |
| User Input | 5% |

不同模式可使用不同的 context policy。

例如：

- RAG 模式提高 RAG Results 预算
- Tool Calling 模式提高 Tool Schema 预算
- 语音快速响应模式减少 Recent Turns 和 RAG Results

## 5.5 Mandatory 与 Optional

### Mandatory

必须注入：

- system prompt
- policy
- current user input
- current task state
- output schema
- 必要 tool schema

### Optional

可裁剪：

- 较早对话
- RAG chunk
- 长期记忆
- 非关键 tool schema
- 历史 tool result

如果 mandatory 超预算，需要压缩 prompt、拆分调用或降级。

## 5.6 候选上下文打分

每个上下文片段都应带 metadata：

```json
{
  "type": "rag_chunk",
  "content": "...",
  "token_count": 420,
  "relevance": 0.87,
  "priority": 80,
  "recency": 0.6,
  "source": "kb_support",
  "required": false
}
```

排序可参考：

```text
final_score = relevance * 0.5 + priority * 0.3 + recency * 0.2
```

## 5.7 超预算降级顺序

当上下文超预算时，按顺序处理：

1. 丢弃低分 RAG chunk
2. 丢弃低相关长期记忆
3. 减少历史对话轮数
4. 使用 session summary 替代完整历史
5. 压缩 tool schema
6. 压缩 retrieved content
7. 拆分成多次 LLM 调用
8. 降级为澄清问题

禁止简单从尾部截断。

## 5.8 Budget Report

每次上下文构建都应记录 budget report：

```json
{
  "model": "xxx",
  "context_window": 128000,
  "available_input_budget": 116000,
  "used_tokens": 38200,
  "sections": {
    "system": 1200,
    "skill": 800,
    "task_state": 600,
    "recent_turns": 3200,
    "rag": 22000,
    "tools": 7000,
    "memory": 1200,
    "output_schema": 900
  },
  "dropped": {
    "rag_chunks": 8,
    "memories": 3,
    "turns": 12
  }
}
```

---

## 6. 短期记忆管理

## 6.1 Recent Turns

保存最近 N 轮原始对话。

用途：

- 指代消解
- 接话
- 语音中的“刚才那个”

建议：

- MVP 最近 5 轮
- 长会话用 rolling summary

## 6.2 Session Summary

对较长会话做滚动摘要。

用途：

- 控制上下文长度
- 会话恢复
- 长对话压缩

## 6.3 Entity Cache

保存最近出现的关键实体：

- order_id
- destination
- date
- product_name
- ticket_id
- option_index

语音场景尤其重要，因为用户常说：

- “就这个”
- “刚才那个”
- “第二个”

## 6.4 Last Presented Options

保存最近展示给用户的选项。

例如系统播报：

```text
我找到三个航班，第一个 13:20，第二个 16:45，第三个 19:10。
```

用户说：

```text
第二个吧
```

系统需要从 `last_presented_options` 解析出对应实体。

---

## 7. 长期记忆写入设计

## 7.1 写入原则

长期记忆只记录未来可能复用、稳定、合规、有价值的信息。

适合写入：

- 用户明确表达的偏好
- 稳定身份和常用配置
- 长期业务关系
- 可复用任务偏好
- 用户明确纠正的信息
- 可复用任务完成摘要

不适合写入：

- 临时信息
- 一次性任务状态
- 低置信推断
- 未授权敏感信息
- 企业知识

## 7.2 写入流程

```text
Conversation / Task Completion
  ->
Memory Candidate Extraction
  ->
Memory Classification
  ->
Memory Policy Check
  ->
Dedup / Merge
  ->
Consent / Approval if needed
  ->
Memory Store
```

## 7.3 Memory Candidate Extraction

从对话或任务结果中抽取候选记忆。

示例：

用户说：

```text
以后帮我订机票都尽量不要早班机。
```

候选记忆：

```json
{
  "candidate_id": "mem_cand_001",
  "memory_type": "preference",
  "subject": "user",
  "scope": "travel_booking",
  "key": "flight_time_preference",
  "value": "avoid_early_morning_flights",
  "evidence": "以后帮我订机票都尽量不要早班机",
  "confidence": 0.95,
  "source": "explicit_user_statement",
  "sensitivity": "low",
  "ttl": null
}
```

## 7.4 Memory Classification

建议分类：

- `preference`
- `profile`
- `business_context`
- `relationship`
- `correction`
- `habit`
- `task_summary`
- `do_not_remember`

## 7.5 Memory Policy Check

Policy 判断：

- 是否允许记录
- 是否需要用户确认
- 是否敏感
- 是否可跨 Skill 使用
- 是否过期
- 是否与已有记忆冲突
- 是否属于企业知识而不是用户记忆

示例：

```json
{
  "action": "save",
  "requires_consent": false,
  "reason": "explicit low-risk preference",
  "scope": "travel_booking"
}
```

敏感信息：

```json
{
  "action": "ask_consent",
  "requires_consent": true,
  "reason": "contains personal address"
}
```

临时状态：

```json
{
  "action": "discard",
  "reason": "one-time task state"
}
```

## 7.6 Dedup / Merge

长期记忆必须去重和合并。

已有记忆：

```json
{
  "key": "flight_time_preference",
  "value": "prefer_afternoon"
}
```

新候选：

```json
{
  "key": "flight_time_preference",
  "value": "avoid_early_morning"
}
```

合并结果：

```json
{
  "key": "flight_time_preference",
  "value": {
    "prefer": ["afternoon"],
    "avoid": ["early_morning"]
  }
}
```

用户明确纠正时，新记忆应覆盖旧记忆，并保留版本历史。

---

## 8. 长期记忆存储模型

## 8.1 MySQL 主存

MySQL 是长期记忆真相源。

字段建议：

- `memory_id`
- `tenant_id`
- `user_id`
- `memory_type`
- `scope`
- `key`
- `value`
- `confidence`
- `sensitivity`
- `source_type`
- `evidence_ref`
- `status`
- `created_at`
- `updated_at`
- `expires_at`

示例：

```json
{
  "memory_id": "mem_001",
  "tenant_id": "t_001",
  "user_id": "u_123",
  "memory_type": "preference",
  "scope": "travel_booking",
  "key": "flight_time_preference",
  "value": {
    "avoid": ["early_morning"]
  },
  "confidence": 0.95,
  "sensitivity": "low",
  "source_type": "explicit_user_statement",
  "evidence_ref": "turn_789",
  "status": "active",
  "expires_at": null
}
```

## 8.2 向量索引

用于语义检索，不作为真相源。

字段建议：

- `memory_id`
- `tenant_id`
- `user_id`
- `scope`
- `memory_text`
- `embedding`
- `sensitivity`
- `status`

---

## 9. 长期记忆检索设计

## 9.1 检索流程

```text
User Event
  ->
Memory Retrieval Planner
  ->
Scope Filter
  ->
Hybrid Retrieval
  ->
Rerank
  ->
Policy Filter
  ->
Context Injection Candidate
```

## 9.2 Memory Retrieval Planner

先判断当前任务需要什么类型的记忆。

旅行场景示例：

```json
{
  "memory_query": "travel preferences for booking flights",
  "scopes": ["travel_booking", "global_profile"],
  "memory_types": ["preference", "profile"],
  "max_results": 5
}
```

客服场景示例：

```json
{
  "memory_query": "customer support communication preferences",
  "scopes": ["customer_support", "global_profile"],
  "memory_types": ["preference", "business_context"],
  "max_results": 3
}
```

## 9.3 Scope 过滤

长期记忆必须有 scope。

订票任务只查：

- `travel_booking`
- `global_profile`

不查：

- `hr`
- `medical`
- `unrelated_task_summary`

Scope 过滤是保证长期记忆检索质量最重要的手段。

## 9.4 结构化过滤优先

检索前先过滤：

- tenant_id
- user_id
- status
- scope
- memory_type
- sensitivity
- permission

不要纯向量查全库。

## 9.5 Hybrid Retrieval

记忆检索建议混合：

- 结构化 key 查询
- 关键词
- 向量语义

例如用户说“还是按我平时坐飞机的习惯”，向量可召回“不要早班机”；结构化过滤确保只查 travel scope。

## 9.6 Rerank

召回后根据以下维度 rerank：

- semantic_score
- scope_match
- confidence
- recency
- explicitness
- sensitivity
- usage_success_history

示例打分：

```text
score = semantic_score * 0.4
      + scope_match * 0.25
      + confidence * 0.15
      + recency * 0.1
      + explicitness * 0.1
```

## 9.7 Context Injection

最终注入 LLM 的长期记忆要少而准。

建议：

- 普通任务最多 3 到 5 条
- 复杂任务最多 8 条
- 高敏感记忆默认不注入

注入格式：

```json
{
  "relevant_memories": [
    {
      "type": "preference",
      "scope": "travel_booking",
      "content": "User prefers to avoid early morning flights.",
      "confidence": 0.95
    }
  ]
}
```

---

## 10. 记忆质量治理

## 10.1 写入门槛

- 明确表达才自动写
- 推断类记忆默认不写或提高阈值
- 敏感类记忆需要确认
- 低置信候选丢弃

## 10.2 版本与覆盖

每条记忆支持版本和覆盖。

用户明确纠正时，应覆盖旧记忆。

## 10.3 记忆衰减

长期未使用或多次无效的记忆降低权重。

## 10.4 使用反馈

如果某条记忆被使用后用户否定：

```text
不是，我现在不这样了。
```

则应：

- 标记 memory conflict
- 降权或更新
- 触发 memory repair

---

## 11. Context 与 Memory 的协作链路

## 11.1 LLM 调用前

```text
AI Orchestrator
  ->
Memory Service retrieve
  ->
Knowledge Service retrieve
  ->
Context Builder
  ->
LLM Context
```

## 11.2 LLM 调用后

```text
Conversation / Task Result
  ->
Memory Candidate Extractor
  ->
Memory Policy Engine
  ->
Memory Store
```

## 11.3 关键原则

- Memory Service 不直接拼 prompt
- Context Builder 决定是否注入和注入多少
- LLM 不直接写长期记忆
- Knowledge 和 Memory 分开管理

---

## 12. 隐私与合规

长期记忆必须具备：

- 用户可查看
- 用户可删除
- 可禁用记忆
- 记忆来源可追溯
- 敏感字段加密
- scope 控制
- 置信度
- 过期时间
- 写入策略
- 审计日志

多国家、多语言场景下，需要根据区域合规要求控制记忆存储和跨境使用。

---

## 13. MVP 范围

## 13.1 Phase 1

先做：

- Task State
- Recent Turns
- Entity Cache
- Session Summary
- Context Builder
- 固定 token budget
- budget report

## 13.2 Phase 2

再做：

- Memory Candidate Extractor
- 用户偏好类长期记忆
- Memory Retrieval
- Memory Policy

## 13.3 Phase 3

再做：

- 用户可视化记忆管理
- 记忆删除/导出
- 记忆审计
- 向量化长期记忆
- 跨渠道长期个性化

---

## 14. 总结

上下文工程的核心不是“把更多信息塞进模型”，而是对信息进行选择、压缩、排序和治理。

推荐整体设计：

```text
Context Builder
  ->
Short-term Session / Task Memory
  ->
Long-term Memory Service
  ->
Knowledge Service
```

关键原则：

- Context 是运行时组装结果
- Short-term Memory 服务当前会话和任务
- Long-term Memory 服务跨会话个性化
- Knowledge 服务企业知识问答
- Task State 是状态机，不是记忆
- 不让 LLM 自由读写记忆
- 记忆必须通过结构化候选、策略判断、权限控制和审计流程来管理

