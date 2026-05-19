import type { FlowEdgeSpec, FlowNodeSpec, SchemaField, WorkflowConfig } from "../../platform/configTypes";
import type { AgentFlowEdge, AgentFlowNode, AgentFlowNodeType, AgentFlowSpec, AgentProfile, SkillSpec, ToolSpec } from "../../types/platform";
import { agentFlowInputSchema, agentFlowOutputSchema } from "./domain";

const tenantId = "tenant_1";

type AgentFlowConfigResources = {
  agents?: AgentProfile[];
  skills?: SkillSpec[];
  tools?: ToolSpec[];
};

export function isAgentFlowWorkflowConfig(workflow: WorkflowConfig): boolean {
  return workflow.ui?.kind === "agent_flow";
}

export function agentFlowFromWorkflowConfig(workflow: WorkflowConfig): AgentFlowSpec {
  const status = resourceStatus(workflow);
  const orchestrationMode = workflow.ui?.orchestration_mode === "supervisor" ? "supervisor" : "workflow";
  const legacySpec = workflow.spec as unknown as {
    description?: string;
    entry_node_id?: string;
    policy?: Record<string, unknown>;
  };
  const policies = workflow.spec.policies ?? legacySpec.policy;
  const graph = {
    id: workflow.spec.id || workflow.id,
    tenantId,
    name: workflow.spec.name || workflow.name,
    description: legacySpec.description ?? workflow.description,
    status,
    version: workflow.version,
    entryNodeId: stringFromUnknown(workflow.ui?.entry_node_id) || legacySpec.entry_node_id || "start",
    nodes: Object.fromEntries((workflow.spec.nodes ?? []).map((node) => [node.id, agentFlowNodeFromSpec(node, workflow.ui)])),
    edges: (workflow.spec.edges ?? []).map((edge) => agentFlowEdgeFromSpec(edge, workflow.ui)),
    policy: {
      maxSteps: numberFromRecord(policies, "max_steps") ?? numberFromRecord(policies, "max_node_executions"),
      maxParallelism: numberFromRecord(policies, "max_parallelism"),
      timeoutMillis: durationMillis(recordFromUnknown(policies).timeout, undefined)
    }
  };

  return {
    id: workflow.id,
    tenantId,
    name: workflow.name,
    description: workflow.description,
    businessDomain: workflow.labels?.[0] ?? "General",
    ownerTeam: workflow.owner?.team ?? "AI Platform",
    status,
    orchestrationMode,
    supervisor: supervisorConfig(workflow),
    graph,
    contextSchema: recordFromUnknown(workflow.spec.context_schema),
    inputSchema: agentFlowInputSchema,
    outputSchema: agentFlowOutputSchema,
    version: workflow.version || "v1"
  };
}

export function workflowConfigFromAgentFlow(flow: AgentFlowSpec, current?: WorkflowConfig, resources: AgentFlowConfigResources = {}): WorkflowConfig {
  const id = flow.id || stableResourceId("agent_flow", flow.name);
  const status = flow.status;
  const next: WorkflowConfig = {
    id,
    name: flow.name.trim(),
    description: flow.description ?? "",
    version: flow.version || "v1",
    disabled: status !== "enabled",
    labels: [flow.businessDomain ?? "General"].filter(Boolean),
    owner: {
      team: flow.ownerTeam ?? "AI Platform",
      email: ""
    },
    metadata: {
      status
    },
    spec: {
      id,
      name: flow.name,
      version: flow.version || "v1",
      nodes: Object.values(flow.graph.nodes).map((node) => flowNodeToSpec(node, resources, flow.orchestrationMode, flow.graph)),
      edges: flow.graph.edges.map(flowEdgeToSpec),
      context_schema: {
        flow_input: schemaFieldsFromJsonSchema(agentFlowInputSchema),
        flow_output: schemaFieldsFromJsonSchema(agentFlowOutputSchema),
        variables: schemaFieldsFromJsonSchema(flow.contextSchema ?? {}),
        node_context: {}
      },
      policies: {
        max_node_executions: flow.graph.policy?.maxSteps
      }
    },
    ui: {
      kind: "agent_flow",
      entry_node_id: flow.graph.entryNodeId,
      graph: {
        description: flow.description,
        max_parallelism: flow.graph.policy?.maxParallelism,
        timeout_ms: flow.graph.policy?.timeoutMillis
      },
      nodes: Object.fromEntries(
        Object.values(flow.graph.nodes).map((node) => [
          node.id,
          {
            description: node.description,
            config: node.config,
            timeout_ms: node.timeoutMillis,
            type: node.type
          }
        ])
      ),
      edges: Object.fromEntries(
        flow.graph.edges.map((edge) => [
          edge.id || `${edge.fromNodeId}-${edge.toNodeId}`,
          {
            from: edge.fromNodeId,
            to: edge.toNodeId,
            type: edge.type,
            condition: edge.condition,
            description: edge.description
          }
        ])
      ),
      orchestration_mode: flow.orchestrationMode,
      supervisor: flow.supervisor
    },
    publish: {
      status,
      revision: 0
    },
    runtime: {
      network: true,
      server_proxy_allowed: true
    }
  };
  if (!current) return next;
  return {
    ...current,
    ...next,
    annotations: current.annotations ?? next.annotations,
    metadata: {
      ...(current.metadata ?? {}),
      ...(next.metadata ?? {})
    },
    spec: next.spec,
    ui: {
      ...(current.ui ?? {}),
      ...(next.ui ?? {})
    },
    publish: {
      ...(current.publish ?? {}),
      ...(next.publish ?? {})
    },
    runtime: {
      ...(current.runtime ?? {}),
      ...(next.runtime ?? {})
    }
  };
}

