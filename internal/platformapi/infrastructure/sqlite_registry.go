package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"flow-anything/internal/platform/contracts/agent"
	"flow-anything/internal/platform/contracts/agentflow"
	"flow-anything/internal/platform/contracts/connector"
	"flow-anything/internal/platform/contracts/skill"
	"flow-anything/internal/platform/contracts/tool"
	"flow-anything/internal/platform/contracts/workflow"
	apperrors "flow-anything/internal/platform/kernel/errors"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"

	_ "modernc.org/sqlite"
)

type SQLiteRegistry struct {
	db *sql.DB
}

func OpenSQLiteRegistry(ctx context.Context, dsn string) (*SQLiteRegistry, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to open sqlite registry", err)
	}
	db.SetMaxOpenConns(1)

	registry := &SQLiteRegistry{db: db}
	if err := registry.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return registry, nil
}

func (r *SQLiteRegistry) Close() error {
	return r.db.Close()
}

func (r *SQLiteRegistry) Migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS agents (
			tenant_id TEXT NOT NULL,
			id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			business_domain TEXT NOT NULL DEFAULT '',
			owner_team TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',
			skill_ids_json TEXT NOT NULL DEFAULT '[]',
			tool_ids_json TEXT NOT NULL DEFAULT '[]',
			default_lang TEXT NOT NULL DEFAULT '',
			supported_languages_json TEXT NOT NULL DEFAULT '[]',
			channels_json TEXT NOT NULL DEFAULT '[]',
			system_prompt TEXT NOT NULL DEFAULT '',
			welcome_message TEXT NOT NULL DEFAULT '',
			model_config_json TEXT NOT NULL DEFAULT '{}',
			runtime_policy_json TEXT NOT NULL DEFAULT '{}',
			version TEXT NOT NULL,
			PRIMARY KEY (tenant_id, id)
		)`,
		`CREATE TABLE IF NOT EXISTS agent_flows (
			tenant_id TEXT NOT NULL,
			id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			business_domain TEXT NOT NULL DEFAULT '',
			owner_team TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',
			orchestration_mode TEXT NOT NULL DEFAULT 'workflow',
			supervisor_json TEXT NOT NULL DEFAULT '{}',
			graph_json TEXT NOT NULL DEFAULT '{}',
			context_schema_json TEXT NOT NULL DEFAULT '{}',
			input_schema_json TEXT NOT NULL DEFAULT '{}',
			output_schema_json TEXT NOT NULL DEFAULT '{}',
			version TEXT NOT NULL,
			PRIMARY KEY (tenant_id, id)
		)`,
		`CREATE TABLE IF NOT EXISTS workflows (
			tenant_id TEXT NOT NULL,
			id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			business_domain TEXT NOT NULL DEFAULT '',
			owner_team TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',
			profile TEXT NOT NULL DEFAULT 'tool_workflow',
			context_schema_json TEXT NOT NULL DEFAULT '{}',
			input_schema_json TEXT NOT NULL DEFAULT '{}',
			output_schema_json TEXT NOT NULL DEFAULT '{}',
			graph_json TEXT NOT NULL DEFAULT '{}',
			policy_json TEXT NOT NULL DEFAULT '{}',
			ui_json TEXT NOT NULL DEFAULT '{}',
			version TEXT NOT NULL,
			PRIMARY KEY (tenant_id, id)
		)`,
		`CREATE TABLE IF NOT EXISTS skills (
			tenant_id TEXT NOT NULL,
			id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			business_domain TEXT NOT NULL DEFAULT '',
			owner_team TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',
			tool_ids_json TEXT NOT NULL DEFAULT '[]',
			knowledge_ids_json TEXT NOT NULL DEFAULT '[]',
			system_prompt TEXT NOT NULL DEFAULT '',
			use_cases_json TEXT NOT NULL DEFAULT '[]',
			exclusions_json TEXT NOT NULL DEFAULT '[]',
			output_format TEXT NOT NULL DEFAULT '',
			risk_level TEXT NOT NULL DEFAULT 'low',
			execution_policy_json TEXT NOT NULL DEFAULT '{}',
			policy_version TEXT NOT NULL DEFAULT '',
			version TEXT NOT NULL,
			PRIMARY KEY (tenant_id, id)
		)`,
		`CREATE TABLE IF NOT EXISTS tools (
			tenant_id TEXT NOT NULL,
			id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			business_domain TEXT NOT NULL DEFAULT '',
			owner_team TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',
			llm_description TEXT NOT NULL DEFAULT '',
			implementation TEXT NOT NULL,
			binding_json TEXT NOT NULL DEFAULT '{}',
			input_schema_json TEXT NOT NULL DEFAULT '{}',
			output_schema_json TEXT NOT NULL DEFAULT '{}',
			side_effect TEXT NOT NULL,
			risk_level TEXT NOT NULL,
			requires_confirmation INTEGER NOT NULL,
			timeout_ms INTEGER NOT NULL,
			retry_policy_json TEXT NOT NULL DEFAULT '{}',
			version TEXT NOT NULL,
			PRIMARY KEY (tenant_id, id)
		)`,
		`CREATE TABLE IF NOT EXISTS connectors (
			tenant_id TEXT NOT NULL,
			id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			business_domain TEXT NOT NULL DEFAULT '',
			owner_team TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'draft',
			base_url TEXT NOT NULL,
			headers_json TEXT NOT NULL DEFAULT '{}',
			auth_json TEXT NOT NULL DEFAULT '{}',
			timeout_ms INTEGER NOT NULL,
			version TEXT NOT NULL,
			PRIMARY KEY (tenant_id, id)
		)`,
		`CREATE TABLE IF NOT EXISTS connector_operations (
			tenant_id TEXT NOT NULL,
			id TEXT NOT NULL,
			connector_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			business_domain TEXT NOT NULL DEFAULT '',
			owner_team TEXT NOT NULL DEFAULT '',
			implementation_mode TEXT NOT NULL DEFAULT 'simple_http',
			type TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'draft',
			base_url TEXT NOT NULL,
			method TEXT NOT NULL,
			path TEXT NOT NULL,
			headers_json TEXT NOT NULL DEFAULT '{}',
			auth_json TEXT NOT NULL DEFAULT '{}',
			input_schema_json TEXT NOT NULL DEFAULT '{}',
			output_schema_json TEXT NOT NULL DEFAULT '{}',
			timeout_ms INTEGER NOT NULL,
			version TEXT NOT NULL,
			PRIMARY KEY (tenant_id, id)
		)`,
	}

	for _, statement := range statements {
		if _, err := r.db.ExecContext(ctx, statement); err != nil {
			return apperrors.Wrap(apperrors.CodeInternal, "failed to migrate sqlite registry", err)
		}
	}

	if err := r.ensureColumn(ctx, "connector_operations", "business_domain", "business_domain TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "connector_operations", "connector_id", "connector_id TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "connector_operations", "owner_team", "owner_team TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "connector_operations", "implementation_mode", "implementation_mode TEXT NOT NULL DEFAULT 'simple_http'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "connector_operations", "status", "status TEXT NOT NULL DEFAULT 'draft'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "connector_operations", "auth_json", "auth_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "connector_operations", "output_schema_json", "output_schema_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "tools", "business_domain", "business_domain TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "tools", "owner_team", "owner_team TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "tools", "status", "status TEXT NOT NULL DEFAULT 'draft'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "tools", "llm_description", "llm_description TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "tools", "retry_policy_json", "retry_policy_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "skills", "business_domain", "business_domain TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "skills", "owner_team", "owner_team TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "skills", "status", "status TEXT NOT NULL DEFAULT 'draft'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "skills", "use_cases_json", "use_cases_json TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "skills", "exclusions_json", "exclusions_json TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "skills", "output_format", "output_format TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "skills", "risk_level", "risk_level TEXT NOT NULL DEFAULT 'low'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "skills", "execution_policy_json", "execution_policy_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agents", "business_domain", "business_domain TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agents", "owner_team", "owner_team TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agents", "status", "status TEXT NOT NULL DEFAULT 'draft'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agents", "tool_ids_json", "tool_ids_json TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agents", "supported_languages_json", "supported_languages_json TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agents", "channels_json", "channels_json TEXT NOT NULL DEFAULT '[]'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agents", "system_prompt", "system_prompt TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agents", "welcome_message", "welcome_message TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agents", "model_config_json", "model_config_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agents", "runtime_policy_json", "runtime_policy_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agent_flows", "business_domain", "business_domain TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agent_flows", "orchestration_mode", "orchestration_mode TEXT NOT NULL DEFAULT 'workflow'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agent_flows", "supervisor_json", "supervisor_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agent_flows", "owner_team", "owner_team TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agent_flows", "status", "status TEXT NOT NULL DEFAULT 'draft'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agent_flows", "graph_json", "graph_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agent_flows", "context_schema_json", "context_schema_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agent_flows", "input_schema_json", "input_schema_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "agent_flows", "output_schema_json", "output_schema_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "workflows", "business_domain", "business_domain TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "workflows", "owner_team", "owner_team TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "workflows", "context_schema_json", "context_schema_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "workflows", "policy_json", "policy_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}
	if err := r.ensureColumn(ctx, "workflows", "ui_json", "ui_json TEXT NOT NULL DEFAULT '{}'"); err != nil {
		return err
	}

	return nil
}

func (r *SQLiteRegistry) ensureColumn(ctx context.Context, tableName, columnName, columnDefinition string) error {
	rows, err := r.db.QueryContext(ctx, "PRAGMA table_info("+tableName+")")
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to inspect sqlite table", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return apperrors.Wrap(apperrors.CodeInternal, "failed to scan sqlite table info", err)
		}
		if name == columnName {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to iterate sqlite table info", err)
	}

	if _, err := r.db.ExecContext(ctx, "ALTER TABLE "+tableName+" ADD COLUMN "+columnDefinition); err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to add sqlite column", err)
	}
	return nil
}

func (r *SQLiteRegistry) SaveAgent(ctx context.Context, profile agent.Profile) error {
	if profile.SkillIDs == nil {
		profile.SkillIDs = []id.ID{}
	}
	if profile.ToolIDs == nil {
		profile.ToolIDs = []id.ID{}
	}
	if profile.SupportedLanguages == nil {
		profile.SupportedLanguages = []string{}
	}
	if profile.Channels == nil {
		profile.Channels = []string{}
	}
	skillIDs, err := encodeJSON(profile.SkillIDs)
	if err != nil {
		return err
	}
	toolIDs, err := encodeJSON(profile.ToolIDs)
	if err != nil {
		return err
	}
	supportedLanguages, err := encodeJSON(profile.SupportedLanguages)
	if err != nil {
		return err
	}
	channels, err := encodeJSON(profile.Channels)
	if err != nil {
		return err
	}
	modelConfig, err := encodeJSON(profile.ModelConfig)
	if err != nil {
		return err
	}
	runtimePolicy, err := encodeJSON(profile.RuntimePolicy)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO agents (
			tenant_id, id, name, description, business_domain, owner_team, status,
			skill_ids_json, tool_ids_json, default_lang, supported_languages_json, channels_json,
			system_prompt, welcome_message, model_config_json, runtime_policy_json, version
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			business_domain = excluded.business_domain,
			owner_team = excluded.owner_team,
			status = excluded.status,
			skill_ids_json = excluded.skill_ids_json,
			tool_ids_json = excluded.tool_ids_json,
			default_lang = excluded.default_lang,
			supported_languages_json = excluded.supported_languages_json,
			channels_json = excluded.channels_json,
			system_prompt = excluded.system_prompt,
			welcome_message = excluded.welcome_message,
			model_config_json = excluded.model_config_json,
			runtime_policy_json = excluded.runtime_policy_json,
			version = excluded.version
	`, profile.TenantID.String(), profile.ID.String(), profile.Name, profile.Description, profile.BusinessDomain, profile.OwnerTeam, profile.Status,
		skillIDs, toolIDs, profile.DefaultLang, supportedLanguages, channels, profile.SystemPrompt, profile.WelcomeMessage, modelConfig, runtimePolicy, profile.Version)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to save agent", err)
	}

	return nil
}

