import type {
  AgentProfile,
  AgentDependencies,
  Connector,
  ConnectorDependencies,
  ConnectorOperation,
  DashboardMetric,
  ModelProvider,
  SkillDependencies,
  SkillSpec,
  ToolDependencies,
  ToolExecution,
  ToolSpec
} from "../types/platform";

export const metrics: DashboardMetric[] = [
  { label: "Agents Online", value: "12", delta: "+3 this week", tone: "blue" },
  { label: "Tool Success", value: "98.2%", delta: "+1.4%", tone: "green" },
  { label: "P95 Latency", value: "1.8s", delta: "-320ms", tone: "green" },
  { label: "Needs Review", value: "7", delta: "2 high risk", tone: "amber" }
];

export const agents: AgentProfile[] = [
  {
    id: "agent_order",
    tenantId: "tenant_1",
    name: "Order Agent",
    description: "Handles order lookup, refund guidance, and customer support handoff.",
    businessDomain: "Order",
    ownerTeam: "Customer Service Platform",
    status: "enabled",
    skillIds: ["skill_order"],
    defaultLang: "zh-CN",
    supportedLanguages: ["zh-CN", "en-US"],
    channels: ["text", "voice"],
    systemPrompt: "You are a concise customer service agent. Resolve order questions using configured skills.",
    welcomeMessage: "您好，我可以帮您查询订单和处理售后问题。",
    modelConfig: {
      providerId: "provider_mock",
      model: "mock-chat",
      temperature: 0.2
    },
    runtimePolicy: {
      maxTurns: 12,
      maxToolCalls: 6,
      responseTimeoutMs: 30000
    },
    version: "v1"
  },
  {
    id: "agent_help",
    tenantId: "tenant_1",
    name: "Help Agent",
    description: "Answers product policy questions using curated knowledge tools.",
    businessDomain: "Support",
    ownerTeam: "Help Center",
    status: "draft",
    skillIds: ["skill_help"],
    defaultLang: "zh-CN",
    supportedLanguages: ["zh-CN"],
    channels: ["text"],
    systemPrompt: "Answer support questions only with configured knowledge-backed skills.",
    welcomeMessage: "您好，我可以帮您查询帮助中心政策。",
    modelConfig: {
      providerId: "provider_mock",
      model: "mock-chat",
      temperature: 0.1
    },
    runtimePolicy: {
      maxTurns: 8,
      maxToolCalls: 3,
      responseTimeoutMs: 20000
    },
    version: "v1"
  },
  {
    id: "agent_weather",
    tenantId: "tenant_1",
    name: "Weather Agent",
    description: "Answers current weather questions through a mock weather business API.",
    businessDomain: "Weather",
    ownerTeam: "AI Platform",
    status: "enabled",
    skillIds: ["skill_weather"],
    defaultLang: "zh-CN",
    supportedLanguages: ["zh-CN", "en-US"],
    channels: ["text", "voice"],
    systemPrompt: "You are a concise weather assistant. When users ask about current weather, call query_weather with the city name.",
    welcomeMessage: "您好，我可以帮您查询城市实时天气。",
    modelConfig: {
      providerId: "provider_mock",
      model: "mock-chat",
      temperature: 0.1
    },
    runtimePolicy: {
      maxTurns: 6,
      maxToolCalls: 2,
      responseTimeoutMs: 15000
    },
    version: "v1"
  }
];