function agentFlowNodeFromSpec(node: FlowNodeSpec, ui: Record<string, unknown> | undefined): AgentFlowNode {
  const metadata = uiNodeMetadata(ui, node.id);
  const uiConfig = recordFromUnknown(metadata.config);
  const config = Object.keys(uiConfig).length > 0 ? uiConfig : uiConfigFromEngineNode(node);
  const uiType = stringFromUnknown(metadata.type);
  return {
    id: node.id,
    type: agentFlowNodeType(uiType || node.type),
    name: node.name ?? node.id,
    description: stringFromUnknown(metadata.description),
    config,
    timeoutMillis: numberFromRecord(metadata, "timeout_ms") ?? durationMillis(undefined, undefined)
  };
}

function flowNodeToSpec(
  node: AgentFlowNode,
  resources: AgentFlowConfigResources,
  orchestrationMode: AgentFlowSpec["orchestrationMode"],
  graph: AgentFlowSpec["graph"]
): FlowNodeSpec {
  const inputMapping = inputMappingForNode(node);
  return {
    id: node.id,
    type: engineNodeType(node.type),
    name: node.name,
    config: engineNodeConfig(node, resources, orchestrationMode, graph),
    input_mappings: fieldBindingsFromRecord(inputMapping),
    output_writes: contextWritesFromRecord((node.config?.write_context ?? node.config?.context_writes) as Record<string, unknown> | undefined, true),
    timeout: node.timeoutMillis ? node.timeoutMillis * 1000000 : undefined,
    retry_policy: node.retryPolicy
      ? {
          max_attempts: node.retryPolicy.maxAttempts,
          backoff: (node.retryPolicy.backoffMillis ?? 0) * 1000000
        }
      : undefined
  };
}

function inputMappingForNode(node: AgentFlowNode): Record<string, unknown> | undefined {
  const configured = node.config?.input_mapping as Record<string, unknown> | undefined;
  if (configured && Object.keys(configured).length > 0) return configured;
  if (node.type === "agent" || node.type === "agent_node" || node.type === "supervisor_node") {
    return { user_request: "$flow_input.user_request" };
  }
  return configured;
}

function agentFlowEdgeFromSpec(edge: FlowEdgeSpec, ui: Record<string, unknown> | undefined): AgentFlowEdge {
  const metadata = uiEdgeMetadata(ui, edge.from, edge.to);
  return {
    id: stringFromUnknown(metadata.id) || `${edge.from}-${edge.to}`,
    fromNodeId: edge.from,
    toNodeId: edge.to,
    type: metadata.type === "conditional" || metadata.type === "fallback" ? metadata.type : "default",
    condition: metadata.condition as AgentFlowEdge["condition"],
    description: stringFromUnknown(metadata.description)
  };
}

function flowEdgeToSpec(edge: AgentFlowEdge): FlowEdgeSpec {
  return {
    from: edge.fromNodeId,
    to: edge.toNodeId
  };
}

