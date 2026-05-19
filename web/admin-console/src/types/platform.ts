export type ResourceStatus = "draft" | "published" | "enabled" | "disabled";

export type RiskLevel = "low" | "medium" | "high";

export type KnowledgeBase = {
  id: string;
  tenantId: string;
  name: string;
  description?: string;
  status: "draft" | "enabled" | "disabled";
  embeddingModel?: string;
  documentCount: number;
  chunkCount: number;
  metadata?: Record<string, unknown>;
  version: string;
  updatedAt: string;
};

export type KnowledgeDocument = {
  id: string;
  tenantId: string;
  kbId: string;
  title: string;
  text: string;
  metadata?: Record<string, unknown>;
  version: string;
};

export type KnowledgeChunk = {
  id: string;
  docId: string;
  kbId: string;
  text: string;
  score: number;
  metadata?: Record<string, unknown>;
};

export type KnowledgeSearchResult = {
  queryId: string;
  chunks: KnowledgeChunk[];
};

export type ToolImplementation =
  | "connector"
  | "knowledge"
  | "mcp"
  | "python"
  | "workflow";

export type AgentProfile = {
  id: string;
  tenantId: string;
  name: string;
  description: string;
  businessDomain?: string;
  ownerTeam?: string;
  status: "draft" | "enabled" | "disabled";
  skillIds: string[];
  toolIds?: string[];
  defaultLang: string;
  supportedLanguages?: string[];
  channels?: string[];
  systemPrompt?: string;
  welcomeMessage?: string;
  modelConfig?: {
    providerId?: string;
    model?: string;
    temperature?: number;
  };
  runtimePolicy?: {
    maxTurns: number;
    maxToolCalls: number;
    responseTimeoutMs: number;
  };
  version: string;
};

export type AgentDependencies = {
  agentId: string;
  summary: {
    directSkillCount: number;
    directToolCount: number;
    reachableToolCount: number;
    disabledSkillCount: number;
    totalCapabilityCount: number;
  };
  directSkills: Array<{
    id: string;
    name: string;
    status?: string;
  }>;
  reachableTools: Array<{
    id: string;
    name: string;
    viaSkillId?: string;
    source?: "direct" | "skill" | string;
    implementation?: ToolImplementation;
    riskLevel?: RiskLevel;
    status?: string;
  }>;
};

export type AgentDebugEvent = {
  id?: string;
  tenantId: string;
  traceId?: string;
  userId: string;
  sessionId: string;
  agentId: string;
  type: "user_message_committed" | "user_utterance_committed";
  channel: "text" | "voice";
  payload: {
    text: string;
  };
  occurredAt?: string;
};

export type AgentDebugAction = {
  type: "speak" | "display_text" | "ask_question" | "ask_confirmation" | "call_tool" | "wait" | "end_turn";
  text?: string;
  toolCall?: {
    id?: string;
    toolId?: string;
    name?: string;
    args?: Record<string, unknown>;
    confirmed?: boolean;
    traceId?: string;
  };
  toolResult?: ToolExecutionResult;
};

export type AgentDebugResponse = {
  eventId: string;
  traceId: string;
  actions: AgentDebugAction[];
};

export type RuntimeEventType =
  | "connected"
  | "run_started"
  | "run_completed"
  | "run_failed"
  | "planning_started"
  | "planning_completed"
  | "action_planned"
  | "action_started"
  | "action_completed"
  | "action_failed"
  | "llm_started"
  | "llm_completed"
  | "llm_failed"
  | "trace_step_added"
  | "context_assembled"
  | "assistant_message_completed";

export type RuntimeEvent = {
  id?: string;
  type: RuntimeEventType | string;
  tenantId: string;
  runId?: string;
  traceId?: string;
  eventId?: string;
  agentId?: string;
  sessionId?: string;
  parentId?: string;
  stepId?: string;
  stepType?: string;
  name?: string;
  status?: string;
  message?: string;
  payload?: Record<string, unknown>;
  createdAt?: string;
};

