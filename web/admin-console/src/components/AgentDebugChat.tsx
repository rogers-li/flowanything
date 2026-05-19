import { useEffect, useState } from "react";
import type { AgentDebugResponse, AgentTrace, RuntimeEvent, TraceStep } from "../types/platform";

export type DebugChatMessage = {
  id: string;
  role: "user" | "assistant";
  text: string;
  traceId?: string;
  trace?: AgentTrace | null;
  liveEvents?: RuntimeEvent[];
  pending?: boolean;
};

type TraceTreeNode = {
  children: TraceTreeNode[];
  index: number;
  step: TraceStep;
};

type RuntimeProgressItem = {
  detail?: string;
  id: string;
  title: string;
  tone: string;
};

export function ChatBubble({
  active,
  message,
  onOpenTrace,
  onOpenTraceId
}: {
  active: boolean;
  message: DebugChatMessage;
  onOpenTrace: (trace: AgentTrace) => void;
  onOpenTraceId?: (traceId: string) => void | Promise<void>;
}) {
  const canOpenTrace = message.role === "assistant" && Boolean(message.trace || (message.traceId && onOpenTraceId));
  const openTrace = () => {
    if (message.trace) {
      onOpenTrace(message.trace);
      return;
    }
    if (message.traceId && onOpenTraceId) {
      void onOpenTraceId(message.traceId);
    }
  };
  const progressItems = runtimeProgressItems(message.liveEvents ?? []);
  const visibleProgressItems = message.pending ? progressItems.slice(-8) : progressItems;

  return (
    <div className={message.role === "user" ? "agent-chat-bubble-row agent-chat-bubble-row-user" : "agent-chat-bubble-row"}>
      <div
        className={[
          message.role === "user" ? "agent-chat-bubble agent-chat-bubble-user" : "agent-chat-bubble agent-chat-bubble-agent",
          canOpenTrace ? "agent-chat-bubble-clickable" : "",
          active ? "agent-chat-bubble-active" : ""
        ]
          .filter(Boolean)
          .join(" ")}
        onClick={canOpenTrace ? openTrace : undefined}
        onKeyDown={(event) => {
          if (canOpenTrace && (event.key === "Enter" || event.key === " ")) {
            event.preventDefault();
            openTrace();
          }
        }}
        role={canOpenTrace ? "button" : undefined}
        tabIndex={canOpenTrace ? 0 : undefined}
      >
        <p>{message.text}</p>
        {visibleProgressItems.length > 0 ? (
          <ol className="agent-live-event-list">
            {visibleProgressItems.map((item) => (
              <li key={item.id} className={`agent-live-event agent-live-event-${item.tone}`}>
                <span>
                  <strong>{item.title}</strong>
                  {item.detail ? <small>{item.detail}</small> : null}
                </span>
              </li>
            ))}
          </ol>
        ) : null}
        {canOpenTrace ? (
          <button
            className="trace-icon-button"
            type="button"
            onClick={(event) => {
              event.stopPropagation();
              openTrace();
            }}
            aria-label="Open trace"
          >
            ⤢
          </button>
        ) : null}
      </div>
    </div>
  );
}

export function runtimeProgressHeadline(events: RuntimeEvent[]): string {
  const items = runtimeProgressItems(events);
  const latest = items[items.length - 1];
  if (latest) return latest.title;
  if (events.some((event) => event.type === "planning_started" || event.type === "llm_started")) {
    return "正在分析请求并规划下一步...";
  }
  if (events.some((event) => event.type === "run_completed")) return "处理完成。";
  if (events.some((event) => event.type === "run_failed")) return "处理失败。";
  return "Agent 正在处理...";
}

function runtimeProgressItems(events: RuntimeEvent[]): RuntimeProgressItem[] {
  const items: RuntimeProgressItem[] = [];
  const seen = new Set<string>();
  events.forEach((event, index) => {
    const item = runtimeProgressItem(event, index);
    if (!item) return;
    if (seen.has(item.id)) return;
    seen.add(item.id);
    items.push(item);
  });
  return items;
}

