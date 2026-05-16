import type {
  AgentDependencies,
  AgentDebugEvent,
  AgentDebugResponse,
  AgentFlowEdge,
  AgentFlowGraph,
  AgentFlowNode,
  AgentFlowNodeRun,
  AgentFlowRun,
  AgentFlowRunResponse,
  AgentFlowSpec,
  AgentTrace,
  AgentProfile,
  Connector,
  ConnectorDependencies,
  ConnectorInvokeResult,
  ConnectorOperation,
  KnowledgeBase,
  KnowledgeDocument,
  KnowledgeSearchResult,
  SkillDependencies,
  SkillSpec,
  ToolDependencies,
  ToolExecution,
  ToolExecutionResult,
  ToolSpec,
  RuntimeEvent,
  WorkflowEdge,
  WorkflowGraph,
  WorkflowNode,
  WorkflowNodeRun,
  WorkflowRun,
  WorkflowRunResponse,
  WorkflowSpec
} from "../types/platform";

type ApiListResponse<T> = {
  items: T[];
};

type AgentDTO = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  name: string;
  description?: string;
  business_domain?: string;
  businessDomain?: string;
  owner_team?: string;
  ownerTeam?: string;
  status?: AgentProfile["status"];
  skill_ids?: string[];
  skillIds?: string[];
  tool_ids?: string[];
  toolIds?: string[];
  default_lang?: string;
  defaultLang?: string;
  supported_languages?: string[];
  supportedLanguages?: string[];
  channels?: string[];
  system_prompt?: string;
  systemPrompt?: string;
  welcome_message?: string;
  welcomeMessage?: string;
  model_config?: {
    provider_id?: string;
    providerId?: string;
    model?: string;
    temperature?: number;
  };
  modelConfig?: AgentProfile["modelConfig"];
  runtime_policy?: {
    max_turns?: number;
    maxTurns?: number;
    max_tool_calls?: number;
    maxToolCalls?: number;
    response_timeout_ms?: number;
    responseTimeoutMs?: number;
  };
  runtimePolicy?: AgentProfile["runtimePolicy"];
  version?: string;
};

type AgentDependenciesDTO = {
  agent_id?: string;
  agentId?: string;
  summary: {
    direct_skill_count?: number;
    directSkillCount?: number;
    direct_tool_count?: number;
    directToolCount?: number;
    reachable_tool_count?: number;
    reachableToolCount?: number;
    disabled_skill_count?: number;
    disabledSkillCount?: number;
    total_capability_count?: number;
    totalCapabilityCount?: number;
  };
  direct_skills?: Array<{ id: string; name: string; status?: string }>;
  directSkills?: AgentDependencies["directSkills"];
  reachable_tools?: Array<{
    id: string;
    name: string;
    via_skill_id?: string;
    viaSkillId?: string;
    source?: string;
    implementation?: AgentDependencies["reachableTools"][number]["implementation"];
    risk_level?: AgentDependencies["reachableTools"][number]["riskLevel"];
    riskLevel?: AgentDependencies["reachableTools"][number]["riskLevel"];
    status?: string;
  }>;
  reachableTools?: AgentDependencies["reachableTools"];
};

type AgentDebugEventDTO = {
  id?: string;
  tenant_id: string;
  trace_id?: string;
  user_id: string;
  session_id: string;
  agent_id: string;
  type: AgentDebugEvent["type"];
  channel: AgentDebugEvent["channel"];
  payload: AgentDebugEvent["payload"];
  occurred_at?: string;
};

type AgentDebugResponseDTO = {
  event_id?: string;
  eventId?: string;
  trace_id?: string;
  traceId?: string;
  actions?: Array<{
    type: AgentDebugResponse["actions"][number]["type"];
    text?: string;
    tool_call?: {
      id?: string;
      tool_id?: string;
      toolId?: string;
      name?: string;
      args?: Record<string, unknown>;
      confirmed?: boolean;
      trace_id?: string;
      traceId?: string;
    };
    toolCall?: AgentDebugResponse["actions"][number]["toolCall"];
    tool_result?: ToolExecutionResultDTO;
    toolResult?: ToolExecutionResultDTO;
  }>;
};

type AgentDebugToolCallDTO = {
  id?: string;
  tool_id?: string;
  toolId?: string;
  name?: string;
  args?: Record<string, unknown>;
  confirmed?: boolean;
  trace_id?: string;
  traceId?: string;
};

type AgentTraceDTO = {
  trace_id?: string;
  traceId?: string;
  tenant_id?: string;
  tenantId?: string;
  agent_id?: string;
  agentId?: string;
  session_id?: string;
  sessionId?: string;
  event_id?: string;
  eventId?: string;
  status?: AgentTrace["status"];
  started_at?: string;
  startedAt?: string;
  finished_at?: string;
  finishedAt?: string;
  duration_ms?: number;
  durationMillis?: number;
  error?: string;
  steps?: TraceStepDTO[];
};

type RuntimeEventDTO = {
  id?: string;
  type: string;
  tenant_id?: string;
  tenantId?: string;
  run_id?: string;
  runId?: string;
  trace_id?: string;
  traceId?: string;
  event_id?: string;
  eventId?: string;
  agent_id?: string;
  agentId?: string;
  session_id?: string;
  sessionId?: string;
  parent_id?: string;
  parentId?: string;
  step_id?: string;
  stepId?: string;
  step_type?: string;
  stepType?: string;
  name?: string;
  status?: string;
  message?: string;
  payload?: Record<string, unknown>;
  created_at?: string;
  createdAt?: string;
};

type AgentFlowNodeDTO = {
  id: string;
  type: AgentFlowNode["type"];
  name: string;
  description?: string;
  config?: Record<string, unknown>;
  timeout_ms?: number;
  timeoutMillis?: number;
  retry_policy?: {
    max_attempts?: number;
    maxAttempts?: number;
    backoff_ms?: number;
    backoffMillis?: number;
  };
  retryPolicy?: AgentFlowNode["retryPolicy"];
};

type AgentFlowEdgeDTO = {
  id?: string;
  from_node_id?: string;
  fromNodeId?: string;
  to_node_id?: string;
  toNodeId?: string;
  type?: AgentFlowEdge["type"];
  condition?: AgentFlowEdge["condition"];
  description?: string;
};

type AgentFlowGraphDTO = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  name: string;
  description?: string;
  status?: AgentFlowSpec["status"];
  version?: string;
  entry_node_id?: string;
  entryNodeId?: string;
  nodes?: Record<string, AgentFlowNodeDTO>;
  edges?: AgentFlowEdgeDTO[];
  policy?: {
    max_steps?: number;
    maxSteps?: number;
    max_parallelism?: number;
    maxParallelism?: number;
    timeout_ms?: number;
    timeoutMillis?: number;
  };
};

type AgentFlowDTO = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  name: string;
  description?: string;
  business_domain?: string;
  businessDomain?: string;
  owner_team?: string;
  ownerTeam?: string;
  status?: AgentFlowSpec["status"];
  orchestration_mode?: AgentFlowSpec["orchestrationMode"];
  orchestrationMode?: AgentFlowSpec["orchestrationMode"];
  supervisor?: {
    supervisor_agent_id?: string;
    supervisorAgentId?: string;
    sub_agent_ids?: string[];
    subAgentIds?: string[];
    max_depth?: number;
    maxDepth?: number;
    max_sub_agent_calls?: number;
    maxSubAgentCalls?: number;
    planning_prompt?: string;
    planningPrompt?: string;
    final_prompt?: string;
    finalPrompt?: string;
  };
  graph?: AgentFlowGraphDTO;
  context_schema?: Record<string, unknown>;
  contextSchema?: Record<string, unknown>;
  input_schema?: Record<string, unknown>;
  inputSchema?: Record<string, unknown>;
  output_schema?: Record<string, unknown>;
  outputSchema?: Record<string, unknown>;
  version?: string;
};

type AgentFlowRunDTO = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  flow_id?: string;
  flowId?: string;
  flow_version?: string;
  flowVersion?: string;
  status: AgentFlowRun["status"];
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: string;
  started_at?: string;
  startedAt?: string;
  finished_at?: string;
  finishedAt?: string;
};

type AgentFlowNodeRunDTO = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  run_id?: string;
  runId?: string;
  flow_id?: string;
  flowId?: string;
  node_id?: string;
  nodeId?: string;
  node_type?: AgentFlowNodeRun["nodeType"];
  nodeType?: AgentFlowNodeRun["nodeType"];
  node_name?: string;
  nodeName?: string;
  status: AgentFlowNodeRun["status"];
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: string;
  started_at?: string;
  startedAt?: string;
  finished_at?: string;
  finishedAt?: string;
};

type AgentFlowRunResponseDTO = {
  run: AgentFlowRunDTO;
  node_runs?: AgentFlowNodeRunDTO[];
  nodeRuns?: AgentFlowNodeRunDTO[];
  error?: string;
};

type WorkflowNodeDTO = {
  id: string;
  type: WorkflowNode["type"];
  name: string;
  description?: string;
  position?: {
    x?: number;
    y?: number;
  };
  config?: Record<string, unknown>;
  timeout_ms?: number;
  timeoutMillis?: number;
  retry_policy?: {
    max_attempts?: number;
    maxAttempts?: number;
    backoff_ms?: number;
    backoffMillis?: number;
  };
  retryPolicy?: WorkflowNode["retryPolicy"];
};

type WorkflowEdgeDTO = {
  id?: string;
  from_node_id?: string;
  fromNodeId?: string;
  to_node_id?: string;
  toNodeId?: string;
  type?: WorkflowEdge["type"];
  condition?: WorkflowEdge["condition"];
  description?: string;
};

type WorkflowGraphDTO = {
  entry_node_id?: string;
  entryNodeId?: string;
  nodes?: Record<string, WorkflowNodeDTO>;
  edges?: WorkflowEdgeDTO[];
};

type WorkflowDTO = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  name: string;
  description?: string;
  business_domain?: string;
  businessDomain?: string;
  owner_team?: string;
  ownerTeam?: string;
  status?: WorkflowSpec["status"];
  profile?: WorkflowSpec["profile"];
  context_schema?: Record<string, unknown>;
  contextSchema?: Record<string, unknown>;
  input_schema?: Record<string, unknown>;
  inputSchema?: Record<string, unknown>;
  output_schema?: Record<string, unknown>;
  outputSchema?: Record<string, unknown>;
  graph?: WorkflowGraphDTO;
  policy?: {
    max_steps?: number;
    maxSteps?: number;
    max_parallelism?: number;
    maxParallelism?: number;
    timeout_ms?: number;
    timeoutMillis?: number;
  };
  ui?: Record<string, unknown>;
  version?: string;
};

