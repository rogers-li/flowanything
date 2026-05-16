# AI 中台产品设计文档

## 1. 文档目的

本文档从产品经理视角重新设计 AI 中台，重点定义：

- AI 中台的产品定位
- 目标用户与核心使用场景
- 平台能力地图
- 核心资源模型
- 运营管理流程
- 后台信息架构
- MVP 范围与迭代路线
- 产品指标与成功标准

本文档不以技术服务拆分为主，而以“平台如何被使用、配置、运营、治理和交付业务价值”为主。

---

## 2. 产品定位

## 2.1 一句话定位

AI 中台是一个面向企业的智能体能力生产与运营平台，用于快速构建、管理、发布和持续优化可接入多渠道的 AI Agent。

## 2.2 产品要解决的问题

企业在建设 AI Agent 时通常会遇到以下问题：

- 每接入一个业务系统都需要重复开发
- Tool、Skill、Prompt、知识库分散管理，缺少统一治理
- Agent 能力上线缺少测试、审批、灰度和回滚
- 知识库导入后缺少质量运营和使用分析
- 运行时出了问题难以追溯配置版本和执行链路
- 不同渠道、租户、业务线无法复用同一套 AI 能力

AI 中台的目标是把这些能力平台化，让业务团队可以像配置产品能力一样创建和运营 AI Agent。

## 2.3 产品边界

AI 中台负责：

- 管理 Agent、Skill、Tool、Connector、Knowledge、Prompt、Policy
- 支持能力测试、发布、灰度、审计和回滚
- 支持多租户、多业务线、多渠道配置
- 提供运行监控、效果评估和问题回放

AI 中台不负责：

- 替代业务系统成为主数据源
- 替代语音网关、ASR、TTS 等实时语音链路
- 替代企业已有 IAM、审批、工单等基础平台
- 自动保证所有 Agent 场景可用，仍需要运营和评测闭环

---

## 3. 目标用户

## 3.1 平台管理员

关注：

- 租户、环境、权限和发布治理
- 平台稳定性和资源健康状态
- 审计和合规

核心动作：

- 创建租户
- 管理角色权限
- 审批高风险能力发布
- 查看平台运行概览

## 3.2 AI 产品经理 / Agent 设计师

关注：

- Agent 具备哪些能力
- 用户体验是否符合业务预期
- Skill、Tool、Knowledge 如何组合

核心动作：

- 创建 Agent Profile
- 编排 Skill
- 选择 Tool Set
- 配置 Prompt / Policy
- 测试 Agent 效果

## 3.3 平台开发者

关注：

- 如何接入外部 API
- 如何创建 Tool
- 如何接入 Python Code Tool / MCP / Workflow 等扩展能力

核心动作：

- 注册 Connector
- 创建 Connector Operation
- 创建 Tool
- 配置 execution binding
- 运行沙箱测试

## 3.4 知识运营人员

关注：

- 文档是否完整、准确、及时
- 检索结果是否命中正确内容
- 哪些问题没有被知识库覆盖

核心动作：

- 创建知识库
- 上传或同步文档
- 管理权限标签
- 查看索引状态
- 测试检索效果
- 维护评测集

## 3.5 业务负责人

关注：

- Agent 是否带来业务价值
- 用户满意度和任务完成率
- 风险是否可控

核心动作：

- 查看业务指标
- 查看高风险操作审计
- 审批关键能力上线
- 查看用户反馈和失败原因

---

## 4. 产品总览

## 4.1 产品能力地图

AI 中台分为八大产品域：

- `Agent 管理`
- `Skill 管理`
- `Tool 管理`
- `Connector 管理`
- `Knowledge 管理`
- `Prompt / Policy 管理`
- `测试与发布`
- `观测与评估`

## 4.2 核心资源层级

建议采用以下产品对象层级：

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
    ->