export type TraceStepType = "event" | "agent" | "skill" | "model" | "tool" | "workflow" | "node" | "connector";

export type TraceStep = {
  id: string;
  parentId?: string;
  type: TraceStepType;
  name: string;
  status: "started" | "succeeded" | "failed" | "skipped";
  startedAt: string;
  finishedAt?: string;
  durationMillis?: number;
  metadata?: Record<string, unknown>;
  error?: string;
};

export type AgentTrace = {
  traceId: string;
  tenantId: string;
  agentId?: string;
  sessionId?: string;
  eventId?: string;
  status: "running" | "succeeded" | "failed";
  startedAt: string;
  finishedAt?: string;
  durationMillis?: number;
  error?: string;
  steps: TraceStep[];
};

export type AgentFlowStatus = "draft" | "enabled" | "disabled";
export type AgentFlowOrchestrationMode = "workflow" | "supervisor";

export type AgentFlowNodeType =
  | "start"
  | "supervisor_node"
  | "agent_node"
  | "planner_node"
  | "router_node"
  | "aggregator_node"
  | "verifier_node"
  | "join_node"
  | "end"
  | "connector_operation"
  | "tool"
  | "skill"
  | "agent"
  | "transform"
  | "condition"
  | "join";

export type AgentFlowEdgeType = "default" | "conditional" | "fallback";

export type AgentFlowNode = {
  id: string;
  type: AgentFlowNodeType;
  name: string;
  description?: string;
  config?: Record<string, unknown>;
  timeoutMillis?: number;
  retryPolicy?: {
    maxAttempts?: number;
    backoffMillis?: number;
  };
};

export type AgentFlowEdge = {
  id?: string;
  fromNodeId: string;
  toNodeId: string;
  type?: AgentFlowEdgeType;
  condition?: {
    path: string;
    equals?: unknown;
    exists?: boolean;
  };
  description?: string;
};

export type AgentFlowGraph = {
  id: string;
  tenantId: string;
  name: string;
  description?: string;
  status: AgentFlowStatus;
  version?: string;
  entryNodeId: string;
  nodes: Record<string, AgentFlowNode>;
  edges: AgentFlowEdge[];
  policy?: {
    maxSteps?: number;
    maxParallelism?: number;
    timeoutMillis?: number;
  };
};

export type AgentFlowSpec = {
  id: string;
  tenantId: string;
  name: string;
  description?: string;
  businessDomain?: string;
  ownerTeam?: string;
  status: AgentFlowStatus;
  orchestrationMode: AgentFlowOrchestrationMode;
  supervisor?: {
    supervisorAgentId?: string;
    subAgentIds: string[];
    maxDepth: number;
    maxSubAgentCalls: number;
    planningPrompt?: string;
    finalPrompt?: string;
  };
  graph: AgentFlowGraph;
  contextSchema?: Record<string, unknown>;
  inputSchema?: Record<string, unknown>;
  outputSchema?: Record<string, unknown>;
  version: string;
};

export type AgentFlowRun = {
  id: string;
  tenantId: string;
  flowId: string;
  flowVersion?: string;
  status: "pending" | "running" | "succeeded" | "failed" | "canceled";
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: string;
  startedAt: string;
  finishedAt?: string;
};

export type AgentFlowNodeRun = {
  id: string;
  tenantId: string;
  runId: string;
  flowId: string;
  nodeId: string;
  nodeType: AgentFlowNodeType;
  nodeName?: string;
  status: "pending" | "running" | "succeeded" | "failed" | "skipped" | "canceled";
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: string;
  startedAt: string;
  finishedAt?: string;
};

export type AgentFlowRunResponse = {
  run: AgentFlowRun;
  nodeRuns: AgentFlowNodeRun[];
  error?: string;
};

export type WorkflowStatus = "draft" | "enabled" | "disabled";
export type WorkflowProfile = "tool_workflow" | "agent_workflow";