export const agentDependencies: Record<string, AgentDependencies> = {
  agent_order: {
    agentId: "agent_order",
    summary: {
      directSkillCount: 1,
      directToolCount: 0,
      reachableToolCount: 1,
      disabledSkillCount: 0,
      totalCapabilityCount: 2
    },
    directSkills: [{ id: "skill_order", name: "Order Service", status: "enabled" }],
    reachableTools: [
      {
        id: "tool_query_order",
        name: "query_order",
        viaSkillId: "skill_order",
        implementation: "connector",
        riskLevel: "low",
        status: "enabled"
      }
    ]
  },
  agent_help: {
    agentId: "agent_help",
    summary: {
      directSkillCount: 1,
      directToolCount: 0,
      reachableToolCount: 1,
      disabledSkillCount: 1,
      totalCapabilityCount: 2
    },
    directSkills: [{ id: "skill_help", name: "Help Center", status: "draft" }],
    reachableTools: [
      {
        id: "tool_search_help",
        name: "search_help_center",
        viaSkillId: "skill_help",
        implementation: "knowledge",
        riskLevel: "low",
        status: "enabled"
      }
    ]
  },
  agent_weather: {
    agentId: "agent_weather",
    summary: {
      directSkillCount: 1,
      directToolCount: 0,
      reachableToolCount: 1,
      disabledSkillCount: 0,
      totalCapabilityCount: 2
    },
    directSkills: [{ id: "skill_weather", name: "Weather Service", status: "enabled" }],
    reachableTools: [
      {
        id: "tool_query_weather",
        name: "query_weather",
        viaSkillId: "skill_weather",
        implementation: "connector",
        riskLevel: "low",
        status: "enabled"
      }
    ]
  }
};

export const skills: SkillSpec[] = [
  {
    id: "skill_order",
    tenantId: "tenant_1",
    name: "Order Service",
    description: "Order lookup and customer order service workflow.",
    businessDomain: "Order",
    ownerTeam: "Customer Service Platform",
    status: "enabled",
    toolIds: ["tool_query_order"],
    knowledgeIds: [],
    systemPrompt: "When users ask about an order, call query_order first.",
    useCases: ["Order status lookup", "Delivery progress explanation"],
    exclusions: ["Do not refund orders directly without explicit confirmation."],
    outputFormat: "Summarize order status in a concise customer-facing answer.",
    riskLevel: "low",
    executionPolicy: {
      maxToolCalls: 3,
      timeoutMillis: 30000,
      allowWriteTools: false,
      requireConfirmation: false
    },
    policyVersion: "v1",
    version: "v1"
  },
  {
    id: "skill_help",
    tenantId: "tenant_1",
    name: "Help Center",
    description: "Knowledge-grounded help center answers.",
    businessDomain: "Support",
    ownerTeam: "Help Center",
    status: "draft",
    toolIds: ["tool_search_help"],
    knowledgeIds: ["kb_help"],
    systemPrompt: "Use search_help_center for policy and support questions.",
    useCases: ["Support policy explanation", "Product FAQ"],
    exclusions: ["Do not invent policy details when retrieval returns no evidence."],
    outputFormat: "Answer with cited policy snippets when available.",
    riskLevel: "low",
    executionPolicy: {
      maxToolCalls: 2,
      timeoutMillis: 20000,
      allowWriteTools: false,
      requireConfirmation: false
    },
    policyVersion: "v1",
    version: "v1"
  },
  {
    id: "skill_weather",
    tenantId: "tenant_1",
    name: "Weather Service",
    description: "Current weather lookup through a governed connector tool.",
    businessDomain: "Weather",
    ownerTeam: "AI Platform",
    status: "enabled",
    toolIds: ["tool_query_weather"],
    knowledgeIds: [],
    systemPrompt: "When users ask about the weather, call query_weather with city. If city is missing, ask the user to provide one.",
    useCases: ["Current weather lookup", "Weather answer for text and voice demos"],
    exclusions: ["Do not provide severe weather alerts or long-term forecasts."],
    outputFormat: "Answer with city, condition, temperature, humidity, and wind in a concise sentence.",
    riskLevel: "low",
    executionPolicy: {
      maxToolCalls: 2,
      timeoutMillis: 15000,
      allowWriteTools: false,
      requireConfirmation: false
    },
    policyVersion: "v1",
    version: "v1"
  }
];

export const skillDependencies: Record<string, SkillDependencies> = {
  skill_order: {
    skillId: "skill_order",
    summary: {
      directAgentCount: 1,
      totalConsumerCount: 1
    },
    directAgents: [{ id: "agent_order", name: "Order Agent" }]
  },
  skill_help: {
    skillId: "skill_help",
    summary: {
      directAgentCount: 1,
      totalConsumerCount: 1
    },
    directAgents: [{ id: "agent_help", name: "Help Agent" }]
  },
  skill_weather: {
    skillId: "skill_weather",
    summary: {
      directAgentCount: 1,
      totalConsumerCount: 1
    },
    directAgents: [{ id: "agent_weather", name: "Weather Agent" }]
  }
};

