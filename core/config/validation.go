package config

import (
	"fmt"
	"strings"
	"time"

	coreschema "flow-anything/core/schema"
)

type ValidationError struct {
	Path    string
	Message string
}

func (e ValidationError) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return e.Path + ": " + e.Message
}

type ValidationErrors []ValidationError

func (e *ValidationErrors) Add(path, message string) {
	*e = append(*e, ValidationError{Path: path, Message: message})
}

func (e ValidationErrors) Error() string {
	parts := make([]string, 0, len(e))
	for _, item := range e {
		parts = append(parts, item.Error())
	}
	return strings.Join(parts, "; ")
}

func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

func ValidateBundle(bundle BundleSpec) error {
	var errs ValidationErrors
	if bundle.SchemaVersion == "" {
		errs.Add("schema_version", "schema version is required")
	} else if bundle.SchemaVersion != SchemaVersionV1 {
		errs.Add("schema_version", fmt.Sprintf("unsupported schema version %q", bundle.SchemaVersion))
	}
	if bundle.Kind != "" && bundle.Kind != BundleKind {
		errs.Add("kind", fmt.Sprintf("unsupported bundle kind %q", bundle.Kind))
	}
	if bundle.ID == "" {
		errs.Add("id", "bundle id is required")
	}
	if bundle.Version == "" {
		errs.Add("version", "bundle version is required")
	}
	validateRuntimeTargetSpec(&errs, "runtime", bundle.Runtime)

	index, err := BuildIndex(bundle)
	if err != nil {
		if validationErrors, ok := err.(ValidationErrors); ok {
			errs = append(errs, validationErrors...)
		} else {
			errs.Add("resources", err.Error())
		}
	}
	validateTopLevelDependencies(&errs, index, bundle.Dependencies)
	validateAgents(&errs, index, bundle.Resources.Agents)
	validateSkills(&errs, index, bundle.Resources.Skills)
	validateTools(&errs, index, bundle.Resources.Tools)
	validateWorkflows(&errs, bundle.Resources.Workflows)
	validateConnectors(&errs, bundle.Resources.Connectors)
	validateModels(&errs, bundle.Resources.Models)
	validateKnowledge(&errs, index, bundle.Resources.KnowledgeBases)
	validatePolicies(&errs, bundle.Resources.Policies)
	if errs.HasErrors() {
		return errs
	}
	return nil
}

func NormalizeBundle(bundle BundleSpec) BundleSpec {
	if bundle.Kind == "" {
		bundle.Kind = BundleKind
	}
	return bundle
}

func validateTopLevelDependencies(errs *ValidationErrors, index Index, refs []ResourceRef) {
	for i, ref := range refs {
		validateRef(errs, index, fmt.Sprintf("dependencies[%d]", i), ref, "")
	}
}

func validateAgents(errs *ValidationErrors, index Index, agents []AgentConfig) {
	for _, agent := range agents {
		path := "resources.agents." + agent.ID
		validateMeta(errs, path, agent.ResourceMeta)
		validatePrompt(errs, path+".prompt", agent.Prompt, true)
		if agent.ModelRef.ID != "" {
			validateRef(errs, index, path+".model_ref", agent.ModelRef, ResourceModel)
		}
		validateBindings(errs, index, path+".skills", agent.Skills, ResourceSkill)
		validateBindings(errs, index, path+".tools", agent.Tools, ResourceTool)
		validateBindings(errs, index, path+".workflows", agent.Workflows, ResourceWorkflow)
		validateBindings(errs, index, path+".knowledge", agent.Knowledge, ResourceKnowledge)
		validateRefs(errs, index, path+".policies", agent.Policies, ResourcePolicy)
		validateSchema(errs, path+".output_schema", agent.OutputSchema)
		validateRuntimeRequirement(errs, path+".runtime", agent.Runtime)
	}
}

func validateSkills(errs *ValidationErrors, index Index, skills []SkillConfig) {
	for _, skill := range skills {
		path := "resources.skills." + skill.ID
		validateMeta(errs, path, skill.ResourceMeta)
		validatePrompt(errs, path+".prompt", skill.Prompt, true)
		validateSchema(errs, path+".input_schema", skill.InputSchema)
		validateSchema(errs, path+".output_schema", skill.OutputSchema)
		validateBindings(errs, index, path+".tools", skill.Tools, ResourceTool)
		validateBindings(errs, index, path+".knowledge", skill.Knowledge, ResourceKnowledge)
		validateRefs(errs, index, path+".policies", skill.Policies, ResourcePolicy)
		validateRuntimeRequirement(errs, path+".runtime", skill.Runtime)
	}
}

