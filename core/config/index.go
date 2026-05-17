package config

import (
	"fmt"
	"sort"
)

type ResourceDescriptor struct {
	Kind        ResourceKind
	ID          string
	Name        string
	Description string
	Version     string
	Disabled    bool
	ParentID    string
}

type ConnectorOperationDescriptor struct {
	ConnectorID string
	Operation   ConnectorOperationConfig
}

type Index struct {
	Agents              map[string]AgentConfig
	Skills              map[string]SkillConfig
	Tools               map[string]ToolConfig
	Workflows           map[string]WorkflowConfig
	Connectors          map[string]ConnectorConfig
	ConnectorOperations map[string]ConnectorOperationDescriptor
	Models              map[string]ModelConfig
	KnowledgeBases      map[string]KnowledgeConfig
	Policies            map[string]PolicyConfig
}

func BuildIndex(bundle BundleSpec) (Index, error) {
	index := Index{
		Agents:              map[string]AgentConfig{},
		Skills:              map[string]SkillConfig{},
		Tools:               map[string]ToolConfig{},
		Workflows:           map[string]WorkflowConfig{},
		Connectors:          map[string]ConnectorConfig{},
		ConnectorOperations: map[string]ConnectorOperationDescriptor{},
		Models:              map[string]ModelConfig{},
		KnowledgeBases:      map[string]KnowledgeConfig{},
		Policies:            map[string]PolicyConfig{},
	}
	var errs ValidationErrors
	for _, item := range bundle.Resources.Agents {
		addResource(&errs, ResourceAgent, item.ID, index.has(ResourceAgent, item.ID), func() { index.Agents[item.ID] = item })
	}
	for _, item := range bundle.Resources.Skills {
		addResource(&errs, ResourceSkill, item.ID, index.has(ResourceSkill, item.ID), func() { index.Skills[item.ID] = item })
	}
	for _, item := range bundle.Resources.Tools {
		addResource(&errs, ResourceTool, item.ID, index.has(ResourceTool, item.ID), func() { index.Tools[item.ID] = item })
	}
	for _, item := range bundle.Resources.Workflows {
		addResource(&errs, ResourceWorkflow, item.ID, index.has(ResourceWorkflow, item.ID), func() { index.Workflows[item.ID] = item })
	}
	for _, item := range bundle.Resources.Connectors {
		addResource(&errs, ResourceConnector, item.ID, index.has(ResourceConnector, item.ID), func() { index.Connectors[item.ID] = item })
		for _, operation := range item.Operations {
			id := operation.ID
			if id == "" {
				errs.Add("resources.connectors."+item.ID+".operations", "connector operation id is required")
				continue
			}
			if _, exists := index.ConnectorOperations[id]; exists {
				errs.Add("resources.connector_operations."+id, "duplicate connector operation id")
				continue
			}
			index.ConnectorOperations[id] = ConnectorOperationDescriptor{ConnectorID: item.ID, Operation: operation}
		}
	}
	for _, item := range bundle.Resources.Models {
		addResource(&errs, ResourceModel, item.ID, index.has(ResourceModel, item.ID), func() { index.Models[item.ID] = item })
	}
	for _, item := range bundle.Resources.KnowledgeBases {
		addResource(&errs, ResourceKnowledge, item.ID, index.has(ResourceKnowledge, item.ID), func() { index.KnowledgeBases[item.ID] = item })
	}
	for _, item := range bundle.Resources.Policies {
		addResource(&errs, ResourcePolicy, item.ID, index.has(ResourcePolicy, item.ID), func() { index.Policies[item.ID] = item })
	}
	if errs.HasErrors() {
		return Index{}, errs
	}
	return index, nil
}

func (i Index) Exists(ref ResourceRef) bool {
	_, ok := i.Resolve(ref)
	return ok
}

