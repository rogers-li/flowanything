import { useEffect, useMemo, useState, type ReactNode } from "react";
import { ChatBubble, TraceInspector } from "../components/AgentDebugChat";
import { Badge } from "../components/Badge";
import { PromptRichEditor } from "../components/PromptRichEditor";
import { ToolSelector } from "../components/ToolSelector";
import { toggleSkill, toggleTool } from "../features/agents/domain";
import { useAgents } from "../features/agents/useAgents";
import { inferProviderIdFromModelProviders, useModelProviders } from "../features/models/useModelProviders";
import { runtimeApiV2 } from "../platform/configApi";
import { agentTraceFromTraceResponse } from "../platform/traceViewModel";
import type { AgentProfile, AgentTrace, SkillSpec } from "../types/platform";

const statusTone = {
  draft: "gray",
  enabled: "green",
  disabled: "red"
} as const;

const statusCopy = {
  draft: "Draft",
  enabled: "Live",
  disabled: "Paused"
} as const;

type AgentView = "list" | "detail";
type ConfigSectionKey = "basic" | "model" | "skills" | "tools" | "runtime" | "messages";

function formatDebugSessionTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(undefined, {
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    month: "short"
  }).format(date);
}

function debugSessionPreview(messages: Array<{ role: string; text: string }>): string {
  const message = messages.find((item) => item.role === "user") ?? messages[0];
  if (!message) return "No messages";
  return message.text.length > 72 ? `${message.text.slice(0, 72)}...` : message.text;
}

