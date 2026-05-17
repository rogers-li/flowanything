import { platformApi, runtimeApi } from "../../lib/api";
import type { ToolDependencies, ToolExecutionResult, ToolSpec } from "../../types/platform";

export type ToolsClient = {
  listTools: () => Promise<ToolSpec[]>;
  saveTool: (tool: ToolSpec) => Promise<ToolSpec>;
  importConnectorTools: (connectorId: string, tools: ToolSpec[]) => Promise<ToolSpec[]>;
  enableTool: (toolId: string) => Promise<ToolSpec>;
  disableTool: (toolId: string) => Promise<ToolSpec>;
  getDependencies: (toolId: string) => Promise<ToolDependencies>;
  executeTool: (toolId: string, args: Record<string, unknown>, confirmed: boolean) => Promise<ToolExecutionResult>;
};

export const toolsClient: ToolsClient = {
  async listTools() {
    const response = await platformApi.listTools();
    return response.items;
  },
  saveTool(tool) {
    if (tool.id) {
      return platformApi.updateTool(tool);
    }
    return platformApi.createTool(tool);
  },
  async importConnectorTools(connectorId, tools) {
    const response = await platformApi.importConnectorTools(connectorId, tools);
    return response.items;
  },
  enableTool(toolId) {
    return platformApi.enableTool(toolId);
  },
  disableTool(toolId) {
    return platformApi.disableTool(toolId);
  },
  getDependencies(toolId) {
    return platformApi.getToolDependencies(toolId);
  },
  executeTool(toolId, args, confirmed) {
    return runtimeApi.executeTool(toolId, args, confirmed);
  }
};