type WorkflowRunDTO = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  workflow_id?: string;
  workflowId?: string;
  version?: string;
  status: WorkflowRun["status"];
  input?: Record<string, unknown>;
  context?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: string;
  trace_id?: string;
  traceId?: string;
  started_at?: string;
  startedAt?: string;
  finished_at?: string;
  finishedAt?: string;
};

type WorkflowRunListDTO = {
  items: WorkflowRunDTO[];
};

type WorkflowNodeRunDTO = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  run_id?: string;
  runId?: string;
  workflow_id?: string;
  workflowId?: string;
  node_id?: string;
  nodeId?: string;
  node_type?: WorkflowNodeRun["nodeType"];
  nodeType?: WorkflowNodeRun["nodeType"];
  node_name?: string;
  nodeName?: string;
  status: WorkflowNodeRun["status"];
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  context?: Record<string, unknown>;
  error?: string;
  started_at?: string;
  startedAt?: string;
  finished_at?: string;
  finishedAt?: string;
};

type WorkflowRunResponseDTO = {
  run: WorkflowRunDTO;
  node_runs?: WorkflowNodeRunDTO[];
  nodeRuns?: WorkflowNodeRunDTO[];
  error?: string;
};

type TraceStepDTO = {
  id: string;
  parent_id?: string;
  parentId?: string;
  type: AgentTrace["steps"][number]["type"];
  name: string;
  status: AgentTrace["steps"][number]["status"];
  started_at?: string;
  startedAt?: string;
  finished_at?: string;
  finishedAt?: string;
  duration_ms?: number;
  durationMillis?: number;
  metadata?: Record<string, unknown>;
  error?: string;
};

type AgentModelConfigDTO = {
  provider_id?: string;
  providerId?: string;
  model?: string;
  temperature?: number;
};

type AgentRuntimePolicyDTO = {
  max_turns?: number;
  maxTurns?: number;
  max_tool_calls?: number;
  maxToolCalls?: number;
  response_timeout_ms?: number;
  responseTimeoutMs?: number;
};

type SkillDTO = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  name: string;
  description?: string;
  business_domain?: string;
  businessDomain?: string;
  owner_team?: string;
  ownerTeam?: string;
  status?: SkillSpec["status"];
  tool_ids?: string[];
  toolIds?: string[];
  knowledge_ids?: string[];
  knowledgeIds?: string[];
  system_prompt?: string;
  systemPrompt?: string;
  use_cases?: string[];
  useCases?: string[];
  exclusions?: string[];
  output_format?: string;
  outputFormat?: string;
  risk_level?: SkillSpec["riskLevel"];
  riskLevel?: SkillSpec["riskLevel"];
  execution_policy?: {
    max_tool_calls?: number;
    maxToolCalls?: number;
    timeout_ms?: number;
    timeoutMillis?: number;
    allow_write_tools?: boolean;
    allowWriteTools?: boolean;
    require_confirmation?: boolean;
    requireConfirmation?: boolean;
  };
  executionPolicy?: SkillSpec["executionPolicy"];
  policy_version?: string;
  policyVersion?: string;
  version?: string;
};

type SkillDependenciesDTO = {
  skill_id?: string;
  skillId?: string;
  summary: {
    direct_agent_count?: number;
    directAgentCount?: number;
    total_consumer_count?: number;
    totalConsumerCount?: number;
  };
  direct_agents?: Array<{ id: string; name: string }>;
  directAgents?: SkillDependencies["directAgents"];
};

type SkillExecutionPolicyDTO = {
  max_tool_calls?: number;
  maxToolCalls?: number;
  timeout_ms?: number;
  timeoutMillis?: number;
  allow_write_tools?: boolean;
  allowWriteTools?: boolean;
  require_confirmation?: boolean;
  requireConfirmation?: boolean;
};

function normalizeAgent(raw: AgentDTO): AgentProfile {
  const modelConfig = (raw.modelConfig ?? raw.model_config ?? {}) as AgentModelConfigDTO;
  const runtimePolicy = (raw.runtimePolicy ?? raw.runtime_policy ?? {}) as AgentRuntimePolicyDTO;
  return {
    id: raw.id,
    tenantId: raw.tenantId ?? raw.tenant_id ?? defaultTenantId,
    name: raw.name,
    description: raw.description ?? "",
    businessDomain: raw.businessDomain ?? raw.business_domain,
    ownerTeam: raw.ownerTeam ?? raw.owner_team,
    status: raw.status ?? "draft",
    skillIds: raw.skillIds ?? raw.skill_ids ?? [],
    toolIds: raw.toolIds ?? raw.tool_ids ?? [],
    defaultLang: raw.defaultLang ?? raw.default_lang ?? "zh-CN",
    supportedLanguages: raw.supportedLanguages ?? raw.supported_languages ?? [],
    channels: raw.channels ?? [],
    systemPrompt: raw.systemPrompt ?? raw.system_prompt,
    welcomeMessage: raw.welcomeMessage ?? raw.welcome_message,
    modelConfig: {
      providerId: modelConfig.providerId ?? modelConfig.provider_id,
      model: modelConfig.model ?? "default",
      temperature: modelConfig.temperature ?? 0
    },
    runtimePolicy: {
      maxTurns: runtimePolicy.maxTurns ?? runtimePolicy.max_turns ?? 12,
      maxToolCalls: runtimePolicy.maxToolCalls ?? runtimePolicy.max_tool_calls ?? 6,
      responseTimeoutMs: runtimePolicy.responseTimeoutMs ?? runtimePolicy.response_timeout_ms ?? 30000
    },
    version: raw.version ?? "v1"
  };
}

function serializeAgent(agent: AgentProfile): AgentDTO {
  return {
    id: agent.id,
    tenant_id: agent.tenantId,
    name: agent.name,
    description: agent.description,
    business_domain: agent.businessDomain,
    owner_team: agent.ownerTeam,
    status: agent.status,
    skill_ids: agent.skillIds,
    tool_ids: agent.toolIds ?? [],
    default_lang: agent.defaultLang,
    supported_languages: agent.supportedLanguages,
    channels: agent.channels,
    system_prompt: agent.systemPrompt,
    welcome_message: agent.welcomeMessage,
    model_config: {
      provider_id: agent.modelConfig?.providerId,
      model: agent.modelConfig?.model,
      temperature: agent.modelConfig?.temperature ?? 0
    },
    runtime_policy: {
      max_turns: agent.runtimePolicy?.maxTurns ?? 12,
      max_tool_calls: agent.runtimePolicy?.maxToolCalls ?? 6,
      response_timeout_ms: agent.runtimePolicy?.responseTimeoutMs ?? 30000
    },
    version: agent.version
  };
}

function normalizeAgentFlowNode(raw: AgentFlowNodeDTO): AgentFlowNode {
  const retry = raw.retryPolicy ?? {};
  const retrySnake = raw.retry_policy ?? {};
  return {
    id: raw.id,
    type: raw.type,
    name: raw.name,
    description: raw.description,
    config: raw.config ?? {},
    timeoutMillis: raw.timeoutMillis ?? raw.timeout_ms,
    retryPolicy: {
      maxAttempts: retry.maxAttempts ?? retrySnake.max_attempts,
      backoffMillis: retry.backoffMillis ?? retrySnake.backoff_ms
    }
  };
}

function serializeAgentFlowNode(node: AgentFlowNode): AgentFlowNodeDTO {
  return {
    id: node.id,
    type: node.type,
    name: node.name,
    description: node.description,
    config: node.config ?? {},
    timeout_ms: node.timeoutMillis,
    retry_policy: {
      max_attempts: node.retryPolicy?.maxAttempts,
      backoff_ms: node.retryPolicy?.backoffMillis
    }
  };
}

function normalizeAgentFlowEdge(raw: AgentFlowEdgeDTO): AgentFlowEdge {
  return {
    id: raw.id,
    fromNodeId: raw.fromNodeId ?? raw.from_node_id ?? "",
    toNodeId: raw.toNodeId ?? raw.to_node_id ?? "",
    type: raw.type ?? "default",
    condition: raw.condition,
    description: raw.description
  };
}

function serializeAgentFlowEdge(edge: AgentFlowEdge): AgentFlowEdgeDTO {
  return {
    id: edge.id,
    from_node_id: edge.fromNodeId,
    to_node_id: edge.toNodeId,
    type: edge.type ?? "default",
    condition: edge.condition,
    description: edge.description
  };
}

function normalizeAgentFlowGraph(raw: AgentFlowGraphDTO | undefined, fallback: AgentFlowDTO): AgentFlowGraph {
  const nodes = raw?.nodes ?? {};
  const policy = raw?.policy ?? {};
  return {
    id: raw?.id ?? fallback.id,
    tenantId: raw?.tenantId ?? raw?.tenant_id ?? fallback.tenantId ?? fallback.tenant_id ?? defaultTenantId,
    name: raw?.name ?? fallback.name,
    description: raw?.description ?? fallback.description,
    status: raw?.status ?? fallback.status ?? "draft",
    version: raw?.version ?? fallback.version ?? "v1",
    entryNodeId: raw?.entryNodeId ?? raw?.entry_node_id ?? "start",
    nodes: Object.fromEntries(Object.entries(nodes).map(([key, node]) => [key, normalizeAgentFlowNode(node)])),
    edges: (raw?.edges ?? []).map(normalizeAgentFlowEdge),
    policy: {
      maxSteps: policy.maxSteps ?? policy.max_steps,
      maxParallelism: policy.maxParallelism ?? policy.max_parallelism,
      timeoutMillis: policy.timeoutMillis ?? policy.timeout_ms
    }
  };
}