export type WorkflowNodeType =
  | "start"
  | "end"
  | "join"
  | "transform"
  | "condition"
  | "connector_operation"
  | "tool"
  | "skill"
  | "agent";

export type WorkflowEdgeType = "default" | "conditional" | "fallback";

export type WorkflowNode = {
  id: string;
  type: WorkflowNodeType;
  name: string;
  description?: string;
  position?: {
    x?: number;
    y?: number;
  };
  config?: Record<string, unknown>;
  timeoutMillis?: number;
  retryPolicy?: {
    maxAttempts?: number;
    backoffMillis?: number;
  };
};

export type WorkflowEdge = {
  id?: string;
  fromNodeId: string;
  toNodeId: string;
  type?: WorkflowEdgeType;
  condition?: {
    path: string;
    equals?: unknown;
    exists?: boolean;
  };
  description?: string;
};

export type WorkflowGraph = {
  entryNodeId: string;
  nodes: Record<string, WorkflowNode>;
  edges: WorkflowEdge[];
};

export type WorkflowSpec = {
  id: string;
  tenantId: string;
  name: string;
  description?: string;
  businessDomain?: string;
  ownerTeam?: string;
  status: WorkflowStatus;
  profile: WorkflowProfile;
  contextSchema?: Record<string, unknown>;
  inputSchema?: Record<string, unknown>;
  outputSchema?: Record<string, unknown>;
  graph: WorkflowGraph;
  policy?: {
    maxSteps?: number;
    maxParallelism?: number;
    timeoutMillis?: number;
  };
  ui?: Record<string, unknown>;
  version: string;
};

export type WorkflowRun = {
  id: string;
  tenantId: string;
  workflowId: string;
  version?: string;
  status: "pending" | "running" | "succeeded" | "failed" | "canceled";
  input?: Record<string, unknown>;
  context?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: string;
  traceId?: string;
  startedAt: string;
  finishedAt?: string;
};

export type WorkflowNodeRun = {
  id: string;
  tenantId: string;
  runId: string;
  workflowId: string;
  nodeId: string;
  nodeType: WorkflowNodeType;
  nodeName?: string;
  status: "pending" | "running" | "succeeded" | "failed" | "skipped" | "canceled";
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  context?: Record<string, unknown>;
  error?: string;
  startedAt: string;
  finishedAt?: string;
};

export type WorkflowRunResponse = {
  run: WorkflowRun;
  nodeRuns: WorkflowNodeRun[];
  error?: string;
};

export type SkillSpec = {
  id: string;
  tenantId: string;
  name: string;
  description: string;
  businessDomain?: string;
  ownerTeam?: string;
  status: "draft" | "enabled" | "disabled";
  toolIds: string[];
  knowledgeIds: string[];
  systemPrompt: string;
  useCases?: string[];
  exclusions?: string[];
  outputFormat?: string;
  riskLevel: RiskLevel;
  executionPolicy?: {
    maxToolCalls: number;
    timeoutMillis: number;
    allowWriteTools: boolean;
    requireConfirmation: boolean;
  };
  policyVersion?: string;
  version: string;
};

export type SkillDependencies = {
  skillId: string;
  summary: {
    directAgentCount: number;
    totalConsumerCount: number;
  };
  directAgents: Array<{
    id: string;
    name: string;
  }>;
};

export type ToolSpec = {
  id: string;
  tenantId: string;
  name: string;
  description: string;
  businessDomain?: string;
  ownerTeam?: string;
  llmDescription?: string;
  implementation: ToolImplementation;
  binding?: {
    connectorOperationId?: string;
    knowledgeBaseIds?: string[];
    pythonPackageId?: string;
    mcpServerId?: string;
    mcpServerUrl?: string;
    mcpTransport?: "streamable_http" | "sse" | string;
    mcpHeaders?: Record<string, string>;
    mcpToolName?: string;
    workflowId?: string;
  };
  inputSchema?: Record<string, unknown>;
  outputSchema?: Record<string, unknown>;
  sideEffect: "none" | "read" | "write";
  riskLevel: RiskLevel;
  requiresConfirmation: boolean;
  timeoutMillis: number;
  retryPolicy?: {
    maxAttempts: number;
    backoffMillis: number;
  };
  status: "draft" | "enabled" | "disabled";
  version: string;
};

