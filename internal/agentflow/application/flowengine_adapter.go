package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"flow-anything/internal/agentflow/domain"
	"flow-anything/internal/agentflow/ports"
	"flow-anything/internal/flowengine"
	"flow-anything/internal/platform/contracts/workflow"
	"flow-anything/internal/platform/kernel/id"
	"flow-anything/internal/platform/kernel/tenant"
)

const agentNodeRuntimeSystemPromptConfigKey = "runtime_system_prompt"

func (e *Executor) canExecuteWithFlowEngine(graph domain.FlowGraph) bool {
	for _, node := range graph.Nodes {
		switch node.Type {
		case domain.NodeTypeStart,
			domain.NodeTypeJoin,
			domain.NodeTypeWorkflowJoin,
			domain.NodeTypeEnd,
			domain.NodeTypeAgent,
			domain.NodeTypeSupervisor,
			domain.NodeTypeWorkflowAgent,
			domain.NodeTypeConnectorOperation,
			domain.NodeTypeTool,
			domain.NodeTypeTransform,
			domain.NodeTypeCondition:
		default:
			return false
		}
	}
	return true
}

func (e *Executor) executeWithFlowEngine(ctx context.Context, graph domain.FlowGraph, input map[string]any) (domain.FlowRun, error) {
	store := &agentFlowRunStoreAdapter{store: e.store}
	registry := e.flowEngineRegistry
	if registry == nil {
		registry = flowengine.NewNodeRegistry()
	}
	if executor, ok := e.agentNodeExecutor(); ok {
		adapter := &agentNodeFlowEngineAdapter{executor: executor}
		registry.Register(workflow.NodeTypeAgent, adapter)
	}

	engine := flowengine.NewExecutor(e.logger, store, registry)
	spec := workflow.Spec{
		ID:       graph.ID,
		TenantID: graph.TenantID,
		Name:     graph.Name,
		Status:   workflow.StatusEnabled,
		Profile:  workflow.ProfileAgentWorkflow,
		Graph:    convertAgentFlowGraph(graph),
		Policy: workflow.ExecutionPolicy{
			MaxSteps:       graph.Policy.MaxSteps,
			MaxParallelism: graph.Policy.MaxParallelism,
			TimeoutMillis:  graph.Policy.TimeoutMillis,
		},
		Version: graph.Version,
	}

	run, err := engine.Execute(ctx, spec, input, input, "")
	return convertWorkflowRun(run), err
}

func (e *Executor) agentNodeExecutor() (ports.NodeExecutor, bool) {
	if e.registry == nil {
		return nil, false
	}
	e.registry.mu.RLock()
	defer e.registry.mu.RUnlock()
	executor, ok := e.registry.executors[domain.NodeTypeAgent]
	return executor, ok
}

func convertAgentFlowGraph(graph domain.FlowGraph) workflow.Graph {
	nodes := make(map[id.ID]workflow.Node, len(graph.Nodes))
	for nodeID, node := range graph.Nodes {
		config := cloneConfig(node.Config)
		config["_agentflow_node_type"] = string(node.Type)
		nodes[nodeID] = workflow.Node{
			ID:            node.ID,
			Type:          convertAgentFlowNodeType(node.Type),
			Name:          node.Name,
			Description:   node.Description,
			Config:        config,
			TimeoutMillis: node.TimeoutMillis,
			RetryPolicy: workflow.RetryPolicy{
				MaxAttempts:   node.RetryPolicy.MaxAttempts,
				BackoffMillis: node.RetryPolicy.BackoffMillis,
			},
		}
	}
	edges := make([]workflow.Edge, 0, len(graph.Edges))
	for _, edge := range graph.Edges {
		edges = append(edges, workflow.Edge{
			ID:          edge.ID,
			FromNodeID:  edge.FromNodeID,
			ToNodeID:    edge.ToNodeID,
			Type:        workflow.EdgeType(edge.Type),
			Condition:   convertAgentFlowEdgeCondition(edge.Condition),
			Description: edge.Description,
		})
	}
	return workflow.Graph{
		EntryNodeID: graph.EntryNodeID,
		Nodes:       nodes,
		Edges:       edges,
	}
}

