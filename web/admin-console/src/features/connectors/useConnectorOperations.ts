import { useEffect, useMemo, useState } from "react";
import type { AgentConfig, ConnectorConfig, ConnectorOperationConfig, SkillConfig, ToolConfig } from "../../platform/configTypes";
import { resourceApi, runtimeApiV2 } from "../../platform/configApi";
import { useConfigWorkspace } from "../../platform/ConfigWorkspaceProvider";
import type { Connector, ConnectorDependencies, ConnectorInvokeResult, ConnectorOperation } from "../../types/platform";
import { useAutoDismissNotice, type NoticeState } from "../common/useAutoDismissNotice";
import {
  connectorConfigFromDraft,
  connectorFromConfig,
  connectorInvokeResultFromRuntime,
  connectorOperationConfigFromDraft,
  connectorOperationFromConfig,
  dependenciesForConnectorOperation
} from "./configModel";
import {
  createConnectorConfigDraft,
  createConnectorDraft,
  draftFromConnector,
  draftFromOperation,
  emptyDependencyReport,
  operationFromDraft,
  type ConnectorDraft,
  type ConnectorOperationDraft
} from "./domain";

type SaveResult = NoticeState;

export function useConnectorOperations() {
  const workspace = useConfigWorkspace();
  const [connectors, setConnectors] = useState<Connector[]>([]);
  const [connectorConfigs, setConnectorConfigs] = useState<ConnectorConfig[]>([]);
  const [selectedConnectorId, setSelectedConnectorId] = useState("");
  const [connectorDraft, setConnectorDraft] = useState<ConnectorDraft>(() => createConnectorConfigDraft());
  const [operations, setOperations] = useState<ConnectorOperation[]>([]);
  const [selectedOperationId, setSelectedOperationId] = useState("");
  const [draft, setDraft] = useState<ConnectorOperationDraft>(() => createConnectorDraft());
  const [dependenciesByOperation, setDependenciesByOperation] = useState<Record<string, ConnectorDependencies>>({});
  const [testArgsJson, setTestArgsJson] = useState('{\n  "order_id": "o_123"\n}');
  const [testResult, setTestResult] = useState<ConnectorInvokeResult | null>(null);
  const [notice, setNotice] = useAutoDismissNotice<SaveResult>();

  const selectedOperation = useMemo(
    () => operations.find((operation) => operation.id === selectedOperationId),
    [operations, selectedOperationId]
  );
  const selectedConnector = useMemo(
    () => connectors.find((connector) => connector.id === selectedConnectorId),
    [connectors, selectedConnectorId]
  );
  const selectedDependencies = selectedOperation
    ? dependenciesByOperation[selectedOperation.id] ?? emptyDependencyReport(selectedOperation.id)
    : emptyDependencyReport("");

  useEffect(() => {
    void refreshRegistry();
  }, [workspace.activeBundleId]);

  useEffect(() => {
    if (selectedConnector) {
      setConnectorDraft(draftFromConnector(selectedConnector));
    }
  }, [selectedConnector?.id]);

  useEffect(() => {
    if (!selectedOperation) {
      setDraft(createConnectorDraft(selectedConnector));
      return;
    }
    if (selectedOperation.connectorId) {
      setSelectedConnectorId(selectedOperation.connectorId);
    }
    setDraft(draftFromOperation(selectedOperation));
    void loadDependencies(selectedOperation.id);
  }, [selectedOperation?.id]);

  async function refreshRegistry() {
    if (!workspace.activeBundleId) {
      setConnectors([]);
      setConnectorConfigs([]);
      setOperations([]);
      return;
    }
    try {
      const response = await resourceApi.listResourcesByKind<ConnectorConfig>(workspace.activeBundleId, "connector");
      const configs = response.items.map((item) => item.resource);
      const nextConnectors = configs.map(connectorFromConfig);
      const nextOperations = configs.flatMap((connector) =>
        (connector.operations ?? []).map((operation) => connectorOperationFromConfig(connector, operation))
      );
      setConnectorConfigs(configs);
      setConnectors(nextConnectors);
      setOperations(nextOperations);
      setSelectedConnectorId((current) => current || nextConnectors[0]?.id || "");
      setSelectedOperationId((current) => current || nextOperations[0]?.id || "");
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to load connectors." });
    }
  }

  async function loadDependencies(operationId: string) {
    if (!workspace.activeBundleId) return;
    try {
      const [tools, skills, agents] = await Promise.all([
        resourceApi.listResourcesByKind<ToolConfig>(workspace.activeBundleId, "tool"),
        resourceApi.listResourcesByKind<SkillConfig>(workspace.activeBundleId, "skill"),
        resourceApi.listResourcesByKind<AgentConfig>(workspace.activeBundleId, "agent")
      ]);
      setDependenciesByOperation((current) => ({
        ...current,
        [operationId]: dependenciesForConnectorOperation(
          operationId,
          tools.items.map((item) => item.resource),
          skills.items.map((item) => item.resource),
          agents.items.map((item) => item.resource)
        )
      }));
    } catch {
      setDependenciesByOperation((current) => ({
        ...current,
        [operationId]: current[operationId] ?? emptyDependencyReport(operationId)
      }));
    }
  }

  async function saveConnectorDraft() {
    if (!workspace.activeBundleId) {
      setNotice({ ok: false, message: "No active config bundle selected." });
      return;
    }
    try {
      const connectorId = connectorDraft.id || stableResourceId("conn", connectorDraft.name);
      const current = connectorConfigs.find((item) => item.id === connectorId);
      const operationConfigs = current?.operations ?? [];
      const connector = connectorConfigFromDraft({ ...connectorDraft, id: connectorId }, operationConfigs, current);
      await resourceApi.upsertResource(workspace.activeBundleId, "connector", connector);
      const saved = connectorFromConfig(connector);
      upsertConnector(saved);
      setConnectorConfigs((current) => upsertConfig(current, connector));
      setSelectedConnectorId(saved.id);
      setConnectorDraft(draftFromConnector(saved));
      setNotice({ ok: true, message: "Connector saved." });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to save connector." });
    }
  }

  async function saveDraft() {
    if (!workspace.activeBundleId) {
      setNotice({ ok: false, message: "No active config bundle selected." });
      return;
    }
    try {
      const operation = operationFromDraft(draft, "tenant_1");
      const connectorId = operation.connectorId || selectedConnectorId;
      if (!connectorId) {
        throw new Error("Save the connector before adding operations.");
      }
      const connector = connectorConfigs.find((item) => item.id === connectorId);
      const connectorView = connectors.find((item) => item.id === connectorId);
      if (!connector || !connectorView) {
        throw new Error("Connector not found in active config bundle.");
      }
      const currentOperation = connector.operations?.find((item) => item.id === operation.id);
      const operationConfig = connectorOperationConfigFromDraft({ ...draft, id: operation.id, connectorId }, currentOperation);
      await resourceApi.upsertConnectorOperation(workspace.activeBundleId, connectorId, operationConfig);
      const savedOperation = connectorOperationFromConfig(connector, operationConfig);
      upsertOperation(savedOperation);
      setConnectorConfigs((current) =>
        current.map((item) =>
          item.id === connectorId
            ? {
                ...item,
                operations: upsertConfig(item.operations ?? [], operationConfig)
              }
            : item
        )
      );
      setSelectedOperationId(savedOperation.id);
      setDraft(draftFromOperation(savedOperation));
      setNotice({ ok: true, message: "Connector operation saved." });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to save operation." });
    }
  }

  async function enableSelected() {
    if (!selectedOperation) return;
    await changeStatus(selectedOperation.id, "enabled");
  }

  async function disableSelected() {
    if (!selectedOperation) return;
    await changeStatus(selectedOperation.id, "disabled");
  }

  async function enableSelectedConnector() {
    if (!selectedConnector) return;
    await changeConnectorStatus(selectedConnector.id, "enabled");
  }

  async function disableSelectedConnector() {
    if (!selectedConnector) return;
    await changeConnectorStatus(selectedConnector.id, "disabled");
  }

  async function changeConnectorStatus(connectorId: string, status: Connector["status"]) {
    if (!workspace.activeBundleId) return;
    try {
      const current = connectorConfigs.find((connector) => connector.id === connectorId);
      if (!current) throw new Error("Connector not found in active config bundle.");
      const currentView = connectors.find((connector) => connector.id === connectorId);
      const config = connectorConfigFromDraft(
        {
          ...(currentView ? draftFromConnector(currentView) : createConnectorConfigDraft()),
          status
        },
        current.operations ?? [],
        current
      );
      await resourceApi.upsertResource(workspace.activeBundleId, "connector", config);
      const saved = connectorFromConfig(config);
      upsertConnector(saved);
      setConnectorConfigs((items) => upsertConfig(items, config));
      setConnectorDraft(draftFromConnector(saved));
      setNotice({ ok: true, message: `Connector ${status}.` });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : `Failed to mark connector ${status}.` });
    }
  }

  async function changeStatus(operationId: string, status: ConnectorOperation["status"]) {
    if (!workspace.activeBundleId) return;
    try {
      const current = operations.find((operation) => operation.id === operationId);
      if (!current?.connectorId) throw new Error("Connector operation is not bound to a connector.");
      const connector = connectorConfigs.find((item) => item.id === current.connectorId);
      if (!connector) throw new Error("Connector not found in active config bundle.");
      const currentOperationConfig = connector.operations?.find((operation) => operation.id === operationId);
      const operationConfig = connectorOperationConfigFromDraft({
        ...draftFromOperation(current),
        status
      }, currentOperationConfig);
      await resourceApi.upsertConnectorOperation(workspace.activeBundleId, current.connectorId, operationConfig);
      const saved = connectorOperationFromConfig(connector, operationConfig);
      upsertOperation(saved);
      setConnectorConfigs((items) =>
        items.map((item) =>
          item.id === current.connectorId
            ? {
                ...item,
                operations: upsertConfig(item.operations ?? [], operationConfig)
              }
            : item
        )
      );
      setDraft(draftFromOperation(saved));
      setNotice({ ok: true, message: `Connector operation ${status}.` });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : `Failed to mark operation ${status}.` });
    }
  }

  async function testSelectedOperation() {
    if (!selectedOperation) return;
    try {
      const args = JSON.parse(testArgsJson || "{}") as Record<string, unknown>;
      const result = await runtimeApiV2.invokeConnector({ operation_id: selectedOperation.id, input: args });
      const normalized = connectorInvokeResultFromRuntime(selectedOperation.id, result);
      setTestResult(normalized);
      setNotice({ ok: normalized.success, message: normalized.success ? "Test invocation succeeded." : "Test invocation failed." });
    } catch (error) {
      setTestResult(null);
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to test operation." });
    }
  }

  function startNewConnector() {
    setSelectedConnectorId("");
    setSelectedOperationId("");
    setConnectorDraft(createConnectorConfigDraft());
    setTestResult(null);
  }

  function startNewOperation(connectorId = selectedConnectorId) {
    const parent = connectors.find((connector) => connector.id === connectorId);
    if (connectorId) {
      setSelectedConnectorId(connectorId);
    }
    setSelectedOperationId("");
    setDraft(createConnectorDraft(parent));
    setTestResult(null);
  }

  function selectConnector(connectorId: string) {
    const connector = connectors.find((item) => item.id === connectorId);
    setSelectedConnectorId(connectorId);
    setSelectedOperationId("");
    if (connector) {
      setConnectorDraft(draftFromConnector(connector));
    }
    setTestResult(null);
  }

  function selectOperation(operationId: string) {
    const operation = operations.find((item) => item.id === operationId);
    if (operation?.connectorId) {
      setSelectedConnectorId(operation.connectorId);
    }
    setSelectedOperationId(operationId);
    setTestResult(null);
  }

  function upsertConnector(connector: Connector) {
    setConnectors((current) => upsertById(current, connector));
  }

  function upsertOperation(operation: ConnectorOperation) {
    setOperations((current) => upsertById(current, operation));
  }

  return {
    connectors,
    operations,
    selectedConnector,
    selectedConnectorId,
    selectedOperation,
    selectedOperationId,
    selectedDependencies,
    dependenciesByOperation,
    connectorDraft,
    setConnectorDraft,
    draft,
    setDraft,
    testArgsJson,
    setTestArgsJson,
    testResult,
    notice,
    selectConnector,
    selectOperation,
    startNewConnector,
    startNewOperation,
    saveConnectorDraft,
    saveDraft,
    enableSelectedConnector,
    disableSelectedConnector,
    enableSelected,
    disableSelected,
    testSelectedOperation
  };
}

function upsertById<TItem extends { id: string }>(items: TItem[], item: TItem): TItem[] {
  return items.some((current) => current.id === item.id)
    ? items.map((current) => (current.id === item.id ? item : current))
    : [item, ...items];
}

function upsertConfig<TItem extends { id: string }>(items: TItem[], item: TItem): TItem[] {
  return upsertById(items, item);
}

function stableResourceId(prefix: string, name: string): string {
  const normalized = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
  return `${prefix}_${normalized || Date.now().toString(16)}`;
}
