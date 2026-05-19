package platformconfig

import (
	"context"
	"fmt"
	"strings"

	coreconfig "flow-anything/core/config"
)

type ResourceDocument struct {
	Kind     coreconfig.ResourceKind `json:"kind"`
	ID       string                  `json:"id"`
	ParentID string                  `json:"parent_id,omitempty"`
	Resource any                     `json:"resource"`
}

type ResourceListFilter struct {
	Kind  coreconfig.ResourceKind
	Query string
}

type BundleInspection struct {
	Bundle       coreconfig.BundleSpec           `json:"bundle"`
	Snapshot     BundleSnapshotInfo              `json:"snapshot"`
	Counts       ResourceCounts                  `json:"counts"`
	Resources    []coreconfig.ResourceDescriptor `json:"resources,omitempty"`
	Dependencies []coreconfig.DependencyEdge     `json:"dependencies,omitempty"`
	Diagnostics  []coreconfig.Diagnostic         `json:"diagnostics,omitempty"`
}

func (s *Service) InspectBundle(ctx context.Context, id string, lifecycle coreconfig.BundleLifecycle) (BundleInspection, error) {
	bundle, err := s.loadBundle(ctx, id, lifecycle)
	if err != nil {
		return BundleInspection{}, err
	}
	state := coreconfig.InspectBundle(bundle)
	return BundleInspection{
		Bundle:       state.Bundle,
		Snapshot:     snapshotInfo(bundle),
		Counts:       CountResources(bundle),
		Resources:    state.Resources,
		Dependencies: state.Dependencies,
		Diagnostics:  state.Diagnostics,
	}, nil
}

func (s *Service) ListResources(ctx context.Context, bundleID string, filter ResourceListFilter) ([]ResourceDocument, error) {
	bundle, err := s.drafts.LoadBundle(ctx, bundleID)
	if err != nil {
		return nil, err
	}
	out := make([]ResourceDocument, 0)
	appendIfSelected := func(document ResourceDocument) {
		if filter.Kind != "" && document.Kind != filter.Kind {
			return
		}
		if !matchesResourceQuery(document, filter.Query) {
			return
		}
		out = append(out, document)
	}
	for _, resource := range bundle.Resources.Agents {
		appendIfSelected(ResourceDocument{Kind: coreconfig.ResourceAgent, ID: resource.ID, Resource: resource})
	}
	for _, resource := range bundle.Resources.Skills {
		appendIfSelected(ResourceDocument{Kind: coreconfig.ResourceSkill, ID: resource.ID, Resource: resource})
	}
	for _, resource := range bundle.Resources.Tools {
		appendIfSelected(ResourceDocument{Kind: coreconfig.ResourceTool, ID: resource.ID, Resource: resource})
	}
	for _, resource := range bundle.Resources.Workflows {
		appendIfSelected(ResourceDocument{Kind: coreconfig.ResourceWorkflow, ID: resource.ID, Resource: resource})
	}
	for _, resource := range bundle.Resources.Connectors {
		appendIfSelected(ResourceDocument{Kind: coreconfig.ResourceConnector, ID: resource.ID, Resource: resource})
		for _, operation := range resource.Operations {
			appendIfSelected(ResourceDocument{
				Kind:     coreconfig.ResourceConnectorOperation,
				ID:       operation.ID,
				ParentID: resource.ID,
				Resource: operation,
			})
		}
	}
	for _, resource := range bundle.Resources.Models {
		appendIfSelected(ResourceDocument{Kind: coreconfig.ResourceModel, ID: resource.ID, Resource: resource})
	}
	for _, resource := range bundle.Resources.KnowledgeBases {
		appendIfSelected(ResourceDocument{Kind: coreconfig.ResourceKnowledge, ID: resource.ID, Resource: resource})
	}
	for _, resource := range bundle.Resources.Policies {
		appendIfSelected(ResourceDocument{Kind: coreconfig.ResourcePolicy, ID: resource.ID, Resource: resource})
	}
	return out, nil
}

