package config

import (
	"flow-anything/core/flowengine"
	"flow-anything/core/schema"
)

const (
	SchemaVersionV1 = "v1"
	BundleKind      = "flow-anything.bundle"
)

type BundleLifecycle string

const (
	BundleLifecycleDraft   BundleLifecycle = "draft"
	BundleLifecyclePreview BundleLifecycle = "preview"
	BundleLifecycleRelease BundleLifecycle = "release"
)

const (
	BundleMetadataLifecycle       = "lifecycle"
	BundleMetadataSourceBundleID  = "source_bundle_id"
	BundleMetadataContentHash     = "content_hash"
	BundleMetadataCreatedAt       = "created_at"
	BundleMetadataEntrypoint      = "entrypoint"
	BundleMetadataEntrypoints     = "entrypoints"
	BundleMetadataDependencyGraph = "dependency_graph"
)

type ResourceKind string

const (
	ResourceAgent              ResourceKind = "agent"
	ResourceSkill              ResourceKind = "skill"
	ResourceTool               ResourceKind = "tool"
	ResourceWorkflow           ResourceKind = "workflow"
	ResourceConnector          ResourceKind = "connector"
	ResourceConnectorOperation ResourceKind = "connector_operation"
	ResourceModel              ResourceKind = "model"
	ResourceKnowledge          ResourceKind = "knowledge"
	ResourcePolicy             ResourceKind = "policy"
)

type RuntimeTarget string

const (
	RuntimeServer  RuntimeTarget = "server"
	RuntimeMobile  RuntimeTarget = "mobile"
	RuntimeIOS     RuntimeTarget = "ios"
	RuntimeAndroid RuntimeTarget = "android"
	RuntimeDesktop RuntimeTarget = "desktop"
	RuntimeEdge    RuntimeTarget = "edge"
	RuntimeTest    RuntimeTarget = "test"
)

type BundleSpec struct {
	SchemaVersion string             `json:"schema_version"`
	Kind          string             `json:"kind"`
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Version       string             `json:"version"`
	Description   string             `json:"description"`
	Runtime       RuntimeTargetSpec  `json:"runtime"`
	Dependencies  []ResourceRef      `json:"dependencies"`
	Permissions   PermissionSpec     `json:"permissions"`
	Signature     SignatureSpec      `json:"signature"`
	Resources     ResourceCollection `json:"resources"`
	Metadata      map[string]any     `json:"metadata"`
}

type ResourceCollection struct {
	Agents         []AgentConfig     `json:"agents"`
	Skills         []SkillConfig     `json:"skills"`
	Tools          []ToolConfig      `json:"tools"`
	Workflows      []WorkflowConfig  `json:"workflows"`
	Connectors     []ConnectorConfig `json:"connectors"`
	Models         []ModelConfig     `json:"models"`
	KnowledgeBases []KnowledgeConfig `json:"knowledge_bases"`
	Policies       []PolicyConfig    `json:"policies"`
}

type ResourceMeta struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Disabled    bool              `json:"disabled"`
	Labels      []string          `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	Owner       OwnerSpec         `json:"owner"`
	Metadata    map[string]any    `json:"metadata"`
}

type OwnerSpec struct {
	Team  string `json:"team"`
	Email string `json:"email"`
}

type ResourceRef struct {
	Kind     ResourceKind `json:"kind"`
	ID       string       `json:"id"`
	Alias    string       `json:"alias"`
	Optional bool         `json:"optional"`
}

type BundleEntrypoint struct {
	Kind ResourceKind `json:"kind"`
	ID   string       `json:"id"`
}

type ResourceBinding struct {
	Ref      ResourceRef    `json:"ref"`
	Alias    string         `json:"alias"`
	Disabled bool           `json:"disabled"`
	Config   map[string]any `json:"config"`
}

type RuntimeTargetSpec struct {
	Targets              []RuntimeTarget         `json:"targets"`
	MinRuntimeVersion    string                  `json:"min_runtime_version"`
	RequiredCapabilities []CapabilityRequirement `json:"required_capabilities"`
	Config               map[string]any          `json:"config"`
}

type RuntimeRequirementSpec struct {
	Network            bool                    `json:"network"`
	FileRead           bool                    `json:"file_read"`
	FileWrite          bool                    `json:"file_write"`
	Location           bool                    `json:"location"`
	Camera             bool                    `json:"camera"`
	Microphone         bool                    `json:"microphone"`
	ServerProxyAllowed bool                    `json:"server_proxy_allowed"`
	Secrets            []SecretRequirement     `json:"secrets"`
	Capabilities       []CapabilityRequirement `json:"capabilities"`
}

type SecretRequirement struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type CapabilityRequirement struct {
	Name     string         `json:"name"`
	Version  string         `json:"version"`
	Required bool           `json:"required"`
	Config   map[string]any `json:"config"`
}

type PermissionSpec struct {
	NetworkDomains []string `json:"network_domains"`
	SecretRefs     []string `json:"secret_refs"`
	FileScopes     []string `json:"file_scopes"`
}

type SignatureSpec struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"key_id"`
	Value     string `json:"value"`
}

type SchemaField = schema.Field

type PromptConfig struct {
	System    string            `json:"system"`
	Developer string            `json:"developer"`
	Templates map[string]string `json:"templates"`
	Variables []SchemaField     `json:"variables"`
	Metadata  map[string]any    `json:"metadata"`
}

type ReasoningConfig struct {
	Mode   string         `json:"mode"`
	Config map[string]any `json:"config"`
}

