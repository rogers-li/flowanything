import { useEffect, useId, useMemo, useRef, useState, type ReactNode } from "react";
import {
  Background,
  Controls,
  Handle,
  MarkerType,
  MiniMap,
  Position,
  ReactFlow,
  addEdge,
  applyEdgeChanges,
  applyNodeChanges,
  type Connection,
  type EdgeChange,
  type NodeChange,
  type NodeProps
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { ChatBubble, runtimeProgressHeadline, TraceInspector, type DebugChatMessage } from "../components/AgentDebugChat";
import { Badge } from "../components/Badge";
import { FlowDeletableEdge } from "../components/FlowDeletableEdge";
import { PromptRichEditor, type PromptAgentMention } from "../components/PromptRichEditor";
import { ToolSelector } from "../components/ToolSelector";
import { useConfigWorkspace } from "../platform/ConfigWorkspaceProvider";
import { debugSessionApi, resourceApi, runHistoryApi, runtimeApiV2 } from "../platform/configApi";
import type { AgentConfig, ConnectorConfig, RunRecord, SkillConfig, ToolConfig, WorkflowConfig, WorkflowRunResponse as RuntimeWorkflowRunResponse } from "../platform/configTypes";
import { agentTraceFromTraceResponse } from "../platform/traceViewModel";
import type {
  AgentFlowNodeType,
  AgentFlowRunResponse,
  AgentFlowSpec,
  AgentProfile,
  AgentTrace,
  ConnectorOperation,
  RuntimeEvent,
  TraceStep,
  SkillSpec,
  ToolSpec,
  WorkflowNode
} from "../types/platform";
import { useAutoDismissNotice, type NoticeState } from "../features/common/useAutoDismissNotice";
import {
  canvasToGraph,
  agentFlowInputSchema,
  agentFlowOutputSchema,
  createAgentFlowDraft,
  createFlowNode,
  createSupervisorAgentFlowDraft,
  existingAgentLeafValidationMessage,
  flowToCanvasEdges,
  flowToCanvasNodes,
  isExistingAgentCanvasNode,
  nodeHasOutgoingEdges,
  nodeDescription,
  nodeLabel,
  withGraph,
  type AgentFlowCanvasEdge,
  type AgentFlowCanvasNode,
  type AgentFlowCanvasNodeData,
  type LocalAgentConfig
} from "../features/agentflows/domain";
import {
  applyNodeConfigRows,
  contextFieldsFromSchema,
  contextSchemaFromFields,
  createContextField,
  createMappingRow,
  mappingRowsFromRecord,
  nodeConfigRows,
  nodeTypeLabel,
  recordFromMappingRows,
  type ContextFieldDraft,
  type MappingRowDraft,
  type NodeConfigRows
} from "../features/workflows/domain";
import { transformFunctionById, transformFunctionDefinitions } from "../features/workflows/transformFunctions";
import { agentProfileFromConfig, skillSpecFromConfig, toolSpecFromConfig } from "../features/agents/configModel";
import { connectorOperationFromConfig } from "../features/connectors/configModel";
import { agentFlowFromWorkflowConfig, isAgentFlowWorkflowConfig, workflowConfigFromAgentFlow } from "../features/agentflows/configModel";

type Notice = NoticeState;
type AgentFlowPageMode = "list" | "editor";
type FlowCreateType = "supervisor" | "workflow";
type MappingOption = { value: string; label: string; detail?: string };
type ConditionRuleDraft = {
  id: string;
  left: string;
  operator: string;
  right: string;
};
type ConditionBranchDraft = {
  id: string;
  name: string;
  mode: "all" | "any";
  rules: ConditionRuleDraft[];
  writeRows: MappingRowDraft[];
  nextNodeId: string;
};
type ConditionDefaultBranchDraft = {
  writeRows: MappingRowDraft[];
  nextNodeId: string;
};
type FlowTestSessionRecord = {
  createdAt: string;
  flowKey: string;
  flowName: string;
  messages: DebugChatMessage[];
  sessionId: string;
  updatedAt: string;
};

const flowTestSessionStorageKey = "flow-anything.agent-flow-test-sessions.v1";
const maxFlowTestSessionsPerFlow = 24;

const flowStatusTone = {
  draft: "gray",
  enabled: "green",
  disabled: "red"
} as const;

const flowStatusCopy = {
  draft: "Draft",
  enabled: "Live",
  disabled: "Paused"
} as const;

const nodeTypes = {
  agentFlowNode: FlowNode
};

const edgeTypes = {
  deletableEdge: FlowDeletableEdge
};

const fixedAgentFlowInputFields = contextFieldsFromSchema(agentFlowInputSchema);
const fixedAgentFlowOutputFields = contextFieldsFromSchema(agentFlowOutputSchema);

const palette: Array<{ type: AgentFlowNodeType; label: string }> = [
  { type: "agent_node", label: "Agent" },
  { type: "router_node", label: "Router" },
  { type: "aggregator_node", label: "Aggregator" },
  { type: "verifier_node", label: "Verifier" },
  { type: "join_node", label: "Join" }
];

const workflowPalette: Array<{ type: AgentFlowNodeType; label: string }> = [
  { type: "tool", label: "Tool" },
  { type: "agent_node", label: "Agent" },
  { type: "transform", label: "Transform" },
  { type: "condition", label: "Condition" },
  { type: "join", label: "Join" },
  { type: "end", label: "End" }
];

const conditionOperatorOptions = [
  { value: "equals", label: "equals" },
  { value: "not_equals", label: "not equals" },
  { value: "contains", label: "contains" },
  { value: "not_contains", label: "not contains" },
  { value: "starts_with", label: "starts with" },
  { value: "ends_with", label: "ends with" },
  { value: "greater_than", label: "greater than" },
  { value: "greater_or_equals", label: "greater or equals" },
  { value: "less_than", label: "less than" },
  { value: "less_or_equals", label: "less or equals" },
  { value: "exists", label: "exists" },
  { value: "not_exists", label: "not exists" },
  { value: "is_empty", label: "is empty" },
  { value: "is_not_empty", label: "is not empty" },
  { value: "in", label: "in" },
  { value: "not_in", label: "not in" },
  { value: "regex_match", label: "regex match" }
];

function isAgentLikeNode(nodeType: AgentFlowNodeType): boolean {
  return nodeType === "agent_node" || nodeType === "supervisor_node" || nodeType === "agent";
}

function flowTestSessionFlowKey(flow: AgentFlowSpec): string {
  return flow.id || flow.graph.id || flow.name || "draft_agent_flow";
}

function isFlowTestSessionRecord(value: unknown): value is FlowTestSessionRecord {
  if (!value || typeof value !== "object") return false;
  const record = value as Partial<FlowTestSessionRecord>;
  return (
    typeof record.createdAt === "string" &&
    typeof record.flowKey === "string" &&
    typeof record.flowName === "string" &&
    Array.isArray(record.messages) &&
    typeof record.sessionId === "string" &&
    typeof record.updatedAt === "string"
  );
}

function readAllFlowTestSessions(): FlowTestSessionRecord[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(flowTestSessionStorageKey);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    return Array.isArray(parsed)
      ? parsed.filter(isFlowTestSessionRecord).map((session) => ({ ...session, messages: messagesForHistory(session.messages) }))
      : [];
  } catch {
    return [];
  }
}

function readFlowTestSessions(flow: AgentFlowSpec): FlowTestSessionRecord[] {
  const flowKey = flowTestSessionFlowKey(flow);
  return readAllFlowTestSessions()
    .filter((session) => session.flowKey === flowKey)
    .sort((left, right) => right.updatedAt.localeCompare(left.updatedAt));
}

function persistFlowTestSession(flow: AgentFlowSpec, sessionId: string, messages: DebugChatMessage[]) {
  if (typeof window === "undefined" || messages.length === 0) return;

  const flowKey = flowTestSessionFlowKey(flow);
  const now = new Date().toISOString();
  const historyMessages = completedFlowTestMessagesForHistory(messages);
  if (historyMessages.length === 0) return;
  const allSessions = readAllFlowTestSessions();
  const existing = allSessions.find((session) => session.flowKey === flowKey && session.sessionId === sessionId);
  const messagesChanged = existing ? flowTestMessagesKey(existing.messages) !== flowTestMessagesKey(historyMessages) : true;
  const nextRecord: FlowTestSessionRecord = {
    createdAt: existing?.createdAt ?? now,
    flowKey,
    flowName: flow.name || flow.graph.name || "Untitled Agent Flow",
    messages: historyMessages,
    sessionId,
    updatedAt: messagesChanged ? now : existing?.updatedAt ?? now
  };

  const merged = [
    nextRecord,
    ...allSessions.filter((session) => !(session.flowKey === flowKey && session.sessionId === sessionId))
  ].sort((left, right) => right.updatedAt.localeCompare(left.updatedAt));
  const keptCurrentFlowSessionIds = new Set(
    merged
      .filter((session) => session.flowKey === flowKey)
      .slice(0, maxFlowTestSessionsPerFlow)
      .map((session) => session.sessionId)
  );

  try {
    window.localStorage.setItem(
      flowTestSessionStorageKey,
      JSON.stringify(merged.filter((session) => session.flowKey !== flowKey || keptCurrentFlowSessionIds.has(session.sessionId)))
    );
  } catch {
    // Local history is convenience data. Runtime execution must not fail if browser storage is unavailable.
  }
}

function formatFlowTestSessionTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat(undefined, {
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    month: "short"
  }).format(date);
}

function flowTestSessionPreview(messages: DebugChatMessage[]): string {
  const message = messages.find((item) => item.role === "user") ?? messages[0];
  if (!message) return "No messages";
  return message.text.length > 72 ? `${message.text.slice(0, 72)}...` : message.text;
}

function messagesForHistory(messages: DebugChatMessage[]): DebugChatMessage[] {
  return messages.map((message) => ({
    id: message.id,
    role: message.role,
    text: message.text,
    traceId: message.traceId,
    liveEvents: message.liveEvents,
    pending: false
  }));
}

function completedFlowTestMessagesForHistory(messages: DebugChatMessage[]): DebugChatMessage[] {
  const compactMessages = messagesForHistory(messages).filter((message) => !isFlowProgressMessage(message));
  let lastUserIndex = -1;
  for (let index = compactMessages.length - 1; index >= 0; index -= 1) {
    if (compactMessages[index]?.role === "user") {
      lastUserIndex = index;
      break;
    }
  }
  if (lastUserIndex < 0) return [];
  const hasAssistantAfterLastUser = compactMessages.slice(lastUserIndex + 1).some((message) => message.role === "assistant" && message.text.trim());
  return hasAssistantAfterLastUser ? compactMessages : [];
}

function isFlowProgressMessage(message: DebugChatMessage): boolean {
  if (message.role !== "assistant") return false;
  if (message.pending) return true;
  if ((message.liveEvents?.length ?? 0) > 0) return true;
  const text = message.text.trim();
  return (
    text === "正在执行 Agent Flow..." ||
    text === "正在执行Agent Flow" ||
    text === "正在执行 Flow..." ||
    text === "Agent Flow 执行完成。" ||
    text === "Agent 正在处理..." ||
    text === "处理完成。" ||
    text.startsWith("正在分析请求") ||
    text.startsWith("正在调用") ||
    text.startsWith("计划调用") ||
    text.startsWith("处理过程：")
  );
}

function flowTestMessagesKey(messages: DebugChatMessage[]): string {
  return JSON.stringify(messagesForHistory(messages));
}

type AgentFlowsPageProps = {
  onExit?: () => void;
};

