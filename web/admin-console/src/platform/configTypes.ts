export type JsonObject = Record<string, unknown>;

export type BundleLifecycle = "draft" | "preview" | "release";

export type ResourceKind =
  | "agent"
  | "skill"
  | "tool"
  | "workflow"
  | "connector"
  | "connector_operation"
  | "model"
  | "knowledge"
  | "policy";

export type RuntimeTarget = "server" | "mobile" | "ios" | "android" | "desktop" | "edge" | "test";

export type BundleEntrypoint = {
  kind: ResourceKind;
  id: string;
};

export type ResourceRef = {
  kind?: ResourceKind;
  id: string;
  alias?: string;
  optional?: boolean;
};

export type ResourceBinding = {
  ref: ResourceRef;
  alias?: string;
  disabled?: boolean;
  config?: JsonObject;
};

export type OwnerSpec = {
  team?: string;
  email?: string;
};

export type ResourceMeta = {
  id: string;
  name: string;
  description?: string;
  version?: string;
  disabled?: boolean;
  labels?: string[];
  annotations?: Record<string, string>;
  owner?: OwnerSpec;
  metadata?: JsonObject;
};

export type SchemaField = {
  name: string;
  path?: string;
  type?: string;
  description?: string;
  required?: boolean;
  repeated?: boolean;
  children?: SchemaField[];
  enum_values?: string[];
  default_value?: unknown;
  metadata?: JsonObject;
};

export type PromptConfig = {
  system?: string;
  developer?: string;
  templates?: Record<string, string>;
  variables?: SchemaField[];
  metadata?: JsonObject;
};

export type ReasoningConfig = {
  mode?: string;
  config?: JsonObject;
};

export type RuntimeRequirementSpec = {
  network?: boolean;
  file_read?: boolean;
  file_write?: boolean;
  location?: boolean;
  camera?: boolean;
  microphone?: boolean;
  server_proxy_allowed?: boolean;
  secrets?: SecretRequirement[];
  capabilities?: CapabilityRequirement[];
};

export type SecretRequirement = {
  name: string;
  description?: string;
  required?: boolean;
};

export type CapabilityRequirement = {
  name: string;
  version?: string;
  required?: boolean;
  config?: JsonObject;
};

export type RuntimeTargetSpec = {
  targets?: RuntimeTarget[];
  min_runtime_version?: string;
  required_capabilities?: CapabilityRequirement[];
  config?: JsonObject;
};

export type PermissionSpec = {
  network_domains?: string[];
  secret_refs?: string[];
  file_scopes?: string[];
};

export type SignatureSpec = {
  algorithm?: string;
  key_id?: string;
  value?: string;
};

export type ExecutionPolicy = {
  timeout?: string;
  require_review?: boolean;
  retry_policy?: RetryPolicy;
};

export type RetryPolicy = {
  max_attempts?: number;
  backoff?: string;
};

export type AgentConfig = ResourceMeta & {
  prompt?: PromptConfig;
  reasoning?: ReasoningConfig;
  model_ref?: ResourceRef;
  skills?: ResourceBinding[];
  tools?: ResourceBinding[];
  workflows?: ResourceBinding[];
  knowledge?: ResourceBinding[];
  policies?: ResourceRef[];
  output_schema?: SchemaField[];
  runtime?: RuntimeRequirementSpec;
};

export type SkillConfig = ResourceMeta & {
  prompt?: PromptConfig;
  input_schema?: SchemaField[];
  output_schema?: SchemaField[];
  tools?: ResourceBinding[];
  knowledge?: ResourceBinding[];
  policies?: ResourceRef[];
  runtime?: RuntimeRequirementSpec;
};

export type ToolType = "native" | "connector" | "workflow" | "mcp" | "script" | "remote_capability";

export type ToolImplementationSpec = {
  kind: string;
  ref?: ResourceRef;
  config?: JsonObject;
};

export type ToolConfig = ResourceMeta & {
  type: ToolType;
  input_schema?: SchemaField[];
  output_schema?: SchemaField[];
  implementation: ToolImplementationSpec;
  policy?: ExecutionPolicy;
  runtime?: RuntimeRequirementSpec;
};

export type FlowSpec = {
  id: string;
  name?: string;
  version?: string;
  context_schema?: FlowContextSchema;
  nodes?: FlowNodeSpec[];
  edges?: FlowEdgeSpec[];
  policies?: JsonObject;
};