func (s *Service) GetResource(ctx context.Context, bundleID string, kind coreconfig.ResourceKind, resourceID string) (ResourceDocument, error) {
	if resourceID == "" {
		return ResourceDocument{}, fmt.Errorf("resource id is required")
	}
	bundle, err := s.drafts.LoadBundle(ctx, bundleID)
	if err != nil {
		return ResourceDocument{}, err
	}
	switch kind {
	case coreconfig.ResourceAgent:
		if resource, ok := getByID(bundle.Resources.Agents, resourceID, func(item coreconfig.AgentConfig) string { return item.ID }); ok {
			return ResourceDocument{Kind: kind, ID: resourceID, Resource: resource}, nil
		}
	case coreconfig.ResourceSkill:
		if resource, ok := getByID(bundle.Resources.Skills, resourceID, func(item coreconfig.SkillConfig) string { return item.ID }); ok {
			return ResourceDocument{Kind: kind, ID: resourceID, Resource: resource}, nil
		}
	case coreconfig.ResourceTool:
		if resource, ok := getByID(bundle.Resources.Tools, resourceID, func(item coreconfig.ToolConfig) string { return item.ID }); ok {
			return ResourceDocument{Kind: kind, ID: resourceID, Resource: resource}, nil
		}
	case coreconfig.ResourceWorkflow:
		if resource, ok := getByID(bundle.Resources.Workflows, resourceID, func(item coreconfig.WorkflowConfig) string { return item.ID }); ok {
			return ResourceDocument{Kind: kind, ID: resourceID, Resource: resource}, nil
		}
	case coreconfig.ResourceConnector:
		if resource, ok := getByID(bundle.Resources.Connectors, resourceID, func(item coreconfig.ConnectorConfig) string { return item.ID }); ok {
			return ResourceDocument{Kind: kind, ID: resourceID, Resource: resource}, nil
		}
	case coreconfig.ResourceModel:
		if resource, ok := getByID(bundle.Resources.Models, resourceID, func(item coreconfig.ModelConfig) string { return item.ID }); ok {
			return ResourceDocument{Kind: kind, ID: resourceID, Resource: resource}, nil
		}
	case coreconfig.ResourceKnowledge:
		if resource, ok := getByID(bundle.Resources.KnowledgeBases, resourceID, func(item coreconfig.KnowledgeConfig) string { return item.ID }); ok {
			return ResourceDocument{Kind: kind, ID: resourceID, Resource: resource}, nil
		}
	case coreconfig.ResourcePolicy:
		if resource, ok := getByID(bundle.Resources.Policies, resourceID, func(item coreconfig.PolicyConfig) string { return item.ID }); ok {
			return ResourceDocument{Kind: kind, ID: resourceID, Resource: resource}, nil
		}
	case coreconfig.ResourceConnectorOperation:
		return ResourceDocument{}, fmt.Errorf("connector operation requires connector id; use connector operation API")
	default:
		return ResourceDocument{}, unsupportedResourceKind(kind)
	}
	return ResourceDocument{}, fmt.Errorf("%s %q not found", kind, resourceID)
}