type AgentConfig struct {
	ResourceMeta
	Prompt       PromptConfig           `json:"prompt"`
	Reasoning    ReasoningConfig        `json:"reasoning"`
	ModelRef     ResourceRef            `json:"model_ref"`
	Skills       []ResourceBinding      `json:"skills"`
	Tools        []ResourceBinding      `json:"tools"`
	Workflows    []ResourceBinding      `json:"workflows"`
	Knowledge    []ResourceBinding      `json:"knowledge"`
	Policies     []ResourceRef          `json:"policies"`
	OutputSchema []SchemaField          `json:"output_schema"`
	Runtime      RuntimeRequirementSpec `json:"runtime"`
}

type SkillConfig struct {
	ResourceMeta
	Prompt       PromptConfig           `json:"prompt"`
	InputSchema  []SchemaField          `json:"input_schema"`
	OutputSchema []SchemaField          `json:"output_schema"`
	Tools        []ResourceBinding      `json:"tools"`
	Knowledge    []ResourceBinding      `json:"knowledge"`
	Policies     []ResourceRef          `json:"policies"`
	Runtime      RuntimeRequirementSpec `json:"runtime"`
}

type ToolType string

const (
	ToolTypeNative    ToolType = "native"
	ToolTypeConnector ToolType = "connector"
	ToolTypeWorkflow  ToolType = "workflow"
	ToolTypeMCP       ToolType = "mcp"
	ToolTypeScript    ToolType = "script"
	ToolTypeRemote    ToolType = "remote_capability"
)

type ToolConfig struct {
	ResourceMeta
	Type           ToolType               `json:"type"`
	InputSchema    []SchemaField          `json:"input_schema"`
	OutputSchema   []SchemaField          `json:"output_schema"`
	Implementation ToolImplementationSpec `json:"implementation"`
	Policy         ExecutionPolicy        `json:"policy"`
	Runtime        RuntimeRequirementSpec `json:"runtime"`
}

type ToolImplementationSpec struct {
	Kind   string         `json:"kind"`
	Ref    ResourceRef    `json:"ref"`
	Config map[string]any `json:"config"`
}

type WorkflowConfig struct {
	ResourceMeta
	Spec    flowengine.FlowSpec    `json:"spec"`
	UI      map[string]any         `json:"ui"`
	Publish PublishSpec            `json:"publish"`
	Runtime RuntimeRequirementSpec `json:"runtime"`
}

type PublishSpec struct {
	Status       string `json:"status"`
	Revision     int64  `json:"revision"`
	SnapshotID   string `json:"snapshot_id"`
	SnapshotHash string `json:"snapshot_hash"`
}

type ConnectorConfig struct {
	ResourceMeta
	Protocol   ConnectorProtocolSpec      `json:"protocol"`
	Auth       ConnectorAuthSpec          `json:"auth"`
	Operations []ConnectorOperationConfig `json:"operations"`
	Runtime    RuntimeRequirementSpec     `json:"runtime"`
}

type ConnectorProtocolSpec struct {
	Kind    string         `json:"kind"`
	BaseURL string         `json:"base_url"`
	Config  map[string]any `json:"config"`
}

type ConnectorAuthSpec struct {
	Type      string         `json:"type"`
	SecretRef string         `json:"secret_ref"`
	Config    map[string]any `json:"config"`
}

type ConnectorOperationConfig struct {
	ResourceMeta
	InputSchema  []SchemaField              `json:"input_schema"`
	OutputSchema []SchemaField              `json:"output_schema"`
	Request      ConnectorOperationRequest  `json:"request"`
	Response     ConnectorOperationResponse `json:"response"`
	Policy       ExecutionPolicy            `json:"policy"`
}

type ConnectorOperationRequest struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	PathParams  map[string]string `json:"path_params"`
	Headers     map[string]string `json:"headers"`
	Query       map[string]string `json:"query"`
	QueryParams map[string]string `json:"query_params"`
	BodyField   string            `json:"body_field"`
	Config      map[string]any    `json:"config"`
}

type ConnectorOperationResponse struct {
	SuccessStatusCodes []int          `json:"success_status_codes"`
	Config             map[string]any `json:"config"`
}

type ModelConfig struct {
	ResourceMeta
	Provider          string                 `json:"provider"`
	Model             string                 `json:"model"`
	EndpointRef       string                 `json:"endpoint_ref"`
	DefaultParameters map[string]any         `json:"default_parameters"`
	Runtime           RuntimeRequirementSpec `json:"runtime"`
	Policy            ExecutionPolicy        `json:"policy"`
}

type KnowledgeConfig struct {
	ResourceMeta
	Type              string                 `json:"type"`
	Source            KnowledgeSourceSpec    `json:"source"`
	EmbeddingModelRef ResourceRef            `json:"embedding_model_ref"`
	Chunking          map[string]any         `json:"chunking"`
	Index             map[string]any         `json:"index"`
	Runtime           RuntimeRequirementSpec `json:"runtime"`
}

type KnowledgeSourceSpec struct {
	Kind   string         `json:"kind"`
	URI    string         `json:"uri"`
	Config map[string]any `json:"config"`
}

type PolicyConfig struct {
	ResourceMeta
	Scope string         `json:"scope"`
	Rules map[string]any `json:"rules"`
}

type ExecutionPolicy struct {
	Timeout       string      `json:"timeout"`
	RequireReview bool        `json:"require_review"`
	RetryPolicy   RetryPolicy `json:"retry_policy"`
}

type RetryPolicy struct {
	MaxAttempts int    `json:"max_attempts"`
	Backoff     string `json:"backoff"`
}

type RuntimeManifest struct {
	RuntimeID    string              `json:"runtime_id"`
	Target       RuntimeTarget       `json:"target"`
	Version      string              `json:"version"`
	Capabilities []CapabilitySupport `json:"capabilities"`
	Metadata     map[string]any      `json:"metadata"`
}

type CapabilitySupport struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
