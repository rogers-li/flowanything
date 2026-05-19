import { getJson, sendJson } from "./http";
import type {
  AgentGraphRunRequest,
  AgentRunRequest,
  AgentRunResponse,
  BundleEntrypoint,
  BundleInspection,
  BundleLifecycle,
  BundleSpec,
  BundleSummary,
  ConnectorOperationConfig,
  CreateDebugSessionRequest,
  CreateDebugSessionResponse,
  DebugSessionSnapshot,
  JsonObject,
  PreviewBundleResponse,
  PublishAndReloadResponse,
  PublishResult,
  ResourceDocument,
  ResourceKind,
  RunRecord,
  RuntimeSnapshot,
  TraceResponse,
  ValidationResult,
  WorkflowRunRequest,
  WorkflowRunResponse
} from "./configTypes";

type ListResponse<T> = {
  items: T[];
};

type BundleResponse = {
  bundle: BundleSpec;
};

type ConnectorOperationListResponse = {
  items: ConnectorOperationConfig[];
};

type ConnectorOperationResponse = {
  operation: ConnectorOperationConfig;
};

type DebugSessionResponse = {
  session: DebugSessionSnapshot;
};

type RunHistoryResponse = {
  run: RunRecord;
};

export const platformRuntimeBaseUrl = import.meta.env.VITE_AI_PLATFORM_RUNTIME_URL ?? "/ai-platform-runtime";

export const bundleApi = {
  listDrafts: () => getJson<ListResponse<BundleSummary>>(`${platformRuntimeBaseUrl}/v1/bundles`),
  listPreviews: () => getJson<ListResponse<BundleSummary>>(`${platformRuntimeBaseUrl}/v1/previews`),
  listReleases: () => getJson<ListResponse<BundleSummary>>(`${platformRuntimeBaseUrl}/v1/releases`),
  getBundle: (bundleId: string) => getJson<BundleResponse>(`${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}`),
  saveBundle: (bundle: BundleSpec) => sendJson<BundleResponse>(`${platformRuntimeBaseUrl}/v1/bundles`, "POST", bundle),
  updateBundle: (bundle: BundleSpec) =>
    sendJson<BundleResponse>(`${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundle.id)}`, "PUT", bundle),
  deleteBundle: (bundleId: string) =>
    sendJson<{ deleted: boolean }>(`${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}`, "DELETE"),
  validateBundle: (bundle: BundleSpec) => sendJson<ValidationResult>(`${platformRuntimeBaseUrl}/v1/bundles/validate`, "POST", bundle),
  validateStoredBundle: (bundleId: string) =>
    sendJson<ValidationResult>(`${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}/validate`, "POST"),
  inspectBundle: (bundleId: string, lifecycle: BundleLifecycle = "draft") => {
    const params = new URLSearchParams({ lifecycle });
    return getJson<BundleInspection>(`${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}/inspect?${params.toString()}`);
  },
  buildPreview: (bundleId: string, entrypoint: BundleEntrypoint) =>
    sendJson<PreviewBundleResponse>(`${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}/preview`, "POST", { entrypoint }),
  publish: (bundleId: string) =>
    sendJson<PublishResult>(`${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}/publish`, "POST"),
  publishAndReload: (bundleId: string) =>
    sendJson<PublishAndReloadResponse>(`${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}/publish-and-reload`, "POST")
};

export const resourceApi = {
  listResources: (bundleId: string, filter: { kind?: ResourceKind; query?: string } = {}) => {
    const params = new URLSearchParams();
    if (filter.kind) params.set("kind", filter.kind);
    if (filter.query) params.set("q", filter.query);
    const query = params.toString();
    const suffix = query ? `?${query}` : "";
    return getJson<ListResponse<ResourceDocument>>(`${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}/resources${suffix}`);
  },
  listResourcesByKind: <TResource>(bundleId: string, kind: ResourceKind, query = "") => {
    const params = new URLSearchParams();
    if (query) params.set("q", query);
    const suffix = params.toString() ? `?${params.toString()}` : "";
    return getJson<ListResponse<ResourceDocument<TResource>>>(
      `${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}/resources/${encodeURIComponent(kind)}${suffix}`
    );
  },
  getResource: <TResource>(bundleId: string, kind: ResourceKind, resourceId: string) =>
    getJson<ResourceDocument<TResource>>(
      `${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}/resources/${encodeURIComponent(kind)}/${encodeURIComponent(resourceId)}`
    ),
  upsertResource: <TResource extends { id: string }>(bundleId: string, kind: ResourceKind, resource: TResource) =>
    sendJson<BundleResponse>(
      `${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}/resources/${encodeURIComponent(kind)}/${encodeURIComponent(resource.id)}`,
      "PUT",
      resource
    ),
  deleteResource: (bundleId: string, kind: ResourceKind, resourceId: string) =>
    sendJson<BundleResponse>(
      `${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}/resources/${encodeURIComponent(kind)}/${encodeURIComponent(resourceId)}`,
      "DELETE"
    ),
  listConnectorOperations: (bundleId: string, connectorId: string, query = "") => {
    const params = new URLSearchParams();
    if (query) params.set("q", query);
    const suffix = params.toString() ? `?${params.toString()}` : "";
    return getJson<ConnectorOperationListResponse>(
      `${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}/connectors/${encodeURIComponent(connectorId)}/operations${suffix}`
    );
  },
  getConnectorOperation: (bundleId: string, connectorId: string, operationId: string) =>
    getJson<ConnectorOperationResponse>(
      `${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}/connectors/${encodeURIComponent(connectorId)}/operations/${encodeURIComponent(operationId)}`
    ),
  upsertConnectorOperation: (bundleId: string, connectorId: string, operation: ConnectorOperationConfig) =>
    sendJson<BundleResponse>(
      `${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}/connectors/${encodeURIComponent(connectorId)}/operations/${encodeURIComponent(operation.id)}`,
      "PUT",
      operation
    ),
  deleteConnectorOperation: (bundleId: string, connectorId: string, operationId: string) =>
    sendJson<BundleResponse>(
      `${platformRuntimeBaseUrl}/v1/bundles/${encodeURIComponent(bundleId)}/connectors/${encodeURIComponent(connectorId)}/operations/${encodeURIComponent(operationId)}`,
      "DELETE"
    )
};

