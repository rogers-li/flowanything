import {
  BaseEdge,
  EdgeLabelRenderer,
  getSmoothStepPath,
  type Edge,
  type EdgeProps
} from "@xyflow/react";

export type FlowDeletableEdgeData = {
  edgeType?: string;
  onDelete?: (edgeId: string) => void;
};

type FlowDeletableEdgeModel = Edge<FlowDeletableEdgeData>;

export function FlowDeletableEdge({
  data,
  id,
  label,
  markerEnd,
  selected,
  sourcePosition,
  sourceX,
  sourceY,
  style,
  targetPosition,
  targetX,
  targetY
}: EdgeProps<FlowDeletableEdgeModel>) {
  const [edgePath, labelX, labelY] = getSmoothStepPath({
    sourcePosition,
    sourceX,
    sourceY,
    targetPosition,
    targetX,
    targetY
  });

  return (
    <>
      <BaseEdge id={id} markerEnd={markerEnd} path={edgePath} style={style} />
      <EdgeLabelRenderer>
        <div
          className={selected ? "flow-edge-action flow-edge-action-selected" : "flow-edge-action"}
          style={{ transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)` }}
        >
          {label ? <span className="flow-edge-label-text">{label}</span> : null}
          <button
            className="flow-edge-delete"
            type="button"
            aria-label="Delete connection"
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              data?.onDelete?.(id);
            }}
            onPointerDown={(event) => event.stopPropagation()}
          >
            ×
          </button>
        </div>
      </EdgeLabelRenderer>
    </>
  );
}
