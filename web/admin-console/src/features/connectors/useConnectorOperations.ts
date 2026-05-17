import { useEffect, useMemo, useState } from "react";
import { defaultTenantId } from "../../lib/api";
import { connectorDependencies, connectorOperations, connectors as mockConnectors } from "../../lib/mockData";
import type { Connector, ConnectorDependencies, ConnectorInvokeResult, ConnectorOperation } from "../../types/platform";
import { useAutoDismissNotice, type NoticeState } from "../common/useAutoDismissNotice";
import { connectorOperationsClient } from "./api";
import {
  connectorFromDraft,
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
  const [connectors, setConnectors] = useState<Connector[]>(mockConnectors);
  const [selectedConnectorId, setSelectedConnectorId] = useState(mockConnectors[0]?.id ?? "");
  const [connectorDraft, setConnectorDraft] = useState<ConnectorDraft>(() =>
    mockConnectors[0] ? draftFromConnector(mockConnectors[0]) : createConnectorConfigDraft()
  );
  const [operations, setOperations] = useState<ConnectorOperation[]>(connectorOperations);
  const [selectedOperationId, setSelectedOperationId] = useState(connectorOperations[0]?.id ?? "");
  const [draft, setDraft] = useState<ConnectorOperationDraft>(() =>
    connectorOperations[0] ? draftFromOperation(connectorOperations[0]) : createConnectorDraft()
  );
  const [dependenciesByOperation, setDependenciesByOperation] =
    useState<Record<string, ConnectorDependencies>>(connectorDependencies);
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
  }, []);

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
    try {
      const [remoteConnectors, remoteOperations] = await Promise.all([
        connectorOperationsClient.listConnectors(),
        connectorOperationsClient.listOperations()
      ]);
      setConnectors(remoteConnectors);
      setOperations(remoteOperations);
      setSelectedConnectorId((current) => current || remoteConnectors[0]?.id || "");
      setSelectedOperationId((current) => current || remoteOperations[0]?.id || "");
    } catch {
      setNotice({ ok: false, message: "Using local connector mock data because Platform API is unavailable." });
    }
  }

  async function loadDependencies(operationId: string) {
    try {
      const dependencies = await connectorOperationsClient.getDependencies(operationId);
      setDependenciesByOperation((current) => ({ ...current, [operationId]: dependencies }));
    } catch {
      setDependenciesByOperation((current) => ({
        ...current,
        [operationId]: current[operationId] ?? emptyDependencyReport(operationId)
      }));
    }
  }

  async function saveConnectorDraft() {
    try {
      const connector = connectorFromDraft(connectorDraft, defaultTenantId);
      const saved = await connectorOperationsClient.saveConnector(connector);
      upsertConnector(saved);
      setSelectedConnectorId(saved.id);
      setConnectorDraft(draftFromConnector(saved));
      setNotice({ ok: true, message: "Connector saved." });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to save connector." });
    }
  }

  async function saveDraft() {
    try {
      const operation = operationFromDraft(draft, defaultTenantId);
      const saved = await connectorOperationsClient.saveOperation(operation);
      const savedWithMetadata = preserveCapabilityMetadata(saved, operation);
      upsertOperation(savedWithMetadata);
      setSelectedOperationId(savedWithMetadata.id);
      setDraft(draftFromOperation(savedWithMetadata));
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
    try {
      const saved =
        status === "enabled"
          ? await connectorOperationsClient.enableConnector(connectorId)
          : await connectorOperationsClient.disableConnector(connectorId);
      upsertConnector(saved);
      setConnectorDraft(draftFromConnector(saved));
      setNotice({ ok: true, message: `Connector ${status}.` });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : `Failed to mark connector ${status}.` });
    }
  }

  async function changeStatus(operationId: string, status: ConnectorOperation["status"]) {
    try {
      const saved =
        status === "enabled"
          ? await connectorOperationsClient.enableOperation(operationId)
          : await connectorOperationsClient.disableOperation(operationId);
      const current = operations.find((operation) => operation.id === operationId);
      const savedWithMetadata = current ? preserveCapabilityMetadata(saved, current) : saved;
      upsertOperation(savedWithMetadata);
      setDraft(draftFromOperation(savedWithMetadata));
      setNotice({ ok: true, message: `Connector operation ${status}.` });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : `Failed to mark operation ${status}.` });
    }
  }

  async function testSelectedOperation() {
    if (!selectedOperation) return;
    try {
      const args = JSON.parse(testArgsJson || "{}") as Record<string, unknown>;
      const result = await connectorOperationsClient.testOperation(selectedOperation.id, args);
      setTestResult(result);
      setNotice({ ok: result.success, message: result.success ? "Test invocation succeeded." : "Test invocation failed." });
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
    setConnectors((current) => {
      const exists = current.some((item) => item.id === connector.id);
      if (exists) {
        return current.map((item) => (item.id === connector.id ? connector : item));
      }
      return [connector, ...current];
    });
  }

  function upsertOperation(operation: ConnectorOperation) {
    setOperations((current) => {
      const exists = current.some((item) => item.id === operation.id);
      if (exists) {
        return current.map((item) => (item.id === operation.id ? preserveCapabilityMetadata(operation, item) : item));
      }
      return [operation, ...current];
    });
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

function preserveCapabilityMetadata(operation: ConnectorOperation, fallback: ConnectorOperation): ConnectorOperation {
  return {
    ...operation,
    connectorId: operation.connectorId ?? fallback.connectorId,
    businessDomain: operation.businessDomain ?? fallback.businessDomain,
    ownerTeam: operation.ownerTeam ?? fallback.ownerTeam,
    implementationMode: operation.implementationMode ?? fallback.implementationMode
  };
}
