import { useEffect, useMemo, useState } from "react";
import { Badge } from "../components/Badge";
import { DataTable } from "../components/DataTable";
import { PageHeader } from "../components/PageHeader";
import { useConfigWorkspace } from "../platform/ConfigWorkspaceProvider";
import { resourceApi } from "../platform/configApi";
import type { KnowledgeConfig } from "../platform/configTypes";
import type { KnowledgeBase, KnowledgeDocument, KnowledgeSearchResult } from "../types/platform";

const statusTone = {
  draft: "gray",
  enabled: "green",
  disabled: "red"
} as const;

type KnowledgeDraft = {
  name: string;
  description: string;
  embeddingModel: string;
};

type DocumentDraft = {
  title: string;
  text: string;
};

function createKnowledgeDraft(): KnowledgeDraft {
  return {
    name: "",
    description: "",
    embeddingModel: "lexical-memory"
  };
}

function createDocumentDraft(): DocumentDraft {
  return {
    title: "",
    text: ""
  };
}

export function KnowledgeBasesPage() {
  const workspace = useConfigWorkspace();
  const [knowledgeConfigs, setKnowledgeConfigs] = useState<KnowledgeConfig[]>([]);
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBase[]>([]);
  const [selectedBaseId, setSelectedBaseId] = useState("");
  const [documents, setDocuments] = useState<KnowledgeDocument[]>([]);
  const [knowledgeDraft, setKnowledgeDraft] = useState<KnowledgeDraft>(() => createKnowledgeDraft());
  const [documentDraft, setDocumentDraft] = useState<DocumentDraft>(() => createDocumentDraft());
  const [searchText, setSearchText] = useState("");
  const [searchResult, setSearchResult] = useState<KnowledgeSearchResult | null>(null);
  const [notice, setNotice] = useState<{ ok: boolean; message: string } | null>(null);
  const [loading, setLoading] = useState(false);

  const selectedBase = useMemo(
    () => knowledgeBases.find((item) => item.id === selectedBaseId) ?? knowledgeBases[0],
    [knowledgeBases, selectedBaseId]
  );

  useEffect(() => {
    void loadKnowledgeBases();
  }, [workspace.activeBundleId]);

  useEffect(() => {
    if (!notice) return;
    const timeout = window.setTimeout(() => setNotice(null), 4500);
    return () => window.clearTimeout(timeout);
  }, [notice]);

  useEffect(() => {
    if (!selectedBase?.id) {
      setDocuments([]);
      return;
    }
    void loadDocuments(selectedBase.id);
  }, [selectedBase?.id]);

  const loadKnowledgeBases = async () => {
    setLoading(true);
    try {
      if (!workspace.activeBundleId) {
        setKnowledgeConfigs([]);
        setKnowledgeBases([]);
        setSelectedBaseId("");
        return;
      }
      const resp = await resourceApi.listResourcesByKind<KnowledgeConfig>(workspace.activeBundleId, "knowledge");
      const configs = resp.items.map((item) => item.resource);
      const bases = configs.map(knowledgeBaseFromConfig);
      setKnowledgeConfigs(configs);
      setKnowledgeBases(bases);
      setSelectedBaseId((current) => current || bases[0]?.id || "");
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to load knowledge bases." });
    } finally {
      setLoading(false);
    }
  };

  const loadDocuments = async (kbId: string) => {
    const config = knowledgeConfigs.find((item) => item.id === kbId);
    setDocuments(config ? documentsFromKnowledgeConfig(config) : []);
  };

  const createKnowledgeBase = async () => {
    if (!knowledgeDraft.name.trim()) {
      setNotice({ ok: false, message: "Knowledge base name is required." });
      return;
    }
    try {
      if (!workspace.activeBundleId) throw new Error("No active config bundle selected.");
      const saved = knowledgeConfigFromDraft(knowledgeDraft);
      await resourceApi.upsertResource(workspace.activeBundleId, "knowledge", saved);
      setKnowledgeDraft(createKnowledgeDraft());
      setSelectedBaseId(saved.id);
      await loadKnowledgeBases();
      setNotice({ ok: true, message: "Knowledge base created." });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to create knowledge base." });
    }
  };

  const changeKnowledgeBaseStatus = async (status: "enabled" | "disabled") => {
    if (!selectedBase) return;
    try {
      if (!workspace.activeBundleId) throw new Error("No active config bundle selected.");
      const current = knowledgeConfigs.find((item) => item.id === selectedBase.id);
      if (!current) throw new Error("Knowledge base not found in active config bundle.");
      const saved: KnowledgeConfig = {
        ...current,
        disabled: status !== "enabled",
        metadata: {
          ...(current.metadata ?? {}),
          status,
          updated_at: new Date().toISOString()
        }
      };
      await resourceApi.upsertResource(workspace.activeBundleId, "knowledge", saved);
      setKnowledgeConfigs((items) => items.map((item) => (item.id === saved.id ? saved : item)));
      setKnowledgeBases((items) => items.map((item) => (item.id === saved.id ? knowledgeBaseFromConfig(saved) : item)));
      setNotice({ ok: true, message: `Knowledge base ${status}.` });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to change status." });
    }
  };

  const indexDocument = async () => {
    if (!selectedBase) {
      setNotice({ ok: false, message: "Select a knowledge base first." });
      return;
    }
    if (!documentDraft.title.trim() || !documentDraft.text.trim()) {
      setNotice({ ok: false, message: "Document title and content are required." });
      return;
    }
    try {
      if (!workspace.activeBundleId) throw new Error("No active config bundle selected.");
      const current = knowledgeConfigs.find((item) => item.id === selectedBase.id);
      if (!current) throw new Error("Knowledge base not found in active config bundle.");
      const nextDocuments = [
        ...documentsFromKnowledgeConfig(current),
        {
          id: stableResourceId("doc", documentDraft.title),
          tenantId: "tenant_1",
          kbId: selectedBase.id,
          title: documentDraft.title.trim(),
          text: documentDraft.text,
          version: "v1",
          metadata: {
            title: documentDraft.title.trim()
          }
        }
      ];
      const saved = knowledgeConfigWithDocuments(current, nextDocuments);
      await resourceApi.upsertResource(workspace.activeBundleId, "knowledge", saved);
      setKnowledgeConfigs((items) => items.map((item) => (item.id === saved.id ? saved : item)));
      setKnowledgeBases((items) => items.map((item) => (item.id === saved.id ? knowledgeBaseFromConfig(saved) : item)));
      setDocumentDraft(createDocumentDraft());
      setDocuments(nextDocuments);
      setNotice({ ok: true, message: "Document indexed." });
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Failed to index document." });
    }
  };

  const runSearch = async () => {
    if (!selectedBase) return;
    if (!searchText.trim()) {
      setNotice({ ok: false, message: "Search text is required." });
      return;
    }
    try {
      setSearchResult(localKnowledgeSearch(selectedBase.id, documents, searchText.trim()));
    } catch (error) {
      setNotice({ ok: false, message: error instanceof Error ? error.message : "Search failed." });
    }
  };

  return (
    <div className="page-stack">
      <PageHeader
        eyebrow="Knowledge Bases"
        title="Manage retrieval knowledge for agents"
        description="Create knowledge bases, index documents, and test retrieval before binding knowledge tools to agents."
        actions={
          notice ? <span className={notice.ok ? "inline-notice inline-notice-ok" : "inline-notice inline-notice-error"}>{notice.message}</span> : null
        }
      />

      <section className="content-grid">
        <article className="panel wide-panel">
          <div className="panel-heading">
            <div>
              <span className="eyebrow">Knowledge Inventory</span>
              <h2>Knowledge bases</h2>
            </div>
            <button className="secondary-action compact-action" type="button" onClick={() => void loadKnowledgeBases()}>
              {loading ? "Loading..." : "Refresh"}
            </button>
          </div>

          <DataTable<KnowledgeBase>
            rows={knowledgeBases}
            getRowKey={(item) => item.id}
            onRowClick={(item) => setSelectedBaseId(item.id)}
            columns={[
              {
                key: "name",
                header: "Name",
                render: (item) => (
                  <div className="stacked-cell">
                    <strong>{item.name}</strong>
                    <code>{item.id}</code>
                  </div>
                )
              },
              { key: "description", header: "Description", render: (item) => item.description || "No description" },
              { key: "documents", header: "Documents", render: (item) => `${item.documentCount}` },
              { key: "chunks", header: "Chunks", render: (item) => `${item.chunkCount}` },
              { key: "embedding", header: "Embedding", render: (item) => item.embeddingModel || "lexical-memory" },
              {
                key: "status",
                header: "Status",
                render: (item) => <Badge tone={statusTone[item.status]}>{item.status}</Badge>
              },
              { key: "updated", header: "Updated", render: (item) => formatDate(item.updatedAt) }
            ]}
            emptyMessage="No knowledge bases yet. Create one from the setup panel."
          />
        </article>

        <aside className="panel">
          <div className="panel-heading">
            <div>
              <span className="eyebrow">Create</span>
              <h2>New knowledge base</h2>
            </div>
          </div>
          <div className="form-stack">
            <label>
              Name
              <input value={knowledgeDraft.name} onChange={(event) => setKnowledgeDraft({ ...knowledgeDraft, name: event.target.value })} />
            </label>
            <label>
              Description
              <textarea value={knowledgeDraft.description} onChange={(event) => setKnowledgeDraft({ ...knowledgeDraft, description: event.target.value })} />
            </label>
            <label>
              Retrieval backend
              <input value={knowledgeDraft.embeddingModel} onChange={(event) => setKnowledgeDraft({ ...knowledgeDraft, embeddingModel: event.target.value })} />
            </label>
            <button className="primary-action compact-action" type="button" onClick={() => void createKnowledgeBase()}>
              Create Knowledge Base
            </button>
          </div>
        </aside>
      </section>

      {selectedBase ? (
        <section className="content-grid">
          <article className="panel wide-panel">
            <div className="panel-heading">
              <div>
                <span className="eyebrow">Selected Knowledge Base</span>
                <h2>{selectedBase.name}</h2>
              </div>
              <div className="button-row">
                {selectedBase.status === "enabled" ? (
                  <button className="secondary-action compact-action" type="button" onClick={() => void changeKnowledgeBaseStatus("disabled")}>
                    Disable
                  </button>
                ) : (
                  <button className="primary-action compact-action" type="button" onClick={() => void changeKnowledgeBaseStatus("enabled")}>
                    Enable
                  </button>
                )}
              </div>
            </div>

            <DataTable<KnowledgeDocument>
              rows={documents}
              getRowKey={(item) => item.id}
              columns={[
                {
                  key: "title",
                  header: "Document",
                  render: (item) => (
                    <div className="stacked-cell">
                      <strong>{item.title}</strong>
                      <code>{item.id}</code>
                    </div>
                  )
                },
                { key: "version", header: "Version", render: (item) => item.version },
                { key: "preview", header: "Preview", render: (item) => item.text.slice(0, 120) || "No content" }
              ]}
              emptyMessage="No documents indexed in this knowledge base."
            />
          </article>

          <aside className="panel">
            <div className="panel-heading">
              <div>
                <span className="eyebrow">Index</span>
                <h2>Add document</h2>
              </div>
            </div>
            <div className="form-stack">
              <label>
                Title
                <input value={documentDraft.title} onChange={(event) => setDocumentDraft({ ...documentDraft, title: event.target.value })} />
              </label>
              <label>
                Markdown / Text
                <textarea
                  className="large-textarea"
                  value={documentDraft.text}
                  onChange={(event) => setDocumentDraft({ ...documentDraft, text: event.target.value })}
                />
              </label>
              <button className="primary-action compact-action" type="button" onClick={() => void indexDocument()}>
                Index Document
              </button>
            </div>
          </aside>
        </section>
      ) : null}

      {selectedBase ? (
        <section className="panel">
          <div className="panel-heading">
            <div>
              <span className="eyebrow">Retrieval Test</span>
              <h2>Search this knowledge base</h2>
            </div>
          </div>
          <div className="knowledge-search-layout">
            <div className="form-stack">
              <label>
                Query
                <textarea value={searchText} onChange={(event) => setSearchText(event.target.value)} placeholder="Ask a question against indexed documents..." />
              </label>
              <button className="secondary-action compact-action" type="button" onClick={() => void runSearch()}>
                Run Search
              </button>
            </div>
            <div className="knowledge-result-list">
              {(searchResult?.chunks ?? []).map((chunk) => (
                <article key={chunk.id} className="knowledge-result-item">
                  <div>
                    <strong>{chunk.metadata?.title ? String(chunk.metadata.title) : chunk.docId}</strong>
                    <span>{chunk.score.toFixed(2)}</span>
                  </div>
                  <p>{chunk.text}</p>
                </article>
              ))}
              {searchResult && searchResult.chunks.length === 0 ? <p className="muted-copy">No chunks matched this query.</p> : null}
            </div>
          </div>
        </section>
      ) : null}
    </div>
  );
}

