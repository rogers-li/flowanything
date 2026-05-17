package config

import (
	"fmt"
	"strconv"
	"time"

	"flow-anything/core/agentcore"
	"flow-anything/core/connector"
	coreprompt "flow-anything/core/prompt"
	"flow-anything/core/tools"
	"flow-anything/core/workflow"
)

type RuntimeCatalog struct {
	Bundle              BundleSpec
	Index               Index
	Agents              map[string]agentcore.AgentSpec
	Skills              map[string]SkillConfig
	Tools               map[string]tools.ToolSpec
	Workflows           map[string]workflow.WorkflowDocument
	Connectors          map[string]connector.ConnectorSpec
	ConnectorOperations map[string]connector.OperationSpec
	Models              map[string]agentcore.ModelConfig
	KnowledgeBases      map[string]KnowledgeConfig
	Policies            map[string]PolicyConfig
}

// CompileRuntimeCatalog validates BundleSpec and converts config resources into
// the runtime-neutral core specs consumed by agentcore, tools, connector, and
// workflow packages. Host applications still provide concrete adapters such as
// model clients, protocol executors, secret resolvers, and local device APIs.
func CompileRuntimeCatalog(bundle BundleSpec) (RuntimeCatalog, error) {
	normalized := NormalizeBundle(bundle)
	if err := ValidateBundle(normalized); err != nil {
		return RuntimeCatalog{}, err
	}
	index, err := BuildIndex(normalized)
	if err != nil {
		return RuntimeCatalog{}, err
	}
	catalog := RuntimeCatalog{
		Bundle:              normalized,
		Index:               index,
		Agents:              map[string]agentcore.AgentSpec{},
		Skills:              map[string]SkillConfig{},
		Tools:               map[string]tools.ToolSpec{},
		Workflows:           map[string]workflow.WorkflowDocument{},
		Connectors:          map[string]connector.ConnectorSpec{},
		ConnectorOperations: map[string]connector.OperationSpec{},
		Models:              map[string]agentcore.ModelConfig{},
		KnowledgeBases:      map[string]KnowledgeConfig{},
		Policies:            map[string]PolicyConfig{},
	}
	for _, model := range normalized.Resources.Models {
		if model.Disabled {
			continue
		}
		catalog.Models[model.ID] = toAgentModelConfig(model)
	}
	for _, connectorConfig := range normalized.Resources.Connectors {
		if connectorConfig.Disabled {
			continue
		}
		catalog.Connectors[connectorConfig.ID] = toConnectorSpec(connectorConfig)
		for _, operation := range connectorConfig.Operations {
			if operation.Disabled {
				continue
			}
			catalog.ConnectorOperations[operation.ID] = toConnectorOperationSpec(connectorConfig.ID, operation)
		}
	}
	for _, tool := range normalized.Resources.Tools {
		if tool.Disabled {
			continue
		}
		runtimeTool, err := toToolSpec(tool)
		if err != nil {
			return RuntimeCatalog{}, fmt.Errorf("compile tool %q: %w", tool.ID, err)
		}
		catalog.Tools[tool.ID] = runtimeTool
	}
	for _, workflowConfig := range normalized.Resources.Workflows {
		if workflowConfig.Disabled {
			continue
		}
		catalog.Workflows[workflowConfig.ID] = toWorkflowDocument(workflowConfig)
	}
	for _, skill := range normalized.Resources.Skills {
		if skill.Disabled {
			continue
		}
		catalog.Skills[skill.ID] = skill
	}
	for _, knowledge := range normalized.Resources.KnowledgeBases {
		if knowledge.Disabled {
			continue
		}
		catalog.KnowledgeBases[knowledge.ID] = knowledge
	}
	for _, policy := range normalized.Resources.Policies {
		if policy.Disabled {
			continue
		}
		catalog.Policies[policy.ID] = policy
	}
	for _, agent := range normalized.Resources.Agents {
		if agent.Disabled {
			continue
		}
		catalog.Agents[agent.ID] = toAgentSpec(agent, catalog)
	}
	return catalog, nil
}

func toAgentSpec(agent AgentConfig, catalog RuntimeCatalog) agentcore.AgentSpec {
	model := catalog.Models[agent.ModelRef.ID]
	return agentcore.AgentSpec{
		ID:            agent.ID,
		Name:          agent.Name,
		Description:   agent.Description,
		Prompt:        BuildPromptText(agent.Prompt),
		ReasoningMode: agent.Reasoning.Mode,
		Model:         model,
		Capabilities:  capabilityDescriptors(agent, catalog),
		OutputSchema:  toAgentSchema(agent.OutputSchema),
	}
}

