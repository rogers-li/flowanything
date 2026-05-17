// Package editor contains pure editing helpers for config-as-code drafts.
//
// The package deliberately does not persist data, call runtimes, or depend on a
// specific UI framework. Console, CLI, or future mobile editors can keep a full
// draft locally and call these helpers for inspection, patching, dependency
// analysis, and canvas-oriented workflow edits.
package editor
