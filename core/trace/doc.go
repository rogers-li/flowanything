// Package trace consumes core events and builds queryable trace spans.
//
// Flow Engine, Agent Core, Tool Core, and Connector Core only publish events.
// This package listens to those events and converts them into an internal span
// model aligned with OpenTelemetry/OpenInference concepts.
package trace