function formatDate(value: string): string {
  if (!value) return "Not indexed";
  const timestamp = Date.parse(value);
  if (Number.isNaN(timestamp)) return value;
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(timestamp));
}

function knowledgeConfigFromDraft(draft: KnowledgeDraft): KnowledgeConfig {
  const id = stableResourceId("kb", draft.name);
  const now = new Date().toISOString();
  return {
    id,
    name: draft.name.trim(),
    description: draft.description.trim(),
    version: "v1",
    disabled: true,
    metadata: {
      status: "draft",
      updated_at: now
    },
    type: "manual",
    source: {
      kind: "manual",
      config: {
        documents: []
      }
    },
    embedding_model_ref: {
      kind: "model",
      id: draft.embeddingModel.trim() || "lexical-memory"
    },
    runtime: {
      server_proxy_allowed: true
    }
  };
}

function knowledgeBaseFromConfig(config: KnowledgeConfig): KnowledgeBase {
  const documents = documentsFromKnowledgeConfig(config);
  return {
    id: config.id,
    tenantId: "tenant_1",
    name: config.name,
    description: config.description,
    status: knowledgeStatus(config),
    embeddingModel: config.embedding_model_ref?.id,
    documentCount: documents.length,
    chunkCount: documents.length,
    metadata: config.metadata,
    version: config.version ?? "v1",
    updatedAt: typeof config.metadata?.updated_at === "string" ? config.metadata.updated_at : ""
  };
}

