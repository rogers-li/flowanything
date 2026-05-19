import { useEffect, useMemo, useState } from "react";
import { assistantTextFromDebugResult, runtimeProgressHeadline, type DebugChatMessage } from "../../components/AgentDebugChat";
import { debugSessionApi, resourceApi, runHistoryApi, runtimeApiV2 } from "../../platform/configApi";
import type { AgentConfig, RunRecord, SkillConfig, ToolConfig } from "../../platform/configTypes";
import { useConfigWorkspace } from "../../platform/ConfigWorkspaceProvider";
import { agentTraceFromTraceResponse } from "../../platform/traceViewModel";
import type { AgentDebugResponse, AgentDependencies, AgentProfile, AgentTrace, RuntimeEvent, SkillSpec, ToolSpec } from "../../types/platform";
import { useAutoDismissNotice, type NoticeState } from "../common/useAutoDismissNotice";
import { agentConfigFromDraft, agentProfileFromConfig, dependenciesForAgent, skillSpecFromConfig, toolSpecFromConfig } from "./configModel";
import { createAgentDraft, draftFromAgent, type AgentDraft } from "./domain";

type Notice = NoticeState;

type AgentDebugSessionRecord = {
  agentId: string;
  agentName: string;
  createdAt: string;
  messages: DebugChatMessage[];
  sessionId: string;
  updatedAt: string;
};

