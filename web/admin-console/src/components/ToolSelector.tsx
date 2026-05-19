import { useMemo, useState } from "react";
import type { ToolSpec } from "../types/platform";
import { Badge } from "./Badge";

const statusTone = {
  draft: "gray",
  enabled: "green",
  disabled: "red"
} as const;

type ToolSelectorSummaryItem = {
  label: string;
  value: number | string;
};

type ToolSelectorProps = {
  emptyMessage?: string;
  inheritedToolIds?: string[];
  noMatchesMessage?: string;
  onToggle: (toolId: string, checked: boolean) => void;
  searchLabel?: string;
  searchPlaceholder?: string;
  selectedToolIds: string[];
  summaryItems?: ToolSelectorSummaryItem[];
  tools: ToolSpec[];
};

export function ToolSelector({
  emptyMessage = "No Tools available.",
  inheritedToolIds = [],
  noMatchesMessage = "No Tools match this search.",
  onToggle,
  searchLabel = "Search tools",
  searchPlaceholder = "Search by name, source, description...",
  selectedToolIds,
  summaryItems,
  tools
}: ToolSelectorProps) {
  const [query, setQuery] = useState("");
  const inheritedToolSet = useMemo(() => new Set(inheritedToolIds), [inheritedToolIds]);
  const selectedToolSet = useMemo(() => new Set(selectedToolIds), [selectedToolIds]);
  const visibleTools = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    const matchesQuery = (tool: ToolSpec) =>
      !normalizedQuery ||
      [
        tool.name,
        tool.id,
        tool.description,
        tool.llmDescription,
        tool.implementation,
        tool.businessDomain,
        tool.ownerTeam,
        tool.binding?.connectorOperationId,
        tool.binding?.mcpServerId,
        tool.binding?.mcpToolName
      ]
        .filter(Boolean)
        .some((value) => value?.toLowerCase().includes(normalizedQuery));

    return [...tools]
      .filter(matchesQuery)
      .sort((left, right) => {
        const leftRank = toolSelectionRank(left, selectedToolSet, inheritedToolSet);
        const rightRank = toolSelectionRank(right, selectedToolSet, inheritedToolSet);
        if (leftRank !== rightRank) return leftRank - rightRank;
        return left.name.localeCompare(right.name);
      });
  }, [inheritedToolSet, query, selectedToolSet, tools]);

  return (
    <div className="tool-selector">
      <label className="agent-config-search">
        {searchLabel}
        <input value={query} placeholder={searchPlaceholder} onChange={(event) => setQuery(event.target.value)} />
      </label>

      <div className="agent-editor-skill-list agent-editor-tool-list tool-selector-list">
        {visibleTools.map((tool) => {
          const checked = selectedToolSet.has(tool.id);
          const inherited = inheritedToolSet.has(tool.id);
          return <ToolSelectorRow checked={checked} inherited={inherited} key={tool.id} tool={tool} onChange={(value) => onToggle(tool.id, value)} />;
        })}
        {tools.length === 0 ? <p className="tool-selector-empty">{emptyMessage}</p> : null}
        {tools.length > 0 && visibleTools.length === 0 ? <p className="tool-selector-empty">{noMatchesMessage}</p> : null}
      </div>

      {summaryItems && summaryItems.length > 0 ? (
        <div className="agent-config-summary-line">
          {summaryItems.map((item) => (
            <span key={item.label}>{`${item.value} ${item.label}`}</span>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function toolSelectionRank(tool: ToolSpec, selectedToolSet: Set<string>, inheritedToolSet: Set<string>): number {
  if (selectedToolSet.has(tool.id)) return 0;
  if (inheritedToolSet.has(tool.id)) return 1;
  return 2;
}

function ToolSelectorRow({
  checked,
  inherited,
  onChange,
  tool
}: {
  checked: boolean;
  inherited: boolean;
  onChange: (checked: boolean) => void;
  tool: ToolSpec;
}) {
  const selected = checked || inherited;
  const inheritedOnly = inherited && !checked;
  return (
    <label className={selected ? "agent-tool-row agent-tool-row-active" : "agent-tool-row"}>
      <input
        checked={selected}
        disabled={inheritedOnly}
        title={inheritedOnly ? "This tool is selected through a bound Skill. Remove the Skill to unselect it." : undefined}
        type="checkbox"
        onChange={(event) => onChange(event.target.checked)}
      />
      <span>
        <strong>{tool.name}</strong>
        <small>{tool.description}</small>
      </span>
      <Badge tone={checked ? "blue" : inherited ? "amber" : statusTone[tool.status]}>{checked ? "direct" : inherited ? "skill" : tool.status}</Badge>
    </label>
  );
}