function supervisorConfig(workflow: WorkflowConfig): NonNullable<AgentFlowSpec["supervisor"]> {
  const supervisor = recordFromUnknown(workflow.ui?.supervisor);
  return {
    supervisorAgentId: typeof supervisor.supervisorAgentId === "string" ? supervisor.supervisorAgentId : undefined,
    subAgentIds: Array.isArray(supervisor.subAgentIds) ? supervisor.subAgentIds.map(String) : [],
    maxDepth: numberFromRecord(supervisor, "maxDepth") ?? 4,
    maxSubAgentCalls: numberFromRecord(supervisor, "maxSubAgentCalls") ?? 5,
    planningPrompt: typeof supervisor.planningPrompt === "string" ? supervisor.planningPrompt : undefined,
    finalPrompt: typeof supervisor.finalPrompt === "string" ? supervisor.finalPrompt : undefined
  };
}

function engineNodeType(value: AgentFlowNodeType): string {
  switch (value) {
    case "start":
      return "control.start";
    case "end":
      return "control.end";
    case "join":
    case "join_node":
      return "control.join";
    case "condition":
      return "control.condition";
    case "transform":
    case "planner_node":
    case "router_node":
    case "aggregator_node":
    case "verifier_node":
      return "workflow.transform";
    case "connector_operation":
      return "workflow.connector";
    case "tool":
      return "workflow.tool";
    case "agent":
    case "agent_node":
    case "supervisor_node":
      return "workflow.agent";
    default:
      return value;
  }
}

function engineNodeConfig(
  node: AgentFlowNode,
  resources: AgentFlowConfigResources,
  orchestrationMode: AgentFlowSpec["orchestrationMode"],
  graph: AgentFlowSpec["graph"]
): Record<string, unknown> {
  const config = recordFromUnknown(node.config);
  switch (node.type) {
    case "agent":
    case "agent_node":
    case "supervisor_node":
      return cleanObject({
        agent: agentSpecFromNode(node, resources, orchestrationMode, graph),
        message_field: stringConfig(config, "message_field") || "user_request",
        metadata: {
          node_type: node.type,
          agent_routing_mode: stringConfig(config, "agent_routing_mode")
        }
      });
    case "connector_operation":
      return cleanObject({
        operation_id: stringConfig(config, "connector_operation_id") || stringConfig(config, "operation_id"),
        metadata: recordFromUnknown(config.metadata)
      });
    case "tool":
      return cleanObject({
        tool_id: stringConfig(config, "tool_id"),
        metadata: recordFromUnknown(config.metadata)
      });
    case "transform":
    case "planner_node":
    case "router_node":
    case "aggregator_node":
    case "verifier_node":
      return cleanObject({
        function: stringConfig(config, "function") || stringConfig(config, "function_id") || "json.pick",
        args: recordFromUnknown(config.args)
      });
    default:
      return cleanObject(config);
  }
}

function uiConfigFromEngineNode(node: FlowNodeSpec): Record<string, unknown> {
  const config = { ...(node.config ?? {}) };
  if (node.type === "workflow.connector") {
    config.connector_operation_id = stringConfig(config, "operation_id");
  }
  if (node.type === "workflow.tool") {
    config.tool_id = stringConfig(config, "tool_id");
  }
  return cleanObject({
    ...config,
    input_mapping: recordFromFieldBindings(node.input_mappings),
    write_context: recordFromContextWrites(node.output_writes)
  });
}

function agentSpecFromNode(
  node: AgentFlowNode,
  resources: AgentFlowConfigResources,
  orchestrationMode: AgentFlowSpec["orchestrationMode"],
  graph: AgentFlowSpec["graph"]
): Record<string, unknown> {
  const config = recordFromUnknown(node.config);
  const localAgent = recordFromUnknown(config.local_agent);
  const mode = stringConfig(config, "agent_mode");
  const reasoningMode = agentFlowReasoningMode(orchestrationMode);
  const maxIterations = orchestrationMode === "supervisor" ? 1 : 8;
  const routingMode = stringConfig(config, "agent_routing_mode");
  const routingOutputSchema = routingMode === "agent_directed" ? agentDirectedOutputSchema(node, graph) : undefined;
  if (mode === "local" && Object.keys(localAgent).length > 0) {
    const name = stringConfig(localAgent, "name") || node.name;
    const model = stringConfig(localAgent, "model") || "deepseek-v4-flash";
    const skillIds = stringList(localAgent.skillIds ?? localAgent.skill_ids);
    const toolIds = stringList(localAgent.toolIds ?? localAgent.tool_ids);
    return cleanObject({
      id: stableResourceId("agent_local", `${node.id}_${name}`),
      name,
      description: stringConfig(localAgent, "description") || node.description || "",
      prompt: stringConfig(localAgent, "systemPrompt") || stringConfig(localAgent, "system_prompt"),
      reasoning_mode: reasoningMode,
      model: {
        provider: providerFromModel(model),
        model,
        temperature: 0.2
      },
      capabilities: capabilityDescriptors(skillIds, toolIds, resources),
      output_schema: routingOutputSchema,
      policy: {
        max_iterations: maxIterations,
        max_actions: 8,
        validate_final_output: routingMode === "agent_directed"
      }
    });
  }

  const agentId = stringConfig(config, "agent_id") || stringConfig(config, "agentId") || node.id;
  const existing = resources.agents?.find((agent) => agent.id === agentId);
  if (existing) {
    return agentSpecFromProfile(existing, resources, orchestrationMode, routingOutputSchema, routingMode === "agent_directed");
  }
  return cleanObject({
    id: agentId,
    name: node.name,
    description: node.description || "",
    reasoning_mode: reasoningMode,
    model: {
      provider: "deepseek",
      model: "deepseek-v4-flash",
      temperature: 0.2
    },
    policy: {
      max_iterations: maxIterations,
      max_actions: 8,
      validate_final_output: routingMode === "agent_directed"
    },
    output_schema: routingOutputSchema
  });
}