func convertAgentFlowNodeType(nodeType domain.NodeType) workflow.NodeType {
	switch nodeType {
	case domain.NodeTypeStart:
		return workflow.NodeTypeStart
	case domain.NodeTypeJoin:
		return workflow.NodeTypeJoin
	case domain.NodeTypeAgent, domain.NodeTypeSupervisor, domain.NodeTypeWorkflowAgent:
		return workflow.NodeTypeAgent
	case domain.NodeTypeWorkflowJoin:
		return workflow.NodeTypeJoin
	default:
		return workflow.NodeType(nodeType)
	}
}

func convertAgentFlowEdgeCondition(condition *domain.EdgeCondition) *workflow.EdgeCondition {
	if condition == nil {
		return nil
	}
	return &workflow.EdgeCondition{
		Path:   condition.Path,
		Equals: condition.Equals,
		Exists: condition.Exists,
	}
}

type agentNodeFlowEngineAdapter struct {
	executor ports.NodeExecutor
}

func (a *agentNodeFlowEngineAdapter) ExecuteNode(ctx context.Context, request flowengine.NodeExecutionRequest) (flowengine.NodeResult, error) {
	agentNode := domain.Node{
		ID:            request.Node.ID,
		Type:          domain.NodeType(fmt.Sprint(request.Node.Config["_agentflow_node_type"])),
		Name:          request.Node.Name,
		Description:   request.Node.Description,
		Config:        cloneConfig(request.Node.Config),
		TimeoutMillis: request.Node.TimeoutMillis,
		RetryPolicy: domain.RetryPolicy{
			MaxAttempts:   request.Node.RetryPolicy.MaxAttempts,
			BackoffMillis: request.Node.RetryPolicy.BackoffMillis,
		},
	}
	if agentNode.Type == "" {
		agentNode.Type = domain.NodeTypeAgent
	}
	delete(agentNode.Config, "_agentflow_node_type")
	if runtimePrompt := buildAgentWorkflowRuntimePrompt(request.Node, request.Graph, request.Input); runtimePrompt != "" {
		agentNode.Config[agentNodeRuntimeSystemPromptConfigKey] = runtimePrompt
	}

	result, err := a.executor.ExecuteNode(ctx, ports.NodeExecutionRequest{
		Run:     convertWorkflowRun(request.Run),
		Node:    agentNode,
		Context: convertWorkflowContext(request.Context, request.Input),
	})
	if err != nil {
		return flowengine.NodeResult{}, err
	}

	writes := map[string]any{}
	if !hasExplicitContextWrites(request.Node.Config) {
		if text, ok := result.Output["text"].(string); ok && strings.TrimSpace(text) != "" {
			writes["flow_output.return_message"] = strings.TrimSpace(text)
		}
		for key, value := range result.Variables {
			writes["variables."+key] = value
		}
	}
	return flowengine.NodeResult{
		Output:        result.Output,
		ContextWrites: writes,
		ResponseWrites: map[string]any{
			"responses.agent." + agentResponseAlias(request.Node): cloneConfig(result.Output),
		},
		NextNodeIDs: result.NextNodeIDs,
		Stop:        result.Stop,
	}, nil
}

func convertWorkflowContext(ctx flowengine.RunContext, nodeInput map[string]any) domain.RunContext {
	variables := map[string]any{}
	for key, value := range ctx.Ctx {
		variables[key] = value
	}
	if runtimeVariables, ok := ctx.Ctx["variables"].(map[string]any); ok {
		for key, value := range runtimeVariables {
			variables[key] = value
		}
	}
	for key, value := range ctx.Vars {
		variables[key] = value
	}
	return domain.RunContext{
		Input:       cloneConfig(nodeInput),
		Variables:   variables,
		NodeOutputs: map[id.ID]map[string]any{},
	}
}

