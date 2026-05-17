package editor

import (
	"encoding/json"
	"fmt"

	"flow-anything/core/config"
	"flow-anything/core/flowengine"
)

// UpsertWorkflowNode adds or replaces a node in a workflow draft. Existing
// edges and UI metadata are preserved.
func UpsertWorkflowNode(bundle config.BundleSpec, workflowID string, node flowengine.NodeSpec) (config.BundleSpec, error) {
	if node.ID == "" {
		return config.BundleSpec{}, fmt.Errorf("%w: workflow node id is required", ErrInvalidPatchPath)
	}
	edited, err := cloneBundle(bundle)
	if err != nil {
		return config.BundleSpec{}, err
	}
	workflow, err := workflowByID(&edited, workflowID)
	if err != nil {
		return config.BundleSpec{}, err
	}
	for i := range workflow.Spec.Nodes {
		if workflow.Spec.Nodes[i].ID == node.ID {
			workflow.Spec.Nodes[i] = node
			return edited, nil
		}
	}
	workflow.Spec.Nodes = append(workflow.Spec.Nodes, node)
	return edited, nil
}

// DeleteWorkflowNode removes a node and all connected edges. UI metadata for
// the node and connected edge keys is also removed when present.
func DeleteWorkflowNode(bundle config.BundleSpec, workflowID, nodeID string) (config.BundleSpec, error) {
	edited, err := cloneBundle(bundle)
	if err != nil {
		return config.BundleSpec{}, err
	}
	workflow, err := workflowByID(&edited, workflowID)
	if err != nil {
		return config.BundleSpec{}, err
	}
	nodes := make([]flowengine.NodeSpec, 0, len(workflow.Spec.Nodes))
	found := false
	for _, node := range workflow.Spec.Nodes {
		if node.ID == nodeID {
			found = true
			continue
		}
		nodes = append(nodes, node)
	}
	if !found {
		return config.BundleSpec{}, fmt.Errorf("%w: workflow node %q", ErrResourceNotFound, nodeID)
	}
	edges := make([]flowengine.EdgeSpec, 0, len(workflow.Spec.Edges))
	removedEdgeKeys := []string{}
	for _, edge := range workflow.Spec.Edges {
		if edge.From == nodeID || edge.To == nodeID {
			removedEdgeKeys = append(removedEdgeKeys, edgeKey(edge))
			continue
		}
		edges = append(edges, edge)
	}
	workflow.Spec.Nodes = nodes
	workflow.Spec.Edges = edges
	removeWorkflowNodeUI(workflow.UI, nodeID)
	for _, key := range removedEdgeKeys {
		removeWorkflowEdgeUI(workflow.UI, key)
	}
	return edited, nil
}

// UpsertWorkflowEdge adds an edge unless the same from/to pair already exists.
func UpsertWorkflowEdge(bundle config.BundleSpec, workflowID string, edge flowengine.EdgeSpec) (config.BundleSpec, error) {
	if edge.From == "" || edge.To == "" {
		return config.BundleSpec{}, fmt.Errorf("%w: workflow edge endpoints are required", ErrInvalidPatchPath)
	}
	edited, err := cloneBundle(bundle)
	if err != nil {
		return config.BundleSpec{}, err
	}
	workflow, err := workflowByID(&edited, workflowID)
	if err != nil {
		return config.BundleSpec{}, err
	}
	if !workflowHasNode(*workflow, edge.From) || !workflowHasNode(*workflow, edge.To) {
		return config.BundleSpec{}, fmt.Errorf("%w: workflow edge references missing node", ErrResourceNotFound)
	}
	for _, existing := range workflow.Spec.Edges {
		if existing.From == edge.From && existing.To == edge.To {
			return edited, nil
		}
	}
	workflow.Spec.Edges = append(workflow.Spec.Edges, edge)
	return edited, nil
}

func DeleteWorkflowEdge(bundle config.BundleSpec, workflowID, from, to string) (config.BundleSpec, error) {
	edited, err := cloneBundle(bundle)
	if err != nil {
		return config.BundleSpec{}, err
	}
	workflow, err := workflowByID(&edited, workflowID)
	if err != nil {
		return config.BundleSpec{}, err
	}
	edges := make([]flowengine.EdgeSpec, 0, len(workflow.Spec.Edges))
	found := false
	for _, edge := range workflow.Spec.Edges {
		if edge.From == from && edge.To == to {
			found = true
			continue
		}
		edges = append(edges, edge)
	}
	if !found {
		return config.BundleSpec{}, fmt.Errorf("%w: workflow edge %q -> %q", ErrResourceNotFound, from, to)
	}
	workflow.Spec.Edges = edges
	removeWorkflowEdgeUI(workflow.UI, from+"->"+to)
	return edited, nil
}

func SetWorkflowContextSchema(bundle config.BundleSpec, workflowID string, schema flowengine.ContextSchema) (config.BundleSpec, error) {
	edited, err := cloneBundle(bundle)
	if err != nil {
		return config.BundleSpec{}, err
	}
	workflow, err := workflowByID(&edited, workflowID)
	if err != nil {
		return config.BundleSpec{}, err
	}
	workflow.Spec.ContextSchema = schema
	return edited, nil
}

func workflowByID(bundle *config.BundleSpec, workflowID string) (*config.WorkflowConfig, error) {
	for i := range bundle.Resources.Workflows {
		if bundle.Resources.Workflows[i].ID == workflowID {
			return &bundle.Resources.Workflows[i], nil
		}
	}
	return nil, fmt.Errorf("%w: workflow %q", ErrResourceNotFound, workflowID)
}

func workflowHasNode(workflow config.WorkflowConfig, nodeID string) bool {
	for _, node := range workflow.Spec.Nodes {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}

func cloneBundle(bundle config.BundleSpec) (config.BundleSpec, error) {
	data, err := json.Marshal(bundle)
	if err != nil {
		return config.BundleSpec{}, err
	}
	var out config.BundleSpec
	if err := json.Unmarshal(data, &out); err != nil {
		return config.BundleSpec{}, err
	}
	return out, nil
}

func edgeKey(edge flowengine.EdgeSpec) string {
	return edge.From + "->" + edge.To
}

func workflowNodeUI(ui map[string]any) map[string]any {
	nodes, _ := ui["nodes"].(map[string]any)
	if nodes == nil {
		return map[string]any{}
	}
	return nodes
}

func removeWorkflowNodeUI(ui map[string]any, nodeID string) {
	nodes, _ := ui["nodes"].(map[string]any)
	if nodes != nil {
		delete(nodes, nodeID)
	}
}

func removeWorkflowEdgeUI(ui map[string]any, key string) {
	edges, _ := ui["edges"].(map[string]any)
	if edges != nil {
		delete(edges, key)
	}
}