export const runtimeApiV2 = {
  catalog: () => getJson<BundleSpec>(`${platformRuntimeBaseUrl}/v1/catalog`),
  activeBundle: () => getJson<RuntimeSnapshot>(`${platformRuntimeBaseUrl}/v1/runtime/active-bundle`),
  reload: (bundleId: string) => sendJson<RuntimeSnapshot>(`${platformRuntimeBaseUrl}/v1/runtime/reload`, "POST", { bundle_id: bundleId }),
  runAgent: (request: AgentRunRequest) => sendJson<AgentRunResponse>(`${platformRuntimeBaseUrl}/v1/agents/run`, "POST", request),
  runWorkflow: (request: WorkflowRunRequest) => sendJson<WorkflowRunResponse>(`${platformRuntimeBaseUrl}/v1/workflows/run`, "POST", request),
  runAgentGraph: (request: AgentGraphRunRequest) => sendJson<WorkflowRunResponse>(`${platformRuntimeBaseUrl}/v1/agent-graphs/run`, "POST", request),
  invokeTool: (request: { tool_id: string; input?: JsonObject; metadata?: JsonObject; trace_id?: string }) =>
    sendJson<{ result: unknown }>(`${platformRuntimeBaseUrl}/v1/tools/invoke`, "POST", request),
  invokeConnector: (request: { operation_id: string; input?: JsonObject; metadata?: JsonObject; trace_id?: string }) =>
    sendJson<{ result: unknown }>(`${platformRuntimeBaseUrl}/v1/connectors/invoke`, "POST", request),
  getTrace: (traceId: string) => getJson<TraceResponse>(`${platformRuntimeBaseUrl}/v1/traces/${encodeURIComponent(traceId)}`)
};

export const debugSessionApi = {
  listSessions: () => getJson<ListResponse<DebugSessionSnapshot>>(`${platformRuntimeBaseUrl}/v1/debug-sessions`),
  createSession: (request: CreateDebugSessionRequest) =>
    sendJson<CreateDebugSessionResponse>(`${platformRuntimeBaseUrl}/v1/debug-sessions`, "POST", request),
  getSession: (sessionId: string) =>
    getJson<DebugSessionResponse>(`${platformRuntimeBaseUrl}/v1/debug-sessions/${encodeURIComponent(sessionId)}`),
  deleteSession: (sessionId: string) =>
    sendJson<{ deleted: boolean }>(`${platformRuntimeBaseUrl}/v1/debug-sessions/${encodeURIComponent(sessionId)}`, "DELETE"),
  runAgent: (sessionId: string, request: AgentRunRequest) => {
    const { trace_id: _traceId, ...debugRequest } = request;
    return sendJson<AgentRunResponse>(`${platformRuntimeBaseUrl}/v1/debug-sessions/${encodeURIComponent(sessionId)}/agents/run`, "POST", debugRequest);
  },
  runWorkflow: (sessionId: string, request: WorkflowRunRequest) =>
    sendJson<WorkflowRunResponse>(`${platformRuntimeBaseUrl}/v1/debug-sessions/${encodeURIComponent(sessionId)}/workflows/run`, "POST", request),
  runAgentGraph: (sessionId: string, request: AgentGraphRunRequest) =>
    sendJson<WorkflowRunResponse>(`${platformRuntimeBaseUrl}/v1/debug-sessions/${encodeURIComponent(sessionId)}/agent-graphs/run`, "POST", request)
};

export const runHistoryApi = {
  list: () => getJson<ListResponse<RunRecord>>(`${platformRuntimeBaseUrl}/v1/run-history`),
  get: (runId: string) => getJson<RunHistoryResponse>(`${platformRuntimeBaseUrl}/v1/run-history/${encodeURIComponent(runId)}`),
  replay: (runId: string) =>
    sendJson<RunHistoryResponse>(`${platformRuntimeBaseUrl}/v1/run-history/${encodeURIComponent(runId)}/replay`, "POST")
};