function agentSpecFromProfile(
  agent: AgentProfile,
  resources: AgentFlowConfigResources,
  orchestrationMode: AgentFlowSpec["orchestrationMode"],
  outputSchema?: SchemaField[],
  validateFinalOutput = false
): Record<string, unknown> {
  const model = agent.modelConfig?.model || "deepseek-v4-flash";
  const maxIterations = orchestrationMode === "supervisor" ? 1 : (agent.runtimePolicy?.maxTurns ?? 8);
  return cleanObject({
    id: agent.id,
    name: agent.name,
    description: agent.description,
    prompt: agent.systemPrompt,
    reasoning_mode: agentFlowReasoningMode(orchestrationMode),
    model: {
      provider: agent.modelConfig?.providerId || providerFromModel(model),
      model,
      temperature: agent.modelConfig?.temperature ?? 0.2
    },
    capabilities: capabilityDescriptors(agent.skillIds ?? [], agent.toolIds ?? [], resources),
    output_schema: outputSchema,
    policy: {
      max_iterations: maxIterations,
      max_actions: agent.runtimePolicy?.maxToolCalls ?? 8,
      validate_final_output: validateFinalOutput
    }
  });
}

function agentDirectedOutputSchema(node: AgentFlowNode, graph: AgentFlowSpec["graph"]): SchemaField[] {
  const nextOptions = graph.edges
    .filter((edge) => edge.fromNodeId === node.id)
    .map((edge) => {
      const target = graph.nodes[edge.toNodeId];
      const label = target?.name || edge.toNodeId;
      return `${edge.toNodeId} (${label})`;
    });
  const routingDescription =
    nextOptions.length > 0
      ? `Select zero or more next node ids from connected outgoing nodes only: ${nextOptions.join(", ")}. Return [] when the flow should stop after this node.`
      : "Return [] because this node has no connected outgoing nodes.";
  return [
    {
      name: "text",
      type: "string",
      required: true,
      description: "User-facing response or concise progress summary from this agent node."
    },
    {
      name: "next_node_ids",
      type: "array",
      required: true,
      description: routingDescription
    }
  ];
}

function agentFlowReasoningMode(orchestrationMode: AgentFlowSpec["orchestrationMode"]): string {
  return orchestrationMode === "supervisor" ? "rewoo" : "react";
}

function capabilityDescriptors(skillIds: string[], toolIds: string[], resources: AgentFlowConfigResources): Array<Record<string, unknown>> {
  const descriptors: Array<Record<string, unknown>> = [];
  const skills = resources.skills ?? [];
  const tools = resources.tools ?? [];
  for (const skillId of skillIds) {
    const skill = skills.find((item) => item.id === skillId);
    descriptors.push({
      id: skillId,
      type: "skill",
      name: skill?.name ?? skillId,
      description: skill?.description ?? ""
    });
  }
  // Skill-owned tools must stay inside the Skill runtime. Exposing them here
  // lets the parent Agent bypass the Skill and call tools directly.
  const directToolIds = new Set(toolIds);
  for (const toolId of directToolIds) {
    const tool = tools.find((item) => item.id === toolId);
    descriptors.push({
      id: toolId,
      type: "tool",
      name: tool?.name ?? toolId,
      description: tool?.llmDescription || tool?.description || ""
    });
  }
  return descriptors;
}

