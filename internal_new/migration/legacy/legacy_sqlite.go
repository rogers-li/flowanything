package legacy

import (
	"context"
	"database/sql"
	"fmt"
)

type legacyAgent struct {
	ID                 string
	TenantID           string
	Name               string
	Description        string
	BusinessDomain     string
	OwnerTeam          string
	Status             string
	SkillIDs           []string
	ToolIDs            []string
	DefaultLang        string
	SupportedLanguages []string
	Channels           []string
	SystemPrompt       string
	WelcomeMessage     string
	ModelConfig        legacyModelConfig
	RuntimePolicy      legacyRuntimePolicy
	Version            string
}

type legacyModelConfig struct {
	ProviderID  string  `json:"provider_id,omitempty"`
	Model       string  `json:"model,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

type legacyRuntimePolicy struct {
	MaxTurns          int `json:"max_turns"`
	MaxToolCalls      int `json:"max_tool_calls"`
	ResponseTimeoutMs int `json:"response_timeout_ms"`
}

type legacySkill struct {
	ID              string
	TenantID        string
	Name            string
	Description     string
	BusinessDomain  string
	OwnerTeam       string
	Status          string
	ToolIDs         []string
	KnowledgeIDs    []string
	SystemPrompt    string
	UseCases        []string
	Exclusions      []string
	OutputFormat    string
	RiskLevel       string
	ExecutionPolicy legacySkillExecutionPolicy
	PolicyVersion   string
	Version         string
}

type legacySkillExecutionPolicy struct {
	MaxToolCalls        int  `json:"max_tool_calls"`
	TimeoutMillis       int  `json:"timeout_ms"`
	AllowWriteTools     bool `json:"allow_write_tools"`
	RequireConfirmation bool `json:"require_confirmation"`
}

type legacyTool struct {
	ID                   string
	TenantID             string
	Name                 string
	Description          string
	BusinessDomain       string
	OwnerTeam            string
	Status               string
	LLMDescription       string
	Implementation       string
	Binding              legacyToolBinding
	InputSchema          map[string]any
	OutputSchema         map[string]any
	SideEffect           string
	RiskLevel            string
	RequiresConfirmation bool
	TimeoutMillis        int
	RetryPolicy          legacyRetryPolicy
	Version              string
}

type legacyToolBinding struct {
	ConnectorOperationID string            `json:"connector_operation_id,omitempty"`
	KnowledgeBaseIDs     []string          `json:"knowledge_base_ids,omitempty"`
	PythonPackageID      string            `json:"python_package_id,omitempty"`
	MCPServerID          string            `json:"mcp_server_id,omitempty"`
	MCPServerURL         string            `json:"mcp_server_url,omitempty"`
	MCPTransport         string            `json:"mcp_transport,omitempty"`
	MCPHeaders           map[string]string `json:"mcp_headers,omitempty"`
	MCPToolName          string            `json:"mcp_tool_name,omitempty"`
	WorkflowID           string            `json:"workflow_id,omitempty"`
}

type legacyRetryPolicy struct {
	MaxAttempts   int `json:"max_attempts"`
	BackoffMillis int `json:"backoff_ms"`
}

type legacyConnector struct {
	ID             string
	TenantID       string
	Name           string
	Description    string
	BusinessDomain string
	OwnerTeam      string
	Type           string
	Status         string
	BaseURL        string
	Headers        map[string]string
	Auth           legacyAuthConfig
	TimeoutMillis  int
	Version        string
}

type legacyOperation struct {
	ID                 string
	TenantID           string
	ConnectorID        string
	Name               string
	Description        string
	BusinessDomain     string
	OwnerTeam          string
	ImplementationMode string
	Type               string
	Status             string
	BaseURL            string
	Method             string
	Path               string
	Headers            map[string]string
	Auth               legacyAuthConfig
	InputSchema        map[string]any
	OutputSchema       map[string]any
	TimeoutMillis      int
	Version            string
}

type legacyAuthConfig struct {
	Type                 string `json:"type"`
	HeaderName           string `json:"header_name,omitempty"`
	SecretRef            string `json:"secret_ref,omitempty"`
	Provider             string `json:"provider,omitempty"`
	ClientIDRef          string `json:"client_id_ref,omitempty"`
	ClientSecretRef      string `json:"client_secret_ref,omitempty"`
	RefreshTokenRef      string `json:"refresh_token_ref,omitempty"`
	AuthorizationCodeRef string `json:"authorization_code_ref,omitempty"`
	AppAccessTokenURL    string `json:"app_access_token_url,omitempty"`
	AccessTokenURL       string `json:"access_token_url,omitempty"`
	RefreshTokenURL      string `json:"refresh_token_url,omitempty"`
	TenantTokenURL       string `json:"tenant_access_token_url,omitempty"`
}

func loadAgents(ctx context.Context, db *sql.DB, tenantID string) ([]legacyAgent, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, status,
			skill_ids_json, tool_ids_json, default_lang, supported_languages_json, channels_json,
			system_prompt, welcome_message, model_config_json, runtime_policy_json, version
		FROM agents
		WHERE tenant_id = ?
		ORDER BY id
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("load legacy agents: %w", err)
	}
	defer rows.Close()

	var result []legacyAgent
	for rows.Next() {
		var item legacyAgent
		var skillIDs, toolIDs, supportedLanguages, channels, modelConfig, runtimePolicy string
		if err := rows.Scan(
			&item.TenantID, &item.ID, &item.Name, &item.Description, &item.BusinessDomain, &item.OwnerTeam, &item.Status,
			&skillIDs, &toolIDs, &item.DefaultLang, &supportedLanguages, &channels,
			&item.SystemPrompt, &item.WelcomeMessage, &modelConfig, &runtimePolicy, &item.Version,
		); err != nil {
			return nil, fmt.Errorf("scan legacy agent: %w", err)
		}
		if err := decodeJSON(skillIDs, &item.SkillIDs); err != nil {
			return nil, err
		}
		if err := decodeJSON(toolIDs, &item.ToolIDs); err != nil {
			return nil, err
		}
		if err := decodeJSON(supportedLanguages, &item.SupportedLanguages); err != nil {
			return nil, err
		}
		if err := decodeJSON(channels, &item.Channels); err != nil {
			return nil, err
		}
		if err := decodeJSON(modelConfig, &item.ModelConfig); err != nil {
			return nil, err
		}
		if err := decodeJSON(runtimePolicy, &item.RuntimePolicy); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func loadSkills(ctx context.Context, db *sql.DB, tenantID string) ([]legacySkill, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, status,
			tool_ids_json, knowledge_ids_json, system_prompt, use_cases_json, exclusions_json,
			output_format, risk_level, execution_policy_json, policy_version, version
		FROM skills
		WHERE tenant_id = ?
		ORDER BY id
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("load legacy skills: %w", err)
	}
	defer rows.Close()

	var result []legacySkill
	for rows.Next() {
		var item legacySkill
		var toolIDs, knowledgeIDs, useCases, exclusions, executionPolicy string
		if err := rows.Scan(
			&item.TenantID, &item.ID, &item.Name, &item.Description, &item.BusinessDomain, &item.OwnerTeam, &item.Status,
			&toolIDs, &knowledgeIDs, &item.SystemPrompt, &useCases, &exclusions,
			&item.OutputFormat, &item.RiskLevel, &executionPolicy, &item.PolicyVersion, &item.Version,
		); err != nil {
			return nil, fmt.Errorf("scan legacy skill: %w", err)
		}
		if err := decodeJSON(toolIDs, &item.ToolIDs); err != nil {
			return nil, err
		}
		if err := decodeJSON(knowledgeIDs, &item.KnowledgeIDs); err != nil {
			return nil, err
		}
		if err := decodeJSON(useCases, &item.UseCases); err != nil {
			return nil, err
		}
		if err := decodeJSON(exclusions, &item.Exclusions); err != nil {
			return nil, err
		}
		if err := decodeJSON(executionPolicy, &item.ExecutionPolicy); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func loadTools(ctx context.Context, db *sql.DB, tenantID string) ([]legacyTool, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, status, llm_description,
			implementation, binding_json, input_schema_json, output_schema_json, side_effect, risk_level,
			requires_confirmation, timeout_ms, retry_policy_json, version
		FROM tools
		WHERE tenant_id = ?
		ORDER BY id
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("load legacy tools: %w", err)
	}
	defer rows.Close()

	var result []legacyTool
	for rows.Next() {
		var item legacyTool
		var binding, inputSchema, outputSchema, retryPolicy string
		var requiresConfirmation int
		if err := rows.Scan(
			&item.TenantID, &item.ID, &item.Name, &item.Description, &item.BusinessDomain, &item.OwnerTeam, &item.Status, &item.LLMDescription,
			&item.Implementation, &binding, &inputSchema, &outputSchema, &item.SideEffect, &item.RiskLevel,
			&requiresConfirmation, &item.TimeoutMillis, &retryPolicy, &item.Version,
		); err != nil {
			return nil, fmt.Errorf("scan legacy tool: %w", err)
		}
		item.RequiresConfirmation = requiresConfirmation == 1
		if err := decodeJSON(binding, &item.Binding); err != nil {
			return nil, err
		}
		if err := decodeJSON(inputSchema, &item.InputSchema); err != nil {
			return nil, err
		}
		if err := decodeJSON(outputSchema, &item.OutputSchema); err != nil {
			return nil, err
		}
		if err := decodeJSON(retryPolicy, &item.RetryPolicy); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func loadConnectors(ctx context.Context, db *sql.DB, tenantID string) ([]legacyConnector, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, type, status, base_url, headers_json, auth_json, timeout_ms, version
		FROM connectors
		WHERE tenant_id = ?
		ORDER BY id
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("load legacy connectors: %w", err)
	}
	defer rows.Close()

	var result []legacyConnector
	for rows.Next() {
		var item legacyConnector
		var headers, auth string
		if err := rows.Scan(
			&item.TenantID, &item.ID, &item.Name, &item.Description, &item.BusinessDomain, &item.OwnerTeam, &item.Type, &item.Status,
			&item.BaseURL, &headers, &auth, &item.TimeoutMillis, &item.Version,
		); err != nil {
			return nil, fmt.Errorf("scan legacy connector: %w", err)
		}
		if err := decodeJSON(headers, &item.Headers); err != nil {
			return nil, err
		}
		if err := decodeJSON(auth, &item.Auth); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func loadOperations(ctx context.Context, db *sql.DB, tenantID string) ([]legacyOperation, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT tenant_id, id, connector_id, name, description, business_domain, owner_team, implementation_mode, type, status, base_url, method, path, headers_json,
			auth_json, input_schema_json, output_schema_json, timeout_ms, version
		FROM connector_operations
		WHERE tenant_id = ?
		ORDER BY id
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("load legacy connector operations: %w", err)
	}
	defer rows.Close()

	var result []legacyOperation
	for rows.Next() {
		var item legacyOperation
		var headers, auth, inputSchema, outputSchema string
		if err := rows.Scan(
			&item.TenantID, &item.ID, &item.ConnectorID, &item.Name, &item.Description, &item.BusinessDomain, &item.OwnerTeam, &item.ImplementationMode,
			&item.Type, &item.Status, &item.BaseURL, &item.Method, &item.Path, &headers,
			&auth, &inputSchema, &outputSchema, &item.TimeoutMillis, &item.Version,
		); err != nil {
			return nil, fmt.Errorf("scan legacy connector operation: %w", err)
		}
		if err := decodeJSON(headers, &item.Headers); err != nil {
			return nil, err
		}
		if err := decodeJSON(auth, &item.Auth); err != nil {
			return nil, err
		}
		if err := decodeJSON(inputSchema, &item.InputSchema); err != nil {
			return nil, err
		}
		if err := decodeJSON(outputSchema, &item.OutputSchema); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
