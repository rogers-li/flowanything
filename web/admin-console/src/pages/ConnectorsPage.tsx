import { useMemo, useState } from "react";
import { Badge } from "../components/Badge";
import { HeaderTable } from "../features/connectors/components/HeaderTable";
import { SchemaFieldTable } from "../features/connectors/components/SchemaFieldTable";
import {
  connectorAuthRequiresEnvironmentSecret,
  connectorSecretEnvName,
  connectorSecretRefForDraft,
  contractSummary
} from "../features/connectors/domain";
import { useConnectorOperations } from "../features/connectors/useConnectorOperations";
import type { Connector, ConnectorOperation } from "../types/platform";

const statusTone = {
  draft: "gray",
  enabled: "green",
  disabled: "red"
} as const;

type ConnectorView = "list" | "connectorDetail" | "operationDetail";
type ConnectorSection = "basic" | "connection" | "operations";
type OperationSection = "configuration" | "test" | "usage";

const connectorSections: Array<{ key: ConnectorSection; label: string }> = [
  { key: "basic", label: "Basic Settings" },
  { key: "connection", label: "Connection & Auth" },
  { key: "operations", label: "Operations" }
];

const operationSections: Array<{ key: OperationSection; label: string }> = [
  { key: "configuration", label: "Operation Configuration" },
  { key: "test", label: "Test" },
  { key: "usage", label: "Usage" }
];

type ConnectorGroup = {
  id: string;
  connector?: Connector;
  operations: ConnectorOperation[];
  isLegacy?: boolean;
};

