import type { ReactNode } from "react";
import { Icon, type IconName } from "./Icon";

export type SectionKey =
  | "agents"
  | "agentFlows"
  | "skills"
  | "tools"
  | "connectors"
  | "knowledge"
  | "models";

type NavigationItem = {
  key: SectionKey;
  label: string;
  description: string;
  icon: IconName;
};

const navigation: NavigationItem[] = [
  { key: "agents", label: "Agents", description: "Profiles and testing", icon: "agent" },
  { key: "agentFlows", label: "Agent Flows", description: "Multi-agent flows", icon: "flow" },
  { key: "skills", label: "Skills", description: "Capabilities", icon: "skill" },
  { key: "tools", label: "Tools", description: "Governed actions", icon: "tool" },
  {
    key: "connectors",
    label: "Connectors",
    description: "External APIs",
    icon: "connector"
  },
  { key: "knowledge", label: "Knowledge Bases", description: "Documents and RAG", icon: "knowledge" },
  { key: "models", label: "Model Gateway", description: "Providers", icon: "model" }
];

type AppShellProps = {
  activeSection: SectionKey;
  onNavigate: (section: SectionKey) => void;
  children: ReactNode;
  workspaceStatus?: ReactNode;
};

export function AppShell({ activeSection, onNavigate, children, workspaceStatus }: AppShellProps) {
  return (
    <div className="console-shell">
      <a className="skip-link" href="#main-content">
        Skip to main content
      </a>

      <aside className="console-sidebar" aria-label="Primary navigation">
        <button className="brand-button" type="button" onClick={() => onNavigate("agents")}>
          <span className="brand-mark" aria-hidden="true">
            FA
          </span>
          <div>
            <strong>Flow Anything</strong>
            <span>AI Platform Console</span>
          </div>
        </button>

        <nav className="section-tabs" aria-label="Primary navigation">
          {navigation.map((item) => (
            <button
              className={item.key === activeSection ? "section-tab section-tab-active" : "section-tab"}
              key={item.key}
              type="button"
              onClick={() => onNavigate(item.key)}
              aria-current={item.key === activeSection ? "page" : undefined}
            >
              <Icon name={item.icon} />
              <span className="section-tab-copy">
                <strong>{item.label}</strong>
              </span>
            </button>
          ))}
        </nav>

        {workspaceStatus ? (
          <div className="sidebar-status" aria-label="Workspace status">
            {workspaceStatus}
          </div>
        ) : null}
      </aside>

      <main id="main-content" className="console-main">
        {children}
      </main>
    </div>
  );
}