function serializeAgentFlowGraph(graph: AgentFlowGraph): AgentFlowGraphDTO {
  return {
    id: graph.id,
    tenant_id: graph.tenantId,
    name: graph.name,
    description: graph.description,
    status: graph.status,
    version: graph.version,
    entry_node_id: graph.entryNodeId,
    nodes: Object.fromEntries(Object.entries(graph.nodes).map(([key, node]) => [key, serializeAgentFlowNode(node)])),
    edges: graph.edges.map(serializeAgentFlowEdge),
    policy: {
      max_steps: graph.policy?.maxSteps,
      max_parallelism: graph.policy?.maxParallelism,
      timeout_ms: graph.policy?.timeoutMillis
    }
  };
}

function normalizeAgentFlow(raw: AgentFlowDTO): AgentFlowSpec {
  const supervisor = raw.supervisor ?? {};
  return {
    id: raw.id,
    tenantId: raw.tenantId ?? raw.tenant_id ?? defaultTenantId,
    name: raw.name,
    description: raw.description ?? "",
    businessDomain: raw.businessDomain ?? raw.business_domain,
    ownerTeam: raw.ownerTeam ?? raw.owner_team,
    status: raw.status ?? "draft",
    orchestrationMode: raw.orchestrationMode ?? raw.orchestration_mode ?? "workflow",
    supervisor: {
      supervisorAgentId: supervisor.supervisorAgentId ?? supervisor.supervisor_agent_id,
      subAgentIds: supervisor.subAgentIds ?? supervisor.sub_agent_ids ?? [],
      maxDepth: supervisor.maxDepth ?? supervisor.max_depth ?? 4,
      maxSubAgentCalls: supervisor.maxSubAgentCalls ?? supervisor.max_sub_agent_calls ?? 5,
      planningPrompt: supervisor.planningPrompt ?? supervisor.planning_prompt,
      finalPrompt: supervisor.finalPrompt ?? supervisor.final_prompt
    },
    graph: normalizeAgentFlowGraph(raw.graph, raw),
    contextSchema: raw.contextSchema ?? raw.context_schema ?? {},
    inputSchema: raw.inputSchema ?? raw.input_schema ?? {},
    outputSchema: raw.outputSchema ?? raw.output_schema ?? {},
    version: raw.version ?? "v1"
  };
}

function serializeAgentFlow(flow: AgentFlowSpec): AgentFlowDTO {
  return {
    id: flow.id,
    tenant_id: flow.tenantId,
    name: flow.name,
    description: flow.description,
    business_domain: flow.businessDomain,
    owner_team: flow.ownerTeam,
    status: flow.status,
    orchestration_mode: flow.orchestrationMode,
    supervisor: {
      supervisor_agent_id: flow.supervisor?.supervisorAgentId,
      sub_agent_ids: flow.supervisor?.subAgentIds ?? [],
      max_depth: flow.supervisor?.maxDepth ?? 4,
      max_sub_agent_calls: flow.supervisor?.maxSubAgentCalls ?? 5,
      planning_prompt: flow.supervisor?.planningPrompt,
      final_prompt: flow.supervisor?.finalPrompt
    },
    graph: serializeAgentFlowGraph(flow.graph),
    context_schema: flow.contextSchema,
    input_schema: flow.inputSchema,
    output_schema: flow.outputSchema,
    version: flow.version
  };
}

function normalizeAgentFlowRun(raw: AgentFlowRunDTO): AgentFlowRun {
  return {
    id: raw.id,
    tenantId: raw.tenantId ?? raw.tenant_id ?? defaultTenantId,
    flowId: raw.flowId ?? raw.flow_id ?? "",
    flowVersion: raw.flowVersion ?? raw.flow_version,
    status: raw.status,
    input: raw.input,
    output: raw.output,
    error: raw.error,
    startedAt: raw.startedAt ?? raw.started_at ?? "",
    finishedAt: raw.finishedAt ?? raw.finished_at
  };
}

function normalizeAgentFlowNodeRun(raw: AgentFlowNodeRunDTO): AgentFlowNodeRun {
  return {
    id: raw.id,
    tenantId: raw.tenantId ?? raw.tenant_id ?? defaultTenantId,
    runId: raw.runId ?? raw.run_id ?? "",
    flowId: raw.flowId ?? raw.flow_id ?? "",
    nodeId: raw.nodeId ?? raw.node_id ?? "",
    nodeType: raw.nodeType ?? raw.node_type ?? "start",
    nodeName: raw.nodeName ?? raw.node_name,
    status: raw.status,
    input: raw.input,
    output: raw.output,
    error: raw.error,
    startedAt: raw.startedAt ?? raw.started_at ?? "",
    finishedAt: raw.finishedAt ?? raw.finished_at
  };
}

function normalizeAgentFlowRunResponse(raw: AgentFlowRunResponseDTO): AgentFlowRunResponse {
  return {
    run: normalizeAgentFlowRun(raw.run),
    nodeRuns: (raw.nodeRuns ?? raw.node_runs ?? []).map(normalizeAgentFlowNodeRun),
    error: raw.error
  };
}

function normalizeWorkflowNode(raw: WorkflowNodeDTO): WorkflowNode {
  const retry = raw.retryPolicy ?? {};
  const retrySnake = raw.retry_policy ?? {};
  return {
    id: raw.id,
    type: raw.type,
    name: raw.name,
    description: raw.description,
    position: raw.position,
    config: raw.config ?? {},
    timeoutMillis: raw.timeoutMillis ?? raw.timeout_ms,
    retryPolicy: {
      maxAttempts: retry.maxAttempts ?? retrySnake.max_attempts,
      backoffMillis: retry.backoffMillis ?? retrySnake.backoff_ms
    }
  };
}

function serializeWorkflowNode(node: WorkflowNode): WorkflowNodeDTO {
  return {
    id: node.id,
    type: node.type,
    name: node.name,
    description: node.description,
    position: node.position,
    config: node.config ?? {},
    timeout_ms: node.timeoutMillis,
    retry_policy: {
      max_attempts: node.retryPolicy?.maxAttempts,
      backoff_ms: node.retryPolicy?.backoffMillis
    }
  };
}

function normalizeWorkflowEdge(raw: WorkflowEdgeDTO): WorkflowEdge {
  return {
    id: raw.id,
    fromNodeId: raw.fromNodeId ?? raw.from_node_id ?? "",
    toNodeId: raw.toNodeId ?? raw.to_node_id ?? "",
    type: raw.type ?? "default",
    condition: raw.condition,
    description: raw.description
  };
}

function serializeWorkflowEdge(edge: WorkflowEdge): WorkflowEdgeDTO {
  return {
    id: edge.id,
    from_node_id: edge.fromNodeId,
    to_node_id: edge.toNodeId,
    type: edge.type ?? "default",
    condition: edge.condition,
    description: edge.description
  };
}

function normalizeWorkflowGraph(raw: WorkflowGraphDTO | undefined): WorkflowGraph {
  const nodes = raw?.nodes ?? {
    start: {
      id: "start",
      type: "start" as const,
      name: "Start"
    }
  };
  return {
    entryNodeId: raw?.entryNodeId ?? raw?.entry_node_id ?? "start",
    nodes: Object.fromEntries(Object.entries(nodes).map(([key, node]) => [key, normalizeWorkflowNode(node)])),
    edges: (raw?.edges ?? []).map(normalizeWorkflowEdge)
  };
}

function serializeWorkflowGraph(graph: WorkflowGraph): WorkflowGraphDTO {
  return {
    entry_node_id: graph.entryNodeId,
    nodes: Object.fromEntries(Object.entries(graph.nodes).map(([key, node]) => [key, serializeWorkflowNode(node)])),
    edges: graph.edges.map(serializeWorkflowEdge)
  };
}

function normalizeWorkflow(raw: WorkflowDTO): WorkflowSpec {
  const policy = raw.policy ?? {};
  return {
    id: raw.id,
    tenantId: raw.tenantId ?? raw.tenant_id ?? defaultTenantId,
    name: raw.name,
    description: raw.description ?? "",
    businessDomain: raw.businessDomain ?? raw.business_domain,
    ownerTeam: raw.ownerTeam ?? raw.owner_team,
    status: raw.status ?? "draft",
    profile: raw.profile ?? "tool_workflow",
    contextSchema: raw.contextSchema ?? raw.context_schema ?? {},
    inputSchema: raw.inputSchema ?? raw.input_schema ?? {},
    outputSchema: raw.outputSchema ?? raw.output_schema ?? {},
    graph: normalizeWorkflowGraph(raw.graph),
    policy: {
      maxSteps: policy.maxSteps ?? policy.max_steps,
      maxParallelism: policy.maxParallelism ?? policy.max_parallelism,
      timeoutMillis: policy.timeoutMillis ?? policy.timeout_ms
    },
    ui: raw.ui ?? {},
    version: raw.version ?? "v1"
  };
}

function serializeWorkflow(workflow: WorkflowSpec): WorkflowDTO {
  return {
    id: workflow.id,
    tenant_id: workflow.tenantId,
    name: workflow.name,
    description: workflow.description,
    business_domain: workflow.businessDomain,
    owner_team: workflow.ownerTeam,
    status: workflow.status,
    profile: workflow.profile,
    context_schema: workflow.contextSchema,
    input_schema: workflow.inputSchema,
    output_schema: workflow.outputSchema,
    graph: serializeWorkflowGraph(workflow.graph),
    policy: {
      max_steps: workflow.policy?.maxSteps,
      max_parallelism: workflow.policy?.maxParallelism,
      timeout_ms: workflow.policy?.timeoutMillis
    },
    ui: workflow.ui,
    version: workflow.version
  };
}

function normalizeWorkflowRun(raw: WorkflowRunDTO): WorkflowRun {
  return {
    id: raw.id,
    tenantId: raw.tenantId ?? raw.tenant_id ?? defaultTenantId,
    workflowId: raw.workflowId ?? raw.workflow_id ?? "",
    version: raw.version,
    status: raw.status,
    input: raw.input,
    context: raw.context,
    output: raw.output,
    error: raw.error,
    traceId: raw.traceId ?? raw.trace_id,
    startedAt: raw.startedAt ?? raw.started_at ?? "",
    finishedAt: raw.finishedAt ?? raw.finished_at
  };
}

