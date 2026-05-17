import { useEffect, useMemo, useState } from "react";
import { defaultTenantId, platformApi } from "../../lib/api";
import { connectorOperations, connectors, toolDependencies, tools } from "../../lib/mockData";
import type { Connector, ConnectorOperation, ToolDependencies, ToolExecutionResult, ToolSpec, WorkflowSpec } from "../../types/platform";
import { useAutoDismissNotice, type NoticeState } from "../common/useAutoDismissNotice";
import { toolsClient } from "./api";
import {
  createToolDraft,
  createToolDraftForImplementation,
  draftFromTool,
  emptyToolDependencies,
  sampleArgsFromFields,
  toolFromDraft,
  type ToolDraft
} from "./domain";

type Notice = NoticeState;

export function useTools() {
  const [toolItems, setToolItems] = useState<ToolSpec[]>(tools);
  const [connectorSystems, setConnectorSystems] = useState<Connector[]>(connectors);
  const [connectorItems, setConnectorItems] = useState<ConnectorOperation[]>(connectorOperations);
  const [workflowItems, setWorkflowItems] = useState<WorkflowSpec[]>([]);
  const [selectedToolId, setSelectedToolId] = useState(tools[0]?.id ?? "");
  const [draft, setDraft] = useState<ToolDraft>(() => (tools[0] ? draftFromTool(tools[0]) : createToolDraft()));
  const [dependenciesByTool, setDependenciesByTool] = useState<Record<string, ToolDependencies>>(toolDependencies);
  const [testArgsJson, setTestArgsJson] = useState('{\n  "order_id": "o_123"\n}');
  const [testConfirmed, setTestConfirmed] = useState(false);
  const [testResult, setTestResult] = useState<ToolExecutionResult | null>(null);
  const [notice, setNotice] = useAutoDismissNotice<Notice>();

  const selectedTool = useMemo(
    () => toolItems.find((tool) => tool.id === selectedToolId),
    [toolItems, selectedToolId]
  );
  const selectedDependencies = selectedTool
    ? dependenciesByTool[selectedTool.id] ?? emptyToolDependencies(selectedTool.id)
    : emptyToolDependencies("");

  useEffect(() => {
    void refresh();
  }, []);

  useEffect(() => {
    if (!selectedTool) {
      // An empty selectedToolId means the user is creating a new draft. Keep the
      // draft initialized by startNewTool instead of overwriting the chosen type.
      if (selectedToolId === "") return;
      const nextDraft = createToolDraft();
      setDraft(nextDraft);
      setTestArgsJson(JSON.stringify(sampleArgsFromFields(nextDraft.inputFields), null, 2));
      return;
    }
    const nextDraft = draftFromTool(selectedTool);
    setDraft(nextDraft);
    setTestArgsJson(JSON.stringify(sampleArgsFromFields(nextDraft.inputFields), null, 2));
    void loadDependencies(selectedTool.id);
  }, [selectedTool?.id, selectedToolId]);

  async function refresh() {
    try {
      const [remoteTools, remoteConnectors, remoteWorkflows] = await Promise.all([
        toolsClient.listTools(),
        Promise.all([
          platformApi.listConnectors().then((response) => response.items),
          platformApi.listConnectorOperations().then((response) => response.items)
        ]),
        platformApi.listWorkflows({ profile: "tool_workflow", status: "all" }).then((response) => response.items)
      ]);
      // Once the backend responds successfully, it becomes the source of truth.
      // Empty remote lists are meaningful and must clear the initial mock data.
      const dedupedTools = dedupeToolsByStableKey(remoteTools);
      setToolItems(dedupedTools);
      setSelectedToolId((current) => (dedupedTools.some((tool) => tool.id === current) ? current : dedupedTools[0]?.id ?? ""));
      setConnectorSystems(remoteConnectors[0]);
      setConnectorItems(remoteConnectors[1]);
      setWorkflowItems(remoteWorkflows);
    } catch {
      setNotice({ ok: false, message: "Using local tool mock data because backend APIs are unavailable." });
    }
  }

  async function loadDependencies(toolId: string) {
    try {
      const dependencies = await toolsClient.getDependencies(toolId);
      setDependenciesByTool((current) => ({ ...current, [toolId]: dependencies }));
    } catch {
      setDependenciesByTool((current) => ({
        ...current,
        [toolId]: current[toolId] ?? emptyToolDependencies(toolId)
      }));
    }
  }

  async function saveDraft(nextDraft: ToolDraft = draft) {
    try {
      const tool = toolFromDraft(nextDraft, defaultTenantId);
      const saved = await toolsClient.saveTool(tool);
      const savedWithMetadata = preserveMetadata(saved, tool);
      upsertTool(savedWithMetadata);
      setSelectedToolId(savedWithMetadata.id);
      setDraft(draftFromTool(savedWithMetadata));
      setNotice({ ok: true, message: "Tool saved." });
      return savedWithMetadata;
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to save tool." });
      return null;
    }
  }

  async function saveTools(toolSpecs: ToolSpec[]) {
    try {
      const specsWithStableIDs = attachExistingToolIDs(toolSpecs, toolItems);
      const savedTools = await Promise.all(specsWithStableIDs.map((tool) => toolsClient.saveTool(tool)));
      setToolItems((current) => dedupeToolsByStableKey(upsertMany(current, savedTools)));
      if (savedTools[0]) {
        setSelectedToolId(savedTools[0].id);
        setDraft(draftFromTool(savedTools[0]));
      }
      setNotice({ ok: true, message: `${savedTools.length} tools saved.` });
      return true;
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to save tools." });
      return false;
    }
  }

  async function importConnectorTools(connectorId: string, toolSpecs: ToolSpec[]) {
    try {
      const specsWithStableIDs = attachExistingToolIDs(toolSpecs, toolItems);
      const savedTools = await toolsClient.importConnectorTools(connectorId, specsWithStableIDs);
      setToolItems((current) => dedupeToolsByStableKey(upsertMany(current, savedTools)));
      if (savedTools[0]) {
        setSelectedToolId(savedTools[0].id);
        setDraft(draftFromTool(savedTools[0]));
      }
      setNotice({ ok: true, message: `${savedTools.length} connector tools imported.` });
      return true;
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to import connector tools." });
      return false;
    }
  }

  async function enableSelected() {
    if (!selectedTool) return;
    await changeStatus(selectedTool.id, "enabled");
  }

  async function disableSelected() {
    if (!selectedTool) return;
    await changeStatus(selectedTool.id, "disabled");
  }

  async function setToolStatus(toolId: string, status: ToolSpec["status"]) {
    await changeStatus(toolId, status);
  }

  async function changeStatus(toolId: string, status: ToolSpec["status"]) {
    try {
      const saved = status === "enabled" ? await toolsClient.enableTool(toolId) : await toolsClient.disableTool(toolId);
      const current = toolItems.find((tool) => tool.id === toolId);
      const savedWithMetadata = current ? preserveMetadata(saved, current) : saved;
      upsertTool(savedWithMetadata);
      setDraft(draftFromTool(savedWithMetadata));
      setNotice({ ok: true, message: `Tool ${status}.` });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : `Failed to mark tool ${status}.` });
    }
  }

  async function executeSelected() {
    if (!selectedTool) return;
    try {
      const args = JSON.parse(testArgsJson || "{}") as Record<string, unknown>;
      const result = await toolsClient.executeTool(selectedTool.id, args, testConfirmed);
      setTestResult(result);
      setNotice({ ok: result.success, message: result.success ? "Tool execution succeeded." : "Tool execution failed." });
    } catch (error) {
      setTestResult(null);
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to execute tool." });
    }
  }

  function startNewTool(implementation: ToolSpec["implementation"] = "connector") {
    const nextDraft = createToolDraftForImplementation(implementation);
    setSelectedToolId("");
    setDraft(nextDraft);
    setTestArgsJson(JSON.stringify(sampleArgsFromFields(nextDraft.inputFields), null, 2));
    setTestResult(null);
  }

  function selectTool(toolId: string) {
    setSelectedToolId(toolId);
    setTestResult(null);
  }

  function upsertTool(tool: ToolSpec) {
    setToolItems((current) => {
      const exists = current.some((item) => item.id === tool.id);
      if (exists) {
        return current.map((item) => (item.id === tool.id ? preserveMetadata(tool, item) : item));
      }
      return [tool, ...current];
    });
  }

  function upsertWorkflow(workflow: WorkflowSpec) {
    setWorkflowItems((current) => {
      const exists = current.some((item) => item.id === workflow.id);
      if (exists) {
        return current.map((item) => (item.id === workflow.id ? workflow : item));
      }
      return [workflow, ...current];
    });
  }

  return {
    tools: toolItems,
    connectorSystems,
    connectors: connectorItems,
    workflows: workflowItems,
    selectedTool,
    selectedDependencies,
    dependenciesByTool,
    draft,
    setDraft,
    testArgsJson,
    setTestArgsJson,
    testConfirmed,
    setTestConfirmed,
    testResult,
    notice,
    selectTool,
    startNewTool,
    saveDraft,
    saveTools,
    importConnectorTools,
    enableSelected,
    disableSelected,
    setToolStatus,
    executeSelected,
    upsertWorkflow
  };
}