function fieldBindingsFromRecord(record: Record<string, unknown> | undefined) {
  return Object.entries(record ?? {})
    .filter(([field]) => field.trim())
    .map(([field, source]) => ({
      field: field.trim(),
      source: valueSourceFromExpression(source, false),
      enabled: true
    }));
}

function contextWritesFromRecord(record: Record<string, unknown> | undefined, preferNodeOutput: boolean) {
  return Object.entries(record ?? {})
    .filter(([target]) => target.trim())
    .map(([target, source]) => ({
      target: target.trim(),
      source: valueSourceFromExpression(source, preferNodeOutput),
      enabled: true
    }));
}

function recordFromFieldBindings(bindings: FlowNodeSpec["input_mappings"]): Record<string, unknown> {
  return (bindings ?? []).reduce<Record<string, unknown>>((acc, binding) => {
    if (!binding.field || binding.enabled === false) return acc;
    acc[binding.field] = expressionFromValueSource(binding.source);
    return acc;
  }, {});
}

function recordFromContextWrites(writes: FlowNodeSpec["output_writes"]): Record<string, unknown> {
  return (writes ?? []).reduce<Record<string, unknown>>((acc, write) => {
    if (!write.target || write.enabled === false) return acc;
    acc[write.target] = expressionFromValueSource(write.source);
    return acc;
  }, {});
}

function valueSourceFromExpression(value: unknown, preferNodeOutput: boolean): { type: "context" | "const" | "node_output"; path?: string; value?: unknown } {
  if (typeof value !== "string") return { type: "const", value };
  const trimmed = value.trim();
  if (!trimmed) return { type: "const", value: "" };
  if (trimmed.startsWith("$")) {
    if (preferNodeOutput && (trimmed === "$" || trimmed.startsWith("$."))) {
      return { type: "node_output", path: trimmed };
    }
    return { type: "context", path: normalizeContextExpression(trimmed) };
  }
  return { type: "const", value: parseMaybeJSON(trimmed) };
}

function expressionFromValueSource(source: { type?: string; path?: string; value?: unknown } | undefined): unknown {
  if (!source) return "";
  if (source.type === "context") return denormalizeContextExpression(source.path ?? "");
  if (source.type === "node_output") return source.path || "$";
  return stringifyConst(source.value);
}

function normalizeContextExpression(value: string): string {
  if (value.startsWith("$responses.connector.")) return value.replace("$responses.connector.", "$node_context.connector.responses.");
  if (value.startsWith("$responses.tool.")) return value.replace("$responses.tool.", "$node_context.tool.calls.");
  return value;
}

function denormalizeContextExpression(value: string): string {
  if (value.startsWith("$node_context.connector.responses.")) return value.replace("$node_context.connector.responses.", "$responses.connector.");
  if (value.startsWith("$node_context.tool.calls.")) return value.replace("$node_context.tool.calls.", "$responses.tool.");
  return value;
}

function providerFromModel(model: string): string {
  const normalized = model.toLowerCase();
  if (normalized.includes("deepseek")) return "deepseek";
  if (normalized.includes("mock")) return "mock";
  return "openai-compatible";
}

function schemaFieldsFromJsonSchema(schema: Record<string, unknown>): SchemaField[] {
	const explicitFields = schema["x-flow-fields"];
	if (Array.isArray(explicitFields)) {
		const flatFields = explicitFields
			.map((entry) => recordFromUnknown(entry))
			.filter((entry) => typeof entry.path === "string")
			.map((entry) => ({
				path: String(entry.path),
				type: typeof entry.type === "string" ? entry.type : "string",
				description: typeof entry.description === "string" ? entry.description : "",
				required: Boolean(entry.required)
			}));
		return schemaFieldsFromFlatPaths(flatFields);
	}

	const properties = recordFromUnknown(schema.properties);
	const required = new Set(Array.isArray(schema.required) ? schema.required.map(String) : []);
	return Object.entries(properties).map(([name, value]) => {
    const definition = recordFromUnknown(value);
		return {
			name,
			type: typeof definition.type === "string" ? definition.type : "string",
			description: typeof definition.description === "string" ? definition.description : "",
			required: required.has(name),
			children: schemaFieldsFromJsonSchema(definition)
		};
	});
}

