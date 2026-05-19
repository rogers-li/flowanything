package legacy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	coreconfig "flow-anything/core/config"
	coreschema "flow-anything/core/schema"

	_ "modernc.org/sqlite"
)

type Options struct {
	SourcePath string
	TenantID   string
	BundleID   string
	Name       string
	Version    string
	Lifecycle  string
}

type Report struct {
	Agents              int      `json:"agents"`
	Skills              int      `json:"skills"`
	Tools               int      `json:"tools"`
	SkippedTools        int      `json:"skipped_tools"`
	Connectors          int      `json:"connectors"`
	ConnectorOperations int      `json:"connector_operations"`
	Models              int      `json:"models"`
	Warnings            []string `json:"warnings"`
}

type Result struct {
	Bundle coreconfig.BundleSpec `json:"bundle"`
	Report Report                `json:"report"`
}

// MigrateP0 converts legacy platform registry data into the new config-as-code
// bundle format. It intentionally migrates only P0 resource configuration:
// connectors, connector operations, tools, skills, agents, and model refs.
func MigrateP0(ctx context.Context, options Options) (Result, error) {
	if strings.TrimSpace(options.SourcePath) == "" {
		return Result{}, fmt.Errorf("source sqlite path is required")
	}
	tenantID := strings.TrimSpace(options.TenantID)
	if tenantID == "" {
		tenantID = "tenant_1"
	}
	db, err := sql.Open("sqlite", sqliteReadOnlyDSN(options.SourcePath))
	if err != nil {
		return Result{}, fmt.Errorf("open sqlite source: %w", err)
	}
	defer db.Close()

	source, err := loadSource(ctx, db, tenantID)
	if err != nil {
		return Result{}, err
	}
	converter := p0Converter{
		source:      source,
		tenantID:    tenantID,
		bundleID:    defaultString(options.BundleID, "workspace_migrated"),
		bundleName:  defaultString(options.Name, "Migrated AI Platform Workspace"),
		bundleVer:   defaultString(options.Version, "migrated"),
		lifecycle:   defaultString(options.Lifecycle, string(coreconfig.BundleLifecycleDraft)),
		modelRefs:   map[string]legacyModelConfig{},
		connByID:    map[string]*coreconfig.ConnectorConfig{},
		operationID: map[string]string{},
		toolID:      map[string]bool{},
	}
	bundle, report := converter.convert()
	if err := coreconfig.ValidateBundle(bundle); err != nil {
		return Result{}, fmt.Errorf("generated bundle is invalid: %w", err)
	}
	report.Models = len(bundle.Resources.Models)
	return Result{Bundle: bundle, Report: report}, nil
}

func sqliteReadOnlyDSN(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	// The migrator only executes SELECT statements. Avoid SQLite URI readonly
	// mode here because modernc/sqlite can return misleading low-level errors
	// for some relative file URIs.
	return "file:" + abs + "?cache=shared"
}

type sourceData struct {
	Agents     []legacyAgent
	Skills     []legacySkill
	Tools      []legacyTool
	Connectors []legacyConnector
	Operations []legacyOperation
}

func loadSource(ctx context.Context, db *sql.DB, tenantID string) (sourceData, error) {
	connectors, err := loadConnectors(ctx, db, tenantID)
	if err != nil {
		return sourceData{}, err
	}
	operations, err := loadOperations(ctx, db, tenantID)
	if err != nil {
		return sourceData{}, err
	}
	tools, err := loadTools(ctx, db, tenantID)
	if err != nil {
		return sourceData{}, err
	}
	skills, err := loadSkills(ctx, db, tenantID)
	if err != nil {
		return sourceData{}, err
	}
	agents, err := loadAgents(ctx, db, tenantID)
	if err != nil {
		return sourceData{}, err
	}
	return sourceData{
		Agents:     agents,
		Skills:     skills,
		Tools:      tools,
		Connectors: connectors,
		Operations: operations,
	}, nil
}

type p0Converter struct {
	source     sourceData
	tenantID   string
	bundleID   string
	bundleName string
	bundleVer  string
	lifecycle  string
	report     Report
	modelRefs  map[string]legacyModelConfig
	connByID   map[string]*coreconfig.ConnectorConfig
	// operationID maps legacy operation IDs that survived migration.
	operationID map[string]string
	// toolID maps legacy tool IDs that survived migration.
	toolID map[string]bool
}

