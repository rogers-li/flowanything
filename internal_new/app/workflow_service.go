package app

import (
	"context"
	"fmt"
	"time"

	coreconfig "flow-anything/core/config"
	"flow-anything/core/flowengine"
	"flow-anything/core/runtimecontext"
	"flow-anything/core/workflow"
)

type WorkflowService struct {
	catalog  coreconfig.RuntimeCatalog
	compiler *workflow.Compiler
	runtime  *workflow.Runtime
}

type WorkflowRequest struct {
	WorkflowID   string
	Input        map[string]any
	TraceContext runtimecontext.TraceContext
}

type WorkflowResult struct {
	Instance flowengine.FlowInstance
	Output   map[string]any
}

// RunWorkflow compiles the current workflow document and executes it through
// core/workflow. Compilation is intentionally thin and should preserve the
// editor-visible FlowSpec semantics.
func (h *Host) RunWorkflow(ctx context.Context, req WorkflowRequest) (WorkflowResult, error) {
	return h.workflowService.Run(ctx, req)
}

func (s *WorkflowService) Run(ctx context.Context, req WorkflowRequest) (WorkflowResult, error) {
	if s.runtime == nil || s.compiler == nil {
		return WorkflowResult{}, fmt.Errorf("workflow runtime is not configured")
	}
	document, ok := s.catalog.Workflows[req.WorkflowID]
	if !ok {
		return WorkflowResult{}, fmt.Errorf("workflow %q not found", req.WorkflowID)
	}
	compiled, _, err := s.compiler.Compile(ctx, document)
	if err != nil {
		return WorkflowResult{}, err
	}
	if req.TraceContext.TraceID != "" {
		ctx = runtimecontext.WithTraceContext(ctx, req.TraceContext)
	}
	instance, err := s.runtime.Start(ctx, compiled, req.Input)
	if err != nil {
		return WorkflowResult{}, err
	}
	output := map[string]any{}
	if instance.Context != nil && instance.Context.FlowOutput != nil {
		output = instance.Context.FlowOutput
	}
	if isAgentWorkflowConfig(s.catalog.Bundle, req.WorkflowID) {
		output = fallbackAgentWorkflowOutput(output, instance, document)
	}
	return WorkflowResult{Instance: instance, Output: output}, nil
}

func isAgentWorkflowConfig(bundle coreconfig.BundleSpec, workflowID string) bool {
	for _, workflowConfig := range bundle.Resources.Workflows {
		if workflowConfig.ID != workflowID {
			continue
		}
		return stringValue(workflowConfig.UI, "orchestration_mode") == "workflow"
	}
	return false
}

func fallbackAgentWorkflowOutput(output map[string]any, instance flowengine.FlowInstance, document workflow.WorkflowDocument) map[string]any {
	if output == nil {
		output = map[string]any{}
	}
	if userFacingText(output) != "" {
		return output
	}
	nodeTypeByID := map[string]string{}
	for _, node := range document.Spec.Nodes {
		nodeTypeByID[node.ID] = node.Type
	}
	text, ok := latestAgentNodeText(instance.NodeStates, nodeTypeByID)
	if ok {
		output["return_message"] = text
	}
	return output
}

func latestAgentNodeText(states map[string]flowengine.NodeState, nodeTypeByID map[string]string) (string, bool) {
	latestText := ""
	latestAt := time.Time{}
	for nodeID, state := range states {
		if nodeTypeByID[nodeID] != workflow.NodeTypeAgent || state.Status != flowengine.NodeCompleted {
			continue
		}
		text := userFacingText(state.Output)
		if text == "" {
			continue
		}
		if latestText == "" || state.FinishedAt.After(latestAt) {
			latestText = text
			latestAt = state.FinishedAt
		}
	}
	return latestText, latestText != ""
}

func userFacingText(output map[string]any) string {
	for _, key := range []string{"return_message", "text", "answer", "message", "final_answer", "result"} {
		if value := stringValue(output, key); value != "" {
			return value
		}
	}
	return ""
}