function runtimeProgressItem(event: RuntimeEvent, index: number): RuntimeProgressItem | null {
  if (event.type === "action_planned" || event.type === "action_started" || event.type === "action_completed" || event.type === "action_failed") {
    const action = objectFromUnknown(event.payload?.action) ?? objectFromUnknown(event.payload);
    const name = actionDisplayName(action);
    const typeLabel = actionTypeLabel(action);
    const detail = actionDetail(action);
    const state = {
      action_planned: "计划调用",
      action_started: "正在调用",
      action_completed: "已完成",
      action_failed: "调用失败"
    }[event.type];
    const tone = {
      action_planned: "planned",
      action_started: "running",
      action_completed: "completed",
      action_failed: "failed"
    }[event.type];
    return {
      id: `${event.type}_${event.id ?? action?.action_id ?? action?.target_id ?? action?.id ?? index}`,
      title: `${state}${typeLabel}：${name}`,
      detail,
      tone
    };
  }

  if (event.type !== "trace_step_added") return null;
  const step = objectFromUnknown(event.payload?.step);
  const stepType = stringFromUnknown(step?.type);
  const stepName = stringFromUnknown(step?.name);
  const stepStatus = stringFromUnknown(step?.status);
  if (stepType === "agent" && stepName && stepName !== "default") {
    return {
      id: `agent_${event.id ?? stepName}_${index}`,
      title: `进入 Agent：${stepName}`,
      tone: "agent"
    };
  }
  if (stepType === "model" && stepName) {
    const isPlanning = stepName.toLowerCase().includes("planning") || stepName.toLowerCase().includes("plan");
    return {
      id: `model_${event.id ?? stepName}_${stepStatus}_${index}`,
      title: `${stepStatus === "succeeded" ? "LLM 已完成" : stepStatus === "failed" ? "LLM 调用失败" : isPlanning ? "LLM 正在规划" : "LLM 正在生成"}：${stepName}`,
      tone: stepStatus === "failed" ? "failed" : "running"
    };
  }
  if (stepType === "tool" && stepName) {
    return {
      id: `tool_${event.id ?? stepName}_${stepStatus}_${index}`,
      title: `${stepStatus === "succeeded" ? "Tool 已完成" : stepStatus === "failed" ? "Tool 调用失败" : "正在调用 Tool"}：${stepName}`,
      tone: stepStatus === "failed" ? "failed" : stepStatus === "succeeded" ? "completed" : "running"
    };
  }
  if (stepType === "connector" && stepName) {
    return {
      id: `connector_${event.id ?? stepName}_${index}`,
      title: `${stepStatus === "failed" ? "外部接口调用失败" : "访问外部接口"}：${stepName}`,
      tone: stepStatus === "failed" ? "failed" : "connector"
    };
  }
  if (stepType === "workflow" && stepName) {
    return {
      id: `workflow_${event.id ?? stepName}_${index}`,
      title: `${stepStatus === "failed" ? "Workflow 执行失败" : "执行 Workflow"}：${stepName}`,
      tone: stepStatus === "failed" ? "failed" : "workflow"
    };
  }
  return null;
}

function actionDisplayName(action: Record<string, unknown> | null): string {
  if (!action) return "未命名能力";
  return (
    stringFromUnknown(action.target_name) ||
    stringFromUnknown(action.name) ||
    stringFromUnknown(action.agent_name) ||
    stringFromUnknown(action.tool_name) ||
    stringFromUnknown(action.skill_name) ||
    stringFromUnknown(action.node_name) ||
    stringFromUnknown(action.target_id) ||
    stringFromUnknown(action.node_id) ||
    stringFromUnknown(action.agent_id) ||
    stringFromUnknown(action.tool_id) ||
    stringFromUnknown(action.skill_id) ||
    stringFromUnknown(action.id) ||
    "未命名能力"
  );
}

