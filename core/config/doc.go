// Package config defines the Config-as-Code protocol for Flow Anything.
//
// The package is intentionally runtime-neutral: Admin Console, CLI tools,
// server runtimes, mobile runtimes, and edge runtimes can all exchange the same
// BundleSpec. A host runtime may support only a subset of capabilities, but it
// should validate that subset through this protocol before execution.
package config
