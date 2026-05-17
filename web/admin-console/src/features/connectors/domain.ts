import type { Connector, ConnectorDependencies, ConnectorOperation } from "../../types/platform";

export type SchemaFieldType = "string" | "number" | "integer" | "boolean" | "object" | "array";

export type SchemaFieldDraft = {
  id: string;
  name: string;
  type: SchemaFieldType;
  arrayItemType?: SchemaFieldType;
  required: boolean;
  description: string;
  children?: SchemaFieldDraft[];
};

export type HeaderDraft = {
  id: string;
  name: string;
  value: string;
};

export type ConnectorDraft = {
  id: string;
  name: string;
  description: string;
  businessDomain: string;
  ownerTeam: string;
  type: Connector["type"];
  status: Connector["status"];
  baseUrl: string;
  timeoutMillis: number;
  authType: NonNullable<Connector["auth"]>["type"];
  secretRef: string;
  headers: HeaderDraft[];
};

export type ConnectorOperationDraft = {
  id: string;
  connectorId: string;
  name: string;
  description: string;
  businessDomain: string;
  ownerTeam: string;
  implementationMode: NonNullable<ConnectorOperation["implementationMode"]>;
  method: ConnectorOperation["method"];
  baseUrl: string;
  path: string;
  timeoutMillis: number;
  status: ConnectorOperation["status"];
  authType: NonNullable<ConnectorOperation["auth"]>["type"];
  secretRef: string;
  headers: HeaderDraft[];
  inputFields: SchemaFieldDraft[];
  outputFields: SchemaFieldDraft[];
};

export const emptyDependencyReport = (operationId: string): ConnectorDependencies => ({
  operationId,
  summary: {
    directToolCount: 0,
    indirectSkillCount: 0,
    indirectAgentCount: 0,
    totalConsumerCount: 0,
    blockingToolCount: 0
  },
  directTools: [],
  indirectSkills: [],
  indirectAgents: []
});

export function createConnectorConfigDraft(): ConnectorDraft {
  return {
    id: "",
    name: "",
    description: "",
    businessDomain: "General",
    ownerTeam: "Integration Team",
    type: "http",
    status: "draft",
    baseUrl: "http://localhost:8090",
    timeoutMillis: 10000,
    authType: "none",
    secretRef: "",
    headers: []
  };
}

export function draftFromConnector(connector: Connector): ConnectorDraft {
  return {
    id: connector.id,
    name: connector.name,
    description: connector.description,
    businessDomain: connector.businessDomain ?? "General",
    ownerTeam: connector.ownerTeam ?? "Integration Team",
    type: connector.type,
    status: connector.status,
    baseUrl: connector.baseUrl,
    timeoutMillis: connector.timeoutMillis,
    authType: connector.auth?.type ?? "none",
    secretRef: connector.auth?.secretRef ?? "",
    headers: headersFromRecord(connector.headers ?? {})
  };
}

export function connectorFromDraft(draft: ConnectorDraft, tenantId: string): Connector {
  const secretRef = connectorSecretRefForDraft(draft);
  return {
    id: draft.id,
    tenantId,
    name: draft.name.trim(),
    description: draft.description.trim(),
    businessDomain: draft.businessDomain.trim(),
    ownerTeam: draft.ownerTeam.trim(),
    type: draft.type,
    status: draft.status,
    baseUrl: draft.baseUrl.trim(),
    headers: headersToRecord(draft.headers),
    auth: {
      type: draft.authType,
      secretRef: secretRef || undefined
    },
    timeoutMillis: draft.timeoutMillis
  };
}

