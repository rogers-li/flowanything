import type { Edge, Node } from "@xyflow/react";
import { MarkerType } from "@xyflow/react";
import type { FlowDeletableEdgeData } from "../../components/FlowDeletableEdge";
import { defaultTenantId } from "../../lib/api";
import type { AgentFlowEdge, AgentFlowGraph, AgentFlowNode, AgentFlowNodeType, AgentFlowSpec } from "../../types/platform";

export type LocalAgentConfig = {
  name: string;
  description: string;
  model: string;
  systemPrompt: string;
  toolIds: string[];
  skillIds: string[];
};

export type AgentFlowCanvasNodeData = {
  label: string;
  description?: string;
  nodeType: AgentFlowNodeType;
  agentId?: string;
  agentMode?: "existing" | "local";
  localAgent?: LocalAgentConfig;
  config?: Record<string, unknown>;
  timeoutMillis?: number;
  deletable?: boolean;
  onDelete?: (nodeId: string) => void;
};

export type AgentFlowCanvasNode = Node<AgentFlowCanvasNodeData, "agentFlowNode">;
export type AgentFlowCanvasEdge = Edge<FlowDeletableEdgeData & { edgeType?: AgentFlowEdge["type"] }>;

const defaultPositionStep = 220;

export const agentFlowInputSchema = {
  type: "object",
  description: "Agent Flow user-facing input contract.",
  properties: {
    user_request: {
      type: "string",
      description: "The raw request text submitted by the user."
    }
  },
  required: ["user_request"],
  "x-flow-fields": [
    {
      path: "user_request",
      type: "string",
      description: "The raw request text submitted by the user.",
      required: true
    }
  ]
};

export const agentFlowOutputSchema = {
  type: "object",
  description: "Agent Flow user-facing output contract.",
  properties: {
    return_message: {
      type: "string",
      description: "The final message returned to the user."
    }
  },
  required: ["return_message"],
  "x-flow-fields": [
    {
      path: "return_message",
      type: "string",
      description: "The final message returned to the user.",
      required: true
    }
  ]
};

export function createAgentFlowDraft(): AgentFlowSpec {
  const graph = createDefaultGraph("", "Untitled Agent Flow", "Draft multi-agent orchestration flow.");
  return {
    id: "",
    tenantId: defaultTenantId,
    name: graph.name,
    description: graph.description,
    businessDomain: "General",
    ownerTeam: "AI Platform",
    status: "draft",
    orchestrationMode: "workflow",
    supervisor: {
      supervisorAgentId: undefined,
      subAgentIds: [],
      maxDepth: 4,
      maxSubAgentCalls: 5
    },
    graph,
    contextSchema: {},
    inputSchema: agentFlowInputSchema,
    outputSchema: agentFlowOutputSchema,
    version: "v1"
  };
}

export function createSupervisorAgentFlowDraft(): AgentFlowSpec {
  const flow = createAgentFlowDraft();
  const supervisorNode: AgentFlowNode = {
    id: "supervisor",
    type: "supervisor_node",
    name: "Supervisor",
    description: "Coordinate Sub-Agents and synthesize their results.",
    config: {
      position: { x: 360, y: 180 }
    },
    timeoutMillis: 90000
  };

  return {
    ...flow,
    name: "Untitled Agent Graph",
    description: "Recursive Agent Graph orchestration flow.",
    orchestrationMode: "supervisor",
    supervisor: {
      supervisorAgentId: undefined,
      subAgentIds: [],
      maxDepth: 4,
      maxSubAgentCalls: 5
    },
    graph: {
      ...flow.graph,
      name: "Untitled Agent Graph",
      description: "Recursive Agent Graph orchestration flow.",
      nodes: {
        ...flow.graph.nodes,
        [supervisorNode.id]: supervisorNode
      },
      edges: [
        {
          id: "start-supervisor",
          fromNodeId: "start",
          toNodeId: "supervisor",
          type: "default"
        }
      ]
    }
  };
}

export function createDefaultGraph(flowId: string, name: string, description = ""): AgentFlowGraph {
  return {
    id: flowId,
    tenantId: defaultTenantId,
    name,
    description,
    status: "draft",
    version: "v1",
    entryNodeId: "start",
    nodes: {
      start: {
        id: "start",
        type: "start",
        name: "Start",
        description: "Entry point for the flow.",
        config: {
          position: { x: 80, y: 180 }
        }
      }
    },
    edges: [],
    policy: {
      maxSteps: 24,
      maxParallelism: 4,
      timeoutMillis: 120000
    }
  };
}