function knowledgeConfigWithDocuments(config: KnowledgeConfig, documents: KnowledgeDocument[]): KnowledgeConfig {
  return {
    ...config,
    metadata: {
      ...(config.metadata ?? {}),
      updated_at: new Date().toISOString()
    },
    source: {
      kind: config.source?.kind ?? "manual",
      uri: config.source?.uri,
      config: {
        ...(config.source?.config ?? {}),
        documents
      }
    }
  };
}

function documentsFromKnowledgeConfig(config: KnowledgeConfig): KnowledgeDocument[] {
  const documents = config.source?.config?.documents;
  if (!Array.isArray(documents)) return [];
  return documents
    .map((item) => (item && typeof item === "object" && !Array.isArray(item) ? (item as Partial<KnowledgeDocument>) : null))
    .filter((item): item is Partial<KnowledgeDocument> => Boolean(item))
    .map((item) => ({
      id: typeof item.id === "string" ? item.id : stableResourceId("doc", item.title ?? config.name),
      tenantId: "tenant_1",
      kbId: config.id,
      title: typeof item.title === "string" ? item.title : "Untitled document",
      text: typeof item.text === "string" ? item.text : "",
      metadata: item.metadata,
      version: typeof item.version === "string" ? item.version : "v1"
    }));
}