export function AgentsPage() {
  const modelProviders = useModelProviders();
  const [view, setView] = useState<AgentView>("list");
  const [openConfigSections, setOpenConfigSections] = useState<ConfigSectionKey[]>([]);
  const [configCollapsed, setConfigCollapsed] = useState(false);
  const [debugCollapsed, setDebugCollapsed] = useState(false);
  const [isDebugHistoryOpen, setIsDebugHistoryOpen] = useState(false);
  const [activeTrace, setActiveTrace] = useState<AgentTrace | null>(null);
  const {
    agents,
    skills,
    tools,
    selectedAgent,
    selectedDependencies,
    draft,
    setDraft,
    debugMessage,
    setDebugMessage,
    debugSessionId,
    debugChatMessages,
    debugSessions,
    isDebugRunning,
    notice,
    selectAgent,
    startNewAgent,
    saveDraft,
    enableSelected,
    disableSelected,
    resetDebugSession,
    restoreDebugSession,
    runDebug
  } = useAgents();

  const openNewAgent = () => {
    startNewAgent();
    setOpenConfigSections([]);
    setActiveTrace(null);
    setIsDebugHistoryOpen(false);
    setView("detail");
  };

  const openAgent = (agentId: string) => {
    selectAgent(agentId);
    setOpenConfigSections([]);
    setActiveTrace(null);
    setIsDebugHistoryOpen(false);
    setView("detail");
  };

  const selectModelProvider = (providerId: string) => {
    const provider = modelProviders.find((item) => item.id === providerId);
    setDraft({
      ...draft,
      modelProviderId: providerId,
      model: provider?.defaultModel ?? draft.model
    });
  };

  const changeModel = (model: string) => {
    setDraft({
      ...draft,
      model,
      modelProviderId: inferProviderIdFromModelProviders(model, modelProviders) ?? draft.modelProviderId
    });
  };

  const bindableTools = useMemo(() => tools.filter((tool) => tool.status === "enabled"), [tools]);
  const enabledToolIds = useMemo(() => new Set(bindableTools.map((tool) => tool.id)), [bindableTools]);
  const skillToolIds = useMemo(
    () =>
      new Set(
        skills
          .filter((skill) => draft.skillIds.includes(skill.id))
          .flatMap((skill) => skill.toolIds.filter((toolId) => enabledToolIds.has(toolId)))
      ),
    [draft.skillIds, enabledToolIds, skills]
  );
  const availableToolIds = useMemo(
    () => Array.from(new Set([...draft.toolIds.filter((toolId) => enabledToolIds.has(toolId)), ...skillToolIds])),
    [draft.toolIds, enabledToolIds, skillToolIds]
  );
  const bindSkillToDraft = (skillId: string) => {
    if (draft.skillIds.includes(skillId)) return;
    setDraft((current) => ({ ...current, skillIds: toggleSkill(current.skillIds, skillId, true) }));
  };

  const bindToolToDraft = (toolId: string) => {
    if (!enabledToolIds.has(toolId)) return;
    if (availableToolIds.includes(toolId)) return;
    setDraft((current) => ({ ...current, toolIds: toggleTool(current.toolIds, toolId, true) }));
  };
  const openTraceById = async (traceId: string) => {
    try {
      setActiveTrace(agentTraceFromTraceResponse(await runtimeApiV2.getTrace(traceId)));
    } catch {
      // The run may still be starting or the trace may have expired; keep the chat usable.
    }
  };
  const isConfigSectionOpen = (section: ConfigSectionKey) => openConfigSections.includes(section);
  const toggleConfigSection = (section: ConfigSectionKey) => {
    setOpenConfigSections((current) =>
      current.includes(section) ? current.filter((item) => item !== section) : [...current, section]
    );
  };

  if (view === "detail") {
    return (
      <div className="agent-editor-page">
        <header className="agent-editor-topbar">
          <button className="agent-editor-back" type="button" onClick={() => setView("list")} aria-label="Back to Agents">
            <span aria-hidden="true">←</span>
          </button>
          <div className="agent-editor-title">
            <h1>{draft.name || "Untitled Agent"}</h1>
            <div>
              <Badge tone="blue">{draft.defaultLang}</Badge>
              <Badge tone={statusTone[selectedAgent?.status ?? draft.status]}>{statusCopy[selectedAgent?.status ?? draft.status]}</Badge>
            </div>
          </div>
          <div className="agent-editor-actions">
            <button className="secondary-action" type="button" onClick={() => void disableSelected()} disabled={!selectedAgent}>
              Pause
            </button>
            <button className="secondary-action" type="button" onClick={() => void enableSelected()} disabled={!selectedAgent}>
              Go Live
            </button>
            <button className="primary-action" type="button" onClick={() => void saveDraft()}>
              Save
            </button>
          </div>
        </header>

        {notice ? <p className={notice.ok ? "notice notice-ok agent-editor-notice" : "notice notice-error agent-editor-notice"}>{notice.message}</p> : null}

        <section
          className={[
            "agent-workbench-layout",
            configCollapsed ? "agent-workbench-config-collapsed" : "",
            debugCollapsed ? "agent-workbench-debug-collapsed" : ""
          ]
            .filter(Boolean)
            .join(" ")}
        >
          {configCollapsed ? (
            <button className="agent-side-rail-button" type="button" onClick={() => setConfigCollapsed(false)}>
              Config
            </button>
          ) : (
          <aside className="agent-config-sidebar" aria-label="Agent configuration">
            <div className="agent-config-title">
              <div>
                <strong>Config</strong>
                <span>Single Agent</span>
              </div>
              <button className="icon-action" type="button" onClick={() => setConfigCollapsed(true)} aria-label="Hide configuration">
                ‹
              </button>
            </div>

            <ConfigSection
              open={isConfigSectionOpen("basic")}
              title="Basic"
              meta={draft.ownerTeam || "Owner"}
              onToggle={() => toggleConfigSection("basic")}
            >
              <label>
                Name
                <input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
              </label>
              <label>
                Description
                <textarea value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} />
              </label>
              <label>
                Owner
                <input value={draft.ownerTeam} onChange={(event) => setDraft({ ...draft, ownerTeam: event.target.value })} />
              </label>
            </ConfigSection>

            <ConfigSection
              open={isConfigSectionOpen("model")}
              title="Model"
              meta={draft.model || "model"}
              onToggle={() => toggleConfigSection("model")}
            >
              <div className="agent-model-pill">
                <span>{draft.modelProviderId || "provider"}</span>
                <strong>{draft.model || "model"}</strong>
              </div>
              <label>
                Provider
                <select value={draft.modelProviderId} onChange={(event) => selectModelProvider(event.target.value)}>
                  {modelProviders.map((provider) => (
                    <option key={provider.id} value={provider.id}>
                      {provider.name}
                    </option>
                  ))}
                  {!modelProviders.some((provider) => provider.id === draft.modelProviderId) ? (
                    <option value={draft.modelProviderId}>{draft.modelProviderId}</option>
                  ) : null}
                </select>
              </label>
              <label>
                Model
                <input value={draft.model} onChange={(event) => changeModel(event.target.value)} />
              </label>
              <label>
                Temperature
                <input
                  type="number"
                  min="0"
                  max="2"
                  step="0.1"
                  value={draft.temperature}
                  onChange={(event) => setDraft({ ...draft, temperature: Number(event.target.value) })}
                />
              </label>
            </ConfigSection>

            <ConfigSection
              open={isConfigSectionOpen("skills")}
              title="Skills"
              meta={`${draft.skillIds.length} selected`}
              onToggle={() => toggleConfigSection("skills")}
            >
              <div className="agent-editor-skill-list">
                {skills.map((skill) => (
                  <SkillToggle
                    checked={draft.skillIds.includes(skill.id)}
                    key={skill.id}
                    skill={skill}
                    onChange={(checked) => setDraft({ ...draft, skillIds: toggleSkill(draft.skillIds, skill.id, checked) })}
                  />
                ))}
                {skills.length === 0 ? <p>No Skills available.</p> : null}
              </div>
              <div className="agent-config-summary-line">
                <span>{selectedDependencies.summary.directSkillCount} Skills</span>
                <span>{selectedDependencies.summary.reachableToolCount} Tools</span>
                <span>{selectedDependencies.summary.disabledSkillCount} Inactive</span>
              </div>
            </ConfigSection>

            <ConfigSection
              open={isConfigSectionOpen("tools")}
              title="Tools"
              meta={`${draft.toolIds.length} direct`}
              onToggle={() => toggleConfigSection("tools")}
            >
              <ToolSelector
                emptyMessage="No enabled Tools available."
                inheritedToolIds={Array.from(skillToolIds)}
                searchLabel="Search enabled tools"
                searchPlaceholder="Search by name, MCP server, description..."
                selectedToolIds={draft.toolIds}
                summaryItems={[
                  { label: "Direct", value: draft.toolIds.length },
                  { label: "From Skills", value: skillToolIds.size },
                  { label: "Available", value: availableToolIds.length }
                ]}
                tools={bindableTools}
                onToggle={(toolId, checked) => setDraft({ ...draft, toolIds: toggleTool(draft.toolIds, toolId, checked) })}
              />
            </ConfigSection>

            <ConfigSection
              open={isConfigSectionOpen("runtime")}
              title="Runtime"
              meta={draft.defaultLang}
              onToggle={() => toggleConfigSection("runtime")}
            >
              <label>
                Default Language
                <input value={draft.defaultLang} onChange={(event) => setDraft({ ...draft, defaultLang: event.target.value })} />
              </label>
              <label>
                Channels
                <input value={draft.channelsText} onChange={(event) => setDraft({ ...draft, channelsText: event.target.value })} />
              </label>
              <div className="agent-config-inline">
                <label>
                  Turns
                  <input
                    type="number"
                    min="0"
                    value={draft.maxTurns}
                    onChange={(event) => setDraft({ ...draft, maxTurns: Number(event.target.value) })}
                  />
                </label>
                <label>
                  Tools
                  <input
                    type="number"
                    min="0"
                    value={draft.maxToolCalls}
                    onChange={(event) => setDraft({ ...draft, maxToolCalls: Number(event.target.value) })}
                  />
                </label>
              </div>
            </ConfigSection>

            <ConfigSection
              open={isConfigSectionOpen("messages")}
              title="Messages"
              meta="welcome"
              onToggle={() => toggleConfigSection("messages")}
            >
              <label>
                Welcome Message
                <textarea value={draft.welcomeMessage} onChange={(event) => setDraft({ ...draft, welcomeMessage: event.target.value })} />
              </label>
            </ConfigSection>
          </aside>
          )}

          <main className="agent-prompt-workspace">
            <div className="agent-prompt-header">
              <div>
                <h2>Role and Prompt</h2>
                <p>Write the Agent role, behavioral rules, tool-use guidance, and answer style here.</p>
              </div>
            </div>

            <PromptRichEditor
              skills={skills}
              tools={bindableTools}
              selectedSkillIds={draft.skillIds}
              selectedToolIds={availableToolIds}
              value={draft.systemPrompt}
              onBindSkill={bindSkillToDraft}
              onBindTool={bindToolToDraft}
              onChange={(systemPrompt) => setDraft((current) => ({ ...current, systemPrompt }))}
            />
          </main>

          {debugCollapsed ? (
            <button className="agent-side-rail-button agent-side-rail-button-right" type="button" onClick={() => setDebugCollapsed(false)}>
              Debug
            </button>
          ) : (
            <aside className="agent-debug-sidebar" aria-label="Agent debug">
              <div className="agent-debug-workbench-header">
                <div>
                  <strong>Debug</strong>
                  <span>
                    Session <code>{debugSessionId}</code>
                  </span>
                </div>
                <div>
                  <button
                    className={isDebugHistoryOpen ? "secondary-action compact-action is-active" : "secondary-action compact-action"}
                    type="button"
                    onClick={() => setIsDebugHistoryOpen((current) => !current)}
                  >
                    History
                  </button>
                  <button
                    className="secondary-action compact-action"
                    type="button"
                    onClick={() => {
                      resetDebugSession();
                      setActiveTrace(null);
                      setIsDebugHistoryOpen(false);
                    }}
                  >
                    New
                  </button>
                  <button className="icon-action" type="button" onClick={() => setDebugCollapsed(true)} aria-label="Hide debug">
                    ›
                  </button>
                </div>
              </div>

              {isDebugHistoryOpen ? (
                <section className="debug-session-history" aria-label="Agent debug session history">
                  {debugSessions.length > 0 ? (
                    debugSessions.map((session) => (
                      <button
                        key={session.sessionId}
                        className={session.sessionId === debugSessionId ? "debug-session-item is-active" : "debug-session-item"}
                        type="button"
                        onClick={() => {
                          restoreDebugSession(session);
                          setActiveTrace(null);
                          setIsDebugHistoryOpen(false);
                        }}
                      >
                        <strong>{debugSessionPreview(session.messages)}</strong>
                        <span>
                          {formatDebugSessionTime(session.updatedAt)} · {session.messages.length} messages
                        </span>
                      </button>
                    ))
                  ) : (
                    <p>No saved debug sessions yet.</p>
                  )}
                </section>
              ) : null}

              <div className="agent-chat-window">
                {debugChatMessages.length === 0 ? (
                  <div className="agent-chat-empty">
                    <strong>Start a test chat</strong>
                    <p>Send a user message to verify prompt, model, tools, and connector flow.</p>
                  </div>
                ) : (
                  debugChatMessages.map((message) => (
                    <ChatBubble
                      active={message.traceId !== undefined && message.traceId === activeTrace?.traceId}
                      key={message.id}
                      message={message}
                      onOpenTrace={(trace) => setActiveTrace(trace)}
                      onOpenTraceId={openTraceById}
                    />
                  ))
                )}
              </div>

              <div className="agent-chat-composer">
                <textarea
                  value={debugMessage}
                  placeholder="输入测试问题，例如：帮我查一下深圳天气"
                  onChange={(event) => setDebugMessage(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.nativeEvent.isComposing) return;
                    if (event.key === "Enter" && !event.ctrlKey && !event.shiftKey && !event.altKey && !event.metaKey) {
                      event.preventDefault();
                      void runDebug();
                    }
                  }}
                />
                <button className="primary-action" type="button" onClick={() => void runDebug()} disabled={!selectedAgent || isDebugRunning}>
                  {isDebugRunning ? "Running..." : "Send"}
                </button>
              </div>

            </aside>
          )}
          {activeTrace ? <TraceInspector trace={activeTrace} onClose={() => setActiveTrace(null)} /> : null}
        </section>
      </div>
    );
  }

  return (
    <div className="agent-gallery-page">
      <header className="agent-gallery-header">
        <div>
          <h1>Agents</h1>
        </div>
        <button className="primary-action" type="button" onClick={openNewAgent}>
          New Agent
        </button>
      </header>

      {notice ? <p className={notice.ok ? "notice notice-ok" : "notice notice-error"}>{notice.message}</p> : null}

      <section className="agent-card-grid" aria-label="Agent list">
        {agents.map((agent) => (
          <AgentCard key={agent.id} agent={agent} onOpen={() => openAgent(agent.id)} />
        ))}
      </section>
    </div>
  );
}