export const tools: ToolSpec[] = [
  {
    id: "tool_query_order",
    tenantId: "tenant_1",
    name: "query_order",
    description: "Read-only order lookup via Connector Service.",
    businessDomain: "Order",
    ownerTeam: "Customer Service Platform",
    llmDescription: "Use this tool when the user asks about order status, payment, delivery, or order details.",
    implementation: "connector",
    binding: {
      connectorOperationId: "connop_query_order"
    },
    inputSchema: {
      type: "object",
      required: ["order_id"],
      properties: {
        order_id: { type: "string", description: "Order identifier." }
      }
    },
    outputSchema: {
      type: "object",
      properties: {
        order_id: { type: "string" },
        status: { type: "string" }
      }
    },
    sideEffect: "read",
    riskLevel: "low",
    requiresConfirmation: false,
    timeoutMillis: 10000,
    retryPolicy: { maxAttempts: 0, backoffMillis: 0 },
    status: "enabled",
    version: "v1"
  },
  {
    id: "tool_refund_order",
    tenantId: "tenant_1",
    name: "refund_order",
    description: "Refunds an order after explicit customer confirmation.",
    businessDomain: "Refund",
    ownerTeam: "Customer Service Platform",
    llmDescription: "Use this tool only after the user clearly confirms that they want to refund an order.",
    implementation: "workflow",
    binding: {
      workflowId: "workflow_refund_order"
    },
    inputSchema: {
      type: "object",
      required: ["order_id", "reason"],
      properties: {
        order_id: { type: "string", description: "Order identifier." },
        reason: { type: "string", description: "Refund reason." }
      }
    },
    outputSchema: {
      type: "object",
      properties: {
        refund_id: { type: "string" },
        status: { type: "string" }
      }
    },
    sideEffect: "write",
    riskLevel: "high",
    requiresConfirmation: true,
    timeoutMillis: 15000,
    retryPolicy: { maxAttempts: 0, backoffMillis: 0 },
    status: "draft",
    version: "v1"
  },
  {
    id: "tool_search_help",
    tenantId: "tenant_1",
    name: "search_help_center",
    description: "Retrieves policy chunks from the help knowledge base.",
    businessDomain: "Support",
    ownerTeam: "Help Center",
    llmDescription: "Use this tool to retrieve curated support policy snippets before answering policy questions.",
    implementation: "knowledge",
    binding: {
      knowledgeBaseIds: ["kb_help"]
    },
    inputSchema: {
      type: "object",
      required: ["query"],
      properties: {
        query: { type: "string", description: "User support question." }
      }
    },
    outputSchema: {
      type: "object",
      properties: {
        chunks: { type: "array" }
      }
    },
    sideEffect: "none",
    riskLevel: "low",
    requiresConfirmation: false,
    timeoutMillis: 8000,
    retryPolicy: { maxAttempts: 0, backoffMillis: 0 },
    status: "enabled",
    version: "v1"
  },
  {
    id: "tool_query_weather",
    tenantId: "tenant_1",
    name: "query_weather",
    description: "Read-only current weather lookup through Mock Business API.",
    businessDomain: "Weather",
    ownerTeam: "AI Platform",
    llmDescription: "Use this tool when the user asks for current weather in a city.",
    implementation: "connector",
    binding: {
      connectorOperationId: "connop_query_weather"
    },
    inputSchema: {
      type: "object",
      required: ["city"],
      properties: {
        city: { type: "string", description: "City name, for example 深圳 or Shanghai." },
        unit: { type: "string", description: "Temperature unit. Use metric by default." }
      }
    },
    outputSchema: {
      type: "object",
      properties: {
        city: { type: "string" },
        condition: { type: "string" },
        temperature_c: { type: "number" },
        humidity: { type: "integer" },
        wind_kph: { type: "number" }
      }
    },
    sideEffect: "read",
    riskLevel: "low",
    requiresConfirmation: false,
    timeoutMillis: 8000,
    retryPolicy: { maxAttempts: 0, backoffMillis: 0 },
    status: "enabled",
    version: "v1"
  }
];

