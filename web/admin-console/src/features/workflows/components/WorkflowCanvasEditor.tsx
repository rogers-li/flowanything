import { useEffect, useId, useMemo, useState, type ReactNode } from "react";
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
  type Edge,
  type EdgeChange,
  type Node,
  type NodeChange,
  type NodeProps
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { ChatBubble, TraceInspector, type DebugChatMessage } from "../../../components/AgentDebugChat";
import { Badge } from "../../../components/Badge";
import { FlowDeletableEdge, type FlowDeletableEdgeData } from "../../../components/FlowDeletableEdge";
import { useConfigWorkspace } from "../../../platform/ConfigWorkspaceProvider";
import { debugSessionApi, runHistoryApi, runtimeApiV2 } from "../../../platform/configApi";
import type { RunRecord, WorkflowRunResponse as RuntimeWorkflowRunResponse } from "../../../platform/configTypes";
import { agentTraceFromTraceResponse } from "../../../platform/traceViewModel";
import type {
  AgentTrace,
  AgentProfile,
  ConnectorOperation,
  SkillSpec,
  ToolSpec,
  TraceStep,
  WorkflowEdge,
  WorkflowNode,
  WorkflowRun,
  WorkflowNodeRun,
  WorkflowNodeType,
  WorkflowRunResponse,
  WorkflowSpec
} from "../../../types/platform";
import {
  applyNodeConfigRows,
  contextFieldsFromSchema,
  contextSchemaFromFields,
  createContextField,
  createMappingRow,
  createWorkflowNode,
  mappingRowsFromRecord,
  nodeConfigRows,
  nodeTypeLabel,
  recordFromMappingRows,
  type ContextFieldDraft,
  type MappingRowDraft,
  type NodeConfigRows
} from "../domain";
import {
  transformFunctionById,
  transformFunctionDefinitions
} from "../transformFunctions";

type WorkflowCanvasNodeData = {
  label: string;
  description?: string;
  nodeType: WorkflowNodeType;
  config?: Record<string, unknown>;
  timeoutMillis?: number;
  deletable?: boolean;
  onDelete?: (nodeId: string) => void;
};

type WorkflowCanvasNode = Node<WorkflowCanvasNodeData, "workflowCanvasNode">;
type WorkflowCanvasEdge = Edge<FlowDeletableEdgeData & { edgeType?: WorkflowEdge["type"] }>;
type MappingOption = { value: string; label: string; detail?: string };
type SchemaFieldOption = MappingOption & { type?: string; required?: boolean };
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

const nodeTypes = {
  workflowCanvasNode: WorkflowCanvasNodeView
};

const edgeTypes = {
  deletableEdge: FlowDeletableEdge
};

