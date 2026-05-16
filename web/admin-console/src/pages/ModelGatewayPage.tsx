import { Badge } from "../components/Badge";
import { DataTable } from "../components/DataTable";
import { PageHeader } from "../components/PageHeader";
import { modelProviders } from "../lib/mockData";
import type { ModelProvider } from "../types/platform";

export function ModelGatewayPage() {
  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="Model Gateway"
        title="Manage provider routes and defaults"
        description="Centralize model provider metadata before adding routing, fallback, token usage, and cost analytics."
      />

      <article className="panel">
        <DataTable<ModelProvider>
          rows={modelProviders}
          getRowKey={(provider) => provider.id}
          columns={[
            {
              key: "provider",
              header: "Provider",
              render: (provider) => (
                <div className="stacked-cell">
                  <strong>{provider.name}</strong>
                  <code>{provider.id}</code>
                </div>
              )
            },
            { key: "type", header: "Type", render: (provider) => provider.type },
            { key: "model", header: "Default Model", render: (provider) => <code>{provider.defaultModel}</code> },
            { key: "timeout", header: "Timeout", render: (provider) => `${provider.timeoutMillis}ms` },
            {
              key: "status",
              header: "Status",
              render: (provider) => (
                <Badge tone={provider.status === "published" ? "green" : "amber"}>{provider.status}</Badge>
              )
            }
          ]}
        />
      </article>
    </div>
  );
}