func (c *p0Converter) convert() (coreconfig.BundleSpec, Report) {
	bundle := coreconfig.BundleSpec{
		SchemaVersion: coreconfig.SchemaVersionV1,
		Kind:          coreconfig.BundleKind,
		ID:            c.bundleID,
		Name:          c.bundleName,
		Version:       c.bundleVer,
		Description:   "Migrated P0 configuration from the legacy local registry.",
		Runtime: coreconfig.RuntimeTargetSpec{
			Targets:           []coreconfig.RuntimeTarget{coreconfig.RuntimeServer},
			MinRuntimeVersion: "v1",
			Config:            map[string]any{},
		},
		Dependencies: []coreconfig.ResourceRef{},
		Permissions:  coreconfig.PermissionSpec{},
		Signature:    coreconfig.SignatureSpec{},
		Resources:    coreconfig.ResourceCollection{},
		Metadata: map[string]any{
			coreconfig.BundleMetadataLifecycle: c.lifecycle,
			"migration": map[string]any{
				"source":     "legacy_sqlite_p0",
				"tenant_id":  c.tenantID,
				"lifecycle":  c.lifecycle,
				"created_at": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	c.convertConnectors()
	c.convertOperations()
	bundle.Resources.Connectors = sortedConnectors(c.connByID)
	bundle.Resources.Tools = c.convertTools()
	bundle.Resources.Skills = c.convertSkills()
	bundle.Resources.Agents = c.convertAgents()
	bundle.Resources.Models = c.convertModels()
	bundle.Permissions.SecretRefs = collectSecretRefs(bundle.Resources.Connectors)
	c.report.Connectors = len(bundle.Resources.Connectors)
	c.report.ConnectorOperations = countConnectorOperations(bundle.Resources.Connectors)
	c.report.Tools = len(bundle.Resources.Tools)
	c.report.Skills = len(bundle.Resources.Skills)
	c.report.Agents = len(bundle.Resources.Agents)
	return bundle, c.report
}

func (c *p0Converter) convertConnectors() {
	for _, item := range c.source.Connectors {
		connector := coreconfig.ConnectorConfig{
			ResourceMeta: resourceMeta(item.ID, item.Name, item.Description, item.Version, item.Status, item.BusinessDomain, item.OwnerTeam, map[string]any{
				"timeout_ms": item.TimeoutMillis,
				"type":       item.Type,
			}),
			Protocol: coreconfig.ConnectorProtocolSpec{
				Kind:    defaultString(item.Type, "http"),
				BaseURL: item.BaseURL,
				Config: map[string]any{
					"headers": item.Headers,
				},
			},
			Auth:       connectorAuth(item.Auth),
			Operations: []coreconfig.ConnectorOperationConfig{},
			Runtime:    networkRuntime(),
		}
		c.connByID[item.ID] = &connector
	}
}

func (c *p0Converter) convertOperations() {
	for _, item := range c.source.Operations {
		connectorID := strings.TrimSpace(item.ConnectorID)
		if connectorID == "" {
			connectorID = "conn_" + item.ID
			c.warn("operation %q had no connector_id; created synthetic connector %q", item.ID, connectorID)
		}
		connector := c.connByID[connectorID]
		if connector == nil {
			connector = &coreconfig.ConnectorConfig{
				ResourceMeta: resourceMeta(connectorID, connectorID, "Synthetic connector generated from legacy operation "+item.ID, "v1", item.Status, item.BusinessDomain, item.OwnerTeam, map[string]any{
					"synthetic":  true,
					"source_op":  item.ID,
					"timeout_ms": item.TimeoutMillis,
				}),
				Protocol: coreconfig.ConnectorProtocolSpec{
					Kind:    defaultString(item.Type, "http"),
					BaseURL: item.BaseURL,
					Config: map[string]any{
						"headers": item.Headers,
					},
				},
				Auth:       connectorAuth(item.Auth),
				Operations: []coreconfig.ConnectorOperationConfig{},
				Runtime:    networkRuntime(),
			}
			c.connByID[connectorID] = connector
			c.warn("operation %q referenced missing connector %q; created synthetic connector", item.ID, connectorID)
		}
		if connector.Protocol.BaseURL == "" {
			connector.Protocol.BaseURL = item.BaseURL
		} else if item.BaseURL != "" && connector.Protocol.BaseURL != item.BaseURL {
			c.warn("operation %q base_url differs from connector %q; migrated connector base_url %q and operation path %q", item.ID, connectorID, connector.Protocol.BaseURL, item.Path)
		}
		if connector.Auth.Type == "" || connector.Auth.Type == "none" {
			connector.Auth = connectorAuth(item.Auth)
		} else if authType(item.Auth) != "" && authType(item.Auth) != "none" && connector.Auth.Type != authType(item.Auth) {
			c.warn("operation %q auth differs from connector %q; operation auth was not promoted", item.ID, connectorID)
		}
		operation := coreconfig.ConnectorOperationConfig{
			ResourceMeta: resourceMeta(item.ID, item.Name, item.Description, item.Version, item.Status, item.BusinessDomain, item.OwnerTeam, map[string]any{
				"implementation_mode": item.ImplementationMode,
			}),
			InputSchema:  schemaFieldsFromJSONSchema(item.InputSchema),
			OutputSchema: schemaFieldsFromJSONSchema(item.OutputSchema),
			Request: coreconfig.ConnectorOperationRequest{
				Method:  strings.ToUpper(defaultString(item.Method, "GET")),
				Path:    defaultString(item.Path, "/"),
				Headers: item.Headers,
				Config: map[string]any{
					"legacy_type": item.Type,
				},
			},
			Response: coreconfig.ConnectorOperationResponse{
				SuccessStatusCodes: []int{200, 201, 202, 204},
				Config:             map[string]any{},
			},
			Policy: coreconfig.ExecutionPolicy{
				Timeout: durationMillis(item.TimeoutMillis, 10000),
				RetryPolicy: coreconfig.RetryPolicy{
					MaxAttempts: 0,
					Backoff:     "0ms",
				},
			},
		}
		connector.Operations = append(connector.Operations, operation)
		c.operationID[item.ID] = item.ID
	}
}

func (c *p0Converter) convertTools() []coreconfig.ToolConfig {
	out := make([]coreconfig.ToolConfig, 0, len(c.source.Tools))
	for _, item := range c.source.Tools {
		converted, ok := c.convertTool(item)
		if !ok {
			c.report.SkippedTools++
			continue
		}
		c.toolID[converted.ID] = true
		out = append(out, converted)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (c *p0Converter) convertTool(item legacyTool) (coreconfig.ToolConfig, bool) {
	toolType, implKind := toolTypeAndKind(item.Implementation)
	if toolType == "" {
		c.warn("tool %q implementation %q is not supported by P0 migration; skipped", item.ID, item.Implementation)
		return coreconfig.ToolConfig{}, false
	}
	ref := coreconfig.ResourceRef{}
	config := map[string]any{}
	switch item.Implementation {
	case "connector":
		opID := strings.TrimSpace(item.Binding.ConnectorOperationID)
		if opID == "" {
			c.warn("connector tool %q has no connector operation binding; skipped", item.ID)
			return coreconfig.ToolConfig{}, false
		}
		if _, exists := c.operationID[opID]; !exists {
			c.warn("connector tool %q references missing operation %q; skipped", item.ID, opID)
			return coreconfig.ToolConfig{}, false
		}
		ref = coreconfig.ResourceRef{Kind: coreconfig.ResourceConnectorOperation, ID: opID}
	case "workflow":
		c.warn("workflow tool %q was skipped because workflow migration is outside P0", item.ID)
		return coreconfig.ToolConfig{}, false
	case "knowledge":
		c.warn("knowledge tool %q was skipped because knowledge migration is outside P0", item.ID)
		return coreconfig.ToolConfig{}, false
	case "python":
		config["python_package_id"] = item.Binding.PythonPackageID
	case "mcp":
		config["mcp_server_id"] = item.Binding.MCPServerID
		config["mcp_server_url"] = item.Binding.MCPServerURL
		config["mcp_transport"] = item.Binding.MCPTransport
		config["mcp_headers"] = item.Binding.MCPHeaders
		config["mcp_tool_name"] = item.Binding.MCPToolName
	}
	return coreconfig.ToolConfig{
		ResourceMeta: resourceMeta(item.ID, item.Name, item.Description, item.Version, item.Status, item.BusinessDomain, item.OwnerTeam, map[string]any{
			"status":          item.Status,
			"llm_description": firstNonEmpty(item.LLMDescription, item.Description),
			"side_effect":     item.SideEffect,
			"risk_level":      item.RiskLevel,
		}),
		Type:         toolType,
		InputSchema:  schemaFieldsFromJSONSchema(item.InputSchema),
		OutputSchema: schemaFieldsFromJSONSchema(item.OutputSchema),
		Implementation: coreconfig.ToolImplementationSpec{
			Kind:   implKind,
			Ref:    ref,
			Config: config,
		},
		Policy: coreconfig.ExecutionPolicy{
			Timeout:       durationMillis(item.TimeoutMillis, 10000),
			RequireReview: item.RequiresConfirmation || item.RiskLevel == "high",
			RetryPolicy: coreconfig.RetryPolicy{
				MaxAttempts: item.RetryPolicy.MaxAttempts,
				Backoff:     durationMillis(item.RetryPolicy.BackoffMillis, 0),
			},
		},
		Runtime: networkRuntime(),
	}, true
}

func (c *p0Converter) convertSkills() []coreconfig.SkillConfig {
	out := make([]coreconfig.SkillConfig, 0, len(c.source.Skills))
	for _, item := range c.source.Skills {
		toolBindings := make([]coreconfig.ResourceBinding, 0, len(item.ToolIDs))
		for _, toolID := range item.ToolIDs {
			if !c.toolID[toolID] {
				c.warn("skill %q references non-migrated tool %q; binding omitted", item.ID, toolID)
				continue
			}
			toolBindings = append(toolBindings, binding(coreconfig.ResourceTool, toolID))
		}
		knowledgeBindings := make([]coreconfig.ResourceBinding, 0)
		for _, knowledgeID := range item.KnowledgeIDs {
			c.warn("skill %q references knowledge base %q; knowledge migration is outside P0 and binding was omitted", item.ID, knowledgeID)
		}
		out = append(out, coreconfig.SkillConfig{
			ResourceMeta: resourceMeta(item.ID, item.Name, item.Description, item.Version, item.Status, item.BusinessDomain, item.OwnerTeam, map[string]any{
				"status":               item.Status,
				"use_cases":            item.UseCases,
				"exclusions":           item.Exclusions,
				"output_format":        item.OutputFormat,
				"risk_level":           item.RiskLevel,
				"max_tool_calls":       item.ExecutionPolicy.MaxToolCalls,
				"timeout_ms":           item.ExecutionPolicy.TimeoutMillis,
				"allow_write_tools":    item.ExecutionPolicy.AllowWriteTools,
				"require_confirmation": item.ExecutionPolicy.RequireConfirmation,
				"policy_version":       item.PolicyVersion,
			}),
			Prompt: coreconfig.PromptConfig{
				System:    defaultString(item.SystemPrompt, "Use the configured tools to complete this skill."),
				Developer: "",
				Templates: map[string]string{},
				Variables: []coreconfig.SchemaField{},
				Metadata:  map[string]any{},
			},
			InputSchema:  []coreconfig.SchemaField{},
			OutputSchema: []coreconfig.SchemaField{},
			Tools:        toolBindings,
			Knowledge:    knowledgeBindings,
			Policies:     []coreconfig.ResourceRef{},
			Runtime:      networkRuntime(),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (c *p0Converter) convertAgents() []coreconfig.AgentConfig {
	availableSkills := map[string]bool{}
	for _, skill := range c.source.Skills {
		availableSkills[skill.ID] = true
	}
	out := make([]coreconfig.AgentConfig, 0, len(c.source.Agents))
	for _, item := range c.source.Agents {
		skillBindings := make([]coreconfig.ResourceBinding, 0, len(item.SkillIDs))
		for _, skillID := range item.SkillIDs {
			if !availableSkills[skillID] {
				c.warn("agent %q references missing skill %q; binding omitted", item.ID, skillID)
				continue
			}
			skillBindings = append(skillBindings, binding(coreconfig.ResourceSkill, skillID))
		}
		toolBindings := make([]coreconfig.ResourceBinding, 0, len(item.ToolIDs))
		for _, toolID := range item.ToolIDs {
			if !c.toolID[toolID] {
				c.warn("agent %q references non-migrated tool %q; binding omitted", item.ID, toolID)
				continue
			}
			toolBindings = append(toolBindings, binding(coreconfig.ResourceTool, toolID))
		}
		modelRef := c.modelRef(item.ModelConfig)
		out = append(out, coreconfig.AgentConfig{
			ResourceMeta: resourceMeta(item.ID, item.Name, item.Description, item.Version, item.Status, item.BusinessDomain, item.OwnerTeam, map[string]any{
				"status":              item.Status,
				"default_lang":        item.DefaultLang,
				"supported_languages": item.SupportedLanguages,
				"channels":            item.Channels,
				"welcome_message":     item.WelcomeMessage,
				"model":               item.ModelConfig.Model,
				"temperature":         item.ModelConfig.Temperature,
				"max_turns":           item.RuntimePolicy.MaxTurns,
				"max_tool_calls":      item.RuntimePolicy.MaxToolCalls,
				"response_timeout_ms": item.RuntimePolicy.ResponseTimeoutMs,
			}),
			Prompt: coreconfig.PromptConfig{
				System:    defaultString(item.SystemPrompt, "You are an enterprise AI assistant. Help the user with the configured skills and tools."),
				Developer: "",
				Templates: map[string]string{},
				Variables: []coreconfig.SchemaField{},
				Metadata:  map[string]any{},
			},
			Reasoning: coreconfig.ReasoningConfig{
				Mode:   "react",
				Config: map[string]any{},
			},
			ModelRef:     modelRef,
			Skills:       skillBindings,
			Tools:        toolBindings,
			Workflows:    []coreconfig.ResourceBinding{},
			Knowledge:    []coreconfig.ResourceBinding{},
			Policies:     []coreconfig.ResourceRef{},
			OutputSchema: []coreconfig.SchemaField{},
			Runtime:      networkRuntime(),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (c *p0Converter) modelRef(model legacyModelConfig) coreconfig.ResourceRef {
	id := strings.TrimSpace(model.ProviderID)
	if id == "" {
		id = "model_mock"
	}
	c.modelRefs[id] = model
	return coreconfig.ResourceRef{Kind: coreconfig.ResourceModel, ID: id, Alias: model.Model}
}

func (c *p0Converter) convertModels() []coreconfig.ModelConfig {
	if len(c.modelRefs) == 0 {
		c.modelRefs["model_mock"] = legacyModelConfig{}
	}
	out := make([]coreconfig.ModelConfig, 0, len(c.modelRefs))
	ids := sortedKeys(c.modelRefs)
	for _, id := range ids {
		model := c.modelRefs[id]
		modelName := defaultString(model.Model, "mock-chat")
		out = append(out, coreconfig.ModelConfig{
			ResourceMeta: resourceMeta(id, id, "Migrated model reference "+id, "v1", "enabled", "AI", "AI Platform", map[string]any{
				"temperature": model.Temperature,
			}),
			Provider:          inferProvider(id, modelName),
			Model:             modelName,
			EndpointRef:       "",
			DefaultParameters: map[string]any{},
			Runtime:           networkRuntime(),
			Policy: coreconfig.ExecutionPolicy{
				Timeout:       "10m",
				RequireReview: false,
				RetryPolicy: coreconfig.RetryPolicy{
					MaxAttempts: 0,
					Backoff:     "0ms",
				},
			},
		})
	}
	return out
}

func (c *p0Converter) warn(format string, args ...any) {
	c.report.Warnings = append(c.report.Warnings, fmt.Sprintf(format, args...))
}

func resourceMeta(id, name, description, version, status, domain, owner string, metadata map[string]any) coreconfig.ResourceMeta {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["status"] = defaultString(status, "enabled")
	return coreconfig.ResourceMeta{
		ID:          id,
		Name:        defaultString(name, id),
		Description: description,
		Version:     defaultString(version, "v1"),
		Disabled:    status != "" && status != "enabled",
		Labels:      stringList(domain),
		Annotations: map[string]string{},
		Owner: coreconfig.OwnerSpec{
			Team:  owner,
			Email: "",
		},
		Metadata: metadata,
	}
}

func connectorAuth(auth legacyAuthConfig) coreconfig.ConnectorAuthSpec {
	config := map[string]any{}
	putAuthLocation(config, auth.HeaderName)
	putNonEmpty(config, "provider", auth.Provider)
	putNonEmpty(config, "client_id_ref", auth.ClientIDRef)
	putNonEmpty(config, "client_secret_ref", auth.ClientSecretRef)
	putNonEmpty(config, "refresh_token_ref", auth.RefreshTokenRef)
	putNonEmpty(config, "authorization_code_ref", auth.AuthorizationCodeRef)
	putNonEmpty(config, "app_access_token_url", auth.AppAccessTokenURL)
	putNonEmpty(config, "access_token_url", auth.AccessTokenURL)
	putNonEmpty(config, "refresh_token_url", auth.RefreshTokenURL)
	putNonEmpty(config, "tenant_access_token_url", auth.TenantTokenURL)
	authType := defaultString(auth.Type, "")
	if authType == "none" {
		authType = ""
	}
	return coreconfig.ConnectorAuthSpec{
		Type:      authType,
		SecretRef: auth.SecretRef,
		Config:    config,
	}
}

func putAuthLocation(config map[string]any, headerName string) {
	headerName = strings.TrimSpace(headerName)
	if headerName == "" {
		return
	}
	if strings.HasPrefix(headerName, "query:") {
		putNonEmpty(config, "in", "query")
		putNonEmpty(config, "name", strings.TrimSpace(strings.TrimPrefix(headerName, "query:")))
		return
	}
	putNonEmpty(config, "in", "header")
	putNonEmpty(config, "name", headerName)
}

func authType(auth legacyAuthConfig) string {
	return defaultString(auth.Type, "none")
}

func toolTypeAndKind(implementation string) (coreconfig.ToolType, string) {
	switch implementation {
	case "connector":
		return coreconfig.ToolTypeConnector, "connector"
	case "mcp":
		return coreconfig.ToolTypeMCP, "mcp"
	case "python":
		return coreconfig.ToolTypeScript, "python"
	default:
		return "", ""
	}
}

func schemaFieldsFromJSONSchema(value map[string]any) []coreconfig.SchemaField {
	if len(value) == 0 {
		return []coreconfig.SchemaField{}
	}
	properties := objectMap(value["properties"])
	if len(properties) == 0 && !looksLikeSchemaObject(value) {
		properties = value
	}
	required := requiredSet(value["required"])
	names := sortedKeysAny(properties)
	fields := make([]coreconfig.SchemaField, 0, len(names))
	for _, name := range names {
		definition := objectMap(properties[name])
		if len(definition) == 0 {
			definition = map[string]any{"type": "any"}
		}
		fields = append(fields, schemaField(name, definition, required[name]))
	}
	return fields
}

func schemaField(name string, definition map[string]any, required bool) coreconfig.SchemaField {
	fieldType := schemaFieldType(stringValue(definition["type"]), definition)
	field := coreconfig.SchemaField{
		Name:        name,
		Type:        fieldType,
		Description: stringValue(definition["description"]),
		Required:    required,
		Examples:    arrayValue(definition["examples"]),
		Metadata:    schemaMetadata(definition),
	}
	switch fieldType {
	case coreschema.TypeObject:
		field.Children = schemaFieldsFromJSONSchema(definition)
	case coreschema.TypeArray:
		items := objectMap(definition["items"])
		if len(items) > 0 {
			field.Children = schemaFieldsFromJSONSchema(items)
			if itemType := stringValue(items["type"]); itemType != "" {
				if field.Metadata == nil {
					field.Metadata = map[string]any{}
				}
				field.Metadata["items_type"] = itemType
			}
		}
	}
	return field
}

func schemaFieldType(value string, definition map[string]any) coreschema.FieldType {
	switch strings.ToLower(value) {
	case "string":
		return coreschema.TypeString
	case "number":
		return coreschema.TypeNumber
	case "integer", "int":
		return coreschema.TypeInteger
	case "boolean", "bool":
		return coreschema.TypeBoolean
	case "object":
		return coreschema.TypeObject
	case "array":
		return coreschema.TypeArray
	case "":
		if len(objectMap(definition["properties"])) > 0 {
			return coreschema.TypeObject
		}
		if len(objectMap(definition["items"])) > 0 {
			return coreschema.TypeArray
		}
		return coreschema.TypeAny
	default:
		return coreschema.TypeAny
	}
}

func schemaMetadata(definition map[string]any) map[string]any {
	metadata := map[string]any{}
	for _, key := range []string{"enum", "default", "format", "nullable"} {
		if value, ok := definition[key]; ok {
			metadata[key] = value
		}
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func requiredSet(value any) map[string]bool {
	out := map[string]bool{}
	for _, item := range arrayValue(value) {
		if text, ok := item.(string); ok {
			out[text] = true
		}
	}
	return out
}

func objectMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func arrayValue(value any) []any {
	if typed, ok := value.([]any); ok {
		return typed
	}
	return nil
}

func looksLikeSchemaObject(value map[string]any) bool {
	_, hasType := value["type"]
	_, hasProps := value["properties"]
	return hasType || hasProps
}

func binding(kind coreconfig.ResourceKind, id string) coreconfig.ResourceBinding {
	return coreconfig.ResourceBinding{
		Ref: coreconfig.ResourceRef{
			Kind: kind,
			ID:   id,
		},
		Alias:    id,
		Disabled: false,
		Config:   map[string]any{},
	}
}

func networkRuntime() coreconfig.RuntimeRequirementSpec {
	return coreconfig.RuntimeRequirementSpec{
		Network:            true,
		ServerProxyAllowed: true,
		Secrets:            []coreconfig.SecretRequirement{},
		Capabilities:       []coreconfig.CapabilityRequirement{},
	}
}

func sortedConnectors(items map[string]*coreconfig.ConnectorConfig) []coreconfig.ConnectorConfig {
	ids := sortedKeys(items)
	out := make([]coreconfig.ConnectorConfig, 0, len(ids))
	for _, id := range ids {
		connector := *items[id]
		sort.SliceStable(connector.Operations, func(i, j int) bool {
			return connector.Operations[i].ID < connector.Operations[j].ID
		})
		out = append(out, connector)
	}
	return out
}

func countConnectorOperations(connectors []coreconfig.ConnectorConfig) int {
	count := 0
	for _, connector := range connectors {
		count += len(connector.Operations)
	}
	return count
}

func collectSecretRefs(connectors []coreconfig.ConnectorConfig) []string {
	seen := map[string]bool{}
	for _, connector := range connectors {
		if connector.Auth.SecretRef != "" {
			seen[connector.Auth.SecretRef] = true
		}
		for key, value := range connector.Auth.Config {
			text, ok := value.(string)
			if !ok || text == "" {
				continue
			}
			normalizedKey := strings.ToLower(strings.TrimSpace(key))
			if strings.HasSuffix(normalizedKey, "_ref") || normalizedKey == "secret_ref" {
				seen[text] = true
			}
		}
	}
	return sortedKeys(seen)
}

func inferProvider(id string, model string) string {
	normalized := strings.ToLower(id + " " + model)
	switch {
	case strings.Contains(normalized, "deepseek"):
		return "deepseek"
	case strings.Contains(normalized, "openai"):
		return "openai-compatible"
	case strings.Contains(normalized, "mock"):
		return "mock"
	default:
		return defaultString(id, "mock")
	}
}

func durationMillis(value int, fallback int) string {
	if value <= 0 {
		value = fallback
	}
	return fmt.Sprintf("%dms", value)
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func putNonEmpty(target map[string]any, key string, value string) {
	if strings.TrimSpace(value) != "" {
		target[key] = strings.TrimSpace(value)
	}
}

func stringValue(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func stringList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	return []string{strings.TrimSpace(value)}
}

func sortedKeys[V any](items map[string]V) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysAny(items map[string]any) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func decodeJSON(value string, target any) error {
	if strings.TrimSpace(value) == "" {
		value = "{}"
	}
	if err := json.Unmarshal([]byte(value), target); err != nil {
		return fmt.Errorf("decode json %q: %w", value, err)
	}
	return nil
}