func (r *SQLiteRegistry) GetAgent(ctx context.Context, tenantID tenant.ID, agentID id.ID) (agent.Profile, error) {
	var profile agent.Profile
	var skillIDs, toolIDs, supportedLanguages, channels, modelConfig, runtimePolicy string
	err := r.db.QueryRowContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, status,
			skill_ids_json, tool_ids_json, default_lang, supported_languages_json, channels_json,
			system_prompt, welcome_message, model_config_json, runtime_policy_json, version
		FROM agents
		WHERE tenant_id = ? AND id = ?
	`, tenantID.String(), agentID.String()).Scan(
		&profile.TenantID,
		&profile.ID,
		&profile.Name,
		&profile.Description,
		&profile.BusinessDomain,
		&profile.OwnerTeam,
		&profile.Status,
		&skillIDs,
		&toolIDs,
		&profile.DefaultLang,
		&supportedLanguages,
		&channels,
		&profile.SystemPrompt,
		&profile.WelcomeMessage,
		&modelConfig,
		&runtimePolicy,
		&profile.Version,
	)
	if err != nil {
		return agent.Profile{}, mapSQLError(err, "agent not found")
	}
	if err := decodeJSON(skillIDs, &profile.SkillIDs); err != nil {
		return agent.Profile{}, err
	}
	if err := decodeJSON(toolIDs, &profile.ToolIDs); err != nil {
		return agent.Profile{}, err
	}
	if err := decodeJSON(supportedLanguages, &profile.SupportedLanguages); err != nil {
		return agent.Profile{}, err
	}
	if err := decodeJSON(channels, &profile.Channels); err != nil {
		return agent.Profile{}, err
	}
	if err := decodeJSON(modelConfig, &profile.ModelConfig); err != nil {
		return agent.Profile{}, err
	}
	if err := decodeJSON(runtimePolicy, &profile.RuntimePolicy); err != nil {
		return agent.Profile{}, err
	}

	return profile, nil
}

func (r *SQLiteRegistry) ListAgents(ctx context.Context, tenantID tenant.ID) ([]agent.Profile, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, status,
			skill_ids_json, tool_ids_json, default_lang, supported_languages_json, channels_json,
			system_prompt, welcome_message, model_config_json, runtime_policy_json, version
		FROM agents
		WHERE tenant_id = ?
		ORDER BY id
	`, tenantID.String())
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to list agents", err)
	}
	defer rows.Close()

	var result []agent.Profile
	for rows.Next() {
		var profile agent.Profile
		var skillIDs, toolIDs, supportedLanguages, channels, modelConfig, runtimePolicy string
		if err := rows.Scan(
			&profile.TenantID,
			&profile.ID,
			&profile.Name,
			&profile.Description,
			&profile.BusinessDomain,
			&profile.OwnerTeam,
			&profile.Status,
			&skillIDs,
			&toolIDs,
			&profile.DefaultLang,
			&supportedLanguages,
			&channels,
			&profile.SystemPrompt,
			&profile.WelcomeMessage,
			&modelConfig,
			&runtimePolicy,
			&profile.Version,
		); err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to scan agent", err)
		}
		if err := decodeJSON(skillIDs, &profile.SkillIDs); err != nil {
			return nil, err
		}
		if err := decodeJSON(toolIDs, &profile.ToolIDs); err != nil {
			return nil, err
		}
		if err := decodeJSON(supportedLanguages, &profile.SupportedLanguages); err != nil {
			return nil, err
		}
		if err := decodeJSON(channels, &profile.Channels); err != nil {
			return nil, err
		}
		if err := decodeJSON(modelConfig, &profile.ModelConfig); err != nil {
			return nil, err
		}
		if err := decodeJSON(runtimePolicy, &profile.RuntimePolicy); err != nil {
			return nil, err
		}
		result = append(result, profile)
	}

	return result, rows.Err()
}

