import {
  createSchemaField,
  type SchemaFieldType,
  type SchemaFieldDraft
} from "../connectors/domain";
import type { ConnectorOperation, ToolDependencies, ToolSpec } from "../../types/platform";

export type ToolDraft = {
  id: string;
  name: string;
  description: string;
  businessDomain: string;
  ownerTeam: string;
  llmDescription: string;
  implementation: ToolSpec["implementation"];
  connectorOperationId: string;
  knowledgeBaseIds: string;
  pythonPackageId: string;
  mcpServerId: string;
  mcpServerUrl: string;
  mcpTransport: "streamable_http" | "sse" | string;
  mcpHeaders: Record<string, string>;
  mcpToolName: string;
  workflowId: string;
  inputFields: SchemaFieldDraft[];
  outputFields: SchemaFieldDraft[];
  sideEffect: ToolSpec["sideEffect"];
  riskLevel: ToolSpec["riskLevel"];
  requiresConfirmation: boolean;
  timeoutMillis: number;
  retryMaxAttempts: number;
  retryBackoffMillis: number;
  status: ToolSpec["status"];
};

export const emptyToolDependencies = (toolId: string): ToolDependencies => ({
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
});

export function createToolDraft(): ToolDraft {
  return {
    id: "",
    name: "",
    description: "",
    businessDomain: "General",
    ownerTeam: "AI Platform",
    llmDescription: "",
    implementation: "connector",
    connectorOperationId: "",
    knowledgeBaseIds: "",
    pythonPackageId: "",
    mcpServerId: "",
    mcpServerUrl: "",
    mcpTransport: "streamable_http",
    mcpHeaders: {},
    mcpToolName: "",
    workflowId: "",
    inputFields: [createSchemaField({ name: "order_id", required: true, description: "Order identifier." })],
    outputFields: [createSchemaField({ name: "status", description: "Normalized result status." })],
    sideEffect: "read",
    riskLevel: "low",
    requiresConfirmation: false,
    timeoutMillis: 10000,
    retryMaxAttempts: 0,
    retryBackoffMillis: 0,
    status: "disabled"
  };
}

export function createToolDraftForImplementation(implementation: ToolSpec["implementation"]): ToolDraft {
  const draft = createToolDraft();
  draft.implementation = implementation;

  switch (implementation) {
    case "knowledge":
      return {
        ...draft,
        name: "",
        description: "",
        llmDescription: "Use this tool when the answer needs retrieval from managed knowledge bases.",
        inputFields: [createSchemaField({ name: "query", required: true, description: "Natural language retrieval query." })],
        outputFields: [createSchemaField({ name: "documents", type: "array", description: "Ranked retrieval results." })],
        sideEffect: "read",
        riskLevel: "low"
      };
    case "mcp":
      return {
        ...draft,
        llmDescription: "Use this tool when the task should be delegated to a registered MCP server tool.",
        inputFields: [createSchemaField({ name: "arguments", type: "object", required: true, description: "Arguments passed to the MCP tool." })],
        outputFields: [createSchemaField({ name: "result", type: "object", description: "Result returned by the MCP tool." })]
      };
    case "python":
      return {
        ...draft,
        llmDescription: "Use this tool when the task requires a managed code adapter package.",
        inputFields: [createSchemaField({ name: "input", type: "object", required: true, description: "Structured input for the code adapter." })],
        outputFields: [createSchemaField({ name: "output", type: "object", description: "Structured output from the code adapter." })]
      };
    case "workflow":
      return {
        ...draft,
        llmDescription: "Use this tool when the task should trigger a governed platform workflow.",
        inputFields: [createSchemaField({ name: "payload", type: "object", required: true, description: "Workflow trigger payload." })],
        outputFields: [createSchemaField({ name: "workflow_run_id", description: "Workflow run identifier." })],
        sideEffect: "write",
        riskLevel: "medium",
        requiresConfirmation: true
      };
    case "connector":
    default:
      return draft;
  }
}

export function sampleArgsFromFields(fields: SchemaFieldDraft[]): Record<string, unknown> {
  const args: Record<string, unknown> = {};
  for (const field of fields) {
    const name = field.name.trim();
    if (!name) continue;
    args[name] = sampleValueForField(field);
  }
  return args;
}

