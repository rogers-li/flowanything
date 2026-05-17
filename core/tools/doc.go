// Package tools defines the platform-neutral tool abstraction.
//
// Tool Core intentionally does not know whether a tool is backed by a connector
// operation, workflow, MCP server method, script, or native function. Those are
// implementation adapters registered behind a common ToolExecutor interface.
package tools