func validateTools(errs *ValidationErrors, index Index, tools []ToolConfig) {
	for _, tool := range tools {
		path := "resources.tools." + tool.ID
		validateMeta(errs, path, tool.ResourceMeta)
		if tool.Type == "" {
			errs.Add(path+".type", "tool type is required")
		}
		if tool.Implementation.Kind == "" {
			errs.Add(path+".implementation.kind", "tool implementation kind is required")
		}
		validateToolImplementationRef(errs, index, path+".implementation.ref", tool)
		validateSchema(errs, path+".input_schema", tool.InputSchema)
		validateSchema(errs, path+".output_schema", tool.OutputSchema)
		validateExecutionPolicy(errs, path+".policy", tool.Policy)
		validateRuntimeRequirement(errs, path+".runtime", tool.Runtime)
	}
}

func validateToolImplementationRef(errs *ValidationErrors, index Index, path string, tool ToolConfig) {
	if tool.Implementation.Ref.ID == "" {
		if tool.Type == ToolTypeConnector || tool.Type == ToolTypeWorkflow {
			errs.Add(path, fmt.Sprintf("%s tool requires implementation ref", tool.Type))
		}
		return
	}
	switch tool.Type {
	case ToolTypeConnector:
		validateRef(errs, index, path, tool.Implementation.Ref, ResourceConnectorOperation)
	case ToolTypeWorkflow:
		validateRef(errs, index, path, tool.Implementation.Ref, ResourceWorkflow)
	default:
		validateRef(errs, index, path, tool.Implementation.Ref, tool.Implementation.Ref.Kind)
	}
}

func validateWorkflows(errs *ValidationErrors, workflows []WorkflowConfig) {
	for _, workflow := range workflows {
		path := "resources.workflows." + workflow.ID
		validateMeta(errs, path, workflow.ResourceMeta)
		if workflow.Spec.ID == "" {
			errs.Add(path+".spec.id", "workflow spec id is required")
		}
		if len(workflow.Spec.Nodes) == 0 {
			errs.Add(path+".spec.nodes", "workflow requires at least one node")
		}
		seen := map[string]bool{}
		for _, node := range workflow.Spec.Nodes {
			nodePath := path + ".spec.nodes." + node.ID
			if node.ID == "" {
				errs.Add(path+".spec.nodes", "node id is required")
				continue
			}
			if seen[node.ID] {
				errs.Add(nodePath, "duplicate node id")
			}
			seen[node.ID] = true
			if node.Type == "" {
				errs.Add(nodePath+".type", "node type is required")
			}
		}
		for _, edge := range workflow.Spec.Edges {
			if !seen[edge.From] || !seen[edge.To] {
				errs.Add(path+".spec.edges", fmt.Sprintf("edge %q -> %q references unknown node", edge.From, edge.To))
			}
		}
		validateRuntimeRequirement(errs, path+".runtime", workflow.Runtime)
	}
}

func validateConnectors(errs *ValidationErrors, connectors []ConnectorConfig) {
	for _, connector := range connectors {
		path := "resources.connectors." + connector.ID
		validateMeta(errs, path, connector.ResourceMeta)
		if connector.Protocol.Kind == "" {
			errs.Add(path+".protocol.kind", "connector protocol kind is required")
		}
		if connector.Auth.Type != "" && connector.Auth.SecretRef == "" && len(connector.Auth.Config) == 0 {
			errs.Add(path+".auth", "connector auth requires secret_ref or config")
		}
		validateRuntimeRequirement(errs, path+".runtime", connector.Runtime)
		for _, operation := range connector.Operations {
			operationPath := path + ".operations." + operation.ID
			validateMeta(errs, operationPath, operation.ResourceMeta)
			if operation.Request.Method == "" {
				errs.Add(operationPath+".request.method", "operation request method is required")
			}
			if operation.Request.Path == "" {
				errs.Add(operationPath+".request.path", "operation request path is required")
			}
			validateSchema(errs, operationPath+".input_schema", operation.InputSchema)
			validateSchema(errs, operationPath+".output_schema", operation.OutputSchema)
			validateExecutionPolicy(errs, operationPath+".policy", operation.Policy)
		}
	}
}

func validateModels(errs *ValidationErrors, models []ModelConfig) {
	for _, model := range models {
		path := "resources.models." + model.ID
		validateMeta(errs, path, model.ResourceMeta)
		if model.Provider == "" {
			errs.Add(path+".provider", "model provider is required")
		}
		if model.Model == "" {
			errs.Add(path+".model", "model name is required")
		}
		validateRuntimeRequirement(errs, path+".runtime", model.Runtime)
		validateExecutionPolicy(errs, path+".policy", model.Policy)
	}
}