function normalizeWorkflowNodeRun(raw: WorkflowNodeRunDTO): WorkflowNodeRun {
  return {
    id: raw.id,
    tenantId: raw.tenantId ?? raw.tenant_id ?? defaultTenantId,
    runId: raw.runId ?? raw.run_id ?? "",
    workflowId: raw.workflowId ?? raw.workflow_id ?? "",
    nodeId: raw.nodeId ?? raw.node_id ?? "",
    nodeType: raw.nodeType ?? raw.node_type ?? "start",
    nodeName: raw.nodeName ?? raw.node_name,
    status: raw.status,
    input: raw.input,
    output: raw.output,
    context: raw.context,
    error: raw.error,
    startedAt: raw.startedAt ?? raw.started_at ?? "",
    finishedAt: raw.finishedAt ?? raw.finished_at
  };
}

function normalizeWorkflowRunResponse(raw: WorkflowRunResponseDTO): WorkflowRunResponse {
  return {
    run: normalizeWorkflowRun(raw.run),
    nodeRuns: (raw.nodeRuns ?? raw.node_runs ?? []).map(normalizeWorkflowNodeRun),
    error: raw.error
  };
}

function normalizeKnowledgeBase(raw: KnowledgeBaseDTO): KnowledgeBase {
  return {
    id: raw.id,
    tenantId: raw.tenantId ?? raw.tenant_id ?? defaultTenantId,
    name: raw.name,
    description: raw.description ?? "",
    status: raw.status ?? "draft",
    embeddingModel: raw.embeddingModel ?? raw.embedding_model ?? "",
    documentCount: raw.documentCount ?? raw.document_count ?? 0,
    chunkCount: raw.chunkCount ?? raw.chunk_count ?? 0,
    metadata: raw.metadata,
    version: raw.version ?? "v1",
    updatedAt: raw.updatedAt ?? raw.updated_at ?? ""
  };
}

function serializeKnowledgeBase(base: KnowledgeBase): KnowledgeBaseDTO {
  return {
    id: base.id,
    tenant_id: base.tenantId,
    name: base.name,
    description: base.description,
    status: base.status,
    embedding_model: base.embeddingModel,
    metadata: base.metadata,
    version: base.version
  };
}

function normalizeKnowledgeDocument(raw: KnowledgeDocumentDTO): KnowledgeDocument {
  return {
    id: raw.id,
    tenantId: raw.tenantId ?? raw.tenant_id ?? defaultTenantId,
    kbId: raw.kbId ?? raw.kb_id ?? "",
    title: raw.title,
    text: raw.text,
    metadata: raw.metadata,
    version: raw.version ?? "v1"
  };
}

function serializeKnowledgeDocument(document: KnowledgeDocument): KnowledgeDocumentDTO {
  return {
    id: document.id,
    tenant_id: document.tenantId,
    kb_id: document.kbId,
    title: document.title,
    text: document.text,
    metadata: document.metadata,
    version: document.version
  };
}

function normalizeKnowledgeSearchResult(raw: KnowledgeSearchResultDTO): KnowledgeSearchResult {
  return {
    queryId: raw.queryId ?? raw.query_id ?? "",
    chunks: (raw.chunks ?? []).map((chunk) => ({
      id: chunk.id,
      docId: chunk.docId ?? chunk.doc_id ?? "",
      kbId: chunk.kbId ?? chunk.kb_id ?? "",
      text: chunk.text,
      score: chunk.score ?? 0,
      metadata: chunk.metadata
    }))
  };
}

function serializeAgentDebugEvent(evt: AgentDebugEvent): AgentDebugEventDTO {
  return {
    id: evt.id,
    tenant_id: evt.tenantId,
    trace_id: evt.traceId,
    user_id: evt.userId,
    session_id: evt.sessionId,
    agent_id: evt.agentId,
    type: evt.type,
    channel: evt.channel,
    payload: evt.payload,
    occurred_at: evt.occurredAt
  };
}

function normalizeAgentDebugResponse(raw: AgentDebugResponseDTO): AgentDebugResponse {
  return {
    eventId: raw.eventId ?? raw.event_id ?? "",
    traceId: raw.traceId ?? raw.trace_id ?? "",
    actions: (raw.actions ?? []).map((action) => {
      const toolCall = (action.toolCall ?? action.tool_call) as AgentDebugToolCallDTO | undefined;
      const toolResult = action.toolResult ?? action.tool_result;
      return {
        type: action.type,
        text: action.text,
        toolCall: toolCall
          ? {
              id: toolCall.id,
              toolId: toolCall.toolId ?? toolCall.tool_id,
              name: toolCall.name,
              args: toolCall.args,
              confirmed: toolCall.confirmed,
              traceId: toolCall.traceId ?? toolCall.trace_id
            }
          : undefined,
        toolResult: toolResult ? normalizeToolExecutionResult(toolResult) : undefined
      };
    })
  };
}

function normalizeAgentTrace(raw: AgentTraceDTO): AgentTrace {
  return {
    traceId: raw.traceId ?? raw.trace_id ?? "",
    tenantId: raw.tenantId ?? raw.tenant_id ?? defaultTenantId,
    agentId: raw.agentId ?? raw.agent_id,
    sessionId: raw.sessionId ?? raw.session_id,
    eventId: raw.eventId ?? raw.event_id,
    status: raw.status ?? "running",
    startedAt: raw.startedAt ?? raw.started_at ?? "",
    finishedAt: raw.finishedAt ?? raw.finished_at,
    durationMillis: raw.durationMillis ?? raw.duration_ms,
    error: raw.error,
    steps: (raw.steps ?? []).map((step) => ({
      id: step.id,
      parentId: step.parentId ?? step.parent_id,
      type: step.type,
      name: step.name,
      status: step.status,
      startedAt: step.startedAt ?? step.started_at ?? "",
      finishedAt: step.finishedAt ?? step.finished_at,
      durationMillis: step.durationMillis ?? step.duration_ms,
      metadata: step.metadata,
      error: step.error
    }))
  };
}

function normalizeRuntimeEvent(raw: RuntimeEventDTO): RuntimeEvent {
  return {
    id: raw.id,
    type: raw.type,
    tenantId: raw.tenantId ?? raw.tenant_id ?? defaultTenantId,
    runId: raw.runId ?? raw.run_id,
    traceId: raw.traceId ?? raw.trace_id,
    eventId: raw.eventId ?? raw.event_id,
    agentId: raw.agentId ?? raw.agent_id,
    sessionId: raw.sessionId ?? raw.session_id,
    parentId: raw.parentId ?? raw.parent_id,
    stepId: raw.stepId ?? raw.step_id,
    stepType: raw.stepType ?? raw.step_type,
    name: raw.name,
    status: raw.status,
    message: raw.message,
    payload: raw.payload,
    createdAt: raw.createdAt ?? raw.created_at
  };
}

type ConnectorOperationDTO = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  connector_id?: string;
  connectorId?: string;
  name: string;
  description?: string;
  business_domain?: string;
  businessDomain?: string;
  owner_team?: string;
  ownerTeam?: string;
  type?: "http";
  status?: "draft" | "enabled" | "disabled";
  implementation_mode?: ConnectorOperation["implementationMode"];
  implementationMode?: ConnectorOperation["implementationMode"];
  method: ConnectorOperation["method"];
  base_url?: string;
  baseUrl?: string;
  path: string;
  headers?: Record<string, string>;
  auth?: ConnectorAuthDTO;
  input_schema?: Record<string, unknown>;
  inputSchema?: Record<string, unknown>;
  output_schema?: Record<string, unknown>;
  outputSchema?: Record<string, unknown>;
  timeout_ms?: number;
  timeoutMillis?: number;
  version?: string;
};

type ConnectorDTO = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  name: string;
  description?: string;
  business_domain?: string;
  businessDomain?: string;
  owner_team?: string;
  ownerTeam?: string;
  type?: "http";
  status?: "draft" | "enabled" | "disabled";
  base_url?: string;
  baseUrl?: string;
  headers?: Record<string, string>;
  auth?: ConnectorAuthDTO;
  timeout_ms?: number;
  timeoutMillis?: number;
  version?: string;
};

type ConnectorAuthDTO = {
  type?: Connector["auth"] extends infer T ? T extends { type?: infer U } ? U : never : never;
  header_name?: string;
  headerName?: string;
  secret_ref?: string;
  secretRef?: string;
  provider?: string;
  client_id_ref?: string;
  clientIDRef?: string;
  clientIdRef?: string;
  client_secret_ref?: string;
  clientSecretRef?: string;
  refresh_token_ref?: string;
  refreshTokenRef?: string;
  authorization_code_ref?: string;
  authorizationCodeRef?: string;
  app_access_token_url?: string;
  appAccessTokenURL?: string;
  appAccessTokenUrl?: string;
  access_token_url?: string;
  accessTokenURL?: string;
  accessTokenUrl?: string;
  refresh_token_url?: string;
  refreshTokenURL?: string;
  refreshTokenUrl?: string;
  tenant_access_token_url?: string;
  tenantTokenURL?: string;
  tenantTokenUrl?: string;
};

type ToolDTO = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  name: string;
  description?: string;
  business_domain?: string;
  businessDomain?: string;
  owner_team?: string;
  ownerTeam?: string;
  status?: ToolSpec["status"];
  llm_description?: string;
  llmDescription?: string;
  implementation: ToolSpec["implementation"];
  binding?: {
    connector_operation_id?: string;
    connectorOperationId?: string;
    knowledge_base_ids?: string[];
    knowledgeBaseIds?: string[];
    python_package_id?: string;
    pythonPackageId?: string;
    mcp_server_id?: string;
    mcpServerId?: string;
    mcp_server_url?: string;
    mcpServerUrl?: string;
    mcp_transport?: string;
    mcpTransport?: string;
    mcp_headers?: Record<string, string>;
    mcpHeaders?: Record<string, string>;
    mcp_tool_name?: string;
    mcpToolName?: string;
    workflow_id?: string;
    workflowId?: string;
  };
  input_schema?: Record<string, unknown>;
  inputSchema?: Record<string, unknown>;
  output_schema?: Record<string, unknown>;
  outputSchema?: Record<string, unknown>;
  side_effect?: ToolSpec["sideEffect"];
  sideEffect?: ToolSpec["sideEffect"];
  risk_level?: ToolSpec["riskLevel"];
  riskLevel?: ToolSpec["riskLevel"];
  requires_confirmation?: boolean;
  requiresConfirmation?: boolean;
  timeout_ms?: number;
  timeoutMillis?: number;
  retry_policy?: {
    max_attempts?: number;
    maxAttempts?: number;
    backoff_ms?: number;
    backoffMillis?: number;
  };
  retryPolicy?: ToolSpec["retryPolicy"];
  version?: string;
};