export type FlowContextSchema = {
  flow_input?: SchemaField[];
  flow_output?: SchemaField[];
  variables?: SchemaField[];
  node_context?: Record<string, SchemaField[]>;
};

export type FlowValueSource = {
  type: "context" | "const" | "node_output";
  path?: string;
  value?: unknown;
};

export type FlowFieldBinding = {
  field: string;
  source: FlowValueSource;
  enabled?: boolean;
  description?: string;
};

export type FlowContextWrite = {
  target: string;
  source: FlowValueSource;
  enabled?: boolean;
  description?: string;
};

export type FlowRetryPolicy = {
  max_attempts?: number;
  backoff?: number;
};

export type FlowNodeSpec = {
  id: string;
  type: string;
  name?: string;
  config?: JsonObject;
  input_mappings?: FlowFieldBinding[];
  output_writes?: FlowContextWrite[];
  timeout?: number;
  retry_policy?: FlowRetryPolicy;
};

export type FlowEdgeSpec = {
  from: string;
  to: string;
};

export type WorkflowConfig = ResourceMeta & {
  spec: FlowSpec;
  ui?: JsonObject;
  publish?: PublishSpec;
  runtime?: RuntimeRequirementSpec;
};

export type PublishSpec = {
  status?: string;
  revision?: number;
  snapshot_id?: string;
  snapshot_hash?: string;
};

export type ConnectorConfig = ResourceMeta & {
  protocol?: ConnectorProtocolSpec;
  auth?: ConnectorAuthSpec;
  operations?: ConnectorOperationConfig[];
  runtime?: RuntimeRequirementSpec;
};

export type ConnectorProtocolSpec = {
  kind: string;
  base_url?: string;
  config?: JsonObject;
};

export type ConnectorAuthSpec = {
  type?: string;
  secret_ref?: string;
  config?: JsonObject;
};

export type ConnectorOperationConfig = ResourceMeta & {
  input_schema?: SchemaField[];
  output_schema?: SchemaField[];
  request?: ConnectorOperationRequest;
  response?: ConnectorOperationResponse;
  policy?: ExecutionPolicy;
};

export type ConnectorOperationRequest = {
  method?: string;
  path?: string;
  path_params?: Record<string, string>;
  headers?: Record<string, string>;
  query?: Record<string, string>;
  query_params?: Record<string, string>;
  body_field?: string;
  config?: JsonObject;
};

export type ConnectorOperationResponse = {
  success_status_codes?: number[];
  config?: JsonObject;
};

export type ModelConfig = ResourceMeta & {
  provider?: string;
  model?: string;
  endpoint_ref?: string;
  default_parameters?: JsonObject;
  runtime?: RuntimeRequirementSpec;
  policy?: ExecutionPolicy;
};

export type KnowledgeConfig = ResourceMeta & {
  type?: string;
  source?: KnowledgeSourceSpec;
  embedding_model_ref?: ResourceRef;
  chunking?: JsonObject;
  index?: JsonObject;
  runtime?: RuntimeRequirementSpec;
};

export type KnowledgeSourceSpec = {
  kind: string;
  uri?: string;
  config?: JsonObject;
};

export type PolicyConfig = ResourceMeta & {
  scope?: string;
  rules?: JsonObject;
};

export type ResourceCollection = {
  agents?: AgentConfig[];
  skills?: SkillConfig[];
  tools?: ToolConfig[];
  workflows?: WorkflowConfig[];
  connectors?: ConnectorConfig[];
  models?: ModelConfig[];
  knowledge_bases?: KnowledgeConfig[];
  policies?: PolicyConfig[];
};

export type BundleSpec = {
  schema_version: "v1";
  kind?: "flow-anything.bundle";
  id: string;
  name: string;
  version: string;
  description?: string;
  runtime?: RuntimeTargetSpec;
  dependencies?: ResourceRef[];
  permissions?: PermissionSpec;
  signature?: SignatureSpec;
  resources?: ResourceCollection;
  metadata?: JsonObject;
};

export type BundleSummary = {
  id: string;
  name: string;
  version: string;
  lifecycle?: BundleLifecycle;
  source_bundle_id?: string;
  content_hash?: string;
  updated_at?: string;
};

export type ResourceDocument<TResource = unknown> = {
  kind: ResourceKind;
  id: string;
  parent_id?: string;
  resource: TResource;
};

export type ResourceDescriptor = {
  kind: ResourceKind;
  id: string;
  name?: string;
  description?: string;
  disabled?: boolean;
  labels?: string[];
  owner?: OwnerSpec;
  metadata?: JsonObject;
};

