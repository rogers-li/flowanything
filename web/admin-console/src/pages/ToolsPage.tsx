import { useEffect, useState } from "react";
import { Badge } from "../components/Badge";
import { PageHeader } from "../components/PageHeader";
import { SchemaFieldTable, SchemaFieldViewer } from "../features/connectors/components/SchemaFieldTable";
import { createHeader, createSchemaField, type HeaderDraft, type SchemaFieldDraft } from "../features/connectors/domain";
import { WorkflowCanvasEditor } from "../features/workflows/components/WorkflowCanvasEditor";
import { createWorkflowDraft, workflowFromDraft } from "../features/workflows/domain";
import {
  bindingLabel,
  createToolDraftForImplementation,
  draftFromTool,
  fieldsFromSchema,
  implementationLabel,
  sampleArgsFromFields,
  toolFromDraft,
  type ToolDraft
} from "../features/tools/domain";
import { workflowConfigFromSpec, workflowSpecFromConfig } from "../features/tools/configModel";
import { useTools } from "../features/tools/useTools";
import { resourceApi } from "../platform/configApi";
import type { WorkflowConfig } from "../platform/configTypes";
import type { Connector, ConnectorOperation, ToolExecutionResult, ToolSpec, WorkflowSpec } from "../types/platform";

const localTenantId = "tenant_1";

const riskTone = {
  low: "green",
  medium: "amber",
  high: "red"
} as const;

const statusTone = {
  draft: "gray",
  enabled: "green",
  disabled: "red"
} as const;

type ToolView = "list" | "detail" | "mcpImport" | "connectorImport" | "workflowCanvas";
type DetailSection = "basic" | "binding" | "parameters" | "test";
type MCPImportStep = "server" | "preview";
type ConnectorImportStep = "connector" | "preview";
type ToolSourceFilter = "all" | ToolSpec["implementation"];
type ToolStatusFilter = "enabled" | "all";
type MCPTransport = "streamable_http" | "sse";

type MCPServerImportDraft = {
  name: string;
  description: string;
  url: string;
  transport: MCPTransport;
  headers: HeaderDraft[];
  requireAuthorization: boolean;
};

type MCPImportedToolDraft = {
  id: string;
  toolId?: string;
  name: string;
  description: string;
  inputFields: SchemaFieldDraft[];
  outputFields: SchemaFieldDraft[];
  authRequired: boolean;
  timeoutSeconds: number;
  status?: ToolSpec["status"];
};

type MCPImportMode = "create" | "edit";

type MCPServerGroup = {
  serverId: string;
  tools: ToolSpec[];
};

type ConnectorImportedToolDraft = {
  id: string;
  toolId?: string;
  operationId: string;
  name: string;
  description: string;
  inputFields: SchemaFieldDraft[];
  outputFields: SchemaFieldDraft[];
  sideEffect: ToolSpec["sideEffect"];
  riskLevel: ToolSpec["riskLevel"];
  requiresConfirmation: boolean;
  timeoutMillis: number;
  status?: ToolSpec["status"];
};

type ConnectorToolGroup = {
  connectorId: string;
  connector?: Connector;
  tools: ToolSpec[];
};

const toolTypeOptions: Array<{
  implementation: ToolSpec["implementation"];
  title: string;
  subtitle: string;
  flowLabel: string;
}> = [
  {
    implementation: "connector",
    title: "Connector Tools",
    subtitle: "Import all Operations from one Connector and expose them as agent-callable tools.",
    flowLabel: "Connector Import"
  },
  {
    implementation: "mcp",
    title: "MCP Tool",
    subtitle: "Proxy a registered MCP server capability with platform governance and tracing.",
    flowLabel: "MCP Binding"
  },
  {
    implementation: "knowledge",
    title: "Knowledge Tool",
    subtitle: "Retrieve managed knowledge documents and return structured evidence to agents.",
    flowLabel: "Knowledge Scope"
  },
  {
    implementation: "python",
    title: "Code Adapter Tool",
    subtitle: "Execute a managed code adapter package for custom deterministic logic.",
    flowLabel: "Code Adapter"
  },
  {
    implementation: "workflow",
    title: "Workflow Tool",
    subtitle: "Trigger a governed business workflow that may perform multi-step actions.",
    flowLabel: "Workflow Binding"
  }
];