type MCPDiscoveryRequestDTO = {
  name: string;
  url: string;
  transport: string;
  headers?: Array<{ name: string; value: string }>;
  require_authorization?: boolean;
};

type MCPDiscoveryResponseDTO = {
  server_id?: string;
  serverId?: string;
  tools?: Array<{
    name: string;
    description?: string;
    input_schema?: Record<string, unknown>;
    inputSchema?: Record<string, unknown>;
    output_schema?: Record<string, unknown>;
    outputSchema?: Record<string, unknown>;
  }>;
};

type ToolRetryPolicyDTO = {
  max_attempts?: number;
  maxAttempts?: number;
  backoff_ms?: number;
  backoffMillis?: number;
};

type ToolDependenciesDTO = {
  tool_id?: string;
  toolId?: string;
  summary: {
    direct_skill_count?: number;
    directSkillCount?: number;
    direct_agent_count?: number;
    directAgentCount?: number;
    indirect_agent_count?: number;
    indirectAgentCount?: number;
    total_consumer_count?: number;
    totalConsumerCount?: number;
  };
  direct_skills?: Array<{ id: string; name: string }>;
  directSkills?: ToolDependencies["directSkills"];
  direct_agents?: Array<{ id: string; name: string; via_skill_id?: string; viaSkillId?: string; source?: string }>;
  directAgents?: ToolDependencies["directAgents"];
  indirect_agents?: Array<{ id: string; name: string; via_skill_id?: string; viaSkillId?: string; source?: string }>;
  indirectAgents?: ToolDependencies["indirectAgents"];
};

type ToolExecutionResultDTO = {
  call_id?: string;
  callId?: string;
  tool_id?: string;
  toolId?: string;
  success: boolean;
  data?: Record<string, unknown>;
  error_code?: string;
  errorCode?: string;
  error_reason?: string;
  errorReason?: string;
  started_at?: string;
  startedAt?: string;
  finished_at?: string;
  finishedAt?: string;
};

type ToolListOptions = {
  status?: ToolSpec["status"] | "all";
};

type ConnectorDependenciesDTO = {
  operation_id?: string;
  operationId?: string;
  summary: {
    direct_tool_count?: number;
    directToolCount?: number;
    indirect_skill_count?: number;
    indirectSkillCount?: number;
    indirect_agent_count?: number;
    indirectAgentCount?: number;
    total_consumer_count?: number;
    totalConsumerCount?: number;
    blocking_tool_count?: number;
    blockingToolCount?: number;
  };
  direct_tools?: Array<{
    id: string;
    name: string;
    description?: string;
    requires_review?: boolean;
    requiresReview?: boolean;
  }>;
  directTools?: ConnectorDependencies["directTools"];
  indirect_skills?: Array<{
    id: string;
    name: string;
    via_tool_id?: string;
    viaToolId?: string;
  }>;
  indirectSkills?: ConnectorDependencies["indirectSkills"];
  indirect_agents?: Array<{
    id: string;
    name: string;
    via_skill_id?: string;
    viaSkillId?: string;
  }>;
  indirectAgents?: ConnectorDependencies["indirectAgents"];
};

type ConnectorInvokeResultDTO = {
  request_id?: string;
  requestId?: string;
  success: boolean;
  data?: Record<string, unknown>;
  error_code?: string;
  errorCode?: string;
  finished_at?: string;
  finishedAt?: string;
};

type KnowledgeBaseDTO = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  name: string;
  description?: string;
  status?: KnowledgeBase["status"];
  embedding_model?: string;
  embeddingModel?: string;
  document_count?: number;
  documentCount?: number;
  chunk_count?: number;
  chunkCount?: number;
  metadata?: Record<string, unknown>;
  version?: string;
  updated_at?: string;
  updatedAt?: string;
};

type KnowledgeDocumentDTO = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  kb_id?: string;
  kbId?: string;
  title: string;
  text: string;
  metadata?: Record<string, unknown>;
  version?: string;
};

type KnowledgeChunkDTO = {
  id: string;
  doc_id?: string;
  docId?: string;
  kb_id?: string;
  kbId?: string;
  text: string;
  score?: number;
  metadata?: Record<string, unknown>;
};

type KnowledgeSearchResultDTO = {
  query_id?: string;
  queryId?: string;
  chunks?: KnowledgeChunkDTO[];
};

export const defaultTenantId = "tenant_1";

const endpoints = {
  platform: import.meta.env.VITE_PLATFORM_API_URL ?? "/platform-api",
  orchestrator: import.meta.env.VITE_AI_ORCHESTRATOR_URL ?? "/ai-orchestrator",
  runtime: import.meta.env.VITE_AGENT_RUNTIME_URL ?? "/agent-runtime",
  agentFlowRuntime: import.meta.env.VITE_AGENT_FLOW_RUNTIME_URL ?? "/agent-flow-runtime",
  connector: import.meta.env.VITE_CONNECTOR_SERVICE_URL ?? "/connector-service",
  knowledge: import.meta.env.VITE_KNOWLEDGE_SERVICE_URL ?? "/knowledge-service",
  modelGateway: import.meta.env.VITE_MODEL_GATEWAY_URL ?? "/model-gateway"
};

async function getJson<T>(url: string): Promise<T> {
  const response = await fetch(url, {
    headers: {
      Accept: "application/json"
    }
  });
  if (!response.ok) {
    throw new Error(await requestErrorMessage(response, url));
  }
  return response.json() as Promise<T>;
}

async function sendJson<T>(url: string, method: "POST" | "PUT", body: unknown): Promise<T> {
  const response = await fetch(url, {
    method,
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json"
    },
    body: JSON.stringify(body)
  });
  if (!response.ok) {
    throw new Error(await requestErrorMessage(response, url));
  }
  return response.json() as Promise<T>;
}

async function requestErrorMessage(response: Response, url: string): Promise<string> {
  const detail = await readErrorDetail(response);
  const suffix = detail ? `: ${detail}` : "";
  return `Request failed: ${response.status} ${response.statusText} (${url})${suffix}`;
}

async function readErrorDetail(response: Response): Promise<string> {
  const text = await response.text();
  if (!text) {
    return "";
  }

  try {
    const body = JSON.parse(text) as { error?: { code?: string; message?: string } };
    if (body.error?.message) {
      return body.error.code ? `${body.error.code} - ${body.error.message}` : body.error.message;
    }
  } catch {
    // Fall back to the raw response body below.
  }

  return text.slice(0, 300);
}

function normalizeConnectorOperation(raw: ConnectorOperationDTO): ConnectorOperation {
  return {
    id: raw.id,
    tenantId: raw.tenantId ?? raw.tenant_id ?? defaultTenantId,
    connectorId: raw.connectorId ?? raw.connector_id,
    name: raw.name,
    description: raw.description ?? "",
    businessDomain: raw.businessDomain ?? raw.business_domain,
    ownerTeam: raw.ownerTeam ?? raw.owner_team,
    type: raw.type ?? "http",
    status: raw.status ?? "draft",
    implementationMode: raw.implementationMode ?? raw.implementation_mode ?? "simple_http",
    method: raw.method,
    baseUrl: raw.baseUrl ?? raw.base_url ?? "",
    path: raw.path,
    headers: raw.headers ?? {},
    auth: normalizeConnectorAuth(raw.auth),
    inputSchema: raw.inputSchema ?? raw.input_schema ?? {},
    outputSchema: raw.outputSchema ?? raw.output_schema ?? {},
    timeoutMillis: raw.timeoutMillis ?? raw.timeout_ms ?? 10000,
    version: raw.version
  };
}

function normalizeConnector(raw: ConnectorDTO): Connector {
  return {
    id: raw.id,
    tenantId: raw.tenantId ?? raw.tenant_id ?? defaultTenantId,
    name: raw.name,
    description: raw.description ?? "",
    businessDomain: raw.businessDomain ?? raw.business_domain,
    ownerTeam: raw.ownerTeam ?? raw.owner_team,
    type: raw.type ?? "http",
    status: raw.status ?? "draft",
    baseUrl: raw.baseUrl ?? raw.base_url ?? "",
    headers: raw.headers ?? {},
    auth: normalizeConnectorAuth(raw.auth),
    timeoutMillis: raw.timeoutMillis ?? raw.timeout_ms ?? 10000,
    version: raw.version ?? "v1"
  };
}

function normalizeTool(raw: ToolDTO): ToolSpec {
  const binding = raw.binding ?? {};
  const retry = (raw.retryPolicy ?? raw.retry_policy ?? {}) as ToolRetryPolicyDTO;
  return {
    id: raw.id,
    tenantId: raw.tenantId ?? raw.tenant_id ?? defaultTenantId,
    name: raw.name,
    description: raw.description ?? "",
    businessDomain: raw.businessDomain ?? raw.business_domain,
    ownerTeam: raw.ownerTeam ?? raw.owner_team,
    llmDescription: raw.llmDescription ?? raw.llm_description,
    implementation: raw.implementation,
    binding: {
      connectorOperationId: binding.connectorOperationId ?? binding.connector_operation_id,
      knowledgeBaseIds: binding.knowledgeBaseIds ?? binding.knowledge_base_ids,
      pythonPackageId: binding.pythonPackageId ?? binding.python_package_id,
      mcpServerId: binding.mcpServerId ?? binding.mcp_server_id,
      mcpServerUrl: binding.mcpServerUrl ?? binding.mcp_server_url,
      mcpTransport: binding.mcpTransport ?? binding.mcp_transport,
      mcpHeaders: binding.mcpHeaders ?? binding.mcp_headers,
      mcpToolName: binding.mcpToolName ?? binding.mcp_tool_name,
      workflowId: binding.workflowId ?? binding.workflow_id
    },
    inputSchema: raw.inputSchema ?? raw.input_schema ?? {},
    outputSchema: raw.outputSchema ?? raw.output_schema ?? {},
    sideEffect: raw.sideEffect ?? raw.side_effect ?? "none",
    riskLevel: raw.riskLevel ?? raw.risk_level ?? "low",
    requiresConfirmation: raw.requiresConfirmation ?? raw.requires_confirmation ?? false,
    timeoutMillis: raw.timeoutMillis ?? raw.timeout_ms ?? 10000,
    retryPolicy: {
      maxAttempts: retry?.maxAttempts ?? retry?.max_attempts ?? 0,
      backoffMillis: retry?.backoffMillis ?? retry?.backoff_ms ?? 0
    },
    status: raw.status ?? "draft",
    version: raw.version ?? "v1"
  };
}

