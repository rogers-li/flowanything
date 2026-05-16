import type { AgentDependencies, AgentProfile, SkillSpec } from "../../types/platform";

export type AgentDraft = {
  id: string;
  name: string;
  description: string;
  businessDomain: string;
  ownerTeam: string;
  skillIds: string[];
  toolIds: string[];
  defaultLang: string;
  supportedLanguagesText: string;
  channelsText: string;
  systemPrompt: string;
  welcomeMessage: string;
  modelProviderId: string;
  model: string;
  temperature: number;
  maxTurns: number;
  maxToolCalls: number;
  responseTimeoutMs: number;
  status: AgentProfile["status"];
};

export const emptyAgentDependencies = (agentId: string): AgentDependencies => ({
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
});

export function createAgentDraft(): AgentDraft {
  return {
    id: "",
    name: "",
    description: "",
    businessDomain: "General",
    ownerTeam: "AI Platform",
    skillIds: [],
    toolIds: [],
    defaultLang: "zh-CN",
    supportedLanguagesText: "zh-CN",
    channelsText: "text, voice",
    systemPrompt: "",
    welcomeMessage: "",
    modelProviderId: "provider_mock",
    model: "mock-chat",
    temperature: 0.2,
    maxTurns: 12,
    maxToolCalls: 6,
    responseTimeoutMs: 30000,
    status: "draft"
  };
}

export function draftFromAgent(agent: AgentProfile): AgentDraft {
  return {
    id: agent.id,
    name: agent.name,
    description: agent.description,
    businessDomain: agent.businessDomain ?? "General",
    ownerTeam: agent.ownerTeam ?? "AI Platform",
    skillIds: agent.skillIds,
    toolIds: agent.toolIds ?? [],
    defaultLang: agent.defaultLang,
    supportedLanguagesText: (agent.supportedLanguages ?? []).join(", "),
    channelsText: (agent.channels ?? []).join(", "),
    systemPrompt: agent.systemPrompt ?? "",
    welcomeMessage: agent.welcomeMessage ?? "",
    modelProviderId: agent.modelConfig?.providerId ?? "provider_mock",
    model: agent.modelConfig?.model ?? "mock-chat",
    temperature: agent.modelConfig?.temperature ?? 0.2,
    maxTurns: agent.runtimePolicy?.maxTurns ?? 12,
    maxToolCalls: agent.runtimePolicy?.maxToolCalls ?? 6,
    responseTimeoutMs: agent.runtimePolicy?.responseTimeoutMs ?? 30000,
    status: agent.status
  };
}

export function agentFromDraft(draft: AgentDraft, tenantId: string): AgentProfile {
  return {
    id: draft.id,
    tenantId,
    name: draft.name.trim(),
    description: draft.description.trim(),
    businessDomain: draft.businessDomain.trim(),
    ownerTeam: draft.ownerTeam.trim(),
    status: draft.status,
    skillIds: draft.skillIds,
    toolIds: draft.toolIds,
    defaultLang: draft.defaultLang.trim() || "zh-CN",
    supportedLanguages: splitCSV(draft.supportedLanguagesText),
    channels: splitCSV(draft.channelsText),
    systemPrompt: draft.systemPrompt.trim(),
    welcomeMessage: draft.welcomeMessage.trim(),
    modelConfig: {
      providerId: draft.modelProviderId.trim(),
      model: draft.model.trim(),
      temperature: draft.temperature
    },
    runtimePolicy: {
      maxTurns: draft.maxTurns,
      maxToolCalls: draft.maxToolCalls,
      responseTimeoutMs: draft.responseTimeoutMs
    },
    version: "v1"
  };
}

export function toggleSkill(skillIds: string[], skillId: string, checked: boolean): string[] {
  if (checked) return Array.from(new Set([...skillIds, skillId]));
  return skillIds.filter((id) => id !== skillId);
}

export function toggleTool(toolIds: string[], toolId: string, checked: boolean): string[] {
  if (checked) return Array.from(new Set([...toolIds, toolId]));
  return toolIds.filter((id) => id !== toolId);
}

export function skillNames(agent: AgentProfile, skills: SkillSpec[]): string {
  if (agent.skillIds.length === 0) return "None";
  return agent.skillIds
    .map((skillId) => skills.find((skill) => skill.id === skillId)?.name ?? skillId)
    .join(", ");
}

function splitCSV(value: string): string[] {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}
