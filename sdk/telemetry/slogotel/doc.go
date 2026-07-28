// Package slogotel bridges the standard library's slog to the telemetry package
// for slog-based connectors. It provides slog.Handler wrappers that add trace
// correlation, fan a record out to several sinks, gate a sink by level, and feed
// records into an OpenTelemetry log provider.
//
// [NewHandler] assembles the common pipeline in one call: a JSON handler on an
// io.Writer carrying trace_id/span_id fields, fanned out alongside a level-gated
// OTLP bridge. The stdout sink carries the trace fields for human/log-search
// correlation; the OTLP bridge needs none, because OTLP log records carry trace
// context natively.
//
// This subpackage's only extra dependency over the core telemetry package is the
// otelslog bridge, so connectors that log through another backend never compile
// it in.
package slogotel