function normalizeSkill(raw: SkillDTO): SkillSpec {
  const policy = (raw.executionPolicy ?? raw.execution_policy ?? {}) as SkillExecutionPolicyDTO;
  return {
    id: raw.id,
    tenantId: raw.tenantId ?? raw.tenant_id ?? defaultTenantId,
    name: raw.name,
    description: raw.description ?? "",
    businessDomain: raw.businessDomain ?? raw.business_domain,
    ownerTeam: raw.ownerTeam ?? raw.owner_team,
    status: raw.status ?? "draft",
    toolIds: raw.toolIds ?? raw.tool_ids ?? [],
    knowledgeIds: raw.knowledgeIds ?? raw.knowledge_ids ?? [],
    systemPrompt: raw.systemPrompt ?? raw.system_prompt ?? "",
    useCases: raw.useCases ?? raw.use_cases ?? [],
    exclusions: raw.exclusions ?? [],
    outputFormat: raw.outputFormat ?? raw.output_format,
    riskLevel: raw.riskLevel ?? raw.risk_level ?? "low",
    executionPolicy: {
      maxToolCalls: policy.maxToolCalls ?? policy.max_tool_calls ?? 4,
      timeoutMillis: policy.timeoutMillis ?? policy.timeout_ms ?? 30000,
      allowWriteTools: policy.allowWriteTools ?? policy.allow_write_tools ?? false,
      requireConfirmation: policy.requireConfirmation ?? policy.require_confirmation ?? false
    },
    policyVersion: raw.policyVersion ?? raw.policy_version,
    version: raw.version ?? "v1"
  };
}

function serializeSkill(skill: SkillSpec): SkillDTO {
  return {
    id: skill.id,
    tenant_id: skill.tenantId,
    name: skill.name,
    description: skill.description,
    business_domain: skill.businessDomain,
    owner_team: skill.ownerTeam,
    status: skill.status,
    tool_ids: skill.toolIds,
    knowledge_ids: skill.knowledgeIds,
    system_prompt: skill.systemPrompt,
    use_cases: skill.useCases,
    exclusions: skill.exclusions,
    output_format: skill.outputFormat,
    risk_level: skill.riskLevel,
    execution_policy: {
      max_tool_calls: skill.executionPolicy?.maxToolCalls ?? 4,
      timeout_ms: skill.executionPolicy?.timeoutMillis ?? 30000,
      allow_write_tools: skill.executionPolicy?.allowWriteTools ?? false,
      require_confirmation: skill.executionPolicy?.requireConfirmation ?? false
    },
    policy_version: skill.policyVersion,
    version: skill.version
  };
}

function serializeTool(tool: ToolSpec): ToolDTO {
  return {
    id: tool.id,
    tenant_id: tool.tenantId,
    name: tool.name,
    description: tool.description,
    business_domain: tool.businessDomain,
    owner_team: tool.ownerTeam,
    status: tool.status,
    llm_description: tool.llmDescription,
    implementation: tool.implementation,
    binding: {
      connector_operation_id: tool.binding?.connectorOperationId,
      knowledge_base_ids: tool.binding?.knowledgeBaseIds,
      python_package_id: tool.binding?.pythonPackageId,
      mcp_server_id: tool.binding?.mcpServerId,
      mcp_server_url: tool.binding?.mcpServerUrl,
      mcp_transport: tool.binding?.mcpTransport,
      mcp_headers: tool.binding?.mcpHeaders,
      mcp_tool_name: tool.binding?.mcpToolName,
      workflow_id: tool.binding?.workflowId
    },
    input_schema: tool.inputSchema,
    output_schema: tool.outputSchema,
    side_effect: tool.sideEffect,
    risk_level: tool.riskLevel,
    requires_confirmation: tool.requiresConfirmation,
    timeout_ms: tool.timeoutMillis,
    retry_policy: {
      max_attempts: tool.retryPolicy?.maxAttempts ?? 0,
      backoff_ms: tool.retryPolicy?.backoffMillis ?? 0
    },
    version: tool.version
  };
}

function serializeConnectorOperation(operation: ConnectorOperation): ConnectorOperationDTO {
  return {
    id: operation.id,
    tenant_id: operation.tenantId,
    connector_id: operation.connectorId,
    name: operation.name,
    description: operation.description,
    business_domain: operation.businessDomain,
    owner_team: operation.ownerTeam,
    type: operation.type,
    status: operation.status,
    implementation_mode: operation.implementationMode,
    method: operation.method,
    base_url: operation.baseUrl,
    path: operation.path,
    headers: operation.headers,
    auth: serializeConnectorAuth(operation.auth),
    input_schema: operation.inputSchema,
    output_schema: operation.outputSchema,
    timeout_ms: operation.timeoutMillis
  };
}

function serializeConnector(connector: Connector): ConnectorDTO {
  return {
    id: connector.id,
    tenant_id: connector.tenantId,
    name: connector.name,
    description: connector.description,
    business_domain: connector.businessDomain,
    owner_team: connector.ownerTeam,
    type: connector.type,
    status: connector.status,
    base_url: connector.baseUrl,
    headers: connector.headers,
    auth: serializeConnectorAuth(connector.auth),
    timeout_ms: connector.timeoutMillis,
    version: connector.version
  };
}

function normalizeConnectorAuth(raw?: ConnectorAuthDTO): Connector["auth"] {
  if (!raw) {
    return { type: "none" };
  }
  return {
    type: raw.type ?? "none",
    headerName: raw.headerName ?? raw.header_name,
    secretRef: raw.secretRef ?? raw.secret_ref
  };
}

function serializeConnectorAuth(auth?: Connector["auth"]): ConnectorAuthDTO {
  if (!auth) {
    return { type: "none" };
  }
  return {
    type: auth.type,
    header_name: auth.headerName,
    secret_ref: auth.secretRef
  };
}

function normalizeToolDependencies(raw: ToolDependenciesDTO): ToolDependencies {
  const summary = raw.summary;
  return {
    toolId: raw.toolId ?? raw.tool_id ?? "",
    summary: {
      directSkillCount: summary.directSkillCount ?? summary.direct_skill_count ?? 0,
      directAgentCount: summary.directAgentCount ?? summary.direct_agent_count ?? 0,
      indirectAgentCount: summary.indirectAgentCount ?? summary.indirect_agent_count ?? 0,
      totalConsumerCount: summary.totalConsumerCount ?? summary.total_consumer_count ?? 0
    },
    directSkills: raw.directSkills ?? raw.direct_skills ?? [],
    directAgents: raw.directAgents ?? (raw.direct_agents ?? []).map((item) => ({
      id: item.id,
      name: item.name,
      viaSkillId: item.viaSkillId ?? item.via_skill_id,
      source: item.source
    })),
    indirectAgents: raw.indirectAgents ?? (raw.indirect_agents ?? []).map((item) => ({
      id: item.id,
      name: item.name,
      viaSkillId: item.viaSkillId ?? item.via_skill_id,
      source: item.source
    }))
  };
}

function normalizeSkillDependencies(raw: SkillDependenciesDTO): SkillDependencies {
  const summary = raw.summary;
  return {
    skillId: raw.skillId ?? raw.skill_id ?? "",
    summary: {
      directAgentCount: summary.directAgentCount ?? summary.direct_agent_count ?? 0,
      totalConsumerCount: summary.totalConsumerCount ?? summary.total_consumer_count ?? 0
    },
    directAgents: raw.directAgents ?? raw.direct_agents ?? []
  };
}

function normalizeAgentDependencies(raw: AgentDependenciesDTO): AgentDependencies {
  const summary = raw.summary;
  return {
    agentId: raw.agentId ?? raw.agent_id ?? "",
    summary: {
      directSkillCount: summary.directSkillCount ?? summary.direct_skill_count ?? 0,
      directToolCount: summary.directToolCount ?? summary.direct_tool_count ?? 0,
      reachableToolCount: summary.reachableToolCount ?? summary.reachable_tool_count ?? 0,
      disabledSkillCount: summary.disabledSkillCount ?? summary.disabled_skill_count ?? 0,
      totalCapabilityCount: summary.totalCapabilityCount ?? summary.total_capability_count ?? 0
    },
    directSkills: raw.directSkills ?? raw.direct_skills ?? [],
    reachableTools: raw.reachableTools ?? (raw.reachable_tools ?? []).map((item) => ({
      id: item.id,
      name: item.name,
      viaSkillId: item.viaSkillId ?? item.via_skill_id,
      source: item.source,
      implementation: item.implementation,
      riskLevel: item.riskLevel ?? item.risk_level,
      status: item.status
    }))
  };
}

function normalizeToolExecutionResult(raw: ToolExecutionResultDTO): ToolExecutionResult {
  return {
    callId: raw.callId ?? raw.call_id ?? "",
    toolId: raw.toolId ?? raw.tool_id ?? "",
    success: raw.success,
    data: raw.data,
    errorCode: raw.errorCode ?? raw.error_code,
    errorReason: raw.errorReason ?? raw.error_reason,
    startedAt: raw.startedAt ?? raw.started_at,
    finishedAt: raw.finishedAt ?? raw.finished_at
  };
}

function normalizeConnectorDependencies(raw: ConnectorDependenciesDTO): ConnectorDependencies {
  const summary = raw.summary;
  return {
    operationId: raw.operationId ?? raw.operation_id ?? "",
    summary: {
      directToolCount: summary.directToolCount ?? summary.direct_tool_count ?? 0,
      indirectSkillCount: summary.indirectSkillCount ?? summary.indirect_skill_count ?? 0,
      indirectAgentCount: summary.indirectAgentCount ?? summary.indirect_agent_count ?? 0,
      totalConsumerCount: summary.totalConsumerCount ?? summary.total_consumer_count ?? 0,
      blockingToolCount: summary.blockingToolCount ?? summary.blocking_tool_count ?? 0
    },
    directTools: raw.directTools ?? (raw.direct_tools ?? []).map((item) => ({
      id: item.id,
      name: item.name,
      description: item.description,
      requiresReview: item.requiresReview ?? item.requires_review ?? false
    })),
    indirectSkills: raw.indirectSkills ?? (raw.indirect_skills ?? []).map((item) => ({
      id: item.id,
      name: item.name,
      viaToolId: item.viaToolId ?? item.via_tool_id ?? ""
    })),
    indirectAgents: raw.indirectAgents ?? (raw.indirect_agents ?? []).map((item) => ({
      id: item.id,
      name: item.name,
      viaSkillId: item.viaSkillId ?? item.via_skill_id ?? ""
    }))
  };
}