export function ConnectorsPage() {
  const [view, setView] = useState<ConnectorView>("list");
  const [connectorSection, setConnectorSection] = useState<ConnectorSection>("basic");
  const [operationSection, setOperationSection] = useState<OperationSection>("configuration");
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({});
  const {
    connectors,
    operations,
    selectedConnector,
    selectedOperation,
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
  } = useConnectorOperations();

  const groups = useMemo(() => buildConnectorGroups(connectors, operations), [connectors, operations]);

  const openNewConnector = () => {
    startNewConnector();
    setConnectorSection("basic");
    setView("connectorDetail");
  };

  const openConnector = (connectorId: string) => {
    selectConnector(connectorId);
    setConnectorSection("basic");
    setView("connectorDetail");
  };

  const openNewOperation = (connectorId?: string) => {
    startNewOperation(connectorId);
    setOperationSection("configuration");
    setView("operationDetail");
  };

  const openOperation = (operationId: string) => {
    selectOperation(operationId);
    setOperationSection("configuration");
    setView("operationDetail");
  };

  if (view === "connectorDetail") {
    return renderConnectorEditor();
  }

  if (view === "operationDetail") {
    return renderOperationEditor();
  }

  return (
    <div className="page-stack connector-registry-page">
      <header className="connector-registry-topbar">
        <div>
          <span className="eyebrow">Connector Registry</span>
          <h1>External Systems</h1>
        </div>
        <button className="primary-action" type="button" onClick={openNewConnector}>
          New Connector
        </button>
      </header>

      {notice ? <p className={notice.ok ? "notice notice-ok" : "notice notice-error"}>{notice.message}</p> : null}

      <section className="connector-group-list" aria-label="Connector groups">
        {groups.map((group) => {
          const isExpanded = expandedGroups[group.id] ?? true;
          const connector = group.connector;
          const status = connector?.status ?? "draft";
          return (
            <article className="connector-group-card" key={group.id}>
              <div className="connector-group-header">
                <button
                  className="connector-collapse-button"
                  type="button"
                  onClick={() => setExpandedGroups((current) => ({ ...current, [group.id]: !isExpanded }))}
                  aria-label={isExpanded ? "Collapse operations" : "Expand operations"}
                >
                  {isExpanded ? "v" : ">"}
                </button>
                <div className="connector-avatar">{group.isLegacy ? "L" : "C"}</div>
                <div className="connector-group-title">
                  <button
                    className="link-button connector-title-button"
                    type="button"
                    onClick={() => (connector ? openConnector(connector.id) : undefined)}
                    disabled={!connector}
                  >
                    {connector?.name ?? "Legacy Operations"}
                  </button>
                  <p>{connector?.description || "Operations that still carry their own endpoint settings."}</p>
                  <code>{connector?.baseUrl ?? "operation-level endpoint"}</code>
                </div>
                <div className="connector-group-meta">
                  <Badge tone={statusTone[status]}>{status}</Badge>
                  <span>{group.operations.length} operations</span>
                  <span>{connector?.ownerTeam ?? "Unassigned"}</span>
                </div>
                <div className="connector-group-actions">
                  {connector ? (
                    <button className="secondary-action" type="button" onClick={() => openConnector(connector.id)}>
                      Configure
                    </button>
                  ) : null}
                  <button className="primary-action" type="button" onClick={() => openNewOperation(connector?.id)}>
                    Add Operation
                  </button>
                </div>
              </div>

              {isExpanded ? (
                <div className="connector-operation-list">
                  {group.operations.map((operation) => (
                    <button className="connector-operation-row" key={operation.id} type="button" onClick={() => openOperation(operation.id)}>
                      <span className="connector-operation-main">
                        <strong>{operation.name}</strong>
                        <span>{operation.description}</span>
                      </span>
                      <span>{contractSummary(operation)}</span>
                      <code>{operation.method} {operation.path}</code>
                      <Badge tone={statusTone[operation.status]}>{operation.status}</Badge>
                    </button>
                  ))}
                  {group.operations.length === 0 ? <p className="connector-empty-state">No operations yet. Add the first API operation under this connector.</p> : null}
                </div>
              ) : null}
            </article>
          );
        })}
        {groups.length === 0 ? <p className="connector-empty-state">No connectors yet. Create one external system connection first.</p> : null}
      </section>
    </div>
  );

  function renderConnectorEditor() {
    const activeIndex = stepIndex(connectorSections, connectorSection);
    const operationsForConnector = operations.filter((operation) => operation.connectorId === selectedConnector?.id);

    return (
      <div className="tool-editor-page connector-editor-page">
        <header className="tool-editor-topbar">
          <button className="agent-editor-back" type="button" onClick={() => setView("list")} aria-label="Back to connectors">
            <span aria-hidden="true">←</span>
          </button>
          <div className="agent-editor-title">
            <h1>{connectorDraft.id ? `Edit Connector - ${connectorDraft.name || connectorDraft.id}` : "Create New Connector"}</h1>
            <div>
              <Badge tone={statusTone[selectedConnector?.status ?? connectorDraft.status]}>{selectedConnector?.status ?? connectorDraft.status}</Badge>
              <span>{connectorDraft.baseUrl || "No endpoint configured"}</span>
            </div>
          </div>
          <div className="agent-editor-actions">
            <button className="secondary-action" type="button" onClick={() => void disableSelectedConnector()} disabled={!selectedConnector}>
              Disable
            </button>
            <button className="secondary-action" type="button" onClick={() => void enableSelectedConnector()} disabled={!selectedConnector}>
              Enable
            </button>
            <button className="primary-action" type="button" onClick={() => void saveConnectorDraft()}>
              Save
            </button>
          </div>
        </header>

        {notice ? <p className={notice.ok ? "notice notice-ok tool-editor-notice" : "notice notice-error tool-editor-notice"}>{notice.message}</p> : null}

        <main className="tool-editor-workspace connector-editor-workspace">
          <ConnectorStepNav
            activeKey={connectorSection}
            onSelect={(section) => setConnectorSection(section as ConnectorSection)}
            steps={connectorSections}
          />
          <section className="tool-editor-content">{renderConnectorSection(operationsForConnector)}</section>
        </main>

        <footer className="tool-editor-footer">
          <button
            className="secondary-action"
            type="button"
            onClick={() => setConnectorSection(connectorSections[activeIndex - 1]?.key ?? connectorSection)}
            disabled={activeIndex === 0}
          >
            Previous
          </button>
          <span>{`${connectorDraft.type.toUpperCase()} · ${connectorDraft.businessDomain || "General"} · ${operationsForConnector.length} operations`}</span>
          {activeIndex === connectorSections.length - 1 ? (
            <button className="primary-action" type="button" onClick={() => void saveConnectorDraft()}>
              Save Connector
            </button>
          ) : (
            <button
              className="primary-action"
              type="button"
              onClick={() => setConnectorSection(connectorSections[activeIndex + 1]?.key ?? connectorSection)}
            >
              Next
            </button>
          )}
        </footer>
      </div>
    );
  }

  function renderOperationEditor() {
    const activeIndex = stepIndex(operationSections, operationSection);

    return (
      <div className="tool-editor-page connector-editor-page">
        <header className="tool-editor-topbar">
          <button className="agent-editor-back" type="button" onClick={() => setView("list")} aria-label="Back to connector operations">
            <span aria-hidden="true">←</span>
          </button>
          <div className="agent-editor-title">
            <h1>{draft.id ? `Edit Operation - ${draft.name || draft.id}` : "Create New Operation"}</h1>
            <div>
              <Badge tone={statusTone[selectedOperation?.status ?? draft.status]}>{selectedOperation?.status ?? draft.status}</Badge>
            </div>
          </div>
          <div className="agent-editor-actions">
            <button className="secondary-action" type="button" onClick={() => void disableSelected()} disabled={!selectedOperation}>
              Disable
            </button>
            <button className="secondary-action" type="button" onClick={() => void enableSelected()} disabled={!selectedOperation}>
              Enable
            </button>
            <button className="primary-action" type="button" onClick={() => void saveDraft()}>
              Save
            </button>
          </div>
        </header>

        {notice ? <p className={notice.ok ? "notice notice-ok tool-editor-notice" : "notice notice-error tool-editor-notice"}>{notice.message}</p> : null}

        <main className="tool-editor-workspace connector-editor-workspace">
          <ConnectorStepNav
            activeKey={operationSection}
            onSelect={(section) => setOperationSection(section as OperationSection)}
            steps={operationSections}
          />
          <section className="tool-editor-content">{renderOperationSection()}</section>
        </main>

        <footer className="tool-editor-footer">
          <button
            className="secondary-action"
            type="button"
            onClick={() => setOperationSection(operationSections[activeIndex - 1]?.key ?? operationSection)}
            disabled={activeIndex === 0}
          >
            Previous
          </button>
          <span>{`${draft.method} ${draft.path || "/"} · ${selectedDependencies.summary.totalConsumerCount} consumers`}</span>
          {activeIndex === operationSections.length - 1 ? (
            <button className="primary-action" type="button" onClick={() => void saveDraft()}>
              Save Operation
            </button>
          ) : (
            <button
              className="primary-action"
              type="button"
              onClick={() => setOperationSection(operationSections[activeIndex + 1]?.key ?? operationSection)}
            >
              Next
            </button>
          )}
        </footer>
      </div>
    );
  }

  function renderConnectorSection(operationsForConnector: ConnectorOperation[]) {
    if (connectorSection === "connection") {
      const secretRef = connectorSecretRefForDraft(connectorDraft);
      const secretEnvName = connectorSecretEnvName(connectorDraft);
      const requiresSecret = connectorAuthRequiresEnvironmentSecret(connectorDraft.authType);

      return (
        <article className="tool-editor-panel">
          <div className="tool-editor-section-title">
            <div>
              <span className="eyebrow">Step 2</span>
              <h2>Connection & Auth</h2>
              <p>Keep protocol, base URL, shared headers, and authentication here so operations only describe concrete APIs.</p>
            </div>
          </div>
          <div className="operation-form tool-editor-form">
            <div className="form-grid">
              <label>
                Connector Type
                <select value={connectorDraft.type} onChange={(event) => setConnectorDraft({ ...connectorDraft, type: event.target.value as Connector["type"] })}>
                  <option value="http">HTTP API</option>
                </select>
              </label>
              <label>
                Timeout
                <input
                  type="number"
                  min="100"
                  value={connectorDraft.timeoutMillis}
                  onChange={(event) => setConnectorDraft({ ...connectorDraft, timeoutMillis: Number(event.target.value) })}
                />
              </label>
            </div>
            <label>
              Base URL
              <input value={connectorDraft.baseUrl} onChange={(event) => setConnectorDraft({ ...connectorDraft, baseUrl: event.target.value })} />
            </label>
            <div className="form-grid">
              <label>
                Auth Type
                <select
                  value={connectorDraft.authType}
                  onChange={(event) =>
                    setConnectorDraft({ ...connectorDraft, authType: event.target.value as NonNullable<Connector["auth"]>["type"] })
                  }
                >
                  <option value="none">none</option>
                  <option value="api_key">api_key</option>
                  <option value="bearer">bearer</option>
                  <option value="basic">basic</option>
                  <option value="oauth2">oauth2</option>
                </select>
              </label>
              <div className="connector-secret-ref-card" aria-live="polite">
                <span>{requiresSecret ? "Environment Variable" : "Secret Reference"}</span>
                <code>{requiresSecret ? secretRef : "No secret required"}</code>
                <p>
                  {requiresSecret
                    ? `Configure ${secretEnvName} in the service runtime environment. The console never accepts or stores the actual API key.`
                    : "This auth type does not require an API key value in the connector console."}
                </p>
              </div>
            </div>
            <section>
              <h3>Shared Headers</h3>
              <HeaderTable headers={connectorDraft.headers} onChange={(headers) => setConnectorDraft({ ...connectorDraft, headers })} />
            </section>
          </div>
        </article>
      );
    }

    if (connectorSection === "operations") {
      return (
        <article className="tool-editor-panel">
          <div className="tool-editor-section-title">
            <div>
              <span className="eyebrow">Step 3</span>
              <h2>Operations</h2>
              <p>Operations are concrete APIs under this connector. They inherit the connector connection settings at runtime.</p>
            </div>
            <button className="primary-action" type="button" onClick={() => openNewOperation(selectedConnector?.id)} disabled={!selectedConnector}>
              Add Operation
            </button>
          </div>
          <div className="connector-operation-list connector-operation-list-bordered">
            {operationsForConnector.map((operation) => (
              <button className="connector-operation-row" key={operation.id} type="button" onClick={() => openOperation(operation.id)}>
                <span className="connector-operation-main">
                  <strong>{operation.name}</strong>
                  <span>{operation.description}</span>
                </span>
                <span>{contractSummary(operation)}</span>
                <code>{operation.method} {operation.path}</code>
                <Badge tone={statusTone[operation.status]}>{operation.status}</Badge>
              </button>
            ))}
            {!selectedConnector ? <p className="connector-empty-state">Save the connector before adding operations.</p> : null}
            {selectedConnector && operationsForConnector.length === 0 ? <p className="connector-empty-state">No operations yet.</p> : null}
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
            <p>Name the external system and assign clear ownership. Operations will be managed under this connector.</p>
          </div>
        </div>
        <section className="connector-capability-summary" aria-label="Connector summary">
          <div>
            <span>Domain</span>
            <strong>{connectorDraft.businessDomain}</strong>
          </div>
          <div>
            <span>Owner</span>
            <strong>{connectorDraft.ownerTeam}</strong>
          </div>
          <div>
            <span>Operations</span>
            <strong>{operationsForConnector.length}</strong>
          </div>
          <div>
            <span>Status</span>
            <Badge tone={statusTone[selectedConnector?.status ?? connectorDraft.status]}>{selectedConnector?.status ?? connectorDraft.status}</Badge>
          </div>
        </section>
        <div className="operation-form tool-editor-form">
          <label>
            Connector Name
            <input value={connectorDraft.name} onChange={(event) => setConnectorDraft({ ...connectorDraft, name: event.target.value })} />
          </label>
          <label>
            Description
            <textarea value={connectorDraft.description} onChange={(event) => setConnectorDraft({ ...connectorDraft, description: event.target.value })} />
          </label>
          <div className="form-grid">
            <label>
              Business Domain
              <input value={connectorDraft.businessDomain} onChange={(event) => setConnectorDraft({ ...connectorDraft, businessDomain: event.target.value })} />
            </label>
            <label>
              Owner Team
              <input value={connectorDraft.ownerTeam} onChange={(event) => setConnectorDraft({ ...connectorDraft, ownerTeam: event.target.value })} />
            </label>
          </div>
        </div>
      </article>
    );
  }

  function renderOperationSection() {
    if (operationSection === "usage") {
      return (
        <article className="tool-editor-panel">
          <div className="tool-editor-section-title">
            <div>
              <span className="eyebrow">Step 3</span>
              <h2>Usage and Impact</h2>
              <p>Review consumers before changing or disabling this operation.</p>
            </div>
            <Badge tone={selectedDependencies.summary.blockingToolCount > 0 ? "amber" : "green"}>
              {`${selectedDependencies.summary.totalConsumerCount} consumers`}
            </Badge>
          </div>
          <div className="dependency-grid">
            <span>Tools</span>
            <strong>{selectedDependencies.summary.directToolCount}</strong>
            <span>Skills</span>
            <strong>{selectedDependencies.summary.indirectSkillCount}</strong>
            <span>Agents</span>
            <strong>{selectedDependencies.summary.indirectAgentCount}</strong>
          </div>
          <div className="dependency-list">
            {selectedDependencies.directTools.map((tool) => (
              <div key={tool.id}>
                <strong>{tool.name}</strong>
                <code>{tool.id}</code>
              </div>
            ))}
            {selectedDependencies.directTools.length === 0 ? <p>No direct Tool consumers.</p> : null}
          </div>
        </article>
      );
    }

    if (operationSection === "test") {
      return (
        <article className="tool-editor-panel">
          <div className="tool-editor-section-title">
            <div>
              <span className="eyebrow">Step 2</span>
              <h2>Test Console</h2>
              <p>Invoke the normalized operation contract before binding it to Tools.</p>
            </div>
            <button className="primary-action" type="button" onClick={() => void testSelectedOperation()} disabled={!selectedOperation}>
              Run Test
            </button>
          </div>
          <textarea className="test-args-input" value={testArgsJson} onChange={(event) => setTestArgsJson(event.target.value)} />
          {testResult ? (
            <pre className="json-result">
              {JSON.stringify(
                {
                  success: testResult.success,
                  requestId: testResult.requestId,
                  errorCode: testResult.errorCode,
                  data: testResult.data
                },
                null,
                2
              )}
            </pre>
          ) : null}
        </article>
      );
    }

    return (
      <article className="tool-editor-panel">
        <div className="tool-editor-section-title">
          <div>
            <span className="eyebrow">Step 1</span>
            <h2>Operation Configuration</h2>
            <p>Define the API operation that belongs to this connector. Connection, authentication, and ownership are inherited from the connector.</p>
          </div>
        </div>
        <div className="operation-form tool-editor-form">
          <label>
            Operation Name
            <input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
          </label>
          <label>
            Business Description
            <textarea value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} />
          </label>
          <div className="form-grid">
            <label>
              Method
              <select
                value={draft.method}
                onChange={(event) => setDraft({ ...draft, method: event.target.value as ConnectorOperation["method"] })}
              >
                <option>GET</option>
                <option>POST</option>
                <option>PUT</option>
                <option>PATCH</option>
                <option>DELETE</option>
              </select>
            </label>
            <label>
              Timeout
              <input
                type="number"
                min="100"
                value={draft.timeoutMillis}
                onChange={(event) => setDraft({ ...draft, timeoutMillis: Number(event.target.value) })}
              />
            </label>
          </div>
          <label>
            Operation Path
            <input value={draft.path} onChange={(event) => setDraft({ ...draft, path: event.target.value })} />
          </label>
          <div className="schema-section-stack">
            <section>
              <h3>Input Fields</h3>
              <SchemaFieldTable fields={draft.inputFields} onChange={(inputFields) => setDraft({ ...draft, inputFields })} />
            </section>
            <section>
              <h3>Output Fields</h3>
              <SchemaFieldTable fields={draft.outputFields} onChange={(outputFields) => setDraft({ ...draft, outputFields })} />
            </section>
          </div>
        </div>
      </article>
    );
  }
}