export type ToolDependencies = {
  toolId: string;
  summary: {
    directSkillCount: number;
    directAgentCount: number;
    indirectAgentCount: number;
    totalConsumerCount: number;
  };
  directSkills: Array<{
    id: string;
    name: string;
  }>;
  indirectAgents: Array<{
    id: string;
    name: string;
    viaSkillId?: string;
    source?: string;
  }>;
  directAgents: Array<{
    id: string;
    name: string;
    viaSkillId?: string;
    source?: string;
  }>;
};

export type ToolExecutionResult = {
  callId: string;
  toolId: string;
  success: boolean;
  data?: Record<string, unknown>;
  errorCode?: string;
  errorReason?: string;
  startedAt?: string;
  finishedAt?: string;
};

export type ConnectorOperation = {
  id: string;
  tenantId: string;
  connectorId?: string;
  name: string;
  description: string;
  businessDomain?: string;
  ownerTeam?: string;
  type: "http";
  status: "draft" | "enabled" | "disabled";
  implementationMode?: "simple_http" | "template_mapping" | "adapter_service" | "workflow" | "mock";
  method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  baseUrl: string;
  path: string;
  headers?: Record<string, string>;
  auth?: {
    type: "none" | "api_key" | "bearer" | "basic" | "oauth2";
    headerName?: string;
    secretRef?: string;
    config?: Record<string, unknown>;
  };
  inputSchema?: Record<string, unknown>;
  outputSchema?: Record<string, unknown>;
  timeoutMillis: number;
  version?: string;
};

export type Connector = {
  id: string;
  tenantId: string;
  name: string;
  description: string;
  businessDomain?: string;
  ownerTeam?: string;
  type: "http";
  status: "draft" | "enabled" | "disabled";
  baseUrl: string;
  headers?: Record<string, string>;
  auth?: {
    type: "none" | "api_key" | "bearer" | "basic" | "oauth2";
    headerName?: string;
    secretRef?: string;
    config?: Record<string, unknown>;
  };
  timeoutMillis: number;
  version?: string;
};

export type ConnectorToolConsumer = {
  id: string;
  name: string;
  description?: string;
  requiresReview: boolean;
};

export type ConnectorSkillConsumer = {
  id: string;
  name: string;
  viaToolId: string;
};

export type ConnectorAgentConsumer = {
  id: string;
  name: string;
  viaSkillId: string;
};

export type ConnectorDependencies = {
  operationId: string;
  summary: {
    directToolCount: number;
    indirectSkillCount: number;
    indirectAgentCount: number;
    totalConsumerCount: number;
    blockingToolCount: number;
  };
  directTools: ConnectorToolConsumer[];
  indirectSkills: ConnectorSkillConsumer[];
  indirectAgents: ConnectorAgentConsumer[];
};

export type ConnectorInvokeResult = {
  requestId: string;
  success: boolean;
  data?: Record<string, unknown>;
  errorCode?: string;
  finishedAt: string;
};

export type ExecutionStatus = "started" | "succeeded" | "failed";

export type ToolExecution = {
  callId: string;
  tenantId: string;
  toolId: string;
  toolName: string;
  implementation: ToolImplementation;
  riskLevel: RiskLevel;
  requiresConfirmation: boolean;
  confirmed: boolean;
  traceId: string;
  status: ExecutionStatus;
  errorCode?: string;
  durationMillis: number;
  startedAt: string;
};

export type ModelProvider = {
  id: string;
  name: string;
  type: "mock" | "openai-compatible" | "deepseek";
  baseUrl: string;
  defaultModel: string;
  status: ResourceStatus;
  timeoutMillis: number;
};

export type DashboardMetric = {
  label: string;
  value: string;
  delta: string;
  tone: "blue" | "green" | "amber" | "red";
};
