import { useEffect, useMemo, useState } from "react";
import { resourceApi, runtimeApiV2 } from "../../platform/configApi";
import { useConfigWorkspace } from "../../platform/ConfigWorkspaceProvider";
import type { ConnectorConfig, SkillConfig, ToolConfig, WorkflowConfig } from "../../platform/configTypes";
import type { Connector, ConnectorOperation, ToolDependencies, ToolExecutionResult, ToolSpec, WorkflowSpec } from "../../types/platform";
import { toolSpecFromConfig } from "../agents/configModel";
import { useAutoDismissNotice, type NoticeState } from "../common/useAutoDismissNotice";
import {
  connectorFromConfig,
  connectorOperationFromConfig,
  dependenciesForTool,
  toolConfigFromDraft,
  toolExecutionResultFromRuntime,
  workflowSpecFromConfig
} from "./configModel";
import {
  createToolDraft,
  createToolDraftForImplementation,
  draftFromTool,
  emptyToolDependencies,
  sampleArgsFromFields,
  type ToolDraft
} from "./domain";

type Notice = NoticeState;

export function useTools() {
  const workspace = useConfigWorkspace();
  const [toolItems, setToolItems] = useState<ToolSpec[]>([]);
  const [connectorSystems, setConnectorSystems] = useState<Connector[]>([]);
  const [connectorItems, setConnectorItems] = useState<ConnectorOperation[]>([]);
  const [workflowItems, setWorkflowItems] = useState<WorkflowSpec[]>([]);
  const [toolConfigs, setToolConfigs] = useState<ToolConfig[]>([]);
  const [skillConfigs, setSkillConfigs] = useState<SkillConfig[]>([]);
  const [selectedToolId, setSelectedToolId] = useState("");
  const [draft, setDraft] = useState<ToolDraft>(() => createToolDraft());
  const [testArgsJson, setTestArgsJson] = useState('{\n  "order_id": "o_123"\n}');
  const [testConfirmed, setTestConfirmed] = useState(false);
  const [testResult, setTestResult] = useState<ToolExecutionResult | null>(null);
  const [notice, setNotice] = useAutoDismissNotice<Notice>();

  const selectedTool = useMemo(
    () => toolItems.find((tool) => tool.id === selectedToolId),
    [toolItems, selectedToolId]
  );

  const selectedDependencies = useMemo(
    () => dependenciesForTool(selectedTool, skillConfigs),
    [selectedTool, skillConfigs]
  );

  const dependenciesByTool = useMemo<Record<string, ToolDependencies>>(
    () => Object.fromEntries(toolItems.map((tool) => [tool.id, dependenciesForTool(tool, skillConfigs)])),
    [skillConfigs, toolItems]
  );

  useEffect(() => {
    void refresh();
  }, [workspace.activeBundleId]);

  useEffect(() => {
    if (!selectedTool) {
      if (selectedToolId === "") return;
      const nextDraft = createToolDraft();
      setDraft(nextDraft);
      setTestArgsJson(JSON.stringify(sampleArgsFromFields(nextDraft.inputFields), null, 2));
      return;
    }
    const nextDraft = draftFromTool(selectedTool);
    setDraft(nextDraft);
    setTestArgsJson(JSON.stringify(sampleArgsFromFields(nextDraft.inputFields), null, 2));
  }, [selectedTool?.id, selectedToolId]);

  async function refresh() {
    if (!workspace.activeBundleId) {
      setToolItems([]);
      setConnectorSystems([]);
      setConnectorItems([]);
      setWorkflowItems([]);
      setToolConfigs([]);
      setSkillConfigs([]);
      setSelectedToolId("");
      return;
    }
    try {
      const [tools, connectors, workflows, skills] = await Promise.all([
        resourceApi.listResourcesByKind<ToolConfig>(workspace.activeBundleId, "tool"),
        resourceApi.listResourcesByKind<ConnectorConfig>(workspace.activeBundleId, "connector"),
        resourceApi.listResourcesByKind<WorkflowConfig>(workspace.activeBundleId, "workflow"),
        resourceApi.listResourcesByKind<SkillConfig>(workspace.activeBundleId, "skill")
      ]);
      const nextToolConfigs = tools.items.map((item) => item.resource);
      const nextTools = dedupeToolsByStableKey(nextToolConfigs.map((item) => toolSpecFromConfig(item)));
      const nextConnectors = connectors.items.map((item) => item.resource);
      setToolConfigs(nextToolConfigs);
      setToolItems(nextTools);
      setConnectorSystems(nextConnectors.map(connectorFromConfig));
      setConnectorItems(nextConnectors.flatMap((connector) => (connector.operations ?? []).map((operation) => connectorOperationFromConfig(connector, operation))));
      setWorkflowItems(workflows.items.map((item) => workflowSpecFromConfig(item.resource)));
      setSkillConfigs(skills.items.map((item) => item.resource));
      setSelectedToolId((current) => (nextTools.some((tool) => tool.id === current) ? current : nextTools[0]?.id ?? ""));
      if (nextTools.length === 0) {
        setDraft(createToolDraft());
      }
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to load Tools from active Bundle." });
    }
  }

  async function saveDraft(nextDraft: ToolDraft = draft) {
    if (!workspace.activeBundleId) {
      setNotice({ ok: false, message: "Create or select a draft Bundle before saving Tools." });
      return null;
    }
    try {
      const current = toolConfigs.find((item) => item.id === nextDraft.id);
      const saved = await saveToolConfig(toolConfigFromDraft(nextDraft, current));
      setNotice({ ok: true, message: "Tool saved to draft Bundle." });
      return saved;
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to save Tool." });
      return null;
    }
  }

  async function saveTools(toolSpecs: ToolSpec[]) {
    try {
      const specsWithStableIDs = attachExistingToolIDs(toolSpecs, toolItems);
      await Promise.all(
        specsWithStableIDs.map((tool) => {
          const toolDraft = draftFromTool(tool);
          const current = toolConfigs.find((item) => item.id === toolDraft.id);
          return saveToolConfig(toolConfigFromDraft(toolDraft, current));
        })
      );
      setNotice({ ok: true, message: `${toolSpecs.length} tools saved to draft Bundle.` });
      return true;
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to save tools." });
      return false;
    }
  }

  async function importConnectorTools(_connectorId: string, toolSpecs: ToolSpec[]) {
    return saveTools(toolSpecs);
  }

  async function saveToolConfig(config: ToolConfig): Promise<ToolSpec> {
    if (!workspace.activeBundleId) throw new Error("No active draft Bundle selected.");
    const response = await resourceApi.upsertResource(workspace.activeBundleId, "tool", config);
    const saved = response.bundle.resources?.tools?.find((tool) => tool.id === config.id) ?? config;
    const viewModel = toolSpecFromConfig(saved);
    upsertToolConfig(saved);
    upsertTool(viewModel);
    setSelectedToolId(viewModel.id);
    setDraft(draftFromTool(viewModel));
    await workspace.refresh();
    return viewModel;
  }

  async function enableSelected() {
    if (!selectedTool) return;
    await changeDisabled(selectedTool.id, false);
  }

  async function disableSelected() {
    if (!selectedTool) return;
    await changeDisabled(selectedTool.id, true);
  }

  async function setToolStatus(toolId: string, status: ToolSpec["status"]) {
    await changeDisabled(toolId, status === "disabled");
  }

  async function changeDisabled(toolId: string, disabled: boolean) {
    if (!workspace.activeBundleId) return;
    try {
      const resource = await resourceApi.getResource<ToolConfig>(workspace.activeBundleId, "tool", toolId);
      const next = { ...resource.resource, disabled };
      await resourceApi.upsertResource(workspace.activeBundleId, "tool", next);
      const saved = toolSpecFromConfig(next);
      upsertToolConfig(next);
      upsertTool(saved);
      setDraft(draftFromTool(saved));
      await workspace.refresh();
      setNotice({ ok: true, message: `Tool ${disabled ? "disabled" : "enabled"}.` });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to update Tool status." });
    }
  }

  async function executeSelected() {
    if (!selectedTool) return;
    try {
      const input = JSON.parse(testArgsJson || "{}") as Record<string, unknown>;
      const result = await runtimeApiV2.invokeTool({
        tool_id: selectedTool.id,
        input,
        metadata: {
          confirmed: testConfirmed
        },
        trace_id: `tool_test_${Date.now().toString(16)}`
      });
      const execution = toolExecutionResultFromRuntime(selectedTool.id, result);
      setTestResult(execution);
      setNotice({ ok: execution.success, message: execution.success ? "Tool execution succeeded." : "Tool execution failed." });
    } catch (error) {
      setTestResult(null);
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to execute Tool." });
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
        return current.map((item) => (item.id === tool.id ? tool : item));
      }
      return [tool, ...current];
    });
  }

  function upsertToolConfig(config: ToolConfig) {
    setToolConfigs((current) => {
      const exists = current.some((item) => item.id === config.id);
      if (exists) {
        return current.map((item) => (item.id === config.id ? config : item));
      }
      return [config, ...current];
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
    activeBundleId: workspace.activeBundleId,
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
    return operationID ? `connector::${operationID}` : "";
  }
  if (tool.implementation === "workflow") {
    const workflowID = tool.binding?.workflowId?.trim();
    return workflowID ? `workflow::${workflowID}` : "";
  }
  return mcpToolIdentity(tool);
}

function mcpToolIdentity(tool: ToolSpec): string {
  if (tool.implementation !== "mcp") return "";
  const serverId = tool.binding?.mcpServerId?.trim();
  const toolName = tool.binding?.mcpToolName?.trim() || tool.name.trim();
  if (!serverId || !toolName) return "";
  return `${serverId}::${toolName}`;
}
