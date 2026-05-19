import { useEffect, useState } from "react";
import { useConfigWorkspace } from "../../platform/ConfigWorkspaceProvider";
import { resourceApi } from "../../platform/configApi";
import type { ModelConfig } from "../../platform/configTypes";
import type { ModelProvider } from "../../types/platform";

export const fallbackModelProviders: ModelProvider[] = [
  {
    id: "provider_mock",
    name: "Local Mock Provider",
    type: "mock",
    baseUrl: "local",
    defaultModel: "mock-chat",
    status: "published",
    timeoutMillis: 30000
  },
  {
    id: "provider_deepseek",
    name: "DeepSeek",
    type: "deepseek",
    baseUrl: "https://api.deepseek.com",
    defaultModel: "deepseek-v4-flash",
    status: "published",
    timeoutMillis: 600000
  }
];

export function useModelProviders() {
  const workspace = useConfigWorkspace();
  const [providers, setProviders] = useState<ModelProvider[]>(fallbackModelProviders);

  useEffect(() => {
    void loadProviders();
  }, [workspace.activeBundleId]);

  async function loadProviders() {
    if (!workspace.activeBundleId) {
      setProviders(fallbackModelProviders);
      return;
    }
    try {
      const response = await resourceApi.listResourcesByKind<ModelConfig>(workspace.activeBundleId, "model");
      const nextProviders = response.items.map((item) => modelProviderFromConfig(item.resource));
      setProviders(nextProviders.length > 0 ? nextProviders : fallbackModelProviders);
    } catch {
      setProviders(fallbackModelProviders);
    }
  }

  return providers;
}

export function inferProviderIdFromModelProviders(model: string, providers: ModelProvider[]): string | undefined {
  const normalized = model.trim().toLowerCase();
  if (!normalized) return undefined;

  const provider = providers.find((item) => {
    const providerType = item.type.toLowerCase();
    return (
      item.id.toLowerCase() === normalized ||
      item.name.toLowerCase() === normalized ||
      item.defaultModel.toLowerCase() === normalized ||
      providerType === normalized ||
      normalized.startsWith(`${providerType}-`)
    );
  });

  return provider?.id;
}

function modelProviderFromConfig(config: ModelConfig): ModelProvider {
  const provider = providerType(config.provider);
  return {
    id: config.id,
    name: config.name,
    type: provider,
    baseUrl: typeof config.metadata?.base_url === "string" ? config.metadata.base_url : config.endpoint_ref ?? "",
    defaultModel: config.model ?? config.id,
    status: config.disabled ? "disabled" : "published",
    timeoutMillis: durationMillis(config.policy?.timeout, 30000)
  };
}

function providerType(value?: string): ModelProvider["type"] {
  if (value === "mock" || value === "deepseek") return value;
  return "openai-compatible";
}

function durationMillis(value: unknown, fallback: number): number {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value !== "string" || !value.trim()) return fallback;
  const normalized = value.trim();
  if (normalized.endsWith("ms")) return Number(normalized.slice(0, -2)) || fallback;
  if (normalized.endsWith("s")) return (Number(normalized.slice(0, -1)) || fallback / 1000) * 1000;
  return Number(normalized) || fallback;
}
