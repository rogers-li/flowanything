import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import type { AgentProfile, SkillSpec, ToolSpec } from "../types/platform";

type PromptMentionKind = "agent" | "skill" | "tool";

type PromptMention = {
  id: string;
  kind: PromptMentionKind;
  name: string;
  description: string;
  meta: string;
  bound: boolean;
};

type MentionRange = {
  start: number;
  end: number;
  query: string;
};

export type PromptAgentMention = {
  id: string;
  name: string;
  description?: string;
  meta?: string;
  bound?: boolean;
};

type PromptRichEditorProps = {
  agentMentions?: PromptAgentMention[];
  agents?: AgentProfile[];
  ariaLabel?: string;
  onBindAgent?: (agentId: string) => void;
  onBindSkill: (skillId: string) => void;
  onBindTool: (toolId: string) => void;
  onChange: (value: string) => void;
  placeholder?: string;
  selectedAgentIds?: string[];
  selectedSkillIds: string[];
  selectedToolIds: string[];
  skills: SkillSpec[];
  tools: ToolSpec[];
  toolbarLabel?: string;
  value: string;
};

export function PromptRichEditor({
  agentMentions,
  agents = [],
  ariaLabel = "Agent system prompt",
  onBindAgent,
  onBindSkill,
  onBindTool,
  onChange,
  placeholder = "## Role\nDescribe what this Agent does.\n\n## Rules\nUse @ to insert Tools, Skills, or Agents when needed.",
  selectedAgentIds = [],
  selectedSkillIds,
  selectedToolIds,
  skills,
  tools,
  toolbarLabel,
  value
}: PromptRichEditorProps) {
  const editorRef = useRef<HTMLDivElement | null>(null);
  const latestValueRef = useRef(value);
  const activeMentionDomRangeRef = useRef<Range | null>(null);
  const mentionRangeRef = useRef<MentionRange | null>(null);
  const pendingMentionStartRef = useRef<number | null>(null);
  const [mentionQuery, setMentionQuery] = useState<string | null>(null);
  const [activeMentionIndex, setActiveMentionIndex] = useState(0);
  const [mentionMenuPosition, setMentionMenuPosition] = useState({ left: 0, top: 0, width: 360 });

  const mentionOptions = useMemo<PromptMention[]>(() => {
    const toolMentions = tools.map((tool) => ({
        id: tool.id,
        kind: "tool" as const,
        name: tool.name,
        description: tool.llmDescription || tool.description,
        meta: `${tool.implementation} · ${tool.riskLevel}`,
        bound: selectedToolIds.includes(tool.id)
      }));
    const skillMentions = skills.map((skill) => ({
        id: skill.id,
        kind: "skill" as const,
        name: skill.name,
        description: skill.description,
        meta: `${skill.businessDomain ?? "General"} · ${skill.status}`,
        bound: selectedSkillIds.includes(skill.id)
      }));
    const agentMentionSource =
      agentMentions ??
      agents.map((agent) => ({
        id: agent.id,
        name: agent.name,
        description: agent.description,
        meta: `${agent.businessDomain ?? "General"} · ${agent.status}`,
        bound: selectedAgentIds.includes(agent.id)
      }));
    const agentMentionItems = agentMentionSource.map((agent) => ({
      id: agent.id,
      kind: "agent" as const,
      name: agent.name,
      description: agent.description ?? "",
      meta: agent.meta ?? "Agent",
      bound: agent.bound ?? selectedAgentIds.includes(agent.id)
    }));

    return [...agentMentionItems, ...skillMentions, ...toolMentions];
  }, [agentMentions, agents, selectedAgentIds, selectedSkillIds, selectedToolIds, skills, tools]);

  const filteredMentions = useMemo(() => {
    if (mentionQuery === null) return [];
    const query = mentionQuery.trim().toLowerCase();
    const matches = mentionOptions
      .filter((mention) => {
        if (!query) return true;
        return (
          mention.name.toLowerCase().includes(query) ||
          mention.id.toLowerCase().includes(query) ||
          mention.description.toLowerCase().includes(query) ||
          mention.kind.toLowerCase().includes(query)
        );
      });
    if (query) return matches.slice(0, 14);
    return balancedMentionOptions(matches);
  }, [mentionOptions, mentionQuery]);

  useEffect(() => {
    setActiveMentionIndex(0);
  }, [mentionQuery]);

  useEffect(() => {
    setActiveMentionIndex((current) => (filteredMentions.length === 0 ? 0 : Math.min(current, filteredMentions.length - 1)));
  }, [filteredMentions.length]);

  useEffect(() => {
    const editor = editorRef.current;
    if (!editor || document.activeElement === editor) return;
    editor.innerHTML = renderPromptHTML(value, mentionOptions);
    latestValueRef.current = value;
  }, [mentionOptions, value]);

  const updateMentionMenuPosition = () => {
    const editor = editorRef.current;
    if (!editor) return;
    const editorRect = editor.getBoundingClientRect();
    const anchorRect = caretAnchorRect(editor) ?? editorRect;
    const menuWidth = Math.max(280, Math.min(editorRect.width, 420));
    const belowTop = anchorRect.bottom + 8;
    const aboveTop = anchorRect.top - 328;
    const top = belowTop > window.innerHeight - 320 ? Math.max(12, aboveTop) : Math.max(12, belowTop);
    setMentionMenuPosition({
      left: Math.max(12, Math.min(anchorRect.left, window.innerWidth - menuWidth - 12)),
      top,
      width: menuWidth
    });
  };

  const openMentionMenu = (query = "") => {
    setMentionQuery(query);
    requestAnimationFrame(updateMentionMenuPosition);
  };

  const closeMentionMenu = () => {
    activeMentionDomRangeRef.current = null;
    mentionRangeRef.current = null;
    pendingMentionStartRef.current = null;
    setMentionQuery(null);
  };

  const commitEditorValue = () => {
    const editor = editorRef.current;
    if (!editor) return "";
    const nextValue = readPromptText(editor);
    latestValueRef.current = nextValue;
    onChange(nextValue);
    return nextValue;
  };

  const startMentionInput = () => {
    const editor = editorRef.current;
    const selection = window.getSelection();
    if (!editor || !selection || selection.rangeCount === 0) return false;
    const range = selection.getRangeAt(0);
    if (!rangeBelongsTo(editor, range)) return false;

    range.deleteContents();
    const marker = document.createTextNode("@");
    range.insertNode(marker);

    const triggerRange = document.createRange();
    triggerRange.setStart(marker, 0);
    triggerRange.setEnd(marker, 1);
    activeMentionDomRangeRef.current = triggerRange;

    const caretRange = document.createRange();
    caretRange.setStart(marker, 1);
    caretRange.collapse(true);
    selection.removeAllRanges();
    selection.addRange(caretRange);

    const nextValue = commitEditorValue();
    const caretOffset = plainTextCaretOffset(editor);
    pendingMentionStartRef.current = Math.max(0, caretOffset - 1);
    mentionRangeRef.current = { start: pendingMentionStartRef.current, end: caretOffset, query: "" };
    openMentionMenu("");
    return Boolean(nextValue || editor.innerText);
  };

  const refreshMentionQuery = (keepOpenIfUnresolved = false) => {
    const editor = editorRef.current;
    if (!editor) return;
    const value = readPromptText(editor);
    const caretOffset = plainTextCaretOffset(editor);
    const activeRange = activeMentionDomRangeRef.current;
    const activeTrigger = mentionTriggerFromDomRange(editor, activeRange);
    if (activeTrigger) {
      mentionRangeRef.current = activeTrigger;
      openMentionMenu(activeTrigger.query);
      return;
    }
    const pendingTrigger = mentionTriggerFromPendingStart(value, caretOffset, pendingMentionStartRef.current);
    if (pendingTrigger) {
      mentionRangeRef.current = {
        start: pendingTrigger.start,
        end: pendingTrigger.end,
        query: pendingTrigger.query
      };
      openMentionMenu(pendingTrigger.query);
      return;
    }
    const trigger = mentionTriggerFromEditor(editor, value);
    if (trigger) {
      mentionRangeRef.current = {
        start: trigger.start,
        end: trigger.end,
        query: trigger.query
      };
      openMentionMenu(trigger.query);
      return;
    }
    if (keepOpenIfUnresolved) {
      openMentionMenu("");
      return;
    }
    closeMentionMenu();
  };

  const syncValueFromEditor = () => {
    const editor = editorRef.current;
    if (!editor) return;
    const nextValue = readPromptText(editor);
    const caretOffset = plainTextCaretOffset(editor);
    latestValueRef.current = nextValue;
    onChange(nextValue);
    const activeTrigger = mentionTriggerFromDomRange(editor, activeMentionDomRangeRef.current);
    const pendingTrigger = mentionTriggerFromPendingStart(nextValue, caretOffset, pendingMentionStartRef.current);
    const trigger = activeTrigger ?? pendingTrigger ?? mentionTriggerFromEditor(editor, nextValue);
    if (trigger) {
      mentionRangeRef.current = {
        start: trigger.start,
        end: trigger.end,
        query: trigger.query
      };
      openMentionMenu(trigger.query);
    } else if (mentionQuery !== null) {
      closeMentionMenu();
    }
  };

  const insertMention = (mention: PromptMention) => {
    const editor = editorRef.current;
    const token = formatPromptMention(mention);
    const domInsertResult = replaceActiveMentionDomRange(editor, activeMentionDomRangeRef.current, token);
    if (editor && domInsertResult) {
      const nextValue = readPromptText(editor);
      const nextCaretOffset = domInsertResult.caretOffset;
      latestValueRef.current = nextValue;
      closeMentionMenu();
      onChange(nextValue);
      bindMention(mention);
      editor.innerHTML = renderPromptHTML(nextValue, mentionOptions);
      requestAnimationFrame(() => {
        editor.focus();
        placeCaretAtPlainTextOffset(editor, nextCaretOffset);
      });
      return;
    }

    const currentValue = editor ? readPromptText(editor) : latestValueRef.current;
    const fallbackCaretOffset = editor ? plainTextCaretOffset(editor) : currentValue.length;
    const fallbackTrigger = findMentionTrigger(currentValue, fallbackCaretOffset);
    const mentionRange = resolveMentionInsertionRange(
      currentValue,
      mentionRangeRef.current,
      pendingMentionStartRef.current,
      fallbackCaretOffset
    );
    const start = mentionRange?.start ?? fallbackTrigger?.start ?? fallbackCaretOffset;
    const end = mentionRange?.end ?? fallbackCaretOffset;
    const trailingValue = currentValue.slice(end);
    const separator = trailingValue.startsWith(" ") || trailingValue.startsWith("\n") ? "" : " ";
    const nextValue = `${currentValue.slice(0, start)}${token}${separator}${trailingValue}`;
    const nextCaretOffset = start + token.length + separator.length;

    latestValueRef.current = nextValue;
    closeMentionMenu();
    onChange(nextValue);
    bindMention(mention);

    if (editor) {
      editor.innerHTML = renderPromptHTML(nextValue, mentionOptions);
      requestAnimationFrame(() => {
        editor.focus();
        placeCaretAtPlainTextOffset(editor, nextCaretOffset);
      });
    }
  };

  const bindMention = (mention: PromptMention) => {
    if (mention.kind === "skill") {
      onBindSkill(mention.id);
    } else if (mention.kind === "tool") {
      onBindTool(mention.id);
    } else {
      onBindAgent?.(mention.id);
    }
  };

  const hasAgentMentions = (agentMentions?.length ?? agents.length) > 0;
  const mentionLabel = toolbarLabel ?? (hasAgentMentions ? "Type @ to insert Skills, Tools, or Agents" : "Type @ to insert Skills or Tools");
  const mentionMenu =
    mentionQuery !== null ? (
      <div
        className="prompt-mention-menu prompt-mention-menu-floating"
        style={{
          left: mentionMenuPosition.left,
          top: mentionMenuPosition.top,
          width: mentionMenuPosition.width
        }}
      >
        {filteredMentions.map((mention, index) => (
          <button
            className={index === activeMentionIndex ? "prompt-mention-option-active" : ""}
            key={`${mention.kind}:${mention.id}`}
            type="button"
            onMouseDown={(event) => {
              event.preventDefault();
              insertMention(mention);
            }}
            onMouseEnter={() => setActiveMentionIndex(index)}
          >
            <span className={`prompt-token prompt-token-${mention.kind}`}>{formatPromptMention(mention)}</span>
            <strong>{mention.name}</strong>
            <small>
              {mention.bound ? "Bound" : "Select to bind"} · {mention.meta}
            </small>
          </button>
        ))}
        {filteredMentions.length === 0 ? <p>No matching Skills, Tools, or Agents.</p> : null}
      </div>
    ) : null;

  return (
    <div className="prompt-rich-editor-shell">
      <div className="prompt-rich-toolbar">
        <span>{mentionLabel}</span>
      </div>
      <div className="prompt-rich-editor-wrap">
        <div
          aria-label={ariaLabel}
          className="agent-prompt-editor"
          contentEditable
          data-placeholder={placeholder}
          onBeforeInput={(event) => {
            const inputEvent = event.nativeEvent as InputEvent;
            if (inputEvent.data === "@") {
              event.preventDefault();
              if (!startMentionInput()) {
                openMentionMenu("");
              }
            }
          }}
          onBlur={() => {
            closeMentionMenu();
            const editor = editorRef.current;
            if (editor) {
              editor.innerHTML = renderPromptHTML(latestValueRef.current, mentionOptions);
            }
          }}
          onInput={syncValueFromEditor}
          onKeyDown={(event) => {
            if (event.key === "@") {
              openMentionMenu("");
            }
            if (mentionQuery !== null) {
              if (event.key === "Escape") {
                event.preventDefault();
                closeMentionMenu();
              }
              if (event.key === "ArrowDown") {
                event.preventDefault();
                setActiveMentionIndex((current) => (filteredMentions.length === 0 ? 0 : (current + 1) % filteredMentions.length));
              }
              if (event.key === "ArrowUp") {
                event.preventDefault();
                setActiveMentionIndex((current) => (filteredMentions.length === 0 ? 0 : (current - 1 + filteredMentions.length) % filteredMentions.length));
              }
              if ((event.key === "Enter" || event.key === "Tab") && filteredMentions[activeMentionIndex]) {
                event.preventDefault();
                insertMention(filteredMentions[activeMentionIndex]);
              }
            }
          }}
          onKeyUp={(event) => {
            if (isModifierKey(event.key) || (mentionQuery !== null && isMenuControlKey(event.key))) {
              return;
            }
            if (event.key === "@") {
              openMentionMenu("");
              return;
            }
            if (mentionQuery !== null) {
              refreshMentionQuery();
            }
          }}
          ref={editorRef}
          role="textbox"
          suppressContentEditableWarning
          tabIndex={0}
        />
      </div>
      {mentionMenu && typeof document !== "undefined" ? createPortal(mentionMenu, document.body) : null}
    </div>
  );
}