function actionTypeLabel(action: Record<string, unknown> | null): string {
  const type = stringFromUnknown(action?.type);
  if (type === "agent") return " Agent";
  if (type === "skill") return " Skill";
  if (type === "tool") return " Tool";
  if (type === "workflow") return " Workflow";
  return "能力";
}

function actionDetail(action: Record<string, unknown> | null): string | undefined {
  const task = stringFromUnknown(action?.task);
  const reason = stringFromUnknown(action?.reason);
  if (task && reason) return `${reason}；任务：${task}`;
  return reason || (task ? `任务：${task}` : undefined);
}

function objectFromUnknown(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function stringFromUnknown(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

export function TraceInspector({ trace, onClose }: { trace: AgentTrace; onClose: () => void }) {
  const tree = buildTraceTree(trace.steps);
  const [collapsedStepIds, setCollapsedStepIds] = useState<Set<string>>(() => new Set());
  const [detailCollapseVersion, setDetailCollapseVersion] = useState(0);
  const [compactTree, setCompactTree] = useState(false);

  useEffect(() => {
    setCollapsedStepIds(new Set());
    setDetailCollapseVersion(0);
    setCompactTree(false);
  }, [trace.traceId]);

  const toggleSubtree = (stepId: string) => {
    setCollapsedStepIds((current) => {
      const next = new Set(current);
      if (next.has(stepId)) {
        next.delete(stepId);
      } else {
        next.add(stepId);
      }
      return next;
    });
  };

  const collapseAgentSubtrees = () => {
    setCollapsedStepIds(new Set(collectCollapsibleStepIds(tree, (node) => node.step.type === "agent")));
    setDetailCollapseVersion((current) => current + 1);
    setCompactTree(true);
  };

  const expandAllSubtrees = () => {
    setCollapsedStepIds(new Set());
    setCompactTree(false);
  };

  return (
    <section className={compactTree ? "trace-inspector trace-inspector-compact" : "trace-inspector"}>
      <header className="trace-inspector-header">
        <div>
          <strong>Trace details</strong>
          <span>{`${trace.status}${trace.durationMillis !== undefined ? ` · ${trace.durationMillis}ms` : ""}`}</span>
        </div>
        <div className="trace-inspector-actions">
          <button className="trace-inspector-link" type="button" onClick={collapseAgentSubtrees}>
            Collapse agents
          </button>
          <button className="trace-inspector-link" type="button" onClick={expandAllSubtrees}>
            Expand all
          </button>
          <button className="icon-action" type="button" onClick={onClose} aria-label="Close trace">
            ×
          </button>
        </div>
      </header>
      <code className="trace-inspector-id">{trace.traceId}</code>
      <div className="trace-step-detail-list">
        {tree.map((node) => (
          <TraceTreeBranch
            key={node.step.id}
            collapsedStepIds={collapsedStepIds}
            depth={0}
            detailCollapseVersion={detailCollapseVersion}
            node={node}
            onToggleSubtree={toggleSubtree}
          />
        ))}
      </div>
    </section>
  );
}

export function assistantTextFromDebugResult(result: AgentDebugResponse): string {
  const reply = result.actions.find((action) => action.text && (action.type === "speak" || action.type === "display_text" || action.type === "ask_question"));
  if (reply?.text) return reply.text;
  const confirmation = result.actions.find((action) => action.type === "ask_confirmation");
  if (confirmation?.text) return confirmation.text;
  return "Agent completed the turn.";
}

function TraceTreeBranch({
  collapsedStepIds,
  depth,
  detailCollapseVersion,
  node,
  onToggleSubtree
}: {
  collapsedStepIds: Set<string>;
  depth: number;
  detailCollapseVersion: number;
  node: TraceTreeNode;
  onToggleSubtree: (stepId: string) => void;
}) {
  const hasChildren = node.children.length > 0;
  const collapsed = collapsedStepIds.has(node.step.id);

  return (
    <div
      className={[
        depth === 0 ? "trace-tree-branch trace-tree-branch-root" : "trace-tree-branch",
        collapsed ? "trace-tree-branch-collapsed" : ""
      ]
        .filter(Boolean)
        .join(" ")}
    >
      <TraceStepDetail
        childCount={node.children.length}
        depth={depth}
        detailCollapseVersion={detailCollapseVersion}
        index={node.index}
        isSubtreeCollapsed={collapsed}
        onToggleSubtree={hasChildren ? () => onToggleSubtree(node.step.id) : undefined}
        step={node.step}
      />
      {hasChildren && !collapsed ? (
        <div className="trace-tree-children">
          {node.children.map((child) => (
            <TraceTreeBranch
              key={child.step.id}
              collapsedStepIds={collapsedStepIds}
              depth={depth + 1}
              detailCollapseVersion={detailCollapseVersion}
              node={child}
              onToggleSubtree={onToggleSubtree}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}

function TraceStepDetail({
  childCount,
  depth,
  detailCollapseVersion,
  index,
  isSubtreeCollapsed,
  onToggleSubtree,
  step
}: {
  childCount: number;
  depth: number;
  detailCollapseVersion: number;
  index: number;
  isSubtreeCollapsed: boolean;
  onToggleSubtree?: () => void;
  step: TraceStep;
}) {
  const [open, setOpen] = useState(step.status === "failed" || index === 0 || (childCount > 0 && depth <= 1));
  const metadata = step.metadata ?? {};
  const request = metadata.request;
  const response = metadata.response;
  const summary = traceMetadataSummary(metadata);
  const metadataRest = metadataWithoutPayload(metadata);
  const hasMetadata = Object.keys(metadataRest).length > 0;

  useEffect(() => {
    if (detailCollapseVersion > 0) {
      setOpen(false);
    }
  }, [detailCollapseVersion]);

  return (
    <article className={`trace-step-detail trace-step-${step.type} ${open ? "trace-step-detail-open" : ""}`}>
      <div className="trace-step-detail-summary-row">
        <button className="trace-step-detail-summary" type="button" aria-expanded={open} onClick={() => setOpen((current) => !current)}>
          <span>{traceStepBadge(step, index)}</span>
          <div>
            <strong>{traceStepTitle(step)}</strong>
            <small>{summary || traceStepSummary(step)}</small>
          </div>
          <small>{traceStepMeta(step, childCount)}</small>
        </button>
        {onToggleSubtree ? (
          <button
            className="trace-subtree-toggle"
            type="button"
            aria-label={isSubtreeCollapsed ? "Expand trace subtree" : "Collapse trace subtree"}
            aria-expanded={!isSubtreeCollapsed}
            onClick={onToggleSubtree}
            title={isSubtreeCollapsed ? `Expand ${childCount} child trace steps` : `Collapse ${childCount} child trace steps`}
          >
            {isSubtreeCollapsed ? "+" : "-"}
          </button>
        ) : null}
      </div>
      {open ? (
        <div className="trace-step-detail-body">
          {summary ? <p>{summary}</p> : null}
          {request !== undefined ? <JsonBlock title="Request" value={request} /> : null}
          {response !== undefined ? <JsonBlock title="Response" value={response} /> : null}
          {hasMetadata ? <JsonBlock title="Metadata" value={metadataRest} /> : null}
        </div>
      ) : null}
    </article>
  );
}

function buildTraceTree(steps: TraceStep[]): TraceTreeNode[] {
  const nodeById = new Map<string, TraceTreeNode>();
  const roots: TraceTreeNode[] = [];

  steps.forEach((step, index) => {
    nodeById.set(step.id, {
      children: [],
      index,
      step
    });
  });

  steps.forEach((step) => {
    const node = nodeById.get(step.id);
    if (!node) return;
    const parent = step.parentId ? nodeById.get(step.parentId) : undefined;
    if (parent && parent.step.id !== step.id) {
      parent.children.push(node);
      return;
    }
    roots.push(node);
  });

  sortTraceTree(roots);
  return roots;
}

function sortTraceTree(nodes: TraceTreeNode[]) {
  nodes.sort((left, right) => left.index - right.index);
  nodes.forEach((node) => sortTraceTree(node.children));
}

function collectCollapsibleStepIds(nodes: TraceTreeNode[], predicate: (node: TraceTreeNode) => boolean): string[] {
  const ids: string[] = [];
  const visit = (node: TraceTreeNode) => {
    if (node.children.length > 0 && predicate(node)) {
      ids.push(node.step.id);
    }
    node.children.forEach(visit);
  };
  nodes.forEach(visit);
  return ids;
}

function JsonBlock({ title, value }: { title: string; value: unknown }) {
  const [open, setOpen] = useState(false);

  return (
    <div className={`trace-json-block ${open ? "trace-json-block-open" : ""}`}>
      <button className="trace-json-trigger" type="button" aria-expanded={open} onClick={() => setOpen((current) => !current)}>
        <span>{open ? "v" : ">"}</span>
        {title}
      </button>
      {open ? (
        <div className="trace-json-scroll">
          <pre>{JSON.stringify(value, null, 2)}</pre>
        </div>
      ) : null}
    </div>
  );
}

function metadataWithoutPayload(metadata: Record<string, unknown>): Record<string, unknown> {
  const { request: _request, response: _response, ...rest } = metadata;
  return rest;
}

function traceStepTitle(step: TraceStep): string {
  const kind = stringFromUnknown(step.metadata?.kind);
  if (kind === "planning") return `ReAct · ${step.name || "Planning"} · ${step.status}`;
  if (kind === "llm") return `LLM · ${step.name || "Model call"} · ${step.status}`;
  if (step.type === "tool") return `Tool · ${step.name || "unnamed"} · ${step.status}`;
  if (step.type === "connector") return `Connector API · ${step.name || "unnamed"} · ${step.status}`;
  if (step.type === "node") return `Node · ${stringFromUnknown(step.metadata?.node_name) || step.name || "unnamed"} · ${step.status}`;
  if (step.type === "agent") return `Agent · ${step.name || "unnamed"} · ${step.status}`;
  return `${step.type} · ${step.name || "unnamed"} · ${step.status}`;
}

function traceStepBadge(step: TraceStep, index: number): string {
  const label = {
    connector: "C",
    event: "E",
    agent: "A",
    model: "M",
    node: "N",
    skill: "S",
    tool: "T",
    workflow: "W"
  }[step.type];
  return label ?? String(index + 1);
}

function traceStepMeta(step: TraceStep, childCount: number): string {
  const duration = traceStepSummary(step);
  if (childCount <= 0) return duration;
  return `${duration} · ${childCount} child${childCount > 1 ? "ren" : ""}`;
}

function traceStepSummary(step: TraceStep): string {
  const duration = step.durationMillis !== undefined ? `${step.durationMillis}ms` : "instant";
  if (step.error) return `${duration} · ${step.error}`;
  return duration;
}

function traceMetadataSummary(metadata: Record<string, unknown>): string {
  const keys = [
    "scope",
    "node_name",
    "node_type",
    "node_id",
    "agent_id",
    "agent_name",
    "phase",
    "provider",
    "response_model",
    "requested_model",
    "skill_id",
    "tool_id",
    "implementation",
    "workflow_id",
    "run_id",
    "connector_operation_id",
    "inner_trace_id",
    "agent_flow_run_id",
    "finish_reason",
    "total_tokens",
    "success"
  ];
  return keys
    .filter((key) => metadata[key] !== undefined && metadata[key] !== "")
    .map((key) => `${key}=${formatTraceValue(metadata[key])}`)
    .join(" · ");
}

function formatTraceValue(value: unknown): string {
  if (Array.isArray(value)) return value.length === 0 ? "[]" : JSON.stringify(value);
  if (typeof value === "object" && value !== null) return JSON.stringify(value);
  return String(value);
}
