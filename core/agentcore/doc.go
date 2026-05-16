// Package agentcore contains a standalone LLM agent execution core.
//
// The core knows how to run reasoning strategies and invoke generic
// capabilities. It intentionally does not know platform concepts such as
// Connector, Knowledge Base, Workflow Tool, or Agent Flow; those concepts should
// be adapted into Capability implementations by product modules.
package agentcore
