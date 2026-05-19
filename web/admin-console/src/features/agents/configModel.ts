import type { AgentDependencies, AgentProfile, SkillSpec, ToolImplementation, ToolSpec } from "../../types/platform";
import type { AgentConfig, ResourceBinding, ResourceRef, SkillConfig, ToolConfig, ToolType } from "../../platform/configTypes";
import type { AgentDraft } from "./domain";

export function agentProfileFromConfig(agent: AgentConfig): AgentProfile {
  const modelRef = agent.model_ref;
  return {
    id: agent.id,
    tenantId: "tenant_1",
    name: agent.name,
    description: agent.description ?? "",
    businessDomain: agent.labels?.[0] ?? "General",
    ownerTeam: agent.owner?.team ?? "AI Platform",
    status: agent.disabled ? "disabled" : "enabled",
    skillIds: bindingsToIds(agent.skills),
    toolIds: bindingsToIds(agent.tools),
    defaultLang: stringMeta(agent, "default_lang") || "zh-CN",
    supportedLanguages: stringListMeta(agent, "supported_languages", ["zh-CN"]),
    channels: stringListMeta(agent, "channels", ["text"]),
    systemPrompt: agent.prompt?.system ?? "",
    welcomeMessage: stringMeta(agent, "welcome_message"),
    modelConfig: {
      providerId: modelRef?.id || "provider_mock",
      model: stringMeta(agent, "model") || modelRef?.alias || modelRef?.id || "mock-chat",
      temperature: numberMeta(agent, "temperature", 0.2)
    },
    runtimePolicy: {
      maxTurns: numberMeta(agent, "max_turns", 12),
      maxToolCalls: numberMeta(agent, "max_tool_calls", 6),
      responseTimeoutMs: numberMeta(agent, "response_timeout_ms", 30000)
    },
    version: agent.version || "v1"
  };
}

export function agentConfigFromDraft(draft: AgentDraft): AgentConfig {
  return {
    id: draft.id || stableResourceId("agent", draft.name),
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
      default_lang: draft.defaultLang.trim() || "zh-CN",
      supported_languages: splitCSV(draft.supportedLanguagesText),
      channels: splitCSV(draft.channelsText),
      welcome_message: draft.welcomeMessage.trim(),
      model: draft.model.trim(),
      temperature: draft.temperature,
      max_turns: draft.maxTurns,
      max_tool_calls: draft.maxToolCalls,
      response_timeout_ms: draft.responseTimeoutMs
    },
    prompt: {
      system: draft.systemPrompt,
      developer: "",
      templates: {},
      variables: [],
      metadata: {}
    },
    reasoning: {
      mode: "react",
      config: {}
    },
    model_ref: {
      kind: "model",
      id: draft.modelProviderId.trim() || "provider_mock",
      alias: draft.model.trim()
    },
    skills: idsToBindings("skill", draft.skillIds),
    tools: idsToBindings("tool", draft.toolIds),
    workflows: [],
    knowledge: [],
    policies: [],
    output_schema: [],
    runtime: {
      network: true,
      server_proxy_allowed: true
    }
  };
}

export function skillSpecFromConfig(skill: SkillConfig): SkillSpec {
  return {
    id: skill.id,
    tenantId: "tenant_1",
    name: skill.name,
    description: skill.description ?? "",
    businessDomain: skill.labels?.[0] ?? "General",
    ownerTeam: skill.owner?.team ?? "AI Platform",
    status: skill.disabled ? "disabled" : "enabled",
    toolIds: bindingsToIds(skill.tools),
    knowledgeIds: bindingsToIds(skill.knowledge),
    systemPrompt: skill.prompt?.system ?? "",
    useCases: stringListMeta(skill, "use_cases", []),
    exclusions: stringListMeta(skill, "exclusions", []),
    outputFormat: stringMeta(skill, "output_format"),
    riskLevel: riskLevelFromMeta(skill.metadata?.risk_level),
    executionPolicy: {
      maxToolCalls: numberMeta(skill, "max_tool_calls", 4),
      timeoutMillis: numberMeta(skill, "timeout_ms", 30000),
      allowWriteTools: Boolean(skill.metadata?.allow_write_tools),
      requireConfirmation: Boolean(skill.metadata?.require_confirmation)
    },
    policyVersion: stringMeta(skill, "policy_version"),
    version: skill.version || "v1"
  };
}