export type DependencyEdge = {
  from: ResourceRef;
  to: ResourceRef;
  optional?: boolean;
};

export type Diagnostic = {
  severity: "error" | "warning";
  path?: string;
  message: string;
};

export type ResourceCounts = Partial<Record<ResourceKind | "knowledge_bases", number>>;

export type BundleSnapshotInfo = {
  bundle_id: string;
  source_bundle_id?: string;
  version?: string;
  lifecycle?: BundleLifecycle;
  content_hash?: string;
  created_at?: string;
  entrypoint?: BundleEntrypoint;
};

export type BundleInspection = {
  bundle: BundleSpec;
  snapshot: BundleSnapshotInfo;
  counts: ResourceCounts;
  resources?: ResourceDescriptor[];
  dependencies?: DependencyEdge[];
  diagnostics?: Diagnostic[];
};

export type ValidationResult = {
  valid: boolean;
  diagnostics?: Diagnostic[];
  error?: string;
};

export type PreviewBundleResponse = {
  bundle: BundleSpec;
  preview: BundleSnapshotInfo;
};

export type PublishResult = {
  bundle_id: string;
  source_bundle_id?: string;
  version?: string;
  lifecycle?: BundleLifecycle;
  content_hash?: string;
};

export type PublishAndReloadResponse = {
  publish: PublishResult;
  runtime: RuntimeSnapshot;
};

export type RuntimeSnapshot = {
  bundle_id: string;
  source_bundle_id?: string;
  version?: string;
  lifecycle?: BundleLifecycle;
  content_hash?: string;
};

export type TraceContext = {
  trace_id?: string;
  span_id?: string;
  parent_span_id?: string;
  baggage?: JsonObject;
};

export type AgentRunRequest = {
  agent_id: string;
  user_message: string;
  conversation?: Array<{ role: string; content: string }>;
  context?: JsonObject;
  trace_id?: string;
  trace_context?: TraceContext;
};

export type AgentRunResponse = {
  result: {
    text: string;
    output?: JsonObject;
    actions?: unknown[];
    raw?: unknown;
  };
};

export type WorkflowRunRequest = {
  workflow_id: string;
  input?: JsonObject;
  trace_context?: TraceContext;
};

export type AgentGraphRunRequest = {
  agent_flow_id: string;
  input?: JsonObject;
  trace_context?: TraceContext;
};

export type WorkflowRunResponse = {
  instance_id: string;
  status: string;
  output?: JsonObject;
};

export type DebugSessionSnapshot = {
  id: string;
  bundle_id: string;
  source_bundle_id?: string;
  lifecycle?: BundleLifecycle;
  content_hash?: string;
  version?: string;
  entrypoint?: BundleEntrypoint;
  created_at?: string;
  updated_at?: string;
};

export type CreateDebugSessionRequest = {
  bundle_id: string;
  entrypoint: BundleEntrypoint;
};

export type CreateDebugSessionResponse = {
  session: DebugSessionSnapshot;
  preview: BundleSnapshotInfo;
};

export type RunRecord = {
  id: string;
  type: "agent" | "workflow";
  status: "succeeded" | "failed";
  session_id?: string;
  trace_id?: string;
  bundle_id?: string;
  source_bundle_id?: string;
  bundle_version?: string;
  bundle_lifecycle?: BundleLifecycle | string;
  content_hash?: string;
  entrypoint?: BundleEntrypoint;
  agent_request?: {
    agent_id?: string;
    user_message?: string;
    trace_id?: string;
  };
  workflow_request?: {
    workflow_id?: string;
    input?: JsonObject;
  };
  result?: unknown;
  error?: string;
  started_at?: string;
  finished_at?: string;
};

export type TraceResponse = {
  trace: {
    trace_id: string;
    spans: TraceSpan[];
  };
  tree?: TraceTreeNode[];
};

export type TraceTreeNode = {
  span: TraceSpan;
  children?: TraceTreeNode[];
};

export type TraceSpan = {
  trace_id: string;
  span_id: string;
  parent_span_id?: string;
  name: string;
  kind: string;
  status: string;
  started_at?: string;
  finished_at?: string;
  attributes?: JsonObject;
  events?: TraceSpanEvent[];
  input?: JsonObject;
  output?: JsonObject;
  error?: string;
};

export type TraceSpanEvent = {
  name: string;
  attributes?: JsonObject;
  timestamp?: string;
};