func validateKnowledge(errs *ValidationErrors, index Index, knowledgeBases []KnowledgeConfig) {
	for _, knowledge := range knowledgeBases {
		path := "resources.knowledge_bases." + knowledge.ID
		validateMeta(errs, path, knowledge.ResourceMeta)
		if knowledge.Type == "" {
			errs.Add(path+".type", "knowledge type is required")
		}
		if knowledge.Source.Kind == "" {
			errs.Add(path+".source.kind", "knowledge source kind is required")
		}
		if knowledge.EmbeddingModelRef.ID != "" {
			validateRef(errs, index, path+".embedding_model_ref", knowledge.EmbeddingModelRef, ResourceModel)
		}
		validateRuntimeRequirement(errs, path+".runtime", knowledge.Runtime)
	}
}

func validatePolicies(errs *ValidationErrors, policies []PolicyConfig) {
	for _, policy := range policies {
		path := "resources.policies." + policy.ID
		validateMeta(errs, path, policy.ResourceMeta)
		if policy.Scope == "" {
			errs.Add(path+".scope", "policy scope is required")
		}
	}
}

func validateMeta(errs *ValidationErrors, path string, meta ResourceMeta) {
	if meta.ID == "" {
		errs.Add(path+".id", "resource id is required")
	}
	if meta.Name == "" {
		errs.Add(path+".name", "resource name is required")
	}
}

func validatePrompt(errs *ValidationErrors, path string, prompt PromptConfig, required bool) {
	if required && strings.TrimSpace(prompt.System) == "" && strings.TrimSpace(prompt.Developer) == "" {
		errs.Add(path, "prompt system or developer content is required")
	}
	validateSchema(errs, path+".variables", prompt.Variables)
}

func validateBindings(errs *ValidationErrors, index Index, path string, bindings []ResourceBinding, defaultKind ResourceKind) {
	for i, binding := range bindings {
		if binding.Disabled {
			continue
		}
		validateRef(errs, index, fmt.Sprintf("%s[%d].ref", path, i), binding.Ref, defaultKind)
	}
}

func validateRefs(errs *ValidationErrors, index Index, path string, refs []ResourceRef, defaultKind ResourceKind) {
	for i, ref := range refs {
		validateRef(errs, index, fmt.Sprintf("%s[%d]", path, i), ref, defaultKind)
	}
}

func validateRef(errs *ValidationErrors, index Index, path string, ref ResourceRef, defaultKind ResourceKind) {
	if ref.ID == "" {
		if !ref.Optional {
			errs.Add(path+".id", "resource ref id is required")
		}
		return
	}
	if ref.Kind == "" {
		ref.Kind = defaultKind
	}
	if ref.Kind == "" {
		errs.Add(path+".kind", "resource ref kind is required")
		return
	}
	if _, ok := index.Resolve(ref); !ok {
		if ref.Optional {
			return
		}
		errs.Add(path, fmt.Sprintf("referenced %s %q does not exist", ref.Kind, ref.ID))
	}
}

func validateRuntimeTargetSpec(errs *ValidationErrors, path string, runtime RuntimeTargetSpec) {
	for i, target := range runtime.Targets {
		if target == "" {
			errs.Add(fmt.Sprintf("%s.targets[%d]", path, i), "runtime target is required")
		}
	}
	for i, capability := range runtime.RequiredCapabilities {
		if capability.Name == "" {
			errs.Add(fmt.Sprintf("%s.required_capabilities[%d].name", path, i), "capability name is required")
		}
	}
}

func validateRuntimeRequirement(errs *ValidationErrors, path string, runtime RuntimeRequirementSpec) {
	for i, secret := range runtime.Secrets {
		if secret.Name == "" {
			errs.Add(fmt.Sprintf("%s.secrets[%d].name", path, i), "secret name is required")
		}
	}
	for i, capability := range runtime.Capabilities {
		if capability.Name == "" {
			errs.Add(fmt.Sprintf("%s.capabilities[%d].name", path, i), "capability name is required")
		}
	}
}

func validateExecutionPolicy(errs *ValidationErrors, path string, policy ExecutionPolicy) {
	validateDuration(errs, path+".timeout", policy.Timeout)
	validateDuration(errs, path+".retry_policy.backoff", policy.RetryPolicy.Backoff)
	if policy.RetryPolicy.MaxAttempts < 0 {
		errs.Add(path+".retry_policy.max_attempts", "max attempts must be >= 0")
	}
}

func validateDuration(errs *ValidationErrors, path, value string) {
	if value == "" {
		return
	}
	if _, err := time.ParseDuration(value); err != nil {
		errs.Add(path, "invalid duration: "+err.Error())
	}
}

func validateSchema(errs *ValidationErrors, path string, fields []SchemaField) {
	if err := coreschema.ValidateDefinition(fields); err != nil {
		if schemaErrors, ok := err.(coreschema.ValidationErrors); ok {
			for _, schemaErr := range schemaErrors {
				errs.Add(path+strings.TrimPrefix(schemaErr.Path, "$"), schemaErr.Message)
			}
			return
		}
		errs.Add(path, err.Error())
	}
}
