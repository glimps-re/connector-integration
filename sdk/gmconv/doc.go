// Package gmconv holds the canonical GMalware (gmalware.*) OpenTelemetry
// attribute keys and typed constructors shared across glimps connectors. They are
// a cross-service contract: the same key strings identify an analysis, a file and
// a verdict on spans, metrics and logs regardless of which connector emitted
// them, so an operator can correlate a single analysis across services.
//
// The package depends only on go.opentelemetry.io/otel/attribute, not the
// OpenTelemetry SDK. It is kept separate from sdk/telemetry on purpose: because
// Go compiles an imported package in full, folding these keys into sdk/telemetry
// would force every consumer of the key contract to compile the whole OTel
// provider tree. As its own leaf package, the contract stays importable without
// that cost.
package gmconv
