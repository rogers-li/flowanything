import type { WorkflowNode, WorkflowNodeType, WorkflowProfile, WorkflowSpec } from "../../types/platform";

export type ContextFieldDraft = {
  id: string;
  path: string;
  type: string;
  description: string;
  required: boolean;
};

export type MappingRowDraft = {
  id: string;
  target: string;
  source: string;
};

export type WorkflowDraft = {
  id: string;
  name: string;
  description: string;
  businessDomain: string;
  ownerTeam: string;
  status: WorkflowSpec["status"];
  profile: WorkflowProfile;
  contextFields: ContextFieldDraft[];
  inputSchema: Record<string, unknown>;
  outputSchema: Record<string, unknown>;
  nodes: WorkflowNode[];
  edges: WorkflowSpec["graph"]["edges"];
  selectedNodeId: string;
  maxSteps: number;
  maxParallelism: number;
  timeoutMillis: number;
  version: string;
};

export type NodeConfigRows = {
  inputRows: MappingRowDraft[];
  outputRows: MappingRowDraft[];
  contextRows: MappingRowDraft[];
};

export const workflowNodeTypes: WorkflowNodeType[] = [
  "start",
  "connector_operation",
  "tool",
  "skill",
  "agent",
  "transform",
  "condition",
  "join",
  "end"
];

export function createWorkflowDraft(profile: WorkflowProfile = "tool_workflow"): WorkflowDraft {
  const startNode: WorkflowNode = {
    id: "start",
    type: "start",
    name: "Start",
    description: "Workflow entry",
    position: { x: 80, y: 120 },
    config: {}
  };
  return {
    id: "",
    name: "",
    description: "",
    businessDomain: "General",
    ownerTeam: "AI Platform",
    status: "draft",
    profile,
    contextFields: [
      createContextField({
        path: "request.query",
        type: "string",
        description: "User or caller request text",
        required: true
      })
    ],
    inputSchema: {},
    outputSchema: {},
    nodes: [startNode],
    edges: [],
    selectedNodeId: startNode.id,
    maxSteps: 32,
    maxParallelism: 4,
    timeoutMillis: 60000,
    version: "v1"
  };
}

export function draftFromWorkflow(workflow: WorkflowSpec): WorkflowDraft {
  const nodes = Object.values(workflow.graph.nodes);
  return {
    id: workflow.id,
    name: workflow.name,
    description: workflow.description ?? "",
    businessDomain: workflow.businessDomain ?? "General",
    ownerTeam: workflow.ownerTeam ?? "AI Platform",
    status: workflow.status,
    profile: workflow.profile,
    contextFields: contextFieldsFromSchema(workflow.contextSchema),
    inputSchema: workflow.inputSchema ?? {},
    outputSchema: workflow.outputSchema ?? {},
    nodes,
    edges: workflow.graph.edges,
    selectedNodeId: workflow.graph.entryNodeId || nodes[0]?.id || "start",
    maxSteps: workflow.policy?.maxSteps ?? 32,
    maxParallelism: workflow.policy?.maxParallelism ?? 4,
    timeoutMillis: workflow.policy?.timeoutMillis ?? 60000,
    version: workflow.version
  };
}

export function workflowFromDraft(draft: WorkflowDraft, tenantId: string): WorkflowSpec {
  const nodes = Object.fromEntries(draft.nodes.map((node) => [node.id, normalizeNode(node)]));
  const entryNodeId = nodes[draft.selectedNodeId]?.id === "start" ? draft.selectedNodeId : "start";
  return {
    id: draft.id,
    tenantId,
    name: draft.name.trim(),
    description: draft.description.trim(),
    businessDomain: draft.businessDomain.trim(),
    ownerTeam: draft.ownerTeam.trim(),
    status: draft.status,
    profile: draft.profile,
    contextSchema: contextSchemaFromFields(draft.contextFields),
    inputSchema: draft.inputSchema,
    outputSchema: draft.outputSchema,
    graph: {
      entryNodeId,
      nodes,
      edges: draft.edges.filter((edge) => nodes[edge.fromNodeId] && nodes[edge.toNodeId])
    },
    policy: {
      maxSteps: draft.maxSteps,
      maxParallelism: draft.maxParallelism,
      timeoutMillis: draft.timeoutMillis
    },
    ui: {},
    version: draft.version || "v1"
  };
}

export function createWorkflowNode(type: WorkflowNodeType, index: number): WorkflowNode {
  const id = `${type}_${randomSuffix()}`;
  return {
    id,
    type,
    name: defaultNodeName(type, index),
    description: defaultNodeDescription(type),
    position: { x: 160 + index * 180, y: 160 },
    config: defaultNodeConfig(type),
    timeoutMillis: type === "start" || type === "end" ? undefined : 30000,
    retryPolicy: { maxAttempts: 0, backoffMillis: 0 }
  };
}