export function useAgents() {
  const workspace = useConfigWorkspace();
  const [agentItems, setAgentItems] = useState<AgentProfile[]>([]);
  const [skillItems, setSkillItems] = useState<SkillSpec[]>([]);
  const [toolItems, setToolItems] = useState<ToolSpec[]>([]);
  const [selectedAgentId, setSelectedAgentId] = useState("");
  const [draft, setDraft] = useState<AgentDraft>(() => createAgentDraft());
  const [debugMessage, setDebugMessage] = useState("帮我查一下深圳天气");
  const [debugSessionId, setDebugSessionId] = useState("");
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

  const selectedDependencies = useMemo(
    () => dependenciesForAgent(selectedAgent, skillItems, toolItems),
    [selectedAgent, skillItems, toolItems]
  );

  const dependenciesByAgent = useMemo<Record<string, AgentDependencies>>(
    () => Object.fromEntries(agentItems.map((agent) => [agent.id, dependenciesForAgent(agent, skillItems, toolItems)])),
    [agentItems, skillItems, toolItems]
  );

  useEffect(() => {
    void refresh();
  }, [workspace.activeBundleId]);

  useEffect(() => {
    if (!selectedAgent) {
      if (!selectedAgentId) setDraft(createAgentDraft());
      setDebugSessions([]);
      return;
    }
    setDraft(draftFromAgent(selectedAgent));
    resetDebugSession();
    void loadDebugHistory(selectedAgent.id);
  }, [selectedAgent?.id]);

  async function refresh() {
    if (!workspace.activeBundleId) {
      setAgentItems([]);
      setSkillItems([]);
      setToolItems([]);
      setSelectedAgentId("");
      return;
    }
    try {
      const [agents, skills, tools] = await Promise.all([
        resourceApi.listResourcesByKind<AgentConfig>(workspace.activeBundleId, "agent"),
        resourceApi.listResourcesByKind<SkillConfig>(workspace.activeBundleId, "skill"),
        resourceApi.listResourcesByKind<ToolConfig>(workspace.activeBundleId, "tool")
      ]);
      const nextAgents = agents.items.map((item) => agentProfileFromConfig(item.resource));
      const nextSkills = skills.items.map((item) => skillSpecFromConfig(item.resource));
      const nextTools = tools.items.map((item) => toolSpecFromConfig(item.resource));
      setAgentItems(nextAgents);
      setSkillItems(nextSkills);
      setToolItems(nextTools);
      setSelectedAgentId((current) => (nextAgents.some((agent) => agent.id === current) ? current : nextAgents[0]?.id ?? ""));
      if (nextAgents.length === 0) {
        setDraft(createAgentDraft());
      }
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to load Agents from active Bundle." });
    }
  }

  async function loadDebugHistory(agentId: string) {
    try {
      const response = await runHistoryApi.list();
      setDebugSessions(
        response.items
          .filter((record) => record.type === "agent" && record.agent_request?.agent_id === agentId)
          .map(debugSessionRecordFromRun)
      );
    } catch {
      setDebugSessions([]);
    }
  }

  async function saveDraft() {
    if (!workspace.activeBundleId) {
      setNotice({ ok: false, message: "Create or select a draft Bundle before saving Agents." });
      return;
    }
    try {
      const config = agentConfigFromDraft(draft);
      const response = await resourceApi.upsertResource(workspace.activeBundleId, "agent", config);
      const saved = response.bundle.resources?.agents?.find((agent) => agent.id === config.id) ?? config;
      const viewModel = agentProfileFromConfig(saved);
      upsertAgent(viewModel);
      setSelectedAgentId(viewModel.id);
      setDraft(draftFromAgent(viewModel));
      await workspace.refresh();
      setNotice({ ok: true, message: "Agent saved to draft Bundle." });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to save Agent." });
    }
  }

  async function enableSelected() {
    if (!selectedAgent) return;
    await changeDisabled(selectedAgent.id, false);
  }

  async function disableSelected() {
    if (!selectedAgent) return;
    await changeDisabled(selectedAgent.id, true);
  }

  async function changeDisabled(agentId: string, disabled: boolean) {
    if (!workspace.activeBundleId) return;
    try {
      const resource = await resourceApi.getResource<AgentConfig>(workspace.activeBundleId, "agent", agentId);
      const next = { ...resource.resource, disabled };
      await resourceApi.upsertResource(workspace.activeBundleId, "agent", next);
      const saved = agentProfileFromConfig(next);
      upsertAgent(saved);
      setDraft(draftFromAgent(saved));
      await workspace.refresh();
      setNotice({ ok: true, message: `Agent ${disabled ? "disabled" : "enabled"}.` });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to update Agent status." });
    }
  }

  async function runDebug() {
    if (!selectedAgent || isDebugRunning) return;
    if (!workspace.activeBundleId) {
      setNotice({ ok: false, message: "Select a draft Bundle before debugging Agents." });
      return;
    }

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
      pending: true
    };

    setDebugMessage("");
    setDebugChatMessages((current) => [...current, userMessage, liveMessage]);
    setIsDebugRunning(true);
    const stopTracePolling = startDebugTracePolling(traceId, liveMessageId);

    try {
      const sessionId = await ensureDebugSession(selectedAgent.id);
      const result = await debugSessionApi.runAgent(sessionId, {
        agent_id: selectedAgent.id,
        user_message: userText,
        conversation: conversationFromMessages(debugChatMessages),
        trace_context: {
          trace_id: traceId
        }
      });
      const response = agentDebugResponseFromRun(result.result.text, traceId);
      setDebugResult(response);

      let trace: AgentTrace | null = null;
      try {
        trace = agentTraceFromTraceResponse(await runtimeApiV2.getTrace(traceId));
        setDebugTrace(trace);
      } catch {
        setDebugTrace(null);
      }

      const assistantReply = assistantTextFromDebugResult(response);
      setDebugChatMessages((current) =>
        current.map((message) =>
          message.id === liveMessageId
            ? {
                ...message,
                text: assistantReply,
                traceId,
                trace,
                liveEvents: trace ? runtimeEventsFromTrace(trace) : message.liveEvents,
                pending: false
              }
            : message
        )
      );
      await loadDebugHistory(selectedAgent.id);
      setNotice({ ok: true, message: "Agent debug completed." });
    } catch (error) {
      setDebugResult(null);
      let trace: AgentTrace | null = null;
      try {
        trace = agentTraceFromTraceResponse(await runtimeApiV2.getTrace(traceId));
        setDebugTrace(trace);
      } catch {
        setDebugTrace(null);
      }
      setDebugChatMessages((current) =>
        current.map((message) =>
          message.id === liveMessageId
            ? {
                ...message,
                text: error instanceof Error ? error.message : "Failed to run Agent debug.",
                trace,
                liveEvents: trace ? runtimeEventsFromTrace(trace) : message.liveEvents,
                pending: false
              }
            : message
        )
      );
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to run Agent debug." });
    } finally {
      stopTracePolling();
      setIsDebugRunning(false);
    }
  }

  function startDebugTracePolling(traceId: string, liveMessageId: string): () => void {
    let stopped = false;
    let timer: number | undefined;

    const poll = async () => {
      if (stopped) return;
      try {
        const trace = agentTraceFromTraceResponse(await runtimeApiV2.getTrace(traceId));
        const liveEvents = runtimeEventsFromTrace(trace);
        setDebugTrace(trace);
        setDebugChatMessages((current) =>
          current.map((message) =>
            message.id === liveMessageId && message.pending
              ? {
                  ...message,
                  text: runtimeProgressHeadline(liveEvents),
                  trace,
                  liveEvents
                }
              : message
          )
        );
      } catch {
        // The trace is created asynchronously after the runtime receives the request.
      }
      if (!stopped) {
        timer = window.setTimeout(poll, 900);
      }
    };

    timer = window.setTimeout(poll, 400);
    return () => {
      stopped = true;
      if (timer !== undefined) {
        window.clearTimeout(timer);
      }
    };
  }

  async function ensureDebugSession(agentId: string): Promise<string> {
    if (debugSessionId) return debugSessionId;
    const response = await debugSessionApi.createSession({
      bundle_id: workspace.activeBundleId,
      entrypoint: {
        kind: "agent",
        id: agentId
      }
    });
    setDebugSessionId(response.session.id);
    return response.session.id;
  }

  function startNewAgent() {
    setSelectedAgentId("");
    setDraft(createAgentDraft());
    resetDebugSession();
  }

  function selectAgent(agentId: string) {
    setSelectedAgentId(agentId);
  }

  function resetDebugSession() {
    setDebugSessionId("");
    setDebugResult(null);
    setDebugTrace(null);
    setDebugChatMessages([]);
  }

  function restoreDebugSession(session: AgentDebugSessionRecord) {
    setDebugSessionId(session.sessionId);
    setDebugResult(null);
    setDebugTrace(null);
    setDebugChatMessages(session.messages);
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
    debugSessionId: debugSessionId || "new_preview_session",
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

function agentDebugResponseFromRun(text: string, traceId: string): AgentDebugResponse {
  return {
    eventId: createClientID("evt"),
    traceId,
    actions: [
      {
        type: "speak",
        text
      },
      {
        type: "end_turn"
      }
    ]
  };
}

function conversationFromMessages(messages: DebugChatMessage[]): Array<{ role: string; content: string }> {
  return messages
    .filter((message) => !message.pending && message.text.trim())
    .map((message) => ({
      role: message.role === "user" ? "user" : "assistant",
      content: message.text
    }));
}

function runtimeEventsFromTrace(trace: AgentTrace): RuntimeEvent[] {
  return trace.steps.map((step) => ({
    id: step.id,
    type: "trace_step_added",
    tenantId: trace.tenantId,
    traceId: trace.traceId,
    sessionId: trace.sessionId,
    stepId: step.id,
    stepType: step.type,
    name: step.name,
    status: step.status,
    payload: {
      step
    },
    createdAt: step.startedAt
  }));
}

function debugSessionRecordFromRun(record: RunRecord): AgentDebugSessionRecord {
  const userText = record.agent_request?.user_message ?? "";
  const assistantText = resultText(record.result) || record.error || "";
  const messages: DebugChatMessage[] = [
    {
      id: `${record.id}_user`,
      role: "user",
      text: userText
    }
  ];
  if (assistantText) {
    messages.push({
      id: `${record.id}_assistant`,
      role: "assistant",
      text: assistantText,
      traceId: record.trace_id,
      pending: false
    });
  }
  return {
    agentId: record.agent_request?.agent_id ?? record.entrypoint?.id ?? "",
    agentName: record.entrypoint?.id ?? "Agent",
    createdAt: record.started_at ?? "",
    messages,
    sessionId: record.session_id ?? "",
    updatedAt: record.finished_at ?? record.started_at ?? ""
  };
}

function resultText(result: unknown): string {
  if (!result || typeof result !== "object") return "";
  const value = result as { text?: unknown; result?: { text?: unknown } };
  if (typeof value.text === "string") return value.text;
  if (typeof value.result?.text === "string") return value.result.text;
  return "";
}
