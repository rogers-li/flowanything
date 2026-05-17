package editor

import (
	"fmt"
	"sort"

	"flow-anything/core/config"
	"flow-anything/core/flowengine"
)

// InspectDraft returns the editor-facing view of a bundle draft. It normalizes
// protocol defaults, indexes resources, extracts dependencies, and adds
// draft-only diagnostics that are useful while editing but not runtime concerns.
func InspectDraft(bundle config.BundleSpec) DraftInspection {
	state := config.InspectBundle(bundle)
	inspection := DraftInspection{
		Bundle:       state.Bundle,
		Resources:    state.Resources,
		Dependencies: state.Dependencies,
		Diagnostics:  append([]config.Diagnostic{}, state.Diagnostics...),
	}
	inspection.Diagnostics = append(inspection.Diagnostics, workflowDraftDiagnostics(state.Bundle)...)
	inspection.Publishable = diagnosticsAllowPublish(inspection.Diagnostics)
	return inspection
}

// ListBindableResources returns resources that a UI may present in bind
// selectors. It is intentionally read-only and applies only generic filters.
func ListBindableResources(bundle config.BundleSpec, filter BindingFilter) ([]config.ResourceDescriptor, error) {
	index, err := config.BuildIndex(config.NormalizeBundle(bundle))
	if err != nil {
		return nil, err
	}
	allowedKinds := map[config.ResourceKind]bool{}
	for _, kind := range filter.Kinds {
		allowedKinds[kind] = true
	}
	descriptors := index.ListDescriptors()
	out := make([]config.ResourceDescriptor, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if len(allowedKinds) > 0 && !allowedKinds[descriptor.Kind] {
			continue
		}
		if !filter.IncludeDisabled && descriptor.Disabled {
			continue
		}
		if filter.ParentID != "" && descriptor.ParentID != filter.ParentID {
			continue
		}
		out = append(out, descriptor)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].Name < out[j].Name
		}
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}

func workflowDraftDiagnostics(bundle config.BundleSpec) []config.Diagnostic {
	out := []config.Diagnostic{}
	for _, workflow := range bundle.Resources.Workflows {
		path := "resources.workflows." + workflow.ID + ".spec"
		starts := 0
		nodeIDs := map[string]bool{}
		outDegree := map[string]int{}
		for _, node := range workflow.Spec.Nodes {
			nodeIDs[node.ID] = true
			if node.Type == flowengine.NodeTypeStart {
				starts++
			}
		}
		for _, edge := range workflow.Spec.Edges {
			outDegree[edge.From]++
		}
		switch {
		case len(workflow.Spec.Nodes) == 0:
			continue
		case starts == 0:
			out = append(out, config.Diagnostic{
				Severity: config.DiagnosticWarning,
				Path:     path + ".nodes",
				Message:  "workflow draft has no start node",
			})
		case starts > 1:
			out = append(out, config.Diagnostic{
				Severity: config.DiagnosticError,
				Path:     path + ".nodes",
				Message:  "workflow draft has multiple start nodes",
			})
		}
		for _, node := range workflow.Spec.Nodes {
			if node.Type == flowengine.NodeTypeStart && outDegree[node.ID] > 1 {
				out = append(out, config.Diagnostic{
					Severity: config.DiagnosticError,
					Path:     fmt.Sprintf("%s.nodes.%s", path, node.ID),
					Message:  "start node can have at most one outgoing edge",
				})
			}
		}
		for nodeID := range workflowNodeUI(workflow.UI) {
			if !nodeIDs[nodeID] {
				out = append(out, config.Diagnostic{
					Severity: config.DiagnosticWarning,
					Path:     fmt.Sprintf("resources.workflows.%s.ui.nodes.%s", workflow.ID, nodeID),
					Message:  "node UI metadata references a missing workflow node",
				})
			}
		}
	}
	return out
}

func diagnosticsAllowPublish(diagnostics []config.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == config.DiagnosticError {
			return false
		}
	}
	return true
}