export function draftFromTool(tool: ToolSpec): ToolDraft {
  return {
    id: tool.id,
    name: tool.name,
    description: tool.description,
    businessDomain: tool.businessDomain ?? "General",
    ownerTeam: tool.ownerTeam ?? "AI Platform",
    llmDescription: tool.llmDescription ?? tool.description,
    implementation: tool.implementation,
    connectorOperationId: tool.binding?.connectorOperationId ?? "",
    knowledgeBaseIds: (tool.binding?.knowledgeBaseIds ?? []).join(", "),
    pythonPackageId: tool.binding?.pythonPackageId ?? "",
    mcpServerId: tool.binding?.mcpServerId ?? "",
    mcpServerUrl: tool.binding?.mcpServerUrl ?? "",
    mcpTransport: tool.binding?.mcpTransport ?? "streamable_http",
    mcpHeaders: tool.binding?.mcpHeaders ?? {},
    mcpToolName: tool.binding?.mcpToolName ?? "",
    workflowId: tool.binding?.workflowId ?? "",
    inputFields: fieldsFromSchema(tool.inputSchema),
    outputFields: fieldsFromSchema(tool.outputSchema),
    sideEffect: tool.sideEffect,
    riskLevel: tool.riskLevel,
    requiresConfirmation: tool.requiresConfirmation,
    timeoutMillis: tool.timeoutMillis,
    retryMaxAttempts: tool.retryPolicy?.maxAttempts ?? 0,
    retryBackoffMillis: tool.retryPolicy?.backoffMillis ?? 0,
    status: tool.status
  };
}

export function toolFromDraft(draft: ToolDraft, tenantId: string): ToolSpec {
  return {
    id: draft.id,
    tenantId,
    name: draft.name.trim(),
    description: draft.description.trim(),
    businessDomain: draft.businessDomain.trim(),
    ownerTeam: draft.ownerTeam.trim(),
    llmDescription: draft.llmDescription.trim(),
    implementation: draft.implementation,
    binding: bindingFromDraft(draft),
    inputSchema: schemaFromFields(draft.inputFields),
    outputSchema: schemaFromFields(draft.outputFields),
    sideEffect: draft.sideEffect,
    riskLevel: draft.riskLevel,
    requiresConfirmation: draft.requiresConfirmation,
    timeoutMillis: draft.timeoutMillis,
    retryPolicy: {
      maxAttempts: draft.retryMaxAttempts,
      backoffMillis: draft.retryBackoffMillis
    },
    status: draft.status,
    version: "v1"
  };
}

export function implementationLabel(kind: ToolSpec["implementation"]): string {
  switch (kind) {
    case "connector":
      return "Connector Operation";
    case "knowledge":
      return "Knowledge Retrieval";
    case "mcp":
      return "MCP Tool";
    case "python":
      return "Code Adapter";
    case "workflow":
      return "Workflow";
    default:
      return kind;
  }
}

export function bindingLabel(tool: ToolSpec, connectors: ConnectorOperation[]): string {
  if (tool.implementation === "connector") {
    const operationID = tool.binding?.connectorOperationId ?? "";
    const operation = connectors.find((item) => item.id === operationID);
    return operation ? operation.name : operationID || "Not bound";
  }
  if (tool.implementation === "knowledge") return (tool.binding?.knowledgeBaseIds ?? []).join(", ") || "Not bound";
  if (tool.implementation === "mcp") return tool.binding?.mcpToolName || "Not bound";
  if (tool.implementation === "python") return tool.binding?.pythonPackageId || "Not bound";
  return tool.binding?.workflowId || "Not bound";
}

function bindingFromDraft(draft: ToolDraft): ToolSpec["binding"] {
  return {
    connectorOperationId: draft.implementation === "connector" ? draft.connectorOperationId.trim() : undefined,
    knowledgeBaseIds:
      draft.implementation === "knowledge"
        ? draft.knowledgeBaseIds
            .split(",")
            .map((item) => item.trim())
            .filter(Boolean)
        : undefined,
    pythonPackageId: draft.implementation === "python" ? draft.pythonPackageId.trim() : undefined,
    mcpServerId: draft.implementation === "mcp" ? draft.mcpServerId.trim() : undefined,
    mcpServerUrl: draft.implementation === "mcp" ? draft.mcpServerUrl.trim() : undefined,
    mcpTransport: draft.implementation === "mcp" ? draft.mcpTransport : undefined,
    mcpHeaders: draft.implementation === "mcp" ? draft.mcpHeaders : undefined,
    mcpToolName: draft.implementation === "mcp" ? draft.mcpToolName.trim() : undefined,
    workflowId: draft.implementation === "workflow" ? draft.workflowId.trim() : undefined
  };
}

export function fieldsFromSchema(schema?: Record<string, unknown>): SchemaFieldDraft[] {
  const flowFields = schema?.["x-flow-fields"];
  if (Array.isArray(flowFields) && flowFields.length > 0) {
    return fieldsFromFlowContextFields(flowFields);
  }

  const properties = objectRecord(schema?.properties);
  if (!properties) return [];
  const requiredFields = new Set(Array.isArray(schema?.required) ? schema.required.map(String) : []);
  return Object.entries(properties).map(([name, definition]) => {
    const fieldDefinition = objectRecord(definition) ?? {};
    const type = inferSchemaFieldType(fieldDefinition);
    const itemSchema = objectRecord(fieldDefinition.items);
    const arrayItemType = type === "array" ? inferArrayItemType(itemSchema) : undefined;
    const childSchema = type === "array" ? itemSchema : fieldDefinition;
    const children =
      type === "object" || (type === "array" && arrayItemType === "object")
        ? fieldsFromSchema(childSchema)
        : undefined;

    return createSchemaField({
      name,
      type,
      arrayItemType,
      required: requiredFields.has(name),
      description: typeof fieldDefinition.description === "string" ? fieldDefinition.description : "",
      children
    });
  });
}