func (s *Service) UpsertResource(ctx context.Context, bundleID string, resource ResourceDocument) (coreconfig.BundleSpec, error) {
	if resource.ID == "" {
		return coreconfig.BundleSpec{}, fmt.Errorf("resource id is required")
	}
	bundle, err := s.drafts.LoadBundle(ctx, bundleID)
	if err != nil {
		return coreconfig.BundleSpec{}, err
	}
	switch resource.Kind {
	case coreconfig.ResourceAgent:
		item, err := typedResource[coreconfig.AgentConfig](resource)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		item.ID, err = normalizedResourceID(item.ID, resource.ID)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		bundle.Resources.Agents = upsertByID(bundle.Resources.Agents, item, func(value coreconfig.AgentConfig) string { return value.ID })
	case coreconfig.ResourceSkill:
		item, err := typedResource[coreconfig.SkillConfig](resource)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		item.ID, err = normalizedResourceID(item.ID, resource.ID)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		bundle.Resources.Skills = upsertByID(bundle.Resources.Skills, item, func(value coreconfig.SkillConfig) string { return value.ID })
	case coreconfig.ResourceTool:
		item, err := typedResource[coreconfig.ToolConfig](resource)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		item.ID, err = normalizedResourceID(item.ID, resource.ID)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		bundle.Resources.Tools = upsertByID(bundle.Resources.Tools, item, func(value coreconfig.ToolConfig) string { return value.ID })
	case coreconfig.ResourceWorkflow:
		item, err := typedResource[coreconfig.WorkflowConfig](resource)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		item.ID, err = normalizedResourceID(item.ID, resource.ID)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		bundle.Resources.Workflows = upsertByID(bundle.Resources.Workflows, item, func(value coreconfig.WorkflowConfig) string { return value.ID })
	case coreconfig.ResourceConnector:
		item, err := typedResource[coreconfig.ConnectorConfig](resource)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		item.ID, err = normalizedResourceID(item.ID, resource.ID)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		bundle.Resources.Connectors = upsertByID(bundle.Resources.Connectors, item, func(value coreconfig.ConnectorConfig) string { return value.ID })
	case coreconfig.ResourceModel:
		item, err := typedResource[coreconfig.ModelConfig](resource)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		item.ID, err = normalizedResourceID(item.ID, resource.ID)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		bundle.Resources.Models = upsertByID(bundle.Resources.Models, item, func(value coreconfig.ModelConfig) string { return value.ID })
	case coreconfig.ResourceKnowledge:
		item, err := typedResource[coreconfig.KnowledgeConfig](resource)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		item.ID, err = normalizedResourceID(item.ID, resource.ID)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		bundle.Resources.KnowledgeBases = upsertByID(bundle.Resources.KnowledgeBases, item, func(value coreconfig.KnowledgeConfig) string { return value.ID })
	case coreconfig.ResourcePolicy:
		item, err := typedResource[coreconfig.PolicyConfig](resource)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		item.ID, err = normalizedResourceID(item.ID, resource.ID)
		if err != nil {
			return coreconfig.BundleSpec{}, err
		}
		bundle.Resources.Policies = upsertByID(bundle.Resources.Policies, item, func(value coreconfig.PolicyConfig) string { return value.ID })
	case coreconfig.ResourceConnectorOperation:
		return coreconfig.BundleSpec{}, fmt.Errorf("connector operation requires connector id; use connector operation API")
	default:
		return coreconfig.BundleSpec{}, unsupportedResourceKind(resource.Kind)
	}
	return s.SaveBundle(ctx, bundle)
}

func (s *Service) DeleteResource(ctx context.Context, bundleID string, kind coreconfig.ResourceKind, resourceID string) (coreconfig.BundleSpec, error) {
	if resourceID == "" {
		return coreconfig.BundleSpec{}, fmt.Errorf("resource id is required")
	}
	bundle, err := s.drafts.LoadBundle(ctx, bundleID)
	if err != nil {
		return coreconfig.BundleSpec{}, err
	}
	switch kind {
	case coreconfig.ResourceAgent:
		bundle.Resources.Agents, err = deleteByID(bundle.Resources.Agents, resourceID, func(item coreconfig.AgentConfig) string { return item.ID })
	case coreconfig.ResourceSkill:
		bundle.Resources.Skills, err = deleteByID(bundle.Resources.Skills, resourceID, func(item coreconfig.SkillConfig) string { return item.ID })
	case coreconfig.ResourceTool:
		bundle.Resources.Tools, err = deleteByID(bundle.Resources.Tools, resourceID, func(item coreconfig.ToolConfig) string { return item.ID })
	case coreconfig.ResourceWorkflow:
		bundle.Resources.Workflows, err = deleteByID(bundle.Resources.Workflows, resourceID, func(item coreconfig.WorkflowConfig) string { return item.ID })
	case coreconfig.ResourceConnector:
		bundle.Resources.Connectors, err = deleteByID(bundle.Resources.Connectors, resourceID, func(item coreconfig.ConnectorConfig) string { return item.ID })
	case coreconfig.ResourceModel:
		bundle.Resources.Models, err = deleteByID(bundle.Resources.Models, resourceID, func(item coreconfig.ModelConfig) string { return item.ID })
	case coreconfig.ResourceKnowledge:
		bundle.Resources.KnowledgeBases, err = deleteByID(bundle.Resources.KnowledgeBases, resourceID, func(item coreconfig.KnowledgeConfig) string { return item.ID })
	case coreconfig.ResourcePolicy:
		bundle.Resources.Policies, err = deleteByID(bundle.Resources.Policies, resourceID, func(item coreconfig.PolicyConfig) string { return item.ID })
	case coreconfig.ResourceConnectorOperation:
		return coreconfig.BundleSpec{}, fmt.Errorf("connector operation requires connector id; use connector operation API")
	default:
		return coreconfig.BundleSpec{}, unsupportedResourceKind(kind)
	}
	if err != nil {
		return coreconfig.BundleSpec{}, err
	}
	return s.SaveBundle(ctx, bundle)
}