export function createFlowNode(nodeType: AgentFlowNodeType, index: number): AgentFlowNode {
  const label = nodeLabel(nodeType);
  return {
    id: createClientID(nodeType.replace("_node", "")),
    type: nodeType,
    name: label,
    description: nodeDescription(nodeType),
    config: {
      position: {
        x: 120 + (index % 3) * defaultPositionStep,
        y: 140 + Math.floor(index / 3) * 150
      }
    },
    timeoutMillis: nodeType === "agent_node" ? 60000 : 15000
  };
}

export function flowToCanvasNodes(graph: AgentFlowGraph): AgentFlowCanvasNode[] {
  return Object.values(graph.nodes).map((node, index) => {
    const position = positionFromNode(node, index);
    return {
      id: node.id,
      type: "agentFlowNode",
      position,
      data: {
        label: node.name,
        description: node.description,
        nodeType: node.type,
        agentId: stringConfig(node.config, "agent_id"),
        agentMode: agentModeFromConfig(node.config),
        localAgent: localAgentFromConfig(node.config),
        config: node.config ?? {},
        timeoutMillis: node.timeoutMillis
      }
    };
  });
}

export function flowToCanvasEdges(graph: AgentFlowGraph): AgentFlowCanvasEdge[] {
  return graph.edges.map((edge, index) => ({
    id: edge.id || `${edge.fromNodeId}-${edge.toNodeId}-${index}`,
    source: edge.fromNodeId,
    target: edge.toNodeId,
    label: edgeLabel(edge),
    type: "smoothstep",
    markerEnd: {
      type: MarkerType.ArrowClosed
    },
    data: {
      edgeType: edge.type ?? "default"
    }
  }));
}

export function canvasToGraph(
  flow: AgentFlowSpec,
  canvasNodes: AgentFlowCanvasNode[],
  canvasEdges: AgentFlowCanvasEdge[]
): AgentFlowGraph {
  const nodes = canvasNodes.reduce<Record<string, AgentFlowNode>>((result, canvasNode) => {
    const existing = flow.graph.nodes[canvasNode.id];
    const config: Record<string, unknown> = {
      ...(existing?.config ?? {}),
      ...(canvasNode.data.config ?? {}),
      position: canvasNode.position
    };
    if (canvasNode.data.agentId && canvasNode.data.agentMode !== "local") {
      config.agent_id = canvasNode.data.agentId;
    } else {
      delete config.agent_id;
    }
    if (canvasNode.data.agentMode) {
      config.agent_mode = canvasNode.data.agentMode;
    }
    if (canvasNode.data.localAgent && canvasNode.data.agentMode === "local") {
      config.local_agent = canvasNode.data.localAgent;
    } else {
      delete config.local_agent;
    }
    result[canvasNode.id] = {
      id: canvasNode.id,
      type: canvasNode.data.nodeType,
      name: canvasNode.data.label,
      description: canvasNode.data.description,
      config,
      timeoutMillis: canvasNode.data.timeoutMillis ?? existing?.timeoutMillis,
      retryPolicy: existing?.retryPolicy
    };
    return result;
  }, {});

  return {
    ...flow.graph,
    id: flow.id,
    tenantId: flow.tenantId,
    name: flow.name,
    description: flow.description,
    status: flow.status,
    version: flow.version,
    entryNodeId: nodes.start ? "start" : Object.keys(nodes)[0] ?? "start",
    nodes,
    edges: canvasEdges.map((edge) => ({
      id: edge.id,
      fromNodeId: edge.source,
      toNodeId: edge.target,
      type: edge.data?.edgeType ?? "default"
    }))
  };
}

export function withGraph(flow: AgentFlowSpec, graph: AgentFlowGraph): AgentFlowSpec {
  return {
    ...flow,
    graph: {
      ...graph,
      id: flow.id,
      tenantId: flow.tenantId,
      name: flow.name,
      description: flow.description,
      status: flow.status,
      version: flow.version
    }
  };
}

export function isExistingAgentCanvasNode(node: AgentFlowCanvasNode): boolean {
  if (!isAgentLikeNodeType(node.data.nodeType)) return false;
  if (node.data.agentMode === "existing") return true;
  return !node.data.agentMode && Boolean(node.data.agentId);
}

export function nodeHasOutgoingEdges(nodeId: string, edges: AgentFlowCanvasEdge[]): boolean {
  return edges.some((edge) => edge.source === nodeId);
}