function preserveMetadata(tool: ToolSpec, fallback: ToolSpec): ToolSpec {
  return {
    ...tool,
    businessDomain: tool.businessDomain ?? fallback.businessDomain,
    ownerTeam: tool.ownerTeam ?? fallback.ownerTeam,
    llmDescription: tool.llmDescription ?? fallback.llmDescription
  };
}

function upsertMany(current: ToolSpec[], toolsToSave: ToolSpec[]): ToolSpec[] {
  let next = [...current];
  for (const tool of toolsToSave) {
    const stableKey = toolStableIdentity(tool);
    if (stableKey) {
      next = next.filter((item) => item.id === tool.id || toolStableIdentity(item) !== stableKey);
    }
    const index = next.findIndex((item) => item.id === tool.id);
    if (index >= 0) {
      next[index] = preserveMetadata(tool, next[index]);
      continue;
    }
    next.unshift(tool);
  }
  return next;
}

function attachExistingToolIDs(toolSpecs: ToolSpec[], current: ToolSpec[]): ToolSpec[] {
  const currentByStableKey = new Map<string, ToolSpec>();
  for (const tool of current) {
    const key = toolStableIdentity(tool);
    if (key && !currentByStableKey.has(key)) {
      currentByStableKey.set(key, tool);
    }
  }

  return toolSpecs.map((tool) => {
    const key = toolStableIdentity(tool);
    if (!key || tool.id) return tool;
    const existing = currentByStableKey.get(key);
    return existing ? { ...tool, id: existing.id } : tool;
  });
}

function dedupeToolsByStableKey(items: ToolSpec[]): ToolSpec[] {
  const seenKeys = new Set<string>();
  const result: ToolSpec[] = [];
  for (const tool of items) {
    const key = toolStableIdentity(tool);
    if (key) {
      if (seenKeys.has(key)) continue;
      seenKeys.add(key);
    }
    result.push(tool);
  }
  return result;
}

function toolStableIdentity(tool: ToolSpec): string {
  if (tool.implementation === "connector") {
    const operationID = tool.binding?.connectorOperationId?.trim();
    return operationID ? `${tool.tenantId}::connector::${operationID}` : "";
  }
  return mcpToolIdentity(tool);
}

function mcpToolIdentity(tool: ToolSpec): string {
  if (tool.implementation !== "mcp") return "";
  const serverId = tool.binding?.mcpServerId?.trim();
  const toolName = tool.binding?.mcpToolName?.trim() || tool.name.trim();
  if (!serverId || !toolName) return "";
  return `${tool.tenantId}::${serverId}::${toolName}`;
}
