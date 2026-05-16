import { connectorApi, platformApi } from "../../lib/api";
import type { Connector, ConnectorDependencies, ConnectorInvokeResult, ConnectorOperation } from "../../types/platform";

export type ConnectorOperationsClient = {
  listConnectors: () => Promise<Connector[]>;
  saveConnector: (connector: Connector) => Promise<Connector>;
  enableConnector: (connectorId: string) => Promise<Connector>;
  disableConnector: (connectorId: string) => Promise<Connector>;
  listOperations: () => Promise<ConnectorOperation[]>;
  saveOperation: (operation: ConnectorOperation) => Promise<ConnectorOperation>;
  enableOperation: (operationId: string) => Promise<ConnectorOperation>;
  disableOperation: (operationId: string) => Promise<ConnectorOperation>;
  getDependencies: (operationId: string) => Promise<ConnectorDependencies>;
  testOperation: (operationId: string, args: Record<string, unknown>) => Promise<ConnectorInvokeResult>;
};

export const connectorOperationsClient: ConnectorOperationsClient = {
  async listConnectors() {
    const response = await platformApi.listConnectors();
    return response.items;
  },
  saveConnector(connector) {
    if (connector.id) {
      return platformApi.updateConnector(connector);
    }
    return platformApi.createConnector(connector);
  },
  enableConnector(connectorId) {
    return platformApi.enableConnector(connectorId);
  },
  disableConnector(connectorId) {
    return platformApi.disableConnector(connectorId);
  },
  async listOperations() {
    const response = await platformApi.listConnectorOperations();
    return response.items;
  },
  saveOperation(operation) {
    if (operation.id) {
      return platformApi.updateConnectorOperation(operation);
    }
    return platformApi.createConnectorOperation(operation);
  },
  enableOperation(operationId) {
    return platformApi.enableConnectorOperation(operationId);
  },
  disableOperation(operationId) {
    return platformApi.disableConnectorOperation(operationId);
  },
  getDependencies(operationId) {
    return platformApi.getConnectorDependencies(operationId);
  },
  testOperation(operationId, args) {
    return connectorApi.invokeOperation(operationId, args);
  }
};
