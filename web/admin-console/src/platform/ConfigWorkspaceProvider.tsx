import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { bundleApi } from "./configApi";
import type {
  BundleEntrypoint,
  BundleInspection,
  BundleSpec,
  BundleSummary,
  PreviewBundleResponse,
  PublishAndReloadResponse,
  PublishResult,
  RuntimeTarget
} from "./configTypes";

const activeBundleStorageKey = "flow-anything.config.active-draft-bundle-id";
const defaultDraftBundleId = import.meta.env.VITE_DEFAULT_BUNDLE_ID ?? "workspace_default";

export type ConfigWorkspaceState = {
  loading: boolean;
  error: string;
  draftBundles: BundleSummary[];
  previewBundles: BundleSummary[];
  releaseBundles: BundleSummary[];
  activeBundleId: string;
  activeBundle: BundleSpec | null;
  inspection: BundleInspection | null;
  selectBundle: (bundleId: string) => Promise<void>;
  refresh: () => Promise<void>;
  createDraftBundle: (draft?: Partial<BundleSpec>) => Promise<BundleSpec>;
  saveActiveBundle: (bundle: BundleSpec) => Promise<BundleSpec>;
  buildPreview: (entrypoint: BundleEntrypoint) => Promise<PreviewBundleResponse>;
  publish: () => Promise<PublishResult>;
  publishAndReload: () => Promise<PublishAndReloadResponse>;
};

const ConfigWorkspaceContext = createContext<ConfigWorkspaceState | null>(null);

export function ConfigWorkspaceProvider({ children }: { children: ReactNode }) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [draftBundles, setDraftBundles] = useState<BundleSummary[]>([]);
  const [previewBundles, setPreviewBundles] = useState<BundleSummary[]>([]);
  const [releaseBundles, setReleaseBundles] = useState<BundleSummary[]>([]);
  const [activeBundleId, setActiveBundleId] = useState(() => localStorage.getItem(activeBundleStorageKey) ?? defaultDraftBundleId);
  const [activeBundle, setActiveBundle] = useState<BundleSpec | null>(null);
  const [inspection, setInspection] = useState<BundleInspection | null>(null);

  const loadBundle = useCallback(async (bundleId: string) => {
    if (!bundleId) {
      setActiveBundle(null);
      setInspection(null);
      return;
    }
    const [{ bundle }, inspected] = await Promise.all([bundleApi.getBundle(bundleId), bundleApi.inspectBundle(bundleId, "draft")]);
    setActiveBundle(bundle);
    setInspection(inspected);
  }, []);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [drafts, previews, releases] = await Promise.all([
        bundleApi.listDrafts(),
        bundleApi.listPreviews(),
        bundleApi.listReleases()
      ]);
      setDraftBundles(drafts.items);
      setPreviewBundles(previews.items);
      setReleaseBundles(releases.items);
      const nextActiveId = pickActiveBundleId(activeBundleId, drafts.items);
      setActiveBundleId(nextActiveId);
      if (nextActiveId) {
        localStorage.setItem(activeBundleStorageKey, nextActiveId);
        await loadBundle(nextActiveId);
      } else {
        setActiveBundle(null);
        setInspection(null);
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Failed to load config workspace.");
      setActiveBundle(null);
      setInspection(null);
    } finally {
      setLoading(false);
    }
  }, [activeBundleId, loadBundle]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const selectBundle = useCallback(
    async (bundleId: string) => {
      setActiveBundleId(bundleId);
      localStorage.setItem(activeBundleStorageKey, bundleId);
      setError("");
      setLoading(true);
      try {
        await loadBundle(bundleId);
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : "Failed to select bundle.");
      } finally {
        setLoading(false);
      }
    },
    [loadBundle]
  );

  const createDraftBundle = useCallback(
    async (draft: Partial<BundleSpec> = {}) => {
      const bundle = createBundleDraft(draft);
      const response = await bundleApi.saveBundle(bundle);
      setActiveBundleId(response.bundle.id);
      localStorage.setItem(activeBundleStorageKey, response.bundle.id);
      await refresh();
      return response.bundle;
    },
    [refresh]
  );

  const saveActiveBundle = useCallback(
    async (bundle: BundleSpec) => {
      const response = await bundleApi.updateBundle(bundle);
      setActiveBundle(response.bundle);
      setInspection(await bundleApi.inspectBundle(response.bundle.id, "draft"));
      await refresh();
      return response.bundle;
    },
    [refresh]
  );

  const buildPreview = useCallback(
    async (entrypoint: BundleEntrypoint) => {
      if (!activeBundleId) throw new Error("No active draft bundle selected.");
      const response = await bundleApi.buildPreview(activeBundleId, entrypoint);
      await refresh();
      return response;
    },
    [activeBundleId, refresh]
  );

  const publish = useCallback(async () => {
    if (!activeBundleId) throw new Error("No active draft bundle selected.");
    const response = await bundleApi.publish(activeBundleId);
    await refresh();
    return response;
  }, [activeBundleId, refresh]);

  const publishAndReload = useCallback(async () => {
    if (!activeBundleId) throw new Error("No active draft bundle selected.");
    const response = await bundleApi.publishAndReload(activeBundleId);
    await refresh();
    return response;
  }, [activeBundleId, refresh]);

  const value = useMemo<ConfigWorkspaceState>(
    () => ({
      loading,
      error,
      draftBundles,
      previewBundles,
      releaseBundles,
      activeBundleId,
      activeBundle,
      inspection,
      selectBundle,
      refresh,
      createDraftBundle,
      saveActiveBundle,
      buildPreview,
      publish,
      publishAndReload
    }),
    [
      loading,
      error,
      draftBundles,
      previewBundles,
      releaseBundles,
      activeBundleId,
      activeBundle,
      inspection,
      selectBundle,
      refresh,
      createDraftBundle,
      saveActiveBundle,
      buildPreview,
      publish,
      publishAndReload
    ]
  );

  return <ConfigWorkspaceContext.Provider value={value}>{children}</ConfigWorkspaceContext.Provider>;
}

export function useConfigWorkspace() {
  const value = useContext(ConfigWorkspaceContext);
  if (!value) {
    throw new Error("useConfigWorkspace must be used inside ConfigWorkspaceProvider.");
  }
  return value;
}

function pickActiveBundleId(current: string, drafts: BundleSummary[]): string {
  if (drafts.some((bundle) => bundle.id === current)) return current;
  return drafts[0]?.id ?? "";
}

function createBundleDraft(draft: Partial<BundleSpec>): BundleSpec {
  const id = draft.id ?? defaultDraftBundleId;
  const targets: RuntimeTarget[] = ["server"];
  return {
    schema_version: "v1",
    kind: "flow-anything.bundle",
    id,
    name: draft.name ?? "Default Workspace Bundle",
    version: draft.version ?? "draft",
    description: draft.description ?? "",
    runtime: draft.runtime ?? {
      targets,
      min_runtime_version: "v1"
    },
    dependencies: draft.dependencies ?? [],
    permissions: draft.permissions ?? {
      network_domains: [],
      secret_refs: [],
      file_scopes: []
    },
    signature: draft.signature ?? {},
    metadata: draft.metadata ?? {},
    resources: draft.resources ?? {
      agents: [],
      skills: [],
      tools: [],
      workflows: [],
      connectors: [],
      models: [],
      knowledge_bases: [],
      policies: []
    }
  };
}
