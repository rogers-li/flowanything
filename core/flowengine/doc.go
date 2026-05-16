// Package flowengine contains a standalone workflow execution core.
//
// The package intentionally has no dependency on the current platform
// services. Product modules can adapt their own Agent, Tool, Connector, or
// Workflow concepts into NodeExecutor implementations without polluting the
// engine itself.
package flowengine