export function ToolsPage() {
  const [view, setView] = useState<ToolView>("list");
  const [activeSection, setActiveSection] = useState<DetailSection>("basic");
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [mcpImportStep, setMCPImportStep] = useState<MCPImportStep>("server");
  const [mcpImportMode, setMCPImportMode] = useState<MCPImportMode>("create");
  const [mcpServerDraft, setMCPServerDraft] = useState<MCPServerImportDraft>(() => createMCPServerImportDraft());
  const [mcpTools, setMCPTools] = useState<MCPImportedToolDraft[]>([]);
  const [mcpDiscovering, setMCPDiscovering] = useState(false);
  const [mcpDiscoveryError, setMCPDiscoveryError] = useState("");
  const [inspectedMCPTool, setInspectedMCPTool] = useState<ToolSpec | null>(null);
  const [collapsedMCPServers, setCollapsedMCPServers] = useState<Record<string, boolean>>({});
  const [connectorImportStep, setConnectorImportStep] = useState<ConnectorImportStep>("connector");
  const [connectorImportId, setConnectorImportId] = useState("");
  const [connectorTools, setConnectorTools] = useState<ConnectorImportedToolDraft[]>([]);
  const [collapsedConnectorGroups, setCollapsedConnectorGroups] = useState<Record<string, boolean>>({});
  const [workflowCanvasSpec, setWorkflowCanvasSpec] = useState<WorkflowSpec | null>(null);
  const [query, setQuery] = useState("");
  const [sourceFilter, setSourceFilter] = useState<ToolSourceFilter>("all");
  const [statusFilter, setStatusFilter] = useState<ToolStatusFilter>("enabled");
  const {
    activeBundleId,
    tools,
    connectorSystems,
    connectors,
    workflows,
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
  } = useTools();

  const visibleTools = tools.filter((tool) => {
    const normalizedQuery = query.trim().toLowerCase();
    const matchesQuery =
      normalizedQuery.length === 0 ||
      [tool.name, tool.id, tool.description, tool.llmDescription, tool.businessDomain, tool.ownerTeam, tool.binding?.mcpServerId, tool.binding?.mcpToolName, bindingLabel(tool, connectors)]
        .filter(Boolean)
        .some((value) => value?.toLowerCase().includes(normalizedQuery));
    const matchesSource = sourceFilter === "all" || tool.implementation === sourceFilter;
    const matchesStatus = statusFilter === "all" || tool.status === statusFilter;
    return matchesQuery && matchesSource && matchesStatus;
  });
  const currentType = toolTypeOptionFor(draft.implementation);
  const wizardSteps = stepsForImplementation(draft.implementation);
  const selectedConnectorOperation = connectors.find((operation) => operation.id === draft.connectorOperationId);
  const nonGroupedTools = visibleTools.filter((tool) => tool.implementation !== "mcp" && tool.implementation !== "connector");
  const connectorGroups = groupConnectorTools(visibleTools.filter((tool) => tool.implementation === "connector"), connectors, connectorSystems);
  const mcpGroups = groupMCPTools(visibleTools.filter((tool) => tool.implementation === "mcp"));
  const selectedConnectorForImport = connectorSystems.find((connector) => connector.id === connectorImportId);
  const boundWorkflowForDraft = draft.implementation === "workflow" ? workflows.find((workflow) => workflow.id === draft.workflowId) : undefined;
  const workflowSchemaLocked = Boolean(boundWorkflowForDraft);
  const effectiveDraft = draftWithWorkflowSchema(draft, boundWorkflowForDraft);
  const parameterInputFields = workflowSchemaLocked ? fieldsFromSchema(boundWorkflowForDraft?.inputSchema) : draft.inputFields;
  const parameterOutputFields = workflowSchemaLocked ? fieldsFromSchema(boundWorkflowForDraft?.outputSchema) : draft.outputFields;
  const testArgs = parseToolArgs(testArgsJson);
  const readinessChecks = toolReadinessChecks(effectiveDraft, testResult);
  const readyCount = readinessChecks.filter((check) => check.ok).length;

  useEffect(() => {
    if (!mcpDiscoveryError) return;

    const timeout = window.setTimeout(() => setMCPDiscoveryError(""), 6500);
    return () => window.clearTimeout(timeout);
  }, [mcpDiscoveryError]);

  useEffect(() => {
    if (!wizardSteps.some((step) => step.key === activeSection)) {
      setActiveSection(wizardSteps[0]?.key ?? "basic");
    }
  }, [activeSection, wizardSteps]);

  const openNewToolDialog = () => {
    setCreateDialogOpen(true);
  };

  const createTool = (implementation: ToolSpec["implementation"]) => {
    if (implementation === "connector") {
      const firstConnector = connectorSystems[0];
      setConnectorImportId(firstConnector?.id ?? "");
      setConnectorTools(firstConnector ? connectorImportedToolsFromOperations(firstConnector, operationsForConnector(firstConnector.id), tools) : []);
      setConnectorImportStep("connector");
      setCreateDialogOpen(false);
      setView("connectorImport");
      return;
    }
    if (implementation === "mcp") {
      setMCPServerDraft(createMCPServerImportDraft());
      setMCPTools([]);
      setMCPDiscoveryError("");
      setMCPImportStep("server");
      setMCPImportMode("create");
      setCreateDialogOpen(false);
      setView("mcpImport");
      return;
    }
    if (implementation === "workflow") {
      startNewTool("workflow");
      setWorkflowCanvasSpec(null);
      setActiveSection("basic");
      setCreateDialogOpen(false);
      setView("detail");
      return;
    }
    startNewTool(implementation);
    setActiveSection("basic");
    setCreateDialogOpen(false);
    setView("detail");
  };

  const openTool = (toolId: string) => {
    selectTool(toolId);
    setActiveSection("basic");
    setView("detail");
  };

  const openWorkflowCanvasForTool = (tool: ToolSpec) => {
    const toolDraft = draftFromTool(tool);
    const workflow = workflows.find((item) => item.id === tool.binding?.workflowId);
    selectTool(tool.id);
    setDraft(toolDraft);
    setWorkflowCanvasSpec(workflowWithToolContract(toolDraft, workflow ?? null));
    setView("workflowCanvas");
  };

  const openConnectorTool = (toolId: string) => {
    selectTool(toolId);
    setActiveSection("parameters");
    setView("detail");
  };

  const openMCPServer = (group: MCPServerGroup) => {
    setMCPServerDraft(createMCPServerImportDraftFromGroup(group));
    setMCPTools(mcpImportedToolsFromSpecs(group.tools));
    setMCPImportStep("preview");
    setMCPImportMode("edit");
    setView("mcpImport");
  };

  const toggleMCPServerCollapse = (serverId: string) => {
    setCollapsedMCPServers((current) => ({ ...current, [serverId]: !current[serverId] }));
  };

  const openConnectorGroup = (group: ConnectorToolGroup) => {
    if (!group.connector) return;
    setConnectorImportId(group.connector.id);
    setConnectorTools(connectorImportedToolsFromOperations(group.connector, operationsForConnector(group.connector.id), tools));
    setConnectorImportStep("preview");
    setView("connectorImport");
  };

  const toggleConnectorGroupCollapse = (connectorId: string) => {
    setCollapsedConnectorGroups((current) => ({ ...current, [connectorId]: !current[connectorId] }));
  };

  const activeWizardIndex = Math.max(
    0,
    wizardSteps.findIndex((step) => step.key === activeSection)
  );
  const goPreviousStep = () => {
    const previous = wizardSteps[activeWizardIndex - 1];
    if (previous) setActiveSection(previous.key);
  };
  const goNextStep = () => {
    const next = wizardSteps[activeWizardIndex + 1];
    if (next) setActiveSection(next.key);
  };

  const saveCurrentDraft = async () => saveDraft(draftWithWorkflowSchema(draft, boundWorkflowForDraft));

  const discoverMCPTools = async () => {
    setMCPDiscovering(true);
    setMCPDiscoveryError("");
    try {
      const discoveredTools = await discoverMCPToolsFromServer(mcpServerDraft);
      setMCPTools(discoveredTools);
      setMCPImportStep("preview");
    } catch (error) {
      setMCPDiscoveryError(error instanceof Error ? error.message : "Failed to discover MCP tools.");
    } finally {
      setMCPDiscovering(false);
    }
  };

  const syncMCPTools = async () => {
    setMCPDiscovering(true);
    setMCPDiscoveryError("");
    try {
      const discoveredTools = await discoverMCPToolsFromServer(mcpServerDraft);
      setMCPTools((current) => mergeMCPImportedTools(current, discoveredTools));
      setMCPImportStep("preview");
    } catch (error) {
      setMCPDiscoveryError(error instanceof Error ? error.message : "Failed to sync MCP tools.");
    } finally {
      setMCPDiscovering(false);
    }
  };

  const releaseMCPTools = async () => {
    const toolSpecs = mcpTools.map((tool) => toolSpecFromMCPImport(mcpServerDraft, tool));
    const saved = await saveTools(toolSpecs);
    if (saved) {
      setSourceFilter("mcp");
      setStatusFilter("all");
      setView("list");
    }
  };

  const selectConnectorForImport = (connectorId: string) => {
    const connector = connectorSystems.find((item) => item.id === connectorId);
    setConnectorImportId(connectorId);
    setConnectorTools(connector ? connectorImportedToolsFromOperations(connector, operationsForConnector(connector.id), tools) : []);
  };

  const previewConnectorTools = () => {
    if (!selectedConnectorForImport) return;
    setConnectorTools(connectorImportedToolsFromOperations(selectedConnectorForImport, operationsForConnector(selectedConnectorForImport.id), tools));
    setConnectorImportStep("preview");
  };

  const releaseConnectorTools = async () => {
    if (!selectedConnectorForImport) return;
    const toolSpecs = connectorTools.map((tool) => toolSpecFromConnectorImport(selectedConnectorForImport, operationForID(tool.operationId), tool));
    const saved = await importConnectorTools(selectedConnectorForImport.id, toolSpecs);
    if (saved) {
      setSourceFilter("connector");
      setStatusFilter("all");
      setView("list");
    }
  };

  const operationsForConnector = (connectorId: string) => connectors.filter((operation) => operation.connectorId === connectorId);

  const operationForID = (operationId: string) => connectors.find((operation) => operation.id === operationId);

  const workflowWithToolContract = (toolDraft: ToolDraft, workflow: WorkflowSpec | null): WorkflowSpec => {
    const fallbackWorkflow = workflowFromDraft(
      {
        ...createWorkflowDraft("tool_workflow"),
        name: toolDraft.name || "Untitled Tool Workflow",
        description: toolDraft.description || "Compose connector operations and tools into one governed capability."
      },
      localTenantId
    );
    const base = workflow ?? fallbackWorkflow;
    const toolSpec = toolFromDraft(toolDraft, localTenantId);
    return {
      ...base,
      profile: "tool_workflow",
      inputSchema: workflow ? base.inputSchema : toolSpec.inputSchema,
      outputSchema: workflow ? base.outputSchema : toolSpec.outputSchema
    };
  };

  const openWorkflowCanvasForDraft = () => {
    const boundWorkflow = workflows.find((item) => item.id === draft.workflowId);
    setWorkflowCanvasSpec(workflowWithToolContract(draft, boundWorkflow ?? null));
    setView("workflowCanvas");
  };

  const bindWorkflowToDraft = (workflowId: string) => {
    const workflow = workflows.find((item) => item.id === workflowId);
    setDraft({
      ...draft,
      workflowId,
      inputFields: workflow ? fieldsFromSchema(workflow.inputSchema) : draft.inputFields,
      outputFields: workflow ? fieldsFromSchema(workflow.outputSchema) : draft.outputFields
    });
  };

  const saveWorkflowCanvas = async (workflow: WorkflowSpec) => {
    try {
      if (!activeBundleId) {
        throw new Error("Create or select a draft Bundle before saving Workflow Tools.");
      }
      const current = workflow.id ? await fetchCurrentWorkflowConfig(activeBundleId, workflow.id) : undefined;
      const config = workflowConfigFromSpec(workflow, current);
      const response = await resourceApi.upsertResource(activeBundleId, "workflow", config);
      const savedWorkflowConfig = response.bundle.resources?.workflows?.find((item) => item.id === config.id) ?? config;
      const savedWorkflow = workflowSpecFromConfig(savedWorkflowConfig);
      setWorkflowCanvasSpec(savedWorkflow);
      upsertWorkflow(savedWorkflow);
      const nextDraft = {
        ...draft,
        name: draft.name || savedWorkflow.name,
        description: draft.description || savedWorkflow.description || "",
        workflowId: savedWorkflow.id,
        inputFields: fieldsFromSchema(savedWorkflow.inputSchema),
        outputFields: fieldsFromSchema(savedWorkflow.outputSchema),
        implementation: "workflow" as const,
        status: draft.status === "draft" ? "disabled" : draft.status
      };
      setDraft(nextDraft);
      const savedTool = await saveDraft(nextDraft);
      if (savedTool) {
        setActiveSection("basic");
      }
    } catch (error) {
      // Keep the editor open; the shared notice system will surface saveDraft
      // failures, and workflow save failures are shown inline here.
      window.alert(error instanceof Error ? error.message : "Failed to save workflow.");
    }
  };

  if (view === "workflowCanvas" && workflowCanvasSpec) {
    return (
      <WorkflowCanvasEditor
        agents={[]}
        connectorOperations={connectors}
        notice={notice}
        skills={[]}
        tools={tools.filter((tool) => tool.implementation !== "workflow")}
        workflow={workflowCanvasSpec}
        onBack={() => setView("detail")}
        onSave={saveWorkflowCanvas}
      />
    );
  }

  if (view === "connectorImport") {
    return (
      <div className="tool-editor-page">
        <header className="tool-editor-topbar">
          <button className="agent-editor-back" type="button" onClick={() => setView("list")} aria-label="Back to tools">
            <span aria-hidden="true">←</span>
          </button>
          <div className="agent-editor-title">
            <h1>{selectedConnectorForImport ? `Import Tools from ${selectedConnectorForImport.name}` : "Create New Tool - Import from Connector"}</h1>
            <div>
              <Badge tone="green">Connector</Badge>
              <span>{connectorTools.length > 0 ? `${connectorTools.length} operations ready` : "select connector"}</span>
            </div>
          </div>
        </header>

        {notice ? <p className={notice.ok ? "notice notice-ok tool-editor-notice" : "notice notice-error tool-editor-notice"}>{notice.message}</p> : null}

        <main className="tool-editor-workspace">
          <ConnectorImportStepNav activeKey={connectorImportStep} />
          <section className={connectorImportStep === "preview" ? "mcp-import-preview-content" : "tool-editor-content"}>
            {connectorImportStep === "connector" ? renderConnectorImportStep() : renderConnectorPreviewStep()}
          </section>
        </main>

        <footer className="tool-editor-footer">
          {connectorImportStep === "connector" ? (
            <button className="secondary-action" type="button" onClick={() => setView("list")}>
              Cancel
            </button>
          ) : (
            <button className="secondary-action" type="button" onClick={() => setConnectorImportStep("connector")}>
              Previous
            </button>
          )}
          <span>
            {connectorImportStep === "connector"
              ? "Choose one external system connector first"
              : `${connectorTools.length} connector operations will be saved as tools`}
          </span>
          {connectorImportStep === "connector" ? (
            <button
              className="primary-action"
              type="button"
              onClick={previewConnectorTools}
              disabled={!selectedConnectorForImport || connectorTools.length === 0}
            >
              Next
            </button>
          ) : (
            <button className="primary-action" type="button" onClick={() => void releaseConnectorTools()} disabled={connectorTools.length === 0}>
              Release
            </button>
          )}
        </footer>
      </div>
    );
  }

  if (view === "mcpImport") {
    return (
      <div className="tool-editor-page">
        <header className="tool-editor-topbar">
          <button className="agent-editor-back" type="button" onClick={() => setView("list")} aria-label="Back to tools">
            ←
          </button>
          <div className="agent-editor-title">
            <h1>{mcpImportMode === "edit" ? `Manage MCP Server - ${mcpServerDraft.name || "MCP Server"}` : "Create New Tool - Import from MCP Server"}</h1>
            <div>
              <Badge tone="green">MCP</Badge>
              <span>{mcpTools.length > 0 ? `${mcpTools.length} tools discovered` : "server discovery"}</span>
            </div>
          </div>
          <div className="agent-editor-actions">
            {mcpImportMode === "edit" ? (
              <button className="secondary-action" type="button" onClick={() => void syncMCPTools()} disabled={mcpDiscovering}>
                {mcpDiscovering ? "Syncing..." : "Sync"}
              </button>
            ) : null}
          </div>
        </header>

        {notice ? <p className={notice.ok ? "notice notice-ok tool-editor-notice" : "notice notice-error tool-editor-notice"}>{notice.message}</p> : null}
        {mcpDiscoveryError ? <p className="notice notice-error tool-editor-notice">{mcpDiscoveryError}</p> : null}

        <main className="tool-editor-workspace">
          <MCPImportStepNav activeKey={mcpImportStep} />
          <section className={mcpImportStep === "preview" ? "mcp-import-preview-content" : "tool-editor-content"}>
            {mcpImportStep === "server" ? renderMCPServerStep() : renderMCPPreviewStep()}
          </section>
        </main>

        <footer className="tool-editor-footer">
          {mcpImportStep === "server" ? (
            <button className="secondary-action" type="button" onClick={() => setView("list")}>
              Cancel
            </button>
          ) : (
            <button className="secondary-action" type="button" onClick={() => setMCPImportStep("server")}>
              Previous
            </button>
          )}
          <span>{mcpImportStep === "server" ? "Configure MCP server connection first" : `${mcpTools.length} MCP interfaces will be saved as tools`}</span>
          {mcpImportStep === "server" ? (
            <button className="primary-action" type="button" onClick={() => void discoverMCPTools()} disabled={mcpDiscovering || !mcpServerDraft.name.trim() || !mcpServerDraft.url.trim()}>
              {mcpDiscovering ? "Discovering..." : "Next"}
            </button>
          ) : (
            <button className="primary-action" type="button" onClick={() => void releaseMCPTools()} disabled={mcpTools.length === 0}>
              {mcpImportMode === "edit" ? "Save" : "Release"}
            </button>
          )}
        </footer>
      </div>
    );
  }

  if (view === "detail") {
    return (
      <div className="tool-editor-page">
        <header className="tool-editor-topbar">
          <button className="agent-editor-back" type="button" onClick={() => setView("list")} aria-label="Back to tools">
            ←
          </button>
          <div className="agent-editor-title">
            <h1>{draft.id ? `Edit Tool - ${draft.name || draft.id}` : `Create New Tool - ${currentType.title}`}</h1>
            <div>
              <Badge tone={statusTone[selectedTool?.status ?? draft.status]}>{selectedTool?.status ?? draft.status}</Badge>
              <Badge tone={riskTone[draft.riskLevel]}>{draft.riskLevel}</Badge>
              <span>{currentType.title}</span>
            </div>
          </div>
          <div className="agent-editor-actions">
            <button className="secondary-action" type="button" onClick={() => void enableSelected()} disabled={!selectedTool}>
              Enable
            </button>
            <button className="secondary-action" type="button" onClick={() => void disableSelected()} disabled={!selectedTool}>
              Disable
            </button>
            <button className="primary-action" type="button" onClick={() => void saveCurrentDraft()}>
              Save
            </button>
          </div>
        </header>

        {notice ? <p className={notice.ok ? "notice notice-ok tool-editor-notice" : "notice notice-error tool-editor-notice"}>{notice.message}</p> : null}

        <main className="tool-editor-workspace">
          <ToolStepNav activeKey={activeSection} onSelect={setActiveSection} steps={wizardSteps} />
          <section className="tool-editor-content">{renderDetailSection()}</section>
        </main>

        <footer className="tool-editor-footer">
          <button className="secondary-action" type="button" onClick={goPreviousStep} disabled={activeWizardIndex === 0}>
            Previous
          </button>
          <span>{`${readyCount}/${readinessChecks.length} readiness checks passed`}</span>
          {activeWizardIndex === wizardSteps.length - 1 ? (
            <button className="primary-action" type="button" onClick={() => void saveCurrentDraft()}>
              Save Tool
            </button>
          ) : (
            <button className="primary-action" type="button" onClick={goNextStep}>
              Next
            </button>
          )}
        </footer>
      </div>
    );
  }

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="Tool Registry"
        title="Govern LLM-callable actions"
        description="Define what Agents can call, how the action is implemented, and which safety gates must pass before execution."
      />

      <article className="panel tool-registry-panel">
        <div className="tool-registry-toolbar">
          <div className="tool-filter-tabs" aria-label="Tool source filters">
            <button className={sourceFilter === "all" ? "tool-filter-tab-active" : ""} type="button" onClick={() => setSourceFilter("all")}>
              Workspace Tools
            </button>
            <button
              className={sourceFilter === "mcp" ? "tool-filter-tab-active" : ""}
              type="button"
              onClick={() => setSourceFilter("mcp")}
            >
              MCP Tools
            </button>
            <button
              className={sourceFilter === "connector" ? "tool-filter-tab-active" : ""}
              type="button"
              onClick={() => setSourceFilter("connector")}
            >
              Connector Tools
            </button>
            <button
              className={sourceFilter === "workflow" ? "tool-filter-tab-active" : ""}
              type="button"
              onClick={() => setSourceFilter("workflow")}
            >
              Workflow Tools
            </button>
          </div>
          <button className="primary-action" type="button" onClick={openNewToolDialog}>
            New Tool
          </button>
        </div>

        <div className="tool-list-controls">
          <label>
            <span>Search tools</span>
            <input value={query} placeholder="Search by name, source, owner..." onChange={(event) => setQuery(event.target.value)} />
          </label>
          <label>
            <span>Source</span>
            <select value={sourceFilter} onChange={(event) => setSourceFilter(event.target.value as ToolSourceFilter)}>
              <option value="all">All sources</option>
              <option value="connector">Connector</option>
              <option value="knowledge">Knowledge</option>
              <option value="mcp">MCP</option>
              <option value="python">Code Adapter</option>
              <option value="workflow">Workflow</option>
            </select>
          </label>
          <label>
            <span>Status</span>
            <select value={statusFilter} onChange={(event) => setStatusFilter(event.target.value as ToolStatusFilter)}>
              <option value="enabled">Enabled only</option>
              <option value="all">All statuses</option>
            </select>
          </label>
        </div>

        <section className="tool-asset-list" aria-label="Tool list">
          {nonGroupedTools.map((tool) => (
            <ToolAssetRow
              key={tool.id}
              tool={tool}
              source={bindingLabel(tool, connectors)}
              usageCount={dependenciesByTool[tool.id]?.summary.totalConsumerCount ?? 0}
              onOpen={() => openTool(tool.id)}
              onOpenWorkflow={tool.implementation === "workflow" ? () => openWorkflowCanvasForTool(tool) : undefined}
              onStatusChange={(status) => void setToolStatus(tool.id, status)}
            />
          ))}
          {connectorGroups.map((group) => (
            <ConnectorToolGroupBlock
              collapsed={Boolean(collapsedConnectorGroups[group.connectorId])}
              group={group}
              key={group.connectorId}
              onOpenConnector={() => openConnectorGroup(group)}
              onOpenTool={openConnectorTool}
              onStatusChange={(toolId, status) => void setToolStatus(toolId, status)}
              onToggleCollapse={() => toggleConnectorGroupCollapse(group.connectorId)}
            />
          ))}
          {mcpGroups.map((group) => (
            <MCPServerGroupBlock
              collapsed={Boolean(collapsedMCPServers[group.serverId])}
              group={group}
              key={group.serverId}
              onToggleCollapse={() => toggleMCPServerCollapse(group.serverId)}
              onOpenServer={() => openMCPServer(group)}
              onOpenTool={setInspectedMCPTool}
              onStatusChange={(toolId, status) => void setToolStatus(toolId, status)}
            />
          ))}
          {nonGroupedTools.length === 0 && connectorGroups.length === 0 && mcpGroups.length === 0 ? (
            <p className="tool-empty-state">No tools match the current filters.</p>
          ) : null}
        </section>
      </article>

      {createDialogOpen ? <CreateToolDialog onClose={() => setCreateDialogOpen(false)} onCreate={createTool} /> : null}
      {inspectedMCPTool ? <MCPToolInspector tool={inspectedMCPTool} onClose={() => setInspectedMCPTool(null)} /> : null}
    </div>
  );

  function renderConnectorImportStep() {
    return (
      <article className="tool-editor-panel">
        <div className="tool-editor-section-title">
          <div>
            <span className="eyebrow">Step 1</span>
            <h2>Connector Settings</h2>
            <p>Select an external system connector. The next step turns every enabled operation under that connector into a governed platform Tool.</p>
          </div>
        </div>
        <div className="operation-form tool-editor-form">
          <label>
            Connector
            <select value={connectorImportId} onChange={(event) => selectConnectorForImport(event.target.value)}>
              <option value="">Select connector</option>
              {connectorSystems.map((connector) => {
                const operationCount = operationsForConnector(connector.id).length;
                return (
                  <option key={connector.id} value={connector.id}>
                    {`${connector.name} · ${operationCount} operations`}
                  </option>
                );
              })}
            </select>
          </label>
          {selectedConnectorForImport ? (
            <section className="tool-binding-preview">
              <strong>{selectedConnectorForImport.name}</strong>
              <p>{selectedConnectorForImport.description || "No connector description."}</p>
              <div>
                <Badge tone={statusTone[selectedConnectorForImport.status]}>{selectedConnectorForImport.status}</Badge>
                <code>{selectedConnectorForImport.baseUrl}</code>
                <span>{`${connectorTools.length} operations`}</span>
              </div>
            </section>
          ) : (
            <p className="section-hint">Create and configure a Connector first, then import its Operations as Tools here.</p>
          )}
        </div>
      </article>
    );
  }

  function renderConnectorPreviewStep() {
    return (
      <article className="tool-editor-panel mcp-preview-panel">
        <div className="tool-editor-section-title">
          <div>
            <span className="eyebrow">Step 2</span>
            <h2>Preview & Tool Settings</h2>
            <p>Each Connector Operation will be released as one Tool. Existing tools bound to the same operation are updated instead of duplicated.</p>
          </div>
        </div>
        <div className="mcp-tool-preview-table connector-tool-preview-table">
          <table>
            <thead>
              <tr>
                <th>Tool</th>
                <th>Input Parameter</th>
                <th>Policy</th>
                <th>Timeout</th>
              </tr>
            </thead>
            <tbody>
              {connectorTools.map((tool) => (
                <tr key={tool.id}>
                  <td>
                    <strong>{tool.name}</strong>
                    <p>{tool.description}</p>
                  </td>
                  <td>
                    <div className="mcp-parameter-list">
                      {tool.inputFields.map((field) => (
                        <div className="mcp-parameter-row" key={field.id}>
                          <span>{field.name}</span>
                          <code>{field.type}</code>
                          <p>{`${field.required ? "Required" : "Optional"} · ${field.description || "No description"}`}</p>
                        </div>
                      ))}
                      {tool.inputFields.length === 0 ? <p>No input parameters.</p> : null}
                    </div>
                  </td>
                  <td>
                    <div className="connector-tool-policy-cell">
                      <Badge tone={riskTone[tool.riskLevel]}>{tool.riskLevel}</Badge>
                      <span>{tool.sideEffect}</span>
                      <label className="checkbox-field">
                        <input
                          checked={tool.requiresConfirmation}
                          type="checkbox"
                          onChange={(event) => updateConnectorTool(tool.id, { requiresConfirmation: event.target.checked })}
                        />
                        Confirm
                      </label>
                    </div>
                  </td>
                  <td>
                    <label className="mcp-timeout-input">
                      <input
                        min="100"
                        type="number"
                        value={tool.timeoutMillis}
                        onChange={(event) => updateConnectorTool(tool.id, { timeoutMillis: Number(event.target.value) })}
                      />
                      <span>ms</span>
                    </label>
                  </td>
                </tr>
              ))}
              {connectorTools.length === 0 ? (
                <tr>
                  <td colSpan={4}>No operations found under this connector.</td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </article>
    );
  }

  function updateConnectorTool(toolId: string, partial: Partial<ConnectorImportedToolDraft>) {
    setConnectorTools((current) => current.map((tool) => (tool.id === toolId ? { ...tool, ...partial } : tool)));
  }

  function renderMCPServerStep() {
    return (
      <article className="tool-editor-panel">
        <div className="tool-editor-section-title">
          <div>
            <span className="eyebrow">Step 1</span>
            <h2>MCP Server Settings</h2>
            <p>Bind an MCP server first. The next step will discover server tools and turn each MCP interface into an independent platform Tool.</p>
          </div>
        </div>
        <div className="operation-form tool-editor-form">
          <label>
            MCP Server Name
            <input
              placeholder="ai_planner_mcp"
              value={mcpServerDraft.name}
              onChange={(event) => setMCPServerDraft({ ...mcpServerDraft, name: event.target.value })}
            />
          </label>
          <label>
            MCP Server Description
            <textarea
              maxLength={500}
              placeholder="Describe what this MCP server provides."
              value={mcpServerDraft.description}
              onChange={(event) => setMCPServerDraft({ ...mcpServerDraft, description: event.target.value })}
            />
            <span className="tool-field-help">
              <span>Used by platform operators to understand this MCP source</span>
              <span>{`${mcpServerDraft.description.length} / 500`}</span>
            </span>
          </label>
          <label>
            MCP Server IP/URL
            <input
              placeholder="https://example.com/planner/mcp"
              value={mcpServerDraft.url}
              onChange={(event) => setMCPServerDraft({ ...mcpServerDraft, url: event.target.value })}
            />
          </label>
          <section className="tool-editor-subsection">
            <h3>Transport Type</h3>
            <div className="tool-api-type-toggle" aria-label="MCP transport type">
              <button
                className={mcpServerDraft.transport === "streamable_http" ? "tool-api-type-active" : ""}
                type="button"
                onClick={() => setMCPServerDraft({ ...mcpServerDraft, transport: "streamable_http" })}
              >
                Streamable HTTP
              </button>
              <button
                className={mcpServerDraft.transport === "sse" ? "tool-api-type-active" : ""}
                type="button"
                onClick={() => setMCPServerDraft({ ...mcpServerDraft, transport: "sse" })}
              >
                SSE
              </button>
            </div>
          </section>
          <p className="mcp-auth-tip">To add authorization for the MCP Server, configure headers below or require authorization when calling tools.</p>
          <section className="tool-editor-subsection">
            <h3>Header</h3>
            <MCPHeaderTable
              headers={mcpServerDraft.headers}
              onAdd={() => setMCPServerDraft({ ...mcpServerDraft, headers: [...mcpServerDraft.headers, createHeader()] })}
              onChange={(headers) => setMCPServerDraft({ ...mcpServerDraft, headers })}
            />
          </section>
          <section className="tool-editor-subsection">
            <h3>Require Authorization When Calling Tools</h3>
            <button
              className={mcpServerDraft.requireAuthorization ? "tool-status-switch tool-status-switch-on" : "tool-status-switch"}
              type="button"
              aria-pressed={mcpServerDraft.requireAuthorization}
              onClick={() => setMCPServerDraft({ ...mcpServerDraft, requireAuthorization: !mcpServerDraft.requireAuthorization })}
            >
              <span />
            </button>
          </section>
        </div>
      </article>
    );
  }

  function renderMCPPreviewStep() {
    return (
      <article className="tool-editor-panel mcp-preview-panel">
        <div className="tool-editor-section-title">
          <div>
            <span className="eyebrow">Step 2</span>
            <h2>Preview & Tool Settings</h2>
            <p>Review discovered MCP interfaces. Each row will be released as one independent Tool in the registry.</p>
          </div>
        </div>
        <div className="mcp-tool-preview-table">
          <table>
            <thead>
              <tr>
                <th>Tool</th>
                <th>Input Parameter</th>
                <th>Auth Require</th>
                <th>Tool Call Timeout</th>
              </tr>
            </thead>
            <tbody>
              {mcpTools.map((tool) => (
                <tr key={tool.id}>
                  <td>
                    <strong>{tool.name}</strong>
                    <p>{tool.description}</p>
                  </td>
                  <td>
                    <div className="mcp-parameter-list">
                      {tool.inputFields.map((field) => (
                        <div className="mcp-parameter-row" key={field.id}>
                          <span>{field.name}</span>
                          <code>{field.type}</code>
                          <p>{`${field.required ? "Required" : "Optional"} · ${field.description || "No description"}`}</p>
                        </div>
                      ))}
                      {tool.inputFields.length === 0 ? <p>No input parameters.</p> : null}
                    </div>
                  </td>
                  <td>{tool.authRequired || mcpServerDraft.requireAuthorization ? "Yes" : "No"}</td>
                  <td>
                    <label className="mcp-timeout-input">
                      <input
                        min="1"
                        type="number"
                        value={tool.timeoutSeconds}
                        onChange={(event) => updateMCPTool(tool.id, { timeoutSeconds: Number(event.target.value) })}
                      />
                      <span>s</span>
                    </label>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </article>
    );
  }

  function updateMCPTool(toolId: string, partial: Partial<MCPImportedToolDraft>) {
    setMCPTools((current) => current.map((tool) => (tool.id === toolId ? { ...tool, ...partial } : tool)));
  }

  function renderDetailSection() {
    if (activeSection === "binding") {
      return (
        <article className="tool-editor-panel">
          <div className="tool-editor-section-title">
            <div>
              <span className="eyebrow">{`Step ${activeWizardIndex + 1}`}</span>
              <h2>{currentType.flowLabel}</h2>
              <p>{bindingHelpText(draft.implementation)}</p>
            </div>
          </div>
          {renderTypeBindingSection()}
        </article>
      );
    }

    if (activeSection === "parameters") {
      return (
        <article className="tool-editor-panel">
          <div className="tool-editor-section-title">
            <div>
              <span className="eyebrow">{`Step ${activeWizardIndex + 1}`}</span>
              <h2>Parameter Configuration</h2>
              <p>
                {workflowSchemaLocked
                  ? "This Workflow Tool is already bound. Flow Input and Flow Output are managed in the workflow canvas and shown here as read-only."
                  : "Maintain the model-facing contract as structured rows. For Workflow Tools, this draft schema will initialize Flow Input and Flow Output."}
              </p>
            </div>
          </div>
          <div className="operation-form tool-editor-form">
            <label>
              LLM Description
              <textarea
                value={draft.llmDescription}
                placeholder="Explain when the agent should call this tool, required arguments, and what result it returns."
                onChange={(event) => setDraft({ ...draft, llmDescription: event.target.value })}
              />
            </label>
          </div>
          <div className="schema-section-stack">
            <section>
              <h3>Input Fields</h3>
              <SchemaFieldTable
                fields={parameterInputFields}
                onChange={(inputFields) => setDraft({ ...draft, inputFields })}
                readOnly={workflowSchemaLocked}
              />
            </section>
            <section>
              <h3>Output Fields</h3>
              <SchemaFieldTable
                fields={parameterOutputFields}
                onChange={(outputFields) => setDraft({ ...draft, outputFields })}
                readOnly={workflowSchemaLocked}
              />
            </section>
          </div>
          {workflowSchemaLocked ? (
            <p className="section-hint">
              Schema is locked here because it is used by workflow nodes. Open Workflow Canvas and edit the Start node to change Flow Input or Flow Output safely.
            </p>
          ) : null}
          <div className="tool-editor-two-column">
            <section className="tool-editor-subpanel">
              <div className="panel-heading">
                <div>
                  <span className="eyebrow">Validation</span>
                  <h3>Publish readiness</h3>
                </div>
                <Badge tone={readyCount === readinessChecks.length ? "green" : "amber"}>{`${readyCount}/${readinessChecks.length}`}</Badge>
              </div>
              <div className="tool-validation-list">
                {readinessChecks.map((check) => (
                  <div className={check.ok ? "tool-validation-item tool-validation-ok" : "tool-validation-item"} key={check.label}>
                    <strong>{check.label}</strong>
                    <span>{check.detail}</span>
                  </div>
                ))}
              </div>
            </section>
            <section className="tool-editor-subpanel">
              <div className="panel-heading">
                <div>
                  <span className="eyebrow">Usage</span>
                  <h3>Current consumers</h3>
                </div>
                <Badge tone="green">{`${selectedDependencies.summary.totalConsumerCount} consumers`}</Badge>
              </div>
              <div className="dependency-grid">
                <span>Skills</span>
                <strong>{selectedDependencies.summary.directSkillCount}</strong>
                <span>Direct Agents</span>
                <strong>{selectedDependencies.summary.directAgentCount}</strong>
                <span>Via Skills</span>
                <strong>{selectedDependencies.summary.indirectAgentCount}</strong>
              </div>
              <div className="dependency-list">
                {selectedDependencies.directSkills.map((skill) => (
                  <div key={skill.id}>
                    <strong>{skill.name}</strong>
                    <code>{skill.id}</code>
                  </div>
                ))}
                {selectedDependencies.directAgents.map((agent) => (
                  <div key={`direct-${agent.id}`}>
                    <strong>{agent.name}</strong>
                    <code>direct agent · {agent.id}</code>
                  </div>
                ))}
                {selectedDependencies.indirectAgents.map((agent) => (
                  <div key={`indirect-${agent.id}-${agent.viaSkillId ?? ""}`}>
                    <strong>{agent.name}</strong>
                    <code>{`via ${agent.viaSkillId ?? "skill"} · ${agent.id}`}</code>
                  </div>
                ))}
                {selectedDependencies.summary.totalConsumerCount === 0 ? <p>No consumers yet.</p> : null}
              </div>
            </section>
          </div>
        </article>
      );
    }

    if (activeSection === "test") {
      return (
        <article className="tool-editor-panel">
          <div className="tool-editor-section-title">
            <div>
              <span className="eyebrow">{`Step ${activeWizardIndex + 1}`}</span>
              <h2>Test</h2>
              <p>Run the tool through Agent Runtime with structured arguments before enabling it for production agents.</p>
            </div>
          </div>
          <label className="checkbox-field">
            <input checked={testConfirmed} type="checkbox" onChange={(event) => setTestConfirmed(event.target.checked)} />
            Simulate user confirmation
          </label>
          <div className="tool-test-toolbar">
            <button
              className="secondary-action compact-action"
              type="button"
              onClick={() => setTestArgsJson(JSON.stringify(sampleArgsFromFields(draft.inputFields), null, 2))}
            >
              Fill sample args
            </button>
            <span>{`${draft.inputFields.length} input fields`}</span>
          </div>
          <div className="tool-test-form">
            {draft.inputFields.map((field) => (
              <ToolArgumentField
                args={testArgs}
                field={field}
                key={field.id}
                onChange={(value) => setTestArgsJson(JSON.stringify({ ...testArgs, [field.name]: coerceToolArgValue(field, value) }, null, 2))}
              />
            ))}
            {draft.inputFields.length === 0 ? <p>No input fields configured.</p> : null}
          </div>
          <details className="debug-raw">
            <summary>Request payload preview</summary>
            <pre>{JSON.stringify(testArgs, null, 2)}</pre>
          </details>
          {testResult ? (
            <pre className="json-result">
              {JSON.stringify(
                {
                  success: testResult.success,
                  callId: testResult.callId,
                  errorCode: testResult.errorCode,
                  errorReason: testResult.errorReason,
                  data: testResult.data
                },
                null,
                2
              )}
            </pre>
          ) : null}
          <div className="tool-test-runbar">
            <button className="primary-action" type="button" onClick={() => void executeSelected()} disabled={!selectedTool}>
              Run Test
            </button>
            {!selectedTool ? <span>Save the tool before executing a runtime test.</span> : null}
          </div>
        </article>
      );
    }

    return (
      <article className="tool-editor-panel">
        <div className="tool-editor-section-title">
          <div>
            <span className="eyebrow">Step 1</span>
            <h2>Basic Settings</h2>
            <p>Define the human-facing identity and runtime safety policy. The implementation type was selected before entering this flow.</p>
          </div>
        </div>
        <div className="tool-selected-type">
          <span className="tool-type-icon" aria-hidden="true">
            {toolInitials(currentType.title)}
          </span>
          <div>
            <strong>{currentType.title}</strong>
            <p>{currentType.subtitle}</p>
          </div>
        </div>
        <div className="operation-form tool-editor-form">
          <label>
            Tool Name
            <input placeholder="Please input tool name" value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
          </label>
          <label>
            Business Description
            <textarea
              maxLength={500}
              placeholder="Please introduce the feature of the tool to help users better understand and use it."
              value={draft.description}
              onChange={(event) => setDraft({ ...draft, description: event.target.value })}
            />
            <span className="tool-field-help">
              <span>Used by agents and operators to decide when to invoke</span>
              <span>{`${draft.description.length} / 500`}</span>
            </span>
          </label>
          <div className="form-grid">
            <label>
              Business Domain
              <input value={draft.businessDomain} onChange={(event) => setDraft({ ...draft, businessDomain: event.target.value })} />
            </label>
            <label>
              Owner Team
              <input value={draft.ownerTeam} onChange={(event) => setDraft({ ...draft, ownerTeam: event.target.value })} />
            </label>
          </div>
          <section className="tool-editor-subsection">
            <h3>Governance</h3>
            <div className="form-grid">
              <label>
                Side Effect
                <select
                  value={draft.sideEffect}
                  onChange={(event) => setDraft({ ...draft, sideEffect: event.target.value as ToolSpec["sideEffect"] })}
                >
                  <option value="none">none</option>
                  <option value="read">read</option>
                  <option value="write">write</option>
                </select>
              </label>
              <label>
                Risk Level
                <select
                  value={draft.riskLevel}
                  onChange={(event) => setDraft({ ...draft, riskLevel: event.target.value as ToolSpec["riskLevel"] })}
                >
                  <option value="low">low</option>
                  <option value="medium">medium</option>
                  <option value="high">high</option>
                </select>
              </label>
            </div>
            <label className="checkbox-field">
              <input
                checked={draft.requiresConfirmation}
                type="checkbox"
                onChange={(event) => setDraft({ ...draft, requiresConfirmation: event.target.checked })}
              />
              Require explicit user confirmation
            </label>
            <div className="form-grid">
              <label>
                Tool Call Timeout ms
                <input
                  type="number"
                  min="100"
                  value={draft.timeoutMillis}
                  onChange={(event) => setDraft({ ...draft, timeoutMillis: Number(event.target.value) })}
                />
              </label>
              <label>
                Retry Attempts
                <input
                  type="number"
                  min="0"
                  value={draft.retryMaxAttempts}
                  onChange={(event) => setDraft({ ...draft, retryMaxAttempts: Number(event.target.value) })}
                />
              </label>
            </div>
            <label>
              Retry Backoff ms
              <input
                type="number"
                min="0"
                value={draft.retryBackoffMillis}
                onChange={(event) => setDraft({ ...draft, retryBackoffMillis: Number(event.target.value) })}
              />
            </label>
          </section>
        </div>
      </article>
    );
  }

  function renderTypeBindingSection() {
    if (draft.implementation === "connector") {
      return (
        <div className="operation-form tool-editor-form">
          <label>
            Connector Operation
            <select
              value={draft.connectorOperationId}
              onChange={(event) => setDraft({ ...draft, connectorOperationId: event.target.value })}
            >
              <option value="">Select operation</option>
              {connectors.map((operation) => (
                <option key={operation.id} value={operation.id}>
                  {operation.name}
                </option>
              ))}
            </select>
          </label>
          {selectedConnectorOperation ? (
            <section className="tool-binding-preview">
              <strong>{selectedConnectorOperation.name}</strong>
              <p>{selectedConnectorOperation.description}</p>
              <div>
                <Badge tone={statusTone[selectedConnectorOperation.status]}>{selectedConnectorOperation.status}</Badge>
                <code>{`${selectedConnectorOperation.method} ${selectedConnectorOperation.path}`}</code>
                <span>{selectedConnectorOperation.implementationMode ?? "simple_http"}</span>
              </div>
            </section>
          ) : (
            <p className="section-hint">Choose a Connector Operation that has already normalized the external API protocol.</p>
          )}
        </div>
      );
    }
    if (draft.implementation === "knowledge") {
      return (
        <div className="operation-form tool-editor-form">
          <label>
            Knowledge Base IDs
            <input
              placeholder="kb_product_docs, kb_runbook"
              value={draft.knowledgeBaseIds}
              onChange={(event) => setDraft({ ...draft, knowledgeBaseIds: event.target.value })}
            />
          </label>
          <section className="tool-binding-preview">
            <strong>Retrieval contract</strong>
            <p>Knowledge tools are read-only by default. The runtime will pass the query and return ranked evidence documents.</p>
            <div>
              <Badge tone="green">read only</Badge>
              <span>vector + keyword retrieval</span>
              <span>citation friendly</span>
            </div>
          </section>
        </div>
      );
    }
    if (draft.implementation === "mcp") {
      return (
        <div className="operation-form tool-editor-form">
          <div className="form-grid">
            <label>
              MCP Server ID
              <input placeholder="mcp_jira_prod" value={draft.mcpServerId} onChange={(event) => setDraft({ ...draft, mcpServerId: event.target.value })} />
            </label>
            <label>
              MCP Tool Name
              <input placeholder="search_issues" value={draft.mcpToolName} onChange={(event) => setDraft({ ...draft, mcpToolName: event.target.value })} />
            </label>
          </div>
          <section className="tool-binding-preview">
            <strong>MCP execution boundary</strong>
            <p>The platform keeps the Tool policy, arguments, test console, and trace while MCP owns the remote capability implementation.</p>
            <div>
              <span>server scoped</span>
              <span>tool name routed</span>
              <span>trace wrapped</span>
            </div>
          </section>
        </div>
      );
    }
    if (draft.implementation === "python") {
      return (
        <div className="operation-form tool-editor-form">
          <label>
            Code Adapter Package ID
            <input placeholder="pkg_finance_calculator_v1" value={draft.pythonPackageId} onChange={(event) => setDraft({ ...draft, pythonPackageId: event.target.value })} />
          </label>
          <section className="tool-binding-preview">
            <strong>Managed code adapter</strong>
            <p>Use this flow for deterministic logic that is packaged, reviewed, and executed inside the platform runtime boundary.</p>
            <div>
              <span>package id</span>
              <span>structured input</span>
              <span>structured output</span>
            </div>
          </section>
        </div>
      );
    }
    return (
      <div className="operation-form tool-editor-form">
        <label>
          Workflow
          <select value={draft.workflowId} onChange={(event) => bindWorkflowToDraft(event.target.value)}>
            <option value="">Select a workflow</option>
            {workflows.map((workflow) => (
              <option key={workflow.id} value={workflow.id}>
                {workflow.name || workflow.id} · {workflow.status}
              </option>
            ))}
          </select>
        </label>
        {workflows.length === 0 ? (
          <p className="tool-selector-empty">No existing tool workflows found. Use the canvas editor when creating a Workflow Tool.</p>
        ) : null}
        <section className="tool-binding-preview">
          <strong>{draft.workflowId ? "Workflow execution" : "Draft workflow"}</strong>
          <p>
            Flow Input and Flow Output are the Tool input/output schema. Open the canvas to edit the workflow steps or adjust the Start node schema.
          </p>
          <div>
            <Badge tone="amber">side effect</Badge>
            <span>{`${draft.inputFields.length} inputs`}</span>
            <span>{`${draft.outputFields.length} outputs`}</span>
          </div>
          <button className="primary-action compact-action" type="button" onClick={openWorkflowCanvasForDraft}>
            Open Workflow Canvas
          </button>
        </section>
      </div>
    );
  }
}

function CreateToolDialog({
  onClose,
  onCreate
}: {
  onClose: () => void;
  onCreate: (implementation: ToolSpec["implementation"]) => void;
}) {
  return (
    <div className="tool-create-dialog-backdrop" role="presentation">
      <section className="tool-create-dialog" role="dialog" aria-modal="true" aria-labelledby="tool-create-dialog-title">
        <div className="tool-create-dialog-header">
          <div>
            <span className="eyebrow">New Tool</span>
            <h2 id="tool-create-dialog-title">Choose tool type</h2>
            <p>Different tool types have different binding requirements, so choose the implementation first and then follow the dedicated setup flow.</p>
          </div>
          <button className="agent-editor-back" type="button" onClick={onClose} aria-label="Close new tool dialog">
            ×
          </button>
        </div>
        <div className="tool-type-grid">
          {toolTypeOptions.map((option) => (
            <button className="tool-type-card" key={option.implementation} type="button" onClick={() => onCreate(option.implementation)}>
              <span className="tool-type-icon" aria-hidden="true">
                {toolInitials(option.title)}
              </span>
              <span>
                <strong>{option.title}</strong>
                <small>{option.subtitle}</small>
              </span>
              <code>{option.flowLabel}</code>
            </button>
          ))}
        </div>
      </section>
    </div>
  );
}

function MCPImportStepNav({ activeKey }: { activeKey: MCPImportStep }) {
  const steps: Array<{ key: MCPImportStep; label: string }> = [
    { key: "server", label: "MCP Server Settings" },
    { key: "preview", label: "Preview & Tool Settings" }
  ];
  const activeIndex = steps.findIndex((step) => step.key === activeKey);

  return (
    <nav className="tool-step-nav mcp-import-step-nav" aria-label="MCP import steps">
      {steps.map((step, index) => (
        <span
          className={
            step.key === activeKey
              ? "tool-step-button tool-step-button-active"
              : index < activeIndex
                ? "tool-step-button tool-step-button-complete"
                : "tool-step-button"
          }
          key={step.key}
        >
          <span className="tool-step-index">{index < activeIndex ? "✓" : index + 1}</span>
          <span>{step.label}</span>
        </span>
      ))}
    </nav>
  );
}

function ConnectorImportStepNav({ activeKey }: { activeKey: ConnectorImportStep }) {
  const steps: Array<{ key: ConnectorImportStep; label: string }> = [
    { key: "connector", label: "Connector Settings" },
    { key: "preview", label: "Preview & Tool Settings" }
  ];
  const activeIndex = steps.findIndex((step) => step.key === activeKey);

  return (
    <nav className="tool-step-nav mcp-import-step-nav" aria-label="Connector import steps">
      {steps.map((step, index) => (
        <span
          className={
            step.key === activeKey
              ? "tool-step-button tool-step-button-active"
              : index < activeIndex
                ? "tool-step-button tool-step-button-complete"
                : "tool-step-button"
          }
          key={step.key}
        >
          <span className="tool-step-index">{index < activeIndex ? "✓" : index + 1}</span>
          <span>{step.label}</span>
        </span>
      ))}
    </nav>
  );
}

function MCPHeaderTable({
  headers,
  onAdd,
  onChange
}: {
  headers: HeaderDraft[];
  onAdd: () => void;
  onChange: (headers: HeaderDraft[]) => void;
}) {
  const updateHeader = (headerId: string, partial: Partial<HeaderDraft>) => {
    onChange(headers.map((header) => (header.id === headerId ? { ...header, ...partial } : header)));
  };
  const removeHeader = (headerId: string) => {
    onChange(headers.filter((header) => header.id !== headerId));
  };

  return (
    <div className="mcp-header-config">
      <table>
        <thead>
          <tr>
            <th>Key</th>
            <th>Value</th>
            <th>Action</th>
          </tr>
        </thead>
        <tbody>
          {headers.map((header) => (
            <tr key={header.id}>
              <td>
                <input value={header.name} placeholder="Authorization" onChange={(event) => updateHeader(header.id, { name: event.target.value })} />
              </td>
              <td>
                <input value={header.value} placeholder="Bearer ${secret}" onChange={(event) => updateHeader(header.id, { value: event.target.value })} />
              </td>
              <td>
                <button className="secondary-action compact-action" type="button" onClick={() => removeHeader(header.id)}>
                  Remove
                </button>
              </td>
            </tr>
          ))}
          {headers.length === 0 ? (
            <tr>
              <td colSpan={3}>No headers</td>
            </tr>
          ) : null}
        </tbody>
      </table>
      <button className="mcp-add-header" type="button" onClick={onAdd}>
        + Header
      </button>
    </div>
  );
}

function MCPServerGroupBlock({
  collapsed,
  group,
  onOpenServer,
  onOpenTool,
  onToggleCollapse,
  onStatusChange
}: {
  collapsed: boolean;
  group: MCPServerGroup;
  onOpenServer: () => void;
  onOpenTool: (tool: ToolSpec) => void;
  onToggleCollapse: () => void;
  onStatusChange: (toolId: string, status: ToolSpec["status"]) => void;
}) {
  const enabledCount = group.tools.filter((tool) => tool.status === "enabled").length;
  return (
    <section className="mcp-server-group">
      <div className="mcp-server-header">
        <button
          className={collapsed ? "mcp-server-collapse" : "mcp-server-collapse mcp-server-collapse-open"}
          type="button"
          onClick={onToggleCollapse}
          aria-label={collapsed ? `Expand ${group.serverId}` : `Collapse ${group.serverId}`}
          aria-expanded={!collapsed}
        >
          ▸
        </button>
        <span className="tool-type-icon" aria-hidden="true">
          MCP
        </span>
        <button className="mcp-server-title-button" type="button" onClick={onOpenServer}>
          <strong>{group.serverId}</strong>
          <small>{`${group.tools.length} tools · ${enabledCount} enabled · click to manage server`}</small>
        </button>
        <Badge tone="green">MCP Server</Badge>
      </div>
      {collapsed ? null : (
        <div className="mcp-server-tool-list">
          {group.tools.map((tool) => (
            <MCPToolListRow key={tool.id} onOpen={() => onOpenTool(tool)} onStatusChange={onStatusChange} tool={tool} />
          ))}
        </div>
      )}
    </section>
  );
}

function ConnectorToolGroupBlock({
  collapsed,
  group,
  onOpenConnector,
  onOpenTool,
  onToggleCollapse,
  onStatusChange
}: {
  collapsed: boolean;
  group: ConnectorToolGroup;
  onOpenConnector: () => void;
  onOpenTool: (toolId: string) => void;
  onToggleCollapse: () => void;
  onStatusChange: (toolId: string, status: ToolSpec["status"]) => void;
}) {
  const enabledCount = group.tools.filter((tool) => tool.status === "enabled").length;
  return (
    <section className="mcp-server-group connector-tool-group">
      <div className="mcp-server-header">
        <button
          className={collapsed ? "mcp-server-collapse" : "mcp-server-collapse mcp-server-collapse-open"}
          type="button"
          onClick={onToggleCollapse}
          aria-label={collapsed ? `Expand ${group.connectorId}` : `Collapse ${group.connectorId}`}
          aria-expanded={!collapsed}
        >
          ▸
        </button>
        <span className="tool-type-icon" aria-hidden="true">
          API
        </span>
        <button className="mcp-server-title-button" type="button" onClick={onOpenConnector} disabled={!group.connector}>
          <strong>{group.connector?.name ?? group.connectorId}</strong>
          <small>{`${group.tools.length} tools · ${enabledCount} enabled · click to manage connector import`}</small>
        </button>
        <Badge tone="green">Connector</Badge>
      </div>
      {collapsed ? null : (
        <div className="mcp-server-tool-list">
          {group.tools.map((tool) => (
            <ConnectorToolListRow key={tool.id} onOpen={() => onOpenTool(tool.id)} onStatusChange={onStatusChange} tool={tool} />
          ))}
        </div>
      )}
    </section>
  );
}

function MCPToolListRow({
  onOpen,
  onStatusChange,
  tool
}: {
  onOpen: () => void;
  onStatusChange: (toolId: string, status: ToolSpec["status"]) => void;
  tool: ToolSpec;
}) {
  const enabled = tool.status === "enabled";
  return (
    <article className="mcp-tool-row">
      <button className="mcp-tool-row-main" type="button" onClick={onOpen}>
        <strong>{tool.binding?.mcpToolName || tool.name}</strong>
        <span>{tool.description || tool.llmDescription || "No description"}</span>
      </button>
      <span>{schemaPropertyCount(tool.inputSchema)} params</span>
      <Badge tone={statusTone[tool.status]}>{tool.status}</Badge>
      <button
        className={enabled ? "tool-status-switch tool-status-switch-on" : "tool-status-switch"}
        type="button"
        onClick={() => onStatusChange(tool.id, enabled ? "disabled" : "enabled")}
        aria-label={enabled ? `Disable ${tool.name}` : `Enable ${tool.name}`}
        aria-pressed={enabled}
      >
        <span />
      </button>
    </article>
  );
}

function ConnectorToolListRow({
  onOpen,
  onStatusChange,
  tool
}: {
  onOpen: () => void;
  onStatusChange: (toolId: string, status: ToolSpec["status"]) => void;
  tool: ToolSpec;
}) {
  const enabled = tool.status === "enabled";
  return (
    <article className="mcp-tool-row">
      <button className="mcp-tool-row-main" type="button" onClick={onOpen}>
        <strong>{tool.name}</strong>
        <span>{tool.description || tool.llmDescription || "No description"}</span>
      </button>
      <span>{schemaPropertyCount(tool.inputSchema)} params</span>
      <Badge tone={statusTone[tool.status]}>{tool.status}</Badge>
      <button
        className={enabled ? "tool-status-switch tool-status-switch-on" : "tool-status-switch"}
        type="button"
        onClick={() => onStatusChange(tool.id, enabled ? "disabled" : "enabled")}
        aria-label={enabled ? `Disable ${tool.name}` : `Enable ${tool.name}`}
        aria-pressed={enabled}
      >
        <span />
      </button>
    </article>
  );
}

function MCPToolInspector({ onClose, tool }: { onClose: () => void; tool: ToolSpec }) {
  const inputFields = schemaFieldsFromToolSchema(tool.inputSchema);
  const outputFields = schemaFieldsFromToolSchema(tool.outputSchema);
  return (
    <aside className="mcp-tool-inspector" aria-label="MCP tool details">
      <header>
        <div>
          <span className="eyebrow">MCP Tool</span>
          <h2>{tool.binding?.mcpToolName || tool.name}</h2>
          <p>{tool.binding?.mcpServerId ? `from ${tool.binding.mcpServerId}` : "MCP server not bound"}</p>
        </div>
        <button className="agent-editor-back" type="button" onClick={onClose} aria-label="Close MCP tool details">
          ×
        </button>
      </header>
      <section>
        <h3>Description</h3>
        <p>{tool.description || tool.llmDescription || "No description provided by MCP server."}</p>
      </section>
      <section>
        <h3>Input Params</h3>
        <SchemaFieldViewer fields={inputFields} emptyText="No input parameters." />
      </section>
      <section>
        <h3>Output Schema</h3>
        <SchemaFieldViewer fields={outputFields} emptyText="No output fields." />
      </section>
      <section className="mcp-inspector-meta">
        <div>
          <span>Status</span>
          <Badge tone={statusTone[tool.status]}>{tool.status}</Badge>
        </div>
        <div>
          <span>Timeout</span>
          <strong>{`${Math.round(tool.timeoutMillis / 1000)}s`}</strong>
        </div>
        <div>
          <span>Risk</span>
          <Badge tone={riskTone[tool.riskLevel]}>{tool.riskLevel}</Badge>
        </div>
      </section>
    </aside>
  );
}

function SchemaFieldReadOnlyList({ emptyText, fields }: { emptyText: string; fields: SchemaFieldDraft[] }) {
  if (fields.length === 0) {
    return <p className="section-hint">{emptyText}</p>;
  }
  return (
    <div className="mcp-inspector-field-list">
      {fields.map((field) => (
        <SchemaFieldReadOnlyItem field={field} key={field.id} />
      ))}
    </div>
  );
}

function SchemaFieldReadOnlyItem({ field }: { field: SchemaFieldDraft }) {
  const children = field.children ?? [];
  return (
    <div className="mcp-inspector-field-item">
      <strong>{field.name}</strong>
      <code>{schemaFieldLabel(field)}</code>
      <span>{field.required ? "required" : "optional"}</span>
      <p>{field.description || "No description"}</p>
      {children.length > 0 ? (
        <div className="mcp-inspector-field-children">
          {children.map((child) => (
            <SchemaFieldReadOnlyItem field={child} key={child.id} />
          ))}
        </div>
      ) : null}
    </div>
  );
}

function createMCPServerImportDraft(): MCPServerImportDraft {
  return {
    name: "ai_planner_mcp",
    description: "",
    url: "https://test.example.com/planner/mcp",
    transport: "streamable_http",
    headers: [],
    requireAuthorization: false
  };
}

function createMCPServerImportDraftFromGroup(group: MCPServerGroup): MCPServerImportDraft {
  const firstTool = group.tools[0];
  return {
    ...createMCPServerImportDraft(),
    name: group.serverId,
    description: `Imported MCP server with ${group.tools.length} tools.`,
    url: firstTool?.binding?.mcpServerUrl ?? "",
    transport: toMCPTransport(firstTool?.binding?.mcpTransport),
    headers: headersFromRecord(firstTool?.binding?.mcpHeaders ?? {}),
    requireAuthorization: group.tools.some((tool) => tool.requiresConfirmation)
  };
}

function mcpImportedToolsFromSpecs(tools: ToolSpec[]): MCPImportedToolDraft[] {
  return dedupeMCPToolSpecs(tools).map((tool) => ({
    id: tool.id,
    toolId: tool.id,
    name: tool.binding?.mcpToolName || tool.name,
    description: tool.description || tool.llmDescription || "",
    inputFields: schemaFieldsFromToolSchema(tool.inputSchema),
    outputFields: schemaFieldsFromToolSchema(tool.outputSchema),
    authRequired: tool.requiresConfirmation,
    timeoutSeconds: Math.max(1, Math.round(tool.timeoutMillis / 1000)),
    status: tool.status
  }));
}

function mergeMCPImportedTools(current: MCPImportedToolDraft[], discovered: MCPImportedToolDraft[]): MCPImportedToolDraft[] {
  const currentByName = new Map(current.map((tool) => [tool.name, tool]));
  return discovered.map((tool) => {
    const existing = currentByName.get(tool.name);
    if (!existing) return tool;
    return {
      ...tool,
      id: existing.id,
      toolId: existing.toolId,
      timeoutSeconds: existing.timeoutSeconds,
      authRequired: existing.authRequired,
      status: existing.status
    };
  });
}

async function discoverMCPToolsFromServer(server: MCPServerImportDraft): Promise<MCPImportedToolDraft[]> {
  throw new Error(`MCP discovery is not available in the new config runtime yet. Define MCP tools manually for server "${server.name.trim()}".`);
}

function createMCPImportedTool(name: string, description: string, inputFields: SchemaFieldDraft[]): MCPImportedToolDraft {
  return {
    id: createLocalDraftID("mcp_tool"),
    name,
    description,
    inputFields,
    outputFields: [
      createSchemaField({ name: "success", type: "boolean", description: "Whether the MCP call succeeded." }),
      createSchemaField({ name: "data", type: "object", description: "MCP tool result payload." })
    ],
    authRequired: false,
    timeoutSeconds: 60,
    status: "disabled"
  };
}

function toolSpecFromMCPImport(server: MCPServerImportDraft, importedTool: MCPImportedToolDraft): ToolSpec {
  const draft = createToolDraftForImplementation("mcp");
  const spec = toolFromDraft(
    {
      ...draft,
      id: importedTool.toolId ?? "",
      name: importedTool.name,
      description: importedTool.description,
      businessDomain: "MCP",
      ownerTeam: "AI Platform",
      llmDescription: importedTool.description,
      mcpServerId: server.name.trim(),
      mcpToolName: importedTool.name,
      inputFields: importedTool.inputFields,
      outputFields: importedTool.outputFields,
      requiresConfirmation: server.requireAuthorization || importedTool.authRequired,
      timeoutMillis: Math.max(1, importedTool.timeoutSeconds) * 1000,
      status: importedTool.status ?? "disabled"
    },
    localTenantId
  );
  return {
    ...spec,
    binding: {
      ...spec.binding,
      mcpServerUrl: server.url.trim(),
      mcpTransport: server.transport,
      mcpHeaders: headersToRecord(server.headers)
    }
  };
}

function connectorImportedToolsFromOperations(
  connector: Connector,
  operations: ConnectorOperation[],
  currentTools: ToolSpec[]
): ConnectorImportedToolDraft[] {
  const currentByOperation = new Map<string, ToolSpec>();
  for (const tool of currentTools) {
    if (tool.implementation !== "connector") continue;
    const operationId = tool.binding?.connectorOperationId;
    if (operationId && !currentByOperation.has(operationId)) {
      currentByOperation.set(operationId, tool);
    }
  }

  return operations.map((operation) => {
    const current = currentByOperation.get(operation.id);
    return {
      id: current?.id ?? createLocalDraftID("connector_tool"),
      toolId: current?.id,
      operationId: operation.id,
      name: current?.name || operation.name,
      description: current?.description || operation.description,
      inputFields: preferredSchemaFields(current?.inputSchema, operation.inputSchema),
      outputFields: preferredSchemaFields(current?.outputSchema, operation.outputSchema),
      sideEffect: current?.sideEffect ?? sideEffectForConnectorOperation(operation),
      riskLevel: current?.riskLevel ?? riskLevelForConnectorOperation(operation),
      requiresConfirmation: current?.requiresConfirmation ?? requiresConfirmationForConnectorOperation(operation),
      timeoutMillis: current?.timeoutMillis ?? operation.timeoutMillis ?? connector.timeoutMillis,
      status: current?.status ?? "disabled"
    };
  });
}

function toolSpecFromConnectorImport(
  connector: Connector,
  operation: ConnectorOperation | undefined,
  importedTool: ConnectorImportedToolDraft
): ToolSpec {
  const draft = createToolDraftForImplementation("connector");
  const spec = toolFromDraft(
    {
      ...draft,
      id: importedTool.toolId ?? "",
      name: importedTool.name,
      description: importedTool.description,
      businessDomain: connector.businessDomain ?? operation?.businessDomain ?? "General",
      ownerTeam: connector.ownerTeam ?? operation?.ownerTeam ?? "AI Platform",
      llmDescription: importedTool.description,
      connectorOperationId: importedTool.operationId,
      inputFields: importedTool.inputFields,
      outputFields: importedTool.outputFields,
      sideEffect: importedTool.sideEffect,
      riskLevel: importedTool.riskLevel,
      requiresConfirmation: importedTool.requiresConfirmation,
      timeoutMillis: importedTool.timeoutMillis,
      status: importedTool.status ?? "disabled"
    },
    localTenantId
  );
  return spec;
}

function sideEffectForConnectorOperation(operation: ConnectorOperation): ToolSpec["sideEffect"] {
  return connectorOperationLooksReadOnly(operation) ? "read" : "write";
}

function riskLevelForConnectorOperation(operation: ConnectorOperation): ToolSpec["riskLevel"] {
  return connectorOperationLooksReadOnly(operation) ? "low" : "medium";
}

function requiresConfirmationForConnectorOperation(operation: ConnectorOperation): boolean {
  return !connectorOperationLooksReadOnly(operation);
}

function connectorOperationLooksReadOnly(operation: ConnectorOperation): boolean {
  const method = operation.method.toUpperCase();
  if (method === "GET" || method === "HEAD" || method === "OPTIONS") {
    return true;
  }
  if (method !== "POST") {
    return false;
  }

  const readIntentKeywords = ["search", "query", "list", "get", "read", "lookup", "find", "fetch", "extract", "crawl", "map"];
  const text = `${operation.name} ${operation.path} ${operation.description}`.toLowerCase();
  return readIntentKeywords.some((keyword) => text.includes(keyword));
}

function headersFromRecord(headers: Record<string, string>): HeaderDraft[] {
  return Object.entries(headers).map(([name, value]) => createHeader({ name, value }));
}

function headersToRecord(headers: HeaderDraft[]): Record<string, string> {
  return headers.reduce<Record<string, string>>((result, header) => {
    const name = header.name.trim();
    if (name) result[name] = header.value;
    return result;
  }, {});
}

function toMCPTransport(value?: string): MCPTransport {
  return value === "sse" ? "sse" : "streamable_http";
}

function createLocalDraftID(prefix: string): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

function groupMCPTools(tools: ToolSpec[]): MCPServerGroup[] {
  const groups = new Map<string, ToolSpec[]>();
  for (const tool of dedupeMCPToolSpecs(tools)) {
    const serverId = tool.binding?.mcpServerId || "unbound_mcp_server";
    groups.set(serverId, [...(groups.get(serverId) ?? []), tool]);
  }
  return Array.from(groups.entries())
    .map(([serverId, groupTools]) => ({
      serverId,
      tools: [...groupTools].sort((left, right) => left.name.localeCompare(right.name))
    }))
    .sort((left, right) => left.serverId.localeCompare(right.serverId));
}

function groupConnectorTools(
  tools: ToolSpec[],
  operations: ConnectorOperation[],
  connectors: Connector[]
): ConnectorToolGroup[] {
  const operationsByID = new Map(operations.map((operation) => [operation.id, operation]));
  const connectorsByID = new Map(connectors.map((connector) => [connector.id, connector]));
  const groups = new Map<string, ToolSpec[]>();
  for (const tool of dedupeConnectorToolSpecs(tools)) {
    const operation = operationsByID.get(tool.binding?.connectorOperationId ?? "");
    const connectorId = operation?.connectorId || "legacy_connector_operations";
    groups.set(connectorId, [...(groups.get(connectorId) ?? []), tool]);
  }
  return Array.from(groups.entries())
    .map(([connectorId, groupTools]) => ({
      connectorId,
      connector: connectorsByID.get(connectorId),
      tools: [...groupTools].sort((left, right) => left.name.localeCompare(right.name))
    }))
    .sort((left, right) => {
      const leftName = left.connector?.name ?? left.connectorId;
      const rightName = right.connector?.name ?? right.connectorId;
      return leftName.localeCompare(rightName);
    });
}

function dedupeConnectorToolSpecs(tools: ToolSpec[]): ToolSpec[] {
  const seen = new Set<string>();
  const result: ToolSpec[] = [];
  for (const tool of tools) {
    const operationId = tool.binding?.connectorOperationId || tool.id;
    const key = `${tool.tenantId}::${operationId}`;
    if (seen.has(key)) continue;
    seen.add(key);
    result.push(tool);
  }
  return result;
}

function dedupeMCPToolSpecs(tools: ToolSpec[]): ToolSpec[] {
  const seen = new Set<string>();
  const result: ToolSpec[] = [];
  for (const tool of tools) {
    const serverId = tool.binding?.mcpServerId || "unbound_mcp_server";
    const toolName = tool.binding?.mcpToolName || tool.name;
    const key = `${serverId}::${toolName}`;
    if (seen.has(key)) continue;
    seen.add(key);
    result.push(tool);
  }
  return result;
}

function schemaFieldsFromToolSchema(schema?: Record<string, unknown>): SchemaFieldDraft[] {
  const properties = objectRecord(schema?.properties);
  if (!properties) return [];
  const requiredFields = new Set(Array.isArray(schema?.required) ? schema.required.map(String) : []);
  return Object.entries(properties).map(([name, definition]) => {
    const fieldDefinition = objectRecord(definition) ?? {};
    const type = inferSchemaFieldType(fieldDefinition);
    const itemSchema = objectRecord(fieldDefinition.items);
    const arrayItemType = type === "array" ? inferArrayItemType(itemSchema) : undefined;
    const childSchema = type === "array" ? itemSchema : fieldDefinition;
    const children =
      type === "object" || (type === "array" && arrayItemType === "object")
        ? schemaFieldsFromToolSchema(childSchema)
        : undefined;

    return createSchemaField({
      name,
      type,
      arrayItemType,
      required: requiredFields.has(name),
      description: typeof fieldDefinition.description === "string" ? fieldDefinition.description : "",
      children
    });
  });
}

function preferredSchemaFields(currentSchema: Record<string, unknown> | undefined, operationSchema: Record<string, unknown> | undefined): SchemaFieldDraft[] {
  const currentFields = schemaFieldsFromToolSchema(currentSchema);
  const operationFields = schemaFieldsFromToolSchema(operationSchema);
  if (currentFields.length === 0) return operationFields;
  if (schemaDetailScore(operationFields) > schemaDetailScore(currentFields)) return operationFields;
  return currentFields;
}

function schemaDetailScore(fields: SchemaFieldDraft[]): number {
  return fields.reduce((total, field) => total + 1 + schemaDetailScore(field.children ?? []), 0);
}

function inferSchemaFieldType(definition: Record<string, unknown>): SchemaFieldDraft["type"] {
  if (definition.type === "array") return "array";
  if (definition.type === "object" || objectRecord(definition.properties)) return "object";
  return schemaFieldTypeFromValue(definition.type);
}

function inferArrayItemType(itemSchema?: Record<string, unknown>): SchemaFieldDraft["type"] {
  if (!itemSchema) return "string";
  if (itemSchema.type === "object" || objectRecord(itemSchema.properties)) return "object";
  return schemaFieldTypeFromValue(itemSchema.type);
}

function schemaFieldTypeFromValue(value: unknown): SchemaFieldDraft["type"] {
  switch (value) {
    case "number":
    case "integer":
    case "boolean":
    case "object":
    case "array":
      return value;
    case "string":
    default:
      return "string";
  }
}

function schemaFieldLabel(field: SchemaFieldDraft): string {
  return field.type === "array" ? `array<${field.arrayItemType ?? "string"}>` : field.type;
}

function objectRecord(value: unknown): Record<string, unknown> | undefined {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;
}

function schemaPropertyCount(schema?: Record<string, unknown>): number {
  const properties = schema?.properties;
  return properties && typeof properties === "object" && !Array.isArray(properties) ? Object.keys(properties).length : 0;
}

function stepsForImplementation(implementation: ToolSpec["implementation"]): Array<{ key: DetailSection; label: string }> {
  if (implementation === "workflow") {
    return [
      { key: "basic", label: "Basic Settings" },
      { key: "parameters", label: "Parameter Configuration" },
      { key: "binding", label: "Workflow Binding" }
    ];
  }

  return [
    { key: "basic", label: "Basic Settings" },
    { key: "binding", label: toolTypeOptionFor(implementation).flowLabel },
    { key: "parameters", label: "Parameter Configuration" },
    { key: "test", label: "Test" }
  ];
}

async function fetchCurrentWorkflowConfig(bundleId: string, workflowId: string): Promise<WorkflowConfig | undefined> {
  try {
    const document = await resourceApi.getResource<WorkflowConfig>(bundleId, "workflow", workflowId);
    return document.resource;
  } catch {
    return undefined;
  }
}

function draftWithWorkflowSchema(draft: ToolDraft, workflow?: WorkflowSpec): ToolDraft {
  if (draft.implementation !== "workflow" || !workflow) return draft;
  return {
    ...draft,
    inputFields: fieldsFromSchema(workflow.inputSchema),
    outputFields: fieldsFromSchema(workflow.outputSchema)
  };
}

function toolTypeOptionFor(implementation: ToolSpec["implementation"]) {
  return toolTypeOptions.find((option) => option.implementation === implementation) ?? toolTypeOptions[0];
}

function bindingHelpText(implementation: ToolSpec["implementation"]): string {
  switch (implementation) {
    case "connector":
      return "Select the Connector Operation that encapsulates the external API. Tool configuration should focus on agent-friendly invocation, not raw protocol mapping.";
    case "knowledge":
      return "Choose the knowledge bases this tool can retrieve from. The parameter step defines the query and response contract used by the agent.";
    case "mcp":
      return "Bind the platform Tool to a specific MCP server and tool name. Platform policy, schema, testing, and trace stay managed here.";
    case "python":
      return "Bind the Tool to a reviewed code adapter package. Use this for deterministic logic that should not be expressed as prompt instructions.";
    case "workflow":
      return "Bind the Tool to a workflow entrypoint. Workflows often have side effects, so review confirmation and risk settings carefully.";
    default:
      return "Bind this Tool to its executable implementation.";
  }
}

function ToolStepNav({
  activeKey,
  onSelect,
  steps
}: {
  activeKey: DetailSection;
  onSelect: (section: DetailSection) => void;
  steps: Array<{ key: DetailSection; label: string }>;
}) {
  const activeIndex = Math.max(
    0,
    steps.findIndex((step) => step.key === activeKey)
  );

  return (
    <nav className="tool-step-nav" aria-label="Tool creation steps">
      {steps.map((step, index) => {
        const isActive = step.key === activeKey;
        const isCompleted = index < activeIndex;
        return (
          <button
            className={isActive ? "tool-step-button tool-step-button-active" : isCompleted ? "tool-step-button tool-step-button-complete" : "tool-step-button"}
            key={step.key}
            type="button"
            onClick={() => onSelect(step.key)}
          >
            <span className="tool-step-index">{index + 1}</span>
            <span>{step.label}</span>
          </button>
        );
      })}
    </nav>
  );
}

function ToolArgumentField({
  args,
  field,
  onChange
}: {
  args: Record<string, unknown>;
  field: SchemaFieldDraft;
  onChange: (value: string | boolean) => void;
}) {
  const value = args[field.name];
  if (field.type === "boolean") {
    return (
      <label className="tool-test-field tool-test-field-inline">
        <input checked={Boolean(value)} type="checkbox" onChange={(event) => onChange(event.target.checked)} />
        <span>
          <strong>{field.name || "unnamed"}</strong>
          <small>{field.description || "Boolean argument"}</small>
        </span>
      </label>
    );
  }

  return (
    <label className="tool-test-field">
      <span>
        <strong>{field.name || "unnamed"}</strong>
        <small>{`${field.type}${field.required ? " · required" : ""}${field.description ? ` · ${field.description}` : ""}`}</small>
      </span>
      <input
        type={field.type === "number" || field.type === "integer" ? "number" : "text"}
        value={stringifyToolArgValue(value)}
        onChange={(event) => onChange(event.target.value)}
      />
    </label>
  );
}

function ToolAssetRow({
  onOpen,
  onOpenWorkflow,
  onStatusChange,
  source,
  tool,
  usageCount
}: {
  onOpen: () => void;
  onOpenWorkflow?: () => void;
  onStatusChange: (status: ToolSpec["status"]) => void;
  source: string;
  tool: ToolSpec;
  usageCount: number;
}) {
  const enabled = tool.status === "enabled";
  return (
    <article className="tool-asset-row">
      <button className="tool-asset-main" type="button" onClick={onOpen}>
        <span className="tool-asset-icon" aria-hidden="true">
          {toolInitials(tool.name)}
        </span>
        <span className="tool-asset-copy">
          <span>
            <strong>{tool.name}</strong>
            <small>{`from ${source || implementationLabel(tool.implementation)}`}</small>
          </span>
          <span>{tool.llmDescription || tool.description || "No description"}</span>
        </span>
      </button>

      <div className="tool-asset-meta">
        <span>{tool.ownerTeam ?? "AI Platform"}</span>
        <span>{tool.businessDomain ?? "General"}</span>
      </div>

      <div className="tool-asset-badges">
        <Badge tone={riskTone[tool.riskLevel]}>{tool.riskLevel}</Badge>
        <Badge tone={statusTone[tool.status]}>{tool.status}</Badge>
      </div>

      <button className="tool-usage-link" type="button" onClick={onOpen}>
        {`${usageCount} Using`}
      </button>

      {onOpenWorkflow ? (
        <button className="secondary-action compact-action" type="button" onClick={onOpenWorkflow}>
          Canvas
        </button>
      ) : null}

      <button
        className={enabled ? "tool-status-switch tool-status-switch-on" : "tool-status-switch"}
        type="button"
        onClick={() => onStatusChange(enabled ? "disabled" : "enabled")}
        aria-label={enabled ? `Disable ${tool.name}` : `Enable ${tool.name}`}
        aria-pressed={enabled}
      >
        <span />
      </button>

      <button className="tool-row-menu" type="button" onClick={onOpen} aria-label={`Configure ${tool.name}`}>
        ...
      </button>
    </article>
  );
}

function parseToolArgs(raw: string): Record<string, unknown> {
  try {
    const parsed = JSON.parse(raw || "{}") as unknown;
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
  } catch {
    // Keep the form usable if the hidden JSON preview ever becomes invalid.
  }
  return {};
}

function stringifyToolArgValue(value: unknown): string {
  if (value === undefined || value === null) return "";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

function coerceToolArgValue(field: SchemaFieldDraft, value: string | boolean): unknown {
  if (field.type === "boolean") return Boolean(value);
  if (field.type === "integer") {
    const parsed = Number.parseInt(String(value), 10);
    return Number.isNaN(parsed) ? 0 : parsed;
  }
  if (field.type === "number") {
    const parsed = Number.parseFloat(String(value));
    return Number.isNaN(parsed) ? 0 : parsed;
  }
  if (field.type === "object" || field.type === "array") {
    try {
      return JSON.parse(String(value));
    } catch {
      return field.type === "array" ? [] : {};
    }
  }
  return String(value);
}

function toolReadinessChecks(draft: ToolDraft, testResult: ToolExecutionResult | null) {
  const hasBinding =
    (draft.implementation === "connector" && draft.connectorOperationId.trim()) ||
    (draft.implementation === "knowledge" && draft.knowledgeBaseIds.trim()) ||
    (draft.implementation === "mcp" && draft.mcpServerId.trim() && draft.mcpToolName.trim()) ||
    (draft.implementation === "python" && draft.pythonPackageId.trim()) ||
    (draft.implementation === "workflow" && draft.workflowId.trim());
  const allInputFieldsDocumented = schemaFieldsDocumented(draft.inputFields);

  return [
    {
      label: "Basic identity",
      ok: Boolean(draft.name.trim() && draft.description.trim()),
      detail: "Tool name and business description should be clear for operators."
    },
    {
      label: "LLM contract",
      ok: draft.llmDescription.trim().length >= 24,
      detail: "LLM Description should explain when to call the tool and what it returns."
    },
    {
      label: "Input schema",
      ok: draft.inputFields.some((field) => field.name.trim()) && allInputFieldsDocumented,
      detail: "Every input field should have a name and description so the model can generate arguments."
    },
    {
      label: "Executable binding",
      ok: Boolean(hasBinding),
      detail: "Tool must bind to a Connector, Knowledge, MCP, Code Adapter, or Workflow capability."
    },
    {
      label: "Governance",
      ok: draft.sideEffect !== "write" || draft.requiresConfirmation || draft.riskLevel === "high",
      detail: "Write tools should require confirmation or be marked high risk."
    },
    {
      label: "Runtime test",
      ok: testResult?.success === true,
      detail: testResult ? "Latest execution result is included in readiness." : "Run the Test Console before enabling production use."
    }
  ];
}

function schemaFieldsDocumented(fields: SchemaFieldDraft[]): boolean {
  return fields.every((field) => {
    const currentDocumented = !field.name.trim() || Boolean(field.description.trim());
    return currentDocumented && schemaFieldsDocumented(field.children ?? []);
  });
}

function toolInitials(name: string): string {
  const words = name.trim().split(/[-_\s]+/).filter(Boolean);
  if (words.length === 0) return "T";
  if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
  return `${words[0][0]}${words[1][0]}`.toUpperCase();
}
