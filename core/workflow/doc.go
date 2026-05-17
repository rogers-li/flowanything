// Package workflow defines the application-level workflow document protocol.
//
// A WorkflowDocument is intentionally close to flowengine.FlowSpec:
//
//	document.spec    = executable engine protocol
//	document.ui      = editor-only metadata
//	document.publish = publish/snapshot metadata
//
// The compiler in this package is deliberately thin. It normalizes, validates,
// and freezes a snapshot, but does not rebuild hidden execution semantics that
// would make the editor view diverge from runtime behavior.
package workflow
