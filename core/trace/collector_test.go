package trace

import (
	"context"
	"testing"
	"time"

	agentcore "flow-anything/core/agentcore"
	connectorcore "flow-anything/core/connector"
	"flow-anything/core/flowengine"
	toolscore "flow-anything/core/tools"
)

func TestCollectorBuildsFlowTree(t *testing.T) {
	store := NewMemoryStore()
	collector := NewCollector(store)
	base := time.Unix(100, 0).UTC()
	ctx := context.Background()

	collector.OnFlowEvent(ctx, flowengine.FlowEvent{
		Type: flowengine.EventFlowStarted, RunID: "run_1", FlowID: "flow_1",
		Input: map[string]any{"user_request": "hello"}, Timestamp: base,
	})
	collector.OnFlowEvent(ctx, flowengine.FlowEvent{
		Type: flowengine.EventNodeStarted, RunID: "run_1", FlowID: "flow_1", NodeID: "node_agent", NodeType: "workflow.agent",
		Input: map[string]any{"message": "hello"}, Data: map[string]any{"node_name": "Agent Node"}, Timestamp: base.Add(time.Second),
	})
	collector.OnFlowEvent(ctx, flowengine.FlowEvent{
		Type: flowengine.EventNodeCompleted, RunID: "run_1", FlowID: "flow_1", NodeID: "node_agent", NodeType: "workflow.agent",
		Output: map[string]any{"answer": "hi"}, Timestamp: base.Add(2 * time.Second),
	})
	collector.OnFlowEvent(ctx, flowengine.FlowEvent{
		Type: flowengine.EventFlowCompleted, RunID: "run_1", FlowID: "flow_1",
		Output: map[string]any{"return_message": "hi"}, Timestamp: base.Add(3 * time.Second),
	})

	trace, err := store.GetTrace(ctx, "run_1")
	if err != nil {
		t.Fatal(err)
	}
	tree := BuildTree(trace.Spans)
	if len(tree) != 1 {
		t.Fatalf("expected one root, got %#v", tree)
	}
	if tree[0].Span.Kind != SpanKindFlow || tree[0].Span.Status != SpanStatusOK {
		t.Fatalf("unexpected root span: %#v", tree[0].Span)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].Span.Kind != SpanKindNode {
		t.Fatalf("expected node child: %#v", tree)
	}
	if tree[0].Children[0].Span.Name != "Agent Node" {
		t.Fatalf("expected node display name, got %q", tree[0].Children[0].Span.Name)
	}
	if tree[0].Children[0].Span.Output["answer"] != "hi" {
		t.Fatalf("unexpected node output: %#v", tree[0].Children[0].Span.Output)
	}
}

