import { platformApi } from "../../lib/api";
import type { SkillDependencies, SkillSpec } from "../../types/platform";

export type SkillsClient = {
  listSkills: () => Promise<SkillSpec[]>;
  saveSkill: (skill: SkillSpec) => Promise<SkillSpec>;
  enableSkill: (skillId: string) => Promise<SkillSpec>;
  disableSkill: (skillId: string) => Promise<SkillSpec>;
  getDependencies: (skillId: string) => Promise<SkillDependencies>;
};

export const skillsClient: SkillsClient = {
  async listSkills() {
    const response = await platformApi.listSkills();
    return response.items;
  },
  saveSkill(skill) {
    if (skill.id) {
      return platformApi.updateSkill(skill);
    }
    return platformApi.createSkill(skill);
  },
  enableSkill(skillId) {
    return platformApi.enableSkill(skillId);
  },
  disableSkill(skillId) {
    return platformApi.disableSkill(skillId);
  },
  getDependencies(skillId) {
    return platformApi.getSkillDependencies(skillId);
  }
};
