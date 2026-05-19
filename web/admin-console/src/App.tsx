import type { ReactElement } from "react";
import { useState } from "react";
import { AppShell, type SectionKey } from "./components/AppShell";
import { AgentsPage } from "./pages/AgentsPage";
import { AgentFlowsPage } from "./pages/AgentFlowsPage";
import { SkillsPage } from "./pages/SkillsPage";
import { ToolsPage } from "./pages/ToolsPage";
import { ConnectorsPage } from "./pages/ConnectorsPage";
import { KnowledgeBasesPage } from "./pages/KnowledgeBasesPage";
import { ModelGatewayPage } from "./pages/ModelGatewayPage";
import { ConfigWorkspaceProvider, useConfigWorkspace } from "./platform/ConfigWorkspaceProvider";

const pages: Record<SectionKey, () => ReactElement> = {
  agents: AgentsPage,
  agentFlows: AgentFlowsPage,
  skills: SkillsPage,
  tools: ToolsPage,
  connectors: ConnectorsPage,
  knowledge: KnowledgeBasesPage,
  models: ModelGatewayPage
};

export default function App() {
  return (
    <ConfigWorkspaceProvider>
      <ConsoleApp />
    </ConfigWorkspaceProvider>
  );
}

function ConsoleApp() {
  const [activeSection, setActiveSection] = useState<SectionKey>("agents");
  const Page = pages[activeSection];

  return (
    <AppShell activeSection={activeSection} onNavigate={setActiveSection} workspaceStatus={<WorkspaceStatus />}>
      <Page />
    </AppShell>
  );
}

function WorkspaceStatus() {
  const { activeBundle, activeBundleId, draftBundles, releaseBundles, loading, error } = useConfigWorkspace();
  const displayName = activeBundle?.name || activeBundleId || "No draft bundle";
  return (
    <>
      <span>Config Bundle</span>
      <strong title={displayName}>{loading ? "Loading..." : displayName}</strong>
      <small>{error ? "Runtime config offline" : `${draftBundles.length} drafts · ${releaseBundles.length} releases`}</small>
    </>
  );
}