export const toolDependencies: Record<string, ToolDependencies> = {
  tool_query_order: {
    toolId: "tool_query_order",
    summary: {
      directSkillCount: 1,
      directAgentCount: 0,
      indirectAgentCount: 1,
      totalConsumerCount: 2
    },
    directSkills: [{ id: "skill_order", name: "Order Service" }],
    directAgents: [],
    indirectAgents: [{ id: "agent_order", name: "Order Agent", viaSkillId: "skill_order" }]
  },
  tool_search_help: {
    toolId: "tool_search_help",
    summary: {
      directSkillCount: 1,
      directAgentCount: 0,
      indirectAgentCount: 1,
      totalConsumerCount: 2
    },
    directSkills: [{ id: "skill_help", name: "Help Center" }],
    directAgents: [],
    indirectAgents: [{ id: "agent_help", name: "Help Agent", viaSkillId: "skill_help" }]
  },
  tool_query_weather: {
    toolId: "tool_query_weather",
    summary: {
      directSkillCount: 1,
      directAgentCount: 0,
      indirectAgentCount: 1,
      totalConsumerCount: 2
    },
    directSkills: [{ id: "skill_weather", name: "Weather Service" }],
    directAgents: [],
    indirectAgents: [{ id: "agent_weather", name: "Weather Agent", viaSkillId: "skill_weather" }]
  }
};

export const connectors: Connector[] = [
  {
    id: "conn_order_platform",
    tenantId: "tenant_1",
    name: "Order Platform",
    description: "Connection profile for the internal order business APIs.",
    businessDomain: "Order",
    ownerTeam: "Order Platform",
    type: "http",
    status: "enabled",
    baseUrl: "http://localhost:8090",
    headers: { "X-Client": "flow-anything" },
    auth: { type: "bearer", secretRef: "secret_order_api_token" },
    timeoutMillis: 10000,
    version: "v1"
  },
  {
    id: "conn_mock_business_api",
    tenantId: "tenant_1",
    name: "Mock Business API",
    description: "Local mock API server used to validate connector, tool, and agent runtime paths.",
    businessDomain: "Weather",
    ownerTeam: "AI Platform",
    type: "http",
    status: "enabled",
    baseUrl: "http://localhost:8090",
    headers: { "X-Client": "flow-anything" },
    auth: { type: "none" },
    timeoutMillis: 8000,
    version: "v1"
  }
];

export const connectorOperations: ConnectorOperation[] = [
  {
    id: "connop_query_order",
    tenantId: "tenant_1",
    connectorId: "conn_order_platform",
    name: "query_order",
    description: "GET /orders/{order_id} from the order platform.",
    businessDomain: "Order",
    ownerTeam: "Order Platform",
    type: "http",
    implementationMode: "simple_http",
    method: "GET",
    baseUrl: "http://localhost:8090",
    path: "/orders/{order_id}",
    headers: { "X-Client": "flow-anything" },
    auth: { type: "bearer", secretRef: "secret_order_api_token" },
    inputSchema: {
      type: "object",
      required: ["order_id"],
      properties: {
        order_id: { type: "string" }
      }
    },
    outputSchema: {
      type: "object",
      properties: {
        order_id: { type: "string" },
        status: { type: "string" }
      }
    },
    timeoutMillis: 10000,
    status: "enabled"
  },
  {
    id: "connop_query_weather",
    tenantId: "tenant_1",
    connectorId: "conn_mock_business_api",
    name: "query_weather",
    description: "GET /weather/current from the local Mock Business API.",
    businessDomain: "Weather",
    ownerTeam: "AI Platform",
    type: "http",
    implementationMode: "simple_http",
    method: "GET",
    baseUrl: "http://localhost:8090",
    path: "/weather/current",
    headers: { "X-Client": "flow-anything" },
    auth: { type: "none" },
    inputSchema: {
      type: "object",
      required: ["city"],
      properties: {
        city: { type: "string" },
        unit: { type: "string" }
      }
    },
    outputSchema: {
      type: "object",
      properties: {
        city: { type: "string" },
        condition: { type: "string" },
        temperature_c: { type: "number" },
        humidity: { type: "integer" },
        wind_kph: { type: "number" }
      }
    },
    timeoutMillis: 8000,
    status: "enabled"
  }
];

