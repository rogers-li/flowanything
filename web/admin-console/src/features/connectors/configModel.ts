import type { AgentConfig, ConnectorConfig, ConnectorOperationConfig, SchemaField, SkillConfig, ToolConfig } from "../../platform/configTypes";
import type { Connector, ConnectorDependencies, ConnectorInvokeResult, ConnectorOperation } from "../../types/platform";
import {
  connectorSecretRefForDraft,
  type ConnectorDraft,
  type ConnectorOperationDraft,
  type SchemaFieldDraft
} from "./domain";

const tenantId = "tenant_1";

export function connectorFromConfig(connector: ConnectorConfig): Connector {
  const status = resourceStatus(connector);
  return {
    id: connector.id,
    tenantId,
    name: connector.name,
    description: connector.description ?? "",
    businessDomain: connector.labels?.[0] ?? "General",
    ownerTeam: connector.owner?.team ?? "AI Platform",
    type: "http",
    status,
    baseUrl: connector.protocol?.base_url ?? "",
    headers: stringRecord(connector.protocol?.config?.headers),
    auth: {
      type: connectorAuthType(connector.auth?.type),
      headerName: typeof connector.auth?.config?.header_name === "string" ? connector.auth.config.header_name : undefined,
      secretRef: connector.auth?.secret_ref,
      config: connector.auth?.config ?? {}
    },
    timeoutMillis: durationMillis(connector.metadata?.timeout_ms, 10000),
    version: connector.version
  };
}

export function connectorConfigFromDraft(draft: ConnectorDraft, operations: ConnectorOperationConfig[] = [], current?: ConnectorConfig): ConnectorConfig {
  const id = draft.id || stableResourceId("conn", draft.name);
  const status = draft.status;
  const headers = headersToRecord(draft.headers);
  const secretRef = connectorSecretRefForDraft(draft);
  const next: ConnectorConfig = {
    id,
    name: draft.name.trim(),
    description: draft.description.trim(),
    version: "v1",
    disabled: status !== "enabled",
    labels: [draft.businessDomain.trim()].filter(Boolean),
    owner: {
      team: draft.ownerTeam.trim(),
      email: ""
    },
    metadata: {
      status,
      timeout_ms: draft.timeoutMillis
    },
    protocol: {
      kind: draft.type,
      base_url: draft.baseUrl.trim(),
      config: {
        headers
      }
    },
    auth: {
      type: draft.authType,
      secret_ref: secretRef || undefined,
      config: draft.authConfig ?? {}
    },
    operations,
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
    protocol: {
      ...current.protocol,
      ...next.protocol,
      kind: next.protocol?.kind ?? current.protocol?.kind ?? draft.type,
      config: {
        ...(current.protocol?.config ?? {}),
        ...(next.protocol?.config ?? {})
      }
    },
    auth: {
      ...current.auth,
      ...next.auth,
      config: {
        ...(current.auth?.config ?? {}),
        ...(next.auth?.config ?? {})
      }
    },
    operations,
    runtime: {
      ...(current.runtime ?? {}),
      ...(next.runtime ?? {})
    }
  };
}

export function connectorOperationFromConfig(connector: ConnectorConfig, operation: ConnectorOperationConfig): ConnectorOperation {
  const connectorView = connectorFromConfig(connector);
  return {
    id: operation.id,
    tenantId,
    connectorId: connector.id,
    name: operation.name,
    description: operation.description ?? "",
    businessDomain: operation.labels?.[0] ?? connectorView.businessDomain,
    ownerTeam: operation.owner?.team ?? connectorView.ownerTeam,
    type: "http",
    status: resourceStatus(operation),
    implementationMode: "simple_http",
    method: connectorMethod(operation.request?.method),
    baseUrl: connectorView.baseUrl,
    path: operation.request?.path ?? "",
    headers: operation.request?.headers,
    auth: connectorView.auth,
    inputSchema: jsonSchemaFromFields(operation.input_schema),
    outputSchema: jsonSchemaFromFields(operation.output_schema),
    timeoutMillis: durationMillis(operation.policy?.timeout, connectorView.timeoutMillis),
    version: operation.version
  };
}