func (r *SQLiteRegistry) SaveAgentFlow(ctx context.Context, spec agentflow.Spec) error {
	if spec.ContextSchema == nil {
		spec.ContextSchema = map[string]any{}
	}
	if spec.InputSchema == nil {
		spec.InputSchema = map[string]any{}
	}
	if spec.OutputSchema == nil {
		spec.OutputSchema = map[string]any{}
	}
	supervisor, err := encodeJSON(spec.Supervisor)
	if err != nil {
		return err
	}
	graph, err := encodeJSON(spec.Graph)
	if err != nil {
		return err
	}
	contextSchema, err := encodeJSON(spec.ContextSchema)
	if err != nil {
		return err
	}
	inputSchema, err := encodeJSON(spec.InputSchema)
	if err != nil {
		return err
	}
	outputSchema, err := encodeJSON(spec.OutputSchema)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO agent_flows (
			tenant_id, id, name, description, business_domain, owner_team, status,
			orchestration_mode, supervisor_json, graph_json, context_schema_json, input_schema_json, output_schema_json, version
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			business_domain = excluded.business_domain,
			owner_team = excluded.owner_team,
			status = excluded.status,
			orchestration_mode = excluded.orchestration_mode,
			supervisor_json = excluded.supervisor_json,
			graph_json = excluded.graph_json,
			context_schema_json = excluded.context_schema_json,
			input_schema_json = excluded.input_schema_json,
			output_schema_json = excluded.output_schema_json,
			version = excluded.version
	`, spec.TenantID.String(), spec.ID.String(), spec.Name, spec.Description, spec.BusinessDomain, spec.OwnerTeam, spec.Status, spec.OrchestrationMode, supervisor, graph, contextSchema, inputSchema, outputSchema, spec.Version)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to save agent flow", err)
	}

	return nil
}

func (r *SQLiteRegistry) GetAgentFlow(ctx context.Context, tenantID tenant.ID, flowID id.ID) (agentflow.Spec, error) {
	var spec agentflow.Spec
	var supervisor, graph, contextSchema, inputSchema, outputSchema string
	err := r.db.QueryRowContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, status,
			orchestration_mode, supervisor_json, graph_json, context_schema_json, input_schema_json, output_schema_json, version
		FROM agent_flows
		WHERE tenant_id = ? AND id = ?
	`, tenantID.String(), flowID.String()).Scan(
		&spec.TenantID,
		&spec.ID,
		&spec.Name,
		&spec.Description,
		&spec.BusinessDomain,
		&spec.OwnerTeam,
		&spec.Status,
		&spec.OrchestrationMode,
		&supervisor,
		&graph,
		&contextSchema,
		&inputSchema,
		&outputSchema,
		&spec.Version,
	)
	if err != nil {
		return agentflow.Spec{}, mapSQLError(err, "agent flow not found")
	}
	if err := decodeJSON(supervisor, &spec.Supervisor); err != nil {
		return agentflow.Spec{}, err
	}
	if err := decodeJSON(graph, &spec.Graph); err != nil {
		return agentflow.Spec{}, err
	}
	if err := decodeJSON(contextSchema, &spec.ContextSchema); err != nil {
		return agentflow.Spec{}, err
	}
	if err := decodeJSON(inputSchema, &spec.InputSchema); err != nil {
		return agentflow.Spec{}, err
	}
	if err := decodeJSON(outputSchema, &spec.OutputSchema); err != nil {
		return agentflow.Spec{}, err
	}

	return spec, nil
}