func (s *Service) ListConnectorOperations(ctx context.Context, bundleID string, connectorID string, query string) ([]coreconfig.ConnectorOperationConfig, error) {
	if connectorID == "" {
		return nil, fmt.Errorf("connector id is required")
	}
	bundle, err := s.drafts.LoadBundle(ctx, bundleID)
	if err != nil {
		return nil, err
	}
	connector, ok := getByID(bundle.Resources.Connectors, connectorID, func(item coreconfig.ConnectorConfig) string { return item.ID })
	if !ok {
		return nil, fmt.Errorf("connector %q not found", connectorID)
	}
	out := make([]coreconfig.ConnectorOperationConfig, 0, len(connector.Operations))
	for _, operation := range connector.Operations {
		document := ResourceDocument{
			Kind:     coreconfig.ResourceConnectorOperation,
			ID:       operation.ID,
			ParentID: connectorID,
			Resource: operation,
		}
		if matchesResourceQuery(document, query) {
			out = append(out, operation)
		}
	}
	return out, nil
}

func (s *Service) GetConnectorOperation(ctx context.Context, bundleID string, connectorID string, operationID string) (coreconfig.ConnectorOperationConfig, error) {
	if connectorID == "" {
		return coreconfig.ConnectorOperationConfig{}, fmt.Errorf("connector id is required")
	}
	if operationID == "" {
		return coreconfig.ConnectorOperationConfig{}, fmt.Errorf("operation id is required")
	}
	bundle, err := s.drafts.LoadBundle(ctx, bundleID)
	if err != nil {
		return coreconfig.ConnectorOperationConfig{}, err
	}
	connector, ok := getByID(bundle.Resources.Connectors, connectorID, func(item coreconfig.ConnectorConfig) string { return item.ID })
	if !ok {
		return coreconfig.ConnectorOperationConfig{}, fmt.Errorf("connector %q not found", connectorID)
	}
	operation, ok := getByID(connector.Operations, operationID, func(item coreconfig.ConnectorOperationConfig) string { return item.ID })
	if !ok {
		return coreconfig.ConnectorOperationConfig{}, fmt.Errorf("connector operation %q not found", operationID)
	}
	return operation, nil
}

func (s *Service) UpsertConnectorOperation(ctx context.Context, bundleID string, connectorID string, operation coreconfig.ConnectorOperationConfig) (coreconfig.BundleSpec, error) {
	if connectorID == "" {
		return coreconfig.BundleSpec{}, fmt.Errorf("connector id is required")
	}
	if operation.ID == "" {
		return coreconfig.BundleSpec{}, fmt.Errorf("operation id is required")
	}
	bundle, err := s.drafts.LoadBundle(ctx, bundleID)
	if err != nil {
		return coreconfig.BundleSpec{}, err
	}
	index, ok := findIndexByID(bundle.Resources.Connectors, connectorID, func(item coreconfig.ConnectorConfig) string { return item.ID })
	if !ok {
		return coreconfig.BundleSpec{}, fmt.Errorf("connector %q not found", connectorID)
	}
	connector := bundle.Resources.Connectors[index]
	connector.Operations = upsertByID(connector.Operations, operation, func(item coreconfig.ConnectorOperationConfig) string { return item.ID })
	bundle.Resources.Connectors[index] = connector
	return s.SaveBundle(ctx, bundle)
}