export function createContextField(seed: Partial<ContextFieldDraft> = {}): ContextFieldDraft {
  return {
    id: seed.id ?? `ctx_${randomSuffix()}`,
    path: seed.path ?? "",
    type: seed.type ?? "string",
    description: seed.description ?? "",
    required: seed.required ?? false
  };
}

export function createMappingRow(seed: Partial<MappingRowDraft> = {}): MappingRowDraft {
  return {
    id: seed.id ?? `map_${randomSuffix()}`,
    target: seed.target ?? "",
    source: seed.source ?? ""
  };
}

export function nodeConfigRows(node: WorkflowNode | undefined): NodeConfigRows {
  const config = node?.config ?? {};
  return {
    inputRows: mappingRowsFromRecord(config.input_mapping as Record<string, unknown> | undefined),
    outputRows: mappingRowsFromRecord(config.output_mapping as Record<string, unknown> | undefined),
    contextRows: mappingRowsFromRecord((config.write_context ?? config.context_writes) as Record<string, unknown> | undefined)
  };
}

export function applyNodeConfigRows(node: WorkflowNode, rows: NodeConfigRows): WorkflowNode {
  return {
    ...node,
    config: {
      ...(node.config ?? {}),
      input_mapping: recordFromMappingRows(rows.inputRows),
      output_mapping: recordFromMappingRows(rows.outputRows),
      write_context: recordFromMappingRows(rows.contextRows)
    }
  };
}

export function mappingRowsFromRecord(record: Record<string, unknown> | undefined): MappingRowDraft[] {
  return Object.entries(record ?? {}).map(([target, source], index) =>
    createMappingRow({
      // Keep row identity stable while editing target text; otherwise React
      // remounts the input on every keystroke and drops focus.
      id: `map_row_${index}`,
      target,
      source: typeof source === "string" ? source : JSON.stringify(source)
    })
  );
}

export function recordFromMappingRows(rows: MappingRowDraft[]): Record<string, unknown> {
  return rows.reduce<Record<string, unknown>>((acc, row) => {
    const target = row.target.trim();
    if (!target) return acc;
    acc[target] = parseMaybeJSON(row.source.trim());
    return acc;
  }, {});
}

export function contextFieldsFromSchema(schema: Record<string, unknown> | undefined): ContextFieldDraft[] {
  const rawFields = schema?.["x-flow-fields"];
  if (Array.isArray(rawFields)) {
    const fields = rawFields
      .map((field) => fieldFromUnknown(field))
      .filter((field): field is ContextFieldDraft => Boolean(field));
    if (fields.length > 0) return fields;
  }

  const properties = schema?.properties;
  if (properties && typeof properties === "object" && !Array.isArray(properties)) {
    return contextFieldsFromProperties(schema);
  }

  return [];
}

function contextFieldsFromProperties(schema: Record<string, unknown>, prefix = ""): ContextFieldDraft[] {
  const properties = schema.properties;
  if (!properties || typeof properties !== "object" || Array.isArray(properties)) return [];
  const requiredFields = new Set(Array.isArray(schema.required) ? schema.required.map(String) : []);

  return Object.entries(properties as Record<string, Record<string, unknown>>).flatMap(([name, value]) => {
    const path = prefix ? `${prefix}.${name}` : name;
    const type = contextFieldType(value);
    const field = createContextField({
      path,
      type,
      description: typeof value.description === "string" ? value.description : "",
      required: requiredFields.has(name)
    });
    const nestedSchema = type === "array" && value.items && typeof value.items === "object" && !Array.isArray(value.items)
      ? (value.items as Record<string, unknown>)
      : value;
    const children = type === "object" || (type === "array" && contextFieldType(nestedSchema) === "object") ? contextFieldsFromProperties(nestedSchema, path) : [];
    return children.length > 0 ? [field, ...children] : [field];
  });
}

function contextFieldType(value: Record<string, unknown>): string {
  if (typeof value.type === "string") return value.type;
  if (value.properties && typeof value.properties === "object" && !Array.isArray(value.properties)) return "object";
  return "string";
}

export function contextSchemaFromFields(fields: ContextFieldDraft[]): Record<string, unknown> {
  const cleanFields = fields
    .map((field) => ({
      path: field.path.trim(),
      type: field.type.trim() || "string",
      description: field.description.trim(),
      required: field.required
    }))
    .filter((field) => field.path);
  const { properties, required } = schemaPropertiesFromContextFields(cleanFields);
  return {
    type: "object",
    description: "Workflow shared business context. Nodes should read and write these named fields rather than depending on node IDs.",
    properties,
    required,
    "x-flow-fields": cleanFields
  };
}