func (r *SQLiteRegistry) ListAgentFlows(ctx context.Context, tenantID tenant.ID) ([]agentflow.Spec, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, status,
			orchestration_mode, supervisor_json, graph_json, context_schema_json, input_schema_json, output_schema_json, version
		FROM agent_flows
		WHERE tenant_id = ?
		ORDER BY id
	`, tenantID.String())
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to list agent flows", err)
	}
	defer rows.Close()

	var result []agentflow.Spec
	for rows.Next() {
		var spec agentflow.Spec
		var supervisor, graph, contextSchema, inputSchema, outputSchema string
		if err := rows.Scan(
			&spec.TenantID,
			&spec.ID,
			&spec.Name,
			&spec.Description,
			&spec.BusinessDomain,
			&spec.OwnerTeam,
			&spec.Status,
			&spec.OrchestrationMode,
			&supervisor,
			&graph,
			&contextSchema,
			&inputSchema,
			&outputSchema,
			&spec.Version,
		); err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to scan agent flow", err)
		}
		if err := decodeJSON(supervisor, &spec.Supervisor); err != nil {
			return nil, err
		}
		if err := decodeJSON(graph, &spec.Graph); err != nil {
			return nil, err
		}
		if err := decodeJSON(contextSchema, &spec.ContextSchema); err != nil {
			return nil, err
		}
		if err := decodeJSON(inputSchema, &spec.InputSchema); err != nil {
			return nil, err
		}
		if err := decodeJSON(outputSchema, &spec.OutputSchema); err != nil {
			return nil, err
		}
		result = append(result, spec)
	}

	return result, rows.Err()
}

func (r *SQLiteRegistry) SaveWorkflow(ctx context.Context, spec workflow.Spec) error {
	if spec.ContextSchema == nil {
		spec.ContextSchema = map[string]any{}
	}
	if spec.InputSchema == nil {
		spec.InputSchema = map[string]any{}
	}
	if spec.OutputSchema == nil {
		spec.OutputSchema = map[string]any{}
	}
	if spec.UI == nil {
		spec.UI = map[string]any{}
	}
	contextSchema, err := encodeJSON(spec.ContextSchema)
	if err != nil {
		return err
	}
	inputSchema, err := encodeJSON(spec.InputSchema)
	if err != nil {
		return err
	}
	outputSchema, err := encodeJSON(spec.OutputSchema)
	if err != nil {
		return err
	}
	graph, err := encodeJSON(spec.Graph)
	if err != nil {
		return err
	}
	policy, err := encodeJSON(spec.Policy)
	if err != nil {
		return err
	}
	ui, err := encodeJSON(spec.UI)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO workflows (
			tenant_id, id, name, description, business_domain, owner_team, status, profile,
			context_schema_json, input_schema_json, output_schema_json, graph_json, policy_json, ui_json, version
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			business_domain = excluded.business_domain,
			owner_team = excluded.owner_team,
			status = excluded.status,
			profile = excluded.profile,
			context_schema_json = excluded.context_schema_json,
			input_schema_json = excluded.input_schema_json,
			output_schema_json = excluded.output_schema_json,
			graph_json = excluded.graph_json,
			policy_json = excluded.policy_json,
			ui_json = excluded.ui_json,
			version = excluded.version
	`, spec.TenantID.String(), spec.ID.String(), spec.Name, spec.Description, spec.BusinessDomain, spec.OwnerTeam, spec.Status, spec.Profile, contextSchema, inputSchema, outputSchema, graph, policy, ui, spec.Version)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to save workflow", err)
	}
	return nil
}

