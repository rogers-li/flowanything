import { useEffect, useMemo, useState } from "react";
import { assistantTextFromDebugResult, runtimeProgressHeadline, type DebugChatMessage } from "../../components/AgentDebugChat";
import { defaultTenantId, orchestratorApi, platformApi } from "../../lib/api";
import { agentDependencies, agents, skills, tools } from "../../lib/mockData";
import type { AgentDebugResponse, AgentDependencies, AgentProfile, AgentTrace, RuntimeEvent, SkillSpec, ToolSpec } from "../../types/platform";
import { useAutoDismissNotice, type NoticeState } from "../common/useAutoDismissNotice";
import { agentsClient } from "./api";
import { agentFromDraft, createAgentDraft, draftFromAgent, emptyAgentDependencies, type AgentDraft } from "./domain";

type Notice = NoticeState;
type AgentDebugSessionRecord = {
  agentId: string;
  agentName: string;
  createdAt: string;
  messages: DebugChatMessage[];
  sessionId: string;
  updatedAt: string;
};

const defaultDebugAgent = agents.find((agent) => agent.id === "agent_weather") ?? agents[0];
const agentDebugSessionStorageKey = "flow-anything.agent-debug-sessions.v1";
const maxAgentDebugSessionsPerAgent = 24;

export function useAgents() {
  const [agentItems, setAgentItems] = useState<AgentProfile[]>(agents);
  const [skillItems, setSkillItems] = useState<SkillSpec[]>(skills);
  const [toolItems, setToolItems] = useState<ToolSpec[]>(() => tools.filter((tool) => tool.status === "enabled"));
  const [selectedAgentId, setSelectedAgentId] = useState(defaultDebugAgent?.id ?? "");
  const [draft, setDraft] = useState<AgentDraft>(() => (defaultDebugAgent ? draftFromAgent(defaultDebugAgent) : createAgentDraft()));
  const [dependenciesByAgent, setDependenciesByAgent] = useState<Record<string, AgentDependencies>>(agentDependencies);
  const [debugMessage, setDebugMessage] = useState("帮我查一下深圳天气");
  const [debugSessionId, setDebugSessionId] = useState(() => createClientID("debug_session"));
  const [debugResult, setDebugResult] = useState<AgentDebugResponse | null>(null);
  const [debugTrace, setDebugTrace] = useState<AgentTrace | null>(null);
  const [debugChatMessages, setDebugChatMessages] = useState<DebugChatMessage[]>([]);
  const [debugSessions, setDebugSessions] = useState<AgentDebugSessionRecord[]>([]);
  const [isDebugRunning, setIsDebugRunning] = useState(false);
  const [notice, setNotice] = useAutoDismissNotice<Notice>();

  const selectedAgent = useMemo(
    () => agentItems.find((agent) => agent.id === selectedAgentId),
    [agentItems, selectedAgentId]
  );
  const selectedDependencies = selectedAgent
    ? dependenciesByAgent[selectedAgent.id] ?? emptyAgentDependencies(selectedAgent.id)
    : emptyAgentDependencies("");

  useEffect(() => {
    void refresh();
  }, []);

  useEffect(() => {
    if (!selectedAgent) {
      setDraft(createAgentDraft());
      setDebugSessions([]);
      return;
    }
    // Keep the editor draft aligned when the backend replaces the initial mock agent with the same id.
    setDraft(draftFromAgent(selectedAgent));
    resetDebugSession();
    setDebugSessions(readAgentDebugSessions(selectedAgent.id));
    void loadDependencies(selectedAgent.id);
  }, [selectedAgent]);

  useEffect(() => {
    if (!selectedAgent) return;
    persistAgentDebugSession(selectedAgent, debugSessionId, debugChatMessages);
    setDebugSessions(readAgentDebugSessions(selectedAgent.id));
  }, [debugChatMessages, debugSessionId, selectedAgent?.id, selectedAgent?.name]);

  async function refresh() {
    try {
      const [remoteAgents, remoteSkills, remoteTools] = await Promise.all([
        agentsClient.listAgents(),
        platformApi.listSkills().then((response) => response.items),
        platformApi.listTools({ status: "enabled" }).then((response) => response.items)
      ]);
      if (remoteAgents.length > 0) {
        setAgentItems(remoteAgents);
        setSelectedAgentId((current) => (remoteAgents.some((agent) => agent.id === current) ? current : remoteAgents[0].id));
      }
      if (remoteSkills.length > 0) {
        setSkillItems(remoteSkills);
      }
      setToolItems(remoteTools);
    } catch {
      setToolItems((current) => current.filter((tool) => tool.status === "enabled"));
      setNotice({ ok: false, message: "Using local agent mock data because backend APIs are unavailable." });
    }
  }

  async function loadDependencies(agentId: string) {
    try {
      const dependencies = await agentsClient.getDependencies(agentId);
      setDependenciesByAgent((current) => ({ ...current, [agentId]: dependencies }));
    } catch {
      setDependenciesByAgent((current) => ({
        ...current,
        [agentId]: current[agentId] ?? emptyAgentDependencies(agentId)
      }));
    }
  }

  async function saveDraft() {
    try {
      const agent = agentFromDraft(draft, defaultTenantId);
      const saved = await agentsClient.saveAgent(agent);
      upsertAgent(saved);
      setSelectedAgentId(saved.id);
      setDraft(draftFromAgent(saved));
      setNotice({ ok: true, message: "Agent saved." });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to save agent." });
    }
  }

  async function enableSelected() {
    if (!selectedAgent) return;
    await changeStatus(selectedAgent.id, "enabled");
  }

  async function disableSelected() {
    if (!selectedAgent) return;
    await changeStatus(selectedAgent.id, "disabled");
  }

  async function runDebug() {
    if (!selectedAgent) return;
    if (isDebugRunning) return;
    const submittedMessage = debugMessage;
    const userText = submittedMessage.trim() || "(empty message)";
    const traceId = createClientID("trace");
    const liveMessageId = createClientID("chat_live");
    const userMessage: DebugChatMessage = {
      id: createClientID("chat_user"),
      role: "user",
      text: userText
    };
    const liveMessage: DebugChatMessage = {
      id: liveMessageId,
      role: "assistant",
      text: "正在分析请求...",
      traceId,
      liveEvents: [],
      pending: true
    };
    setDebugMessage("");
    setDebugChatMessages((current) => [...current, userMessage, liveMessage]);
    setIsDebugRunning(true);
    const stopLiveEvents = orchestratorApi.subscribeLiveEvents(traceId, (event) => {
      if (event.type === "connected") return;
      setDebugChatMessages((current) =>
        current.map((message) => {
          if (message.id !== liveMessageId) return message;
          const liveEvents = [...(message.liveEvents ?? []), event];
          return {
            ...message,
            text: liveDebugText(liveEvents),
            liveEvents
          };
        })
      );
    });
    try {
      const result = await agentsClient.debugAgent({
        tenantId: defaultTenantId,
        traceId,
        userId: "debug_user",
        sessionId: debugSessionId,
        agentId: selectedAgent.id,
        type: "user_message_committed",
        channel: "text",
        payload: {
          text: userText
        },
        occurredAt: new Date().toISOString()
      });
      setDebugResult(result);
      let trace: AgentTrace | null = null;
      try {
        trace = await agentsClient.getTrace(result.traceId);
        setDebugTrace(trace);
      } catch {
        setDebugTrace(null);
      }
      const assistantReply = assistantTextFromDebugResult(result);
      setDebugChatMessages((current) =>
        [
          ...current.map((message) =>
            message.id === liveMessageId
              ? {
                  ...message,
                  text: processMessageText(message.liveEvents),
                  traceId: result.traceId,
                  trace,
                  pending: false
                }
              : message
          ),
          {
            id: createClientID("chat_agent"),
            role: "assistant" as const,
            text: assistantReply,
            traceId: result.traceId,
            trace,
            pending: false
          }
        ]
      );
      setNotice({ ok: true, message: "Agent debug event completed." });
    } catch (error) {
      setDebugResult(null);
      setDebugTrace(null);
      setDebugChatMessages((current) =>
        current.map((message) =>
          message.id === liveMessageId
            ? {
                ...message,
                text: error instanceof Error ? error.message : "Failed to run agent debug event.",
                pending: false
              }
            : message
        )
      );
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to run agent debug event." });
    } finally {
      stopLiveEvents();
      setIsDebugRunning(false);
    }
  }

  async function changeStatus(agentId: string, status: AgentProfile["status"]) {
    try {
      const saved = status === "enabled" ? await agentsClient.enableAgent(agentId) : await agentsClient.disableAgent(agentId);
      upsertAgent(saved);
      setDraft(draftFromAgent(saved));
      setNotice({ ok: true, message: `Agent ${status}.` });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : `Failed to mark agent ${status}.` });
    }
  }

  function startNewAgent() {
    setSelectedAgentId("");
    setDraft(createAgentDraft());
  }

  function selectAgent(agentId: string) {
    setSelectedAgentId(agentId);
  }

  function resetDebugSession() {
    setDebugSessionId(createClientID("debug_session"));
    setDebugResult(null);
    setDebugTrace(null);
    setDebugChatMessages([]);
  }

  function restoreDebugSession(session: AgentDebugSessionRecord) {
    setDebugSessionId(session.sessionId);
    setDebugResult(null);
    setDebugTrace(null);
    setDebugChatMessages(messagesForHistory(session.messages));
  }

  function upsertAgent(agent: AgentProfile) {
    setAgentItems((current) => {
      const exists = current.some((item) => item.id === agent.id);
      if (exists) {
        return current.map((item) => (item.id === agent.id ? agent : item));
      }
      return [agent, ...current];
    });
  }

  return {
    agents: agentItems,
    skills: skillItems,
    tools: toolItems,
    selectedAgent,
    selectedDependencies,
    dependenciesByAgent,
    draft,
    setDraft,
    debugMessage,
    setDebugMessage,
    debugSessionId,
    debugResult,
    debugTrace,
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
  };
}