function formatPromptMention(mention: PromptMention): string {
  return `@${mention.kind}(${mention.name})`;
}

function balancedMentionOptions(mentions: PromptMention[]): PromptMention[] {
  const byKind = (kind: PromptMentionKind) => mentions.filter((mention) => mention.kind === kind).slice(0, 5);
  return [...byKind("agent"), ...byKind("skill"), ...byKind("tool")].slice(0, 15);
}

function renderPromptHTML(value: string, mentions: PromptMention[]): string {
  if (!value) return "";

  const mentionRegistry = new Map<string, PromptMention>();
  mentions.forEach((mention) => {
    mentionRegistry.set(`${mention.kind}:${mention.name.toLowerCase()}`, mention);
    mentionRegistry.set(`${mention.kind}:${mention.id.toLowerCase()}`, mention);
  });

  const parts: string[] = [];
  const pattern = /@(tool|skill|agent)\(([^)]+)\)/gi;
  let cursor = 0;
  for (const match of value.matchAll(pattern)) {
    const index = match.index ?? 0;
    parts.push(escapePromptHTML(value.slice(cursor, index)));
    const kind = match[1].toLowerCase() as PromptMentionKind;
    const key = `${kind}:${match[2].trim().toLowerCase()}`;
    const mention = mentionRegistry.get(key);
    const token = mention ? formatPromptMention(mention) : match[0];
    parts.push(
      `<span class="prompt-token prompt-token-${kind}" contenteditable="false" data-prompt-token="true">${escapeHTML(token)}</span>`
    );
    cursor = index + match[0].length;
  }
  parts.push(escapePromptHTML(value.slice(cursor)));
  return parts.join("");
}