func (r *SQLiteRegistry) GetWorkflow(ctx context.Context, tenantID tenant.ID, workflowID id.ID) (workflow.Spec, error) {
	var spec workflow.Spec
	var contextSchema, inputSchema, outputSchema, graph, policy, ui string
	err := r.db.QueryRowContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, status, profile,
			context_schema_json, input_schema_json, output_schema_json, graph_json, policy_json, ui_json, version
		FROM workflows
		WHERE tenant_id = ? AND id = ?
	`, tenantID.String(), workflowID.String()).Scan(
		&spec.TenantID,
		&spec.ID,
		&spec.Name,
		&spec.Description,
		&spec.BusinessDomain,
		&spec.OwnerTeam,
		&spec.Status,
		&spec.Profile,
		&contextSchema,
		&inputSchema,
		&outputSchema,
		&graph,
		&policy,
		&ui,
		&spec.Version,
	)
	if err != nil {
		return workflow.Spec{}, mapSQLError(err, "workflow not found")
	}
	if err := decodeJSON(contextSchema, &spec.ContextSchema); err != nil {
		return workflow.Spec{}, err
	}
	if err := decodeJSON(inputSchema, &spec.InputSchema); err != nil {
		return workflow.Spec{}, err
	}
	if err := decodeJSON(outputSchema, &spec.OutputSchema); err != nil {
		return workflow.Spec{}, err
	}
	if err := decodeJSON(graph, &spec.Graph); err != nil {
		return workflow.Spec{}, err
	}
	if err := decodeJSON(policy, &spec.Policy); err != nil {
		return workflow.Spec{}, err
	}
	if err := decodeJSON(ui, &spec.UI); err != nil {
		return workflow.Spec{}, err
	}
	return spec, nil
}

func (r *SQLiteRegistry) ListWorkflows(ctx context.Context, tenantID tenant.ID) ([]workflow.Spec, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, status, profile,
			context_schema_json, input_schema_json, output_schema_json, graph_json, policy_json, ui_json, version
		FROM workflows
		WHERE tenant_id = ?
		ORDER BY id
	`, tenantID.String())
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to list workflows", err)
	}
	defer rows.Close()

	var result []workflow.Spec
	for rows.Next() {
		var spec workflow.Spec
		var contextSchema, inputSchema, outputSchema, graph, policy, ui string
		if err := rows.Scan(
			&spec.TenantID,
			&spec.ID,
			&spec.Name,
			&spec.Description,
			&spec.BusinessDomain,
			&spec.OwnerTeam,
			&spec.Status,
			&spec.Profile,
			&contextSchema,
			&inputSchema,
			&outputSchema,
			&graph,
			&policy,
			&ui,
			&spec.Version,
		); err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to scan workflow", err)
		}
		if err := decodeJSON(contextSchema, &spec.ContextSchema); err != nil {
			return nil, err
		}
		if err := decodeJSON(inputSchema, &spec.InputSchema); err != nil {
			return nil, err
		}
		if err := decodeJSON(outputSchema, &spec.OutputSchema); err != nil {
			return nil, err
		}
		if err := decodeJSON(graph, &spec.Graph); err != nil {
			return nil, err
		}
		if err := decodeJSON(policy, &spec.Policy); err != nil {
			return nil, err
		}
		if err := decodeJSON(ui, &spec.UI); err != nil {
			return nil, err
		}
		result = append(result, spec)
	}
	return result, rows.Err()
}

func (r *SQLiteRegistry) SaveTool(ctx context.Context, spec tool.Spec) error {
	if spec.InputSchema == nil {
		spec.InputSchema = map[string]any{}
	}
	if spec.OutputSchema == nil {
		spec.OutputSchema = map[string]any{}
	}
	binding, err := encodeJSON(spec.Binding)
	if err != nil {
		return err
	}
	inputSchema, err := encodeJSON(spec.InputSchema)
	if err != nil {
		return err
	}
	outputSchema, err := encodeJSON(spec.OutputSchema)
	if err != nil {
		return err
	}
	retryPolicy, err := encodeJSON(spec.RetryPolicy)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO tools (
			tenant_id, id, name, description, business_domain, owner_team, status, llm_description,
			implementation, binding_json, input_schema_json, output_schema_json, side_effect, risk_level,
			requires_confirmation, timeout_ms, retry_policy_json, version
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			business_domain = excluded.business_domain,
			owner_team = excluded.owner_team,
			status = excluded.status,
			llm_description = excluded.llm_description,
			implementation = excluded.implementation,
			binding_json = excluded.binding_json,
			input_schema_json = excluded.input_schema_json,
			output_schema_json = excluded.output_schema_json,
			side_effect = excluded.side_effect,
			risk_level = excluded.risk_level,
			requires_confirmation = excluded.requires_confirmation,
			timeout_ms = excluded.timeout_ms,
			retry_policy_json = excluded.retry_policy_json,
			version = excluded.version
	`, spec.TenantID.String(), spec.ID.String(), spec.Name, spec.Description, spec.BusinessDomain, spec.OwnerTeam, spec.Status, spec.LLMDescription, spec.Implementation, binding, inputSchema, outputSchema, spec.SideEffect, spec.RiskLevel, boolToInt(spec.RequiresConfirmation), spec.TimeoutMillis, retryPolicy, spec.Version)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to save tool", err)
	}

	return nil
}

func (r *SQLiteRegistry) GetTool(ctx context.Context, tenantID tenant.ID, toolID id.ID) (tool.Spec, error) {
	var spec tool.Spec
	var binding, inputSchema, outputSchema, retryPolicy string
	var requiresConfirmation int
	err := r.db.QueryRowContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, status, llm_description,
			implementation, binding_json, input_schema_json, output_schema_json, side_effect, risk_level,
			requires_confirmation, timeout_ms, retry_policy_json, version
		FROM tools
		WHERE tenant_id = ? AND id = ?
	`, tenantID.String(), toolID.String()).Scan(
		&spec.TenantID,
		&spec.ID,
		&spec.Name,
		&spec.Description,
		&spec.BusinessDomain,
		&spec.OwnerTeam,
		&spec.Status,
		&spec.LLMDescription,
		&spec.Implementation,
		&binding,
		&inputSchema,
		&outputSchema,
		&spec.SideEffect,
		&spec.RiskLevel,
		&requiresConfirmation,
		&spec.TimeoutMillis,
		&retryPolicy,
		&spec.Version,
	)
	if err != nil {
		return tool.Spec{}, mapSQLError(err, "tool not found")
	}
	spec.RequiresConfirmation = requiresConfirmation == 1
	if err := decodeJSON(binding, &spec.Binding); err != nil {
		return tool.Spec{}, err
	}
	if err := decodeJSON(inputSchema, &spec.InputSchema); err != nil {
		return tool.Spec{}, err
	}
	if err := decodeJSON(outputSchema, &spec.OutputSchema); err != nil {
		return tool.Spec{}, err
	}
	if err := decodeJSON(retryPolicy, &spec.RetryPolicy); err != nil {
		return tool.Spec{}, err
	}

	return spec, nil
}

