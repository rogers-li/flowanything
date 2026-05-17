// Package connector defines the platform-neutral connector abstraction.
//
// Connector Core models external systems as ConnectorSpec and their callable
// APIs as OperationSpec. It does not know concrete business systems such as
// Tavily, Feishu, Jira, or Confluence. Concrete protocols are plugged in via
// ProtocolExecutor implementations.
package connector