func hasExplicitContextWrites(config map[string]any) bool {
	for _, key := range []string{"write_context", "context_writes"} {
		value, ok := config[key]
		if !ok {
			continue
		}
		typed, ok := value.(map[string]any)
		if ok && len(typed) > 0 {
			return true
		}
	}
	return false
}

func agentResponseAlias(node workflow.Node) string {
	if alias, ok := node.Config["response_alias"].(string); ok && alias != "" {
		return alias
	}
	if agentID, ok := node.Config["agent_id"].(string); ok && agentID != "" {
		return agentID
	}
	return node.ID.String()
}

func buildAgentWorkflowRuntimePrompt(node workflow.Node, graph workflow.Graph, input map[string]any) string {
	var builder strings.Builder
	builder.WriteString("你正在 Agent Workflow 中作为一个 Agent Node 执行。用户配置的 Agent Prompt 负责业务角色与业务规则；以下 Runtime Instructions 由平台根据画布配置自动生成，负责输入输出协议和下一步路由协议。\n\n")
	builder.WriteString("Current Node:\n")
	writeRuntimePromptLine(&builder, "- node_id: "+node.ID.String())
	writeRuntimePromptLine(&builder, "- node_name: "+nonEmptyString(node.Name, node.ID.String()))
	if strings.TrimSpace(node.Description) != "" {
		writeRuntimePromptLine(&builder, "- node_description: "+strings.TrimSpace(node.Description))
	}

	builder.WriteString("\nInput Contract:\n")
	inputFields := schemaFieldsFromSchema(node.Config["input_schema"])
	if len(inputFields) > 0 {
		for _, field := range inputFields {
			line := fmt.Sprintf("- %s: %s", field.Path, nonEmptyString(field.Type, "any"))
			if field.Required {
				line += " required"
			}
			if strings.TrimSpace(field.Description) != "" {
				line += " - " + strings.TrimSpace(field.Description)
			}
			writeRuntimePromptLine(&builder, line)
		}
	} else if len(input) == 0 {
		writeRuntimePromptLine(&builder, "- 当前节点没有显式 input fields。")
	} else {
		for _, key := range sortedMapKeys(input) {
			writeRuntimePromptLine(&builder, fmt.Sprintf("- %s: %s", key, valueTypeName(input[key])))
		}
	}
	writeRuntimePromptLine(&builder, "- 你应该基于以上 input fields 和字段描述理解当前节点任务；不要依赖固定字段名。")

	outputMode := strings.ToLower(configString(node.Config, "output_mode"))
	agentDirected := agentRoutingEnabledForWorkflowNode(node.Config)
	outputFields := expectedJSONOutputFields(node.Config, agentDirected, outputMode == "json" || agentDirected)
	builder.WriteString("\nOutput Contract:\n")
	if outputMode == "json" || agentDirected || len(outputFields) > 0 {
		writeRuntimePromptLine(&builder, "- 必须返回 JSON object。不要使用 markdown，不要包裹 ```。")
		writeRuntimePromptLine(&builder, "- 平台会把 JSON 顶层字段作为当前 Agent Node 的 output；Workflow 的 Write Context 可用 $.字段名 读取。")
		if len(outputFields) > 0 {
			writeRuntimePromptLine(&builder, "- JSON 顶层字段至少应包含：")
			for _, field := range outputFields {
				writeRuntimePromptLine(&builder, "  - "+field)
			}
		} else {
			writeRuntimePromptLine(&builder, "- JSON 中应包含当前节点的处理结果字段，例如 answer。")
		}
	} else {
		writeRuntimePromptLine(&builder, "- 当前节点输出模式为 text，可以自然语言返回当前节点处理结果。")
		writeRuntimePromptLine(&builder, "- 平台会把自然语言结果保存为当前 Agent Node 的 output.text；Workflow 的 Write Context 可用 $.text 读取。")
	}

	builder.WriteString("\nRouting Contract:\n")
	nextNodes := outgoingWorkflowNodes(node.ID, graph)
	if agentDirected {
		writeRuntimePromptLine(&builder, "- 当前节点启用了 Agent Directed Routing。你可以通过 JSON 字段 next_node_ids 决策下一步执行哪些已连接下游节点。")
		writeRuntimePromptLine(&builder, "- next_node_ids 必须是字符串数组；如果不需要继续执行，返回空数组 []。")
		writeRuntimePromptLine(&builder, "- 只能选择 Available Next Nodes 中列出的 node_id，不能编造或选择未连接节点。")
		writeRuntimePromptLine(&builder, "- reason 字段用于解释你的路由决策。")
		writeRuntimePromptLine(&builder, "\nAvailable Next Nodes:")
		if len(nextNodes) == 0 {
			writeRuntimePromptLine(&builder, "- 无可选下游节点。next_node_ids 必须返回 []。")
		} else {
			for _, next := range nextNodes {
				line := fmt.Sprintf("- node_id: %s; type: %s; name: %s", next.ID, next.Type, nonEmptyString(next.Name, next.ID.String()))
				if strings.TrimSpace(next.Description) != "" {
					line += "; description: " + strings.TrimSpace(next.Description)
				}
				writeRuntimePromptLine(&builder, line)
			}
		}
		writeRuntimePromptLine(&builder, "\nRecommended JSON shape:")
		writeRuntimePromptLine(&builder, recommendedAgentWorkflowJSONShape(outputFields))
	} else {
		writeRuntimePromptLine(&builder, "- 当前节点下一步由 Workflow 画布连线和 Condition 节点控制。")
		writeRuntimePromptLine(&builder, "- 不要输出 next_node_ids 作为路由决策；如需说明建议，可放在自然语言或业务 JSON 字段中。")
	}

	return builder.String()
}