Channel Deployment
```

含义：

- `External API`：外部系统真实接口
- `Connector Operation`：平台封装后的标准操作
- `Tool`：Agent 可调用能力
- `Tool Set`：一组工具集合
- `Skill`：场景能力包
- `Agent Profile`：某个机器人实际启用的能力组合
- `Channel Deployment`：语音、文本、电话、IM 等渠道部署配置

---

## 5. 核心产品模块设计

## 5.1 Agent 管理

### 目标

让用户可以创建和管理不同业务场景下的 AI Agent。

### 核心能力

- 创建 Agent Profile
- 绑定 Skill
- 绑定渠道
- 配置默认语言、地区、模型策略
- 配置欢迎语、兜底话术、转人工策略
- 查看 Agent 运行指标

### 关键字段

- `agent_id`
- `agent_name`
- `description`
- `tenant_id`
- `enabled_skills`
- `channel_configs`
- `default_locale`
- `model_route_policy`
- `fallback_policy`
- `status`
- `version`

### 典型页面

- Agent 列表
- Agent 详情
- Skill 绑定
- 渠道配置
- 测试沙箱
- 运行指标

---

## 5.2 Skill 管理

### 目标

Skill 是面向业务场景的能力包，用于把 Tool、知识库、Prompt、Policy 组合成一个可复用能力。

### Skill 包含

- Instructions
- Tool Set
- Knowledge Scope
- Prompt Template
- Policy
- Model Route
- Workflow

### 核心能力

- 创建 Skill
- 绑定 Tool Set
- 绑定知识库范围
- 配置场景说明和执行约束
- 配置确认策略和转人工策略
- 发布 Skill 版本

### 典型 Skill

- 售后客服 Skill
- IT Helpdesk Skill
- 订单查询 Skill
- 旅行预订 Skill
- 企业制度问答 Skill

### 关键字段

- `skill_id`
- `skill_name`
- `description`
- `instructions`
- `tool_sets`
- `knowledge_scopes`
- `policies`
- `model_route`
- `status`
- `version`

---

## 5.3 Tool 管理

### 目标

让平台用户可以把不同底层能力包装成 Agent/LLM 可使用的 Tool。

### 支持的 Tool 类型

- Connector Operation Tool
- Python Code Tool
- MCP Tool
- Knowledge Retrieval Tool
- Workflow Tool
- Internal Action Tool

### 核心能力

- 创建 Tool
- 定义 Tool 描述
- 定义输入输出 Schema
- 配置 execution binding
- 配置风险等级
- 配置确认策略
- 配置权限范围
- 沙箱测试
- 发布与回滚

### Tool 页面重点

- LLM 可见描述预览
- 参数 Schema 编辑器
- 执行绑定配置
- 测试样例
- 执行 trace
- 引用关系

---

## 5.4 Connector 管理

### 目标

让平台开发者快速把外部业务系统 API 注册为平台可调用的 Connector Operation。

### 核心能力

- 创建 Connector
- 配置认证方式
- 创建 Operation
- 配置请求映射
- 配置响应映射
- 配置错误映射
- 配置超时、重试、限流
- 测试 Operation

### 关键流程

```text
注册 Connector
    ->
新增 Operation
    ->
配置 Schema / Mapper / Auth
    ->
沙箱测试
    ->
发布 Operation
    ->
