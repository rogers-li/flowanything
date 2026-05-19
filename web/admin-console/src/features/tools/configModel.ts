import type { ConnectorConfig, ConnectorOperationConfig, FlowNodeSpec, SchemaField, ToolConfig, ToolType, WorkflowConfig } from "../../platform/configTypes";
import type { Connector, ConnectorOperation, ToolDependencies, ToolExecutionResult, ToolSpec, WorkflowSpec } from "../../types/platform";
import type { SchemaFieldDraft } from "../connectors/domain";
import type { ToolDraft } from "./domain";

export function toolConfigFromDraft(draft: ToolDraft, current?: ToolConfig): ToolConfig {
  const next: ToolConfig = {
    id: draft.id || stableResourceId("tool", draft.name),
    name: draft.name.trim(),
    description: draft.description.trim(),
    version: "v1",
    disabled: draft.status === "disabled",
    labels: [draft.businessDomain.trim()].filter(Boolean),
    owner: {
      team: draft.ownerTeam.trim(),
      email: ""
    },
    metadata: {
      llm_description: draft.llmDescription.trim(),
      side_effect: draft.sideEffect,
      risk_level: draft.riskLevel
    },
    type: toolTypeFromImplementation(draft.implementation),
    input_schema: schemaFieldsFromDraft(draft.inputFields),
    output_schema: schemaFieldsFromDraft(draft.outputFields),
    implementation: {
      kind: draft.implementation,
      ref: implementationRefFromDraft(draft),
      config: implementationConfigFromDraft(draft)
    },
    policy: {
      timeout: `${draft.timeoutMillis}ms`,
      require_review: draft.requiresConfirmation,
      retry_policy: {
        max_attempts: draft.retryMaxAttempts,
        backoff: `${draft.retryBackoffMillis}ms`
      }
    },
    runtime: {
      network: true,
      server_proxy_allowed: true
    }
  };
  if (!current) return next;
  const sameImplementation = current.implementation?.kind === next.implementation.kind;
  return {
    ...current,
    ...next,
    annotations: current.annotations ?? next.annotations,
    metadata: {
      ...(current.metadata ?? {}),
      ...(next.metadata ?? {})
    },
    implementation: {
      ...current.implementation,
      ...next.implementation,
      ref: next.implementation.ref,
      config: {
        ...(sameImplementation ? current.implementation?.config ?? {} : {}),
        ...(next.implementation?.config ?? {})
      }
    },
    runtime: {
      ...(current.runtime ?? {}),
      ...(next.runtime ?? {})
    }
  };
}

export function connectorFromConfig(connector: ConnectorConfig): Connector {
  return {
    id: connector.id,
    tenantId: "tenant_1",
    name: connector.name,
    description: connector.description ?? "",
    businessDomain: connector.labels?.[0] ?? "General",
    ownerTeam: connector.owner?.team ?? "AI Platform",
    type: "http",
    status: connector.disabled ? "disabled" : "enabled",
    baseUrl: connector.protocol?.base_url ?? "",
    headers: {},
    auth: {
      type: connectorAuthType(connector.auth?.type),
      secretRef: connector.auth?.secret_ref
    },
    timeoutMillis: 10000,
    version: connector.version
  };
}

export function connectorOperationFromConfig(connector: ConnectorConfig, operation: ConnectorOperationConfig): ConnectorOperation {
  return {
    id: operation.id,
    tenantId: "tenant_1",
    connectorId: connector.id,
    name: operation.name,
    description: operation.description ?? "",
    businessDomain: operation.labels?.[0] ?? connector.labels?.[0] ?? "General",
    ownerTeam: operation.owner?.team ?? connector.owner?.team ?? "AI Platform",
    type: "http",
    status: operation.disabled || connector.disabled ? "disabled" : "enabled",
    implementationMode: "simple_http",
    method: connectorMethod(operation.request?.method),
    baseUrl: connector.protocol?.base_url ?? "",
    path: operation.request?.path ?? "",
    headers: operation.request?.headers,
    auth: {
      type: connectorAuthType(connector.auth?.type),
      secretRef: connector.auth?.secret_ref
    },
    inputSchema: jsonSchemaFromFields(operation.input_schema),
    outputSchema: jsonSchemaFromFields(operation.output_schema),
    timeoutMillis: durationMillis(operation.policy?.timeout, 10000),
    version: operation.version
  };
}