export function toolSpecFromConfig(tool: ToolConfig): ToolSpec {
  return {
    id: tool.id,
    tenantId: "tenant_1",
    name: tool.name,
    description: tool.description ?? "",
    businessDomain: tool.labels?.[0] ?? "General",
    ownerTeam: tool.owner?.team ?? "AI Platform",
    llmDescription: stringMeta(tool, "llm_description") || tool.description,
    implementation: toolImplementationFromType(tool.type),
    binding: toolBindingFromConfig(tool),
    inputSchema: schemaObjectFromFields(tool.input_schema),
    outputSchema: schemaObjectFromFields(tool.output_schema),
    sideEffect: sideEffectFromMeta(tool.metadata?.side_effect),
    riskLevel: riskLevelFromMeta(tool.metadata?.risk_level),
    requiresConfirmation: Boolean(tool.policy?.require_review),
    timeoutMillis: timeoutMillis(tool.policy?.timeout, 10000),
    retryPolicy: {
      maxAttempts: tool.policy?.retry_policy?.max_attempts ?? 0,
      backoffMillis: timeoutMillis(tool.policy?.retry_policy?.backoff, 0)
    },
    status: tool.disabled ? "disabled" : "enabled",
    version: tool.version || "v1"
  };
}

export function dependenciesForAgent(agent: AgentProfile | undefined, skills: SkillSpec[], tools: ToolSpec[]): AgentDependencies {
  if (!agent) {
    return emptyDependencies("");
  }
  const selectedSkills = skills.filter((skill) => agent.skillIds.includes(skill.id));
  const directToolIds = new Set(agent.toolIds ?? []);
  const reachableToolIds = new Set([...directToolIds]);
  selectedSkills.forEach((skill) => skill.toolIds.forEach((toolId) => reachableToolIds.add(toolId)));
  const reachableTools = tools.filter((tool) => reachableToolIds.has(tool.id));
  return {
    agentId: agent.id,
    summary: {
      directSkillCount: selectedSkills.length,
      directToolCount: directToolIds.size,
      reachableToolCount: reachableTools.length,
      disabledSkillCount: selectedSkills.filter((skill) => skill.status === "disabled").length,
      totalCapabilityCount: selectedSkills.length + reachableTools.length
    },
    directSkills: selectedSkills.map((skill) => ({ id: skill.id, name: skill.name, status: skill.status })),
    reachableTools: reachableTools.map((tool) => ({
      id: tool.id,
      name: tool.name,
      source: directToolIds.has(tool.id) ? "direct" : "skill",
      implementation: tool.implementation,
      riskLevel: tool.riskLevel,
      status: tool.status
    }))
  };
}

function emptyDependencies(agentId: string): AgentDependencies {
  return {
    agentId,
    summary: {
      directSkillCount: 0,
      directToolCount: 0,
      reachableToolCount: 0,
      disabledSkillCount: 0,
      totalCapabilityCount: 0
    },
    directSkills: [],
    reachableTools: []
  };
}

function bindingsToIds(bindings?: ResourceBinding[]): string[] {
  return (bindings ?? []).filter((binding) => !binding.disabled && binding.ref?.id).map((binding) => binding.ref.id);
}

function idsToBindings(kind: ResourceRef["kind"], ids: string[]): ResourceBinding[] {
  return Array.from(new Set(ids.filter(Boolean))).map((id) => ({
    ref: { kind, id },
    alias: id,
    disabled: false,
    config: {}
  }));
}

function stringMeta(resource: { metadata?: Record<string, unknown> }, key: string): string {
  const value = resource.metadata?.[key];
  return typeof value === "string" ? value : "";
}