export function schemaFromFields(fields: SchemaFieldDraft[]): Record<string, unknown> {
  const properties: Record<string, unknown> = {};
  const required: string[] = [];
  for (const field of fields) {
    const name = field.name.trim();
    if (!name) continue;
    properties[name] = schemaDefinitionFromField(field);
    if (field.required) required.push(name);
  }
  return { type: "object", required, properties };
}

function fieldsFromFlowContextFields(fields: unknown[]): SchemaFieldDraft[] {
  const root: SchemaFieldDraft[] = [];
  for (const field of fields) {
    if (!field || typeof field !== "object") continue;
    const record = field as Record<string, unknown>;
    const path = typeof record.path === "string" ? record.path.trim() : "";
    if (!path) continue;
    const parts = path
      .split(".")
      .map((part) => part.trim())
      .filter(Boolean);
    if (parts.length === 0) continue;

    let siblings = root;
    parts.forEach((part, index) => {
      const isLeaf = index === parts.length - 1;
      let current = siblings.find((item) => item.name === part);
      if (!current) {
        current = createSchemaField({
          name: part,
          type: isLeaf ? schemaFieldTypeFromUnknown(record.type) : "object",
          required: isLeaf ? Boolean(record.required) : false,
          description: isLeaf && typeof record.description === "string" ? record.description : "",
          children: isLeaf ? undefined : []
        });
        siblings.push(current);
      }
      if (isLeaf) {
        current.type = schemaFieldTypeFromUnknown(record.type);
        current.required = Boolean(record.required);
        current.description = typeof record.description === "string" ? record.description : current.description;
        return;
      }
      current.type = "object";
      current.children = current.children ?? [];
      siblings = current.children;
    });
  }
  return root;
}

function schemaFieldTypeFromUnknown(value: unknown): SchemaFieldType {
  switch (value) {
    case "number":
    case "integer":
    case "boolean":
    case "object":
    case "array":
    case "string":
      return value;
    default:
      return "string";
  }
}

function schemaDefinitionFromField(field: SchemaFieldDraft): Record<string, unknown> {
  const definition: Record<string, unknown> = {
    type: field.type,
    ...(field.description.trim() ? { description: field.description.trim() } : {})
  };
  if (field.type === "object") {
    const childSchema = schemaFromFields(field.children ?? []);
    definition.properties = childSchema.properties;
    definition.required = childSchema.required;
  }
  if (field.type === "array") {
    const itemType = field.arrayItemType ?? ((field.children?.length ?? 0) > 0 ? "object" : "string");
    definition.items =
      itemType === "object"
        ? {
            type: "object",
            ...schemaFromFields(field.children ?? [])
          }
        : { type: itemType };
  }
  return definition;
}

function sampleValueForField(field: SchemaFieldDraft): unknown {
  const normalizedName = field.name.toLowerCase();
  if (field.type === "boolean") return true;
  if (field.type === "integer") return normalizedName.includes("humidity") ? 60 : 1;
  if (field.type === "number") return normalizedName.includes("temperature") ? 26.5 : 1.0;
  if (field.type === "array") {
    if ((field.arrayItemType ?? "string") === "object") {
      return [sampleArgsFromFields(field.children ?? [])];
    }
    return [];
  }
  if (field.type === "object") return sampleArgsFromFields(field.children ?? []);
  if (normalizedName.includes("city")) return "深圳";
  if (normalizedName.includes("order")) return "o_123";
  if (normalizedName.includes("query")) return "示例问题";
  return field.description.trim() || "sample";
}

function inferSchemaFieldType(definition: Record<string, unknown>): SchemaFieldDraft["type"] {
  if (definition.type === "array") return "array";
  if (definition.type === "object" || objectRecord(definition.properties)) return "object";
  return schemaFieldTypeFromValue(definition.type);
}

function inferArrayItemType(itemSchema?: Record<string, unknown>): SchemaFieldDraft["type"] {
  if (!itemSchema) return "string";
  if (itemSchema.type === "object" || objectRecord(itemSchema.properties)) return "object";
  return schemaFieldTypeFromValue(itemSchema.type);
}

function schemaFieldTypeFromValue(value: unknown): SchemaFieldDraft["type"] {
  switch (value) {
    case "number":
    case "integer":
    case "boolean":
    case "object":
    case "array":
      return value;
    case "string":
    default:
      return "string";
  }
}

function objectRecord(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;
}