func TestCollectorLinksAgentToolConnectorWithTraceContext(t *testing.T) {
	store := NewMemoryStore()
	collector := NewCollector(store)
	base := time.Unix(200, 0).UTC()
	traceID := "trace_1"
	flowNodeSpanID := nodeSpanID("run_1", "node_agent")
	agentCtx := WithTraceContext(context.Background(), TraceContext{TraceID: traceID, ParentSpanID: flowNodeSpanID})

	collector.OnAgentEvent(agentCtx, agentcore.AgentEvent{
		Type: agentcore.EventAgentStarted, TraceID: traceID, AgentID: "agent_web", Strategy: "action-planning", Timestamp: base,
	})
	collector.OnAgentEvent(agentCtx, agentcore.AgentEvent{
		Type: agentcore.EventPlanningStarted, TraceID: traceID, AgentID: "agent_web", Strategy: "action-planning", Timestamp: base.Add(time.Second),
		Data: map[string]any{"api_key": "secret-value"},
	})
	collector.OnAgentEvent(agentCtx, agentcore.AgentEvent{
		Type: agentcore.EventPlanningCompleted, TraceID: traceID, AgentID: "agent_web", Strategy: "action-planning", Timestamp: base.Add(2 * time.Second),
		Data: map[string]any{"actions": []any{"search"}},
	})
	capabilitySpan := capabilitySpanID(traceID, "agent_web", "tool", "tool_search")
	collector.OnAgentEvent(agentCtx, agentcore.AgentEvent{
		Type: agentcore.EventCapabilityStarted, TraceID: traceID, AgentID: "agent_web", Strategy: "action-planning",
		CapabilityID: "tool_search", CapabilityType: "tool", Timestamp: base.Add(3 * time.Second),
	})

	toolCtx := WithTraceContext(context.Background(), TraceContext{TraceID: traceID, ParentSpanID: capabilitySpan})
	collector.OnToolEvent(toolCtx, toolscore.ToolEvent{
		Type: toolscore.EventToolStarted, TraceID: traceID, CallID: "toolcall_1", ToolID: "tool_search", ToolType: toolscore.ToolTypeConnector,
		Kind: "connector", Input: map[string]any{"query": "ai", "authorization": "bearer secret"}, Timestamp: base.Add(4 * time.Second),
	})
	toolSpan := toolSpanID(traceID, "toolcall_1")
	connectorCtx := WithTraceContext(context.Background(), TraceContext{TraceID: traceID, ParentSpanID: toolSpan})
	collector.OnConnectorEvent(connectorCtx, connectorcore.ConnectorEvent{
		Type: connectorcore.EventInvokeStarted, TraceID: traceID, CallID: "conncall_1", ConnectorID: "conn_search", OperationID: "search",
		Protocol: "http", Input: map[string]any{"query": "ai"}, Timestamp: base.Add(5 * time.Second),
	})
	collector.OnConnectorEvent(connectorCtx, connectorcore.ConnectorEvent{
		Type: connectorcore.EventInvokeCompleted, TraceID: traceID, CallID: "conncall_1", ConnectorID: "conn_search", OperationID: "search",
		Protocol: "http", Output: map[string]any{"results": []any{"news"}}, Timestamp: base.Add(6 * time.Second),
	})
	collector.OnToolEvent(toolCtx, toolscore.ToolEvent{
		Type: toolscore.EventToolCompleted, TraceID: traceID, CallID: "toolcall_1", ToolID: "tool_search", ToolType: toolscore.ToolTypeConnector,
		Kind: "connector", Output: map[string]any{"summary": "news"}, Timestamp: base.Add(7 * time.Second),
	})
	collector.OnAgentEvent(agentCtx, agentcore.AgentEvent{
		Type: agentcore.EventCapabilityCompleted, TraceID: traceID, AgentID: "agent_web", Strategy: "action-planning",
		CapabilityID: "tool_search", CapabilityType: "tool", Timestamp: base.Add(8 * time.Second),
	})
	skillCapabilitySpan := capabilitySpanID(traceID, "agent_web", "skill", "skill_web")
	collector.OnAgentEvent(agentCtx, agentcore.AgentEvent{
		Type: agentcore.EventCapabilityStarted, TraceID: traceID, AgentID: "agent_web", Strategy: "action-planning",
		CapabilityID: "skill_web", CapabilityType: "skill", Timestamp: base.Add(8*time.Second + time.Millisecond),
	})
	collector.OnAgentEvent(agentCtx, agentcore.AgentEvent{
		Type: agentcore.EventCapabilityCompleted, TraceID: traceID, AgentID: "agent_web", Strategy: "action-planning",
		CapabilityID: "skill_web", CapabilityType: "skill", Timestamp: base.Add(8*time.Second + 2*time.Millisecond),
	})
	collector.OnAgentEvent(agentCtx, agentcore.AgentEvent{
		Type: agentcore.EventAgentCompleted, TraceID: traceID, AgentID: "agent_web", Strategy: "action-planning",
		Data: map[string]any{"text": "done"}, Timestamp: base.Add(9 * time.Second),
	})

	trace, err := store.GetTrace(context.Background(), traceID)
	if err != nil {
		t.Fatal(err)
	}
	spansByID := map[string]Span{}
	for _, span := range trace.Spans {
		spansByID[span.SpanID] = span
	}
	agentSpan := spansByID[agentSpanID(traceID, "agent_web")]
	if agentSpan.ParentSpanID != flowNodeSpanID {
		t.Fatalf("agent parent should be flow node, got %q", agentSpan.ParentSpanID)
	}
	toolSpanValue := spansByID[toolSpan]
	if toolSpanValue.ParentSpanID != capabilitySpan {
		t.Fatalf("tool parent should be capability, got %q", toolSpanValue.ParentSpanID)
	}
	connectorSpan := spansByID[connectorSpanID(traceID, "conncall_1")]
	if connectorSpan.ParentSpanID != toolSpan {
		t.Fatalf("connector parent should be tool, got %q", connectorSpan.ParentSpanID)
	}
	if toolSpanValue.Input["authorization"] != "[redacted]" {
		t.Fatalf("tool input should be redacted: %#v", toolSpanValue.Input)
	}
	if skillSpan := spansByID[skillCapabilitySpan]; skillSpan.Kind != SpanKindSkill {
		t.Fatalf("skill capability should create skill span, got %#v", skillSpan)
	}
	planningSpan := spansByID[planningSpanID(traceID, "agent_web")]
	if planningSpan.Input["api_key"] != "[redacted]" {
		t.Fatalf("planning input should be redacted: %#v", planningSpan.Input)
	}
}

func TestBuildTreeKeepsGrandchildren(t *testing.T) {
	base := time.Unix(300, 0).UTC()
	tree := BuildTree([]Span{
		{TraceID: "t", SpanID: "root", Kind: SpanKindFlow, StartedAt: base},
		{TraceID: "t", SpanID: "child", ParentSpanID: "root", Kind: SpanKindNode, StartedAt: base.Add(time.Second)},
		{TraceID: "t", SpanID: "grandchild", ParentSpanID: "child", Kind: SpanKindTool, StartedAt: base.Add(2 * time.Second)},
	})
	if len(tree) != 1 || len(tree[0].Children) != 1 || len(tree[0].Children[0].Children) != 1 {
		t.Fatalf("expected root -> child -> grandchild tree, got %#v", tree)
	}
}