const workflowPalette: Array<{ type: WorkflowNodeType; label: string }> = [
  { type: "connector_operation", label: "Connector" },
  { type: "tool", label: "Tool" },
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

export type WorkflowCanvasEditorProps = {
  agents?: AgentProfile[];
  connectorOperations: ConnectorOperation[];
  skills?: SkillSpec[];
  tools: ToolSpec[];
  workflow: WorkflowSpec;
  notice?: { ok: boolean; message: string } | null;
  onBack: () => void;
  onSave: (workflow: WorkflowSpec) => Promise<void> | void;
};

export function WorkflowCanvasEditor({
  agents = [],
  connectorOperations,
  notice,
  onBack,
  onSave,
  skills = [],
  tools,
  workflow
}: WorkflowCanvasEditorProps) {
  const workspace = useConfigWorkspace();
  const [workflowName, setWorkflowName] = useState(workflow.name || "Untitled Workflow");
  const [nodes, setNodes] = useState<WorkflowCanvasNode[]>(() => workflowToCanvasNodes(workflow));
  const [edges, setEdges] = useState<WorkflowCanvasEdge[]>(() => workflowToCanvasEdges(workflow));
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(workflow.graph.entryNodeId || "start");
  const [ctxSettingsOpen, setCtxSettingsOpen] = useState(false);
  const [flowInputFields, setFlowInputFields] = useState<ContextFieldDraft[]>(() => contextFieldsFromSchema(workflow.inputSchema));
  const [flowOutputFields, setFlowOutputFields] = useState<ContextFieldDraft[]>(() => contextFieldsFromSchema(workflow.outputSchema));
  const [variableFields, setVariableFields] = useState<ContextFieldDraft[]>(() => contextFieldsFromSchema(workflow.contextSchema));
  const [workflowTestOpen, setWorkflowTestOpen] = useState(false);
  const [workflowTestInput, setWorkflowTestInput] = useState(() => JSON.stringify(defaultWorkflowInput(contextFieldsFromSchema(workflow.inputSchema)), null, 2));
  const [workflowTestMessages, setWorkflowTestMessages] = useState<DebugChatMessage[]>([]);
  const [workflowRunHistory, setWorkflowRunHistory] = useState<WorkflowRun[]>([]);
  const [workflowHistoryOpen, setWorkflowHistoryOpen] = useState(false);
  const [activeWorkflowTrace, setActiveWorkflowTrace] = useState<AgentTrace | null>(null);
  const [workflowRunning, setWorkflowRunning] = useState(false);
  const [nodeConfigExpanded, setNodeConfigExpanded] = useState(false);
  const [saving, setSaving] = useState(false);
  const [validationIssues, setValidationIssues] = useState<string[]>([]);
  const [validationOpen, setValidationOpen] = useState(false);
  const selectedNode = useMemo(() => nodes.find((node) => node.id === selectedNodeId), [nodes, selectedNodeId]);
  const selectedWorkflowNode = selectedNode ? canvasNodeToWorkflowNode(selectedNode, workflow.graph.nodes[selectedNode.id]) : undefined;
  const selectedNodeRows = nodeConfigRows(selectedWorkflowNode);
  const selectedOutputEnabledSources = outputEnabledSourcesFromConfig(selectedWorkflowNode?.config);
  const selectedResourceSchemas = selectedWorkflowNode ? bindingSchemasForNode(selectedWorkflowNode, connectorOperations, tools) : emptyBindingSchemas;
  const conditionTargetOptions = useMemo(
    () =>
      nodes
        .filter((node) => node.id !== selectedNodeId && node.data.nodeType !== "start")
        .map((node) => ({ value: node.id, label: node.data.label || node.id, detail: node.data.nodeType })),
    [nodes, selectedNodeId]
  );
  const contextReadOptions = useMemo(
    () => contextReadOptionsFor(flowInputFields, flowOutputFields, variableFields, nodes),
    [flowInputFields, flowOutputFields, nodes, variableFields]
  );
  const contextWriteOptions = useMemo(
    () => contextWriteOptionsFor(flowOutputFields, variableFields),
    [flowOutputFields, variableFields]
  );

  useEffect(() => {
    setWorkflowName(workflow.name || "Untitled Workflow");
    setNodes(workflowToCanvasNodes(workflow));
    setEdges(workflowToCanvasEdges(workflow));
    setSelectedNodeId(workflow.graph.entryNodeId || "start");
    setFlowInputFields(contextFieldsFromSchema(workflow.inputSchema));
    setFlowOutputFields(contextFieldsFromSchema(workflow.outputSchema));
    setVariableFields(contextFieldsFromSchema(workflow.contextSchema));
    setWorkflowTestInput(JSON.stringify(defaultWorkflowInput(contextFieldsFromSchema(workflow.inputSchema)), null, 2));
    setWorkflowTestMessages([]);
    setWorkflowRunHistory([]);
    setActiveWorkflowTrace(null);
    setValidationIssues([]);
    setValidationOpen(false);
  }, [workflow.id, workflow.name, workflow.version]);

  const updateSelectedNodeData = (patch: Partial<WorkflowCanvasNodeData>) => {
    if (!selectedNodeId) return;
    setValidationIssues([]);
    setValidationOpen(false);
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
  };

  const updateNodeConfig = (key: string, value: unknown) => {
    updateSelectedNodeConfigPatch({ [key]: value });
  };

  const updateSelectedNodeConfigPatch = (patch: Record<string, unknown>) => {
    if (!selectedNodeId) return;
    setValidationIssues([]);
    setValidationOpen(false);
    const selectedBeforePatch = nodes.find((node) => node.id === selectedNodeId);
    const nextConfig = {
      ...(selectedBeforePatch?.data.config ?? {}),
      ...patch
    };
    setNodes((current) =>
      current.map((node) =>
        node.id === selectedNodeId
          ? {
              ...node,
              data: {
                ...node.data,
                config: {
                  ...(node.data.config ?? {}),
                  ...patch
                }
              }
            }
          : node
      )
    );
    if (selectedBeforePatch?.data.nodeType === "condition") {
      setEdges((current) => syncConditionOutgoingEdges(current, selectedNodeId, nextConfig));
    }
  };

  const updateNodeRows = (kind: keyof NodeConfigRows, rows: MappingRowDraft[]) => {
    if (!selectedNodeId) return;
    setValidationIssues([]);
    setValidationOpen(false);
    setNodes((current) =>
      current.map((node) => {
        if (node.id !== selectedNodeId) return node;
        const workflowNode = canvasNodeToWorkflowNode(node, workflow.graph.nodes[node.id]);
        const nextNode = applyNodeConfigRows(workflowNode, { ...nodeConfigRows(workflowNode), [kind]: rows });
        return {
          ...node,
          data: {
            ...node.data,
            config: nextNode.config
          }
        };
      })
    );
  };

  const addWorkflowNode = (nodeType: WorkflowNodeType) => {
    setValidationIssues([]);
    setValidationOpen(false);
    const node = createWorkflowNode(nodeType, nodes.length);
    const canvasNode = workflowNodeToCanvasNode(node, nodes.length);
    setNodes((current) => [...current, canvasNode]);
    setSelectedNodeId(node.id);
  };

  const deleteWorkflowNode = (nodeId: string) => {
    if (nodeId === "start") return;
    setValidationIssues([]);
    setValidationOpen(false);
    setNodes((current) =>
      current
        .filter((node) => node.id !== nodeId)
        .map((node) =>
          node.data.nodeType === "condition"
            ? {
                ...node,
                data: {
                  ...node.data,
                  config: clearAllConditionTargetsFromConfig(node.data.config ?? {}, nodeId)
                }
              }
            : node
        )
    );
    setEdges((current) => current.filter((edge) => edge.source !== nodeId && edge.target !== nodeId));
    if (selectedNodeId === nodeId) {
      setSelectedNodeId(null);
      setNodeConfigExpanded(false);
    }
  };

  const deleteWorkflowEdge = (edgeId: string) => {
    setValidationIssues([]);
    setValidationOpen(false);
    setEdges((current) => {
      const removed = current.find((edge) => edge.id === edgeId);
      if (removed) {
        setNodes((nodeList) =>
          nodeList.map((node) =>
            node.id === removed.source && node.data.nodeType === "condition"
              ? {
                  ...node,
                  data: {
                    ...node.data,
                    config: clearConditionTargetFromConfig(node.data.config ?? {}, removed.target, removed.data?.edgeType)
                  }
                }
              : node
          )
        );
      }
      return current.filter((edge) => edge.id !== edgeId);
    });
  };

  const saveWorkflow = async () => {
    const workflowSpec = currentWorkflowSpec();
    const issues = validateWorkflowCanvas(workflowSpec);
    setValidationIssues(issues);
    setValidationOpen(issues.length > 0);
    if (issues.length > 0) return;
    setSaving(true);
    try {
      await onSave(workflowSpec);
    } finally {
      setSaving(false);
    }
  };

  const currentWorkflowSpec = () => canvasToWorkflow({ ...workflow, name: workflowName }, nodes, edges, flowInputFields, flowOutputFields, variableFields);

  useEffect(() => {
    if (!workflowTestOpen) return;
    void loadWorkflowRunHistory();
  }, [workflowTestOpen, workflow.id]);

  const loadWorkflowRunHistory = async () => {
    try {
      const resp = await runHistoryApi.list();
      setWorkflowRunHistory(
        resp.items
          .filter((run) => run.type === "workflow" && runWorkflowId(run) === workflow.id)
          .slice(0, 12)
          .map((run) => workflowRunFromRecord(run, workflow.id))
      );
    } catch {
      setWorkflowRunHistory([]);
    }
  };

  const appendWorkflowResult = (result: WorkflowRunResponse, trace?: AgentTrace) => {
    const nextTrace = trace ?? buildWorkflowTraceFromRun(result);
    const assistantMessage: DebugChatMessage = {
      id: `workflow_test_assistant_${result.run.id}`,
      role: "assistant",
      text: workflowRunText(result),
      traceId: nextTrace.traceId,
      trace: nextTrace
    };
    setWorkflowTestMessages((current) => [...current, assistantMessage]);
    setActiveWorkflowTrace(nextTrace);
  };

  const loadTraceForRun = async (result: WorkflowRunResponse): Promise<AgentTrace> => {
    if (!result.run.traceId) {
      return buildWorkflowTraceFromRun(result);
    }
    try {
      return agentTraceFromTraceResponse(await runtimeApiV2.getTrace(result.run.traceId));
    } catch {
      return buildWorkflowTraceFromRun(result);
    }
  };

  const openWorkflowHistoryRun = async (runID: string) => {
    try {
      const { run } = await runHistoryApi.get(runID);
      const result = workflowRunResponseFromRecord(run, workflow.id);
      const historyMessages: DebugChatMessage[] = [];
      if (result.run.input) {
        historyMessages.push({
          id: `workflow_history_user_${runID}`,
          role: "user",
          text: JSON.stringify(result.run.input, null, 2)
        });
      }
      const trace = await loadTraceForRun(result);
      historyMessages.push({
        id: `workflow_history_assistant_${runID}`,
        role: "assistant",
        text: workflowRunText(result),
        traceId: trace.traceId,
        trace
      });
      setWorkflowTestMessages(historyMessages);
      setActiveWorkflowTrace(trace);
      setWorkflowHistoryOpen(false);
    } catch (error) {
      setWorkflowTestMessages((current) => [
        ...current,
        {
          id: `workflow_history_failed_${Date.now()}`,
          role: "assistant",
          text: error instanceof Error ? error.message : "Failed to load workflow run."
        }
      ]);
    }
  };

  const replayWorkflowHistoryRun = async (runID: string) => {
    if (workflowRunning) return;
    setWorkflowRunning(true);
    setActiveWorkflowTrace(null);
    try {
      const { run } = await runHistoryApi.replay(runID);
      const result = workflowRunResponseFromRecord(run, workflow.id);
      appendWorkflowResult(result, await loadTraceForRun(result));
      setWorkflowHistoryOpen(false);
      await loadWorkflowRunHistory();
    } catch (error) {
      setWorkflowTestMessages((current) => [
        ...current,
        {
          id: `workflow_replay_failed_${Date.now()}`,
          role: "assistant",
          text: error instanceof Error ? error.message : "Workflow replay failed."
        }
      ]);
    } finally {
      setWorkflowRunning(false);
    }
  };

  const runWorkflowTest = async () => {
    if (workflowRunning) return;
    let input: Record<string, unknown>;
    try {
      input = parseWorkflowInput(workflowTestInput);
    } catch (error) {
      setWorkflowTestMessages((current) => [
        ...current,
        {
          id: `workflow_test_error_${Date.now()}`,
          role: "assistant",
          text: error instanceof Error ? error.message : "Flow Input must be valid JSON."
        }
      ]);
      return;
    }

    const userMessage: DebugChatMessage = {
      id: `workflow_test_user_${Date.now()}`,
      role: "user",
      text: JSON.stringify(input, null, 2)
    };
    setWorkflowRunning(true);
    setActiveWorkflowTrace(null);
    setWorkflowTestMessages((current) => [...current, userMessage]);
    let workflowSpec: WorkflowSpec | undefined;
    let traceId = "";
    try {
      workflowSpec = currentWorkflowSpec();
      const issues = validateWorkflowCanvas(workflowSpec);
      setValidationIssues(issues);
      setValidationOpen(issues.length > 0);
      if (issues.length > 0) {
        setWorkflowTestMessages((current) => [
          ...current,
          {
            id: `workflow_validation_failed_${Date.now()}`,
            role: "assistant",
            text: `Workflow validation failed:\n${issues.map((issue) => `- ${issue}`).join("\n")}`
          }
        ]);
        return;
      }
      await onSave(workflowSpec);
      setWorkflowHistoryOpen(false);
      if (!workspace.activeBundleId) {
        throw new Error("No active config bundle selected.");
      }
      traceId = `workflow_test_trace_${Date.now().toString(16)}`;
      const { session } = await debugSessionApi.createSession({
        bundle_id: workspace.activeBundleId,
        entrypoint: {
          kind: "workflow",
          id: workflowSpec.id
        }
      });
      const runtimeResult = await debugSessionApi.runWorkflow(session.id, {
        workflow_id: workflowSpec.id,
        input,
        trace_context: {
          trace_id: traceId
        }
      });
      const result = workflowRunResponseFromRuntime(workflowSpec.id, input, runtimeResult, traceId);
      appendWorkflowResult(result, await loadTraceForRun(result));
      await loadWorkflowRunHistory();
    } catch (error) {
      const trace = await workflowFailureTrace(traceId, workflowSpec, input, error);
      setWorkflowTestMessages((current) => [
        ...current,
        {
          id: `workflow_test_failed_${Date.now()}`,
          role: "assistant",
          text: `${error instanceof Error ? error.message : "Workflow test failed."}\n\nOpen trace to inspect the failed run.`,
          traceId: trace?.traceId,
          trace
        }
      ]);
      if (trace) setActiveWorkflowTrace(trace);
    } finally {
      setWorkflowRunning(false);
    }
  };

  return (
    <div className="agent-flow-page agent-flow-editor-page workflow-canvas-editor-page">
      <header className="agent-flow-topbar">
        <div className="agent-flow-titlebar">
          <button className="flow-icon-button" type="button" onClick={onBack} aria-label="Back">
            <span aria-hidden="true">‹</span>
          </button>
          <div>
            <span className="eyebrow">Workflow Canvas</span>
            <input
              className="workflow-title-input"
              aria-label="Workflow name"
              value={workflowName}
              onChange={(event) => {
                setValidationIssues([]);
                setValidationOpen(false);
                setWorkflowName(event.target.value);
              }}
              onBlur={() => setWorkflowName((current) => current.trim() || "Untitled Workflow")}
            />
          </div>
        </div>

        <div className="agent-flow-actions">
          {notice ? <span className={notice.ok ? "inline-notice inline-notice-ok" : "inline-notice inline-notice-error"}>{notice.message}</span> : null}
          {validationIssues.length > 0 ? (
            <div className="workflow-validation-menu">
              <button className="inline-notice inline-notice-error" type="button" onClick={() => setValidationOpen((current) => !current)}>
                {validationIssues.length} validation issue{validationIssues.length > 1 ? "s" : ""}: {validationIssues[0]}
              </button>
              {validationOpen ? (
                <div className="workflow-validation-popover" role="dialog" aria-label="Workflow validation issues">
                  {validationIssues.map((issue) => (
                    <p key={issue}>{issue}</p>
                  ))}
                </div>
              ) : null}
            </div>
          ) : null}
          <Badge tone={workflow.status === "enabled" ? "green" : workflow.status === "disabled" ? "red" : "gray"}>{workflow.status}</Badge>
          <button
            className="secondary-button"
            type="button"
            onClick={() => {
              setWorkflowTestOpen((current) => !current);
              setCtxSettingsOpen(false);
            }}
          >
            Test Flow
          </button>
          <button
            className="secondary-button"
            type="button"
            onClick={() => {
              setCtxSettingsOpen((current) => !current);
              setWorkflowTestOpen(false);
            }}
          >
            Ctx Settings
          </button>
          <button className="primary-button" type="button" onClick={() => void saveWorkflow()} disabled={saving}>
            {saving ? "Saving..." : "Save"}
          </button>
        </div>
      </header>

      <section className="agent-flow-workbench">
        {workflowTestOpen ? (
          <aside className="agent-flow-test-drawer workflow-test-drawer" aria-label="Test Workflow">
            <div className="node-inspector-header flow-test-drawer-header">
              <div>
                <span className="eyebrow">Test Flow</span>
                <p>Run the current canvas with a Flow Input payload.</p>
              </div>
              <div className="node-inspector-actions">
                <button
                  className="secondary-action compact-action"
                  type="button"
                  onClick={() => {
                    setWorkflowTestMessages([]);
                    setActiveWorkflowTrace(null);
                  }}
                >
                  New
                </button>
                <button className="flow-icon-button" type="button" onClick={() => setWorkflowTestOpen(false)} aria-label="Close workflow test">
                  ×
                </button>
              </div>
            </div>

            <section className="flow-config-panel workflow-test-input-panel">
              <label>
                Flow Input
                <textarea
                  value={workflowTestInput}
                  onChange={(event) => setWorkflowTestInput(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
                      event.preventDefault();
                      void runWorkflowTest();
                    }
                  }}
                  placeholder='{"query":"search AI news"}'
                />
              </label>
              <button className="primary-button" type="button" onClick={() => void runWorkflowTest()} disabled={workflowRunning}>
                {workflowRunning ? "Running..." : "Run Workflow"}
              </button>
            </section>

            <div className="workflow-run-history-menu">
              <button className="secondary-action" type="button" onClick={() => setWorkflowHistoryOpen((current) => !current)}>
                History ({workflowRunHistory.length})
              </button>
              {workflowHistoryOpen ? (
                <WorkflowRunHistoryPanel
                  runs={workflowRunHistory}
                  activeRunId={activeWorkflowTrace?.eventId}
                  running={workflowRunning}
                  onOpenRun={(runID) => void openWorkflowHistoryRun(runID)}
                  onReplayRun={(runID) => void replayWorkflowHistoryRun(runID)}
                />
              ) : null}
            </div>

            <section className="flow-config-panel flow-test-chat-panel">
              <WorkflowTestMessages messages={workflowTestMessages} activeTrace={activeWorkflowTrace} onOpenTrace={setActiveWorkflowTrace} />
            </section>
          </aside>
        ) : null}

        {ctxSettingsOpen ? (
          <ContextSettingsPanel
            flowInputFields={flowInputFields}
            flowOutputFields={flowOutputFields}
            nodes={nodes}
            onClose={() => setCtxSettingsOpen(false)}
            onFlowInputFieldsChange={(fields) => {
              setValidationIssues([]);
              setValidationOpen(false);
              setFlowInputFields(fields);
            }}
            onFlowOutputFieldsChange={(fields) => {
              setValidationIssues([]);
              setValidationOpen(false);
              setFlowOutputFields(fields);
            }}
            onVariableFieldsChange={(fields) => {
              setValidationIssues([]);
              setValidationOpen(false);
              setVariableFields(fields);
            }}
            variableFields={variableFields}
          />
        ) : null}

        {activeWorkflowTrace ? <TraceInspector trace={activeWorkflowTrace} onClose={() => setActiveWorkflowTrace(null)} /> : null}

        <main className="agent-flow-canvas-shell" aria-label="Workflow canvas">
          <div className="agent-flow-toolstrip" aria-label="Add workflow nodes">
            {workflowPalette.map((item) => (
              <button key={item.type} type="button" onClick={() => addWorkflowNode(item.type)}>
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
                onDelete: deleteWorkflowNode
              }
            }))}
            edges={edges.map((edge) => ({
              ...edge,
              type: "deletableEdge",
              data: {
                ...edge.data,
                onDelete: deleteWorkflowEdge
              }
            }))}
            nodeTypes={nodeTypes}
            edgeTypes={edgeTypes}
            deleteKeyCode={null}
            fitView
            fitViewOptions={{ maxZoom: 0.82, padding: 0.34 }}
            onNodesChange={(changes: NodeChange<WorkflowCanvasNode>[]) => {
              setValidationIssues([]);
              setValidationOpen(false);
              setNodes((current) => applyNodeChanges(changes, current));
            }}
            onEdgesChange={(changes: EdgeChange<WorkflowCanvasEdge>[]) => {
              setValidationIssues([]);
              setValidationOpen(false);
              setEdges((current) => applyEdgeChanges(changes, current));
            }}
            onConnect={(connection: Connection) => {
              if (!connection.source || !connection.target) return;
              if (connection.source === "start" && edges.some((edge) => edge.source === "start")) return;
              setValidationIssues([]);
              setValidationOpen(false);
              const sourceNode = nodes.find((node) => node.id === connection.source);
              if (sourceNode?.data.nodeType === "condition") {
                const nextConfig = addConditionTargetToConfig(sourceNode.data.config ?? {}, connection.target);
                setNodes((current) =>
                  current.map((node) =>
                    node.id === connection.source
                      ? {
                          ...node,
                          data: {
                            ...node.data,
                            config: nextConfig
                          }
                        }
                      : node
                  )
                );
                setEdges((current) => syncConditionOutgoingEdges(current, connection.source, nextConfig));
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
            onNodeClick={(_, node) => setSelectedNodeId(node.id)}
          >
            <MiniMap pannable zoomable />
            <Controls />
            <Background gap={18} size={1.2} />
          </ReactFlow>
        </main>

        <aside className={selectedNode ? "agent-flow-right agent-flow-right-open" : "agent-flow-right"}>
          <div className="node-inspector-header">
            <div>
              <span className="eyebrow">Node Config</span>
              <h2>{selectedNode?.data.label || "Select Node"}</h2>
            </div>
            <div className="node-inspector-actions">
              {selectedNode?.data.nodeType !== "start" ? (
                <button className="flow-panel-close" type="button" onClick={() => setNodeConfigExpanded(true)} aria-label="Expand node config">
                  ⛶
                </button>
              ) : null}
              <button className="flow-panel-close" type="button" onClick={() => setSelectedNodeId(null)} aria-label="Close node config">
                ×
              </button>
            </div>
          </div>
          {selectedNode?.data.nodeType === "start" ? (
            <StartNodeSchemaPanel
              flowInputFields={flowInputFields}
              flowOutputFields={flowOutputFields}
              onFlowInputFieldsChange={(fields) => {
                setValidationIssues([]);
                setValidationOpen(false);
                setFlowInputFields(fields);
              }}
              onFlowOutputFieldsChange={(fields) => {
                setValidationIssues([]);
                setValidationOpen(false);
                setFlowOutputFields(fields);
              }}
            />
          ) : selectedNode && selectedWorkflowNode ? (
            <div className="node-config-section-stack">
              <NodeConfigSection defaultOpen title="Basic" meta={nodeTypeLabel(selectedNode.data.nodeType)}>
                <label>
                  Name
                  <input value={selectedNode.data.label} onChange={(event) => updateSelectedNodeData({ label: event.target.value })} />
                </label>
                <label>
                  Description
                  <textarea value={selectedNode.data.description ?? ""} onChange={(event) => updateSelectedNodeData({ description: event.target.value })} />
                </label>
                {isConnectorOrToolNode(selectedNode.data.nodeType) ? (
                  <WorkflowNodeBindingEditor
                    agents={agents}
                    connectorOperations={connectorOperations}
                    contextReadOptions={contextReadOptions}
                    contextWriteOptions={contextWriteOptions}
                    node={selectedWorkflowNode}
                    nodeOptions={conditionTargetOptions}
                    skills={skills}
                    tools={tools}
                    onConfigChange={updateNodeConfig}
                  />
                ) : null}
              </NodeConfigSection>

              {selectedNode.data.nodeType === "condition" ? (
                <ConditionBranchEditor
                  contextReadOptions={contextReadOptions}
                  contextWriteOptions={contextWriteOptions}
                  node={selectedWorkflowNode}
                  nodeOptions={conditionTargetOptions}
                  onConfigChange={updateNodeConfig}
                />
              ) : !isConnectorOrToolNode(selectedNode.data.nodeType) ? (
                <NodeConfigSection defaultOpen title="Binding" meta={bindingMeta(selectedNode.data.nodeType)}>
                  <WorkflowNodeBindingEditor
                    agents={agents}
                    connectorOperations={connectorOperations}
                    contextReadOptions={contextReadOptions}
                    contextWriteOptions={contextWriteOptions}
                    node={selectedWorkflowNode}
                    nodeOptions={conditionTargetOptions}
                    skills={skills}
                    tools={tools}
                    onConfigChange={updateNodeConfig}
                  />
                </NodeConfigSection>
              ) : null}

              {selectedNode.data.nodeType !== "condition" ? (
                <NodeConfigSection title="Input Mapping" meta={`${selectedNodeRows.inputRows.length} fields`}>
                  <div className="workflow-quick-mapping-editor">
                    {selectedResourceSchemas.inputFields.length > 0 ? (
                      <MappingRows
                        fieldOptions={selectedResourceSchemas.inputFields}
                        fieldDriven
                        rows={selectedNodeRows.inputRows}
                        sourceOptions={contextReadOptions}
                        onChange={(rows) => updateNodeRows("inputRows", rows)}
                      />
                    ) : (
                      <MappingRows rows={selectedNodeRows.inputRows} sourceOptions={contextReadOptions} onChange={(rows) => updateNodeRows("inputRows", rows)} />
                    )}
                  </div>
                </NodeConfigSection>
              ) : null}

              {selectedNode.data.nodeType !== "condition" ? (
                <NodeConfigSection title="Output Mapping" meta={`${selectedNodeRows.contextRows.length} writes`}>
                  <div className="workflow-quick-mapping-editor">
                    {supportsFieldDrivenOutput(selectedNode.data.nodeType) && selectedResourceSchemas.outputFields.length > 0 ? (
                      <OutputSchemaMappingRows
                        rows={selectedNodeRows.contextRows}
                        enabledSources={selectedOutputEnabledSources}
                        targetOptions={contextWriteOptions}
                        outputFields={selectedResourceSchemas.outputFields}
                        onEnabledSourcesChange={(sources) => updateNodeConfig("output_mapping_enabled_sources", sources)}
                        onChange={(rows) => updateNodeRows("contextRows", rows)}
                      />
                    ) : (
                      <MappingRows
                        rows={selectedNodeRows.contextRows}
                        sourceOptions={responseReadOptionsFor(selectedWorkflowNode, selectedResourceSchemas.outputFields)}
                        targetOptions={contextWriteOptions}
                        onChange={(rows) => updateNodeRows("contextRows", rows)}
                      />
                    )}
                  </div>
                </NodeConfigSection>
              ) : null}
            </div>
          ) : (
            <p className="muted-copy">Select a node on the canvas.</p>
          )}
        </aside>

        {nodeConfigExpanded && selectedNode && selectedWorkflowNode && selectedNode.data.nodeType !== "start" ? (
          <WorkflowNodeConfigModal
            agents={agents}
            connectorOperations={connectorOperations}
            contextReadOptions={contextReadOptions}
            contextWriteOptions={contextWriteOptions}
            node={selectedWorkflowNode}
            nodeRows={selectedNodeRows}
            onClose={() => setNodeConfigExpanded(false)}
            onConfigChange={updateNodeConfig}
            onNodeDataChange={updateSelectedNodeData}
            onRowsChange={updateNodeRows}
            resourceSchemas={selectedResourceSchemas}
            selectedNode={selectedNode}
            conditionTargetOptions={conditionTargetOptions}
            outputEnabledSources={selectedOutputEnabledSources}
            onOutputEnabledSourcesChange={(sources) => updateNodeConfig("output_mapping_enabled_sources", sources)}
            skills={skills}
            tools={tools}
          />
        ) : null}
      </section>
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

function WorkflowTestMessages({
  activeTrace,
  messages,
  onOpenTrace
}: {
  activeTrace: AgentTrace | null;
  messages: DebugChatMessage[];
  onOpenTrace: (trace: AgentTrace) => void;
}) {
  return (
    <div className="agent-chat-window workflow-test-message-list">
      {messages.length === 0 ? (
        <div className="agent-chat-empty">
          <strong>No workflow run yet</strong>
          <p>Provide Flow Input and run the workflow. Each result can open a node-level trace.</p>
        </div>
      ) : (
        messages.map((message) => (
          <ChatBubble
            active={message.traceId !== undefined && message.traceId === activeTrace?.traceId}
            key={message.id}
            message={message}
            onOpenTrace={onOpenTrace}
          />
        ))
      )}
    </div>
  );
}

function WorkflowRunHistoryPanel({
  activeRunId,
  onOpenRun,
  onReplayRun,
  running,
  runs
}: {
  activeRunId?: string;
  onOpenRun: (runID: string) => void;
  onReplayRun: (runID: string) => void;
  running: boolean;
  runs: WorkflowRun[];
}) {
  return (
    <section className="flow-config-panel workflow-run-history-panel">
      <div className="workflow-run-history-heading">
        <div>
          <span className="eyebrow">Run History</span>
          <p>Open a previous run or replay it with the same input.</p>
        </div>
      </div>
      <div className="workflow-run-history-list">
        {runs.length === 0 ? (
          <p className="muted-copy">No workflow runs in this runtime yet.</p>
        ) : (
          runs.map((run) => (
            <article key={run.id} className={run.id === activeRunId ? "workflow-run-history-item workflow-run-history-item-active" : "workflow-run-history-item"}>
              <button type="button" onClick={() => onOpenRun(run.id)}>
                <strong>{run.status}</strong>
                <span>{formatWorkflowRunTime(run.startedAt)}</span>
              </button>
              <button className="secondary-action compact-action" type="button" disabled={running} onClick={() => onReplayRun(run.id)}>
                Replay
              </button>
            </article>
          ))
        )}
      </div>
    </section>
  );
}

function WorkflowNodeBindingEditor({
  agents,
  connectorOperations,
  contextReadOptions = [],
  contextWriteOptions = [],
  node,
  nodeOptions = [],
  onConfigChange,
  skills,
  tools
}: {
  agents: AgentProfile[];
  connectorOperations: ConnectorOperation[];
  contextReadOptions?: MappingOption[];
  contextWriteOptions?: MappingOption[];
  node: WorkflowNode;
  nodeOptions?: MappingOption[];
  onConfigChange: (key: string, value: unknown) => void;
  skills: SkillSpec[];
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
            <select
              value={value("connector_operation_id") || value("operation_id")}
              onChange={(event) => onConfigChange("connector_operation_id", event.target.value)}
            >
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
            <input value={value("response_alias")} onChange={(event) => onConfigChange("response_alias", event.target.value)} placeholder="search_step_1" />
          </label>
          <p className="workflow-muted-note">Runtime writes the raw response to responses.connector.&lt;alias&gt;. Use alias when the same operation appears more than once.</p>
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
            <input value={value("response_alias")} onChange={(event) => onConfigChange("response_alias", event.target.value)} placeholder="tool_call_1" />
          </label>
          <p className="workflow-muted-note">Runtime writes the raw response to responses.tool.&lt;alias&gt;. Use alias when the same tool appears more than once.</p>
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
          contextReadOptions={contextReadOptions}
          contextWriteOptions={contextWriteOptions}
          node={node}
          nodeOptions={nodeOptions}
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
  const removeBranch = (branchId: string) => {
    commitBranches(branches.filter((branch) => branch.id !== branchId));
  };
  const addBranch = () => {
    const branch = createConditionBranchDraft({ name: `Branch ${branches.length + 1}` });
    commitBranches([...branches, branch]);
    setActiveTab(branch.id);
  };
  const activeBranch = branches.find((branch) => branch.id === activeTab);
  return (
    <div className="workflow-binding-stack">
      <p className="workflow-muted-note">Evaluate branches in order. A blank Next Node means this branch ends the workflow after writing context.</p>
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
            <button type="button" onClick={() => removeBranch(activeBranch.id)} aria-label="Remove branch">
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
            <MappingRows
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
            <MappingRows
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
            <ExpressionInput options={contextReadOptions} value={rule.left} onChange={(value) => update(rule.id, { left: value })} />
            <select value={rule.operator} onChange={(event) => update(rule.id, { operator: event.target.value })}>
              {conditionOperatorOptions.map((operator) => (
                <option key={operator.value} value={operator.value}>
                  {operator.label}
                </option>
              ))}
            </select>
            <ExpressionInput options={contextReadOptions} value={rule.right} onChange={(value) => update(rule.id, { right: value })} />
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

function CompactMappingSummary({
  configuredCount,
  onExpand,
  schemaCount,
  text
}: {
  configuredCount: number;
  onExpand: () => void;
  schemaCount: number;
  text: string;
}) {
  return (
    <div className="workflow-compact-mapping-summary">
      <div>
        <strong>{configuredCount} configured</strong>
        <span>{schemaCount > 0 ? `${schemaCount} schema fields available` : "No schema fields detected"}</span>
      </div>
      <p>{text}</p>
      <button className="secondary-action" type="button" onClick={onExpand}>
        Open full editor
      </button>
    </div>
  );
}

function MappingRows({
  fieldDriven = false,
  fieldOptions = [],
  onChange,
  rows,
  sourceOptions = [],
  targetOptions = []
}: {
  fieldDriven?: boolean;
  fieldOptions?: SchemaFieldOption[];
  onChange: (rows: MappingRowDraft[]) => void;
  rows: MappingRowDraft[];
  sourceOptions?: MappingOption[];
  targetOptions?: MappingOption[];
}) {
  const [draftRows, setDraftRows] = useState(rows);
  const rowsSignature = rows.map((row) => `${row.id}:${row.target}:${row.source}`).join("|");
  useEffect(() => {
    setDraftRows(rows);
  }, [rowsSignature]);
  const displayRows = fieldDriven && fieldOptions.length > 0 ? rowsForSchemaFields(rows, fieldOptions) : draftRows;
  const commitRows = (nextRows: MappingRowDraft[]) => {
    onChange(nextRows.filter((row) => row.target.trim()));
  };
  const update = (rowId: string, patch: Partial<MappingRowDraft>) => {
    const currentRow = displayRows.find((row) => row.id === rowId);
    if (!currentRow) return;
    if (fieldDriven && fieldOptions.length > 0) {
      const nextRow = { ...currentRow, ...patch };
      const exists = rows.some((row) => row.target === currentRow.target);
      onChange(exists ? rows.map((row) => (row.target === currentRow.target ? nextRow : row)) : [...rows, nextRow]);
      return;
    }
    const nextRows = draftRows.map((row) => (row.id === rowId ? { ...row, ...patch } : row));
    setDraftRows(nextRows);
    const updatedRow = nextRows.find((row) => row.id === rowId);
    if (patch.target !== undefined && !updatedRow?.target.trim()) return;
    commitRows(nextRows);
  };
  const toggleSchemaRow = (row: MappingRowDraft, enabled: boolean) => {
    if (!fieldDriven || fieldOptions.length === 0) return;
    if (enabled) {
      if (rows.some((item) => item.target === row.target)) return;
      onChange([...rows, row]);
      return;
    }
    onChange(rows.filter((item) => item.target !== row.target));
  };
  const addRow = () => {
    const nextRows = [...displayRows, createMappingRow({ target: nextMappingTarget(displayRows, targetOptions) })];
    setDraftRows(nextRows);
    commitRows(nextRows);
  };
  const removeRow = (rowId: string) => {
    const nextRows = displayRows.filter((item) => item.id !== rowId);
    setDraftRows(nextRows);
    commitRows(nextRows);
  };
  return (
    <div className="workflow-mapping-block">
      <div className="workflow-mapping-table">
        <span>Target</span>
        <span>Expression</span>
        <span />
        {displayRows.map((row) => {
          const field = fieldOptions.find((item) => item.value === row.target);
          const schemaRowEnabled = !fieldDriven || fieldOptions.length === 0 || rows.some((item) => item.target === row.target);
          return (
          <div className="workflow-table-row" key={row.id}>
            <div className="workflow-mapping-target">
              {targetOptions.length > 0 ? (
                <select value={row.target} onChange={(event) => update(row.id, { target: event.target.value })}>
                  <option value="">Select target</option>
                  {targetOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              ) : fieldDriven ? (
                <label className="workflow-field-enable">
                  <input type="checkbox" checked={schemaRowEnabled} onChange={(event) => toggleSchemaRow(row, event.target.checked)} />
                  <span>
                    <strong>{row.target}</strong>
                    {field?.type ? <small>{field.type}</small> : null}
                    {field?.required ? <em>required</em> : null}
                  </span>
                </label>
              ) : (
                <input value={row.target} onChange={(event) => update(row.id, { target: event.target.value })} placeholder="field or context.path" />
              )}
              {field?.detail ? <small>{field.detail}</small> : null}
            </div>
            <ExpressionInput disabled={!schemaRowEnabled} options={sourceOptions} value={row.source} onChange={(value) => update(row.id, { source: value })} />
            {fieldDriven && fieldOptions.length > 0 ? (
              <span className="workflow-mapping-locked" aria-label="Schema field mapping">
                {schemaRowEnabled ? "on" : "off"}
              </span>
            ) : (
              <button type="button" onClick={() => removeRow(row.id)} aria-label="Remove mapping">
                ×
              </button>
            )}
          </div>
          );
        })}
      </div>
      {fieldDriven && fieldOptions.length > 0 ? null : (
        <button className="secondary-action" type="button" onClick={addRow}>
          + Mapping
        </button>
      )}
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

function OutputSchemaMappingRows({
  enabledSources,
  onChange,
  onEnabledSourcesChange,
  outputFields,
  rows,
  sourcePrefix = "$",
  targetOptions
}: {
  enabledSources: string[];
  onChange: (rows: MappingRowDraft[]) => void;
  onEnabledSourcesChange: (sources: string[]) => void;
  outputFields: SchemaFieldOption[];
  rows: MappingRowDraft[];
  sourcePrefix?: string;
  targetOptions: MappingOption[];
}) {
  const displayRows = rowsForOutputSchemaFields(rows, outputFields, sourcePrefix);
  const enabledSet = new Set(enabledSources);
  for (const row of rows) {
    enabledSet.add(row.source);
  }
  for (const row of displayRows) {
    if (rows.some((item) => equivalentOutputSources(row.source, outputFields, sourcePrefix).includes(item.source))) {
      enabledSet.add(row.source);
    }
  }
  const setSourceEnabled = (source: string, enabled: boolean) => {
    const next = new Set(enabledSources);
    if (enabled) {
      next.add(source);
    } else {
      next.delete(source);
    }
    onEnabledSourcesChange([...next]);
  };
  const update = (rowId: string, target: string) => {
    const currentRow = displayRows.find((row) => row.id === rowId);
    if (!currentRow) return;
    const nextRow = { ...currentRow, target };
    const cleanTarget = target.trim();
    const removableSources = equivalentOutputSources(currentRow.source, outputFields, sourcePrefix);
    const withoutCurrent = rows.filter((row) => !removableSources.includes(row.source));
    const exists = rows.length !== withoutCurrent.length;
    setSourceEnabled(currentRow.source, true);
    if (!cleanTarget) {
      onChange(withoutCurrent);
      return;
    }
    onChange(exists ? [...withoutCurrent, nextRow] : [...rows, nextRow]);
  };
  const toggleOutputRow = (row: MappingRowDraft, enabled: boolean) => {
    setSourceEnabled(row.source, enabled);
    if (enabled) {
      if (!row.target.trim() || rows.some((item) => equivalentOutputSources(row.source, outputFields, sourcePrefix).includes(item.source))) return;
      onChange([...rows, row]);
      return;
    }
    const removableSources = equivalentOutputSources(row.source, outputFields, sourcePrefix);
    onChange(rows.filter((item) => !removableSources.includes(item.source)));
  };
  return (
    <div className="workflow-mapping-block">
      <div className="workflow-output-schema-table">
        <span>Output Field</span>
        <span>Assign To Ctx</span>
        {displayRows.map((row) => {
          const field = outputFields.find((item) => outputFieldSource(item, sourcePrefix) === row.source);
          const enabled = enabledSet.has(row.source);
          return (
            <div className="workflow-table-row" key={row.id}>
              <div className="workflow-mapping-target">
                <label className="workflow-field-enable">
                  <input type="checkbox" checked={enabled} onChange={(event) => toggleOutputRow(row, event.target.checked)} />
                  <span>
                    <strong>{field?.label ?? row.source}</strong>
                    {field?.type ? <small>{field.type}</small> : null}
                  </span>
                </label>
                {field?.detail ? <small>{field.detail}</small> : null}
              </div>
              <ExpressionInput disabled={!enabled} options={targetOptions} value={row.target} onChange={(value) => update(row.id, value)} placeholder="flow_output.result" />
            </div>
          );
        })}
      </div>
    </div>
  );
}

function ExpressionInput({
  constantPlaceholder = "markdown",
  disabled = false,
  onChange,
  options,
  placeholder = "$flow_input.request.query",
  value
}: {
  constantPlaceholder?: string;
  disabled?: boolean;
  onChange: (value: string) => void;
  options: MappingOption[];
  placeholder?: string;
  value: string;
}) {
  const datalistId = useId();
  const [mode, setMode] = useState<"constant" | "variable">(() => expressionMode(value, options));
  useEffect(() => {
    if (value.trim()) {
      setMode(expressionMode(value, options));
    }
  }, [options, value]);
  const handleModeChange = (nextMode: "constant" | "variable") => {
    if (nextMode === mode) return;
    setMode(nextMode);
    if (nextMode === "variable") {
      onChange(options[0]?.value ?? "");
      return;
    }
    if (value.trim() && expressionMode(value, options) === "variable") {
      onChange("");
    }
  };
  const handleValueChange = (nextValue: string) => onChange(nextValue);

  if (options.length === 0) {
    return (
      <div className="workflow-expression-composer">
        <select value="constant" aria-label="Expression mode" disabled>
          <option value="constant">Const</option>
        </select>
        <input value={value} onChange={(event) => handleValueChange(event.target.value)} placeholder={constantPlaceholder} disabled={disabled} />
      </div>
    );
  }
  return (
    <div className="workflow-expression-composer">
      <select value={mode} onChange={(event) => handleModeChange(event.target.value as "constant" | "variable")} aria-label="Expression mode" disabled={disabled}>
        <option value="variable">Var</option>
        <option value="constant">Const</option>
      </select>
      {mode === "variable" ? (
        <input value={value} onChange={(event) => handleValueChange(event.target.value)} list={datalistId} placeholder={placeholder} disabled={disabled} />
      ) : (
        <input value={value} onChange={(event) => handleValueChange(event.target.value)} placeholder={constantPlaceholder} disabled={disabled} />
      )}
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

function expressionMode(value: string, options: MappingOption[]): "constant" | "variable" {
  const trimmed = value.trim();
  if (!trimmed) return "variable";
  if (trimmed.startsWith("$")) return "variable";
  if (options.some((option) => option.value === trimmed)) return "variable";
  return "constant";
}

function ContextSettingsPanel({
  flowInputFields,
  flowOutputFields,
  nodes,
  onClose,
  onFlowInputFieldsChange,
  onFlowOutputFieldsChange,
  onVariableFieldsChange,
  variableFields
}: {
  flowInputFields: ContextFieldDraft[];
  flowOutputFields: ContextFieldDraft[];
  nodes: WorkflowCanvasNode[];
  onClose: () => void;
  onFlowInputFieldsChange: (fields: ContextFieldDraft[]) => void;
  onFlowOutputFieldsChange: (fields: ContextFieldDraft[]) => void;
  onVariableFieldsChange: (fields: ContextFieldDraft[]) => void;
  variableFields: ContextFieldDraft[];
}) {
  const responseAliases = responseAliasOptionsForNodes(nodes);
  return (
    <aside className="workflow-context-sidebar" aria-label="Workflow context settings">
      <header className="node-inspector-header">
        <div>
          <span className="eyebrow">Context Protocol</span>
          <h2>Ctx Settings</h2>
        </div>
        <button className="flow-panel-close" type="button" onClick={onClose} aria-label="Close context settings">
          ×
        </button>
      </header>

      <NodeConfigSection defaultOpen title="Flow Input" meta="read only" helpText="Workflow callers provide these fields. Nodes can read them but cannot write them.">
        <ContextFieldRows fields={flowInputFields} onChange={onFlowInputFieldsChange} />
      </NodeConfigSection>

      <NodeConfigSection title="Flow Output" meta="read / write" helpText="Use output mapping to write final business results here. Nodes can read and write this domain.">
        <ContextFieldRows fields={flowOutputFields} onChange={onFlowOutputFieldsChange} />
      </NodeConfigSection>

      <NodeConfigSection title="Connector / Tool Response" meta="runtime managed" helpText="Connector and tool nodes write raw responses automatically. Configure response alias on the node when the same operation is called multiple times.">
        <div className="workflow-response-alias-list">
          {responseAliases.length > 0 ? (
            responseAliases.map((option) => (
              <code key={option.value}>{option.value}</code>
            ))
          ) : (
            <span>No connector or tool response aliases yet.</span>
          )}
        </div>
      </NodeConfigSection>

      <NodeConfigSection title="Variables" meta="read / write" helpText="Temporary values for orchestration steps. Prefer named business fields over node IDs.">
        <ContextFieldRows fields={variableFields} onChange={onVariableFieldsChange} />
      </NodeConfigSection>
    </aside>
  );
}

function StartNodeSchemaPanel({
  flowInputFields,
  flowOutputFields,
  onFlowInputFieldsChange,
  onFlowOutputFieldsChange
}: {
  flowInputFields: ContextFieldDraft[];
  flowOutputFields: ContextFieldDraft[];
  onFlowInputFieldsChange: (fields: ContextFieldDraft[]) => void;
  onFlowOutputFieldsChange: (fields: ContextFieldDraft[]) => void;
}) {
  return (
    <div className="node-config-section-stack">
      <NodeConfigSection
        defaultOpen
        title="Flow Input"
        meta={`${flowInputFields.length} fields`}
        helpText="Workflow callers provide these fields. For Workflow Tools, this is also the Tool input schema."
      >
        <ContextFieldRows fields={flowInputFields} onChange={onFlowInputFieldsChange} />
      </NodeConfigSection>
      <NodeConfigSection
        defaultOpen
        title="Flow Output"
        meta={`${flowOutputFields.length} fields`}
        helpText="The workflow writes final business results here. For Workflow Tools, this is also the Tool output schema."
      >
        <ContextFieldRows fields={flowOutputFields} onChange={onFlowOutputFieldsChange} />
      </NodeConfigSection>
      <p className="workflow-muted-note">
        Start node owns the workflow boundary contract. Changes here are shared with Ctx Settings and saved as the Workflow Tool schema.
      </p>
    </div>
  );
}

function WorkflowNodeConfigModal({
  agents,
  conditionTargetOptions,
  connectorOperations,
  contextReadOptions,
  contextWriteOptions,
  node,
  nodeRows,
  onClose,
  onConfigChange,
  onNodeDataChange,
  onOutputEnabledSourcesChange,
  onRowsChange,
  outputEnabledSources,
  resourceSchemas,
  selectedNode,
  skills,
  tools
}: {
  agents: AgentProfile[];
  conditionTargetOptions: MappingOption[];
  connectorOperations: ConnectorOperation[];
  contextReadOptions: MappingOption[];
  contextWriteOptions: MappingOption[];
  node: WorkflowNode;
  nodeRows: NodeConfigRows;
  onClose: () => void;
  onConfigChange: (key: string, value: unknown) => void;
  onNodeDataChange: (patch: Partial<WorkflowCanvasNodeData>) => void;
  onOutputEnabledSourcesChange: (sources: string[]) => void;
  onRowsChange: (kind: keyof NodeConfigRows, rows: MappingRowDraft[]) => void;
  outputEnabledSources: string[];
  resourceSchemas: { inputFields: SchemaFieldOption[]; outputFields: SchemaFieldOption[] };
  selectedNode: WorkflowCanvasNode;
  skills: SkillSpec[];
  tools: ToolSpec[];
}) {
  return (
    <div className="agent-node-modal-overlay workflow-node-config-modal-overlay" role="dialog" aria-modal="true" aria-label="Workflow node config">
      <section className="agent-node-modal workflow-node-config-modal">
        <header className="agent-node-modal-topbar">
          <div>
            <strong>{selectedNode.data.label}</strong>
            <span>{nodeTypeLabel(selectedNode.data.nodeType)}</span>
          </div>
          <button className="flow-panel-close" type="button" onClick={onClose} aria-label="Close node config">
            ×
          </button>
        </header>
        <div className="workflow-node-config-modal-layout">
          <div className="workflow-node-config-modal-column">
            <NodeConfigSection defaultOpen title="Basic" meta={nodeTypeLabel(selectedNode.data.nodeType)}>
              <label>
                Name
                <input value={selectedNode.data.label} onChange={(event) => onNodeDataChange({ label: event.target.value })} />
              </label>
              <label>
                Description
                <textarea value={selectedNode.data.description ?? ""} onChange={(event) => onNodeDataChange({ description: event.target.value })} />
              </label>
              {isConnectorOrToolNode(selectedNode.data.nodeType) ? (
                  <WorkflowNodeBindingEditor
                    agents={agents}
                    connectorOperations={connectorOperations}
                    contextReadOptions={contextReadOptions}
                    contextWriteOptions={contextWriteOptions}
                    node={node}
                    nodeOptions={conditionTargetOptions}
                    skills={skills}
                    tools={tools}
                    onConfigChange={onConfigChange}
                />
              ) : null}
            </NodeConfigSection>

            {selectedNode.data.nodeType === "condition" ? (
              <ConditionBranchEditor
                contextReadOptions={contextReadOptions}
                contextWriteOptions={contextWriteOptions}
                node={node}
                nodeOptions={conditionTargetOptions}
                onConfigChange={onConfigChange}
              />
            ) : !isConnectorOrToolNode(selectedNode.data.nodeType) ? (
              <NodeConfigSection defaultOpen title="Binding" meta={bindingMeta(selectedNode.data.nodeType)}>
                <WorkflowNodeBindingEditor
                  agents={agents}
                  connectorOperations={connectorOperations}
                  contextReadOptions={contextReadOptions}
                  contextWriteOptions={contextWriteOptions}
                  node={node}
                  nodeOptions={conditionTargetOptions}
                  skills={skills}
                  tools={tools}
                  onConfigChange={onConfigChange}
                />
              </NodeConfigSection>
            ) : null}
          </div>

          {selectedNode.data.nodeType !== "condition" ? (
            <div className="workflow-node-config-modal-column">
              <NodeConfigSection defaultOpen title="Input Mapping" meta={`${nodeRows.inputRows.length} fields`}>
                <MappingRows
                  fieldOptions={resourceSchemas.inputFields}
                  fieldDriven
                  rows={nodeRows.inputRows}
                  sourceOptions={contextReadOptions}
                  onChange={(rows) => onRowsChange("inputRows", rows)}
                />
              </NodeConfigSection>
            </div>
          ) : null}

          {selectedNode.data.nodeType !== "condition" ? (
            <div className="workflow-node-config-modal-column">
              <NodeConfigSection defaultOpen title="Output Mapping" meta={`${nodeRows.contextRows.length} writes`}>
                {supportsFieldDrivenOutput(selectedNode.data.nodeType) && resourceSchemas.outputFields.length > 0 ? (
                  <OutputSchemaMappingRows
                    rows={nodeRows.contextRows}
                    enabledSources={outputEnabledSources}
                    targetOptions={contextWriteOptions}
                    outputFields={resourceSchemas.outputFields}
                    onEnabledSourcesChange={onOutputEnabledSourcesChange}
                    onChange={(rows) => onRowsChange("contextRows", rows)}
                  />
                ) : (
                  <MappingRows
                    rows={nodeRows.contextRows}
                    sourceOptions={responseReadOptionsFor(node, resourceSchemas.outputFields)}
                    targetOptions={contextWriteOptions}
                    onChange={(rows) => onRowsChange("contextRows", rows)}
                  />
                )}
              </NodeConfigSection>
            </div>
          ) : null}
        </div>
      </section>
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

function WorkflowCanvasNodeView({ data, id, selected }: NodeProps<WorkflowCanvasNode>) {
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
      <span className={`flow-node-type flow-node-type-${data.nodeType}`}>{nodeTypeLabel(data.nodeType)}</span>
      <strong>{data.label}</strong>
      {data.description ? <p className="flow-node-description">{data.description}</p> : null}
      <div className="flow-node-meta">
        <span>{data.nodeType}</span>
      </div>
      <Handle type="source" position={Position.Right} />
    </div>
  );
}

function workflowToCanvasNodes(workflow: WorkflowSpec): WorkflowCanvasNode[] {
  return Object.values(workflow.graph.nodes).map(workflowNodeToCanvasNode);
}

function workflowNodeToCanvasNode(node: WorkflowNode, index = 0): WorkflowCanvasNode {
  return {
    id: node.id,
    type: "workflowCanvasNode",
    position: {
      x: node.position?.x ?? 80 + index * 220,
      y: node.position?.y ?? 180
    },
    data: {
      label: node.name,
      description: node.description,
      nodeType: node.type,
      config: node.config ?? {},
      timeoutMillis: node.timeoutMillis
    }
  };
}

function workflowToCanvasEdges(workflow: WorkflowSpec): WorkflowCanvasEdge[] {
  return workflow.graph.edges.map((edge, index) => ({
    id: edge.id || `${edge.fromNodeId}-${edge.toNodeId}-${index}`,
    source: edge.fromNodeId,
    target: edge.toNodeId,
    label: edge.description,
    type: "smoothstep",
    markerEnd: { type: MarkerType.ArrowClosed },
    data: { edgeType: edge.type ?? "default" }
  }));
}

function canvasToWorkflow(
  workflow: WorkflowSpec,
  nodes: WorkflowCanvasNode[],
  edges: WorkflowCanvasEdge[],
  flowInputFields: ContextFieldDraft[],
  flowOutputFields: ContextFieldDraft[],
  variableFields: ContextFieldDraft[]
): WorkflowSpec {
  const graphNodes = Object.fromEntries(
    nodes.map((node) => [
      node.id,
      canvasNodeToWorkflowNode(node, workflow.graph.nodes[node.id])
    ])
  );
  return {
    ...workflow,
    inputSchema: contextSchemaFromFields(flowInputFields),
    outputSchema: contextSchemaFromFields(flowOutputFields),
    contextSchema: contextSchemaFromFields(variableFields),
    graph: {
      entryNodeId: graphNodes.start ? "start" : nodes[0]?.id ?? "start",
      nodes: graphNodes,
      edges: edges.map((edge) => ({
        id: edge.id,
        fromNodeId: edge.source,
        toNodeId: edge.target,
        type: edge.data?.edgeType ?? "default",
        description: typeof edge.label === "string" ? edge.label : undefined
      }))
    }
  };
}

function syncConditionOutgoingEdges(edges: WorkflowCanvasEdge[], conditionNodeId: string, conditionConfig: Record<string, unknown>): WorkflowCanvasEdge[] {
  const otherEdges = edges.filter((edge) => edge.source !== conditionNodeId);
  const branches = conditionBranchesFromConfig(conditionConfig);
  const defaultBranch = conditionDefaultBranchFromConfig(conditionConfig);
  const branchEdges: WorkflowCanvasEdge[] = branches
    .filter((branch) => branch.nextNodeId.trim())
    .map((branch) => conditionBranchEdge(conditionNodeId, branch.nextNodeId.trim(), branch.name || "Branch", branch.id, "conditional"));
  if (defaultBranch.nextNodeId.trim()) {
    branchEdges.push(conditionBranchEdge(conditionNodeId, defaultBranch.nextNodeId.trim(), "Default", "default", "fallback"));
  }
  return [...otherEdges, ...branchEdges];
}

function addConditionTargetToConfig(config: Record<string, unknown>, targetNodeId: string): Record<string, unknown> {
  const branches = conditionBranchesFromConfig(config);
  const defaultBranch = conditionDefaultBranchFromConfig(config);
  if (!defaultBranch.nextNodeId.trim()) {
    return {
      ...config,
      branches: branches.map(serializeConditionBranch),
      default_branch: serializeConditionDefaultBranch({
        ...defaultBranch,
        nextNodeId: targetNodeId
      })
    };
  }
  const nextBranch = createConditionBranchDraft({
    name: `Branch ${branches.length + 1}`,
    nextNodeId: targetNodeId
  });
  return {
    ...config,
    branches: [...branches, nextBranch].map(serializeConditionBranch),
    default_branch: serializeConditionDefaultBranch(defaultBranch)
  };
}

function clearConditionTargetFromConfig(
  config: Record<string, unknown>,
  targetNodeId: string,
  edgeType?: WorkflowEdge["type"]
): Record<string, unknown> {
  const branches = conditionBranchesFromConfig(config);
  const defaultBranch = conditionDefaultBranchFromConfig(config);
  let clearedBranch = false;
  const nextBranches = branches.map((branch) => {
    if (edgeType !== "fallback" && !clearedBranch && branch.nextNodeId.trim() === targetNodeId) {
      clearedBranch = true;
      return { ...branch, nextNodeId: "" };
    }
    return branch;
  });
  const shouldClearDefault =
    edgeType === "fallback" || (!clearedBranch && defaultBranch.nextNodeId.trim() === targetNodeId);
  return {
    ...config,
    branches: nextBranches.map(serializeConditionBranch),
    default_branch: serializeConditionDefaultBranch({
      ...defaultBranch,
      nextNodeId: shouldClearDefault ? "" : defaultBranch.nextNodeId
    })
  };
}

function clearAllConditionTargetsFromConfig(config: Record<string, unknown>, targetNodeId: string): Record<string, unknown> {
  const branches = conditionBranchesFromConfig(config);
  const defaultBranch = conditionDefaultBranchFromConfig(config);
  return {
    ...config,
    branches: branches
      .map((branch) => ({
        ...branch,
        nextNodeId: branch.nextNodeId.trim() === targetNodeId ? "" : branch.nextNodeId
      }))
      .map(serializeConditionBranch),
    default_branch: serializeConditionDefaultBranch({
      ...defaultBranch,
      nextNodeId: defaultBranch.nextNodeId.trim() === targetNodeId ? "" : defaultBranch.nextNodeId
    })
  };
}

function conditionBranchEdge(
  source: string,
  target: string,
  label: string,
  branchId: string,
  edgeType: WorkflowEdge["type"]
): WorkflowCanvasEdge {
  return {
    id: `${source}-${branchId}-${target}`,
    source,
    target,
    label,
    type: "smoothstep",
    markerEnd: { type: MarkerType.ArrowClosed },
    data: { edgeType }
  };
}

function validateWorkflowCanvas(workflow: WorkflowSpec): string[] {
  const issues: string[] = [];
  const graph = workflow.graph;
  const nodes = Object.values(graph.nodes);
  const nodeIds = new Set(nodes.map((node) => node.id));
  const edges = graph.edges ?? [];
  const outgoing = edgesBySource(edges);

  if (!workflow.name.trim()) {
    issues.push("Workflow name is required.");
  }
  if (!graph.entryNodeId || !nodeIds.has(graph.entryNodeId)) {
    issues.push("Entry node does not exist.");
  }
  if (!nodeIds.has("start")) {
    issues.push("Start node is required.");
  }

  for (const edge of edges) {
    if (!nodeIds.has(edge.fromNodeId)) {
      issues.push(`Edge ${edge.id || `${edge.fromNodeId}->${edge.toNodeId}`} starts from a missing node.`);
    }
    if (!nodeIds.has(edge.toNodeId)) {
      issues.push(`Edge ${edge.id || `${edge.fromNodeId}->${edge.toNodeId}`} points to a missing node.`);
    }
    if (edge.fromNodeId === edge.toNodeId) {
      issues.push(`${workflowNodeLabel(graph.nodes[edge.fromNodeId])} cannot connect to itself.`);
    }
  }

  const startOutgoing = outgoing.get("start") ?? [];
  if (startOutgoing.length > 1) {
    issues.push("Start node can only connect to one next node.");
  }

  const reachable = reachableWorkflowNodes(graph);
  for (const node of nodes) {
    if (node.id !== graph.entryNodeId && !reachable.has(node.id)) {
      issues.push(`${workflowNodeLabel(node)} is not reachable from Start.`);
    }
    issues.push(...validateWorkflowNode(node, graph, edges));
  }

  const cycleAt = firstCycleNodeId(graph);
  if (cycleAt) {
    issues.push(`Workflow graph must be acyclic; cycle detected near ${workflowNodeLabel(graph.nodes[cycleAt])}.`);
  }

  return Array.from(new Set(issues));
}

function validateWorkflowNode(node: WorkflowNode, graph: WorkflowSpec["graph"], edges: WorkflowEdge[]): string[] {
  const issues: string[] = [];
  const label = workflowNodeLabel(node);
  const config = node.config ?? {};

  if (node.type === "connector_operation" && !(stringFromConfig(config, "connector_operation_id") || stringFromConfig(config, "operation_id"))) {
    issues.push(`${label} must bind a connector operation.`);
  }
  if (node.type === "tool" && !stringFromConfig(config, "tool_id")) {
    issues.push(`${label} must bind a tool.`);
  }
  if (node.type === "skill" && !stringFromConfig(config, "skill_id")) {
    issues.push(`${label} must bind a skill.`);
  }
  if (node.type === "agent" && !stringFromConfig(config, "agent_id")) {
    issues.push(`${label} must bind an agent.`);
  }
  if (node.type === "transform" && !stringFromConfig(config, "function_id")) {
    issues.push(`${label} must bind a transform function.`);
  }
  if (node.type === "condition") {
    issues.push(...validateConditionNode(node, graph, edges));
  }

  return issues;
}

function validateConditionNode(node: WorkflowNode, graph: WorkflowSpec["graph"], edges: WorkflowEdge[]): string[] {
  const issues: string[] = [];
  const label = workflowNodeLabel(node);
  const branches = conditionBranchesFromConfig(node.config);
  const defaultBranch = conditionDefaultBranchFromConfig(node.config);
  const hasDefaultWork = Boolean(defaultBranch.nextNodeId.trim() || defaultBranch.writeRows.some((row) => row.target.trim()));

  if (branches.length === 0 && !hasDefaultWork) {
    issues.push(`${label} needs at least one branch or default action.`);
  }

  for (const branch of branches) {
    const branchLabel = `${label} / ${branch.name || branch.id}`;
    if (branch.rules.length === 0) {
      issues.push(`${branchLabel} needs at least one rule.`);
    }
    for (const rule of branch.rules) {
      if (!rule.left.trim()) {
        issues.push(`${branchLabel} has a rule without a left value.`);
      }
      if (!rule.operator.trim()) {
        issues.push(`${branchLabel} has a rule without an operator.`);
      }
      if (conditionOperatorNeedsRightValue(rule.operator) && !rule.right.trim()) {
        issues.push(`${branchLabel} has a rule without a right value.`);
      }
    }
    validateConditionNextNode(branch.nextNodeId, branchLabel, node, graph, edges).forEach((issue) => issues.push(issue));
  }

  validateConditionNextNode(defaultBranch.nextNodeId, `${label} / Default`, node, graph, edges).forEach((issue) => issues.push(issue));
  return issues;
}

function validateConditionNextNode(
  nextNodeId: string,
  branchLabel: string,
  node: WorkflowNode,
  graph: WorkflowSpec["graph"],
  edges: WorkflowEdge[]
): string[] {
  const target = nextNodeId.trim();
  if (!target) return [];
  if (!graph.nodes[target]) {
    return [`${branchLabel} points to a missing next node.`];
  }
  if (!edges.some((edge) => edge.fromNodeId === node.id && edge.toNodeId === target)) {
    return [`${branchLabel} is configured but has no matching canvas edge.`];
  }
  return [];
}

function conditionOperatorNeedsRightValue(operator: string): boolean {
  return !["exists", "not_exists", "is_empty", "is_not_empty"].includes(operator);
}

function reachableWorkflowNodes(graph: WorkflowSpec["graph"]): Set<string> {
  const reachable = new Set<string>();
  const outgoing = edgesBySource(graph.edges ?? []);
  const visit = (nodeId: string) => {
    if (!nodeId || reachable.has(nodeId) || !graph.nodes[nodeId]) return;
    reachable.add(nodeId);
    (outgoing.get(nodeId) ?? []).forEach((edge) => visit(edge.toNodeId));
  };
  visit(graph.entryNodeId);
  return reachable;
}

function firstCycleNodeId(graph: WorkflowSpec["graph"]): string | null {
  const outgoing = edgesBySource(graph.edges ?? []);
  const visiting = new Set<string>();
  const visited = new Set<string>();

  const visit = (nodeId: string): string | null => {
    if (visiting.has(nodeId)) return nodeId;
    if (visited.has(nodeId) || !graph.nodes[nodeId]) return null;
    visiting.add(nodeId);
    for (const edge of outgoing.get(nodeId) ?? []) {
      const cycleAt = visit(edge.toNodeId);
      if (cycleAt) return cycleAt;
    }
    visiting.delete(nodeId);
    visited.add(nodeId);
    return null;
  };

  for (const nodeId of Object.keys(graph.nodes)) {
    const cycleAt = visit(nodeId);
    if (cycleAt) return cycleAt;
  }
  return null;
}

function edgesBySource(edges: WorkflowEdge[]): Map<string, WorkflowEdge[]> {
  const result = new Map<string, WorkflowEdge[]>();
  for (const edge of edges) {
    const current = result.get(edge.fromNodeId) ?? [];
    current.push(edge);
    result.set(edge.fromNodeId, current);
  }
  return result;
}

function workflowNodeLabel(node: WorkflowNode | undefined): string {
  return node?.name || node?.id || "Unknown node";
}

function canvasNodeToWorkflowNode(node: WorkflowCanvasNode, fallback?: WorkflowNode): WorkflowNode {
  return {
    id: node.id,
    type: node.data.nodeType,
    name: node.data.label,
    description: node.data.description,
    position: node.position,
    config: node.data.config ?? fallback?.config ?? {},
    timeoutMillis: node.data.timeoutMillis ?? fallback?.timeoutMillis,
    retryPolicy: fallback?.retryPolicy
  };
}

function parseWorkflowInput(text: string): Record<string, unknown> {
  const trimmed = text.trim();
  if (!trimmed) return {};
  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    throw new Error("Flow Input must be valid JSON.");
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("Flow Input must be a JSON object.");
  }
  return parsed as Record<string, unknown>;
}

function defaultWorkflowInput(fields: ContextFieldDraft[]): Record<string, unknown> {
  const result: Record<string, unknown> = {};
  fields
    .filter((field) => field.path.trim())
    .forEach((field) => setNestedValue(result, field.path.trim(), sampleValueForType(field.type)));
  return result;
}

function setNestedValue(target: Record<string, unknown>, path: string, value: unknown) {
  const parts = path
    .split(".")
    .map((part) => part.trim())
    .filter(Boolean);
  if (parts.length === 0) return;
  let current: Record<string, unknown> = target;
  parts.slice(0, -1).forEach((part) => {
    const existing = current[part];
    if (!existing || typeof existing !== "object" || Array.isArray(existing)) {
      current[part] = {};
    }
    current = current[part] as Record<string, unknown>;
  });
  current[parts[parts.length - 1]] = value;
}

function sampleValueForType(type: string): unknown {
  if (type === "number" || type === "integer") return 0;
  if (type === "boolean") return false;
  if (type === "array") return [];
  if (type === "object") return {};
  return "";
}

function workflowRunFromRecord(run: RunRecord, fallbackWorkflowId: string): WorkflowRun {
  const startedAt = run.started_at ?? new Date().toISOString();
  return {
    id: run.id,
    tenantId: "tenant_1",
    workflowId: runWorkflowId(run) || fallbackWorkflowId,
    status: run.status === "failed" ? "failed" : "succeeded",
    input: run.workflow_request?.input,
    output: workflowOutputFromRunRecord(run),
    error: run.error,
    traceId: run.trace_id,
    startedAt,
    finishedAt: run.finished_at
  };
}

function workflowRunResponseFromRecord(run: RunRecord, fallbackWorkflowId: string): WorkflowRunResponse {
  return {
    run: workflowRunFromRecord(run, fallbackWorkflowId),
    nodeRuns: [],
    error: run.error
  };
}

function workflowRunResponseFromRuntime(
  workflowId: string,
  input: Record<string, unknown>,
  result: RuntimeWorkflowRunResponse,
  traceId: string
): WorkflowRunResponse {
  const now = new Date().toISOString();
  return {
    run: {
      id: result.instance_id || `workflow_run_${Date.now().toString(16)}`,
      tenantId: "tenant_1",
      workflowId,
      status: result.status === "failed" ? "failed" : result.status === "running" ? "running" : "succeeded",
      input,
      output: result.output,
      traceId,
      startedAt: now,
      finishedAt: result.status === "running" ? undefined : now
    },
    nodeRuns: []
  };
}

function runWorkflowId(run: RunRecord): string {
  return run.workflow_request?.workflow_id ?? (run.entrypoint?.kind === "workflow" ? run.entrypoint.id : "");
}

function workflowOutputFromRunRecord(run: RunRecord): Record<string, unknown> | undefined {
  const result = recordFromUnknown(run.result);
  if (!result) return undefined;
  const output = recordFromUnknown(result.output);
  if (output) return output;
  return result;
}

async function workflowFailureTrace(
  traceId: string,
  workflow: WorkflowSpec | undefined,
  input: Record<string, unknown>,
  error: unknown
): Promise<AgentTrace | null> {
  if (traceId) {
    try {
      return agentTraceFromTraceResponse(await runtimeApiV2.getTrace(traceId));
    } catch {
      // Fall through to a local fallback trace so the failed message remains inspectable.
    }
  }
  if (!workflow) return null;
  const now = new Date().toISOString();
  const errorMessage = error instanceof Error ? error.message : "Workflow test failed.";
  return {
    traceId: traceId || `workflow_failed_trace_${Date.now().toString(16)}`,
    tenantId: "tenant_1",
    status: "failed",
    startedAt: now,
    finishedAt: now,
    durationMillis: 0,
    error: errorMessage,
    steps: [
      {
        id: `workflow_failed_${workflow.id || Date.now().toString(16)}`,
        type: "workflow",
        name: workflow.name || workflow.id || "Workflow test",
        status: "failed",
        startedAt: now,
        finishedAt: now,
        durationMillis: 0,
        error: errorMessage,
        metadata: {
          workflow_id: workflow.id,
          request: input
        }
      }
    ]
  };
}

function buildWorkflowTraceFromRun(result: WorkflowRunResponse): AgentTrace {
  const rootStepId = `workflow_run_${result.run.id || result.run.traceId || Date.now().toString(16)}`;
  const steps: TraceStep[] = [
    {
      id: rootStepId,
      type: "event",
      name: "Workflow run",
      status: workflowTraceStepStatusFromRun(result.run.status),
      startedAt: result.run.startedAt,
      finishedAt: result.run.finishedAt,
      durationMillis: durationMillis(result.run.startedAt, result.run.finishedAt),
      error: result.error || result.run.error,
      metadata: {
        scope: "workflow",
        workflow_id: result.run.workflowId,
        run_id: result.run.id,
        node_count: result.nodeRuns.length,
        request: result.run.input,
        response: result.run.output,
        context: result.run.context
      }
    }
  ];

  sortedWorkflowNodeRuns(result.nodeRuns).forEach((nodeRun) => {
    steps.push({
      id: `workflow_node_${nodeRun.id}`,
      parentId: rootStepId,
      type: workflowNodeStepType(nodeRun.nodeType),
      name: nodeRun.nodeName || nodeRun.nodeId,
      status: workflowTraceStatusFromNodeRun(nodeRun.status),
      startedAt: nodeRun.startedAt,
      finishedAt: nodeRun.finishedAt,
      durationMillis: durationMillis(nodeRun.startedAt, nodeRun.finishedAt),
      error: nodeRun.error,
      metadata: {
        scope: "workflow_node",
        workflow_id: nodeRun.workflowId,
        run_id: nodeRun.runId,
        node_id: nodeRun.nodeId,
        node_type: nodeRun.nodeType,
        node_name: nodeRun.nodeName,
        request: nodeRun.input,
        response: nodeRun.output,
        context: nodeRun.context
      }
    });
  });

  return {
    traceId: result.run.traceId || `workflow_trace_${result.run.id}`,
    tenantId: result.run.tenantId,
    eventId: result.run.id,
    status: workflowTraceStatusFromRun(result.run.status),
    startedAt: result.run.startedAt,
    finishedAt: result.run.finishedAt,
    durationMillis: durationMillis(result.run.startedAt, result.run.finishedAt),
    error: result.error || result.run.error,
    steps
  };
}

function workflowRunText(result: WorkflowRunResponse): string {
  const output = recordFromUnknown(result.run.output);
  const runSummary = workflowRunSummaryText(result);
  for (const key of ["text", "answer", "message", "final_answer", "result"]) {
    const value = stringFromRecord(output, key);
    if (value) return `${value}\n\n${runSummary}`;
  }
  if (result.error || result.run.error) return `${result.error || result.run.error || "Workflow failed."}\n\n${runSummary}`;
  if (result.run.output && Object.keys(result.run.output).length > 0) {
    return `Workflow output:\n${JSON.stringify(result.run.output, null, 2)}\n\n${runSummary}`;
  }
  return runSummary;
}

function workflowRunSummaryText(result: WorkflowRunResponse): string {
  const duration = durationMillis(result.run.startedAt, result.run.finishedAt);
  const nodeLines = sortedWorkflowNodeRuns(result.nodeRuns).map((nodeRun, index) => {
    const nodeDuration = durationMillis(nodeRun.startedAt, nodeRun.finishedAt);
    const parts = [
      `${index + 1}. ${nodeRun.nodeName || nodeRun.nodeId}`,
      nodeTypeLabel(nodeRun.nodeType),
      nodeRun.status
    ];
    if (nodeDuration !== undefined) parts.push(`${nodeDuration}ms`);
    if (nodeRun.error) parts.push(nodeRun.error);
    return parts.join(" · ");
  });
  return [
    `Run: ${result.run.status}${duration !== undefined ? ` · ${duration}ms` : ""}`,
    nodeLines.length > 0 ? `Nodes:\n${nodeLines.join("\n")}` : "Nodes: none",
    "Open trace to inspect each node's request, response, and context."
  ].join("\n");
}

function formatWorkflowRunTime(value?: string): string {
  if (!value) return "unknown time";
  const timestamp = Date.parse(value);
  if (Number.isNaN(timestamp)) return value;
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit"
  }).format(new Date(timestamp));
}

function sortedWorkflowNodeRuns(nodeRuns: WorkflowNodeRun[]): WorkflowNodeRun[] {
  return [...nodeRuns].sort((left, right) => (left.startedAt || "").localeCompare(right.startedAt || ""));
}

function workflowTraceStatusFromRun(status: WorkflowRunResponse["run"]["status"]): AgentTrace["status"] {
  if (status === "failed" || status === "canceled") return "failed";
  if (status === "running" || status === "pending") return "running";
  return "succeeded";
}

function workflowTraceStepStatusFromRun(status: WorkflowRunResponse["run"]["status"]): TraceStep["status"] {
  if (status === "failed" || status === "canceled") return "failed";
  if (status === "pending" || status === "running") return "started";
  return "succeeded";
}

function workflowTraceStatusFromNodeRun(status: WorkflowNodeRun["status"]): TraceStep["status"] {
  if (status === "failed" || status === "canceled") return "failed";
  if (status === "pending" || status === "running") return "started";
  if (status === "skipped") return "skipped";
  return "succeeded";
}

function workflowNodeStepType(nodeType: WorkflowNodeType): TraceStep["type"] {
  void nodeType;
  return "node";
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

function bindingMeta(nodeType: WorkflowNodeType): string {
  if (nodeType === "connector_operation") return "operation";
  if (nodeType === "transform") return "function";
  if (nodeType === "tool" || nodeType === "skill" || nodeType === "agent") return nodeType;
  return "mapping";
}

const emptyBindingSchemas: { inputFields: SchemaFieldOption[]; outputFields: SchemaFieldOption[] } = {
  inputFields: [],
  outputFields: []
};

function bindingSchemasForNode(node: WorkflowNode, connectorOperations: ConnectorOperation[], tools: ToolSpec[]) {
  if (node.type === "connector_operation") {
    const operationId = stringFromConfig(node.config, "connector_operation_id") || stringFromConfig(node.config, "operation_id");
    const operation = connectorOperations.find((item) => item.id === operationId);
    return {
      inputFields: schemaFieldOptionsFromSchema(operation?.inputSchema),
      outputFields: schemaFieldOptionsFromSchema(operation?.outputSchema)
    };
  }
  if (node.type === "tool") {
    const toolId = stringFromConfig(node.config, "tool_id");
    const tool = tools.find((item) => item.id === toolId);
    return {
      inputFields: schemaFieldOptionsFromSchema(tool?.inputSchema),
      outputFields: schemaFieldOptionsFromSchema(tool?.outputSchema)
    };
  }
  if (node.type === "transform") {
    const fn = transformFunctionById(stringFromConfig(node.config, "function_id"));
    return {
      inputFields: fn?.inputFields ?? [],
      outputFields: fn?.outputFields ?? []
    };
  }
  return emptyBindingSchemas;
}

function rowsForSchemaFields(rows: MappingRowDraft[], fieldOptions: SchemaFieldOption[]): MappingRowDraft[] {
  const byTarget = new Map(rows.map((row) => [row.target, row]));
  return fieldOptions.map((field) => byTarget.get(field.value) ?? createMappingRow({ target: field.value }));
}

function rowsForOutputSchemaFields(rows: MappingRowDraft[], outputFields: SchemaFieldOption[], sourcePrefix = "$"): MappingRowDraft[] {
  const bySource = new Map(rows.map((row) => [row.source, row]));
  return outputFields.map((field) => {
    const source = outputFieldSource(field, sourcePrefix);
    const legacySource = outputFieldSource(field, "$");
    const existing = bySource.get(source) ?? bySource.get(legacySource);
    return existing ? { ...existing, source } : createMappingRow({ source });
  });
}

function outputFieldSource(field: SchemaFieldOption, sourcePrefix = "$"): string {
  const cleanPrefix = sourcePrefix.trim() || "$";
  if (cleanPrefix === "$") return `$.${field.value}`;
  return `${cleanPrefix.replace(/\.$/, "")}.${field.value}`;
}

function equivalentOutputSources(source: string, outputFields: SchemaFieldOption[], sourcePrefix = "$"): string[] {
  const field = outputFields.find((item) => outputFieldSource(item, sourcePrefix) === source || outputFieldSource(item, "$") === source);
  if (!field) return [source];
  return Array.from(new Set([outputFieldSource(field, sourcePrefix), outputFieldSource(field, "$")]));
}

function isConnectorOrToolNode(nodeType: WorkflowNodeType): boolean {
  return nodeType === "connector_operation" || nodeType === "tool";
}

function supportsFieldDrivenOutput(nodeType: WorkflowNodeType): boolean {
  return nodeType === "connector_operation" || nodeType === "tool" || nodeType === "transform";
}

function outputEnabledSourcesFromConfig(config: Record<string, unknown> | undefined): string[] {
  const value = config?.output_mapping_enabled_sources;
  if (!Array.isArray(value)) return [];
  return value.filter((item): item is string => typeof item === "string");
}

function contextReadOptionsFor(
  flowInputFields: ContextFieldDraft[],
  flowOutputFields: ContextFieldDraft[],
  variableFields: ContextFieldDraft[],
  nodes: WorkflowCanvasNode[]
): MappingOption[] {
  return [
    ...fieldsToOptions(flowInputFields, "$flow_input", "Flow Input"),
    ...fieldsToOptions(flowOutputFields, "$flow_output", "Flow Output"),
    ...fieldsToOptions(variableFields, "$variables", "Variables"),
    ...responseAliasOptionsForNodes(nodes)
  ];
}

function contextWriteOptionsFor(flowOutputFields: ContextFieldDraft[], variableFields: ContextFieldDraft[]): MappingOption[] {
  return [
    ...fieldsToOptions(flowOutputFields, "flow_output", "Flow Output"),
    ...fieldsToOptions(variableFields, "variables", "Variables")
  ];
}

function responseReadOptionsFor(node: WorkflowNode, outputFields: SchemaFieldOption[]): MappingOption[] {
  const alias = responseAliasForNode(node);
  const options: MappingOption[] = [{ value: "$", label: "Raw node response" }];
  options.push(...outputFields.map((field) => ({ ...field, value: outputFieldSource(field), label: `response.${field.label}` })));
  if (node.type === "connector_operation") options.push({ value: `$responses.connector.${alias}`, label: `Stored connector response: ${alias}` });
  if (node.type === "tool") options.push({ value: `$responses.tool.${alias}`, label: `Stored tool response: ${alias}` });
  return options;
}

function fieldsToOptions(fields: ContextFieldDraft[], prefix: string, group: string): MappingOption[] {
  return fields
    .filter((field) => field.path.trim())
    .map((field) => ({
      value: `${prefix}.${field.path.trim()}`,
      label: `${group}: ${field.path.trim()}`,
      detail: field.description
    }));
}

function responseAliasOptionsForNodes(nodes: WorkflowCanvasNode[]): MappingOption[] {
  return nodes.flatMap((node) => {
    if (node.data.nodeType === "connector_operation") {
      const alias = responseAliasForConfig(node.data.config, "connector");
      return [{ value: `$responses.connector.${alias}`, label: `Connector Response: ${alias}` }];
    }
    if (node.data.nodeType === "tool") {
      const alias = responseAliasForConfig(node.data.config, "tool");
      return [{ value: `$responses.tool.${alias}`, label: `Tool Response: ${alias}` }];
    }
    return [];
  });
}

function responseAliasForNode(node: WorkflowNode): string {
  return responseAliasForConfig(node.config, node.type === "tool" ? "tool" : "connector");
}

function responseAliasForConfig(config: Record<string, unknown> | undefined, fallbackType: "connector" | "tool"): string {
  const explicit = stringFromConfig(config, "response_alias");
  if (explicit) return explicit;
  if (fallbackType === "connector") {
    return stringFromConfig(config, "connector_operation_id") || stringFromConfig(config, "operation_id") || "operation_response";
  }
  return stringFromConfig(config, "tool_id") || "tool_response";
}

function schemaFieldOptionsFromSchema(schema?: Record<string, unknown>): SchemaFieldOption[] {
  if (Array.isArray(schema?.["x-flow-fields"])) {
    return contextFieldsFromSchema(schema).map((field) => ({
      value: field.path,
      label: field.path,
      detail: field.description,
      required: field.required,
      type: field.type
    }));
  }
  return flattenJsonSchemaFields(schema);
}

function flattenJsonSchemaFields(schema: Record<string, unknown> | undefined, prefix = ""): SchemaFieldOption[] {
  const properties = objectRecord(schema?.properties);
  if (!properties) return [];
  const requiredFields = new Set(Array.isArray(schema?.required) ? schema.required.map(String) : []);
  return Object.entries(properties).flatMap(([name, definition]) => {
    const fieldSchema = objectRecord(definition) ?? {};
    const path = prefix ? `${prefix}.${name}` : name;
    const type = schemaType(fieldSchema);
    const base: SchemaFieldOption = {
      value: path,
      label: path,
      detail: typeof fieldSchema.description === "string" ? fieldSchema.description : undefined,
      required: requiredFields.has(name),
      type
    };
    const nestedSchema = type === "array" ? objectRecord(fieldSchema.items) : fieldSchema;
    const nested = type === "object" || (type === "array" && schemaType(nestedSchema ?? {}) === "object") ? flattenJsonSchemaFields(nestedSchema, path) : [];
    return nested.length > 0 ? [base, ...nested] : [base];
  });
}

function schemaType(schema: Record<string, unknown>): string {
  if (typeof schema.type === "string") return schema.type;
  if (objectRecord(schema.properties)) return "object";
  return "string";
}

function stringFromConfig(config: Record<string, unknown> | undefined, key: string): string {
  const value = config?.[key];
  return typeof value === "string" ? value : "";
}

function objectRecord(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;
}