export function workflowSpecFromConfig(workflow: WorkflowConfig): WorkflowSpec {
  const spec = workflow.spec as Record<string, unknown>;
  const contextSchema = (workflow.spec.context_schema as Record<string, unknown> | undefined) ?? {};
  const flowInputFields = Array.isArray(contextSchema.flow_input) ? (contextSchema.flow_input as SchemaField[]) : ((spec.input_schema as SchemaField[] | undefined) ?? []);
  const flowOutputFields = Array.isArray(contextSchema.flow_output) ? (contextSchema.flow_output as SchemaField[]) : ((spec.output_schema as SchemaField[] | undefined) ?? []);
  const variableFields = Array.isArray(contextSchema.variables) ? (contextSchema.variables as SchemaField[]) : [];
  return {
    id: workflow.id,
    tenantId: "tenant_1",
    name: workflow.name,
    description: workflow.description,
    businessDomain: workflow.labels?.[0] ?? "General",
    ownerTeam: workflow.owner?.team ?? "AI Platform",
    status: workflow.disabled ? "disabled" : "enabled",
    profile: workflow.ui?.profile === "agent_workflow" ? "agent_workflow" : "tool_workflow",
    contextSchema: jsonSchemaFromFields(variableFields),
    inputSchema: jsonSchemaFromFields(flowInputFields),
    outputSchema: jsonSchemaFromFields(flowOutputFields),
    graph: {
      entryNodeId: "start",
      nodes: Object.fromEntries(
        (workflow.spec.nodes ?? []).map((node) => [
          node.id,
          {
            id: node.id,
            type: workflowNodeTypeFromEngine(node.type),
            name: node.name ?? node.id,
            description: nodeDescriptionFromConfig(node.config),
            position: nodePositionFromUI(workflow.ui, node.id),
            config: uiNodeConfigFromEngineNode(node)
          }
        ])
      ),
      edges: (workflow.spec.edges ?? []).map((edge) => ({
        id: edgeIDFromUI(workflow.ui, edge.from, edge.to),
        fromNodeId: edge.from,
        toNodeId: edge.to,
        type: "default"
      }))
    },
    policy: {},
    ui: workflow.ui,
    version: workflow.version || "v1"
  };
}