export const connectorDependencies: Record<string, ConnectorDependencies> = {
  connop_query_order: {
    operationId: "connop_query_order",
    summary: {
      directToolCount: 1,
      indirectSkillCount: 1,
      indirectAgentCount: 1,
      totalConsumerCount: 3,
      blockingToolCount: 0
    },
    directTools: [
      {
        id: "tool_query_order",
        name: "query_order",
        description: "Read-only order lookup via Connector Service.",
        requiresReview: false
      }
    ],
    indirectSkills: [
      {
        id: "skill_order",
        name: "Order Service",
        viaToolId: "tool_query_order"
      }
    ],
    indirectAgents: [
      {
        id: "agent_order",
        name: "Order Agent",
        viaSkillId: "skill_order"
      }
    ]
  },
  connop_query_weather: {
    operationId: "connop_query_weather",
    summary: {
      directToolCount: 1,
      indirectSkillCount: 1,
      indirectAgentCount: 1,
      totalConsumerCount: 3,
      blockingToolCount: 0
    },
    directTools: [
      {
        id: "tool_query_weather",
        name: "query_weather",
        description: "Read-only current weather lookup through Mock Business API.",
        requiresReview: false
      }
    ],
    indirectSkills: [
      {
        id: "skill_weather",
        name: "Weather Service",
        viaToolId: "tool_query_weather"
      }
    ],
    indirectAgents: [
      {
        id: "agent_weather",
        name: "Weather Agent",
        viaSkillId: "skill_weather"
      }
    ]
  }
};

export const executions: ToolExecution[] = [
  {
    callId: "toolcall_9a81f2",
    tenantId: "tenant_1",
    toolId: "tool_query_order",
    toolName: "query_order",
    implementation: "connector",
    riskLevel: "low",
    requiresConfirmation: false,
    confirmed: false,
    traceId: "trace_42f1",
    status: "succeeded",
    durationMillis: 342,
    startedAt: "2026-04-30T09:32:12Z"
  },
  {
    callId: "toolcall_1e52bc",
    tenantId: "tenant_1",
    toolId: "tool_refund_order",
    toolName: "refund_order",
    implementation: "workflow",
    riskLevel: "high",
    requiresConfirmation: true,
    confirmed: false,
    traceId: "trace_44a7",
    status: "failed",
    errorCode: "confirmation_required",
    durationMillis: 12,
    startedAt: "2026-04-30T09:28:04Z"
  },
  {
    callId: "toolcall_weather_001",
    tenantId: "tenant_1",
    toolId: "tool_query_weather",
    toolName: "query_weather",
    implementation: "connector",
    riskLevel: "low",
    requiresConfirmation: false,
    confirmed: false,
    traceId: "trace_weather_001",
    status: "succeeded",
    durationMillis: 128,
    startedAt: "2026-05-03T04:20:00Z"
  }
];

export const modelProviders: ModelProvider[] = [
  {
    id: "provider_mock",
    name: "Local Mock Provider",
    type: "mock",
    baseUrl: "local",
    defaultModel: "mock-chat",
    status: "published",
    timeoutMillis: 30000
  },
  {
    id: "provider_openai_compatible",
    name: "OpenAI Compatible",
    type: "openai-compatible",
    baseUrl: "https://api.openai.com/v1",
    defaultModel: "configured-by-env",
    status: "draft",
    timeoutMillis: 30000
  },
  {
    id: "provider_deepseek",
    name: "DeepSeek",
    type: "deepseek",
    baseUrl: "https://api.deepseek.com",
    defaultModel: "deepseek-v4-flash",
    status: "published",
    timeoutMillis: 30000
  }
];