func (r *SQLiteRegistry) ListTools(ctx context.Context, tenantID tenant.ID) ([]tool.Spec, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, status, llm_description,
			implementation, binding_json, input_schema_json, output_schema_json, side_effect, risk_level,
			requires_confirmation, timeout_ms, retry_policy_json, version
		FROM tools
		WHERE tenant_id = ?
		ORDER BY id
	`, tenantID.String())
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to list tools", err)
	}
	defer rows.Close()

	var result []tool.Spec
	for rows.Next() {
		var spec tool.Spec
		var binding, inputSchema, outputSchema, retryPolicy string
		var requiresConfirmation int
		if err := rows.Scan(&spec.TenantID, &spec.ID, &spec.Name, &spec.Description, &spec.BusinessDomain, &spec.OwnerTeam, &spec.Status, &spec.LLMDescription, &spec.Implementation, &binding, &inputSchema, &outputSchema, &spec.SideEffect, &spec.RiskLevel, &requiresConfirmation, &spec.TimeoutMillis, &retryPolicy, &spec.Version); err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to scan tool", err)
		}
		spec.RequiresConfirmation = requiresConfirmation == 1
		if err := decodeJSON(binding, &spec.Binding); err != nil {
			return nil, err
		}
		if err := decodeJSON(inputSchema, &spec.InputSchema); err != nil {
			return nil, err
		}
		if err := decodeJSON(outputSchema, &spec.OutputSchema); err != nil {
			return nil, err
		}
		if err := decodeJSON(retryPolicy, &spec.RetryPolicy); err != nil {
			return nil, err
		}
		result = append(result, spec)
	}

	return result, rows.Err()
}

func (r *SQLiteRegistry) SaveSkill(ctx context.Context, spec skill.Spec) error {
	if spec.ToolIDs == nil {
		spec.ToolIDs = []id.ID{}
	}
	if spec.KnowledgeIDs == nil {
		spec.KnowledgeIDs = []id.ID{}
	}
	if spec.UseCases == nil {
		spec.UseCases = []string{}
	}
	if spec.Exclusions == nil {
		spec.Exclusions = []string{}
	}
	toolIDs, err := encodeJSON(spec.ToolIDs)
	if err != nil {
		return err
	}
	knowledgeIDs, err := encodeJSON(spec.KnowledgeIDs)
	if err != nil {
		return err
	}
	useCases, err := encodeJSON(spec.UseCases)
	if err != nil {
		return err
	}
	exclusions, err := encodeJSON(spec.Exclusions)
	if err != nil {
		return err
	}
	executionPolicy, err := encodeJSON(spec.ExecutionPolicy)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO skills (
			tenant_id, id, name, description, business_domain, owner_team, status,
			tool_ids_json, knowledge_ids_json, system_prompt, use_cases_json, exclusions_json,
			output_format, risk_level, execution_policy_json, policy_version, version
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			business_domain = excluded.business_domain,
			owner_team = excluded.owner_team,
			status = excluded.status,
			tool_ids_json = excluded.tool_ids_json,
			knowledge_ids_json = excluded.knowledge_ids_json,
			system_prompt = excluded.system_prompt,
			use_cases_json = excluded.use_cases_json,
			exclusions_json = excluded.exclusions_json,
			output_format = excluded.output_format,
			risk_level = excluded.risk_level,
			execution_policy_json = excluded.execution_policy_json,
			policy_version = excluded.policy_version,
			version = excluded.version
	`, spec.TenantID.String(), spec.ID.String(), spec.Name, spec.Description, spec.BusinessDomain, spec.OwnerTeam, spec.Status,
		toolIDs, knowledgeIDs, spec.SystemPrompt, useCases, exclusions, spec.OutputFormat, spec.RiskLevel, executionPolicy,
		spec.PolicyVersion, spec.Version)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to save skill", err)
	}

	return nil
}