export function createConnectorDraft(connector?: Connector): ConnectorOperationDraft {
  return {
    id: "",
    connectorId: connector?.id ?? "",
    name: "",
    description: "",
    businessDomain: connector?.businessDomain ?? "General",
    ownerTeam: connector?.ownerTeam ?? "Integration Team",
    implementationMode: "simple_http",
    method: "GET",
    baseUrl: connector?.baseUrl ?? "http://localhost:8090",
    path: "/orders/{order_id}",
    timeoutMillis: connector?.timeoutMillis ?? 10000,
    status: "draft",
    authType: connector?.auth?.type ?? "none",
    secretRef: connector?.auth?.secretRef ?? "",
    headers: [],
    inputFields: [createSchemaField({ name: "order_id", required: true, description: "Order identifier." })],
    outputFields: [
      createSchemaField({ name: "order_id", required: true, description: "Order identifier." }),
      createSchemaField({ name: "status", required: false, description: "Normalized order status." })
    ]
  };
}

export function draftFromOperation(operation: ConnectorOperation): ConnectorOperationDraft {
  return {
    id: operation.id,
    connectorId: operation.connectorId ?? "",
    name: operation.name,
    description: operation.description,
    businessDomain: operation.businessDomain ?? inferBusinessDomain(operation),
    ownerTeam: operation.ownerTeam ?? "Integration Team",
    implementationMode: operation.implementationMode ?? "simple_http",
    method: operation.method,
    baseUrl: operation.baseUrl,
    path: operation.path,
    timeoutMillis: operation.timeoutMillis,
    status: operation.status,
    authType: operation.auth?.type ?? "none",
    secretRef: operation.auth?.secretRef ?? "",
    headers: headersFromRecord(operation.headers ?? {}),
    inputFields: fieldsFromSchema(operation.inputSchema),
    outputFields: fieldsFromSchema(operation.outputSchema)
  };
}

export function operationFromDraft(draft: ConnectorOperationDraft, tenantId: string): ConnectorOperation {
  return {
    id: draft.id,
    tenantId,
    connectorId: draft.connectorId || undefined,
    name: draft.name.trim(),
    description: draft.description.trim(),
    businessDomain: draft.businessDomain.trim(),
    ownerTeam: draft.ownerTeam.trim(),
    type: "http",
    status: draft.status,
    implementationMode: draft.implementationMode,
    method: draft.method,
    baseUrl: draft.baseUrl.trim(),
    path: draft.path.trim(),
    headers: headersToRecord(draft.headers),
    auth: {
      type: draft.authType,
      secretRef: draft.secretRef.trim() || undefined
    },
    inputSchema: schemaFromFields(draft.inputFields),
    outputSchema: schemaFromFields(draft.outputFields),
    timeoutMillis: draft.timeoutMillis
  };
}

export function connectorEndpoint(operation: ConnectorOperation): string {
  return `${operation.method} ${operation.baseUrl}${operation.path}`;
}

export function implementationModeLabel(mode?: ConnectorOperation["implementationMode"]): string {
  switch (mode) {
    case "adapter_service":
      return "Adapter Service";
    case "template_mapping":
      return "Template Mapping";
    case "workflow":
      return "Workflow";
    case "mock":
      return "Mock";
    case "simple_http":
    default:
      return "Simple HTTP";
  }
}

export function contractSummary(operation: ConnectorOperation): string {
  const inputCount = schemaPropertyCount(operation.inputSchema);
  const outputCount = schemaPropertyCount(operation.outputSchema);
  return `${inputCount} input / ${outputCount} output fields`;
}

export function inferBusinessDomain(operation: ConnectorOperation): string {
  const name = operation.name.toLowerCase();
  if (name.includes("order")) return "Order";
  if (name.includes("refund")) return "Refund";
  if (name.includes("help") || name.includes("policy")) return "Support";
  return "General";
}

export function canDisableOperation(dependencies: ConnectorDependencies): boolean {
  return dependencies.summary.directToolCount === 0;
}

export function connectorAuthRequiresEnvironmentSecret(authType: NonNullable<Connector["auth"]>["type"]): boolean {
  return authType === "api_key" || authType === "bearer" || authType === "basic";
}