function ConnectorStepNav({
  activeKey,
  onSelect,
  steps
}: {
  activeKey: string;
  onSelect: (section: string) => void;
  steps: Array<{ key: string; label: string }>;
}) {
  const activeIndex = stepIndex(steps, activeKey);

  return (
    <nav className="tool-step-nav connector-step-nav" aria-label="Connector configuration steps">
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

function buildConnectorGroups(connectors: Connector[], operations: ConnectorOperation[]): ConnectorGroup[] {
  const operationsByConnector = new Map<string, ConnectorOperation[]>();
  const legacyOperations: ConnectorOperation[] = [];
  for (const operation of operations) {
    if (!operation.connectorId) {
      legacyOperations.push(operation);
      continue;
    }
    const current = operationsByConnector.get(operation.connectorId) ?? [];
    current.push(operation);
    operationsByConnector.set(operation.connectorId, current);
  }

  const connectorIDs = new Set(connectors.map((connector) => connector.id));
  for (const [connectorID, connectorOperations] of operationsByConnector.entries()) {
    if (!connectorIDs.has(connectorID)) {
      legacyOperations.push(...connectorOperations);
    }
  }

  const groups: ConnectorGroup[] = connectors.map((connector) => ({
    id: connector.id,
    connector,
    operations: operationsByConnector.get(connector.id) ?? []
  }));
  if (legacyOperations.length > 0) {
    groups.push({ id: "legacy", operations: legacyOperations, isLegacy: true });
  }
  return groups;
}

function stepIndex(steps: Array<{ key: string }>, key: string): number {
  return Math.max(
    0,
    steps.findIndex((step) => step.key === key)
  );
}