func (r *SQLiteRegistry) GetSkill(ctx context.Context, tenantID tenant.ID, skillID id.ID) (skill.Spec, error) {
	var spec skill.Spec
	var toolIDs, knowledgeIDs, useCases, exclusions, executionPolicy string
	err := r.db.QueryRowContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, status,
			tool_ids_json, knowledge_ids_json, system_prompt, use_cases_json, exclusions_json,
			output_format, risk_level, execution_policy_json, policy_version, version
		FROM skills
		WHERE tenant_id = ? AND id = ?
	`, tenantID.String(), skillID.String()).Scan(&spec.TenantID, &spec.ID, &spec.Name, &spec.Description, &spec.BusinessDomain, &spec.OwnerTeam, &spec.Status,
		&toolIDs, &knowledgeIDs, &spec.SystemPrompt, &useCases, &exclusions, &spec.OutputFormat, &spec.RiskLevel, &executionPolicy,
		&spec.PolicyVersion, &spec.Version)
	if err != nil {
		return skill.Spec{}, mapSQLError(err, "skill not found")
	}
	if err := decodeJSON(toolIDs, &spec.ToolIDs); err != nil {
		return skill.Spec{}, err
	}
	if err := decodeJSON(knowledgeIDs, &spec.KnowledgeIDs); err != nil {
		return skill.Spec{}, err
	}
	if err := decodeJSON(useCases, &spec.UseCases); err != nil {
		return skill.Spec{}, err
	}
	if err := decodeJSON(exclusions, &spec.Exclusions); err != nil {
		return skill.Spec{}, err
	}
	if err := decodeJSON(executionPolicy, &spec.ExecutionPolicy); err != nil {
		return skill.Spec{}, err
	}

	return spec, nil
}

func (r *SQLiteRegistry) ListSkills(ctx context.Context, tenantID tenant.ID) ([]skill.Spec, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, status,
			tool_ids_json, knowledge_ids_json, system_prompt, use_cases_json, exclusions_json,
			output_format, risk_level, execution_policy_json, policy_version, version
		FROM skills
		WHERE tenant_id = ?
		ORDER BY id
	`, tenantID.String())
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to list skills", err)
	}
	defer rows.Close()

	var result []skill.Spec
	for rows.Next() {
		var spec skill.Spec
		var toolIDs, knowledgeIDs, useCases, exclusions, executionPolicy string
		if err := rows.Scan(&spec.TenantID, &spec.ID, &spec.Name, &spec.Description, &spec.BusinessDomain, &spec.OwnerTeam, &spec.Status,
			&toolIDs, &knowledgeIDs, &spec.SystemPrompt, &useCases, &exclusions, &spec.OutputFormat, &spec.RiskLevel, &executionPolicy,
			&spec.PolicyVersion, &spec.Version); err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to scan skill", err)
		}
		if err := decodeJSON(toolIDs, &spec.ToolIDs); err != nil {
			return nil, err
		}
		if err := decodeJSON(knowledgeIDs, &spec.KnowledgeIDs); err != nil {
			return nil, err
		}
		if err := decodeJSON(useCases, &spec.UseCases); err != nil {
			return nil, err
		}
		if err := decodeJSON(exclusions, &spec.Exclusions); err != nil {
			return nil, err
		}
		if err := decodeJSON(executionPolicy, &spec.ExecutionPolicy); err != nil {
			return nil, err
		}
		result = append(result, spec)
	}

	return result, rows.Err()
}

func (r *SQLiteRegistry) SaveConnector(ctx context.Context, spec connector.Spec) error {
	if spec.Headers == nil {
		spec.Headers = map[string]string{}
	}
	headers, err := encodeJSON(spec.Headers)
	if err != nil {
		return err
	}
	auth, err := encodeJSON(spec.Auth)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO connectors (
			tenant_id, id, name, description, business_domain, owner_team, type, status, base_url, headers_json, auth_json, timeout_ms, version
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			business_domain = excluded.business_domain,
			owner_team = excluded.owner_team,
			type = excluded.type,
			status = excluded.status,
			base_url = excluded.base_url,
			headers_json = excluded.headers_json,
			auth_json = excluded.auth_json,
			timeout_ms = excluded.timeout_ms,
			version = excluded.version
	`, spec.TenantID.String(), spec.ID.String(), spec.Name, spec.Description, spec.BusinessDomain, spec.OwnerTeam, spec.Type, spec.Status, spec.BaseURL, headers, auth, spec.TimeoutMillis, spec.Version)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to save connector", err)
	}

	return nil
}

func (r *SQLiteRegistry) GetConnector(ctx context.Context, tenantID tenant.ID, connectorID id.ID) (connector.Spec, error) {
	var spec connector.Spec
	var headers, auth string
	err := r.db.QueryRowContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, type, status, base_url, headers_json, auth_json, timeout_ms, version
		FROM connectors
		WHERE tenant_id = ? AND id = ?
	`, tenantID.String(), connectorID.String()).Scan(&spec.TenantID, &spec.ID, &spec.Name, &spec.Description, &spec.BusinessDomain, &spec.OwnerTeam, &spec.Type, &spec.Status, &spec.BaseURL, &headers, &auth, &spec.TimeoutMillis, &spec.Version)
	if err != nil {
		return connector.Spec{}, mapSQLError(err, "connector not found")
	}
	if err := decodeJSON(headers, &spec.Headers); err != nil {
		return connector.Spec{}, err
	}
	if err := decodeJSON(auth, &spec.Auth); err != nil {
		return connector.Spec{}, err
	}

	return spec, nil
}

