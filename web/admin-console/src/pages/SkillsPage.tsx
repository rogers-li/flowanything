import { useMemo, useState, type ReactNode } from "react";
import { Badge } from "../components/Badge";
import { ToolSelector } from "../components/ToolSelector";
import { toggleTool } from "../features/skills/domain";
import { useSkills } from "../features/skills/useSkills";
import type { SkillSpec } from "../types/platform";

const statusTone = {
  draft: "gray",
  enabled: "green",
  disabled: "red"
} as const;

const statusCopy = {
  draft: "Draft",
  enabled: "Enabled",
  disabled: "Disabled"
} as const;

const riskTone = {
  low: "green",
  medium: "amber",
  high: "red"
} as const;

type SkillView = "list" | "detail";
type ConfigSectionKey = "basic" | "tools" | "guidance" | "policy" | "usage";

export function SkillsPage() {
  const [view, setView] = useState<SkillView>("list");
  const [openConfigSection, setOpenConfigSection] = useState<ConfigSectionKey | null>(null);
  const {
    skills,
    tools,
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
  } = useSkills();

  const openNewSkill = () => {
    startNewSkill();
    setOpenConfigSection(null);
    setView("detail");
  };

  const openSkill = (skillId: string) => {
    selectSkill(skillId);
    setOpenConfigSection(null);
    setView("detail");
  };
  const bindableTools = useMemo(
    () => tools.filter((tool) => tool.status === "enabled" || draft.toolIds.includes(tool.id)),
    [draft.toolIds, tools]
  );

  if (view === "detail") {
    return (
      <div className="agent-editor-page skill-editor-page">
        <header className="agent-editor-topbar">
          <button className="agent-editor-back" type="button" onClick={() => setView("list")} aria-label="Back to Skills">
            <span aria-hidden="true">←</span>
          </button>
          <div className="agent-editor-title">
            <h1>{draft.name || "Untitled Skill"}</h1>
            <div>
              <Badge tone={riskTone[draft.riskLevel]}>{draft.riskLevel}</Badge>
              <Badge tone={statusTone[selectedSkill?.status ?? draft.status]}>{statusCopy[selectedSkill?.status ?? draft.status]}</Badge>
            </div>
          </div>
          <div className="agent-editor-actions">
            <button className="secondary-action" type="button" onClick={() => void disableSelected()} disabled={!selectedSkill}>
              Disable
            </button>
            <button className="secondary-action" type="button" onClick={() => void enableSelected()} disabled={!selectedSkill}>
              Enable
            </button>
            <button className="primary-action" type="button" onClick={() => void saveDraft()}>
              Save
            </button>
          </div>
        </header>

        {notice ? <p className={notice.ok ? "notice notice-ok agent-editor-notice" : "notice notice-error agent-editor-notice"}>{notice.message}</p> : null}

        <section className="agent-editor-layout">
          <aside className="agent-config-sidebar" aria-label="Skill configuration">
            <div className="agent-config-title">
              <strong>Config</strong>
              <span>Reusable Skill</span>
            </div>

            <ConfigSection
              open={openConfigSection === "basic"}
              title="Basic"
              meta={draft.businessDomain || "General"}
              onToggle={() => setOpenConfigSection(openConfigSection === "basic" ? null : "basic")}
            >
              <label>
                Skill Name
                <input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
              </label>
              <label>
                Description
                <textarea value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} />
              </label>
              <label>
                Business Domain
                <input value={draft.businessDomain} onChange={(event) => setDraft({ ...draft, businessDomain: event.target.value })} />
              </label>
              <label>
                Owner Team
                <input value={draft.ownerTeam} onChange={(event) => setDraft({ ...draft, ownerTeam: event.target.value })} />
              </label>
            </ConfigSection>

            <ConfigSection
              open={openConfigSection === "tools"}
              title="Tools"
              meta={`${draft.toolIds.length} selected`}
              onToggle={() => setOpenConfigSection(openConfigSection === "tools" ? null : "tools")}
            >
              <ToolSelector
                emptyMessage="No enabled Tools available."
                searchLabel="Search enabled tools"
                searchPlaceholder="Search by name, connector, MCP server, description..."
                selectedToolIds={draft.toolIds}
                summaryItems={[
                  { label: "Selected", value: draft.toolIds.length },
                  { label: "Available", value: bindableTools.length }
                ]}
                tools={bindableTools}
                onToggle={(toolId, checked) => setDraft({ ...draft, toolIds: toggleTool(draft.toolIds, toolId, checked) })}
              />
              <label>
                Knowledge Base IDs
                <input value={draft.knowledgeIdsText} onChange={(event) => setDraft({ ...draft, knowledgeIdsText: event.target.value })} />
              </label>
            </ConfigSection>

            <ConfigSection
              open={openConfigSection === "guidance"}
              title="Guidance"
              meta="use cases"
              onToggle={() => setOpenConfigSection(openConfigSection === "guidance" ? null : "guidance")}
            >
              <label>
                Use Cases
                <textarea value={draft.useCasesText} onChange={(event) => setDraft({ ...draft, useCasesText: event.target.value })} />
              </label>
              <label>
                Exclusions
                <textarea value={draft.exclusionsText} onChange={(event) => setDraft({ ...draft, exclusionsText: event.target.value })} />
              </label>
              <label>
                Output Format
                <textarea value={draft.outputFormat} onChange={(event) => setDraft({ ...draft, outputFormat: event.target.value })} />
              </label>
            </ConfigSection>

            <ConfigSection
              open={openConfigSection === "policy"}
              title="Policy"
              meta={draft.riskLevel}
              onToggle={() => setOpenConfigSection(openConfigSection === "policy" ? null : "policy")}
            >
              <label>
                Risk Level
                <select
                  value={draft.riskLevel}
                  onChange={(event) => setDraft({ ...draft, riskLevel: event.target.value as SkillSpec["riskLevel"] })}
                >
                  <option value="low">low</option>
                  <option value="medium">medium</option>
                  <option value="high">high</option>
                </select>
              </label>
              <div className="agent-config-inline">
                <label>
                  Tool Calls
                  <input
                    type="number"
                    min="0"
                    value={draft.maxToolCalls}
                    onChange={(event) => setDraft({ ...draft, maxToolCalls: Number(event.target.value) })}
                  />
                </label>
                <label>
                  Timeout ms
                  <input
                    type="number"
                    min="0"
                    value={draft.timeoutMillis}
                    onChange={(event) => setDraft({ ...draft, timeoutMillis: Number(event.target.value) })}
                  />
                </label>
              </div>
              <label className="agent-checkbox-row">
                <input
                  checked={draft.allowWriteTools}
                  type="checkbox"
                  onChange={(event) => setDraft({ ...draft, allowWriteTools: event.target.checked })}
                />
                Allow write tools
              </label>
              <label className="agent-checkbox-row">
                <input
                  checked={draft.requireConfirmation}
                  type="checkbox"
                  onChange={(event) => setDraft({ ...draft, requireConfirmation: event.target.checked })}
                />
                Require explicit user confirmation
              </label>
            </ConfigSection>

            <ConfigSection
              open={openConfigSection === "usage"}
              title="Usage"
              meta={`${selectedDependencies.summary.totalConsumerCount} consumers`}
              onToggle={() => setOpenConfigSection(openConfigSection === "usage" ? null : "usage")}
            >
              <div className="agent-config-summary-line">
                <span>{selectedDependencies.summary.directAgentCount} Agents</span>
                <span>{selectedDependencies.summary.totalConsumerCount} Consumers</span>
              </div>
              <div className="skill-consumer-list">
                {selectedDependencies.directAgents.map((agent) => (
                  <div key={agent.id}>
                    <strong>{agent.name}</strong>
                    <code>{agent.id}</code>
                  </div>
                ))}
                {selectedDependencies.directAgents.length === 0 ? <p>No Agent consumers.</p> : null}
              </div>
            </ConfigSection>
          </aside>

          <main className="agent-prompt-workspace">
            <div className="agent-prompt-header">
              <div>
                <h2>Skill Prompt</h2>
                <p>Define when this Skill should be used, how it should reason, and how it should call bound tools.</p>
              </div>
            </div>

            <textarea
              className="agent-prompt-editor"
              value={draft.systemPrompt}
              placeholder="## Skill Role&#10;你是一个可复用的业务能力 Skill...&#10;&#10;## When to use&#10;1. 用户问题命中指定业务域。&#10;2. 需要调用绑定工具完成查询或操作。&#10;&#10;## Tool rules&#10;1. 缺少必要参数时先追问。&#10;2. 高风险操作必须确认。"
              onChange={(event) => setDraft({ ...draft, systemPrompt: event.target.value })}
            />
          </main>
        </section>
      </div>
    );
  }

  return (
    <div className="agent-gallery-page skill-gallery-page">
      <header className="agent-gallery-header">
        <div>
          <h1>Skills</h1>
        </div>
        <button className="primary-action" type="button" onClick={openNewSkill}>
          New Skill
        </button>
      </header>

      {notice ? <p className={notice.ok ? "notice notice-ok" : "notice notice-error"}>{notice.message}</p> : null}

      <section className="agent-card-grid" aria-label="Skill list">
        {skills.map((skill) => (
          <SkillCard
            agentCount={dependenciesBySkill[skill.id]?.summary.directAgentCount ?? 0}
            key={skill.id}
            skill={skill}
            toolCount={skill.toolIds.length}
            onOpen={() => openSkill(skill.id)}
          />
        ))}
      </section>
    </div>
  );
}