function escapePromptHTML(value: string): string {
  return escapeHTML(value).replace(/\n/g, "<br>");
}

function escapeHTML(value: string): string {
  return value.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

function readPromptText(editor: HTMLElement): string {
  return editor.innerText.replace(/\n$/, "");
}

function rangeBelongsTo(root: HTMLElement, range: Range): boolean {
  const containsStart = range.startContainer === root || root.contains(range.startContainer);
  const containsEnd = range.endContainer === root || root.contains(range.endContainer);
  return containsStart && containsEnd;
}

function caretAnchorRect(root: HTMLElement): DOMRect | null {
  const selection = window.getSelection();
  if (!selection || selection.rangeCount === 0) return null;
  const range = selection.getRangeAt(0);
  if (range.endContainer !== root && !root.contains(range.endContainer)) return null;

  const caretRange = range.cloneRange();
  caretRange.collapse(false);
  const rect = caretRange.getBoundingClientRect();
  if (rect.width > 0 || rect.height > 0) return rect;

  const firstRect = caretRange.getClientRects()[0];
  if (firstRect) return firstRect;

  const marker = document.createElement("span");
  marker.textContent = "\u200b";
  marker.setAttribute("data-prompt-caret-marker", "true");
  caretRange.insertNode(marker);
  const markerRect = marker.getBoundingClientRect();
  marker.parentNode?.removeChild(marker);

  selection.removeAllRanges();
  selection.addRange(range);
  return markerRect;
}

function mentionTriggerFromDomRange(editor: HTMLElement, triggerRange: Range | null): (MentionRange & { end: number }) | null {
  const selection = window.getSelection();
  if (!triggerRange || !rangeBelongsTo(editor, triggerRange) || !selection || selection.rangeCount === 0) {
    return null;
  }
  const selectionRange = selection.getRangeAt(0);
  if (!rangeBelongsTo(editor, selectionRange)) return null;

  const activeRange = triggerRange.cloneRange();
  try {
    activeRange.setEnd(selectionRange.endContainer, selectionRange.endOffset);
  } catch {
    return null;
  }

  const activeText = activeRange.toString();
  if (!activeText.startsWith("@")) return null;
  const query = activeText.slice(1);
  if (query.includes("@") || /[\s()\n\r]/.test(query)) return null;

  // Keep the DOM range as the source of truth for insertion, not a derived text offset.
  triggerRange.setEnd(selectionRange.endContainer, selectionRange.endOffset);
  const start = plainTextOffsetForBoundary(editor, activeRange.startContainer, activeRange.startOffset);
  return {
    start,
    end: start + activeText.length,
    query
  };
}

function replaceActiveMentionDomRange(
  editor: HTMLElement | null,
  triggerRange: Range | null,
  token: string
): { caretOffset: number } | null {
  if (!editor || !triggerRange || !rangeBelongsTo(editor, triggerRange)) return null;
  const range = triggerRange.cloneRange();
  const selectedText = range.toString();
  if (!selectedText.startsWith("@")) return null;

  const insertedText = `${token} `;
  range.deleteContents();
  const tokenNode = document.createTextNode(insertedText);
  range.insertNode(tokenNode);

  const caretRange = document.createRange();
  caretRange.setStart(tokenNode, insertedText.length);
  caretRange.collapse(true);
  const selection = window.getSelection();
  selection?.removeAllRanges();
  selection?.addRange(caretRange);

  return { caretOffset: plainTextCaretOffset(editor) };
}

function mentionTriggerFromEditor(editor: HTMLElement, value: string): (MentionRange & { end: number }) | null {
  const caretOffset = plainTextCaretOffset(editor);
  const trigger = findMentionTrigger(value, caretOffset);
  return trigger
    ? {
        start: trigger.start,
        end: caretOffset,
        query: trigger.query
      }
    : null;
}

function mentionTriggerFromPendingStart(
  value: string,
  caretOffset: number,
  pendingStart: number | null
): (MentionRange & { end: number }) | null {
  if (pendingStart === null) return null;
  const start = Math.max(0, Math.min(pendingStart, value.length));
  if (value[start] !== "@") return null;
  if (caretOffset < start + 1) return null;

  const query = value.slice(start + 1, caretOffset);
  if (query.includes("@") || /[\s()]/.test(query)) return null;
  return {
    start,
    end: caretOffset,
    query
  };
}

function findMentionTrigger(value: string, caretOffset: number): { start: number; query: string } | null {
  const beforeCaret = value.slice(0, caretOffset);
  const atIndex = beforeCaret.lastIndexOf("@");
  if (atIndex < 0) return null;
  const query = beforeCaret.slice(atIndex + 1);
  if (query.includes("@") || /[\s()]/.test(query)) return null;
  return {
    start: atIndex,
    query
  };
}

function resolveMentionInsertionRange(
  value: string,
  range: MentionRange | null,
  pendingStart: number | null,
  caretOffset: number
): MentionRange | null {
  const pendingRange = mentionRangeFromStart(value, pendingStart, range?.start === pendingStart ? range.end : null);
  if (pendingRange) return pendingRange;

  const cachedRange = mentionRangeFromStart(value, range?.start ?? null, range?.end ?? null);
  if (cachedRange) return cachedRange;

  const trigger = findMentionTrigger(value, caretOffset);
  return trigger ? mentionRangeFromStart(value, trigger.start, caretOffset) : null;
}

function mentionRangeFromStart(
  value: string,
  startValue: number | null,
  endValue: number | null
): MentionRange | null {
  if (startValue === null) return null;
  const start = Math.max(0, Math.min(startValue, value.length));
  if (value[start] !== "@") return null;

  const baseEnd = Math.max(start + 1, Math.min(endValue ?? start + 1, value.length));
  const query = value.slice(start + 1, baseEnd);
  if (query.includes("@") || /[()\n\r]/.test(query)) return null;

  return {
    start,
    end: baseEnd,
    query
  };
}

function isModifierKey(key: string): boolean {
  return key === "Shift" || key === "Control" || key === "Alt" || key === "Meta" || key === "CapsLock";
}

function isMenuControlKey(key: string): boolean {
  return key === "ArrowDown" || key === "ArrowUp" || key === "Enter" || key === "Tab" || key === "Escape";
}

function plainTextCaretOffset(root: HTMLElement): number {
  const selection = window.getSelection();
  if (!selection || selection.rangeCount === 0) return readPromptText(root).length;
  const range = selection.getRangeAt(0);
  if (range.endContainer !== root && !root.contains(range.endContainer)) {
    return readPromptText(root).length;
  }
  const prefixRange = range.cloneRange();
  prefixRange.selectNodeContents(root);
  prefixRange.setEnd(range.endContainer, range.endOffset);
  return prefixRange.toString().length;
}

function plainTextOffsetForBoundary(root: HTMLElement, container: Node, offset: number): number {
  const prefixRange = document.createRange();
  prefixRange.selectNodeContents(root);
  prefixRange.setEnd(container, offset);
  return prefixRange.toString().length;
}

function placeCaretAtEnd(element: HTMLElement) {
  const range = document.createRange();
  range.selectNodeContents(element);
  range.collapse(false);
  const selection = window.getSelection();
  selection?.removeAllRanges();
  selection?.addRange(range);
}

function placeCaretAtPlainTextOffset(element: HTMLElement, targetOffset: number) {
  const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
  let offset = Math.max(0, targetOffset);
  let node = walker.nextNode();

  while (node) {
    const text = node.textContent ?? "";
    if (offset <= text.length) {
      const range = document.createRange();
      range.setStart(node, offset);
      range.collapse(true);
      const selection = window.getSelection();
      selection?.removeAllRanges();
      selection?.addRange(range);
      return;
    }
    offset -= text.length;
    node = walker.nextNode();
  }

  placeCaretAtEnd(element);
}