function AgentCard({ agent, onOpen }: { agent: AgentProfile; onOpen: () => void }) {
  return (
    <button className="agent-app-card" type="button" onClick={onOpen}>
      <div className="agent-card-main">
        <span className="agent-app-icon" aria-hidden="true">
          {initials(agent.name)}
        </span>
        <div>
          <h2>{agent.name}</h2>
          <div className="agent-card-badges">
            <Badge tone={statusTone[agent.status]}>{statusCopy[agent.status]}</Badge>
            <Badge tone="blue">{agent.defaultLang}</Badge>
          </div>
        </div>
      </div>
      <p>{agent.description || "No description"}</p>
      <footer>
        <span>{agent.ownerTeam ?? "AI Platform"}</span>
        <span>{agent.modelConfig?.model ?? "default"}</span>
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

function SkillToggle({ checked, skill, onChange }: { checked: boolean; skill: SkillSpec; onChange: (checked: boolean) => void }) {
  return (
    <label className={checked ? "agent-tool-row agent-tool-row-active" : "agent-tool-row"}>
      <input checked={checked} type="checkbox" onChange={(event) => onChange(event.target.checked)} />
      <span>
        <strong>{skill.name}</strong>
        <small>{skill.description}</small>
      </span>
      <Badge tone={statusTone[skill.status]}>{skill.status}</Badge>
    </label>
  );
}

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "A";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
}