function schemaFieldsFromFlatPaths(fields: Array<{ path: string; type: string; description: string; required: boolean }>): SchemaField[] {
	const root: SchemaField[] = [];
	for (const field of fields) {
		const parts = field.path
			.split(".")
			.map((part) => part.trim())
			.filter(Boolean);
		if (parts.length === 0) continue;
		let current = root;
		parts.forEach((part, index) => {
			const isLeaf = index === parts.length - 1;
			let existing = current.find((item) => item.name === part);
			if (!existing) {
				existing = {
					name: part,
					type: isLeaf ? field.type : "object",
					description: isLeaf ? field.description : "",
					required: isLeaf ? field.required : false,
					children: []
				};
				current.push(existing);
			}
			if (isLeaf) {
				existing.type = field.type;
				existing.description = field.description;
				existing.required = field.required;
				return;
			}
			if (!existing.children) existing.children = [];
			current = existing.children;
		});
	}
	return root;
}

function resourceStatus(resource: { disabled?: boolean; metadata?: Record<string, unknown> }): AgentFlowSpec["status"] {
  const status = resource.metadata?.status;
  if (status === "draft" || status === "enabled" || status === "disabled") return status;
  return resource.disabled ? "disabled" : "enabled";
}

function agentFlowNodeType(value: string): AgentFlowNodeType {
  const mapped: Record<string, AgentFlowNodeType> = {
    "control.start": "start",
    "control.end": "end",
    "control.condition": "condition",
    "control.join": "join",
    "workflow.transform": "transform",
    "workflow.connector": "connector_operation",
    "workflow.tool": "tool",
    "workflow.agent": "agent_node"
  };
  value = mapped[value] ?? value;
  if (
    value === "start" ||
    value === "supervisor_node" ||
    value === "agent_node" ||
    value === "planner_node" ||
    value === "router_node" ||
    value === "aggregator_node" ||
    value === "verifier_node" ||
    value === "join_node" ||
    value === "end" ||
    value === "connector_operation" ||
    value === "tool" ||
    value === "skill" ||
    value === "agent" ||
    value === "transform" ||
    value === "condition" ||
    value === "join"
  ) {
    return value;
  }
  return "agent_node";
}

function recordFromUnknown(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
}

function stringConfig(record: unknown, key: string): string {
  const value = recordFromUnknown(record)[key];
  return typeof value === "string" ? value.trim() : "";
}

function stringFromUnknown(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value : undefined;
}

function stringList(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string" && item.trim().length > 0) : [];
}

function cleanObject<T extends Record<string, unknown>>(value: T): T {
  const output: Record<string, unknown> = {};
  for (const [key, entry] of Object.entries(value)) {
    if (entry === undefined) continue;
    if (entry && typeof entry === "object" && !Array.isArray(entry)) {
      const nested = cleanObject(entry as Record<string, unknown>);
      if (Object.keys(nested).length === 0) continue;
      output[key] = nested;
      continue;
    }
    output[key] = entry;
  }
  return output as T;
}

function parseMaybeJSON(value: string): unknown {
  const trimmed = value.trim();
  if (!trimmed) return "";
  try {
    return JSON.parse(trimmed);
  } catch {
    return value;
  }
}

function stringifyConst(value: unknown): string {
  if (typeof value === "string") return value;
  if (value === undefined) return "";
  return JSON.stringify(value);
}

function uiNodeMetadata(ui: Record<string, unknown> | undefined, nodeID: string): Record<string, unknown> {
  const nodes = recordFromUnknown(ui?.nodes);
  return recordFromUnknown(nodes[nodeID]);
}

function uiEdgeMetadata(ui: Record<string, unknown> | undefined, from: string, to: string): Record<string, unknown> {
  const edges = recordFromUnknown(ui?.edges);
  for (const [id, metadata] of Object.entries(edges)) {
    const record = recordFromUnknown(metadata);
    if (record.from === from && record.to === to) {
      return { ...record, id };
    }
  }
  return {};
}

function numberFromRecord(record: unknown, key: string): number | undefined {
  const value = recordFromUnknown(record)[key];
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function durationMillis(value: unknown, fallback: number | undefined): number | undefined {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value !== "string" || !value.trim()) return fallback;
  const normalized = value.trim();
  if (normalized.endsWith("ms")) return Number(normalized.slice(0, -2)) || fallback;
  if (normalized.endsWith("s")) return (Number(normalized.slice(0, -1)) || 0) * 1000 || fallback;
  return Number(normalized) || fallback;
}

function stableResourceId(prefix: string, name: string): string {
  const normalized = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
  return `${prefix}_${normalized || Date.now().toString(16)}`;
}