function localKnowledgeSearch(kbId: string, documents: KnowledgeDocument[], query: string): KnowledgeSearchResult {
  const terms = query
    .toLowerCase()
    .split(/\s+/)
    .map((term) => term.trim())
    .filter(Boolean);
  const chunks = documents
    .map((document) => {
      const haystack = `${document.title}\n${document.text}`.toLowerCase();
      const hits = terms.reduce((count, term) => count + (haystack.includes(term) ? 1 : 0), 0);
      return {
        id: `${document.id}_preview`,
        docId: document.id,
        kbId,
        text: document.text.slice(0, 800),
        score: terms.length === 0 ? 0 : hits / terms.length,
        metadata: {
          ...(document.metadata ?? {}),
          title: document.title
        }
      };
    })
    .filter((chunk) => chunk.score > 0)
    .sort((left, right) => right.score - left.score)
    .slice(0, 5);
  return {
    queryId: stableResourceId("query", query),
    chunks
  };
}

function knowledgeStatus(config: KnowledgeConfig): KnowledgeBase["status"] {
  const status = config.metadata?.status;
  if (status === "draft" || status === "enabled" || status === "disabled") return status;
  return config.disabled ? "disabled" : "enabled";
}

function stableResourceId(prefix: string, name: string): string {
  const normalized = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
  return `${prefix}_${normalized || Date.now().toString(16)}`;
}
