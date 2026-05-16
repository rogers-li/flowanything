import { orchestratorApi, platformApi } from "../../lib/api";
import type { AgentDebugEvent, AgentDebugResponse, AgentDependencies, AgentProfile, AgentTrace } from "../../types/platform";

export type AgentsClient = {
  listAgents: () => Promise<AgentProfile[]>;
  saveAgent: (agent: AgentProfile) => Promise<AgentProfile>;
  enableAgent: (agentId: string) => Promise<AgentProfile>;
  disableAgent: (agentId: string) => Promise<AgentProfile>;
  getDependencies: (agentId: string) => Promise<AgentDependencies>;
  debugAgent: (evt: AgentDebugEvent) => Promise<AgentDebugResponse>;
  getTrace: (traceId: string) => Promise<AgentTrace>;
};

export const agentsClient: AgentsClient = {
  async listAgents() {
    const response = await platformApi.listAgents();
    return response.items;
  },
  saveAgent(agent) {
    if (agent.id) {
      return platformApi.updateAgent(agent);
    }
    return platformApi.createAgent(agent);
  },
  enableAgent(agentId) {
    return platformApi.enableAgent(agentId);
  },
  disableAgent(agentId) {
    return platformApi.disableAgent(agentId);
  },
  getDependencies(agentId) {
    return platformApi.getAgentDependencies(agentId);
  },
  debugAgent(evt) {
    return orchestratorApi.handleEvent(evt);
  },
  getTrace(traceId) {
    return orchestratorApi.getTrace(traceId);
  }
};