export function existingAgentLeafValidationMessage(nodes: AgentFlowCanvasNode[], edges: AgentFlowCanvasEdge[]): string | null {
  const invalidNode = nodes.find((node) => isExistingAgentCanvasNode(node) && nodeHasOutgoingEdges(node.id, edges));
  if (!invalidNode) return null;
  return `Existing Agent "${invalidNode.data.label}" must be a leaf node. Use Local Agent if this node needs Sub-Agents.`;
}

export function nodeLabel(nodeType: AgentFlowNodeType): string {
  switch (nodeType) {
    case "agent_node":
      return "Agent";
    case "supervisor_node":
      return "Supervisor";
    case "planner_node":
      return "Planner";
    case "router_node":
      return "Router";
    case "aggregator_node":
      return "Aggregator";
    case "verifier_node":
      return "Verifier";
    case "join_node":
      return "Join";
    case "connector_operation":
      return "Connector";
    case "tool":
      return "Tool";
    case "skill":
      return "Skill";
    case "agent":
      return "Agent";
    case "transform":
      return "Transform";
    case "condition":
      return "Condition";
    case "join":
      return "Join";
    case "end":
      return "End";
    case "start":
      return "Start";
  }
}

export function nodeDescription(nodeType: AgentFlowNodeType): string {
  switch (nodeType) {
    case "agent_node":
      return "Invoke a configured single agent and pass the current task.";
    case "supervisor_node":
      return "Coordinate Sub-Agents and synthesize their results.";
    case "planner_node":
      return "Prepare a structured execution plan.";
    case "router_node":
      return "Route the request to one or more downstream nodes.";
    case "aggregator_node":
      return "Merge upstream results into a final response.";
    case "verifier_node":
      return "Validate quality, safety, or completeness.";
    case "join_node":
      return "Wait for parallel branches before continuing.";
    case "connector_operation":
      return "Call a Connector Operation and publish selected output to workflow context.";
    case "tool":
      return "Call a governed platform Tool with mapped input.";
    case "skill":
      return "Invoke a reusable Skill as a deterministic workflow step.";
    case "agent":
      return "Invoke an Agent in a sequential workflow.";
    case "transform":
      return "Normalize input or output fields and write shared context.";
    case "condition":
      return "Route execution based on context values.";
    case "join":
      return "Wait for parallel branches before continuing.";
    case "end":
      return "Finish the workflow.";
    case "start":
      return "Entry point for the flow.";
  }
}

function isAgentLikeNodeType(nodeType: AgentFlowNodeType): boolean {
  return nodeType === "agent_node" || nodeType === "supervisor_node" || nodeType === "agent";
}

function edgeLabel(edge: AgentFlowEdge): string | undefined {
  if (edge.type === "conditional") return "condition";
  if (edge.type === "fallback") return "fallback";
  return undefined;
}

function positionFromNode(node: AgentFlowNode, index: number) {
  const position = node.config?.position;
  if (isPosition(position)) return position;
  return {
    x: 80 + index * defaultPositionStep,
    y: 180
  };
}

function isPosition(value: unknown): value is { x: number; y: number } {
  if (!value || typeof value !== "object") return false;
  const candidate = value as { x?: unknown; y?: unknown };
  return typeof candidate.x === "number" && typeof candidate.y === "number";
}

function stringConfig(config: Record<string, unknown> | undefined, key: string): string | undefined {
  const value = config?.[key];
  return typeof value === "string" ? value : undefined;
}

function agentModeFromConfig(config: Record<string, unknown> | undefined): "existing" | "local" | undefined {
  const value = config?.agent_mode;
  if (value === "existing" || value === "local") return value;
  return stringConfig(config, "agent_id") ? "existing" : undefined;
}

function localAgentFromConfig(config: Record<string, unknown> | undefined): LocalAgentConfig | undefined {
  const value = config?.local_agent;
  if (!value || typeof value !== "object") return undefined;
  const candidate = value as Partial<LocalAgentConfig>;
  return {
    name: typeof candidate.name === "string" ? candidate.name : "Local Agent",
    description: typeof candidate.description === "string" ? candidate.description : "",
    model: typeof candidate.model === "string" ? candidate.model : "deepseek-v4-flash",
    systemPrompt: typeof candidate.systemPrompt === "string" ? candidate.systemPrompt : "",
    toolIds: Array.isArray(candidate.toolIds) ? candidate.toolIds.filter((item): item is string => typeof item === "string") : [],
    skillIds: Array.isArray(candidate.skillIds) ? candidate.skillIds.filter((item): item is string => typeof item === "string") : []
  };
}

function createClientID(prefix: string): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return `${prefix}_${crypto.randomUUID().replaceAll("-", "").slice(0, 10)}`;
  }
  return `${prefix}_${Date.now()}`;
}