func capabilityDescriptors(agent AgentConfig, catalog RuntimeCatalog) []agentcore.CapabilityDescriptor {
	out := []agentcore.CapabilityDescriptor{}
	for _, binding := range agent.Skills {
		if binding.Disabled {
			continue
		}
		if skill, ok := catalog.Skills[binding.Ref.ID]; ok {
			out = append(out, agentcore.CapabilityDescriptor{
				ID:           skill.ID,
				Type:         string(ResourceSkill),
				Name:         skill.Name,
				Description:  skill.Description,
				InputSchema:  toAgentSchema(skill.InputSchema),
				OutputSchema: toAgentSchema(skill.OutputSchema),
			})
		}
	}
	for _, binding := range agent.Tools {
		if binding.Disabled {
			continue
		}
		if tool, ok := catalog.Tools[binding.Ref.ID]; ok {
			out = append(out, agentcore.CapabilityDescriptor{
				ID:           tool.ID,
				Type:         string(ResourceTool),
				Name:         tool.Name,
				Description:  tool.Description,
				InputSchema:  toAgentSchemaFromTool(tool.InputSchema),
				OutputSchema: toAgentSchemaFromTool(tool.OutputSchema),
			})
		}
	}
	for _, binding := range agent.Workflows {
		if binding.Disabled {
			continue
		}
		if workflowDocument, ok := catalog.Workflows[binding.Ref.ID]; ok {
			out = append(out, agentcore.CapabilityDescriptor{
				ID:          workflowDocument.ID,
				Type:        string(ResourceWorkflow),
				Name:        workflowDocument.Spec.Name,
				Description: workflowDocument.Spec.Name,
			})
		}
	}
	for _, binding := range agent.Knowledge {
		if binding.Disabled {
			continue
		}
		if knowledge, ok := catalog.KnowledgeBases[binding.Ref.ID]; ok {
			out = append(out, agentcore.CapabilityDescriptor{
				ID:          knowledge.ID,
				Type:        string(ResourceKnowledge),
				Name:        knowledge.Name,
				Description: knowledge.Description,
			})
		}
	}
	return out
}

func BuildPromptText(prompt PromptConfig) string {
	return coreprompt.BuildText(coreprompt.Spec{
		System:    prompt.System,
		Developer: prompt.Developer,
		Templates: prompt.Templates,
		Variables: prompt.Variables,
		Metadata:  prompt.Metadata,
	})
}

func toAgentModelConfig(model ModelConfig) agentcore.ModelConfig {
	return agentcore.ModelConfig{
		Provider:    model.Provider,
		Model:       model.Model,
		Temperature: numberFromMap(model.DefaultParameters, "temperature"),
		MaxTokens:   intFromMap(model.DefaultParameters, "max_tokens"),
	}
}

func toToolSpec(tool ToolConfig) (tools.ToolSpec, error) {
	timeout, err := parseOptionalDuration(tool.Policy.Timeout)
	if err != nil {
		return tools.ToolSpec{}, err
	}
	backoff, err := parseOptionalDuration(tool.Policy.RetryPolicy.Backoff)
	if err != nil {
		return tools.ToolSpec{}, err
	}
	return tools.ToolSpec{
		ID:           tool.ID,
		Name:         tool.Name,
		Description:  tool.Description,
		Type:         tools.ToolType(tool.Type),
		InputSchema:  toToolSchema(tool.InputSchema),
		OutputSchema: toToolSchema(tool.OutputSchema),
		Implementation: tools.ToolImplementation{
			Kind:   tool.Implementation.Kind,
			Ref:    tool.Implementation.Ref.ID,
			Config: cloneMap(tool.Implementation.Config),
		},
		Policy: tools.ToolPolicy{
			Timeout:       timeout,
			RequireReview: tool.Policy.RequireReview,
			RetryPolicy: tools.RetryPolicy{
				MaxAttempts: tool.Policy.RetryPolicy.MaxAttempts,
				Backoff:     backoff,
			},
		},
		Metadata: cloneMap(tool.Metadata),
		Enabled:  !tool.Disabled,
	}, nil
}

func toConnectorSpec(config ConnectorConfig) connector.ConnectorSpec {
	return connector.ConnectorSpec{
		ID:          config.ID,
		Name:        config.Name,
		Description: config.Description,
		Protocol: connector.ProtocolSpec{
			Kind:    config.Protocol.Kind,
			BaseURL: config.Protocol.BaseURL,
			Config:  cloneMap(config.Protocol.Config),
		},
		Auth: connector.AuthSpec{
			Type:      config.Auth.Type,
			SecretRef: config.Auth.SecretRef,
			Config:    cloneMap(config.Auth.Config),
		},
		Metadata: cloneMap(config.Metadata),
		Enabled:  !config.Disabled,
	}
}

