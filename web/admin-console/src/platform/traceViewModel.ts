import type { AgentTrace, TraceStep } from "../types/platform";
import type { TraceResponse, TraceSpan } from "./configTypes";

export function agentTraceFromTraceResponse(response: TraceResponse): AgentTrace {
  const spans = (response.trace.spans ?? []).filter((span) => traceStepType(span.kind) !== "event");
  const startedAt = minDate(spans.map((span) => span.started_at)) ?? new Date().toISOString();
  const finishedAt = maxDate(spans.map((span) => span.finished_at));
  const failed = spans.some((span) => span.status === "error" || Boolean(span.error));
  return {
    traceId: response.trace.trace_id,
    tenantId: "tenant_1",
    status: failed ? "failed" : finishedAt ? "succeeded" : "running",
    startedAt,
    finishedAt,
    durationMillis: finishedAt ? Math.max(0, new Date(finishedAt).getTime() - new Date(startedAt).getTime()) : undefined,
    error: spans.find((span) => span.error)?.error,
    steps: spans.map(traceStepFromSpan)
  };
}

function traceStepFromSpan(span: TraceSpan): TraceStep {
  const startedAt = span.started_at || new Date().toISOString();
  const finishedAt = span.finished_at;
  return {
    id: span.span_id,
    parentId: span.parent_span_id || undefined,
    type: traceStepType(span.kind),
    name: span.name,
    status: traceStepStatus(span.status),
    startedAt,
    finishedAt,
    durationMillis: finishedAt ? Math.max(0, new Date(finishedAt).getTime() - new Date(startedAt).getTime()) : undefined,
    metadata: {
      kind: normalizeKind(span.kind),
      ...span.attributes,
      request: span.input,
      response: span.output
    },
    error: span.error
  };
}

function traceStepType(kind: string): TraceStep["type"] {
  const normalized = normalizeKind(kind);
  if (normalized === "agent") return "agent";
  if (normalized === "skill") return "skill";
  if (normalized === "tool") return "tool";
  if (normalized === "connector") return "connector";
  if (normalized === "flow" || normalized === "workflow") return "workflow";
  if (normalized === "llm" || normalized === "planning") return "model";
  if (normalized === "node") return "node";
  return "event";
}

function normalizeKind(kind: string): string {
  return (kind || "").toLowerCase();
}

function traceStepStatus(status: string): TraceStep["status"] {
  if (status === "ok") return "succeeded";
  if (status === "error") return "failed";
  if (status === "waiting") return "started";
  return "started";
}

function minDate(values: Array<string | undefined>): string | undefined {
  return dateEdge(values, (left, right) => left < right);
}

function maxDate(values: Array<string | undefined>): string | undefined {
  return dateEdge(values, (left, right) => left > right);
}

function dateEdge(values: Array<string | undefined>, compare: (left: number, right: number) => boolean): string | undefined {
  let selected: string | undefined;
  let selectedTime = 0;
  values.forEach((value) => {
    if (!value) return;
    const time = new Date(value).getTime();
    if (Number.isNaN(time)) return;
    if (!selected || compare(time, selectedTime)) {
      selected = value;
      selectedTime = time;
    }
  });
  return selected;
}