function schemaPropertiesFromContextFields(fields: Array<{ path: string; type: string; description: string; required: boolean }>) {
  const properties: Record<string, Record<string, unknown>> = {};
  const required: string[] = [];

  const fieldsByDepth = [...fields].sort((left, right) => pathDepth(left.path) - pathDepth(right.path));

  for (const field of fieldsByDepth) {
    const parts = field.path
      .split(".")
      .map((part) => part.trim())
      .filter(Boolean);
    if (parts.length === 0) continue;
    let currentProperties = properties;
    let currentRequired = required;

    parts.forEach((part, index) => {
      const isLeaf = index === parts.length - 1;
      if (isLeaf) {
        currentProperties[part] = {
          type: field.type || "string",
          ...(field.description ? { description: field.description } : {})
        };
        if (field.required && !currentRequired.includes(part)) currentRequired.push(part);
        return;
      }

      const objectSchema = ensureObjectSchema(currentProperties[part]);
      currentProperties[part] = objectSchema;
      currentProperties = objectSchema.properties as Record<string, Record<string, unknown>>;
      currentRequired = objectSchema.required as string[];
    });
  }

  return { properties, required };
}

function pathDepth(path: string): number {
  return path.split(".").filter((part) => part.trim()).length;
}

function ensureObjectSchema(value: unknown): Record<string, unknown> {
  const base = value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
  const nestedProperties =
    base.properties && typeof base.properties === "object" && !Array.isArray(base.properties)
      ? (base.properties as Record<string, Record<string, unknown>>)
      : {};
  const nestedRequired = Array.isArray(base.required) ? base.required.filter((item): item is string => typeof item === "string") : [];
  return {
    ...base,
    type: "object",
    properties: nestedProperties,
    required: nestedRequired
  };
}

export function nodeTypeLabel(type: WorkflowNodeType): string {
  return type
    .split("_")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function normalizeNode(node: WorkflowNode): WorkflowNode {
  return {
    ...node,
    name: node.name.trim() || nodeTypeLabel(node.type),
    description: node.description?.trim(),
    config: node.config ?? {}
  };
}

function defaultNodeName(type: WorkflowNodeType, index: number): string {
  if (type === "start") return "Start";
  if (type === "end") return "End";
  return `${nodeTypeLabel(type)} ${index}`;
}

function defaultNodeDescription(type: WorkflowNodeType): string {
  switch (type) {
    case "connector_operation":
      return "Call one connector operation and write selected output to context.";
    case "tool":
      return "Call one platform tool with mapped input.";
    case "skill":
      return "Invoke a reusable skill with mapped context.";
    case "agent":
      return "Invoke an agent as a workflow node.";
    case "transform":
      return "Transform input fields and publish clean context.";
    case "condition":
      return "Route execution based on context values.";
    case "join":
      return "Wait for upstream branches before continuing.";
    case "end":
      return "Finish the workflow.";
    default:
      return "Workflow entry.";
  }
}

function defaultNodeConfig(type: WorkflowNodeType): Record<string, unknown> {
  switch (type) {
    case "connector_operation":
      return { connector_operation_id: "", input_mapping: {}, output_mapping: {}, write_context: {} };
    case "tool":
      return { tool_id: "", input_mapping: {}, output_mapping: {}, write_context: {} };
    case "skill":
      return { skill_id: "", task: "", input_mapping: {}, output_mapping: {}, write_context: {} };
    case "agent":
      return { agent_id: "", task: "", input_mapping: {}, output_mapping: {}, write_context: {} };
    case "condition":
      return { branches: [], default_branch: { write_context: {}, next_node_id: "" }, input_mapping: {}, output_mapping: {}, write_context: {} };
    case "transform":
      return { function_id: "json.remove_fields", function_version: "1.0.0", input_mapping: {}, output_mapping: {}, write_context: {} };
    default:
      return {};
  }
}

function fieldFromUnknown(value: unknown): ContextFieldDraft | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const record = value as Record<string, unknown>;
  return createContextField({
    path: typeof record.path === "string" ? record.path : "",
    type: typeof record.type === "string" ? record.type : "string",
    description: typeof record.description === "string" ? record.description : "",
    required: record.required === true
  });
}

function parseMaybeJSON(value: string): unknown {
  if (!value) return "";
  if (!value.startsWith("{") && !value.startsWith("[") && value !== "true" && value !== "false" && value !== "null") {
    const numberValue = Number(value);
    return Number.isFinite(numberValue) && value.trim() !== "" && !value.startsWith("0") ? numberValue : value;
  }
  try {
    return JSON.parse(value) as unknown;
  } catch {
    return value;
  }
}

function randomSuffix(): string {
  return Math.random().toString(16).slice(2, 10);
}
