import { useEffect, useMemo, useState } from "react";
import { resourceApi } from "../../platform/configApi";
import { useConfigWorkspace } from "../../platform/ConfigWorkspaceProvider";
import type { AgentConfig, SkillConfig, ToolConfig } from "../../platform/configTypes";
import type { SkillDependencies, SkillSpec, ToolSpec } from "../../types/platform";
import { toolSpecFromConfig, skillSpecFromConfig } from "../agents/configModel";
import { useAutoDismissNotice, type NoticeState } from "../common/useAutoDismissNotice";
import { dependenciesForSkill, skillConfigFromDraft } from "./configModel";
import { createSkillDraft, draftFromSkill, type SkillDraft } from "./domain";

type Notice = NoticeState;

export function useSkills() {
  const workspace = useConfigWorkspace();
  const [skillItems, setSkillItems] = useState<SkillSpec[]>([]);
  const [toolItems, setToolItems] = useState<ToolSpec[]>([]);
  const [agentConfigs, setAgentConfigs] = useState<AgentConfig[]>([]);
  const [selectedSkillId, setSelectedSkillId] = useState("");
  const [draft, setDraft] = useState<SkillDraft>(() => createSkillDraft());
  const [notice, setNotice] = useAutoDismissNotice<Notice>();

  const selectedSkill = useMemo(
    () => skillItems.find((skill) => skill.id === selectedSkillId),
    [skillItems, selectedSkillId]
  );

  const selectedDependencies = useMemo(
    () => dependenciesForSkill(selectedSkill, agentConfigs),
    [agentConfigs, selectedSkill]
  );

  const dependenciesBySkill = useMemo<Record<string, SkillDependencies>>(
    () => Object.fromEntries(skillItems.map((skill) => [skill.id, dependenciesForSkill(skill, agentConfigs)])),
    [agentConfigs, skillItems]
  );

  useEffect(() => {
    void refresh();
  }, [workspace.activeBundleId]);

  useEffect(() => {
    if (!selectedSkill) {
      if (!selectedSkillId) setDraft(createSkillDraft());
      return;
    }
    setDraft(draftFromSkill(selectedSkill));
  }, [selectedSkill?.id, selectedSkillId]);

  async function refresh() {
    if (!workspace.activeBundleId) {
      setSkillItems([]);
      setToolItems([]);
      setAgentConfigs([]);
      setSelectedSkillId("");
      return;
    }
    try {
      const [skills, tools, agents] = await Promise.all([
        resourceApi.listResourcesByKind<SkillConfig>(workspace.activeBundleId, "skill"),
        resourceApi.listResourcesByKind<ToolConfig>(workspace.activeBundleId, "tool"),
        resourceApi.listResourcesByKind<AgentConfig>(workspace.activeBundleId, "agent")
      ]);
      const nextSkills = skills.items.map((item) => skillSpecFromConfig(item.resource));
      setSkillItems(nextSkills);
      setToolItems(tools.items.map((item) => toolSpecFromConfig(item.resource)));
      setAgentConfigs(agents.items.map((item) => item.resource));
      setSelectedSkillId((current) => (nextSkills.some((skill) => skill.id === current) ? current : nextSkills[0]?.id ?? ""));
      if (nextSkills.length === 0) {
        setDraft(createSkillDraft());
      }
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to load Skills from active Bundle." });
    }
  }

  async function saveDraft() {
    if (!workspace.activeBundleId) {
      setNotice({ ok: false, message: "Create or select a draft Bundle before saving Skills." });
      return;
    }
    try {
      const config = skillConfigFromDraft(draft);
      const response = await resourceApi.upsertResource(workspace.activeBundleId, "skill", config);
      const saved = response.bundle.resources?.skills?.find((skill) => skill.id === config.id) ?? config;
      const viewModel = skillSpecFromConfig(saved);
      upsertSkill(viewModel);
      setSelectedSkillId(viewModel.id);
      setDraft(draftFromSkill(viewModel));
      await workspace.refresh();
      setNotice({ ok: true, message: "Skill saved to draft Bundle." });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to save Skill." });
    }
  }

  async function enableSelected() {
    if (!selectedSkill) return;
    await changeDisabled(selectedSkill.id, false);
  }

  async function disableSelected() {
    if (!selectedSkill) return;
    await changeDisabled(selectedSkill.id, true);
  }

  async function changeDisabled(skillId: string, disabled: boolean) {
    if (!workspace.activeBundleId) return;
    try {
      const resource = await resourceApi.getResource<SkillConfig>(workspace.activeBundleId, "skill", skillId);
      const next = { ...resource.resource, disabled };
      await resourceApi.upsertResource(workspace.activeBundleId, "skill", next);
      const saved = skillSpecFromConfig(next);
      upsertSkill(saved);
      setDraft(draftFromSkill(saved));
      await workspace.refresh();
      setNotice({ ok: true, message: `Skill ${disabled ? "disabled" : "enabled"}.` });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to update Skill status." });
    }
  }

  function startNewSkill() {
    setSelectedSkillId("");
    setDraft(createSkillDraft());
  }

  function selectSkill(skillId: string) {
    setSelectedSkillId(skillId);
  }

  function upsertSkill(skill: SkillSpec) {
    setSkillItems((current) => {
      const exists = current.some((item) => item.id === skill.id);
      if (exists) {
        return current.map((item) => (item.id === skill.id ? skill : item));
      }
      return [skill, ...current];
    });
  }

  return {
    skills: skillItems,
    tools: toolItems,
    selectedSkill,
    selectedDependencies,
    dependenciesBySkill,
    draft,
    setDraft,
    notice,
    selectSkill,
    startNewSkill,
    saveDraft,
    enableSelected,
    disableSelected
  };
}