func (s *Service) DeleteConnectorOperation(ctx context.Context, bundleID string, connectorID string, operationID string) (coreconfig.BundleSpec, error) {
	if connectorID == "" {
		return coreconfig.BundleSpec{}, fmt.Errorf("connector id is required")
	}
	if operationID == "" {
		return coreconfig.BundleSpec{}, fmt.Errorf("operation id is required")
	}
	bundle, err := s.drafts.LoadBundle(ctx, bundleID)
	if err != nil {
		return coreconfig.BundleSpec{}, err
	}
	index, ok := findIndexByID(bundle.Resources.Connectors, connectorID, func(item coreconfig.ConnectorConfig) string { return item.ID })
	if !ok {
		return coreconfig.BundleSpec{}, fmt.Errorf("connector %q not found", connectorID)
	}
	connector := bundle.Resources.Connectors[index]
	connector.Operations, err = deleteByID(connector.Operations, operationID, func(item coreconfig.ConnectorOperationConfig) string { return item.ID })
	if err != nil {
		return coreconfig.BundleSpec{}, err
	}
	bundle.Resources.Connectors[index] = connector
	return s.SaveBundle(ctx, bundle)
}

func typedResource[T any](resource ResourceDocument) (T, error) {
	value, ok := resource.Resource.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("resource payload does not match %s", resource.Kind)
	}
	return value, nil
}

func normalizedResourceID(bodyID string, pathID string) (string, error) {
	if bodyID == "" {
		return pathID, nil
	}
	if bodyID != pathID {
		return "", fmt.Errorf("resource id mismatch: path has %q but body has %q", pathID, bodyID)
	}
	return bodyID, nil
}

func getByID[T any](items []T, id string, idOf func(T) string) (T, bool) {
	for _, item := range items {
		if idOf(item) == id {
			return item, true
		}
	}
	var zero T
	return zero, false
}

func findIndexByID[T any](items []T, id string, idOf func(T) string) (int, bool) {
	for index, item := range items {
		if idOf(item) == id {
			return index, true
		}
	}
	return -1, false
}

func upsertByID[T any](items []T, value T, idOf func(T) string) []T {
	id := idOf(value)
	out := append([]T(nil), items...)
	for index, item := range out {
		if idOf(item) == id {
			out[index] = value
			return out
		}
	}
	return append(out, value)
}

func deleteByID[T any](items []T, id string, idOf func(T) string) ([]T, error) {
	out := make([]T, 0, len(items))
	found := false
	for _, item := range items {
		if idOf(item) == id {
			found = true
			continue
		}
		out = append(out, item)
	}
	if !found {
		return nil, fmt.Errorf("resource %q not found", id)
	}
	return out, nil
}

func unsupportedResourceKind(kind coreconfig.ResourceKind) error {
	return fmt.Errorf("unsupported resource kind %q", kind)
}

func matchesResourceQuery(document ResourceDocument, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	fields := []string{document.ID, document.ParentID}
	if meta, ok := resourceMeta(document.Resource); ok {
		fields = append(fields, meta.ID, meta.Name, meta.Description, meta.Owner.Team, meta.Owner.Email)
		fields = append(fields, meta.Labels...)
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

func resourceMeta(resource any) (coreconfig.ResourceMeta, bool) {
	switch item := resource.(type) {
	case coreconfig.AgentConfig:
		return item.ResourceMeta, true
	case coreconfig.SkillConfig:
		return item.ResourceMeta, true
	case coreconfig.ToolConfig:
		return item.ResourceMeta, true
	case coreconfig.WorkflowConfig:
		return item.ResourceMeta, true
	case coreconfig.ConnectorConfig:
		return item.ResourceMeta, true
	case coreconfig.ConnectorOperationConfig:
		return item.ResourceMeta, true
	case coreconfig.ModelConfig:
		return item.ResourceMeta, true
	case coreconfig.KnowledgeConfig:
		return item.ResourceMeta, true
	case coreconfig.PolicyConfig:
		return item.ResourceMeta, true
	default:
		return coreconfig.ResourceMeta{}, false
	}
}