function normalizeConnectorInvokeResult(raw: ConnectorInvokeResultDTO): ConnectorInvokeResult {
  return {
    requestId: raw.requestId ?? raw.request_id ?? "",
    success: raw.success,
    data: raw.data,
    errorCode: raw.errorCode ?? raw.error_code,
    finishedAt: raw.finishedAt ?? raw.finished_at ?? ""
  };
}

export const platformApi = {
  listAgents: () =>
    getJson<ApiListResponse<AgentDTO>>(
      `${endpoints.platform}/v1/agents?tenant_id=${defaultTenantId}`
    ).then((resp) => ({ items: resp.items.map(normalizeAgent) })),
  createAgent: (agent: AgentProfile) =>
    sendJson<AgentDTO>(`${endpoints.platform}/v1/agents`, "POST", serializeAgent(agent)).then(normalizeAgent),
  updateAgent: (agent: AgentProfile) =>
    sendJson<AgentDTO>(
      `${endpoints.platform}/v1/agents/${agent.id}?tenant_id=${defaultTenantId}`,
      "PUT",
      serializeAgent(agent)
    ).then(normalizeAgent),
  enableAgent: (agentId: string) =>
    sendJson<AgentDTO>(`${endpoints.platform}/v1/agents/${agentId}/enable?tenant_id=${defaultTenantId}`, "POST", {}).then(
      normalizeAgent
    ),
  disableAgent: (agentId: string) =>
    sendJson<AgentDTO>(`${endpoints.platform}/v1/agents/${agentId}/disable?tenant_id=${defaultTenantId}`, "POST", {}).then(
      normalizeAgent
    ),
  getAgentDependencies: (agentId: string) =>
    getJson<AgentDependenciesDTO>(`${endpoints.platform}/v1/agents/${agentId}/dependencies?tenant_id=${defaultTenantId}`).then(
      normalizeAgentDependencies
    ),
  listAgentFlows: (status?: AgentFlowSpec["status"] | "all") => {
    const params = new URLSearchParams({ tenant_id: defaultTenantId });
    if (status && status !== "all") {
      params.set("status", status);
    }
    return getJson<ApiListResponse<AgentFlowDTO>>(`${endpoints.platform}/v1/agent-flows?${params.toString()}`).then((resp) => ({
      items: resp.items.map(normalizeAgentFlow)
    }));
  },
  createAgentFlow: (flow: AgentFlowSpec) =>
    sendJson<AgentFlowDTO>(`${endpoints.platform}/v1/agent-flows`, "POST", serializeAgentFlow(flow)).then(normalizeAgentFlow),
  updateAgentFlow: (flow: AgentFlowSpec) =>
    sendJson<AgentFlowDTO>(
      `${endpoints.platform}/v1/agent-flows/${flow.id}?tenant_id=${defaultTenantId}`,
      "PUT",
      serializeAgentFlow(flow)
    ).then(normalizeAgentFlow),
  enableAgentFlow: (flowId: string) =>
    sendJson<AgentFlowDTO>(`${endpoints.platform}/v1/agent-flows/${flowId}/enable?tenant_id=${defaultTenantId}`, "POST", {}).then(
      normalizeAgentFlow
    ),
  disableAgentFlow: (flowId: string) =>
    sendJson<AgentFlowDTO>(`${endpoints.platform}/v1/agent-flows/${flowId}/disable?tenant_id=${defaultTenantId}`, "POST", {}).then(
      normalizeAgentFlow
    ),
  listWorkflows: (options: { status?: WorkflowSpec["status"] | "all"; profile?: WorkflowSpec["profile"] | "all" } = {}) => {
    const params = new URLSearchParams({ tenant_id: defaultTenantId });
    if (options.status && options.status !== "all") {
      params.set("status", options.status);
    }
    if (options.profile && options.profile !== "all") {
      params.set("profile", options.profile);
    }
    return getJson<ApiListResponse<WorkflowDTO>>(`${endpoints.platform}/v1/workflows?${params.toString()}`).then((resp) => ({
      items: resp.items.map(normalizeWorkflow)
    }));
  },
  createWorkflow: (workflow: WorkflowSpec) =>
    sendJson<WorkflowDTO>(`${endpoints.platform}/v1/workflows`, "POST", serializeWorkflow(workflow)).then(normalizeWorkflow),
  updateWorkflow: (workflow: WorkflowSpec) =>
    sendJson<WorkflowDTO>(
      `${endpoints.platform}/v1/workflows/${workflow.id}?tenant_id=${defaultTenantId}`,
      "PUT",
      serializeWorkflow(workflow)
    ).then(normalizeWorkflow),
  enableWorkflow: (workflowId: string) =>
    sendJson<WorkflowDTO>(
      `${endpoints.platform}/v1/workflows/${workflowId}/enable?tenant_id=${defaultTenantId}`,
      "POST",
      {}
    ).then(normalizeWorkflow),
  disableWorkflow: (workflowId: string) =>
    sendJson<WorkflowDTO>(
      `${endpoints.platform}/v1/workflows/${workflowId}/disable?tenant_id=${defaultTenantId}`,
      "POST",
      {}
    ).then(normalizeWorkflow),
  listSkills: () =>
    getJson<ApiListResponse<SkillDTO>>(
      `${endpoints.platform}/v1/skills?tenant_id=${defaultTenantId}`
    ).then((resp) => ({ items: resp.items.map(normalizeSkill) })),
  createSkill: (skill: SkillSpec) =>
    sendJson<SkillDTO>(`${endpoints.platform}/v1/skills`, "POST", serializeSkill(skill)).then(normalizeSkill),
  updateSkill: (skill: SkillSpec) =>
    sendJson<SkillDTO>(
      `${endpoints.platform}/v1/skills/${skill.id}?tenant_id=${defaultTenantId}`,
      "PUT",
      serializeSkill(skill)
    ).then(normalizeSkill),
  enableSkill: (skillId: string) =>
    sendJson<SkillDTO>(`${endpoints.platform}/v1/skills/${skillId}/enable?tenant_id=${defaultTenantId}`, "POST", {}).then(
      normalizeSkill
    ),
  disableSkill: (skillId: string) =>
    sendJson<SkillDTO>(`${endpoints.platform}/v1/skills/${skillId}/disable?tenant_id=${defaultTenantId}`, "POST", {}).then(
      normalizeSkill
    ),
  getSkillDependencies: (skillId: string) =>
    getJson<SkillDependenciesDTO>(`${endpoints.platform}/v1/skills/${skillId}/dependencies?tenant_id=${defaultTenantId}`).then(
      normalizeSkillDependencies
    ),
  listTools: (options: ToolListOptions = {}) => {
    const params = new URLSearchParams({ tenant_id: defaultTenantId });
    if (options.status && options.status !== "all") {
      params.set("status", options.status);
    }
    return getJson<ApiListResponse<ToolDTO>>(`${endpoints.platform}/v1/tools?${params.toString()}`).then((resp) => ({
      items: resp.items.map(normalizeTool)
    }));
  },
  createTool: (tool: ToolSpec) =>
    sendJson<ToolDTO>(`${endpoints.platform}/v1/tools`, "POST", serializeTool(tool)).then(normalizeTool),
  updateTool: (tool: ToolSpec) =>
    sendJson<ToolDTO>(
      `${endpoints.platform}/v1/tools/${tool.id}?tenant_id=${defaultTenantId}`,
      "PUT",
      serializeTool(tool)
    ).then(normalizeTool),
  enableTool: (toolId: string) =>
    sendJson<ToolDTO>(`${endpoints.platform}/v1/tools/${toolId}/enable?tenant_id=${defaultTenantId}`, "POST", {}).then(
      normalizeTool
    ),
  disableTool: (toolId: string) =>
    sendJson<ToolDTO>(`${endpoints.platform}/v1/tools/${toolId}/disable?tenant_id=${defaultTenantId}`, "POST", {}).then(
      normalizeTool
    ),
  getToolDependencies: (toolId: string) =>
    getJson<ToolDependenciesDTO>(`${endpoints.platform}/v1/tools/${toolId}/dependencies?tenant_id=${defaultTenantId}`).then(
      normalizeToolDependencies
    ),
  importConnectorTools: (connectorId: string, tools: ToolSpec[]) =>
    sendJson<ApiListResponse<ToolDTO>>(
      `${endpoints.platform}/v1/connectors/${connectorId}/tools/import?tenant_id=${defaultTenantId}`,
      "POST",
      { tools: tools.map(serializeTool) }
    ).then((resp) => ({ items: resp.items.map(normalizeTool) })),
  discoverMCPTools: (request: MCPDiscoveryRequestDTO) =>
    sendJson<MCPDiscoveryResponseDTO>(`${endpoints.platform}/v1/mcp/servers/discover`, "POST", request).then((resp) => ({
      serverId: resp.serverId ?? resp.server_id ?? request.name,
      tools: (resp.tools ?? []).map((tool) => ({
        name: tool.name,
        description: tool.description ?? "",
        inputSchema: tool.inputSchema ?? tool.input_schema ?? {},
        outputSchema: tool.outputSchema ?? tool.output_schema ?? {}
      }))
    })),
  listConnectors: () =>
    getJson<ApiListResponse<ConnectorDTO>>(`${endpoints.platform}/v1/connectors?tenant_id=${defaultTenantId}`).then((resp) => ({
      items: resp.items.map(normalizeConnector)
    })),
  createConnector: (connector: Connector) =>
    sendJson<ConnectorDTO>(`${endpoints.platform}/v1/connectors`, "POST", serializeConnector(connector)).then(
      normalizeConnector
    ),
  updateConnector: (connector: Connector) =>
    sendJson<ConnectorDTO>(
      `${endpoints.platform}/v1/connectors/${connector.id}?tenant_id=${defaultTenantId}`,
      "PUT",
      serializeConnector(connector)
    ).then(normalizeConnector),
  enableConnector: (connectorId: string) =>
    sendJson<ConnectorDTO>(`${endpoints.platform}/v1/connectors/${connectorId}/enable?tenant_id=${defaultTenantId}`, "POST", {}).then(
      normalizeConnector
    ),
  disableConnector: (connectorId: string) =>
    sendJson<ConnectorDTO>(`${endpoints.platform}/v1/connectors/${connectorId}/disable?tenant_id=${defaultTenantId}`, "POST", {}).then(
      normalizeConnector
    ),
  listConnectorOperations: () =>
    getJson<ApiListResponse<ConnectorOperationDTO>>(
      `${endpoints.platform}/v1/connector-operations?tenant_id=${defaultTenantId}`
    ).then((resp) => ({ items: resp.items.map(normalizeConnectorOperation) })),
  createConnectorOperation: (operation: ConnectorOperation) =>
    sendJson<ConnectorOperationDTO>(
      `${endpoints.platform}/v1/connector-operations`,
      "POST",
      serializeConnectorOperation(operation)
    ).then(normalizeConnectorOperation),
  updateConnectorOperation: (operation: ConnectorOperation) =>
    sendJson<ConnectorOperationDTO>(
      `${endpoints.platform}/v1/connector-operations/${operation.id}?tenant_id=${defaultTenantId}`,
      "PUT",
      serializeConnectorOperation(operation)
    ).then(normalizeConnectorOperation),
  enableConnectorOperation: (operationId: string) =>
    sendJson<ConnectorOperationDTO>(
      `${endpoints.platform}/v1/connector-operations/${operationId}/enable?tenant_id=${defaultTenantId}`,
      "POST",
      {}
    ).then(normalizeConnectorOperation),
  disableConnectorOperation: (operationId: string) =>
    sendJson<ConnectorOperationDTO>(
      `${endpoints.platform}/v1/connector-operations/${operationId}/disable?tenant_id=${defaultTenantId}`,
      "POST",
      {}
    ).then(normalizeConnectorOperation),
  getConnectorDependencies: (operationId: string) =>
    getJson<ConnectorDependenciesDTO>(
      `${endpoints.platform}/v1/connector-operations/${operationId}/dependencies?tenant_id=${defaultTenantId}`
    ).then(normalizeConnectorDependencies)
};

