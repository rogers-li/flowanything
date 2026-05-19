import type { AgentConfig, ResourceBinding, ResourceRef, SkillConfig } from "../../platform/configTypes";
import type { SkillDependencies, SkillSpec } from "../../types/platform";
import type { SkillDraft } from "./domain";

export function skillConfigFromDraft(draft: SkillDraft): SkillConfig {
  return {
    id: draft.id || stableResourceId("skill", draft.name),
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
      use_cases: splitLines(draft.useCasesText),
      exclusions: splitLines(draft.exclusionsText),
      output_format: draft.outputFormat.trim(),
      risk_level: draft.riskLevel,
      max_tool_calls: draft.maxToolCalls,
      timeout_ms: draft.timeoutMillis,
      allow_write_tools: draft.allowWriteTools,
      require_confirmation: draft.requireConfirmation,
      policy_version: "v1"
    },
    prompt: {
      system: draft.systemPrompt,
      developer: "",
      templates: {},
      variables: [],
      metadata: {}
    },
    input_schema: [],
    output_schema: [],
    tools: idsToBindings("tool", draft.toolIds),
    knowledge: idsToBindings("knowledge", splitCSV(draft.knowledgeIdsText)),
    policies: [],
    runtime: {
      network: true,
      server_proxy_allowed: true
    }
  };
}

export function dependenciesForSkill(skill: SkillSpec | undefined, agents: AgentConfig[]): SkillDependencies {
  if (!skill) {
    return emptySkillDependencies("");
  }
  const directAgents = agents
    .filter((agent) => (agent.skills ?? []).some((binding) => !binding.disabled && binding.ref?.id === skill.id))
    .map((agent) => ({
      id: agent.id,
      name: agent.name
    }));
  return {
    skillId: skill.id,
    summary: {
      directAgentCount: directAgents.length,
      totalConsumerCount: directAgents.length
    },
    directAgents
  };
}

function emptySkillDependencies(skillId: string): SkillDependencies {
  return {
    skillId,
    summary: {
      directAgentCount: 0,
      totalConsumerCount: 0
    },
    directAgents: []
  };
}

function idsToBindings(kind: ResourceRef["kind"], ids: string[]): ResourceBinding[] {
  return Array.from(new Set(ids.filter(Boolean))).map((id) => ({
    ref: { kind, id },
    alias: id,
    disabled: false,
    config: {}
  }));
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

function stableResourceId(prefix: string, name: string): string {
  const normalized = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
  return `${prefix}_${normalized || Date.now().toString(16)}`;
}