func toConnectorOperationSpec(connectorID string, operation ConnectorOperationConfig) connector.OperationSpec {
	timeout, _ := parseOptionalDuration(operation.Policy.Timeout)
	backoff, _ := parseOptionalDuration(operation.Policy.RetryPolicy.Backoff)
	return connector.OperationSpec{
		ID:           operation.ID,
		ConnectorID:  connectorID,
		Name:         operation.Name,
		Description:  operation.Description,
		InputSchema:  toConnectorSchema(operation.InputSchema),
		OutputSchema: toConnectorSchema(operation.OutputSchema),
		Request: connector.OperationRequest{
			Method:      operation.Request.Method,
			Path:        operation.Request.Path,
			PathParams:  cloneStringMap(operation.Request.PathParams),
			Headers:     cloneStringMap(operation.Request.Headers),
			Query:       cloneStringMap(operation.Request.Query),
			QueryParams: cloneStringMap(operation.Request.QueryParams),
			BodyField:   operation.Request.BodyField,
			Config:      cloneMap(operation.Request.Config),
		},
		Response: connector.OperationResponse{
			SuccessStatusCodes: append([]int(nil), operation.Response.SuccessStatusCodes...),
			Config:             cloneMap(operation.Response.Config),
		},
		Policy: connector.OperationPolicy{
			Timeout: timeout,
			RetryPolicy: connector.RetryPolicy{
				MaxAttempts: operation.Policy.RetryPolicy.MaxAttempts,
				Backoff:     backoff,
			},
		},
		Metadata: cloneMap(operation.Metadata),
		Enabled:  !operation.Disabled,
	}
}

func toWorkflowDocument(config WorkflowConfig) workflow.WorkflowDocument {
	return workflow.WorkflowDocument{
		ID:   config.ID,
		Spec: config.Spec,
		Publish: workflow.PublishMetadata{
			Status:       workflow.PublishStatus(config.Publish.Status),
			Revision:     config.Publish.Revision,
			SnapshotID:   config.Publish.SnapshotID,
			SnapshotHash: config.Publish.SnapshotHash,
		},
	}
}

func toAgentSchema(fields []SchemaField) []agentcore.SchemaField {
	out := make([]agentcore.SchemaField, 0, len(fields))
	for _, field := range fields {
		out = append(out, agentcore.SchemaField{
			Name:        field.Name,
			Type:        field.Type,
			Description: field.Description,
			Required:    field.Required,
			Children:    toAgentSchema(field.Children),
		})
	}
	return out
}

func toAgentSchemaFromTool(fields []tools.SchemaField) []agentcore.SchemaField {
	out := make([]agentcore.SchemaField, 0, len(fields))
	for _, field := range fields {
		out = append(out, agentcore.SchemaField{
			Name:        field.Name,
			Type:        field.Type,
			Description: field.Description,
			Required:    field.Required,
			Children:    toAgentSchemaFromTool(field.Children),
		})
	}
	return out
}

func toToolSchema(fields []SchemaField) []tools.SchemaField {
	out := make([]tools.SchemaField, 0, len(fields))
	for _, field := range fields {
		out = append(out, tools.SchemaField{
			Name:        field.Name,
			Type:        field.Type,
			Description: field.Description,
			Required:    field.Required,
			Children:    toToolSchema(field.Children),
		})
	}
	return out
}

func toConnectorSchema(fields []SchemaField) []connector.SchemaField {
	out := make([]connector.SchemaField, 0, len(fields))
	for _, field := range fields {
		out = append(out, connector.SchemaField{
			Name:        field.Name,
			Type:        field.Type,
			Description: field.Description,
			Required:    field.Required,
			Children:    toConnectorSchema(field.Children),
		})
	}
	return out
}

func parseOptionalDuration(value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	return time.ParseDuration(value)
}

func numberFromMap(values map[string]any, key string) float64 {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case jsonNumber:
		parsed, _ := strconv.ParseFloat(string(value), 64)
		return parsed
	default:
		return 0
	}
}

func intFromMap(values map[string]any, key string) int {
	if values == nil {
		return 0
	}
	switch value := values[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case float32:
		return int(value)
	case jsonNumber:
		parsed, _ := strconv.Atoi(string(value))
		return parsed
	default:
		return 0
	}
}

type jsonNumber string

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
