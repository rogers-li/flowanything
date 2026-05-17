import type { SkillDependencies, SkillSpec, ToolSpec } from "../../types/platform";

export type SkillDraft = {
  id: string;
  name: string;
  description: string;
  businessDomain: string;
  ownerTeam: string;
  systemPrompt: string;
  useCasesText: string;
  exclusionsText: string;
  outputFormat: string;
  toolIds: string[];
  knowledgeIdsText: string;
  riskLevel: SkillSpec["riskLevel"];
  maxToolCalls: number;
  timeoutMillis: number;
  allowWriteTools: boolean;
  requireConfirmation: boolean;
  status: SkillSpec["status"];
};

export const emptySkillDependencies = (skillId: string): SkillDependencies => ({
  skillId,
  summary: {
    directAgentCount: 0,
    totalConsumerCount: 0
  },
  directAgents: []
});

export function createSkillDraft(): SkillDraft {
  return {
    id: "",
    name: "",
    description: "",
    businessDomain: "General",
    ownerTeam: "AI Platform",
    systemPrompt: "",
    useCasesText: "",
    exclusionsText: "",
    outputFormat: "",
    toolIds: [],
    knowledgeIdsText: "",
    riskLevel: "low",
    maxToolCalls: 4,
    timeoutMillis: 30000,
    allowWriteTools: false,
    requireConfirmation: false,
    status: "draft"
  };
}

export function draftFromSkill(skill: SkillSpec): SkillDraft {
  return {
    id: skill.id,
    name: skill.name,
    description: skill.description,
    businessDomain: skill.businessDomain ?? "General",
    ownerTeam: skill.ownerTeam ?? "AI Platform",
    systemPrompt: skill.systemPrompt,
    useCasesText: (skill.useCases ?? []).join("\n"),
    exclusionsText: (skill.exclusions ?? []).join("\n"),
    outputFormat: skill.outputFormat ?? "",
    toolIds: skill.toolIds,
    knowledgeIdsText: skill.knowledgeIds.join(", "),
    riskLevel: skill.riskLevel,
    maxToolCalls: skill.executionPolicy?.maxToolCalls ?? 4,
    timeoutMillis: skill.executionPolicy?.timeoutMillis ?? 30000,
    allowWriteTools: skill.executionPolicy?.allowWriteTools ?? false,
    requireConfirmation: skill.executionPolicy?.requireConfirmation ?? false,
    status: skill.status
  };
}

export function skillFromDraft(draft: SkillDraft, tenantId: string): SkillSpec {
  return {
    id: draft.id,
    tenantId,
    name: draft.name.trim(),
    description: draft.description.trim(),
    businessDomain: draft.businessDomain.trim(),
    ownerTeam: draft.ownerTeam.trim(),
    status: draft.status,
    toolIds: draft.toolIds,
    knowledgeIds: splitCSV(draft.knowledgeIdsText),
    systemPrompt: draft.systemPrompt.trim(),
    useCases: splitLines(draft.useCasesText),
    exclusions: splitLines(draft.exclusionsText),
    outputFormat: draft.outputFormat.trim(),
    riskLevel: draft.riskLevel,
    executionPolicy: {
      maxToolCalls: draft.maxToolCalls,
      timeoutMillis: draft.timeoutMillis,
      allowWriteTools: draft.allowWriteTools,
      requireConfirmation: draft.requireConfirmation
    },
    policyVersion: "v1",
    version: "v1"
  };
}

export function toggleTool(toolIds: string[], toolId: string, checked: boolean): string[] {
  if (checked) return Array.from(new Set([...toolIds, toolId]));
  return toolIds.filter((id) => id !== toolId);
}

export function toolNames(skill: SkillSpec, tools: ToolSpec[]): string {
  if (skill.toolIds.length === 0) return "None";
  return skill.toolIds
    .map((toolId) => tools.find((tool) => tool.id === toolId)?.name ?? toolId)
    .join(", ");
}

function splitLines(value: string): string[] {
  return value
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
}

function splitCSV(value: string): string[] {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);
}