export function AgentFlowsPage({ onExit }: AgentFlowsPageProps = {}) {
  const workspace = useConfigWorkspace();
  const [pageMode, setPageMode] = useState<AgentFlowPageMode>("list");
  const [flows, setFlows] = useState<AgentFlowSpec[]>([]);
  const [workflowConfigs, setWorkflowConfigs] = useState<WorkflowConfig[]>([]);
  const [agents, setAgents] = useState<AgentProfile[]>([]);
  const [skills, setSkills] = useState<SkillSpec[]>([]);
  const [tools, setTools] = useState<ToolSpec[]>([]);
  const [connectorOperations, setConnectorOperations] = useState<ConnectorOperation[]>([]);
  const [selectedFlow, setSelectedFlow] = useState<AgentFlowSpec>(() => createAgentFlowDraft());
  const [nodes, setNodes] = useState<AgentFlowCanvasNode[]>(() => flowToCanvasNodes(createAgentFlowDraft().graph));
  const [edges, setEdges] = useState<AgentFlowCanvasEdge[]>([]);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>("start");
  const [flowTestInput, setFlowTestInput] = useState("帮我搜索一下今天的 AI 新闻，并总结重点");
  const [flowTestMessages, setFlowTestMessages] = useState<DebugChatMessage[]>([]);
  const [flowTestSessions, setFlowTestSessions] = useState<FlowTestSessionRecord[]>([]);
  const [isFlowTestHistoryOpen, setIsFlowTestHistoryOpen] = useState(false);
  const [flowDebugSessionId, setFlowDebugSessionId] = useState(() => createClientID("flow_debug_session"));
  const restoredFlowHistorySnapshotRef = useRef<{ messagesKey: string; sessionId: string } | null>(null);
  const [activeFlowTrace, setActiveFlowTrace] = useState<AgentTrace | null>(null);
  const [nodeTestInput, setNodeTestInput] = useState("帮我搜索一下今天的 AI 新闻，并总结重点");
  const [nodeTestMessages, setNodeTestMessages] = useState<DebugChatMessage[]>([]);
  const [nodeDebugSessionId, setNodeDebugSessionId] = useState(() => createClientID("flow_node_debug_session"));
  const [activeNodeTrace, setActiveNodeTrace] = useState<AgentTrace | null>(null);
  const [isFlowRunning, setIsFlowRunning] = useState(false);
  const [isNodeRunning, setIsNodeRunning] = useState(false);
  const [isRightPanelOpen, setIsRightPanelOpen] = useState(false);
  const [isNodeConfigWorkspaceOpen, setIsNodeConfigWorkspaceOpen] = useState(false);
  const [isFlowSettingsOpen, setIsFlowSettingsOpen] = useState(false);
  const [isFlowTestOpen, setIsFlowTestOpen] = useState(false);
  const [isCtxSettingsOpen, setIsCtxSettingsOpen] = useState(false);
  const [isCreateFlowOpen, setIsCreateFlowOpen] = useState(false);
  const [flowInputFields, setFlowInputFields] = useState<ContextFieldDraft[]>(() => fixedAgentFlowInputFields);
  const [flowOutputFields, setFlowOutputFields] = useState<ContextFieldDraft[]>(() => fixedAgentFlowOutputFields);
  const [variableFields, setVariableFields] = useState<ContextFieldDraft[]>(() => contextFieldsFromSchema(createAgentFlowDraft().contextSchema));
  const [notice, setNotice] = useAutoDismissNotice<Notice>();

  const selectedNode = useMemo(
    () => nodes.find((node) => node.id === selectedNodeId),
    [nodes, selectedNodeId]
  );
  const selectedAgent = useMemo(
    () => agents.find((agent) => agent.id === selectedNode?.data.agentId),
    [agents, selectedNode?.data.agentId]
  );
  const selectedNodeHasChildren = useMemo(
    () => (selectedNode ? nodeHasOutgoingEdges(selectedNode.id, edges) : false),
    [edges, selectedNode]
  );
  const selectedChildNodeIds = useMemo(() => {
    if (!selectedNode) return [];
    return edges.filter((edge) => edge.source === selectedNode.id).map((edge) => edge.target);
  }, [edges, nodes, selectedNode]);
  const canvasAgentMentions = useMemo<PromptAgentMention[]>(
    () =>
      nodes
        .filter((node) => node.id !== selectedNode?.id && isAgentLikeNode(node.data.nodeType))
        .map((node) => ({
          id: node.id,
          name: node.data.label || node.id,
          description: node.data.description,
          meta: node.data.agentMode === "local" ? `Local Agent · ${node.data.localAgent?.model || "model"}` : "Existing Agent",
          bound: selectedChildNodeIds.includes(node.id)
        })),
    [nodes, selectedChildNodeIds, selectedNode?.id]
  );
  const currentGraph = useMemo(
    () => canvasToGraph(selectedFlow, nodes, edges),
    [selectedFlow, nodes, edges]
  );
  const currentFlow = useMemo(
    () => ({
      ...withGraph(selectedFlow, currentGraph),
      inputSchema: agentFlowInputSchema,
      outputSchema: agentFlowOutputSchema,
      contextSchema: contextSchemaFromFields(variableFields)
    }),
    [selectedFlow, currentGraph, variableFields]
  );
  const contextReadOptions = useMemo(
    () => contextReadOptionsForAgentFlow(flowInputFields, flowOutputFields, variableFields, nodes),
    [flowInputFields, flowOutputFields, variableFields, nodes]
  );
  const contextWriteOptions = useMemo(
    () => contextWriteOptionsForAgentFlow(flowOutputFields, variableFields),
    [flowOutputFields, variableFields]
  );

  useEffect(() => {
    void loadInitialData();
  }, [workspace.activeBundleId]);

  useEffect(() => {
    resetNodeDebugSession();
  }, [selectedNodeId]);

  useEffect(() => {
    setFlowTestSessions(readFlowTestSessions(selectedFlow));
  }, [selectedFlow.graph.id, selectedFlow.id]);

  useEffect(() => {
    const restoredSnapshot = restoredFlowHistorySnapshotRef.current;
    if (
      restoredSnapshot &&
      restoredSnapshot.sessionId === flowDebugSessionId &&
      restoredSnapshot.messagesKey === flowTestMessagesKey(flowTestMessages)
    ) {
      restoredFlowHistorySnapshotRef.current = null;
      setFlowTestSessions(readFlowTestSessions(selectedFlow));
      return;
    }
    persistFlowTestSession(selectedFlow, flowDebugSessionId, flowTestMessages);
    setFlowTestSessions(readFlowTestSessions(selectedFlow));
  }, [flowDebugSessionId, flowTestMessages, selectedFlow.graph.id, selectedFlow.id, selectedFlow.name]);

  async function loadInitialData() {
    if (!workspace.activeBundleId) {
      setFlows([]);
      setWorkflowConfigs([]);
      setAgents([]);
      setSkills([]);
      setTools([]);
      setConnectorOperations([]);
      return;
    }

    try {
      const [workflowResponse, agentResponse, skillResponse, toolResponse, connectorResponse] = await Promise.all([
        resourceApi.listResourcesByKind<WorkflowConfig>(workspace.activeBundleId, "workflow"),
        resourceApi.listResourcesByKind<AgentConfig>(workspace.activeBundleId, "agent"),
        resourceApi.listResourcesByKind<SkillConfig>(workspace.activeBundleId, "skill"),
        resourceApi.listResourcesByKind<ToolConfig>(workspace.activeBundleId, "tool"),
        resourceApi.listResourcesByKind<ConnectorConfig>(workspace.activeBundleId, "connector")
      ]);

      const nextWorkflowConfigs = workflowResponse.items.map((item) => item.resource).filter(isAgentFlowWorkflowConfig);
      setWorkflowConfigs(nextWorkflowConfigs);
      setFlows(nextWorkflowConfigs.map(agentFlowFromWorkflowConfig));
      setAgents(agentResponse.items.map((item) => agentProfileFromConfig(item.resource)));
      setSkills(skillResponse.items.map((item) => skillSpecFromConfig(item.resource)));
      setTools(toolResponse.items.map((item) => toolSpecFromConfig(item.resource)).filter((tool) => tool.status === "enabled"));
      setConnectorOperations(
        connectorResponse.items.flatMap((item) => (item.resource.operations ?? []).map((operation) => connectorOperationFromConfig(item.resource, operation)))
      );
    } catch (error) {
      setNotice({
        ok: false,
        message: error instanceof Error ? error.message : "Failed to load Agent Flow resources."
      });
    }
  }

  function openFlow(flow: AgentFlowSpec, clearDebug = true, nextSelectedNodeId?: string | null) {
    setSelectedFlow(flow);
    setNodes(flowToCanvasNodes(flow.graph));
    setEdges(flowToCanvasEdges(flow.graph));
    setSelectedNodeId(nextSelectedNodeId ?? (flow.graph.entryNodeId || "start"));
    setFlowInputFields(fixedAgentFlowInputFields);
    setFlowOutputFields(fixedAgentFlowOutputFields);
    setVariableFields(contextFieldsFromSchema(flow.contextSchema));
    setPageMode("editor");
    if (clearDebug) {
      resetFlowDebugSession();
      resetNodeDebugSession();
    }
  }

  function startNewFlow() {
    setIsCreateFlowOpen(true);
  }

  function createFlowByType(type: FlowCreateType) {
    const draft = type === "supervisor" ? createSupervisorAgentFlowDraft() : createAgentFlowDraft();
    openFlow(draft);
    setSelectedNodeId(type === "supervisor" ? "supervisor" : "start");
    setIsRightPanelOpen(type === "supervisor");
    setIsFlowSettingsOpen(false);
    setIsCreateFlowOpen(false);
  }

  function returnToList() {
    setPageMode("list");
    setIsRightPanelOpen(false);
    setIsNodeConfigWorkspaceOpen(false);
    setIsFlowSettingsOpen(false);
    setIsFlowTestOpen(false);
    setIsCtxSettingsOpen(false);
    resetFlowDebugSession();
    resetNodeDebugSession();
  }

  function addNode(nodeType: AgentFlowNodeType) {
    const flowNode = createFlowNode(nodeType, nodes.length);
    const graph = {
      ...selectedFlow.graph,
      nodes: {
        [flowNode.id]: flowNode
      }
    };
    const [canvasNode] = flowToCanvasNodes(graph);
    setNodes((current) => [...current, canvasNode]);
    setSelectedNodeId(flowNode.id);
    setIsRightPanelOpen(true);
  }

  function deleteNode(nodeId: string) {
    if (nodeId === "start") return;
    setNodes((current) => current.filter((node) => node.id !== nodeId));
    setEdges((current) => current.filter((edge) => edge.source !== nodeId && edge.target !== nodeId));
    if (selectedNodeId === nodeId) {
      setSelectedNodeId(null);
      setIsRightPanelOpen(false);
      setIsNodeConfigWorkspaceOpen(false);
      resetNodeDebugSession();
    }
  }

  function deleteEdge(edgeId: string) {
    setEdges((current) => current.filter((edge) => edge.id !== edgeId));
  }

  function updateSelectedFlow(patch: Partial<AgentFlowSpec>) {
    setSelectedFlow((current) => ({
      ...current,
      ...patch,
      graph: {
        ...current.graph,
        name: patch.name ?? current.graph.name,
        description: patch.description ?? current.graph.description,
        status: patch.status ?? current.graph.status
      }
    }));
  }

  function updateSelectedNodeData(patch: Partial<AgentFlowCanvasNodeData>) {
    if (!selectedNodeId) return;
    setNodes((current) =>
      current.map((node) =>
        node.id === selectedNodeId
          ? {
              ...node,
              data: {
                ...node.data,
                ...patch
              }
            }
          : node
      )
    );
  }

  function updateLocalAgent(patch: Partial<LocalAgentConfig>) {
    const fallback = localAgentDefaults(selectedNode?.data.label || "Local Agent");
    const localAgent = {
      ...fallback,
      ...(selectedNode?.data.localAgent ?? {}),
      ...patch
    };
    updateSelectedNodeData({
      agentMode: "local",
      label: localAgent.name || selectedNode?.data.label || "Local Agent",
      description: localAgent.description,
      localAgent
    });
  }

  function updateSelectedWorkflowNodeConfig(key: string, value: unknown) {
    updateSelectedNodeData({
      config: {
        ...(selectedNode?.data.config ?? {}),
        [key]: value
      }
    });
  }

  function updateSelectedWorkflowNodeRows(kind: keyof NodeConfigRows, rows: MappingRowDraft[]) {
    if (!selectedNode) return;
    const workflowNode = workflowNodeFromCanvasNode(selectedNode);
    const nextNode = applyNodeConfigRows(workflowNode, { ...nodeConfigRows(workflowNode), [kind]: rows });
    updateSelectedNodeData({ config: nextNode.config });
  }

  function bindMentionedAgent(agentOrNodeId: string) {
    if (!selectedNode || !isAgentLikeNode(selectedNode.data.nodeType)) return;
    if (selectedFlow.orchestrationMode === "supervisor" && isExistingAgentCanvasNode(selectedNode)) {
      setNotice({
        ok: false,
        message: "Existing Agent nodes must be leaf nodes. Switch this node to Local Agent before adding Sub-Agents."
      });
      return;
    }

    const existingTarget = nodes.find((node) => node.id === agentOrNodeId || node.data.agentId === agentOrNodeId);
    const agent = agents.find((item) => item.id === agentOrNodeId);
    if (!existingTarget && !agent) {
      setNotice({ ok: false, message: "Agent not found." });
      return;
    }

    const targetNodeId = existingTarget?.id ?? createClientID("agent_node");
    if (selectedNode.id === targetNodeId || wouldCreateCycle(selectedNode.id, targetNodeId, edges)) {
      setNotice({ ok: false, message: "This Agent mention would create a cycle in the Agent Graph." });
      return;
    }

    if (!existingTarget) {
      const siblingIndex = edges.filter((edge) => edge.source === selectedNode.id).length;
      const targetNode: AgentFlowCanvasNode = {
        id: targetNodeId,
        type: "agentFlowNode",
        position: {
          x: selectedNode.position.x + 300,
          y: selectedNode.position.y + siblingIndex * 150
        },
        data: {
          label: agent?.name ?? "Agent",
          description: agent?.description,
          nodeType: "agent_node",
          agentId: agent?.id,
          agentMode: "existing"
        }
      };
      setNodes((current) => [...current, targetNode]);
    }

    setEdges((current) => {
      if (current.some((edge) => edge.source === selectedNode.id && edge.target === targetNodeId)) return current;
      return addEdge(
        {
          id: `${selectedNode.id}-${targetNodeId}`,
          source: selectedNode.id,
          target: targetNodeId,
          type: "smoothstep",
          markerEnd: { type: MarkerType.ArrowClosed }
        },
        current
      );
    });
  }

  async function saveFlow(nextSelectedNodeId = selectedNodeId) {
    try {
      if (!workspace.activeBundleId) {
        throw new Error("No active config bundle selected.");
      }
      if (selectedFlow.orchestrationMode === "supervisor") {
        const validationMessage = existingAgentLeafValidationMessage(nodes, edges);
        if (validationMessage) {
          setNotice({ ok: false, message: validationMessage });
          return null;
        }
      }
      const normalized = withGraph(currentFlow, currentGraph);
      const current = workflowConfigs.find((item) => item.id === normalized.id);
      const config = workflowConfigFromAgentFlow(normalized, current, { agents, skills, tools });
      await resourceApi.upsertResource(workspace.activeBundleId, "workflow", config);
      const saved = agentFlowFromWorkflowConfig(config);
      upsertWorkflowConfig(config);
      upsertFlow(saved);
      openFlow(saved, false, nextSelectedNodeId);
      setNotice({ ok: true, message: "Agent Flow saved." });
      return saved;
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to save Agent Flow." });
      return null;
    }
  }

  async function changeStatus(status: AgentFlowSpec["status"]) {
    const saved = await saveFlow();
    if (!saved?.id) return;
    try {
      if (!workspace.activeBundleId) {
        throw new Error("No active config bundle selected.");
      }
      const nextFlow = {
        ...saved,
        status,
        graph: {
          ...saved.graph,
          status
        }
      };
      const current = workflowConfigs.find((item) => item.id === nextFlow.id);
      const config = workflowConfigFromAgentFlow(nextFlow, current, { agents, skills, tools });
      await resourceApi.upsertResource(workspace.activeBundleId, "workflow", config);
      const next = agentFlowFromWorkflowConfig(config);
      upsertWorkflowConfig(config);
      upsertFlow(next);
      openFlow(next, false);
      setNotice({ ok: true, message: `Agent Flow ${status}.` });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : `Failed to mark flow ${status}.` });
    }
  }

  async function runFlow() {
    if (isFlowRunning) return;
    setIsFlowRunning(true);
    const liveMessageId = createClientID("flow_chat_live");
    const traceId = createClientID("flow_trace");
    let stopFlowTracePolling = () => {};
    try {
      const submittedInput = flowTestInput;
      const saved = await saveFlow();
      if (!saved) return;
      const userText = submittedInput.trim();
      setFlowTestInput("");
      setFlowTestMessages((current) => [
        ...current,
        {
          id: createClientID("flow_chat_user"),
          role: "user",
          text: userText || "(empty message)"
        }
      ]);
      const input = inputFromDebugText(submittedInput);
      const initialProgressTrace = buildRuntimeEventsTrace(traceId, flowDebugSessionId, saved.name, []);
      setFlowTestMessages((current) => [
        ...current,
        {
          id: liveMessageId,
          role: "assistant",
          text: "正在执行 Agent Flow...",
          traceId,
          trace: initialProgressTrace,
          liveEvents: [],
          pending: true
        }
      ]);
      stopFlowTracePolling = startFlowTracePolling(traceId, liveMessageId);
      if (!workspace.activeBundleId) {
        throw new Error("No active config bundle selected.");
      }
      const { session } = await debugSessionApi.createSession({
        bundle_id: workspace.activeBundleId,
        entrypoint: {
          kind: "workflow",
          id: saved.id
        }
      });
      const runtimeResult =
        saved.orchestrationMode === "supervisor"
          ? await debugSessionApi.runAgentGraph(session.id, {
              agent_flow_id: saved.id,
              input,
              trace_context: {
                trace_id: traceId
              }
            })
          : await debugSessionApi.runWorkflow(session.id, {
              workflow_id: saved.id,
              input,
              trace_context: {
                trace_id: traceId
              }
            });
      const result = agentFlowRunResponseFromRuntime(saved.id, input, runtimeResult, traceId);
      const trace = await loadRuntimeTraceOrFallback(traceId, result, flowDebugSessionId);
      setActiveFlowTrace((current) => (current?.traceId === traceId ? trace : current));
      setFlowTestMessages((current) =>
        [
          ...current.map((message) =>
            message.id === liveMessageId
              ? {
                  ...message,
                  text: "Agent Flow 执行完成。",
                  traceId: trace.traceId,
                  trace,
                  liveEvents: runtimeEventsFromTrace(trace),
                  pending: false
                }
              : message
          ),
          {
            id: createClientID("flow_chat_agent"),
            role: "assistant" as const,
            text: flowAssistantText(result),
            traceId: trace.traceId,
            trace
          }
        ]
      );
      setNotice({
        ok: result.run.status === "succeeded",
        message: result.error || `Agent Flow run ${result.run.status}.`
      });
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : "Failed to run Agent Flow.";
      const failedProgressTrace = buildRuntimeEventsTrace(traceId, flowDebugSessionId, selectedFlow.name, [], errorMessage);
      setActiveFlowTrace((current) => (current?.traceId === traceId ? failedProgressTrace : current));
      setFlowTestMessages((current) =>
        current.some((message) => message.id === liveMessageId)
          ? current.map((message) =>
              message.id === liveMessageId
                ? {
                    ...message,
                    text: errorMessage,
                    traceId: message.traceId ?? traceId,
                    trace: buildRuntimeEventsTrace(message.traceId ?? traceId, flowDebugSessionId, selectedFlow.name, message.liveEvents ?? [], errorMessage),
                    pending: false
                  }
                : message
            )
            : [
                ...current,
                {
                  id: createClientID("flow_chat_error"),
                  role: "assistant",
                  text: errorMessage,
                  traceId,
                  trace: failedProgressTrace
                }
              ]
      );
      setNotice({ ok: false, message: errorMessage });
    } finally {
      stopFlowTracePolling();
      setIsFlowRunning(false);
    }
  }

  function startFlowTracePolling(traceId: string, liveMessageId: string): () => void {
    let stopped = false;
    let timer: number | undefined;

    const poll = async () => {
      if (stopped) return;
      try {
        const trace = agentTraceFromTraceResponse(await runtimeApiV2.getTrace(traceId));
        const liveEvents = runtimeEventsFromTrace(trace);
        setActiveFlowTrace((current) => (current?.traceId === traceId ? trace : current));
        setFlowTestMessages((current) =>
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
        // Trace spans are created asynchronously after the runtime receives the request.
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

  function resetFlowDebugSession() {
    setFlowDebugSessionId(createClientID("flow_debug_session"));
    setFlowTestMessages([]);
    setActiveFlowTrace(null);
    setIsFlowTestHistoryOpen(false);
  }

  async function restoreFlowDebugSession(session: FlowTestSessionRecord) {
    const restoredMessages = await restoreFlowTestMessages(selectedFlow, session);
    restoredFlowHistorySnapshotRef.current = {
      messagesKey: flowTestMessagesKey(restoredMessages),
      sessionId: session.sessionId
    };
    setFlowDebugSessionId(session.sessionId);
    setFlowTestMessages(restoredMessages);
    setActiveFlowTrace(null);
    setIsFlowTestHistoryOpen(false);
  }

  function openFlowTrace(trace: AgentTrace) {
    setActiveNodeTrace(null);
    setActiveFlowTrace(trace);
  }

  async function openFlowTraceById(traceId: string) {
    try {
      openFlowTrace(agentTraceFromTraceResponse(await runtimeApiV2.getTrace(traceId)));
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to load flow trace." });
    }
  }

  function openNodeTrace(trace: AgentTrace) {
    setActiveFlowTrace(null);
    setActiveNodeTrace(trace);
  }

  async function openNodeTraceById(traceId: string) {
    try {
      openNodeTrace(agentTraceFromTraceResponse(await runtimeApiV2.getTrace(traceId)));
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to load node trace." });
    }
  }

  async function runSelectedNode() {
    if (!selectedNode) return;
    if (!isAgentLikeNode(selectedNode.data.nodeType)) {
      setNotice({ ok: false, message: "Only Agent nodes support node-level testing." });
      return;
    }

    setIsNodeRunning(true);
    const traceId = createClientID("trace");
    const liveMessageId = createClientID("flow_node_chat_live");
    try {
      const agentID = await resolveNodeAgentIDForTest(selectedNode);
      if (!agentID) {
        setNotice({ ok: false, message: "Node-level test requires an Existing Agent. Local Agent nodes are tested through the whole Agent Flow." });
        return;
      }
      const userText = nodeTestInput.trim();
      const userMessage: DebugChatMessage = {
        id: createClientID("flow_node_chat_user"),
        role: "user",
        text: userText || "(empty message)"
      };
      setNodeTestMessages((current) => [...current, userMessage]);
      setNodeTestMessages((current) => [
        ...current,
        {
          id: liveMessageId,
          role: "assistant",
          text: "正在分析请求...",
          traceId,
          liveEvents: [],
          pending: true
        }
      ]);
      if (!workspace.activeBundleId) {
        throw new Error("No active config bundle selected.");
      }
      const { session } = await debugSessionApi.createSession({
        bundle_id: workspace.activeBundleId,
        entrypoint: {
          kind: "agent",
          id: agentID
        }
      });
      const result = await debugSessionApi.runAgent(session.id, {
        agent_id: agentID,
        user_message: userText,
        trace_context: {
          trace_id: traceId
        }
      });
      let trace: AgentTrace | null = null;
      try {
        trace = agentTraceFromTraceResponse(await runtimeApiV2.getTrace(traceId));
      } catch {
        trace = null;
      }
      setNodeTestMessages((current) =>
        current.map((message) =>
          message.id === liveMessageId
            ? {
                ...message,
                text: result.result.text,
                traceId,
                trace,
                pending: false
              }
            : message
        )
      );
      setNotice({ ok: true, message: "Node agent test completed." });
    } catch (error) {
      setNodeTestMessages((current) =>
        current.map((message) =>
          message.id === liveMessageId
            ? {
                ...message,
                text: error instanceof Error ? error.message : "Failed to test node agent.",
                pending: false
              }
            : message
        )
      );
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to test node agent." });
    } finally {
      setIsNodeRunning(false);
    }
  }

  function resetNodeDebugSession() {
    setNodeDebugSessionId(createClientID("flow_node_debug_session"));
    setNodeTestMessages([]);
    setActiveNodeTrace(null);
  }

  async function resolveNodeAgentIDForTest(node: AgentFlowCanvasNode): Promise<string | undefined> {
    if (node.data.agentId) return node.data.agentId;
    return undefined;
  }

  function upsertFlow(flow: AgentFlowSpec) {
    setFlows((current) => {
      const exists = current.some((item) => item.id === flow.id);
      if (exists) {
        return current.map((item) => (item.id === flow.id ? flow : item));
      }
      return [flow, ...current];
    });
  }

  function upsertWorkflowConfig(config: WorkflowConfig) {
    setWorkflowConfigs((current) => {
      const exists = current.some((item) => item.id === config.id);
      if (exists) {
        return current.map((item) => (item.id === config.id ? config : item));
      }
      return [config, ...current];
    });
  }

  if (pageMode === "list") {
    return (
      <div className="agent-gallery-page agent-flow-gallery-page">
        <header className="agent-gallery-header">
          <div className="agent-flow-list-title">
            {onExit ? (
              <button className="flow-icon-button" type="button" onClick={onExit} aria-label="Back to console">
                <span aria-hidden="true">‹</span>
              </button>
            ) : null}
            <h1>Agent Flows</h1>
          </div>

          <button className="primary-action" type="button" onClick={startNewFlow}>
            New Flow
          </button>
        </header>

        {notice ? <p className={notice.ok ? "notice notice-ok" : "notice notice-error"}>{notice.message}</p> : null}

        <section className="agent-card-grid" aria-label="Agent Flow list">
          {flows.map((flow) => (
            <AgentFlowCard key={flow.id} flow={flow} onOpen={() => openFlow(flow)} />
          ))}
        </section>

        {flows.length === 0 ? (
          <div className="agent-flow-empty-state">
            <strong>No Agent Flows yet</strong>
            <p>Create an Agent Graph or a Workflow to start orchestration design.</p>
            <button className="primary-action" type="button" onClick={startNewFlow}>
              Create Agent Flow
            </button>
          </div>
        ) : null}

        {isCreateFlowOpen ? (
          <CreateFlowTypeDialog onClose={() => setIsCreateFlowOpen(false)} onSelect={createFlowByType} />
        ) : null}
      </div>
    );
  }

  if (isNodeConfigWorkspaceOpen && selectedNode && isAgentLikeNode(selectedNode.data.nodeType)) {
    return (
      <NodeConfigWorkspace
        activeTrace={activeNodeTrace}
        agentMentions={canvasAgentMentions}
        agents={agents}
        hasChildNodes={selectedNodeHasChildren}
        isRunning={isNodeRunning}
        isSupervisorFlow={selectedFlow.orchestrationMode === "supervisor"}
        nodeDebugSessionId={nodeDebugSessionId}
        nodeTestInput={nodeTestInput}
        nodeTestMessages={nodeTestMessages}
        notice={notice}
        contextReadOptions={contextReadOptions}
        contextWriteOptions={contextWriteOptions}
        selectedAgent={selectedAgent}
        selectedChildAgentIds={selectedChildNodeIds}
        selectedFlow={selectedFlow}
        selectedNode={selectedNode}
        skills={skills}
        tools={tools}
        onBack={() => setIsNodeConfigWorkspaceOpen(false)}
        onBindAgent={bindMentionedAgent}
        onLocalAgentChange={updateLocalAgent}
        onNodeChange={updateSelectedNodeData}
        onOpenTrace={openNodeTrace}
        onOpenTraceId={openNodeTraceById}
        onResetSession={resetNodeDebugSession}
        onRun={() => void runSelectedNode()}
        onSave={() => void saveFlow(selectedNode.id)}
        onTestInputChange={setNodeTestInput}
        onWorkflowRowsChange={updateSelectedWorkflowNodeRows}
      />
    );
  }

  return (
    <div className="agent-flow-page agent-flow-editor-page">
      <header className="agent-flow-topbar">
        <div className="agent-flow-titlebar">
          <button className="flow-icon-button" type="button" onClick={returnToList} aria-label="Back to Agent Flow list">
            <span aria-hidden="true">‹</span>
          </button>
          <div>
            <span className="eyebrow">Agent Flow</span>
            <h1>{selectedFlow.name || "Untitled Agent Flow"}</h1>
          </div>
        </div>

        <div className="agent-flow-actions">
          {notice ? (
            <span className={notice.ok ? "inline-notice inline-notice-ok" : "inline-notice inline-notice-error"}>
              {notice.message}
            </span>
          ) : null}
          <button
            className="secondary-button"
            type="button"
            onClick={() => {
              setIsFlowTestOpen(true);
              setIsCtxSettingsOpen(false);
            }}
          >
            Test Flow
          </button>
          {selectedFlow.orchestrationMode === "workflow" ? (
            <button
              className="secondary-button"
              type="button"
              onClick={() => {
                setIsCtxSettingsOpen((current) => !current);
                setIsFlowTestOpen(false);
              }}
            >
              Ctx Settings
            </button>
          ) : null}
          <button className="secondary-button" type="button" onClick={() => void saveFlow()}>
            Save
          </button>
          <button className="ghost-button" type="button" onClick={() => setIsFlowSettingsOpen(true)}>
            Flow Settings
          </button>
          {selectedFlow.status === "enabled" ? (
            <button className="ghost-button" type="button" onClick={() => void changeStatus("disabled")}>
              Disable
            </button>
          ) : (
            <button className="primary-button" type="button" onClick={() => void changeStatus("enabled")}>
              Enable
            </button>
          )}
        </div>
      </header>

      <section className="agent-flow-workbench">
        {isFlowTestOpen ? (
          <aside className="agent-flow-test-drawer" aria-label="Test Agent Flow">
            <div className="node-inspector-header flow-test-drawer-header">
              <div>
                <p>
                  Session <code>{flowDebugSessionId}</code>
                </p>
              </div>
              <div className="node-inspector-actions">
                <button
                  className={isFlowTestHistoryOpen ? "secondary-action compact-action is-active" : "secondary-action compact-action"}
                  type="button"
                  onClick={() => setIsFlowTestHistoryOpen((current) => !current)}
                >
                  History
                </button>
                <button className="secondary-action compact-action" type="button" onClick={resetFlowDebugSession}>
                  New
                </button>
                <button
                  className="flow-icon-button"
                  type="button"
                  onClick={() => {
                    setIsFlowTestOpen(false);
                    setActiveFlowTrace(null);
                  }}
                  aria-label="Close flow test"
                >
                  <span aria-hidden="true">×</span>
                </button>
              </div>
            </div>
            {isFlowTestHistoryOpen ? (
              <section className="flow-test-session-history" aria-label="Flow test session history">
                {flowTestSessions.length > 0 ? (
                  flowTestSessions.map((session) => (
                    <button
                      key={session.sessionId}
                      className={session.sessionId === flowDebugSessionId ? "flow-test-session-item is-active" : "flow-test-session-item"}
                      type="button"
                      onClick={() => void restoreFlowDebugSession(session)}
                    >
                      <strong>{flowTestSessionPreview(session.messages)}</strong>
                      <span>
                        {formatFlowTestSessionTime(session.updatedAt)} · {session.messages.length} messages
                      </span>
                    </button>
                  ))
                ) : (
                  <p>No saved test sessions yet.</p>
                )}
              </section>
            ) : null}
            <section className="flow-config-panel flow-test-chat-panel">
              <NodeDebugChat
                activeTrace={activeFlowTrace}
                emptyDescription="Send a test message to execute the whole Agent Flow and inspect the agent/tool/connector chain."
                emptyTitle="Start a flow test chat"
                messages={flowTestMessages}
                onOpenTrace={openFlowTrace}
                onOpenTraceId={openFlowTraceById}
              />
              <div className="agent-chat-composer">
                <textarea
                  rows={4}
                  value={flowTestInput}
                  placeholder="输入测试问题"
                  onChange={(event) => setFlowTestInput(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.nativeEvent.isComposing) return;
                    if (event.key === "Enter" && !event.ctrlKey && !event.shiftKey && !event.altKey && !event.metaKey) {
                      event.preventDefault();
                      void runFlow();
                    }
                  }}
                />
                <button className="primary-action" type="button" disabled={isFlowRunning} onClick={() => void runFlow()}>
                  {isFlowRunning ? "Running..." : "Send"}
                </button>
              </div>
            </section>
          </aside>
        ) : null}

        {selectedFlow.orchestrationMode === "workflow" && isCtxSettingsOpen ? (
          <AgentFlowContextSettingsPanel
            flowInputFields={flowInputFields}
            flowOutputFields={flowOutputFields}
            nodes={nodes}
            onClose={() => setIsCtxSettingsOpen(false)}
            onVariableFieldsChange={setVariableFields}
            variableFields={variableFields}
          />
        ) : null}

        <main className="agent-flow-canvas-shell" aria-label="Agent Flow canvas">
          <div className="agent-flow-toolstrip" aria-label="Add nodes">
            {(selectedFlow.orchestrationMode === "workflow" ? workflowPalette : palette).map((item) => (
              <button key={item.type} type="button" title={nodeDescription(item.type)} onClick={() => addNode(item.type)}>
                <span className="toolstrip-mark">{item.label.slice(0, 1)}</span>
                <span>{item.label}</span>
              </button>
            ))}
          </div>

          <ReactFlow
            nodes={nodes.map((node) => ({
              ...node,
              data: {
                ...node.data,
                deletable: node.id !== "start",
                onDelete: deleteNode
              }
            }))}
            edges={edges.map((edge) => ({
              ...edge,
              type: "deletableEdge",
              data: {
                ...edge.data,
                onDelete: deleteEdge
              }
            }))}
            nodeTypes={nodeTypes}
            edgeTypes={edgeTypes}
            deleteKeyCode={null}
            fitView
            fitViewOptions={{ maxZoom: 0.82, padding: 0.34 }}
            onNodesChange={(changes: NodeChange<AgentFlowCanvasNode>[]) =>
              setNodes((current) => applyNodeChanges(changes, current))
            }
            onEdgesChange={(changes: EdgeChange<AgentFlowCanvasEdge>[]) =>
              setEdges((current) => applyEdgeChanges(changes, current))
            }
            onConnect={(connection: Connection) => {
              if (!connection.source || !connection.target) return;
              if (connection.source === "start" && edges.some((edge) => edge.source === "start")) {
                setNotice({ ok: false, message: "Start node can only connect to one next node." });
                return;
              }
              const sourceNode = nodes.find((node) => node.id === connection.source);
              if (selectedFlow.orchestrationMode === "supervisor" && sourceNode && isExistingAgentCanvasNode(sourceNode)) {
                setNotice({
                  ok: false,
                  message: `Existing Agent "${sourceNode.data.label}" must be a leaf node. Use Local Agent if it needs Sub-Agents.`
                });
                return;
              }
              setEdges((current) =>
                addEdge(
                  {
                    ...connection,
                    type: "smoothstep",
                    markerEnd: { type: MarkerType.ArrowClosed }
                  },
                  current
                )
              );
            }}
            onNodeClick={(_, node) => {
              setSelectedNodeId(node.id);
              setIsRightPanelOpen(true);
            }}
          >
            <MiniMap pannable zoomable />
            <Controls />
            <Background gap={18} size={1.2} />
          </ReactFlow>
        </main>

        {isRightPanelOpen ? (
        <aside className="agent-flow-right agent-flow-right-open">
          <div className="node-inspector-header">
            <div>
              <span className="eyebrow">Node Config</span>
              <h2>{selectedNode?.data.label || "Select Node"}</h2>
            </div>
            <div className="node-inspector-actions">
              <button
                className="flow-icon-button"
                type="button"
                onClick={() => setIsNodeConfigWorkspaceOpen(true)}
                disabled={!selectedNode || !isAgentLikeNode(selectedNode.data.nodeType)}
                aria-label="Open node config workspace"
              >
                <span aria-hidden="true">⛶</span>
              </button>
              <button className="flow-icon-button" type="button" onClick={() => setIsRightPanelOpen(false)} aria-label="Hide inspector">
                <span aria-hidden="true">×</span>
              </button>
            </div>
          </div>

          <div className="node-config-section-stack">
            {selectedNode ? (
              <>
                {isAgentLikeNode(selectedNode.data.nodeType) ? (
                  <>
                    <AgentNodeConfigSections
                      agents={agents}
                      hasChildNodes={selectedNodeHasChildren}
                      includePrompt
                      isSupervisorFlow={selectedFlow.orchestrationMode === "supervisor"}
                      isWorkflowFlow={selectedFlow.orchestrationMode === "workflow"}
                      selectedAgent={selectedAgent}
                      agentMentions={canvasAgentMentions}
                      selectedChildAgentIds={selectedChildNodeIds}
                      selectedNode={selectedNode}
                      skills={skills}
                      tools={tools}
                      contextReadOptions={contextReadOptions}
                      contextWriteOptions={contextWriteOptions}
                      onBindAgent={bindMentionedAgent}
                      onNodeChange={updateSelectedNodeData}
                      onLocalAgentChange={updateLocalAgent}
                      onWorkflowRowsChange={updateSelectedWorkflowNodeRows}
                    />
                  </>
                ) : (
                  <WorkflowNodeConfigSections
                    agents={agents}
                    connectorOperations={connectorOperations}
                    selectedNode={selectedNode}
                    skills={skills}
                    tools={tools}
                    sourceOptions={contextReadOptions}
                    targetOptions={contextWriteOptions}
                    nodeOptions={nodes
                      .filter((node) => node.id !== selectedNode.id && node.data.nodeType !== "start")
                      .map((node) => ({ value: node.id, label: node.data.label || node.id, detail: node.data.nodeType }))}
                    onConfigChange={updateSelectedWorkflowNodeConfig}
                    onNodeChange={updateSelectedNodeData}
                    onRowsChange={updateSelectedWorkflowNodeRows}
                  />
                )}
              </>
            ) : (
              <p className="muted-copy">Select a node on the canvas to edit its runtime config.</p>
            )}
          </div>
        </aside>
        ) : null}
      </section>

      {isFlowSettingsOpen ? (
        <div className="flow-settings-overlay" role="dialog" aria-modal="true" aria-label="Flow settings">
          <div className="flow-settings-dialog">
            <div className="node-inspector-header">
              <div>
              <span className="eyebrow">Flow Settings</span>
              <h2>{selectedFlow.name || "Untitled Agent Flow"}</h2>
            </div>
              <button className="flow-icon-button" type="button" onClick={() => setIsFlowSettingsOpen(false)} aria-label="Close flow settings">
                <span aria-hidden="true">×</span>
              </button>
            </div>
            <section className="flow-config-panel">
              <label>
                Name
                <input value={selectedFlow.name} onChange={(event) => updateSelectedFlow({ name: event.target.value })} />
              </label>
              <label>
                Description
                <textarea
                  rows={4}
                  value={selectedFlow.description ?? ""}
                  onChange={(event) => updateSelectedFlow({ description: event.target.value })}
                />
              </label>
              <label>
                Orchestration mode
                <select
                  value={selectedFlow.orchestrationMode}
                  onChange={(event) =>
                    updateSelectedFlow({ orchestrationMode: event.target.value as AgentFlowSpec["orchestrationMode"] })
                  }
                >
                  <option value="workflow">Workflow / DAG</option>
                  <option value="supervisor">Agent Graph</option>
                </select>
              </label>
              <div className="flow-policy-grid">
                <span>Nodes</span>
                <strong>{nodes.length}</strong>
                <span>Edges</span>
                <strong>{edges.length}</strong>
                <span>Status</span>
                <strong>{selectedFlow.status}</strong>
              </div>
            </section>
          </div>
        </div>
      ) : null}

      {isCreateFlowOpen ? (
        <CreateFlowTypeDialog onClose={() => setIsCreateFlowOpen(false)} onSelect={createFlowByType} />
      ) : null}

      {activeFlowTrace ? <TraceInspector trace={activeFlowTrace} onClose={() => setActiveFlowTrace(null)} /> : null}
      {activeNodeTrace ? <TraceInspector trace={activeNodeTrace} onClose={() => setActiveNodeTrace(null)} /> : null}
    </div>
  );
}

function previewGraph(graph: ReturnType<typeof canvasToGraph>, flowName: string) {
  if (graph.id) return graph;
  return {
    ...graph,
    id: "flow_preview",
    tenantId: "tenant_1",
    name: flowName || "Preview Flow"
  };
}

function previewFlow(flow: AgentFlowSpec, graph: ReturnType<typeof canvasToGraph>): AgentFlowSpec {
  return {
    ...flow,
    id: flow.id || "flow_preview",
    tenantId: "tenant_1",
    graph: previewGraph(graph, flow.name)
  };
}

function agentFlowRunResponseFromRuntime(
  flowId: string,
  input: Record<string, unknown>,
  result: RuntimeWorkflowRunResponse,
  traceId: string
): AgentFlowRunResponse {
  const now = new Date().toISOString();
  return {
    run: {
      id: result.instance_id || createClientID("agent_flow_run"),
      tenantId: "tenant_1",
      flowId,
      status: result.status === "failed" ? "failed" : result.status === "running" ? "running" : "succeeded",
      input,
      output: result.output,
      startedAt: now,
      finishedAt: result.status === "running" ? undefined : now
    },
    nodeRuns: []
  };
}

async function loadRuntimeTraceOrFallback(traceId: string, result: AgentFlowRunResponse, sessionId: string): Promise<AgentTrace> {
  try {
    return agentTraceFromTraceResponse(await runtimeApiV2.getTrace(traceId));
  } catch {
    return buildFlowTraceFromRun(result, sessionId);
  }
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

async function buildFlowTraceFromRun(result: AgentFlowRunResponse, sessionId: string): Promise<AgentTrace> {
  const rootStepID = `flow_run_${result.run.id}`;
  const steps: TraceStep[] = [
    {
      id: rootStepID,
      type: "event",
      name: "Agent Flow run",
      status: traceStepStatusFromRun(result.run.status),
      startedAt: result.run.startedAt,
      finishedAt: result.run.finishedAt,
      durationMillis: durationMillis(result.run.startedAt, result.run.finishedAt),
      error: result.error || result.run.error,
      metadata: {
        scope: "agent_flow",
        flow_id: result.run.flowId,
        run_id: result.run.id,
        node_count: result.nodeRuns.length,
        request: result.run.input,
        response: result.run.output
      }
    }
  ];

  for (const nodeRun of sortedNodeRuns(result.nodeRuns)) {
    const output = recordFromUnknown(nodeRun.output);
    const input = recordFromUnknown(nodeRun.input);
    const traceID = stringFromRecord(output, "trace_id") || stringFromRecord(output, "traceId");
    const agentID = stringFromRecord(output, "agent_id") || stringFromRecord(output, "agentId") || stringFromRecord(input, "agent_id");
    const phase = stringFromRecord(recordFromUnknown(input.payload), "phase") || stringFromRecord(output, "phase");
    const nodeStepID = `flow_node_${nodeRun.id}`;

    steps.push({
      id: nodeStepID,
      parentId: rootStepID,
      type: isAgentLikeNode(nodeRun.nodeType) ? "agent" : "event",
      name: nodeRun.nodeName || nodeRun.nodeId,
      status: traceStatusFromNodeRun(nodeRun.status),
      startedAt: nodeRun.startedAt,
      finishedAt: nodeRun.finishedAt,
      durationMillis: durationMillis(nodeRun.startedAt, nodeRun.finishedAt),
      error: nodeRun.error,
      metadata: {
        scope: "agent_flow_node",
        node_id: nodeRun.nodeId,
        node_type: nodeRun.nodeType,
        node_name: nodeRun.nodeName,
        agent_id: agentID,
        phase,
        inner_trace_id: traceID,
        request: nodeRun.input,
        response: nodeRun.output
      }
    });

    const innerTrace = traceID ? await getTraceOrNull(traceID) : null;
    if (!innerTrace) continue;
    innerTrace.steps.forEach((step, index) => {
      const remappedStepID = innerTraceStepID(nodeRun.id, step, index);
      const parentIndex = step.parentId ? innerTrace.steps.findIndex((candidate) => candidate.id === step.parentId) : -1;
      const remappedParentID =
        parentIndex >= 0 ? innerTraceStepID(nodeRun.id, innerTrace.steps[parentIndex], parentIndex) : nodeStepID;
      steps.push({
        ...step,
        id: remappedStepID,
        parentId: remappedParentID,
        metadata: {
          ...(step.metadata ?? {}),
          agent_flow_node_id: nodeRun.nodeId,
          agent_flow_node_name: nodeRun.nodeName,
          agent_id: agentID || innerTrace.agentId,
          phase,
          inner_trace_id: innerTrace.traceId
        }
      });
    });
  }

  return {
    traceId: `flow_trace_${result.run.id}`,
    tenantId: result.run.tenantId,
    sessionId,
    eventId: result.run.id,
    status: traceStatusFromRun(result.run.status),
    startedAt: result.run.startedAt,
    finishedAt: result.run.finishedAt,
    durationMillis: durationMillis(result.run.startedAt, result.run.finishedAt),
    error: result.error || result.run.error,
    steps
  };
}

function buildRuntimeEventsTrace(traceId: string, sessionId: string, flowName: string, events: RuntimeEvent[], error?: string): AgentTrace {
  const now = new Date().toISOString();
  const startedAt = events[0]?.createdAt || now;
  const finishedAt = error ? now : undefined;
  const rootStepID = `${safeTraceIDPart(traceId)}_runtime_progress`;
  const steps: TraceStep[] = [
    {
      id: rootStepID,
      type: "event",
      name: flowName ? `${flowName} progress` : "Agent Flow progress",
      status: error ? "failed" : "started",
      startedAt,
      finishedAt,
      error,
      metadata: {
        scope: "agent_flow_progress",
        event_count: events.length
      }
    }
  ];

  events.forEach((event, index) => {
    steps.push(runtimeEventTraceStep(event, index, rootStepID));
  });

  return {
    traceId,
    tenantId: "tenant_1",
    sessionId,
    status: error ? "failed" : "running",
    startedAt,
    finishedAt,
    error,
    steps
  };
}

function runtimeEventTraceStep(event: RuntimeEvent, index: number, rootStepID: string): TraceStep {
  const rawStep = recordFromUnknown(event.payload?.step);
  if (event.type === "trace_step_added" && Object.keys(rawStep).length > 0) {
    const rawStepID = stringFromRecord(rawStep, "id") || event.stepId || event.id || `step_${index + 1}`;
    const metadata = recordFromUnknown(rawStep.metadata);
    return {
      id: `${rootStepID}_${index + 1}_${safeTraceIDPart(rawStepID)}`,
      parentId: rootStepID,
      type: traceStepTypeFromString(stringFromRecord(rawStep, "type") || event.stepType),
      name: stringFromRecord(rawStep, "name") || event.name || event.type,
      status: traceStepStatusFromString(stringFromRecord(rawStep, "status") || event.status || runtimeEventStatus(event)),
      startedAt: stringFromRecord(rawStep, "started_at") || stringFromRecord(rawStep, "startedAt") || event.createdAt || new Date().toISOString(),
      finishedAt: stringFromRecord(rawStep, "finished_at") || stringFromRecord(rawStep, "finishedAt") || undefined,
      durationMillis: numberFromRecord(rawStep, "duration_millis") ?? numberFromRecord(rawStep, "durationMillis"),
      error: stringFromRecord(rawStep, "error") || undefined,
      metadata: {
        ...metadata,
        live_event_type: event.type,
        live_event_id: event.id,
        live_event_payload: event.payload
      }
    };
  }

  return {
    id: `${rootStepID}_${index + 1}_${safeTraceIDPart(event.id || event.type)}`,
    parentId: rootStepID,
    type: traceStepTypeFromString(event.stepType),
    name: event.name || runtimeEventName(event),
    status: traceStepStatusFromString(event.status || runtimeEventStatus(event)),
    startedAt: event.createdAt || new Date().toISOString(),
    error: event.type.endsWith("_failed") ? event.message : undefined,
    metadata: {
      live_event_type: event.type,
      live_event_id: event.id,
      agent_id: event.agentId,
      event_id: event.eventId,
      run_id: event.runId,
      step_id: event.stepId,
      payload: event.payload
    }
  };
}

function runtimeEventName(event: RuntimeEvent): string {
  if (event.message) return event.message;
  if (event.type === "action_planned") return "Action planned";
  if (event.type === "action_started") return "Action started";
  if (event.type === "action_completed") return "Action completed";
  if (event.type === "action_failed") return "Action failed";
  if (event.type === "planning_started") return "Planning started";
  if (event.type === "planning_completed") return "Planning completed";
  if (event.type === "llm_started") return "LLM call started";
  if (event.type === "llm_completed") return "LLM call completed";
  if (event.type === "llm_failed") return "LLM call failed";
  if (event.type === "assistant_message_completed") return "Assistant message completed";
  return event.type;
}

function runtimeEventStatus(event: RuntimeEvent): TraceStep["status"] {
  if (event.type.endsWith("_failed")) return "failed";
  if (event.type.endsWith("_completed")) return "succeeded";
  return "started";
}

function traceStepTypeFromString(value?: string): TraceStep["type"] {
  if (value === "agent" || value === "skill" || value === "model" || value === "tool" || value === "workflow" || value === "node" || value === "connector") {
    return value;
  }
  return "event";
}

function traceStepStatusFromString(value?: string): TraceStep["status"] {
  if (value === "succeeded" || value === "failed" || value === "skipped" || value === "started") {
    return value;
  }
  return "started";
}

function numberFromRecord(record: Record<string, unknown>, key: string): number | undefined {
  const value = record[key];
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function safeTraceIDPart(value: string): string {
  return value.replace(/[^a-zA-Z0-9_-]/g, "_") || "step";
}

async function getTraceOrNull(traceId: string): Promise<AgentTrace | null> {
  try {
    return agentTraceFromTraceResponse(await runtimeApiV2.getTrace(traceId));
  } catch {
    return null;
  }
}

function innerTraceStepID(nodeRunID: string, step: TraceStep, index: number): string {
  return `${nodeRunID}_${step.id}_${index}`;
}

function flowAssistantText(result: AgentFlowRunResponse): string {
  const output = recordFromUnknown(result.run.output);
  for (const key of ["return_message", "text", "answer", "message", "final_answer", "result"]) {
    const value = stringFromRecord(output, key);
    if (value) return value;
  }

  const lastNodeText = [...result.nodeRuns]
    .reverse()
    .map((nodeRun) => stringFromRecord(recordFromUnknown(nodeRun.output), "text"))
    .find(Boolean);
  if (lastNodeText) return lastNodeText;

  if (result.error || result.run.error) return result.error || result.run.error || "Agent Flow failed.";
  if (result.run.output && Object.keys(result.run.output).length > 0) {
    return JSON.stringify(result.run.output, null, 2);
  }
  return `Agent Flow run ${result.run.status}.`;
}

async function restoreFlowTestMessages(flow: AgentFlowSpec, session: FlowTestSessionRecord): Promise<DebugChatMessage[]> {
  const localMessages = messagesForHistory(session.messages);
  const completedLocalMessages = completedFlowTestMessagesForHistory(localMessages);
  if (completedLocalMessages.length > 0) return completedLocalMessages;

  const recoveredFromHistory = await recoverFlowTestMessagesFromRunHistory(flow, session);
  if (recoveredFromHistory.length > 0) return recoveredFromHistory;

  const recoveredFromTrace = await recoverFlowTestMessagesFromTrace(session, localMessages);
  return recoveredFromTrace.length > 0 ? recoveredFromTrace : localMessages;
}

async function recoverFlowTestMessagesFromRunHistory(flow: AgentFlowSpec, session: FlowTestSessionRecord): Promise<DebugChatMessage[]> {
  try {
    const { items } = await runHistoryApi.list();
    const records = items
      .filter((record) => runRecordMatchesFlowSession(record, flow, session))
      .sort((left, right) => (left.started_at || "").localeCompare(right.started_at || ""));
    const messages = records.flatMap((record) => flowTestMessagesFromRunRecord(record));
    return completedFlowTestMessagesForHistory(messages);
  } catch {
    return [];
  }
}

function runRecordMatchesFlowSession(record: RunRecord, flow: AgentFlowSpec, session: FlowTestSessionRecord): boolean {
  if (record.session_id !== session.sessionId) return false;
  const flowIDs = new Set([flow.id, flow.graph.id].filter(Boolean));
  const entrypointID = record.entrypoint?.id || record.workflow_request?.workflow_id;
  return !entrypointID || flowIDs.size === 0 || flowIDs.has(entrypointID);
}

function flowTestMessagesFromRunRecord(record: RunRecord): DebugChatMessage[] {
  const userText = flowUserTextFromRunRecord(record);
  const assistantText = flowAssistantTextFromRunRecord(record);
  const messages: DebugChatMessage[] = [];
  if (userText) {
    messages.push({
      id: `flow_history_user_${record.id}`,
      role: "user",
      text: userText
    });
  }
  if (assistantText) {
    messages.push({
      id: `flow_history_assistant_${record.id}`,
      role: "assistant",
      text: assistantText,
      traceId: record.trace_id
    });
  }
  return messages;
}

function flowUserTextFromRunRecord(record: RunRecord): string {
  const agentMessage = record.agent_request?.user_message?.trim();
  if (agentMessage) return agentMessage;
  const input = record.workflow_request?.input ?? {};
  for (const key of ["user_request", "message", "text", "query", "task", "prompt"]) {
    const value = stringFromRecord(input, key);
    if (value) return value;
  }
  return Object.keys(input).length > 0 ? JSON.stringify(input, null, 2) : "";
}

function flowAssistantTextFromRunRecord(record: RunRecord): string {
  const resultText = assistantTextFromUnknown(record.result);
  if (resultText) return resultText;
  return record.error ? `Agent Flow 执行失败：${record.error}` : "";
}

async function recoverFlowTestMessagesFromTrace(session: FlowTestSessionRecord, localMessages: DebugChatMessage[]): Promise<DebugChatMessage[]> {
  const traceId = [...localMessages].reverse().find((message) => message.traceId)?.traceId;
  if (!traceId) return [];
  const trace = await getTraceOrNull(traceId);
  if (!trace) return [];
  const assistantText = flowAssistantTextFromTrace(trace);
  if (!assistantText) return [];
  const userMessage = [...localMessages].reverse().find((message) => message.role === "user");
  return [
    ...(userMessage ? [userMessage] : []),
    {
      id: `flow_history_assistant_${session.sessionId}_${traceId}`,
      role: "assistant",
      text: assistantText,
      traceId,
      trace
    }
  ];
}

function flowAssistantTextFromTrace(trace: AgentTrace): string {
  const outputBearingSteps = [...trace.steps].reverse();
  for (const step of outputBearingSteps) {
    const metadata = step.metadata ?? {};
    const responseText = assistantTextFromUnknown(metadata.response);
    if (responseText) return responseText;
    const outputText = assistantTextFromUnknown(metadata.output);
    if (outputText) return outputText;
  }
  return trace.error ? `Agent Flow 执行失败：${trace.error}` : "";
}

function assistantTextFromUnknown(value: unknown): string {
  if (typeof value === "string") return value.trim();
  const record = recordFromUnknown(value);
  const output = recordFromUnknown(record.output);
  const result = recordFromUnknown(record.result);
  const data = recordFromUnknown(record.data);
  for (const candidate of [output, result, data, record]) {
    for (const key of ["return_message", "text", "answer", "message", "final_answer", "result"]) {
      const text = stringFromRecord(candidate, key);
      if (text) return text;
    }
  }
  return "";
}

function sortedNodeRuns(nodeRuns: AgentFlowRunResponse["nodeRuns"]): AgentFlowRunResponse["nodeRuns"] {
  return [...nodeRuns].sort((left, right) => (left.startedAt || "").localeCompare(right.startedAt || ""));
}

function traceStatusFromRun(status: AgentFlowRunResponse["run"]["status"]): AgentTrace["status"] {
  if (status === "failed" || status === "canceled") return "failed";
  if (status === "running" || status === "pending") return "running";
  return "succeeded";
}

function traceStatusFromNodeRun(status: AgentFlowRunResponse["nodeRuns"][number]["status"]): TraceStep["status"] {
  if (status === "failed" || status === "canceled") return "failed";
  if (status === "pending" || status === "running") return "started";
  if (status === "skipped") return "skipped";
  return "succeeded";
}

function traceStepStatusFromRun(status: AgentFlowRunResponse["run"]["status"]): TraceStep["status"] {
  if (status === "failed" || status === "canceled") return "failed";
  if (status === "pending" || status === "running") return "started";
  return "succeeded";
}

function durationMillis(startedAt?: string, finishedAt?: string): number | undefined {
  if (!startedAt || !finishedAt) return undefined;
  const started = Date.parse(startedAt);
  const finished = Date.parse(finishedAt);
  if (Number.isNaN(started) || Number.isNaN(finished)) return undefined;
  return Math.max(0, finished - started);
}

function recordFromUnknown(value: unknown): Record<string, unknown> {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return {};
}

function stringFromRecord(record: Record<string, unknown>, key: string): string {
  const value = record[key];
  return typeof value === "string" ? value.trim() : "";
}

function flowModeLabel(mode: AgentFlowSpec["orchestrationMode"]): string {
  return mode === "supervisor" ? "Agent Graph" : "Workflow";
}

function AgentFlowCard({ flow, onOpen }: { flow: AgentFlowSpec; onOpen: () => void }) {
  return (
    <article className="agent-app-card agent-flow-app-card">
      <div className="agent-card-main">
        <span className="agent-app-icon" aria-hidden="true">
          {flow.orchestrationMode === "supervisor" ? "SF" : "WF"}
        </span>
        <div>
          <h2>{flow.name}</h2>
          <div className="agent-card-badges">
            <Badge tone={flowStatusTone[flow.status]}>{flowStatusCopy[flow.status]}</Badge>
            <Badge tone="blue">{flowModeLabel(flow.orchestrationMode)}</Badge>
          </div>
        </div>
      </div>
      <p>{flow.description || "No description"}</p>
      <footer>
        <span>{flow.ownerTeam ?? "AI Platform"}</span>
        <span>{Object.keys(flow.graph.nodes).length} nodes · {flow.version}</span>
      </footer>
      <div className="agent-flow-card-actions">
        <button className="secondary-action" type="button" onClick={onOpen}>
          Edit
        </button>
      </div>
    </article>
  );
}

function CreateFlowTypeDialog({
  onClose,
  onSelect
}: {
  onClose: () => void;
  onSelect: (type: FlowCreateType) => void;
}) {
  return (
    <div className="flow-settings-overlay" role="dialog" aria-modal="true" aria-label="Create Agent Flow">
      <div className="flow-type-dialog">
        <div className="node-inspector-header">
          <div>
            <span className="eyebrow">Create Agent Flow</span>
            <h2>Choose orchestration type</h2>
          </div>
          <button className="flow-icon-button" type="button" onClick={onClose} aria-label="Close create flow dialog">
            <span aria-hidden="true">×</span>
          </button>
        </div>

        <div className="flow-type-options">
          <button type="button" onClick={() => onSelect("supervisor")}>
            <span className="agent-flow-card-icon agent-flow-card-icon-supervisor">S</span>
            <strong>Agent Graph</strong>
            <small>适合递归式多 Agent 编排，每个 Local Agent 节点都可以继续调度自己的 Sub-Agent。</small>
          </button>
          <button type="button" onClick={() => onSelect("workflow")}>
            <span className="agent-flow-card-icon agent-flow-card-icon-workflow">W</span>
            <strong>Workflow</strong>
            <small>适合确定性流程、固定业务步骤、并行/汇聚/条件分支。</small>
          </button>
        </div>
      </div>
    </div>
  );
}

function NodeConfigSection({
  children,
  defaultOpen = false,
  helpText,
  meta,
  title
}: {
  children: ReactNode;
  defaultOpen?: boolean;
  helpText?: string;
  meta?: string;
  title: string;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <section className={open ? "agent-config-group agent-config-group-open" : "agent-config-group"}>
      <button className="agent-config-group-trigger" type="button" onClick={() => setOpen((current) => !current)} aria-expanded={open}>
        <span aria-hidden="true">{open ? "⌄" : "›"}</span>
        <strong>
          {title}
          {helpText ? (
            <span className="workflow-section-help" title={helpText} aria-label={helpText}>
              ?
            </span>
          ) : null}
        </strong>
        <small>{meta}</small>
      </button>
      {open ? <div className="agent-config-group-body">{children}</div> : null}
    </section>
  );
}

function AgentFlowContextSettingsPanel({
  flowInputFields,
  flowOutputFields,
  nodes,
  onClose,
  onVariableFieldsChange,
  variableFields
}: {
  flowInputFields: ContextFieldDraft[];
  flowOutputFields: ContextFieldDraft[];
  nodes: AgentFlowCanvasNode[];
  onClose: () => void;
  onVariableFieldsChange: (fields: ContextFieldDraft[]) => void;
  variableFields: ContextFieldDraft[];
}) {
  const toolAliases = responseAliasesForAgentFlowNodes(nodes, "tool");
  const agentAliases = responseAliasesForAgentFlowNodes(nodes, "agent");
  return (
    <aside className="workflow-context-sidebar" aria-label="Agent Workflow context settings">
      <header className="node-inspector-header">
        <div>
          <span className="eyebrow">Context Protocol</span>
          <h2>Ctx Settings</h2>
        </div>
        <button className="flow-panel-close" type="button" onClick={onClose} aria-label="Close context settings">
          ×
        </button>
      </header>

      <NodeConfigSection
        defaultOpen
        title="Flow Input"
        meta="read only"
        helpText="Agent Flow is user-facing, so the input contract is fixed. Nodes should read the request from $flow_input.user_request."
      >
        <ReadOnlyContextFieldRows fields={flowInputFields} />
      </NodeConfigSection>

      <NodeConfigSection
        title="Flow Output"
        meta="fixed"
        helpText="Agent Flow returns one final user-facing message. Nodes should write the result to $flow_output.return_message."
      >
        <ReadOnlyContextFieldRows fields={flowOutputFields} />
      </NodeConfigSection>

      <NodeConfigSection
        title="Tool Response"
        meta="runtime managed"
        helpText="Tool nodes write raw tool responses automatically. Configure response alias on the node when the same tool is called multiple times."
      >
        <RuntimeAliasList aliases={toolAliases} emptyMessage="No tool response aliases yet." prefix="$responses.tool" />
      </NodeConfigSection>

      <NodeConfigSection
        title="Agent Response"
        meta="runtime managed"
        helpText="Agent nodes write raw agent outputs automatically. Use this for read-only inspection or downstream mappings."
      >
        <RuntimeAliasList aliases={agentAliases} emptyMessage="No agent response aliases yet." prefix="$responses.agent" />
      </NodeConfigSection>

      <NodeConfigSection title="Variables" meta="read / write" helpText="Temporary values for orchestration steps. Prefer named business fields over node IDs.">
        <ContextFieldRows fields={variableFields} onChange={onVariableFieldsChange} />
      </NodeConfigSection>
    </aside>
  );
}

function ReadOnlyContextFieldRows({ fields }: { fields: ContextFieldDraft[] }) {
  return (
    <div className="workflow-context-field-block">
      <div className="workflow-context-table workflow-context-table-readonly">
        <span>Path</span>
        <span>Type</span>
        <span>Req</span>
        <span>Description</span>
        <span />
        {fields.map((field) => (
          <div className="workflow-table-row" key={field.path || field.id}>
            <code>{field.path}</code>
            <span>{field.type}</span>
            <span>{field.required ? "Yes" : "No"}</span>
            <span>{field.description || "-"}</span>
            <span>Locked</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function ContextFieldRows({ fields, onChange }: { fields: ContextFieldDraft[]; onChange: (fields: ContextFieldDraft[]) => void }) {
  const update = (fieldId: string, patch: Partial<ContextFieldDraft>) => {
    onChange(fields.map((field) => (field.id === fieldId ? { ...field, ...patch } : field)));
  };
  return (
    <div className="workflow-context-field-block">
      <div className="workflow-context-table">
        <span>Path</span>
        <span>Type</span>
        <span>Req</span>
        <span>Description</span>
        <span />
        {fields.map((field) => (
          <div className="workflow-table-row" key={field.id}>
            <input value={field.path} onChange={(event) => update(field.id, { path: event.target.value })} placeholder="request.query" />
            <select value={field.type} onChange={(event) => update(field.id, { type: event.target.value })}>
              <option value="string">string</option>
              <option value="number">number</option>
              <option value="integer">integer</option>
              <option value="boolean">boolean</option>
              <option value="object">object</option>
              <option value="array">array</option>
            </select>
            <input type="checkbox" checked={field.required} onChange={(event) => update(field.id, { required: event.target.checked })} />
            <input value={field.description} onChange={(event) => update(field.id, { description: event.target.value })} placeholder="Explain this context field" />
            <button type="button" onClick={() => onChange(fields.filter((item) => item.id !== field.id))} aria-label="Remove context field">
              ×
            </button>
          </div>
        ))}
      </div>
      <button className="secondary-action" type="button" onClick={() => onChange([...fields, createContextField()])}>
        + Field
      </button>
    </div>
  );
}

function AgentInputProtocolRows({
  fields,
  mappingRows,
  onChange,
  sourceOptions
}: {
  fields: ContextFieldDraft[];
  mappingRows: MappingRowDraft[];
  onChange: (fields: ContextFieldDraft[], inputRows: MappingRowDraft[]) => void;
  sourceOptions: MappingOption[];
}) {
  const alignedRows = alignInputProtocolRows(fields, mappingRows);
  const sourceForField = (field: ContextFieldDraft) => alignedRows.find((row) => row.target === field.path.trim())?.source ?? "";
  const updateProtocol = (nextFields: ContextFieldDraft[], nextRows: MappingRowDraft[]) => onChange(nextFields, alignInputProtocolRows(nextFields, nextRows));
  const updateField = (fieldId: string, patch: Partial<ContextFieldDraft>) => {
    const currentField = fields.find((field) => field.id === fieldId);
    const previousPath = currentField?.path.trim() ?? "";
    const nextFields = fields.map((field) => (field.id === fieldId ? { ...field, ...patch } : field));
    let nextRows = alignedRows;
    if (patch.path !== undefined) {
      const nextPath = patch.path.trim();
      nextRows = alignedRows.map((row) => (previousPath && row.target === previousPath ? { ...row, target: nextPath } : row));
    }
    updateProtocol(nextFields, nextRows);
  };
  const updateSource = (field: ContextFieldDraft, source: string) => {
    const target = field.path.trim();
    if (!target) return;
    const row = alignedRows.find((item) => item.target === target);
    const nextRows = row
      ? alignedRows.map((item) => (item.id === row.id ? { ...item, source } : item))
      : [...alignedRows, createMappingRow({ id: `input_map_${field.id}`, target, source })];
    updateProtocol(fields, nextRows);
  };
  const addField = () => {
    const field = createContextField({ path: nextInputProtocolFieldPath(fields) });
    updateProtocol([...fields, field], [...alignedRows, createMappingRow({ id: `input_map_${field.id}`, target: field.path, source: "" })]);
  };
  const removeField = (field: ContextFieldDraft) => {
    const target = field.path.trim();
    updateProtocol(
      fields.filter((item) => item.id !== field.id),
      alignedRows.filter((row) => row.target !== target)
    );
  };
  return (
    <div className="workflow-context-field-block">
      <div className="workflow-context-table workflow-input-protocol-table">
        <span>Field</span>
        <span>Value From</span>
        <span>Type</span>
        <span>Req</span>
        <span>Description</span>
        <span />
        {fields.map((field) => (
          <div className="workflow-table-row" key={field.id}>
            <input value={field.path} onChange={(event) => updateField(field.id, { path: event.target.value })} placeholder="user_task" />
            <FlowExpressionInput options={sourceOptions} value={sourceForField(field)} onChange={(value) => updateSource(field, value)} />
            <select value={field.type} onChange={(event) => updateField(field.id, { type: event.target.value })}>
              <option value="string">string</option>
              <option value="number">number</option>
              <option value="integer">integer</option>
              <option value="boolean">boolean</option>
              <option value="object">object</option>
              <option value="array">array</option>
            </select>
            <input type="checkbox" checked={field.required} onChange={(event) => updateField(field.id, { required: event.target.checked })} />
            <input value={field.description} onChange={(event) => updateField(field.id, { description: event.target.value })} placeholder="Explain this input field" />
            <button type="button" onClick={() => removeField(field)} aria-label="Remove input field">
              ×
            </button>
          </div>
        ))}
      </div>
      <button className="secondary-action" type="button" onClick={addField}>
        + Field
      </button>
    </div>
  );
}

function alignInputProtocolRows(fields: ContextFieldDraft[], rows: MappingRowDraft[]): MappingRowDraft[] {
  const byTarget = new Map(rows.map((row) => [row.target.trim(), row]));
  return fields
    .map((field) => field.path.trim())
    .filter(Boolean)
    .map((path) => {
      const row = byTarget.get(path);
      return row ?? createMappingRow({ id: `input_map_${path}`, target: path, source: "" });
    });
}

function nextInputProtocolFieldPath(fields: ContextFieldDraft[]): string {
  const existing = new Set(fields.map((field) => field.path.trim()).filter(Boolean));
  let index = fields.length + 1;
  let candidate = `field_${index}`;
  while (existing.has(candidate)) {
    index += 1;
    candidate = `field_${index}`;
  }
  return candidate;
}

function RuntimeAliasList({ aliases, emptyMessage, prefix }: { aliases: string[]; emptyMessage: string; prefix: string }) {
  return (
    <div className="workflow-response-alias-list">
      {aliases.length > 0 ? aliases.map((alias) => <code key={alias}>{`${prefix}.${alias}`}</code>) : <span>{emptyMessage}</span>}
    </div>
  );
}

function responseAliasesForAgentFlowNodes(nodes: AgentFlowCanvasNode[], type: "tool" | "agent"): string[] {
  return nodes
    .filter((node) => (type === "tool" ? node.data.nodeType === "tool" : isAgentLikeNode(node.data.nodeType)))
    .map((node) => responseAliasForAgentFlowNode(node, type))
    .filter((alias, index, aliases) => Boolean(alias) && aliases.indexOf(alias) === index);
}

function contextReadOptionsForAgentFlow(
  flowInputFields: ContextFieldDraft[],
  flowOutputFields: ContextFieldDraft[],
  variableFields: ContextFieldDraft[],
  nodes: AgentFlowCanvasNode[]
): MappingOption[] {
  return [
    ...fieldsToMappingOptions(flowInputFields, "$flow_input", "Flow Input"),
    ...fieldsToMappingOptions(flowOutputFields, "$flow_output", "Flow Output"),
    ...fieldsToMappingOptions(variableFields, "$variables", "Variables"),
    ...responseAliasOptionsForAgentFlowNodes(nodes)
  ];
}

function contextWriteOptionsForAgentFlow(flowOutputFields: ContextFieldDraft[], variableFields: ContextFieldDraft[]): MappingOption[] {
  return [
    ...fieldsToMappingOptions(flowOutputFields, "flow_output", "Flow Output"),
    ...fieldsToMappingOptions(variableFields, "variables", "Variables")
  ];
}

function agentOutputReadOptionsFromSchema(fields: ContextFieldDraft[]): MappingOption[] {
  const schemaOptions = fields
    .filter((field) => field.path.trim())
    .map((field) => {
      const path = field.path.trim();
      return {
        value: `$.${path}`,
        label: `Agent Output: ${path}`,
        detail: field.description
      };
    });
  return [
    ...schemaOptions,
    { value: "$.text", label: "Agent Text: text", detail: "Use only when the Agent output mode is Text or you explicitly return a text field." },
    { value: "$.trace_id", label: "Agent Output: trace_id", detail: "Trace id for this Agent execution." }
  ];
}

function fieldsToMappingOptions(fields: ContextFieldDraft[], prefix: string, group: string): MappingOption[] {
  return fields
    .filter((field) => field.path.trim())
    .map((field) => ({
      value: `${prefix}.${field.path.trim()}`,
      label: `${group}: ${field.path.trim()}`,
      detail: field.description
    }));
}

function responseAliasOptionsForAgentFlowNodes(nodes: AgentFlowCanvasNode[]): MappingOption[] {
  return [
    ...responseAliasesForAgentFlowNodes(nodes, "tool").map((alias) => ({
      value: `$responses.tool.${alias}`,
      label: `Tool Response: ${alias}`
    })),
    ...responseAliasesForAgentFlowNodes(nodes, "agent").map((alias) => ({
      value: `$responses.agent.${alias}`,
      label: `Agent Response: ${alias}`
    }))
  ];
}

function responseAliasForAgentFlowNode(node: AgentFlowCanvasNode, type: "tool" | "agent"): string {
  const config = node.data.config ?? {};
  const explicit = typeof config.response_alias === "string" ? config.response_alias.trim() : "";
  if (explicit) return explicit;
  if (type === "tool") {
    const toolId = typeof config.tool_id === "string" ? config.tool_id.trim() : "";
    return toolId || node.id;
  }
  const agentId = typeof config.agent_id === "string" ? config.agent_id.trim() : node.data.agentId || "";
  return agentId || node.id;
}

function NodeConfigWorkspace({
  activeTrace,
  agentMentions,
  agents,
  hasChildNodes,
  isSupervisorFlow,
  isRunning,
  nodeDebugSessionId,
  nodeTestInput,
  nodeTestMessages,
  notice,
  contextReadOptions,
  contextWriteOptions,
  onBack,
  onBindAgent,
  onLocalAgentChange,
  onNodeChange,
  onOpenTrace,
  onOpenTraceId,
  onRun,
  onSave,
  onResetSession,
  onTestInputChange,
  onWorkflowRowsChange,
  selectedAgent,
  selectedChildAgentIds,
  selectedFlow,
  selectedNode,
  skills,
  tools
}: {
  activeTrace: AgentTrace | null;
  agentMentions: PromptAgentMention[];
  agents: AgentProfile[];
  hasChildNodes: boolean;
  isSupervisorFlow: boolean;
  isRunning: boolean;
  nodeDebugSessionId: string;
  nodeTestInput: string;
  nodeTestMessages: DebugChatMessage[];
  notice: Notice | null;
  contextReadOptions: MappingOption[];
  contextWriteOptions: MappingOption[];
  onBack: () => void;
  onBindAgent: (agentId: string) => void;
  onLocalAgentChange: (patch: Partial<LocalAgentConfig>) => void;
  onNodeChange: (patch: Partial<AgentFlowCanvasNodeData>) => void;
  onOpenTrace: (trace: AgentTrace) => void;
  onOpenTraceId: (traceId: string) => void | Promise<void>;
  onRun: () => void;
  onSave: () => void;
  onResetSession: () => void;
  onTestInputChange: (value: string) => void;
  onWorkflowRowsChange: (kind: keyof NodeConfigRows, rows: MappingRowDraft[]) => void;
  selectedAgent?: AgentProfile;
  selectedChildAgentIds: string[];
  selectedFlow: AgentFlowSpec;
  selectedNode: AgentFlowCanvasNode;
  skills: SkillSpec[];
  tools: ToolSpec[];
}) {
  const mode = selectedNode.data.agentMode ?? (selectedNode.data.agentId ? "existing" : "existing");
  const localAgent = selectedNode.data.localAgent ?? localAgentDefaults(selectedNode.data.label || "Local Agent");
  const promptTitle = mode === "local" ? localAgent.name || selectedNode.data.label : selectedAgent?.name || selectedNode.data.label;
  const bindSkillToLocalAgent = (skillId: string) => onLocalAgentChange({ skillIds: toggleID(localAgent.skillIds, skillId, true) });
  const bindToolToLocalAgent = (toolId: string) => onLocalAgentChange({ toolIds: toggleID(localAgent.toolIds, toolId, true) });

  return (
    <div className="agent-editor-page agent-node-editor-page">
      <header className="agent-editor-topbar">
        <button className="agent-editor-back" type="button" onClick={onBack} aria-label="Back to Agent Flow canvas">
          <span aria-hidden="true">←</span>
        </button>
        <div className="agent-editor-title">
          <h1>{promptTitle || "Agent Node"}</h1>
          <div>
            <Badge tone="blue">{nodeLabel(selectedNode.data.nodeType)}</Badge>
            <Badge tone={mode === "local" ? "green" : "gray"}>{mode === "local" ? "Local Agent" : "Existing Agent"}</Badge>
            <Badge tone={flowStatusTone[selectedFlow.status]}>{flowStatusCopy[selectedFlow.status]}</Badge>
          </div>
        </div>
        <div className="agent-editor-actions">
          <button className="secondary-action" type="button" onClick={onBack}>
            Back to Canvas
          </button>
          <button className="primary-action" type="button" onClick={onSave}>
            Save Flow
          </button>
        </div>
      </header>

      {notice ? <p className={notice.ok ? "notice notice-ok agent-editor-notice" : "notice notice-error agent-editor-notice"}>{notice.message}</p> : null}

      <section className="agent-workbench-layout agent-node-workbench-layout">
        <aside className="agent-config-sidebar" aria-label="Node Agent configuration">
            <div className="agent-config-title">
              <div>
                <strong>Config</strong>
                <span>Flow Node</span>
              </div>
            </div>

            {isAgentLikeNode(selectedNode.data.nodeType) ? (
              <AgentNodeConfigSections
                agentMentions={agentMentions}
                agents={agents}
                hasChildNodes={hasChildNodes}
                isSupervisorFlow={isSupervisorFlow}
                isWorkflowFlow={selectedFlow.orchestrationMode === "workflow"}
                selectedAgent={selectedAgent}
                selectedChildAgentIds={selectedChildAgentIds}
                selectedNode={selectedNode}
                skills={skills}
                tools={tools}
                contextReadOptions={contextReadOptions}
                contextWriteOptions={contextWriteOptions}
                onBindAgent={onBindAgent}
                onNodeChange={onNodeChange}
                onLocalAgentChange={onLocalAgentChange}
                onWorkflowRowsChange={onWorkflowRowsChange}
              />
            ) : null}
        </aside>

        <main className="agent-prompt-workspace">
            <div className="agent-prompt-header">
              <div>
                <h2>Role and Prompt</h2>
                <p>{promptTitle}</p>
              </div>
              <span className="node-modal-mode-pill">LLM-Driven</span>
            </div>

            {isAgentLikeNode(selectedNode.data.nodeType) && mode === "local" ? (
              <PromptRichEditor
                agentMentions={agentMentions}
                ariaLabel="Agent Flow node prompt"
                selectedAgentIds={selectedChildAgentIds}
                selectedSkillIds={localAgent.skillIds}
                selectedToolIds={localAgent.toolIds}
                skills={skills}
                tools={tools}
                value={localAgent.systemPrompt}
                onBindAgent={onBindAgent}
                onBindSkill={bindSkillToLocalAgent}
                onBindTool={bindToolToLocalAgent}
                onChange={(systemPrompt) => onLocalAgentChange({ systemPrompt })}
                placeholder="Write the local agent role, rules, tool-use guidance, and answer style."
              />
            ) : (
              <pre className="node-modal-prompt-view">
                {isAgentLikeNode(selectedNode.data.nodeType)
                  ? selectedAgent?.systemPrompt || "Select an Agent to preview its prompt."
                  : selectedNode.data.description || "This node does not have an Agent prompt."}
              </pre>
            )}
        </main>

        <aside className="agent-debug-sidebar" aria-label="Node debug">
            <div className="agent-debug-workbench-header">
              <div>
                <strong>Debug</strong>
                <span>
                  Session <code>{nodeDebugSessionId}</code>
                </span>
              </div>
              <div>
                <button className="secondary-action compact-action" type="button" onClick={onResetSession}>
                  New
                </button>
              </div>
            </div>

            <NodeDebugChat
              activeTrace={activeTrace}
              emptyDescription="Send a user message directly to this node's Agent, without executing the whole flow."
              emptyTitle={`Testing with ${promptTitle}`}
              messages={nodeTestMessages}
              onOpenTrace={onOpenTrace}
              onOpenTraceId={onOpenTraceId}
            />

            <div className="agent-chat-composer agent-node-modal-composer">
              <textarea value={nodeTestInput} placeholder="输入测试问题" onChange={(event) => onTestInputChange(event.target.value)} />
              <button className="primary-action" type="button" onClick={onRun} disabled={isRunning}>
                {isRunning ? "Running..." : "Send"}
              </button>
            </div>
        </aside>
      </section>
    </div>
  );
}

function AgentNodeConfigSections({
  agentMentions,
  agents,
  hasChildNodes,
  includePrompt = false,
  isSupervisorFlow,
  isWorkflowFlow = false,
  contextReadOptions,
  contextWriteOptions,
  onBindAgent,
  onLocalAgentChange,
  onNodeChange,
  onWorkflowRowsChange,
  selectedAgent,
  selectedChildAgentIds,
  selectedNode,
  skills,
  tools
}: {
  agentMentions: PromptAgentMention[];
  agents: AgentProfile[];
  hasChildNodes: boolean;
  includePrompt?: boolean;
  isSupervisorFlow: boolean;
  isWorkflowFlow?: boolean;
  contextReadOptions: MappingOption[];
  contextWriteOptions: MappingOption[];
  onBindAgent: (agentId: string) => void;
  onLocalAgentChange: (patch: Partial<LocalAgentConfig>) => void;
  onNodeChange: (patch: Partial<AgentFlowCanvasNodeData>) => void;
  onWorkflowRowsChange?: (kind: keyof NodeConfigRows, rows: MappingRowDraft[]) => void;
  selectedAgent?: AgentProfile;
  selectedChildAgentIds: string[];
  selectedNode: AgentFlowCanvasNode;
  skills: SkillSpec[];
  tools: ToolSpec[];
}) {
  const mode = selectedNode.data.agentMode ?? (selectedNode.data.agentId ? "existing" : "existing");
  const localAgent = selectedNode.data.localAgent ?? localAgentDefaults(selectedNode.data.label || "Local Agent");
  const selectedSkillIds = mode === "local" ? localAgent.skillIds : selectedAgent?.skillIds ?? [];
  const directToolIds = mode === "local" ? localAgent.toolIds : selectedAgent?.toolIds ?? [];
  const skillToolIds = useMemo(
    () =>
      new Set(
        skills
          .filter((skill) => selectedSkillIds.includes(skill.id))
          .flatMap((skill) => skill.toolIds)
          .filter((toolId) => tools.some((tool) => tool.id === toolId))
      ),
    [selectedSkillIds, skills, tools]
  );
  const reachableToolIds = useMemo(
    () => Array.from(new Set([...directToolIds, ...skillToolIds])),
    [directToolIds, skillToolIds]
  );
  const boundSkills =
    mode === "local" ? resolveResources(localAgent.skillIds, skills) : selectedAgent ? resolveResources(selectedAgent.skillIds, skills) : [];
  const boundTools = resolveResources(reachableToolIds, tools);
  const existingAgentDisabled = isSupervisorFlow && hasChildNodes;
  const bindSkillToLocalAgent = (skillId: string) => onLocalAgentChange({ skillIds: toggleID(localAgent.skillIds, skillId, true) });
  const bindToolToLocalAgent = (toolId: string) => onLocalAgentChange({ toolIds: toggleID(localAgent.toolIds, toolId, true) });
  const workflowRows = nodeConfigRows(workflowNodeFromCanvasNode(selectedNode));
  const inputSchemaSignature = JSON.stringify(selectedNode.data.config?.input_schema ?? {});
  const configuredInputSchemaFields = useMemo(
    () => contextFieldsFromSchema(recordFromUnknown(selectedNode.data.config?.input_schema)),
    [inputSchemaSignature]
  );
  const [draftInputSchemaFields, setDraftInputSchemaFields] = useState<ContextFieldDraft[]>(configuredInputSchemaFields);
  useEffect(() => {
    // Keep row ids stable while editing; only rehydrate after switching nodes.
    setDraftInputSchemaFields(configuredInputSchemaFields);
  }, [selectedNode.id]);
  const outputSchemaSignature = JSON.stringify(selectedNode.data.config?.output_schema ?? {});
  const configuredOutputSchemaFields = useMemo(
    () => contextFieldsFromSchema(recordFromUnknown(selectedNode.data.config?.output_schema)),
    [outputSchemaSignature]
  );
  const [draftOutputSchemaFields, setDraftOutputSchemaFields] = useState<ContextFieldDraft[]>(configuredOutputSchemaFields);
  useEffect(() => {
    // Only hydrate draft rows when switching nodes. Re-hydrating after every
    // schema save regenerates field ids and remounts focused inputs.
    setDraftOutputSchemaFields(configuredOutputSchemaFields);
  }, [selectedNode.id]);
  const agentOutputReadOptions = useMemo(
    () => agentOutputReadOptionsFromSchema(draftOutputSchemaFields),
    [draftOutputSchemaFields]
  );

  const updateAgentOutputSchemaFields = (fields: ContextFieldDraft[]) => {
    setDraftOutputSchemaFields(fields);
    if (fields.some((field) => !field.path.trim())) return;
    const { output_mapping: _legacyOutputMapping, ...nextConfig } = selectedNode.data.config ?? {};
    onNodeChange({
      config: {
        ...nextConfig,
        output_schema: contextSchemaFromFields(fields),
        output_mode: fields.length > 0 ? "json" : nextConfig.output_mode ?? "text"
      }
    });
  };

  const updateAgentInputProtocol = (fields: ContextFieldDraft[], inputRows: MappingRowDraft[]) => {
    setDraftInputSchemaFields(fields);
    if (fields.some((field) => !field.path.trim())) return;
    onNodeChange({
      config: {
        ...(selectedNode.data.config ?? {}),
        input_schema: contextSchemaFromFields(fields),
        input_mapping: recordFromMappingRows(inputRows)
      }
    });
  };

  const selectExistingAgent = (agentId: string) => {
    if (existingAgentDisabled) return;
    const nextAgent = agents.find((agent) => agent.id === agentId);
    onNodeChange({
      agentId: agentId || undefined,
      agentMode: "existing",
      ...(nextAgent
        ? {
            label: nextAgent.name,
            description: nextAgent.description
          }
        : {})
    });
  };

  return (
    <>
      <NodeConfigSection defaultOpen meta={mode === "local" ? localAgent.model : selectedAgent?.modelConfig?.model || "Select Agent"} title="Basic">
        <div className="agent-node-mode-tabs" role="tablist" aria-label="Agent node binding mode">
          <button
            className={mode === "existing" ? "agent-node-mode-active" : ""}
            disabled={existingAgentDisabled}
            type="button"
            title={existingAgentDisabled ? "Existing Agent nodes must be leaf nodes in Agent Graph flows." : undefined}
            onClick={() => onNodeChange({ agentMode: "existing" })}
          >
            Existing Agent
          </button>
          <button
            className={mode === "local" ? "agent-node-mode-active" : ""}
            type="button"
            onClick={() => onNodeChange({ agentId: undefined, agentMode: "local", localAgent })}
          >
            Local Agent
          </button>
        </div>

        {mode === "existing" ? (
          <>
            {existingAgentDisabled ? (
              <div className="agent-node-leaf-warning">
                Existing Agent can only be used as a leaf node. Switch to Local Agent if this node needs Sub-Agents.
              </div>
            ) : null}
            <label>
              Agent
              <select
                disabled={existingAgentDisabled}
                value={selectedNode.data.agentId ?? ""}
                onChange={(event) => selectExistingAgent(event.target.value)}
              >
                <option value="">Select an agent</option>
                {agents.map((agent) => (
                  <option key={agent.id} value={agent.id}>
                    {agent.name}
                  </option>
                ))}
              </select>
            </label>
            <div className="agent-model-pill">
              <span>{selectedAgent?.modelConfig?.providerId || "provider"}</span>
              <strong>{selectedAgent?.modelConfig?.model || "model"}</strong>
            </div>
          </>
        ) : (
          <>
            <label>
              Agent name
              <input value={localAgent.name} onChange={(event) => onLocalAgentChange({ name: event.target.value })} />
            </label>
            <label>
              Agent description
              <textarea value={localAgent.description} onChange={(event) => onLocalAgentChange({ description: event.target.value })} />
            </label>
            <label>
              Model
              <input value={localAgent.model} onChange={(event) => onLocalAgentChange({ model: event.target.value })} />
            </label>
          </>
        )}
      </NodeConfigSection>

      {includePrompt ? (
        <NodeConfigSection meta={mode === "local" ? "editable" : "preview"} title="Prompt">
          {mode === "local" ? (
            <PromptRichEditor
              agentMentions={agentMentions}
              ariaLabel="Agent Flow node prompt"
              selectedAgentIds={selectedChildAgentIds}
              selectedSkillIds={localAgent.skillIds}
              selectedToolIds={localAgent.toolIds}
              skills={skills}
              tools={tools}
              value={localAgent.systemPrompt}
              onBindAgent={onBindAgent}
              onBindSkill={bindSkillToLocalAgent}
              onBindTool={bindToolToLocalAgent}
              onChange={(systemPrompt) => onLocalAgentChange({ systemPrompt })}
              placeholder="Write the local agent role, rules, tool-use guidance, and answer style. Type @ to bind Skills, Tools, or Sub-Agents."
            />
          ) : (
            <div className="bound-agent-prompt-preview">
              <pre>{selectedAgent?.systemPrompt || "Select an Agent to preview its prompt."}</pre>
            </div>
          )}
        </NodeConfigSection>
      ) : null}

      <NodeConfigSection meta={`${boundSkills.length} selected`} title="Skills">
        {mode === "local" ? (
          <SkillChecklist
            selectedSkillIds={localAgent.skillIds}
            skills={skills}
            onToggle={(skillId, checked) => onLocalAgentChange({ skillIds: toggleID(localAgent.skillIds, skillId, checked) })}
          />
        ) : (
          <BoundResourceList emptyMessage="No Skills bound." items={boundSkills} title="Skills" />
        )}
      </NodeConfigSection>

      <NodeConfigSection meta={`${boundTools.length} selected`} title="Tools">
        {mode === "local" ? (
          <ToolSelector
            tools={tools}
            inheritedToolIds={Array.from(skillToolIds)}
            selectedToolIds={localAgent.toolIds}
            onToggle={(toolId, checked) => onLocalAgentChange({ toolIds: toggleID(localAgent.toolIds, toolId, checked) })}
            emptyMessage="No enabled Tools available."
            searchLabel="Tools"
            summaryItems={[
              { label: "Direct", value: localAgent.toolIds.length },
              { label: "From Skills", value: skillToolIds.size },
              { label: "Available", value: tools.length }
            ]}
          />
        ) : (
          <BoundResourceList emptyMessage="No Tools bound." items={boundTools} title="Tools" />
        )}
      </NodeConfigSection>

      {isWorkflowFlow && onWorkflowRowsChange ? (
        <>
          <NodeConfigSection title="Input Protocol" meta={`${workflowRows.inputRows.length} fields`}>
            <p className="workflow-muted-note">
              Define the Agent input object in one place: field contract, business meaning, and how each value is read from workflow context.
            </p>
            <AgentInputProtocolRows
              fields={draftInputSchemaFields}
              mappingRows={workflowRows.inputRows}
              sourceOptions={contextReadOptions}
              onChange={updateAgentInputProtocol}
            />
          </NodeConfigSection>

          <NodeConfigSection title="Output Protocol" meta={typeof selectedNode.data.config?.output_mode === "string" ? selectedNode.data.config.output_mode : "text"}>
            <label>
              Output Mode
              <select
                value={typeof selectedNode.data.config?.output_mode === "string" ? selectedNode.data.config.output_mode : "text"}
                onChange={(event) =>
                  onNodeChange({
                    config: {
                      ...(selectedNode.data.config ?? {}),
                      output_mode: event.target.value
                    }
                  })
                }
              >
                <option value="text">Text</option>
                <option value="json">JSON</option>
              </select>
            </label>
            <p className="workflow-muted-note">
              In JSON mode, the Agent should return an object matching this schema. In Text mode, the runtime exposes the final text as <code>$.text</code> for Write Context.
            </p>
            <ContextFieldRows fields={draftOutputSchemaFields} onChange={updateAgentOutputSchemaFields} />
          </NodeConfigSection>

          <NodeConfigSection title="Routing" meta={typeof selectedNode.data.config?.agent_routing_mode === "string" ? selectedNode.data.config.agent_routing_mode : "flow"}>
            <label>
              Routing Mode
              <select
                value={typeof selectedNode.data.config?.agent_routing_mode === "string" ? selectedNode.data.config.agent_routing_mode : "flow"}
                onChange={(event) =>
                  onNodeChange({
                    config: {
                      ...(selectedNode.data.config ?? {}),
                      agent_routing_mode: event.target.value
                    }
                  })
                }
              >
                <option value="flow">Flow / Condition controls next step</option>
                <option value="agent_directed">Agent selects connected next node</option>
              </select>
            </label>
            <p className="workflow-muted-note">
              Agent-directed routing expects JSON with <code>next_node_ids</code>. The runtime only accepts nodes connected by outgoing edges.
            </p>
          </NodeConfigSection>

          <NodeConfigSection title="Write Context" meta={`${workflowRows.contextRows.length} writes`}>
            <p className="workflow-muted-note">
              After the Agent finishes, the flow engine writes selected Agent output fields into shared context. Use <code>$.field</code> for JSON fields or <code>$.text</code> for text output.
            </p>
            <FlowMappingRows
              rows={workflowRows.contextRows}
              sourceOptions={agentOutputReadOptions}
              targetOptions={contextWriteOptions}
              onChange={(nextRows) => onWorkflowRowsChange("contextRows", nextRows)}
            />
          </NodeConfigSection>
        </>
      ) : null}
    </>
  );
}

function WorkflowNodeConfigSections({
  agents,
  connectorOperations,
  onConfigChange,
  onNodeChange,
  onRowsChange,
  nodeOptions,
  selectedNode,
  sourceOptions,
  skills,
  targetOptions,
  tools
}: {
  agents: AgentProfile[];
  connectorOperations: ConnectorOperation[];
  onConfigChange: (key: string, value: unknown) => void;
  onNodeChange: (patch: Partial<AgentFlowCanvasNodeData>) => void;
  onRowsChange: (kind: keyof NodeConfigRows, rows: MappingRowDraft[]) => void;
  nodeOptions: MappingOption[];
  selectedNode: AgentFlowCanvasNode;
  sourceOptions: MappingOption[];
  skills: SkillSpec[];
  targetOptions: MappingOption[];
  tools: ToolSpec[];
}) {
  const workflowNode = workflowNodeFromCanvasNode(selectedNode);
  const rows = nodeConfigRows(workflowNode);
  return (
    <>
      <NodeConfigSection defaultOpen title="Basic" meta={nodeTypeLabel(workflowNode.type)}>
        <label>
          Name
          <input value={selectedNode.data.label} onChange={(event) => onNodeChange({ label: event.target.value })} />
        </label>
        <label>
          Description
          <textarea value={selectedNode.data.description ?? ""} onChange={(event) => onNodeChange({ description: event.target.value })} />
        </label>
      </NodeConfigSection>

      {workflowNode.type === "condition" ? (
        <WorkflowPrimitiveBinding
          agents={agents}
          connectorOperations={connectorOperations}
          node={workflowNode}
          nodeOptions={nodeOptions}
          sourceOptions={sourceOptions}
          targetOptions={targetOptions}
          skills={skills}
          tools={tools}
          onConfigChange={onConfigChange}
        />
      ) : (
        <NodeConfigSection defaultOpen title="Binding" meta={workflowNode.type}>
          <WorkflowPrimitiveBinding
            agents={agents}
            connectorOperations={connectorOperations}
            node={workflowNode}
            nodeOptions={nodeOptions}
            sourceOptions={sourceOptions}
            targetOptions={targetOptions}
            skills={skills}
            tools={tools}
            onConfigChange={onConfigChange}
          />
        </NodeConfigSection>
      )}

      {workflowNode.type !== "condition" ? (
        <>
          <NodeConfigSection title="Input Mapping" meta={`${rows.inputRows.length} fields`}>
            <FlowMappingRows rows={rows.inputRows} sourceOptions={sourceOptions} onChange={(nextRows) => onRowsChange("inputRows", nextRows)} />
          </NodeConfigSection>

          <NodeConfigSection title="Write Context" meta={`${rows.contextRows.length} writes`}>
            <FlowMappingRows rows={rows.contextRows} sourceOptions={sourceOptions} targetOptions={targetOptions} onChange={(nextRows) => onRowsChange("contextRows", nextRows)} />
          </NodeConfigSection>

          <NodeConfigSection title="Output Mapping" meta={`${rows.outputRows.length} fields`}>
            <FlowMappingRows rows={rows.outputRows} sourceOptions={sourceOptions} onChange={(nextRows) => onRowsChange("outputRows", nextRows)} />
          </NodeConfigSection>
        </>
      ) : null}
    </>
  );
}

function WorkflowPrimitiveBinding({
  agents,
  connectorOperations,
  node,
  nodeOptions,
  onConfigChange,
  skills,
  sourceOptions,
  targetOptions,
  tools
}: {
  agents: AgentProfile[];
  connectorOperations: ConnectorOperation[];
  node: WorkflowNode;
  nodeOptions?: MappingOption[];
  onConfigChange: (key: string, value: unknown) => void;
  skills: SkillSpec[];
  sourceOptions?: MappingOption[];
  targetOptions?: MappingOption[];
  tools: ToolSpec[];
}) {
  const config = node.config ?? {};
  const value = (key: string) => (typeof config[key] === "string" ? (config[key] as string) : "");
  switch (node.type) {
    case "connector_operation":
      return (
        <div className="workflow-binding-stack">
          <label>
            Connector Operation
            <select value={value("connector_operation_id") || value("operation_id")} onChange={(event) => onConfigChange("connector_operation_id", event.target.value)}>
              <option value="">Select operation</option>
              {connectorOperations.map((operation) => (
                <option key={operation.id} value={operation.id}>
                  {operation.name} · {operation.id}
                </option>
              ))}
            </select>
          </label>
          <label>
            Response Alias
            <input value={value("response_alias")} onChange={(event) => onConfigChange("response_alias", event.target.value)} placeholder="operation_step_1" />
          </label>
          <p className="workflow-muted-note">Use alias when this operation is called multiple times in the same workflow.</p>
        </div>
      );
    case "tool":
      return (
        <div className="workflow-binding-stack">
          <label>
            Tool
            <select value={value("tool_id")} onChange={(event) => onConfigChange("tool_id", event.target.value)}>
              <option value="">Select tool</option>
              {tools.map((tool) => (
                <option key={tool.id} value={tool.id}>
                  {tool.name} · {tool.id}
                </option>
              ))}
            </select>
          </label>
          <label>
            Response Alias
            <input value={value("response_alias")} onChange={(event) => onConfigChange("response_alias", event.target.value)} placeholder="tool_step_1" />
          </label>
          <p className="workflow-muted-note">Use alias when this tool is called multiple times in the same workflow.</p>
        </div>
      );
    case "skill":
      return (
        <label>
          Skill
          <select value={value("skill_id")} onChange={(event) => onConfigChange("skill_id", event.target.value)}>
            <option value="">Select skill</option>
            {skills.map((skill) => (
              <option key={skill.id} value={skill.id}>
                {skill.name} · {skill.id}
              </option>
            ))}
          </select>
        </label>
      );
    case "agent":
      return (
        <label>
          Agent
          <select value={value("agent_id")} onChange={(event) => onConfigChange("agent_id", event.target.value)}>
            <option value="">Select agent</option>
            {agents.map((agent) => (
              <option key={agent.id} value={agent.id}>
                {agent.name} · {agent.id}
              </option>
            ))}
          </select>
        </label>
      );
    case "transform": {
      const selectedFunction = transformFunctionById(value("function_id"));
      return (
        <div className="workflow-binding-stack">
          <label>
            Transform Function
            <select
              value={value("function_id")}
              onChange={(event) => {
                onConfigChange("function_id", event.target.value);
                onConfigChange("function_version", transformFunctionById(event.target.value)?.version ?? "1.0.0");
              }}
            >
              <option value="">Select transform function</option>
              {transformFunctionDefinitions.map((fn) => (
                <option key={fn.id} value={fn.id}>
                  {fn.name} · {fn.category}
                </option>
              ))}
            </select>
          </label>
          {selectedFunction ? (
            <div className="workflow-transform-function-card">
              <strong>{selectedFunction.name}</strong>
              <span>{selectedFunction.id}</span>
              <p>{selectedFunction.description}</p>
              <small>
                Inputs: {selectedFunction.inputFields.map((field) => field.value).join(", ")} · Outputs:{" "}
                {selectedFunction.outputFields.map((field) => field.value).join(", ")}
              </small>
            </div>
          ) : (
            <p className="workflow-muted-note">Choose one deterministic data processing function for this node.</p>
          )}
        </div>
      );
    }
    case "condition":
      return (
        <ConditionBranchEditor
          contextReadOptions={sourceOptions ?? []}
          contextWriteOptions={targetOptions ?? []}
          node={node}
          nodeOptions={nodeOptions ?? []}
          onConfigChange={onConfigChange}
        />
      );
    default:
      return <p className="workflow-muted-note">This node is configured by mappings and graph edges.</p>;
  }
}

function ConditionBranchEditor({
  contextReadOptions,
  contextWriteOptions,
  node,
  nodeOptions,
  onConfigChange
}: {
  contextReadOptions: MappingOption[];
  contextWriteOptions: MappingOption[];
  node: WorkflowNode;
  nodeOptions: MappingOption[];
  onConfigChange: (key: string, value: unknown) => void;
}) {
  const branches = conditionBranchesFromConfig(node.config);
  const defaultBranch = conditionDefaultBranchFromConfig(node.config);
  const [activeTab, setActiveTab] = useState(() => branches[0]?.id ?? "default");
  useEffect(() => {
    if (activeTab === "default") return;
    if (!branches.some((branch) => branch.id === activeTab)) {
      setActiveTab(branches[0]?.id ?? "default");
    }
  }, [activeTab, branches]);
  const commitBranches = (nextBranches: ConditionBranchDraft[]) => onConfigChange("branches", nextBranches.map(serializeConditionBranch));
  const commitDefaultBranch = (nextDefault: ConditionDefaultBranchDraft) => onConfigChange("default_branch", serializeConditionDefaultBranch(nextDefault));
  const updateBranch = (branchId: string, patch: Partial<ConditionBranchDraft>) => {
    commitBranches(branches.map((branch) => (branch.id === branchId ? { ...branch, ...patch } : branch)));
  };
  const addBranch = () => {
    const branch = createConditionBranchDraft({ name: `Branch ${branches.length + 1}` });
    commitBranches([...branches, branch]);
    setActiveTab(branch.id);
  };
  const activeBranch = branches.find((branch) => branch.id === activeTab);
  return (
    <div className="workflow-binding-stack">
      <p className="workflow-muted-note">Branches are evaluated in order. Empty Next Node means the workflow ends after this branch writes context.</p>
      <div className="workflow-condition-tabs">
        {branches.map((branch, index) => (
          <button
            className={activeTab === branch.id ? "workflow-condition-tab-active" : ""}
            key={branch.id}
            type="button"
            onClick={() => setActiveTab(branch.id)}
          >
            {branch.name || `Branch ${index + 1}`}
          </button>
        ))}
        <button className={activeTab === "default" ? "workflow-condition-tab-active" : ""} type="button" onClick={() => setActiveTab("default")}>
          Default
        </button>
      </div>
      {activeBranch ? (
        <div className="workflow-condition-tab-panel" key={activeBranch.id}>
          <div className="workflow-condition-branch-header">
            <strong>{activeBranch.name || "Branch"}</strong>
            <button type="button" onClick={() => commitBranches(branches.filter((item) => item.id !== activeBranch.id))} aria-label="Remove branch">
              ×
            </button>
          </div>
          <label>
            Branch Name
            <input value={activeBranch.name} onChange={(event) => updateBranch(activeBranch.id, { name: event.target.value })} />
          </label>
          <label>
            Match Mode
            <select value={activeBranch.mode} onChange={(event) => updateBranch(activeBranch.id, { mode: event.target.value as "all" | "any" })}>
              <option value="all">All rules match</option>
              <option value="any">Any rule matches</option>
            </select>
          </label>
          <ConditionRuleRows
            contextReadOptions={contextReadOptions}
            onChange={(rules) => updateBranch(activeBranch.id, { rules })}
            rules={activeBranch.rules}
          />
          <label>
            Next Node
            <select value={activeBranch.nextNodeId} onChange={(event) => updateBranch(activeBranch.id, { nextNodeId: event.target.value })}>
              <option value="">End workflow</option>
              {nodeOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label} · {option.detail}
                </option>
              ))}
            </select>
          </label>
          <div>
            <strong>Write Context</strong>
            <FlowMappingRows
              rows={activeBranch.writeRows}
              sourceOptions={contextReadOptions}
              targetOptions={contextWriteOptions}
              onChange={(rows) => updateBranch(activeBranch.id, { writeRows: rows })}
            />
          </div>
        </div>
      ) : null}
      <button className="secondary-action" type="button" onClick={addBranch}>
        + Branch
      </button>
      {activeTab === "default" ? (
        <div className="workflow-condition-tab-panel">
        <strong>Default Branch</strong>
        <label>
          Next Node
          <select value={defaultBranch.nextNodeId} onChange={(event) => commitDefaultBranch({ ...defaultBranch, nextNodeId: event.target.value })}>
            <option value="">End workflow</option>
            {nodeOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label} · {option.detail}
              </option>
            ))}
          </select>
        </label>
        <div>
          <strong>Write Context</strong>
          <FlowMappingRows
            rows={defaultBranch.writeRows}
            sourceOptions={contextReadOptions}
            targetOptions={contextWriteOptions}
            onChange={(rows) => commitDefaultBranch({ ...defaultBranch, writeRows: rows })}
          />
        </div>
      </div>
      ) : null}
    </div>
  );
}

function ConditionRuleRows({
  contextReadOptions,
  onChange,
  rules
}: {
  contextReadOptions: MappingOption[];
  onChange: (rules: ConditionRuleDraft[]) => void;
  rules: ConditionRuleDraft[];
}) {
  const update = (ruleId: string, patch: Partial<ConditionRuleDraft>) => {
    onChange(rules.map((rule) => (rule.id === ruleId ? { ...rule, ...patch } : rule)));
  };
  return (
    <div className="workflow-mapping-block">
      <div className="workflow-mapping-table workflow-mapping-table-with-mode workflow-condition-rule-table">
        <span>Left</span>
        <span>Operator</span>
        <span>Right</span>
        <span />
        {rules.map((rule) => (
          <div className="workflow-table-row" key={rule.id}>
            <FlowExpressionInput options={contextReadOptions} value={rule.left} onChange={(value) => update(rule.id, { left: value })} />
            <select value={rule.operator} onChange={(event) => update(rule.id, { operator: event.target.value })}>
              {conditionOperatorOptions.map((operator) => (
                <option key={operator.value} value={operator.value}>
                  {operator.label}
                </option>
              ))}
            </select>
            <FlowExpressionInput options={contextReadOptions} value={rule.right} onChange={(value) => update(rule.id, { right: value })} />
            <button type="button" onClick={() => onChange(rules.filter((item) => item.id !== rule.id))} aria-label="Remove rule">
              ×
            </button>
          </div>
        ))}
      </div>
      <button className="secondary-action" type="button" onClick={() => onChange([...rules, createConditionRuleDraft()])}>
        + Rule
      </button>
    </div>
  );
}

function FlowMappingRows({
  onChange,
  rows,
  sourceOptions = [],
  targetOptions = []
}: {
  onChange: (rows: MappingRowDraft[]) => void;
  rows: MappingRowDraft[];
  sourceOptions?: MappingOption[];
  targetOptions?: MappingOption[];
}) {
  const targetDatalistId = useId();
  const [draftRows, setDraftRows] = useState(rows);
  const rowsSignature = rows.map((row) => `${row.id}:${row.target}:${row.source}`).join("|");
  useEffect(() => {
    setDraftRows(rows);
  }, [rowsSignature]);
  const commitRows = (nextRows: MappingRowDraft[]) => {
    onChange(nextRows.filter((row) => row.target.trim()));
  };
  const update = (rowId: string, patch: Partial<MappingRowDraft>) => {
    const nextRows = draftRows.map((row) => (row.id === rowId ? { ...row, ...patch } : row));
    setDraftRows(nextRows);
    const updatedRow = nextRows.find((row) => row.id === rowId);
    if (patch.target !== undefined && !updatedRow?.target.trim()) return;
    commitRows(nextRows);
  };
  const addRow = () => {
    const nextRows = [...draftRows, createMappingRow({ target: nextMappingTarget(draftRows, targetOptions) })];
    setDraftRows(nextRows);
    commitRows(nextRows);
  };
  const removeRow = (rowId: string) => {
    const nextRows = draftRows.filter((item) => item.id !== rowId);
    setDraftRows(nextRows);
    commitRows(nextRows);
  };
  return (
    <div className="workflow-mapping-block">
      <div className="workflow-mapping-table workflow-mapping-table-with-mode">
        <span>Target</span>
        <span>Expression / Value</span>
        <span />
        {draftRows.map((row) => (
          <div className="workflow-table-row" key={row.id}>
            <input
              value={row.target}
              list={targetOptions.length > 0 ? targetDatalistId : undefined}
              onChange={(event) => update(row.id, { target: event.target.value })}
              placeholder="query"
            />
            <FlowExpressionInput options={sourceOptions} value={row.source} onChange={(value) => update(row.id, { source: value })} />
            <button type="button" onClick={() => removeRow(row.id)} aria-label="Remove mapping">
              ×
            </button>
          </div>
        ))}
      </div>
      <datalist id={targetDatalistId}>
        {targetOptions.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </datalist>
      <button className="secondary-action" type="button" onClick={addRow}>
        + Mapping
      </button>
    </div>
  );
}

function nextMappingTarget(rows: MappingRowDraft[], targetOptions: MappingOption[] = []): string {
  const existing = new Set(rows.map((row) => row.target.trim()).filter(Boolean));
  const firstUnusedOption = targetOptions.find((option) => option.value && !existing.has(option.value));
  if (firstUnusedOption) return firstUnusedOption.value;
  let index = rows.length + 1;
  let candidate = `field_${index}`;
  while (existing.has(candidate)) {
    index += 1;
    candidate = `field_${index}`;
  }
  return candidate;
}

function createConditionRuleDraft(seed: Partial<ConditionRuleDraft> = {}): ConditionRuleDraft {
  return {
    id: seed.id ?? `cond_rule_${Date.now()}_${Math.random().toString(16).slice(2, 8)}`,
    left: seed.left ?? "",
    operator: seed.operator ?? "equals",
    right: seed.right ?? ""
  };
}

function createConditionBranchDraft(seed: Partial<ConditionBranchDraft> = {}): ConditionBranchDraft {
  return {
    id: seed.id ?? `cond_branch_${Date.now()}_${Math.random().toString(16).slice(2, 8)}`,
    name: seed.name ?? "Branch",
    mode: seed.mode ?? "all",
    rules: seed.rules ?? [createConditionRuleDraft()],
    writeRows: seed.writeRows ?? [],
    nextNodeId: seed.nextNodeId ?? ""
  };
}

function conditionBranchesFromConfig(config: Record<string, unknown> | undefined): ConditionBranchDraft[] {
  const rawBranches = Array.isArray(config?.branches) ? config.branches : [];
  return rawBranches.map((raw, index) => conditionBranchFromUnknown(raw, index));
}

function conditionBranchFromUnknown(value: unknown, index: number): ConditionBranchDraft {
  const record = objectRecord(value) ?? {};
  const rules = Array.isArray(record.rules) ? record.rules.map(conditionRuleFromUnknown) : [createConditionRuleDraft()];
  return createConditionBranchDraft({
    id: typeof record.id === "string" && record.id ? record.id : `cond_branch_${index}`,
    name: typeof record.name === "string" ? record.name : `Branch ${index + 1}`,
    mode: record.mode === "any" ? "any" : "all",
    rules: rules.length > 0 ? rules : [createConditionRuleDraft()],
    writeRows: mappingRowsFromRecord(objectRecord(record.write_context ?? record.context_writes)),
    nextNodeId: typeof record.next_node_id === "string" ? record.next_node_id : ""
  });
}

function conditionRuleFromUnknown(value: unknown, index: number): ConditionRuleDraft {
  const record = objectRecord(value) ?? {};
  return createConditionRuleDraft({
    id: typeof record.id === "string" && record.id ? record.id : `cond_rule_${index}`,
    left: conditionValueToInput(record.left ?? record.path),
    operator: typeof record.operator === "string" ? record.operator : typeof record.op === "string" ? record.op : "equals",
    right: conditionValueToInput(record.right ?? record.value)
  });
}

function conditionDefaultBranchFromConfig(config: Record<string, unknown> | undefined): ConditionDefaultBranchDraft {
  const record = objectRecord(config?.default_branch) ?? {};
  return {
    writeRows: mappingRowsFromRecord(objectRecord(record.write_context ?? record.context_writes)),
    nextNodeId: typeof record.next_node_id === "string" ? record.next_node_id : ""
  };
}

function serializeConditionBranch(branch: ConditionBranchDraft): Record<string, unknown> {
  return {
    id: branch.id,
    name: branch.name.trim() || "Branch",
    mode: branch.mode,
    rules: branch.rules.map(serializeConditionRule),
    write_context: recordFromMappingRows(branch.writeRows),
    next_node_id: branch.nextNodeId
  };
}

function serializeConditionDefaultBranch(branch: ConditionDefaultBranchDraft): Record<string, unknown> {
  return {
    write_context: recordFromMappingRows(branch.writeRows),
    next_node_id: branch.nextNodeId
  };
}

function serializeConditionRule(rule: ConditionRuleDraft): Record<string, unknown> {
  return {
    id: rule.id,
    left: conditionInputToValue(rule.left),
    operator: rule.operator || "equals",
    right: conditionInputToValue(rule.right)
  };
}

function conditionValueToInput(value: unknown): string {
  if (typeof value === "string") return value;
  if (value === undefined || value === null) return "";
  return JSON.stringify(value);
}

function conditionInputToValue(value: string): unknown {
  const trimmed = value.trim();
  if (!trimmed) return "";
  if (trimmed.startsWith("$")) return trimmed;
  try {
    return JSON.parse(trimmed);
  } catch {
    return value;
  }
}

function objectRecord(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;
}

function FlowExpressionInput({ onChange, options = [], value }: { onChange: (value: string) => void; options?: MappingOption[]; value: string }) {
  const datalistId = useId();
  const [mode, setMode] = useState<"constant" | "variable">(() => flowExpressionMode(value, options));
  useEffect(() => {
    if (value.trim()) {
      setMode(flowExpressionMode(value, options));
    }
  }, [options, value]);
  const switchMode = (nextMode: "constant" | "variable") => {
    if (nextMode === mode) return;
    setMode(nextMode);
    if (nextMode === "variable") {
      onChange(value.trim().startsWith("$") ? value : options[0]?.value ?? "$flow_input.user_request");
      return;
    }
    if (value.trim().startsWith("$")) {
      onChange("");
    }
  };
  return (
    <div className="workflow-expression-composer">
      <select value={mode} onChange={(event) => switchMode(event.target.value as "constant" | "variable")} aria-label="Expression mode">
        <option value="variable">Var</option>
        <option value="constant">Const</option>
      </select>
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        list={mode === "variable" && options.length > 0 ? datalistId : undefined}
        placeholder={mode === "variable" ? "$flow_input.user_request" : "[\"merge_info\"]"}
      />
      <datalist id={datalistId}>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </datalist>
    </div>
  );
}

function flowExpressionMode(value: string, options: MappingOption[] = []): "constant" | "variable" {
  const trimmed = value.trim();
  if (!trimmed) return "variable";
  if (trimmed.startsWith("$")) return "variable";
  if (options.some((option) => option.value === trimmed)) return "variable";
  return "constant";
}

function workflowNodeFromCanvasNode(node: AgentFlowCanvasNode): WorkflowNode {
  const config = node.data.config ?? {};
  return {
    id: node.id,
    type: normalizeWorkflowNodeType(node.data.nodeType),
    name: node.data.label,
    description: node.data.description,
    position: node.position,
    config,
    timeoutMillis: node.data.timeoutMillis
  };
}

function normalizeWorkflowNodeType(nodeType: AgentFlowNodeType): WorkflowNode["type"] {
  if (nodeType === "agent_node" || nodeType === "supervisor_node") return "agent";
  if (nodeType === "join_node") return "join";
  if (nodeType === "planner_node" || nodeType === "router_node" || nodeType === "aggregator_node" || nodeType === "verifier_node") {
    return "transform";
  }
  return nodeType as WorkflowNode["type"];
}

function BoundResourceList({
  emptyMessage,
  items,
  title
}: {
  emptyMessage: string;
  items: Array<{ id: string; name: string; description?: string }>;
  title: string;
}) {
  return (
    <div className="bound-agent-resource-list">
      <span>{title}</span>
      {items.length > 0 ? (
        <div>
          {items.map((item) => (
            <strong key={item.id} title={item.description || item.id}>
              {item.name}
            </strong>
          ))}
        </div>
      ) : (
        <small>{emptyMessage}</small>
      )}
    </div>
  );
}

function SkillChecklist({
  onToggle,
  selectedSkillIds,
  skills
}: {
  onToggle: (skillId: string, checked: boolean) => void;
  selectedSkillIds: string[];
  skills: SkillSpec[];
}) {
  const skillSet = new Set(selectedSkillIds);
  return (
    <div className="local-skill-list">
      {skills.map((skill) => (
        <label key={skill.id} className={skillSet.has(skill.id) ? "local-skill-row local-skill-row-active" : "local-skill-row"}>
          <input
            checked={skillSet.has(skill.id)}
            type="checkbox"
            onChange={(event) => onToggle(skill.id, event.target.checked)}
          />
          <span>
            <strong>{skill.name}</strong>
            <small>{skill.description}</small>
          </span>
        </label>
      ))}
      {skills.length === 0 ? <p className="muted-copy">No Skills available.</p> : null}
    </div>
  );
}

function localAgentDefaults(name: string): LocalAgentConfig {
  return {
    name,
    description: "",
    model: "deepseek-v4-flash",
    systemPrompt: "",
    skillIds: [],
    toolIds: []
  };
}

function toggleID(ids: string[], id: string, checked: boolean): string[] {
  if (checked) return Array.from(new Set([...ids, id]));
  return ids.filter((item) => item !== id);
}

function resolveResources<T extends { id: string; name: string; description?: string }>(ids: string[], resources: T[]) {
  return ids.map((id) => {
    const resource = resources.find((item) => item.id === id);
    return resource ? { id: resource.id, name: resource.name, description: resource.description } : { id, name: id };
  });
}

function FlowNode({ data, id, selected }: NodeProps<AgentFlowCanvasNode>) {
  const agentType = data.agentMode === "local" ? "Local Agent" : data.agentId ? "Existing Agent" : nodeLabel(data.nodeType);
  const model = data.agentMode === "local" ? data.localAgent?.model || "model" : data.agentId ? "bound profile" : "";
  return (
    <div className={selected ? "flow-canvas-node flow-canvas-node-selected" : "flow-canvas-node"}>
      {data.deletable !== false && data.onDelete ? (
        <button
          className="flow-node-delete"
          type="button"
          aria-label={`Delete ${data.label}`}
          onClick={(event) => {
            event.stopPropagation();
            data.onDelete?.(id);
          }}
          onPointerDown={(event) => event.stopPropagation()}
        >
          ×
        </button>
      ) : null}
      <Handle type="target" position={Position.Left} />
      <span className={`flow-node-type flow-node-type-${data.nodeType.replace("_node", "")}`}>{nodeLabel(data.nodeType)}</span>
      <strong>{data.label}</strong>
      {data.description ? <p className="flow-node-description">{data.description}</p> : null}
      {isAgentLikeNode(data.nodeType) ? (
        <div className="flow-node-meta">
          <span>{agentType}</span>
          {model ? <code>{model}</code> : null}
        </div>
      ) : null}
      <Handle type="source" position={Position.Right} />
    </div>
  );
}

function NodeDebugChat({
  activeTrace,
  emptyDescription,
  emptyTitle,
  messages,
  onOpenTrace,
  onOpenTraceId
}: {
  activeTrace: AgentTrace | null;
  emptyDescription: string;
  emptyTitle: string;
  messages: DebugChatMessage[];
  onOpenTrace: (trace: AgentTrace) => void;
  onOpenTraceId?: (traceId: string) => void | Promise<void>;
}) {
  return (
    <div className="agent-chat-window">
      {messages.length === 0 ? (
        <div className="agent-chat-empty">
          <strong>{emptyTitle}</strong>
          <p>{emptyDescription}</p>
        </div>
      ) : (
        messages.map((message) => (
          <ChatBubble
            active={message.traceId !== undefined && message.traceId === activeTrace?.traceId}
            key={message.id}
            message={message}
            onOpenTrace={onOpenTrace}
            onOpenTraceId={onOpenTraceId}
          />
        ))
      )}
    </div>
  );
}

function inputFromDebugText(text: string): Record<string, unknown> {
  const trimmed = text.trim();
  return {
    user_request: trimmed
  };
}

function wouldCreateCycle(sourceNodeId: string, targetNodeId: string, edges: AgentFlowCanvasEdge[]): boolean {
  const adjacency = new Map<string, string[]>();
  edges.forEach((edge) => {
    adjacency.set(edge.source, [...(adjacency.get(edge.source) ?? []), edge.target]);
  });

  const stack = [targetNodeId];
  const visited = new Set<string>();
  while (stack.length > 0) {
    const current = stack.pop();
    if (!current || visited.has(current)) continue;
    if (current === sourceNodeId) return true;
    visited.add(current);
    stack.push(...(adjacency.get(current) ?? []));
  }
  return false;
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

function processMessageText(events: RuntimeEvent[]): string {
  if (events.some((event) => event.type === "action_planned" || event.type === "trace_step_added")) {
    return "处理过程";
  }
  return "处理过程：执行完成。";
}

function withLiveAgentTraceIds(flow: AgentFlowSpec, sessionId: string): AgentFlowSpec {
  const nodes = Object.fromEntries(
    Object.entries(flow.graph.nodes).map(([nodeId, node]) => {
      if (!isAgentLikeNode(node.type)) return [nodeId, node];
      return [
        nodeId,
        {
          ...node,
          config: {
            ...(node.config ?? {}),
            trace_id: `trace_${sessionId}_${nodeId}`.replace(/[^a-zA-Z0-9_-]/g, "_")
          }
        }
      ];
    })
  );
  return {
    ...flow,
    graph: {
      ...flow.graph,
      nodes
    }
  };
}

function collectAgentTraceIds(flow: AgentFlowSpec): string[] {
  return Object.values(flow.graph.nodes)
    .filter((node) => isAgentLikeNode(node.type))
    .map((node) => (typeof node.config?.trace_id === "string" ? node.config.trace_id : ""))
    .filter(Boolean);
}

function createRuntimeEventDecorator(flow: AgentFlowSpec, agents: AgentProfile[], skills: SkillSpec[], tools: ToolSpec[]) {
  return (event: RuntimeEvent): RuntimeEvent => {
    const payload = cloneRuntimePayload(event.payload);
    const action = objectFromUnknown(payload.action);
    if (action) {
      payload.action = decorateRuntimeAction(action, flow, agents, skills, tools);
    }
    if (Array.isArray(payload.actions)) {
      payload.actions = payload.actions.map((item) => {
        const actionItem = objectFromUnknown(item);
        return actionItem ? decorateRuntimeAction(actionItem, flow, agents, skills, tools) : item;
      });
    }
    return {
      ...event,
      payload
    };
  };
}

function decorateRuntimeAction(action: Record<string, unknown>, flow: AgentFlowSpec, agents: AgentProfile[], skills: SkillSpec[], tools: ToolSpec[]) {
  if (typeof action.target_name === "string" && action.target_name.trim()) return action;
  const actionType = typeof action.type === "string" ? action.type : "";
  const nodeId = firstRuntimeString(action.node_id, action.nodeId);
  const agentId = firstRuntimeString(action.agent_id, action.agentId, action.target_id, action.targetId, action.id);
  const toolId = firstRuntimeString(action.tool_id, action.toolId, action.target_id, action.targetId, action.id);
  const skillId = firstRuntimeString(action.skill_id, action.skillId, action.target_id, action.targetId, action.id);
  const node = nodeId ? flow.graph.nodes[nodeId] : undefined;
  const agent = agentId ? agents.find((item) => item.id === agentId) : undefined;
  const tool = toolId ? tools.find((item) => item.id === toolId) : undefined;
  const skill = skillId ? skills.find((item) => item.id === skillId) : undefined;
  const targetName =
    (actionType === "agent" ? nodeDisplayName(node, agent) : "") ||
    (actionType === "tool" ? tool?.name : "") ||
    (actionType === "skill" ? skill?.name : "") ||
    nodeDisplayName(node, agent) ||
    tool?.name ||
    skill?.name;
  if (!targetName) return action;
  return {
    ...action,
    target_name: targetName
  };
}

function nodeDisplayName(node: AgentFlowSpec["graph"]["nodes"][string] | undefined, agent?: AgentProfile): string {
  if (!node) return agent?.name ?? "";
  const localAgent = objectFromUnknown(node.config?.local_agent) ?? objectFromUnknown(node.config?.localAgent);
  return firstRuntimeString(localAgent?.name, node.name, agent?.name, node.id);
}

function cloneRuntimePayload(payload: RuntimeEvent["payload"]): Record<string, unknown> {
  return payload ? { ...payload } : {};
}

function objectFromUnknown(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function firstRuntimeString(...values: unknown[]): string {
  for (const value of values) {
    if (typeof value === "string" && value.trim()) return value.trim();
  }
  return "";
}