func (r *SQLiteRegistry) ListConnectors(ctx context.Context, tenantID tenant.ID) ([]connector.Spec, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT tenant_id, id, name, description, business_domain, owner_team, type, status, base_url, headers_json, auth_json, timeout_ms, version
		FROM connectors
		WHERE tenant_id = ?
		ORDER BY id
	`, tenantID.String())
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to list connectors", err)
	}
	defer rows.Close()

	var result []connector.Spec
	for rows.Next() {
		var spec connector.Spec
		var headers, auth string
		if err := rows.Scan(&spec.TenantID, &spec.ID, &spec.Name, &spec.Description, &spec.BusinessDomain, &spec.OwnerTeam, &spec.Type, &spec.Status, &spec.BaseURL, &headers, &auth, &spec.TimeoutMillis, &spec.Version); err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to scan connector", err)
		}
		if err := decodeJSON(headers, &spec.Headers); err != nil {
			return nil, err
		}
		if err := decodeJSON(auth, &spec.Auth); err != nil {
			return nil, err
		}
		result = append(result, spec)
	}

	return result, rows.Err()
}

func (r *SQLiteRegistry) SaveConnectorOperation(ctx context.Context, spec connector.OperationSpec) error {
	if spec.Headers == nil {
		spec.Headers = map[string]string{}
	}
	if spec.InputSchema == nil {
		spec.InputSchema = map[string]any{}
	}
	if spec.OutputSchema == nil {
		spec.OutputSchema = map[string]any{}
	}
	headers, err := encodeJSON(spec.Headers)
	if err != nil {
		return err
	}
	auth, err := encodeJSON(spec.Auth)
	if err != nil {
		return err
	}
	inputSchema, err := encodeJSON(spec.InputSchema)
	if err != nil {
		return err
	}
	outputSchema, err := encodeJSON(spec.OutputSchema)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO connector_operations (
			tenant_id, id, connector_id, name, description, business_domain, owner_team, implementation_mode, type, status, base_url, method, path, headers_json,
			auth_json, input_schema_json, output_schema_json, timeout_ms, version
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(tenant_id, id) DO UPDATE SET
			connector_id = excluded.connector_id,
			name = excluded.name,
			description = excluded.description,
			business_domain = excluded.business_domain,
			owner_team = excluded.owner_team,
			implementation_mode = excluded.implementation_mode,
			type = excluded.type,
			status = excluded.status,
			base_url = excluded.base_url,
			method = excluded.method,
			path = excluded.path,
			headers_json = excluded.headers_json,
			auth_json = excluded.auth_json,
			input_schema_json = excluded.input_schema_json,
			output_schema_json = excluded.output_schema_json,
			timeout_ms = excluded.timeout_ms,
			version = excluded.version
	`, spec.TenantID.String(), spec.ID.String(), spec.ConnectorID.String(), spec.Name, spec.Description, spec.BusinessDomain, spec.OwnerTeam, spec.ImplementationMode, spec.Type, spec.Status, spec.BaseURL, spec.Method, spec.Path, headers, auth, inputSchema, outputSchema, spec.TimeoutMillis, spec.Version)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to save connector operation", err)
	}

	return nil
}

func (r *SQLiteRegistry) GetConnectorOperation(ctx context.Context, tenantID tenant.ID, operationID id.ID) (connector.OperationSpec, error) {
	var spec connector.OperationSpec
	var headers, auth, inputSchema, outputSchema string
	err := r.db.QueryRowContext(ctx, `
		SELECT tenant_id, id, connector_id, name, description, business_domain, owner_team, implementation_mode, type, status, base_url, method, path, headers_json,
			auth_json, input_schema_json, output_schema_json, timeout_ms, version
		FROM connector_operations
		WHERE tenant_id = ? AND id = ?
	`, tenantID.String(), operationID.String()).Scan(&spec.TenantID, &spec.ID, &spec.ConnectorID, &spec.Name, &spec.Description, &spec.BusinessDomain, &spec.OwnerTeam, &spec.ImplementationMode, &spec.Type, &spec.Status, &spec.BaseURL, &spec.Method, &spec.Path, &headers, &auth, &inputSchema, &outputSchema, &spec.TimeoutMillis, &spec.Version)
	if err != nil {
		return connector.OperationSpec{}, mapSQLError(err, "connector operation not found")
	}
	if err := decodeJSON(headers, &spec.Headers); err != nil {
		return connector.OperationSpec{}, err
	}
	if err := decodeJSON(auth, &spec.Auth); err != nil {
		return connector.OperationSpec{}, err
	}
	if err := decodeJSON(inputSchema, &spec.InputSchema); err != nil {
		return connector.OperationSpec{}, err
	}
	if err := decodeJSON(outputSchema, &spec.OutputSchema); err != nil {
		return connector.OperationSpec{}, err
	}

	return spec, nil
}

func (r *SQLiteRegistry) ListConnectorOperations(ctx context.Context, tenantID tenant.ID) ([]connector.OperationSpec, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT tenant_id, id, connector_id, name, description, business_domain, owner_team, implementation_mode, type, status, base_url, method, path, headers_json,
			auth_json, input_schema_json, output_schema_json, timeout_ms, version
		FROM connector_operations
		WHERE tenant_id = ?
		ORDER BY id
	`, tenantID.String())
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to list connector operations", err)
	}
	defer rows.Close()

	var result []connector.OperationSpec
	for rows.Next() {
		var spec connector.OperationSpec
		var headers, auth, inputSchema, outputSchema string
		if err := rows.Scan(&spec.TenantID, &spec.ID, &spec.ConnectorID, &spec.Name, &spec.Description, &spec.BusinessDomain, &spec.OwnerTeam, &spec.ImplementationMode, &spec.Type, &spec.Status, &spec.BaseURL, &spec.Method, &spec.Path, &headers, &auth, &inputSchema, &outputSchema, &spec.TimeoutMillis, &spec.Version); err != nil {
			return nil, apperrors.Wrap(apperrors.CodeInternal, "failed to scan connector operation", err)
		}
		if err := decodeJSON(headers, &spec.Headers); err != nil {
			return nil, err
		}
		if err := decodeJSON(auth, &spec.Auth); err != nil {
			return nil, err
		}
		if err := decodeJSON(inputSchema, &spec.InputSchema); err != nil {
			return nil, err
		}
		if err := decodeJSON(outputSchema, &spec.OutputSchema); err != nil {
			return nil, err
		}
		result = append(result, spec)
	}

	return result, rows.Err()
}

func encodeJSON(value any) (string, error) {
	bytes, err := json.Marshal(value)
	if err != nil {
		return "", apperrors.Wrap(apperrors.CodeInvalidArgument, "failed to encode json field", err)
	}
	return string(bytes), nil
}

func decodeJSON(value string, out any) error {
	if value == "" {
		value = "{}"
	}
	if err := json.Unmarshal([]byte(value), out); err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, "failed to decode json field", err)
	}
	return nil
}

func mapSQLError(err error, notFoundMessage string) error {
	if err == sql.ErrNoRows {
		return apperrors.New(apperrors.CodeNotFound, notFoundMessage)
	}
	return apperrors.Wrap(apperrors.CodeInternal, fmt.Sprintf("sqlite query failed: %s", notFoundMessage), err)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