export function workflowConfigFromSpec(workflow: WorkflowSpec, current?: WorkflowConfig): WorkflowConfig {
  const next: WorkflowConfig = {
    id: workflow.id || stableResourceId("workflow", workflow.name),
    name: workflow.name.trim(),
    description: workflow.description ?? "",
    version: workflow.version || "v1",
    disabled: workflow.status === "disabled",
    labels: [workflow.businessDomain ?? "General"].filter(Boolean),
    owner: {
      team: workflow.ownerTeam ?? "AI Platform",
      email: ""
    },
    spec: {
      id: workflow.id || stableResourceId("workflow", workflow.name),
      name: workflow.name,
      version: workflow.version || "v1",
      context_schema: {
        flow_input: schemaFieldsFromJsonSchema(workflow.inputSchema),
        flow_output: schemaFieldsFromJsonSchema(workflow.outputSchema),
        variables: schemaFieldsFromJsonSchema(workflow.contextSchema),
        node_context: {}
      },
      nodes: Object.values(workflow.graph.nodes).map(
        (node): FlowNodeSpec => ({
          id: node.id,
          type: engineNodeType(node.type),
          name: node.name,
          config: engineNodeConfig(node),
          input_mappings: fieldBindingsFromRecord(node.config?.input_mapping as Record<string, unknown> | undefined),
          output_writes: contextWritesFromRecord((node.config?.write_context ?? node.config?.context_writes) as Record<string, unknown> | undefined, true),
          timeout: node.timeoutMillis ? node.timeoutMillis * 1000000 : undefined,
          retry_policy: node.retryPolicy
            ? {
                max_attempts: node.retryPolicy.maxAttempts,
                backoff: (node.retryPolicy.backoffMillis ?? 0) * 1000000
              }
            : undefined
        })
      ),
      edges: workflow.graph.edges.map((edge) => ({
        from: edge.fromNodeId,
        to: edge.toNodeId
      })),
      policies: {
        max_node_executions: workflow.policy?.maxSteps
      }
    },
    ui: {
      ...(workflow.ui ?? {}),
      profile: workflow.profile,
      entry_node_id: workflow.graph.entryNodeId,
      graph: {
        description: workflow.description ?? "",
        input_schema: workflow.inputSchema,
        output_schema: workflow.outputSchema,
        context_schema: workflow.contextSchema
      },
      nodes: Object.fromEntries(
        Object.values(workflow.graph.nodes).map((node) => [
          node.id,
          {
            description: node.description,
            position: node.position,
            type: node.type
          }
        ])
      ),
      edges: Object.fromEntries(
        workflow.graph.edges.map((edge) => [
          edge.id || `${edge.fromNodeId}-${edge.toNodeId}`,
          {
            from: edge.fromNodeId,
            to: edge.toNodeId,
            type: edge.type ?? "default",
            condition: edge.condition
          }
        ])
      )
    },
    publish: {
      status: workflow.status,
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

export function dependenciesForTool(tool: ToolSpec | undefined, skills: Array<{ id: string; name: string; tools?: Array<{ ref?: { id?: string }; disabled?: boolean }> }>): ToolDependencies {
  if (!tool) {
    return emptyToolDependencies("");
  }
  const directSkills = skills
    .filter((skill) => (skill.tools ?? []).some((binding) => !binding.disabled && binding.ref?.id === tool.id))
    .map((skill) => ({ id: skill.id, name: skill.name }));
  return {
    toolId: tool.id,
    summary: {
      directSkillCount: directSkills.length,
      directAgentCount: 0,
      indirectAgentCount: 0,
      totalConsumerCount: directSkills.length
    },
    directSkills,
    directAgents: [],
    indirectAgents: []
  };
}

export function toolExecutionResultFromRuntime(toolId: string, result: { result?: unknown }): ToolExecutionResult {
  const output = result.result && typeof result.result === "object" ? (result.result as { output?: Record<string, unknown>; error?: { code?: string; message?: string } }) : {};
  return {
    callId: stableResourceId("toolcall", toolId),
    toolId,
    success: !output.error,
    data: output.output,
    errorCode: output.error?.code,
    errorReason: output.error?.message,
    startedAt: new Date().toISOString(),
    finishedAt: new Date().toISOString()
  };
}

function emptyToolDependencies(toolId: string): ToolDependencies {
  return {
    toolId,
    summary: {
      directSkillCount: 0,
      directAgentCount: 0,
      indirectAgentCount: 0,
      totalConsumerCount: 0
    },
    directSkills: [],
    directAgents: [],
    indirectAgents: []
  };
}

function toolTypeFromImplementation(implementation: ToolSpec["implementation"]): ToolType {
  if (implementation === "connector") return "connector";
  if (implementation === "workflow") return "workflow";
  if (implementation === "mcp") return "mcp";
  if (implementation === "python") return "script";
  return "native";
}

function implementationRefFromDraft(draft: ToolDraft) {
  if (draft.implementation === "connector") return { kind: "connector_operation" as const, id: draft.connectorOperationId.trim() };
  if (draft.implementation === "workflow") return { kind: "workflow" as const, id: draft.workflowId.trim() };
  return undefined;
}

function implementationConfigFromDraft(draft: ToolDraft) {
  if (draft.implementation === "mcp") {
    return {
      mcp_server_id: draft.mcpServerId.trim(),
      mcp_server_url: draft.mcpServerUrl.trim(),
      mcp_transport: draft.mcpTransport,
      mcp_headers: draft.mcpHeaders,
      mcp_tool_name: draft.mcpToolName.trim()
    };
  }
  if (draft.implementation === "python") {
    return {
      python_package_id: draft.pythonPackageId.trim()
    };
  }
  if (draft.implementation === "knowledge") {
    return {
      knowledge_base_ids: draft.knowledgeBaseIds
        .split(",")
        .map((item) => item.trim())
        .filter(Boolean)
    };
  }
  return {};
}

function schemaFieldsFromDraft(fields: SchemaFieldDraft[], parentPath = ""): SchemaField[] {
	return fields
		.filter((field) => field.name.trim())
		.map((field) => {
			return {
				name: field.name.trim(),
				type: field.type,
				description: field.description.trim(),
				required: field.required,
				children: schemaFieldsFromDraft(field.children ?? [], parentPath ? `${parentPath}.${field.name.trim()}` : field.name.trim())
			};
		});
}

function jsonSchemaFromFields(fields?: SchemaField[]): Record<string, unknown> {
  const properties: Record<string, unknown> = {};
  const required: string[] = [];
  for (const field of fields ?? []) {
    properties[field.name] = {
      type: field.type || "string",
      description: field.description,
      properties: field.children?.length ? (jsonSchemaFromFields(field.children).properties as Record<string, unknown>) : undefined
    };
    if (field.required) required.push(field.name);
  }
  return { type: "object", properties, required };
}

function schemaFieldsFromJsonSchema(schema?: Record<string, unknown>): SchemaField[] {
	const explicitFields = schema?.["x-flow-fields"];
	if (Array.isArray(explicitFields)) {
		const flatFields = explicitFields
			.filter((field): field is Record<string, unknown> => Boolean(field) && typeof field === "object" && !Array.isArray(field))
			.filter((field) => typeof field.path === "string" || typeof field.name === "string")
			.map((field) => ({
				path: String(field.path ?? field.name ?? ""),
				type: typeof field.type === "string" ? field.type : "string",
				description: typeof field.description === "string" ? field.description : "",
				required: Boolean(field.required)
			}));
		return schemaFieldsFromFlatPaths(flatFields);
	}
	const properties = schema?.properties;
	if (!properties || typeof properties !== "object" || Array.isArray(properties)) return [];
	const required = new Set(Array.isArray(schema?.required) ? schema.required.map(String) : []);
	return Object.entries(properties as Record<string, unknown>).map(([name, value]) => {
    const definition = value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
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

function connectorMethod(method?: string): ConnectorOperation["method"] {
  if (method === "GET" || method === "POST" || method === "PUT" || method === "PATCH" || method === "DELETE") return method;
  return "GET";
}

function connectorAuthType(value?: string): NonNullable<Connector["auth"]>["type"] {
  if (value === "api_key" || value === "bearer" || value === "basic" || value === "oauth2") return value;
  return "none";
}

function workflowNodeTypeFromEngine(value: string): WorkflowSpec["graph"]["nodes"][string]["type"] {
  const mapped: Record<string, WorkflowSpec["graph"]["nodes"][string]["type"]> = {
    "control.start": "start",
    "control.end": "end",
    "control.condition": "condition",
    "control.join": "join",
    "workflow.transform": "transform",
    "workflow.connector": "connector_operation",
    "workflow.tool": "tool",
    "workflow.agent": "agent"
  };
  return workflowNodeType(mapped[value] ?? value);
}

function workflowNodeType(value: string): WorkflowSpec["graph"]["nodes"][string]["type"] {
  if (value === "end" || value === "join" || value === "transform" || value === "condition" || value === "connector_operation" || value === "tool" || value === "skill" || value === "agent") {
    return value;
  }
  return "start";
}

function engineNodeType(value: WorkflowSpec["graph"]["nodes"][string]["type"]): string {
  switch (value) {
    case "start":
      return "control.start";
    case "end":
      return "control.end";
    case "condition":
      return "control.condition";
    case "join":
      return "control.join";
    case "transform":
      return "workflow.transform";
    case "connector_operation":
      return "workflow.connector";
    case "tool":
      return "workflow.tool";
    case "agent":
      return "workflow.agent";
    default:
      return value;
  }
}

function engineNodeConfig(node: WorkflowSpec["graph"]["nodes"][string]): Record<string, unknown> {
  const config = node.config ?? {};
  if (node.type === "connector_operation") {
    return cleanObject({
      operation_id: stringConfig(config, "connector_operation_id") || stringConfig(config, "operation_id"),
      metadata: cleanObject({ response_alias: stringConfig(config, "response_alias") })
    });
  }
  if (node.type === "tool") {
    return cleanObject({
      tool_id: stringConfig(config, "tool_id"),
      metadata: cleanObject({ response_alias: stringConfig(config, "response_alias") })
    });
  }
  if (node.type === "transform") {
    return cleanObject({
      function: stringConfig(config, "function_id") || stringConfig(config, "function") || "identity",
      args: transformArgsFromConfig(config)
    });
  }
  if (node.type === "condition") {
    return engineConditionConfig(config);
  }
  const { input_mapping: _inputMapping, output_mapping: _outputMapping, write_context: _writeContext, context_writes: _contextWrites, ...rest } = config;
  return cleanObject(rest);
}

function uiNodeConfigFromEngineNode(node: FlowNodeSpec): Record<string, unknown> {
  const config = { ...(node.config ?? {}) };
  if (node.type === "workflow.connector") {
    config.connector_operation_id = stringConfig(config, "operation_id");
  }
  if (node.type === "workflow.transform") {
    config.function_id = stringConfig(config, "function");
    if (config.args && typeof config.args === "object" && !Array.isArray(config.args)) {
      config.input_mapping = {
        ...(config.input_mapping as Record<string, unknown> | undefined),
        ...(config.args as Record<string, unknown>)
      };
    }
  }
  return cleanObject({
    ...config,
    input_mapping: recordFromFieldBindings(node.input_mappings),
    write_context: recordFromContextWrites(node.output_writes)
  });
}

function nodeDescriptionFromConfig(config: Record<string, unknown> | undefined): string | undefined {
  const metadata = config?.metadata;
  if (metadata && typeof metadata === "object" && !Array.isArray(metadata) && typeof (metadata as Record<string, unknown>).description === "string") {
    return (metadata as Record<string, string>).description;
  }
  return undefined;
}

function nodePositionFromUI(ui: Record<string, unknown> | undefined, nodeID: string): { x: number; y: number } | undefined {
  const nodes = ui?.nodes;
  const metadata = nodes && typeof nodes === "object" && !Array.isArray(nodes) ? (nodes as Record<string, unknown>)[nodeID] : undefined;
  if (!metadata || typeof metadata !== "object" || Array.isArray(metadata)) return undefined;
  const position = (metadata as Record<string, unknown>).position;
  if (!position || typeof position !== "object" || Array.isArray(position)) return undefined;
  const x = (position as Record<string, unknown>).x;
  const y = (position as Record<string, unknown>).y;
  return typeof x === "number" && typeof y === "number" ? { x, y } : undefined;
}

function edgeIDFromUI(ui: Record<string, unknown> | undefined, from: string, to: string): string | undefined {
  const edges = ui?.edges;
  if (!edges || typeof edges !== "object" || Array.isArray(edges)) return undefined;
  for (const [id, metadata] of Object.entries(edges as Record<string, unknown>)) {
    if (!metadata || typeof metadata !== "object" || Array.isArray(metadata)) continue;
    const record = metadata as Record<string, unknown>;
    if (record.from === from && record.to === to) return id;
  }
  return `${from}-${to}`;
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

function expressionFromValueSource(source: { type?: string; path?: string; value?: unknown }): unknown {
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

function transformArgsFromConfig(config: Record<string, unknown>): Record<string, unknown> {
  const args = config.args && typeof config.args === "object" && !Array.isArray(config.args) ? { ...(config.args as Record<string, unknown>) } : {};
  const inputMapping = config.input_mapping && typeof config.input_mapping === "object" && !Array.isArray(config.input_mapping) ? (config.input_mapping as Record<string, unknown>) : {};
  for (const key of ["fields", "separator", "target_type"]) {
    if (args[key] !== undefined || inputMapping[key] === undefined) continue;
    args[key] = valueSourceFromExpression(inputMapping[key], false).value ?? parseMaybeJSON(String(inputMapping[key] ?? ""));
  }
  return args;
}

function engineConditionConfig(config: Record<string, unknown>): Record<string, unknown> {
  const branches = Array.isArray(config.branches) ? config.branches : [];
  const defaultBranch = config.default_branch && typeof config.default_branch === "object" && !Array.isArray(config.default_branch) ? (config.default_branch as Record<string, unknown>) : {};
  return cleanObject({
    branches: branches.map((branch) => engineConditionBranch(branch)),
    default_next_node_ids: stringConfig(defaultBranch, "next_node_id") ? [stringConfig(defaultBranch, "next_node_id")] : [],
    default_writes: contextWritesFromRecord((defaultBranch.write_context ?? defaultBranch.context_writes) as Record<string, unknown> | undefined, false)
  });
}

function engineConditionBranch(value: unknown): Record<string, unknown> {
  const branch = value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
  const nextNodeID = stringConfig(branch, "next_node_id");
  return cleanObject({
    name: stringConfig(branch, "name") || "Branch",
    mode: branch.mode === "any" ? "any" : "all",
    rules: Array.isArray(branch.rules) ? branch.rules.map(engineConditionRule) : [],
    next_node_ids: nextNodeID ? [nextNodeID] : [],
    context_writes: contextWritesFromRecord((branch.write_context ?? branch.context_writes) as Record<string, unknown> | undefined, false)
  });
}

function engineConditionRule(value: unknown): Record<string, unknown> {
  const rule = value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
  const { operator, negate } = engineConditionOperator(typeof rule.operator === "string" ? rule.operator : "equals");
  return cleanObject({
    left: valueSourceFromExpression(rule.left, false),
    operator,
    right: valueSourceFromExpression(rule.right, false),
    negate
  });
}

function engineConditionOperator(value: string): { operator: string; negate?: boolean } {
  switch (value) {
    case "equals":
      return { operator: "eq" };
    case "not_equals":
      return { operator: "neq" };
    case "greater_than":
      return { operator: "gt" };
    case "greater_or_equals":
      return { operator: "gte" };
    case "less_than":
      return { operator: "lt" };
    case "less_or_equals":
      return { operator: "lte" };
    case "is_empty":
      return { operator: "empty" };
    case "is_not_empty":
      return { operator: "not_empty" };
    case "not_contains":
      return { operator: "contains", negate: true };
    case "not_in":
      return { operator: "in", negate: true };
    default:
      return { operator: value };
  }
}

function stringConfig(config: Record<string, unknown> | undefined, key: string): string {
  const value = config?.[key];
  return typeof value === "string" ? value : "";
}

function parseMaybeJSON(value: string): unknown {
  try {
    return JSON.parse(value);
  } catch {
    return value;
  }
}

function stringifyConst(value: unknown): unknown {
  if (typeof value === "string") return value;
  if (value === undefined) return "";
  return JSON.stringify(value);
}

function cleanObject<T extends Record<string, unknown>>(value: T): T {
  return Object.fromEntries(Object.entries(value).filter(([, entry]) => entry !== undefined && entry !== "")) as T;
}

function durationMillis(value: unknown, fallback: number): number {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value !== "string" || !value.trim()) return fallback;
  const normalized = value.trim();
  if (normalized.endsWith("ms")) return Number(normalized.slice(0, -2)) || fallback;
  if (normalized.endsWith("s")) return (Number(normalized.slice(0, -1)) || fallback / 1000) * 1000;
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