function stringListMeta(resource: { metadata?: Record<string, unknown> }, key: string, fallback: string[]): string[] {
  const value = resource.metadata?.[key];
  if (Array.isArray(value)) return value.filter((item): item is string => typeof item === "string");
  if (typeof value === "string") return splitCSV(value);
  return fallback;
}

function numberMeta(resource: { metadata?: Record<string, unknown> }, key: string, fallback: number): number {
  const value = resource.metadata?.[key];
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return fallback;
}

function splitCSV(value: string): string[] {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}

function timeoutMillis(value: unknown, fallback: number): number {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value !== "string" || !value.trim()) return fallback;
  const normalized = value.trim();
  if (normalized.endsWith("ms")) return Number(normalized.slice(0, -2)) || fallback;
  if (normalized.endsWith("s")) return (Number(normalized.slice(0, -1)) || fallback / 1000) * 1000;
  return Number(normalized) || fallback;
}

function riskLevelFromMeta(value: unknown): ToolSpec["riskLevel"] {
  return value === "medium" || value === "high" ? value : "low";
}

function sideEffectFromMeta(value: unknown): ToolSpec["sideEffect"] {
  return value === "read" || value === "write" ? value : "none";
}

function toolImplementationFromType(type: ToolType): ToolImplementation {
  if (type === "connector") return "connector";
  if (type === "workflow") return "workflow";
  if (type === "mcp") return "mcp";
  if (type === "script") return "python";
  if (type === "native") return "knowledge";
  return "connector";
}

function toolBindingFromConfig(tool: ToolConfig): ToolSpec["binding"] {
  const ref = tool.implementation?.ref;
  if (tool.type === "connector") return { connectorOperationId: ref?.id ?? "" };
  if (tool.type === "workflow") return { workflowId: ref?.id ?? "" };
  if (tool.type === "mcp") {
    const config = tool.implementation?.config ?? {};
    return {
      mcpServerId: stringConfig(config, "mcp_server_id") || stringMeta(tool, "mcp_server_id") || ref?.id,
      mcpServerUrl: stringConfig(config, "mcp_server_url") || stringMeta(tool, "mcp_server_url"),
      mcpTransport: stringConfig(config, "mcp_transport") || stringMeta(tool, "mcp_transport") || "streamable_http",
      mcpHeaders: stringRecordConfig(config, "mcp_headers"),
      mcpToolName: stringConfig(config, "mcp_tool_name") || stringMeta(tool, "mcp_tool_name") || tool.name
    };
  }
  if (tool.type === "script") return { pythonPackageId: stringConfig(tool.implementation?.config ?? {}, "python_package_id") || ref?.id };
  if (tool.type === "native") {
    const ids = arrayStringConfig(tool.implementation?.config ?? {}, "knowledge_base_ids");
    return { knowledgeBaseIds: ids.length > 0 ? ids : ref?.id ? [ref.id] : [] };
  }
  return {};
}

function stringConfig(config: Record<string, unknown>, key: string): string {
  const value = config[key];
  return typeof value === "string" ? value : "";
}

function stringRecordConfig(config: Record<string, unknown>, key: string): Record<string, string> {
  const value = config[key];
  if (!value || typeof value !== "object" || Array.isArray(value)) return {};
  return Object.fromEntries(
    Object.entries(value as Record<string, unknown>)
      .filter((entry): entry is [string, string] => typeof entry[1] === "string")
  );
}

function arrayStringConfig(config: Record<string, unknown>, key: string): string[] {
  const value = config[key];
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];
}

function schemaObjectFromFields(fields?: Array<{ name: string; type?: string; description?: string; required?: boolean }>): Record<string, unknown> {
  return {
    type: "object",
    properties: Object.fromEntries(
      (fields ?? []).map((field) => [
        field.name,
        {
          type: field.type || "string",
          description: field.description
        }
      ])
    ),
    required: (fields ?? []).filter((field) => field.required).map((field) => field.name)
  };
}

function stableResourceId(prefix: string, name: string): string {
  const normalized = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
  return `${prefix}_${normalized || Date.now().toString(16)}`;
}