function createClientID(prefix: string): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return `${prefix}_${crypto.randomUUID().replaceAll("-", "").slice(0, 12)}`;
  }
  return `${prefix}_${Date.now()}`;
}

function liveDebugText(events: RuntimeEvent[]): string {
  return runtimeProgressHeadline(events);
}

function processMessageText(events?: RuntimeEvent[]): string {
  if (!events || events.length === 0) return "处理过程";
  if (events.some((event) => event.type === "run_failed")) return "处理过程：执行失败。";
  if (events.some((event) => event.type === "run_completed")) return "处理过程：执行完成。";
  return "处理过程";
}

function isAgentDebugSessionRecord(value: unknown): value is AgentDebugSessionRecord {
  if (!value || typeof value !== "object") return false;
  const record = value as Partial<AgentDebugSessionRecord>;
  return (
    typeof record.agentId === "string" &&
    typeof record.agentName === "string" &&
    typeof record.createdAt === "string" &&
    Array.isArray(record.messages) &&
    typeof record.sessionId === "string" &&
    typeof record.updatedAt === "string"
  );
}

function readAllAgentDebugSessions(): AgentDebugSessionRecord[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(agentDebugSessionStorageKey);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    return Array.isArray(parsed)
      ? parsed.filter(isAgentDebugSessionRecord).map((session) => ({ ...session, messages: messagesForHistory(session.messages) }))
      : [];
  } catch {
    return [];
  }
}