export const runtimeApi = {
  getToolExecution: (callId: string) =>
    getJson<ToolExecution>(
      `${endpoints.runtime}/v1/tool-executions/${callId}?tenant_id=${defaultTenantId}`
    ),
  executeTool: (toolId: string, args: Record<string, unknown>, confirmed = false) =>
    sendJson<ToolExecutionResultDTO>(`${endpoints.runtime}/v1/tools/execute`, "POST", {
      tenant_id: defaultTenantId,
      tool_id: toolId,
      args,
      confirmed
    }).then(normalizeToolExecutionResult)
};

export const agentFlowRuntimeApi = {
  runAgentFlow: (request: { flowId?: string; flow?: AgentFlowSpec; graph?: AgentFlowGraph; input?: Record<string, unknown> }) =>
    sendJson<AgentFlowRunResponseDTO>(`${endpoints.agentFlowRuntime}/v1/agent-flows/run`, "POST", {
      tenant_id: defaultTenantId,
      flow_id: request.flowId,
      flow: request.flow ? serializeAgentFlow(request.flow) : undefined,
      graph: request.graph ? serializeAgentFlowGraph(request.graph) : undefined,
      input: request.input ?? {}
    }).then(normalizeAgentFlowRunResponse)
};

export const workflowRuntimeApi = {
  runWorkflow: (request: { workflowId?: string; workflow?: WorkflowSpec; input?: Record<string, unknown>; context?: Record<string, unknown>; traceId?: string }) =>
    sendJson<WorkflowRunResponseDTO>(`${endpoints.agentFlowRuntime}/v1/workflows/run`, "POST", {
      tenant_id: defaultTenantId,
      workflow_id: request.workflowId ?? request.workflow?.id,
      workflow: request.workflow ? serializeWorkflow(request.workflow) : undefined,
      input: request.input ?? {},
      context: request.context ?? {},
      trace_id: request.traceId
    }).then(normalizeWorkflowRunResponse),
  listRuns: (request: { workflowId?: string; limit?: number } = {}) => {
    const params = new URLSearchParams({ tenant_id: defaultTenantId });
    if (request.workflowId) params.set("workflow_id", request.workflowId);
    if (request.limit) params.set("limit", String(request.limit));
    return getJson<WorkflowRunListDTO>(`${endpoints.agentFlowRuntime}/v1/workflows/runs?${params.toString()}`).then((resp) => ({
      items: resp.items.map(normalizeWorkflowRun)
    }));
  },
  getRun: (runId: string) =>
    getJson<WorkflowRunResponseDTO>(
      `${endpoints.agentFlowRuntime}/v1/workflows/runs/${encodeURIComponent(runId)}?tenant_id=${defaultTenantId}`
    ).then(normalizeWorkflowRunResponse),
  replayRun: (runId: string, request: { input?: Record<string, unknown>; context?: Record<string, unknown>; traceId?: string } = {}) =>
    sendJson<WorkflowRunResponseDTO>(
      `${endpoints.agentFlowRuntime}/v1/workflows/runs/${encodeURIComponent(runId)}/replay?tenant_id=${defaultTenantId}`,
      "POST",
      {
        tenant_id: defaultTenantId,
        input: request.input,
        context: request.context,
        trace_id: request.traceId
      }
    ).then(normalizeWorkflowRunResponse)
};

export const knowledgeApi = {
  listKnowledgeBases: () =>
    getJson<ApiListResponse<KnowledgeBaseDTO>>(`${endpoints.knowledge}/v1/knowledge/bases?tenant_id=${defaultTenantId}`).then((resp) => ({
      items: resp.items.map(normalizeKnowledgeBase)
    })),
  createKnowledgeBase: (base: KnowledgeBase) =>
    sendJson<KnowledgeBaseDTO>(`${endpoints.knowledge}/v1/knowledge/bases`, "POST", serializeKnowledgeBase(base)).then(
      normalizeKnowledgeBase
    ),
  updateKnowledgeBase: (base: KnowledgeBase) =>
    sendJson<KnowledgeBaseDTO>(
      `${endpoints.knowledge}/v1/knowledge/bases/${encodeURIComponent(base.id)}?tenant_id=${defaultTenantId}`,
      "PUT",
      serializeKnowledgeBase(base)
    ).then(normalizeKnowledgeBase),
  setKnowledgeBaseStatus: (kbId: string, status: "enabled" | "disabled") =>
    sendJson<KnowledgeBaseDTO>(
      `${endpoints.knowledge}/v1/knowledge/bases/${encodeURIComponent(kbId)}/${status === "enabled" ? "enable" : "disable"}?tenant_id=${defaultTenantId}`,
      "POST",
      {}
    ).then(normalizeKnowledgeBase),
  listDocuments: (kbId: string) =>
    getJson<ApiListResponse<KnowledgeDocumentDTO>>(
      `${endpoints.knowledge}/v1/knowledge/bases/${encodeURIComponent(kbId)}/documents?tenant_id=${defaultTenantId}`
    ).then((resp) => ({ items: resp.items.map(normalizeKnowledgeDocument) })),
  indexDocument: (document: KnowledgeDocument) =>
    sendJson<{ document_id: string; kb_id: string; chunk_count: number; chunks?: KnowledgeChunkDTO[] }>(
      `${endpoints.knowledge}/v1/knowledge/documents`,
      "POST",
      serializeKnowledgeDocument(document)
    ),
  search: (request: { kbIds?: string[]; text: string; topK?: number; filters?: Record<string, unknown>; traceId?: string }) =>
    sendJson<KnowledgeSearchResultDTO>(`${endpoints.knowledge}/v1/knowledge/search`, "POST", {
      tenant_id: defaultTenantId,
      kb_ids: request.kbIds ?? [],
      text: request.text,
      top_k: request.topK ?? 5,
      filters: request.filters,
      trace_id: request.traceId
    }).then(normalizeKnowledgeSearchResult)
};

export const orchestratorApi = {
  handleEvent: (evt: AgentDebugEvent) =>
    sendJson<AgentDebugResponseDTO>(`${endpoints.orchestrator}/v1/events`, "POST", serializeAgentDebugEvent(evt)).then(
      normalizeAgentDebugResponse
    ),
  subscribeLiveEvents: (traceId: string, onEvent: (event: RuntimeEvent) => void, onError?: (error: Event) => void) => {
    const source = new EventSource(
      `${endpoints.orchestrator}/v1/live-events/${encodeURIComponent(traceId)}?tenant_id=${encodeURIComponent(defaultTenantId)}`
    );
    const eventNames = [
      "connected",
      "run_started",
      "run_completed",
      "run_failed",
      "planning_started",
      "planning_completed",
      "action_planned",
      "action_started",
      "action_completed",
      "action_failed",
      "llm_started",
      "llm_completed",
      "llm_failed",
      "trace_step_added",
      "assistant_message_completed"
    ];
    const handler = (message: MessageEvent<string>) => {
      try {
        onEvent(normalizeRuntimeEvent(JSON.parse(message.data) as RuntimeEventDTO));
      } catch {
        // Ignore malformed stream frames; the final HTTP response still owns correctness.
      }
    };
    eventNames.forEach((eventName) => source.addEventListener(eventName, handler as EventListener));
    if (onError) {
      source.onerror = onError;
    }
    return () => {
      eventNames.forEach((eventName) => source.removeEventListener(eventName, handler as EventListener));
      source.close();
    };
  },
  getTrace: (traceId: string) =>
    getJson<AgentTraceDTO>(
      `${endpoints.orchestrator}/v1/traces/${encodeURIComponent(traceId)}?tenant_id=${defaultTenantId}`
    ).then(normalizeAgentTrace)
};

export const connectorApi = {
  invokeOperation: (operationId: string, args: Record<string, unknown>) =>
    sendJson<ConnectorInvokeResultDTO>(`${endpoints.connector}/v1/connector/invoke`, "POST", {
      tenant_id: defaultTenantId,
      operation_id: operationId,
      args
    }).then(normalizeConnectorInvokeResult)
};