func outgoingWorkflowNodes(nodeID id.ID, graph workflow.Graph) []workflow.Node {
	result := make([]workflow.Node, 0)
	for _, edge := range graph.Edges {
		if edge.FromNodeID != nodeID {
			continue
		}
		next, ok := graph.Nodes[edge.ToNodeID]
		if ok {
			result = append(result, next)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].ID.String() < result[j].ID.String()
	})
	return result
}

func expectedJSONOutputFields(config map[string]any, agentDirected bool, includeMappingFields bool) []string {
	fields := map[string]struct{}{}
	if agentDirected {
		fields["answer"] = struct{}{}
		fields["next_node_ids"] = struct{}{}
		fields["reason"] = struct{}{}
	}
	for _, field := range jsonFieldsFromOutputSchema(config["output_schema"]) {
		fields[field] = struct{}{}
	}
	if includeMappingFields {
		for _, key := range []string{"output_mapping", "write_context", "context_writes"} {
			mapping, ok := config[key].(map[string]any)
			if !ok {
				continue
			}
			for target, source := range mapping {
				for _, field := range jsonFieldsFromMappingValue(target, source) {
					fields[field] = struct{}{}
				}
			}
		}
	}
	result := make([]string, 0, len(fields))
	for field := range fields {
		result = append(result, field)
	}
	sort.Strings(result)
	return result
}

func jsonFieldsFromOutputSchema(value any) []string {
	schemaFields := schemaFieldsFromSchema(value)
	if len(schemaFields) > 0 {
		fields := make([]string, 0, len(schemaFields))
		for _, field := range schemaFields {
			fields = append(fields, topLevelJSONField(field.Path))
		}
		return fields
	}
	return nil
}

type schemaField struct {
	Path        string
	Type        string
	Description string
	Required    bool
}