function SkillCard({ agentCount, skill, toolCount, onOpen }: { agentCount: number; skill: SkillSpec; toolCount: number; onOpen: () => void }) {
  return (
    <button className="agent-app-card skill-app-card" type="button" onClick={onOpen}>
      <div className="agent-card-main">
        <span className="agent-app-icon" aria-hidden="true">
          {initials(skill.name)}
        </span>
        <div>
          <h2>{skill.name}</h2>
          <div className="agent-card-badges">
            <Badge tone={statusTone[skill.status]}>{statusCopy[skill.status]}</Badge>
            <Badge tone={riskTone[skill.riskLevel]}>{skill.riskLevel}</Badge>
          </div>
        </div>
      </div>
      <p>{skill.description || "No description"}</p>
      <footer>
        <span>{skill.ownerTeam ?? "AI Platform"}</span>
        <span>{`${toolCount} tools | ${agentCount} agents`}</span>
      </footer>
    </button>
  );
}

function ConfigSection({
  children,
  meta,
  onToggle,
  open,
  title
}: {
  children: ReactNode;
  meta: string;
  onToggle: () => void;
  open: boolean;
  title: string;
}) {
  return (
    <section className={open ? "agent-config-group agent-config-group-open" : "agent-config-group"}>
      <button className="agent-config-group-trigger" type="button" onClick={onToggle} aria-expanded={open}>
        <span aria-hidden="true">{open ? "⌄" : "›"}</span>
        <strong>{title}</strong>
        <small>{meta}</small>
      </button>
      {open ? <div className="agent-config-group-body">{children}</div> : null}
    </section>
  );
}

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "S";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
}