export function connectorSecretRefForDraft(draft: ConnectorDraft): string {
  if (!connectorAuthRequiresEnvironmentSecret(draft.authType)) {
    return "";
  }
  return safeEnvSecretRef(draft.secretRef) || `env:${connectorSecretEnvName(draft)}`;
}

export function connectorSecretEnvName(draft: ConnectorDraft): string {
  const source = draft.id || draft.name || "CONNECTOR";
  const withoutPrefix = source.replace(/^conn(?:ector)?[_-]?/i, "");
  const snake = withoutPrefix
    .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
    .replace(/[^a-zA-Z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "")
    .toUpperCase();
  const name = snake || "CONNECTOR";
  return name.endsWith("_API_KEY") ? name : `${name}_API_KEY`;
}

export function createSchemaField(partial: Partial<SchemaFieldDraft> = {}): SchemaFieldDraft {
  return {
    id: createDraftID("schema"),
    name: partial.name ?? "",
    type: partial.type ?? "string",
    arrayItemType: partial.arrayItemType,
    required: partial.required ?? false,
    description: partial.description ?? "",
    children: partial.children
  };
}

export function createHeader(partial: Partial<HeaderDraft> = {}): HeaderDraft {
  return {
    id: createDraftID("header"),
    name: partial.name ?? "",
    value: partial.value ?? ""
  };
}

function safeEnvSecretRef(secretRef: string): string {
  const ref = secretRef.trim();
  if (ref === "") {
    return "";
  }
  if (ref.startsWith("env:")) {
    const key = ref.slice("env:".length).trim();
    return isEnvironmentVariableName(key) ? `env:${key}` : "";
  }
  if (ref.startsWith("$")) {
    const key = ref.slice(1).trim();
    return isEnvironmentVariableName(key) ? `env:${key}` : "";
  }
  if (isEnvironmentVariableName(ref)) {
    return `env:${ref}`;
  }
  return "";
}

function isEnvironmentVariableName(value: string): boolean {
  return /^[A-Z_][A-Z0-9_]*$/.test(value);
}

function schemaPropertyCount(schema?: Record<string, unknown>): number {
  const properties = schema?.properties;
  if (properties && typeof properties === "object" && !Array.isArray(properties)) {
    return Object.keys(properties).length;
  }
  return 0;
}

function fieldsFromSchema(schema?: Record<string, unknown>): SchemaFieldDraft[] {
  const properties = objectRecord(schema?.properties);
  if (!properties) {
    return [];
  }

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

function schemaFromFields(fields: SchemaFieldDraft[]): Record<string, unknown> {
  const properties: Record<string, unknown> = {};
  const required: string[] = [];

  for (const field of fields) {
    const name = field.name.trim();
    if (!name) continue;

    properties[name] = schemaDefinitionFromField(field);
    if (field.required) {
      required.push(name);
    }
  }

  return {
    type: "object",
    required,
    properties
  };
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

function inferSchemaFieldType(definition: Record<string, unknown>): SchemaFieldType {
  if (definition.type === "array") return "array";
  if (definition.type === "object" || objectRecord(definition.properties)) return "object";
  return toSchemaFieldType(definition.type);
}

function inferArrayItemType(itemSchema?: Record<string, unknown>): SchemaFieldType {
  if (!itemSchema) return "string";
  if (itemSchema.type === "object" || objectRecord(itemSchema.properties)) return "object";
  return toSchemaFieldType(itemSchema.type);
}

function toSchemaFieldType(value: unknown): SchemaFieldType {
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

function headersFromRecord(headers: Record<string, string>): HeaderDraft[] {
  return Object.entries(headers).map(([name, value]) => createHeader({ name, value }));
}

function headersToRecord(headers: HeaderDraft[]): Record<string, string> {
  return headers.reduce<Record<string, string>>((result, header) => {
    const name = header.name.trim();
    if (name) {
      result[name] = header.value;
    }
    return result;
  }, {});
}

function createDraftID(prefix: string): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}