function readAgentDebugSessions(agentId: string): AgentDebugSessionRecord[] {
  return readAllAgentDebugSessions()
    .filter((session) => session.agentId === agentId)
    .sort((left, right) => right.updatedAt.localeCompare(left.updatedAt));
}

function messagesForHistory(messages: DebugChatMessage[]): DebugChatMessage[] {
  const normalized = messages.map((message) => ({
    id: message.id,
    role: message.role,
    text: message.text,
    traceId: message.traceId,
    liveEvents: message.liveEvents,
    // Restored history is no longer streaming, so show the full progress list.
    pending: false
  }));
  const withRecoveredReplies: DebugChatMessage[] = [];
  for (const message of normalized) {
    withRecoveredReplies.push(message);
    const recoveredReply = finalAssistantTextFromEvents(message.liveEvents);
    if (!message.traceId || !recoveredReply) continue;
    const alreadyExists = normalized.some(
      (candidate) =>
        candidate.role === "assistant" &&
        candidate.traceId === message.traceId &&
        candidate.text.trim() === recoveredReply
    );
    if (alreadyExists) continue;
    withRecoveredReplies.push({
      id: `${message.id}_recovered_reply`,
      role: "assistant",
      text: recoveredReply,
      traceId: message.traceId,
      pending: false
    });
  }
  return withRecoveredReplies;
}

function persistAgentDebugSession(agent: AgentProfile, sessionId: string, messages: DebugChatMessage[]) {
  if (typeof window === "undefined" || messages.length === 0) return;

  const now = new Date().toISOString();
  const allSessions = readAllAgentDebugSessions();
  const existing = allSessions.find((session) => session.agentId === agent.id && session.sessionId === sessionId);
  const nextRecord: AgentDebugSessionRecord = {
    agentId: agent.id,
    agentName: agent.name,
    createdAt: existing?.createdAt ?? now,
    messages: messagesForHistory(messages),
    sessionId,
    updatedAt: now
  };
  const merged = [
    nextRecord,
    ...allSessions.filter((session) => !(session.agentId === agent.id && session.sessionId === sessionId))
  ].sort((left, right) => right.updatedAt.localeCompare(left.updatedAt));
  const keptCurrentAgentSessionIds = new Set(
    merged
      .filter((session) => session.agentId === agent.id)
      .slice(0, maxAgentDebugSessionsPerAgent)
      .map((session) => session.sessionId)
  );

  try {
    window.localStorage.setItem(
      agentDebugSessionStorageKey,
      JSON.stringify(merged.filter((session) => session.agentId !== agent.id || keptCurrentAgentSessionIds.has(session.sessionId)))
    );
  } catch {
    // Debug history is local convenience data. Agent execution should not depend on browser storage.
  }
}

function finalAssistantTextFromEvents(events?: RuntimeEvent[]): string {
  if (!events?.some((event) => event.type === "run_completed")) return "";
  for (let index = events.length - 1; index >= 0; index -= 1) {
    const event = events[index];
    if (event.type !== "assistant_message_completed") continue;
    return stringFromUnknown(event.payload?.text);
  }
  return "";
}

function stringFromUnknown(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}