func schemaFieldsFromSchema(value any) []schemaField {
	schema, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	fields := make([]schemaField, 0)
	if rawFields, ok := schema["x-flow-fields"].([]any); ok {
		for _, item := range rawFields {
			field, ok := item.(map[string]any)
			if !ok {
				continue
			}
			path := strings.TrimSpace(fmt.Sprint(field["path"]))
			if path != "" && path != "<nil>" {
				fields = append(fields, schemaField{
					Path:        path,
					Type:        cleanSchemaString(field["type"]),
					Description: cleanSchemaString(field["description"]),
					Required:    boolFromAny(field["required"]),
				})
			}
		}
	}
	if len(fields) == 0 {
		if properties, ok := schema["properties"].(map[string]any); ok {
			required := stringSetFromAny(schema["required"])
			for name := range properties {
				if clean := strings.TrimSpace(name); clean != "" {
					property, _ := properties[name].(map[string]any)
					fields = append(fields, schemaField{
						Path:        clean,
						Type:        cleanSchemaString(property["type"]),
						Description: cleanSchemaString(property["description"]),
						Required:    required[clean],
					})
				}
			}
		}
	}
	sort.SliceStable(fields, func(i, j int) bool {
		return fields[i].Path < fields[j].Path
	})
	return fields
}

