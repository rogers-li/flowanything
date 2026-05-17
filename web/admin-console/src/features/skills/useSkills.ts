import { useEffect, useMemo, useState } from "react";
import { defaultTenantId, platformApi } from "../../lib/api";
import { skillDependencies, skills, tools } from "../../lib/mockData";
import type { SkillDependencies, SkillSpec, ToolSpec } from "../../types/platform";
import { useAutoDismissNotice, type NoticeState } from "../common/useAutoDismissNotice";
import { skillsClient } from "./api";
import { createSkillDraft, draftFromSkill, emptySkillDependencies, skillFromDraft, type SkillDraft } from "./domain";

type Notice = NoticeState;

export function useSkills() {
  const [skillItems, setSkillItems] = useState<SkillSpec[]>(skills);
  const [toolItems, setToolItems] = useState<ToolSpec[]>(tools);
  const [selectedSkillId, setSelectedSkillId] = useState(skills[0]?.id ?? "");
  const [draft, setDraft] = useState<SkillDraft>(() => (skills[0] ? draftFromSkill(skills[0]) : createSkillDraft()));
  const [dependenciesBySkill, setDependenciesBySkill] = useState<Record<string, SkillDependencies>>(skillDependencies);
  const [notice, setNotice] = useAutoDismissNotice<Notice>();

  const selectedSkill = useMemo(
    () => skillItems.find((skill) => skill.id === selectedSkillId),
    [skillItems, selectedSkillId]
  );
  const selectedDependencies = selectedSkill
    ? dependenciesBySkill[selectedSkill.id] ?? emptySkillDependencies(selectedSkill.id)
    : emptySkillDependencies("");

  useEffect(() => {
    void refresh();
  }, []);

  useEffect(() => {
    if (!selectedSkill) {
      setDraft(createSkillDraft());
      return;
    }
    setDraft(draftFromSkill(selectedSkill));
    void loadDependencies(selectedSkill.id);
  }, [selectedSkill?.id]);

  async function refresh() {
    try {
      const [remoteSkills, remoteTools] = await Promise.all([
        skillsClient.listSkills(),
        platformApi.listTools().then((response) => response.items)
      ]);
      if (remoteSkills.length > 0) {
        setSkillItems(remoteSkills);
        setSelectedSkillId((current) => (remoteSkills.some((skill) => skill.id === current) ? current : remoteSkills[0].id));
      }
      if (remoteTools.length > 0) {
        setToolItems(remoteTools);
      }
    } catch {
      setNotice({ ok: false, message: "Using local skill mock data because backend APIs are unavailable." });
    }
  }

  async function loadDependencies(skillId: string) {
    try {
      const dependencies = await skillsClient.getDependencies(skillId);
      setDependenciesBySkill((current) => ({ ...current, [skillId]: dependencies }));
    } catch {
      setDependenciesBySkill((current) => ({
        ...current,
        [skillId]: current[skillId] ?? emptySkillDependencies(skillId)
      }));
    }
  }

  async function saveDraft() {
    try {
      const skill = skillFromDraft(draft, defaultTenantId);
      const saved = await skillsClient.saveSkill(skill);
      upsertSkill(saved);
      setSelectedSkillId(saved.id);
      setDraft(draftFromSkill(saved));
      setNotice({ ok: true, message: "Skill saved." });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to save skill." });
    }
  }

  async function enableSelected() {
    if (!selectedSkill) return;
    await changeStatus(selectedSkill.id, "enabled");
  }

  async function disableSelected() {
    if (!selectedSkill) return;
    await changeStatus(selectedSkill.id, "disabled");
  }

  async function changeStatus(skillId: string, status: SkillSpec["status"]) {
    try {
      const saved = status === "enabled" ? await skillsClient.enableSkill(skillId) : await skillsClient.disableSkill(skillId);
      upsertSkill(saved);
      setDraft(draftFromSkill(saved));
      setNotice({ ok: true, message: `Skill ${status}.` });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : `Failed to mark skill ${status}.` });
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