创建 Tool 绑定 Operation
```

### 重要原则

Connector Operation 发布后并不会自动暴露给 LLM。  
必须在 Tool 管理中创建 Tool，并显式绑定该 Operation。

---

## 5.5 Knowledge 管理

### 目标

让知识运营人员可以管理知识库、文档、索引和检索质量。

### 核心能力

- 创建知识库
- 上传文档
- 同步外部文档源
- 配置权限标签
- 配置切分策略
- 配置 embedding 模型
- 查看索引状态
- 测试检索效果
- 管理评测集
- 查看无结果问题和低质量召回

### 文档状态

- `uploaded`
- `indexing`
- `indexed`
- `failed`
- `expired`
- `archived`

### 质量指标

- 索引成功率
- 无结果率
- 低置信召回率
- 用户负反馈率
- 文档过期率
- 热门未覆盖问题

---

## 5.6 Prompt / Policy 管理

### 目标

让 Agent 的行为策略可配置、可测试、可发布、可回滚。

### Prompt 管理

支持：

- 模板版本
- 多语言版本
- 变量校验
- 场景绑定
- 灰度发布

### Policy 管理

支持：

- 工具可见性策略
- 高风险确认策略
- 拒答策略
- 转人工策略
- 模型路由策略
- 成本和延迟阈值

---

## 5.7 测试沙箱

### 目标

在能力上线前提供可视化测试环境。

### 支持测试对象

- Connector Operation
- Tool
- Workflow
- Skill
- Agent Profile
- Knowledge Retrieval

### 测试结果展示

- 输入参数
- 输出结果
- Trace
- 使用的 Prompt / Policy / Tool 版本
- 模型调用
- 工具调用
- 错误信息
- 延迟分布

---

## 5.8 发布与审批

### 目标

防止未验证能力直接影响线上用户。

### 生命周期

```text
draft
    ->
testing
    ->
pending_approval
    ->
published
    ->
deprecated
    ->
