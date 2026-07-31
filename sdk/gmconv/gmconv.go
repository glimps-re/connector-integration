package gmconv

import "go.opentelemetry.io/otel/attribute"

// The GMalware cross-service attribute keys. These string values are a stable
// contract shared across connectors and the console; do not change them.
const (
	// AnalysisSIDKey is the AssemblyLine submission SID: the id shared by every
	// span and log line of one analysis across services, so an operator can
	// retrieve a whole analysis by attribute even where the trace id does not
	// survive an async boundary. It may be empty (e.g. gmapi without AV-print
	// permission), in which case the attribute is omitted.
	AnalysisSIDKey = attribute.Key("gmalware.analysis.sid")
	// AnalysisUUIDKey is the backend analysis UUID used by the GMalware UI to look
	// up an analysis. Informational, not a cross-service correlation key.
	AnalysisUUIDKey = attribute.Key("gmalware.analysis.uuid")
	// FileSHA256Key is the analyzed file's SHA-256: a secondary, non-unique
	// correlation pivot that varies across the files of one analysis.
	FileSHA256Key = attribute.Key("gmalware.file.sha256")
	// ServiceNameKey names the analyzer producing a span.
	ServiceNameKey = attribute.Key("gmalware.service.name")
	// AnalysisMalwareKey is the final analysis verdict (malicious or not).
	// Analysis-level: the outcome for the whole analysis, not a per-service
	// result.
	AnalysisMalwareKey = attribute.Key("gmalware.analysis.malware")
	// AnalysisScoreKey is the final analysis score. Analysis-level, same scope as
	// the verdict.
	AnalysisScoreKey = attribute.Key("gmalware.analysis.score")
)

// AnalysisSID builds the gmalware.analysis.sid attribute.
func AnalysisSID(v string) attribute.KeyValue { return AnalysisSIDKey.String(v) }

// AnalysisUUID builds the gmalware.analysis.uuid attribute.
func AnalysisUUID(v string) attribute.KeyValue { return AnalysisUUIDKey.String(v) }

// FileSHA256 builds the gmalware.file.sha256 attribute.
func FileSHA256(v string) attribute.KeyValue { return FileSHA256Key.String(v) }

// ServiceName builds the gmalware.service.name attribute.
func ServiceName(v string) attribute.KeyValue { return ServiceNameKey.String(v) }

// AnalysisMalware builds the gmalware.analysis.malware attribute.
func AnalysisMalware(v bool) attribute.KeyValue { return AnalysisMalwareKey.Bool(v) }

// AnalysisScore builds the gmalware.analysis.score attribute.
func AnalysisScore(v int) attribute.KeyValue { return AnalysisScoreKey.Int(v) }