func (i Index) ListDescriptors() []ResourceDescriptor {
	out := []ResourceDescriptor{}
	for _, item := range i.Agents {
		out = append(out, descriptor(ResourceAgent, item.ResourceMeta, ""))
	}
	for _, item := range i.Skills {
		out = append(out, descriptor(ResourceSkill, item.ResourceMeta, ""))
	}
	for _, item := range i.Tools {
		out = append(out, descriptor(ResourceTool, item.ResourceMeta, ""))
	}
	for _, item := range i.Workflows {
		out = append(out, descriptor(ResourceWorkflow, item.ResourceMeta, ""))
	}
	for _, item := range i.Connectors {
		out = append(out, descriptor(ResourceConnector, item.ResourceMeta, ""))
	}
	for _, item := range i.ConnectorOperations {
		out = append(out, descriptor(ResourceConnectorOperation, item.Operation.ResourceMeta, item.ConnectorID))
	}
	for _, item := range i.Models {
		out = append(out, descriptor(ResourceModel, item.ResourceMeta, ""))
	}
	for _, item := range i.KnowledgeBases {
		out = append(out, descriptor(ResourceKnowledge, item.ResourceMeta, ""))
	}
	for _, item := range i.Policies {
		out = append(out, descriptor(ResourcePolicy, item.ResourceMeta, ""))
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Kind == out[b].Kind {
			return out[a].ID < out[b].ID
		}
		return out[a].Kind < out[b].Kind
	})
	return out
}

func (i Index) Resolve(ref ResourceRef) (ResourceDescriptor, bool) {
	switch ref.Kind {
	case ResourceAgent:
		item, ok := i.Agents[ref.ID]
		return descriptor(ResourceAgent, item.ResourceMeta, ""), ok
	case ResourceSkill:
		item, ok := i.Skills[ref.ID]
		return descriptor(ResourceSkill, item.ResourceMeta, ""), ok
	case ResourceTool:
		item, ok := i.Tools[ref.ID]
		return descriptor(ResourceTool, item.ResourceMeta, ""), ok
	case ResourceWorkflow:
		item, ok := i.Workflows[ref.ID]
		return descriptor(ResourceWorkflow, item.ResourceMeta, ""), ok
	case ResourceConnector:
		item, ok := i.Connectors[ref.ID]
		return descriptor(ResourceConnector, item.ResourceMeta, ""), ok
	case ResourceConnectorOperation:
		item, ok := i.ConnectorOperations[ref.ID]
		return descriptor(ResourceConnectorOperation, item.Operation.ResourceMeta, item.ConnectorID), ok
	case ResourceModel:
		item, ok := i.Models[ref.ID]
		return descriptor(ResourceModel, item.ResourceMeta, ""), ok
	case ResourceKnowledge:
		item, ok := i.KnowledgeBases[ref.ID]
		return descriptor(ResourceKnowledge, item.ResourceMeta, ""), ok
	case ResourcePolicy:
		item, ok := i.Policies[ref.ID]
		return descriptor(ResourcePolicy, item.ResourceMeta, ""), ok
	default:
		return ResourceDescriptor{}, false
	}
}

func descriptor(kind ResourceKind, meta ResourceMeta, parentID string) ResourceDescriptor {
	return ResourceDescriptor{
		Kind:        kind,
		ID:          meta.ID,
		Name:        meta.Name,
		Description: meta.Description,
		Version:     meta.Version,
		Disabled:    meta.Disabled,
		ParentID:    parentID,
	}
}

func (i Index) has(kind ResourceKind, id string) bool {
	if id == "" {
		return false
	}
	_, ok := i.Resolve(ResourceRef{Kind: kind, ID: id})
	return ok
}

func addResource(errs *ValidationErrors, kind ResourceKind, id string, exists bool, add func()) {
	if id == "" {
		errs.Add("resources."+string(kind), fmt.Sprintf("%s id is required", kind))
		return
	}
	if exists {
		errs.Add("resources."+string(kind)+"."+id, fmt.Sprintf("duplicate %s id", kind))
		return
	}
	add()
}

func (m ResourceMeta) IsEnabled() bool {
	return !m.Disabled
}
