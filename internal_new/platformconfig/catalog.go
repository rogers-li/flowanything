package platformconfig

import (
	"context"
	"fmt"
	"sort"
	"sync"

	coreconfig "flow-anything/core/config"
)

// BundleDraft contains bundle-level metadata owned by the platform editor.
//
// Resource definitions live in Catalog. BuildBundle combines both pieces into
// the config-as-code document consumed by runtime services.
type BundleDraft struct {
	ID           string
	Name         string
	Version      string
	Description  string
	Runtime      coreconfig.RuntimeTargetSpec
	Dependencies []coreconfig.ResourceRef
	Permissions  coreconfig.PermissionSpec
	Signature    coreconfig.SignatureSpec
	Metadata     map[string]any
}

// Catalog is the write model for platform configuration.
//
// The catalog intentionally stores core/config resources directly. The editor
// may keep UI-only state elsewhere, but runtime-visible configuration should
// already follow the standard Bundle protocol before it reaches this package.
type Catalog struct {
	mu sync.RWMutex

	agents         map[string]coreconfig.AgentConfig
	skills         map[string]coreconfig.SkillConfig
	tools          map[string]coreconfig.ToolConfig
	workflows      map[string]coreconfig.WorkflowConfig
	connectors     map[string]coreconfig.ConnectorConfig
	models         map[string]coreconfig.ModelConfig
	knowledgeBases map[string]coreconfig.KnowledgeConfig
	policies       map[string]coreconfig.PolicyConfig
}

func NewCatalog() *Catalog {
	return &Catalog{
		agents:         map[string]coreconfig.AgentConfig{},
		skills:         map[string]coreconfig.SkillConfig{},
		tools:          map[string]coreconfig.ToolConfig{},
		workflows:      map[string]coreconfig.WorkflowConfig{},
		connectors:     map[string]coreconfig.ConnectorConfig{},
		models:         map[string]coreconfig.ModelConfig{},
		knowledgeBases: map[string]coreconfig.KnowledgeConfig{},
		policies:       map[string]coreconfig.PolicyConfig{},
	}
}

func (c *Catalog) UpsertAgent(agent coreconfig.AgentConfig) error {
	return c.upsert(agent.ID, "agent", func() {
		c.agents[agent.ID] = agent
	})
}

func (c *Catalog) UpsertSkill(skill coreconfig.SkillConfig) error {
	return c.upsert(skill.ID, "skill", func() {
		c.skills[skill.ID] = skill
	})
}

func (c *Catalog) UpsertTool(tool coreconfig.ToolConfig) error {
	return c.upsert(tool.ID, "tool", func() {
		c.tools[tool.ID] = tool
	})
}

func (c *Catalog) UpsertWorkflow(workflow coreconfig.WorkflowConfig) error {
	return c.upsert(workflow.ID, "workflow", func() {
		c.workflows[workflow.ID] = workflow
	})
}

func (c *Catalog) UpsertConnector(connector coreconfig.ConnectorConfig) error {
	return c.upsert(connector.ID, "connector", func() {
		c.connectors[connector.ID] = connector
	})
}

func (c *Catalog) UpsertModel(model coreconfig.ModelConfig) error {
	return c.upsert(model.ID, "model", func() {
		c.models[model.ID] = model
	})
}

func (c *Catalog) UpsertKnowledgeBase(knowledge coreconfig.KnowledgeConfig) error {
	return c.upsert(knowledge.ID, "knowledge base", func() {
		c.knowledgeBases[knowledge.ID] = knowledge
	})
}

func (c *Catalog) UpsertPolicy(policy coreconfig.PolicyConfig) error {
	return c.upsert(policy.ID, "policy", func() {
		c.policies[policy.ID] = policy
	})
}

func (c *Catalog) upsert(id string, kind string, write func()) error {
	if id == "" {
		return fmt.Errorf("%s id is required", kind)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	write()
	return nil
}

// BuildBundle creates a deterministic BundleSpec from the current catalog.
//
// It performs only structural assembly. Call Publisher.Publish, or
// core/config.ValidateBundle directly, when a publish-grade validation result is
// needed.
func (c *Catalog) BuildBundle(_ context.Context, draft BundleDraft) (coreconfig.BundleSpec, error) {
	if draft.ID == "" {
		return coreconfig.BundleSpec{}, fmt.Errorf("bundle id is required")
	}
	if draft.Version == "" {
		return coreconfig.BundleSpec{}, fmt.Errorf("bundle version is required")
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	return coreconfig.BundleSpec{
		SchemaVersion: coreconfig.SchemaVersionV1,
		Kind:          coreconfig.BundleKind,
		ID:            draft.ID,
		Name:          draft.Name,
		Version:       draft.Version,
		Description:   draft.Description,
		Runtime:       draft.Runtime,
		Dependencies:  append([]coreconfig.ResourceRef(nil), draft.Dependencies...),
		Permissions:   draft.Permissions,
		Signature:     draft.Signature,
		Metadata:      cloneAnyMap(draft.Metadata),
		Resources: coreconfig.ResourceCollection{
			Agents:         sortedValues(c.agents),
			Skills:         sortedValues(c.skills),
			Tools:          sortedValues(c.tools),
			Workflows:      sortedValues(c.workflows),
			Connectors:     sortedValues(c.connectors),
			Models:         sortedValues(c.models),
			KnowledgeBases: sortedValues(c.knowledgeBases),
			Policies:       sortedValues(c.policies),
		},
	}, nil
}

func sortedValues[T any](items map[string]T) []T {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]T, 0, len(keys))
	for _, key := range keys {
		out = append(out, items[key])
	}
	return out
}

func cloneAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