export function connectorOperationConfigFromDraft(draft: ConnectorOperationDraft, current?: ConnectorOperationConfig): ConnectorOperationConfig {
  const status = draft.status;
  const next: ConnectorOperationConfig = {
    id: draft.id || stableResourceId("connop", draft.name),
    name: draft.name.trim(),
    description: draft.description.trim(),
    version: "v1",
    disabled: status !== "enabled",
    labels: [draft.businessDomain.trim()].filter(Boolean),
    owner: {
      team: draft.ownerTeam.trim(),
      email: ""
    },
    metadata: {
      status
    },
    input_schema: schemaFieldsFromDraft(draft.inputFields),
    output_schema: schemaFieldsFromDraft(draft.outputFields),
    request: {
      method: draft.method,
      path: draft.path.trim(),
      headers: headersToRecord(draft.headers)
    },
    response: {
      success_status_codes: [200, 201, 202, 204]
    },
    policy: {
      timeout: `${draft.timeoutMillis}ms`,
      require_review: false
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
    request: {
      ...(current.request ?? {}),
      ...(next.request ?? {}),
      config: {
        ...(current.request?.config ?? {}),
        ...(next.request?.config ?? {})
      }
    },
    response: {
      ...(current.response ?? {}),
      ...(next.response ?? {}),
      success_status_codes: current.response?.success_status_codes ?? next.response?.success_status_codes,
      config: {
        ...(current.response?.config ?? {}),
        ...(next.response?.config ?? {})
      }
    },
    policy: {
      ...(current.policy ?? {}),
      ...(next.policy ?? {}),
      retry_policy: {
        ...(current.policy?.retry_policy ?? {}),
        ...(next.policy?.retry_policy ?? {})
      }
    }
  };
}

export function dependenciesForConnectorOperation(
  operationId: string,
  tools: ToolConfig[],
  skills: SkillConfig[],
  agents: AgentConfig[]
): ConnectorDependencies {
  const directTools = tools
    .filter((tool) => !tool.disabled && tool.implementation.ref?.kind === "connector_operation" && tool.implementation.ref.id === operationId)
    .map((tool) => ({
      id: tool.id,
      name: tool.name,
      description: tool.description,
      requiresReview: Boolean(tool.policy?.require_review)
    }));
  const directToolIds = new Set(directTools.map((tool) => tool.id));
  const indirectSkills = skills
    .filter((skill) => (skill.tools ?? []).some((binding) => !binding.disabled && directToolIds.has(binding.ref.id)))
    .map((skill) => {
      const viaTool = (skill.tools ?? []).find((binding) => !binding.disabled && directToolIds.has(binding.ref.id));
      return {
        id: skill.id,
        name: skill.name,
        viaToolId: viaTool?.ref.id ?? ""
      };
    });
  const indirectSkillIds = new Set(indirectSkills.map((skill) => skill.id));
  const indirectAgents = agents
    .filter((agent) => (agent.skills ?? []).some((binding) => !binding.disabled && indirectSkillIds.has(binding.ref.id)))
    .map((agent) => {
      const viaSkill = (agent.skills ?? []).find((binding) => !binding.disabled && indirectSkillIds.has(binding.ref.id));
      return {
        id: agent.id,
        name: agent.name,
        viaSkillId: viaSkill?.ref.id ?? ""
      };
    });

  return {
    operationId,
    summary: {
      directToolCount: directTools.length,
      indirectSkillCount: indirectSkills.length,
      indirectAgentCount: indirectAgents.length,
      totalConsumerCount: directTools.length + indirectSkills.length + indirectAgents.length,
      blockingToolCount: directTools.length
    },
    directTools,
    indirectSkills,
    indirectAgents
  };
}

export function connectorInvokeResultFromRuntime(operationId: string, result: { result?: unknown }): ConnectorInvokeResult {
  const payload = result.result && typeof result.result === "object" ? (result.result as Record<string, unknown>) : { value: result.result };
  const error = payload.error && typeof payload.error === "object" ? (payload.error as Record<string, unknown>) : undefined;
  const output = payload.output && typeof payload.output === "object" ? (payload.output as Record<string, unknown>) : payload;
  return {
    requestId: stableResourceId("conncall", operationId),
    success: !error,
    data: output,
    errorCode: typeof error?.code === "string" ? error.code : undefined,
    finishedAt: new Date().toISOString()
  };
}

function schemaFieldsFromDraft(fields: SchemaFieldDraft[], parentPath = ""): SchemaField[] {
	return fields
		.filter((field) => field.name.trim())
		.map((field) => {
			const name = field.name.trim();
			return {
				name,
				type: field.type,
				description: field.description.trim(),
				required: field.required,
				children: schemaFieldsFromDraft(field.children ?? [], parentPath ? `${parentPath}.${name}` : name)
			};
		});
}

function jsonSchemaFromFields(fields?: SchemaField[]): Record<string, unknown> {
  const properties: Record<string, unknown> = {};
  const required: string[] = [];
  for (const field of fields ?? []) {
    properties[field.name] = schemaDefinitionFromField(field);
    if (field.required) required.push(field.name);
  }
  return { type: "object", properties, required };
}

function schemaDefinitionFromField(field: SchemaField): Record<string, unknown> {
  const definition: Record<string, unknown> = {
    type: field.type || "string"
  };
  if (field.description) {
    definition.description = field.description;
  }
  if (field.children?.length) {
    definition.properties = jsonSchemaFromFields(field.children).properties;
  }
  if (field.repeated || field.type === "array") {
    return {
      type: "array",
      description: field.description,
      items: field.children?.length ? { type: "object", properties: definition.properties } : { type: "string" }
    };
  }
  return definition;
}

function resourceStatus(resource: { disabled?: boolean; metadata?: Record<string, unknown> }): Connector["status"] {
  const status = resource.metadata?.status;
  if (status === "draft" || status === "enabled" || status === "disabled") return status;
  return resource.disabled ? "disabled" : "enabled";
}

function connectorMethod(method?: string): ConnectorOperation["method"] {
  if (method === "GET" || method === "POST" || method === "PUT" || method === "PATCH" || method === "DELETE") return method;
  return "GET";
}

function connectorAuthType(value?: string): NonNullable<Connector["auth"]>["type"] {
  if (value === "api_key" || value === "bearer" || value === "basic" || value === "oauth2") return value;
  return "none";
}

function headersToRecord(headers: Array<{ name: string; value: string }>): Record<string, string> {
  return headers.reduce<Record<string, string>>((result, header) => {
    const name = header.name.trim();
    if (name) result[name] = header.value;
    return result;
  }, {});
}

function stringRecord(value: unknown): Record<string, string> {
  if (!value || typeof value !== "object" || Array.isArray(value)) return {};
  return Object.entries(value as Record<string, unknown>).reduce<Record<string, string>>((result, [key, entry]) => {
    if (typeof entry === "string") result[key] = entry;
    return result;
  }, {});
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