func cleanSchemaString(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func boolFromAny(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}

func stringSetFromAny(value any) map[string]bool {
	result := map[string]bool{}
	values, ok := value.([]any)
	if !ok {
		return result
	}
	for _, item := range values {
		if text := strings.TrimSpace(fmt.Sprint(item)); text != "" && text != "<nil>" {
			result[text] = true
		}
	}
	return result
}

func recommendedAgentWorkflowJSONShape(fields []string) string {
	if len(fields) == 0 {
		return `{"answer":"当前节点处理结果"}`
	}
	seen := map[string]struct{}{}
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		field = topLevelJSONField(field)
		if field == "" {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		value := `"请根据字段含义填写"`
		switch field {
		case "answer", "text", "final_answer", "return_message":
			value = `"当前节点处理结果"`
		case "reason":
			value = `"选择这些下一节点的原因"`
		case "next_node_ids", "next_nodes":
			value = `["下游节点ID"]`
		}
		parts = append(parts, fmt.Sprintf("%q:%s", field, value))
	}
	if len(parts) == 0 {
		return `{"answer":"当前节点处理结果"}`
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func jsonFieldsFromMappingValue(target string, source any) []string {
	text, ok := source.(string)
	if !ok {
		return nil
	}
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "$.") {
		field := topLevelJSONField(strings.TrimPrefix(text, "$."))
		if field != "" {
			return []string{field}
		}
	}
	if strings.HasPrefix(text, "$output.") {
		field := strings.TrimSpace(target)
		if field != "" {
			return []string{field}
		}
	}
	return nil
}

func topLevelJSONField(path string) string {
	field := strings.TrimSpace(path)
	if dot := strings.IndexByte(field, '.'); dot >= 0 {
		field = field[:dot]
	}
	return strings.TrimSpace(field)
}

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func valueTypeName(value any) string {
	switch typed := value.(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		if typed == nil {
			return "null"
		}
		return fmt.Sprintf("%T", typed)
	}
}

func writeRuntimePromptLine(builder *strings.Builder, line string) {
	builder.WriteString(line)
	builder.WriteByte('\n')
}

func configString(config map[string]any, key string) string {
	value, ok := config[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func nonEmptyString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func agentRoutingEnabledForWorkflowNode(config map[string]any) bool {
	if value, ok := config["agent_directed_routing"].(bool); ok && value {
		return true
	}
	mode := strings.ToLower(configString(config, "agent_routing_mode"))
	return mode == "agent_directed" || mode == "agent-directed"
}

type agentFlowRunStoreAdapter struct {
	store ports.RunStore
}

func (s *agentFlowRunStoreAdapter) CreateRun(ctx context.Context, run workflow.Run) error {
	return s.store.CreateRun(ctx, convertWorkflowRun(run))
}

func (s *agentFlowRunStoreAdapter) UpdateRun(ctx context.Context, run workflow.Run) error {
	return s.store.UpdateRun(ctx, convertWorkflowRun(run))
}

func (s *agentFlowRunStoreAdapter) GetRun(ctx context.Context, tenantID tenant.ID, runID id.ID) (workflow.Run, error) {
	agentRun, err := s.store.GetRun(ctx, tenantID, runID)
	if err != nil {
		return workflow.Run{}, err
	}
	return workflow.Run{
		ID:         agentRun.ID,
		TenantID:   agentRun.TenantID,
		WorkflowID: agentRun.FlowID,
		Version:    agentRun.FlowVersion,
		Status:     workflow.RunStatus(agentRun.Status),
		Input:      cloneConfig(agentRun.Input),
		Output:     cloneConfig(agentRun.Output),
		Error:      agentRun.Error,
		StartedAt:  agentRun.StartedAt,
		FinishedAt: agentRun.FinishedAt,
	}, nil
}

func (s *agentFlowRunStoreAdapter) ListRuns(ctx context.Context, tenantID tenant.ID, workflowID id.ID, limit int) ([]workflow.Run, error) {
	return nil, nil
}

func (s *agentFlowRunStoreAdapter) RecordNodeRun(ctx context.Context, nodeRun workflow.NodeRun) error {
	return s.store.RecordNodeRun(ctx, domain.NodeRun{
		ID:         nodeRun.ID,
		TenantID:   nodeRun.TenantID,
		RunID:      nodeRun.RunID,
		FlowID:     nodeRun.WorkflowID,
		NodeID:     nodeRun.NodeID,
		NodeType:   domain.NodeType(nodeRun.NodeType),
		NodeName:   nodeRun.NodeName,
		Status:     domain.NodeRunStatus(nodeRun.Status),
		Input:      cloneConfig(nodeRun.Input),
		Output:     cloneConfig(nodeRun.Output),
		Error:      nodeRun.Error,
		StartedAt:  nodeRun.StartedAt,
		FinishedAt: nodeRun.FinishedAt,
	})
}

func (s *agentFlowRunStoreAdapter) ListNodeRuns(ctx context.Context, tenantID tenant.ID, runID id.ID) ([]workflow.NodeRun, error) {
	agentRuns, err := s.store.ListNodeRuns(ctx, tenantID, runID)
	if err != nil {
		return nil, err
	}
	result := make([]workflow.NodeRun, 0, len(agentRuns))
	for _, nodeRun := range agentRuns {
		result = append(result, workflow.NodeRun{
			ID:         nodeRun.ID,
			TenantID:   nodeRun.TenantID,
			RunID:      nodeRun.RunID,
			WorkflowID: nodeRun.FlowID,
			NodeID:     nodeRun.NodeID,
			NodeType:   workflow.NodeType(nodeRun.NodeType),
			NodeName:   nodeRun.NodeName,
			Status:     workflow.NodeRunStatus(nodeRun.Status),
			Input:      cloneConfig(nodeRun.Input),
			Output:     cloneConfig(nodeRun.Output),
			Error:      nodeRun.Error,
			StartedAt:  nodeRun.StartedAt,
			FinishedAt: nodeRun.FinishedAt,
		})
	}
	return result, nil
}

func convertWorkflowRun(run workflow.Run) domain.FlowRun {
	return domain.FlowRun{
		ID:          run.ID,
		TenantID:    run.TenantID,
		FlowID:      run.WorkflowID,
		FlowVersion: run.Version,
		Status:      domain.RunStatus(run.Status),
		Input:       cloneConfig(run.Input),
		Output:      cloneConfig(run.Output),
		Error:       run.Error,
		StartedAt:   run.StartedAt,
		FinishedAt:  run.FinishedAt,
	}
}

func cloneConfig(source map[string]any) map[string]any {
	if source == nil {
		return map[string]any{}
	}
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