archived
```

### 必须审批的资源

- 高风险 Tool
- 生产环境 Connector Operation
- 涉及写操作的 Workflow
- 关键 Prompt / Policy
- Agent Profile 生产发布

### 发布前检查

- Schema 是否兼容
- 是否有测试用例
- 是否通过沙箱测试
- 是否影响已发布 Skill / Agent
- 是否涉及高风险操作
- 是否具备回滚版本

---

## 6. 核心用户流程

## 6.1 注册一个新的外部 API 并提供给 Agent 使用

```text
1. 平台开发者创建 Connector
2. 配置认证和目标环境
3. 创建 Connector Operation
4. 配置 input_schema / output_schema
5. 配置 request / response / error mapping
6. 在沙箱中测试 Operation
7. 发布 Connector Operation
8. 创建 Tool
9. 将 Tool 绑定到 Connector Operation
10. 配置 Tool 描述、风险等级和确认策略
11. 测试 Tool
12. 发布 Tool
13. 加入 Tool Set
14. 绑定到 Skill
15. 绑定到 Agent Profile
```

## 6.2 创建一个新的 Skill

```text
1. AI 产品经理创建 Skill
2. 填写场景说明和 instructions
3. 选择 Tool Set
4. 选择知识库范围
5. 配置 Prompt 和 Policy
6. 配置模型路由
7. 在沙箱中测试 Skill
8. 提交审批
9. 发布 Skill
10. 绑定到 Agent Profile
```

## 6.3 运营一个知识库

```text
1. 知识运营人员创建 Knowledge Base
2. 上传或同步文档
3. 配置权限标签
4. 配置切分策略和 embedding 模型
5. 触发索引
6. 查看索引状态
7. 用真实问题测试召回
8. 保存测试样例到评测集
9. 发布知识库
10. 持续观察无结果问题和用户反馈
```

## 6.4 发布一个 Agent 到语音渠道

```text
1. 创建 Agent Profile
2. 绑定 Skill
3. 配置语音渠道参数
4. 配置语言和地区
5. 配置首响应、兜底和转人工策略
6. 运行端到端测试
7. 提交发布审批
8. 灰度发布
9. 观察指标
10. 全量发布或回滚
```

---

## 7. 信息架构

## 7.1 顶层导航

建议后台包含：

- Dashboard
- Agent
- Skill
- Tool
- Connector
- Knowledge
- Prompt / Policy
- Sandbox
- Release
- Observability
- Audit
- Settings

## 7.2 Dashboard

展示：

- Agent 数量
- 已发布 Skill 数量
- 已发布 Tool 数量
- Connector 健康状态
- 知识库索引状态
- 最近发布
- 任务完成率
- Tool 调用成功率
- 知识检索无结果率

## 7.3 Observability

展示：

- 会话 trace
- Tool 调用 trace
- Connector 调用 trace
- Knowledge retrieval trace
- 模型调用统计
- 失败归因

---

## 8. 权限模型

## 8.1 角色

- `platform_admin`
- `agent_designer`
- `connector_developer`
- `tool_designer`
- `knowledge_operator`
- `approver`
- `viewer`

## 8.2 权限维度

- 租户
- 环境
- 资源类型
- 资源实例
- 操作类型

## 8.3 高风险控制

以下操作必须记录审计：

- 发布生产 Tool
- 修改 Connector 认证配置
- 发布写操作 Workflow
- 修改高风险 Policy
- 删除知识库
- 导出运行日志

---

## 9. 产品指标

## 9.1 平台建设指标

- 新 API 注册耗时
- 新 Tool 创建耗时
- 新 Skill 创建耗时
- 发布审批平均耗时
- 配置回滚成功率

## 9.2 运行效果指标

- Agent 任务完成率
- Tool 调用成功率
- Connector 调用成功率
- Knowledge 检索命中率
- 用户重复表达率
- 转人工率

## 9.3 运营质量指标

- 知识库无结果率
- 热门未覆盖问题数
- 高风险 Tool 审批覆盖率
- 线上问题可回放率
- 配置版本可追溯率

---

## 10. MVP 范围

## 10.1 MVP 必须包含

- Connector 管理
- Connector Operation 管理
- Tool 管理
- Tool 到 Connector Operation / Knowledge 的绑定
- Knowledge Base 管理
- 文档上传和索引状态
- Skill 管理
- Agent Profile 管理
- 测试沙箱
- 简单发布流程
- 基础审计日志

## 10.2 MVP 可以暂缓

- 可视化 Workflow 编排器
- MCP Server 全生命周期管理
- Python Tool 在线代码编辑器
- 复杂灰度策略
- 多维成本分析
- 自动质量优化建议

---

## 11. 版本路线

## 11.1 V0.1：平台骨架

目标：

- 能注册 API
- 能创建 Tool
- 能管理知识库
- 能创建 Agent Profile
- 能在沙箱中跑通完整链路

## 11.2 V0.2：运营闭环

目标：

- 支持发布审批
- 支持版本回滚
- 支持 Skill 管理
- 支持基础运行指标和 trace

## 11.3 V0.3：能力扩展

目标：

- 支持 Workflow Tool
- 支持 MCP Tool
- 支持 Python Code Tool
- 支持知识库评测集

## 11.4 V1.0：企业级可用

目标：

- 多租户
- 多环境
- 灰度发布
- 完整审计
- 运行效果评估
- 成本和质量运营

---

## 12. 主要风险

## 12.1 资源模型过早复杂化

风险：

- 页面很多，但核心链路没有跑通

应对：

- MVP 只保留 Connector、Tool、Knowledge、Skill、Agent 五个核心对象

## 12.2 Tool 暴露过度

风险：

- LLM 可见工具过多，选择不稳定

应对：

- 通过 Tool Set 和 Skill 控制工具可见性

## 12.3 知识库只建不运营

风险：

- 文档导入后无人维护，召回质量持续下降

应对：

- 必须提供无结果问题、低质量召回、文档过期提醒

## 12.4 发布流程太重

风险：

- 平台配置效率低，业务团队绕过平台

应对：

- 低风险资源轻审批，高风险资源强审批

---

## 13. 总结

AI 中台的产品核心不是“把 LLM 接进来”，而是建立一套可运营的智能体能力生产体系。

从产品视角看，平台应围绕以下主线设计：

```text
接入能力
    ->
包装 Tool
    ->
组合 Skill
    ->
配置 Agent
    ->
发布渠道
    ->
观测优化
```

其中：

- Connector 解决 API 接入
- Tool 解决能力封装
- Skill 解决场景复用
- Knowledge 解决知识运营
- Agent Profile 解决机器人配置
- Sandbox / Release / Audit 解决平台治理

这套产品设计可以让 AI 中台从“技术运行时”升级为“企业级 AI Agent 生产与运营平台”。
