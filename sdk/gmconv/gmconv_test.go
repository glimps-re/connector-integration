package gmconv

import (
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

// TestKeysPinned locks the cross-service key strings. A connector or the console
// joins on these exact values, so a change here is a deliberate contract change.
func TestKeysPinned(t *testing.T) {
	want := map[attribute.Key]string{
		AnalysisSIDKey:     "gmalware.analysis.sid",
		AnalysisUUIDKey:    "gmalware.analysis.uuid",
		FileSHA256Key:      "gmalware.file.sha256",
		ServiceNameKey:     "gmalware.service.name",
		AnalysisMalwareKey: "gmalware.analysis.malware",
		AnalysisScoreKey:   "gmalware.analysis.score",
	}
	for k, s := range want {
		if string(k) != s {
			t.Errorf("key = %q, want %q", string(k), s)
		}
	}
}

func TestConstructors(t *testing.T) {
	if kv := AnalysisSID("sid-1"); kv.Key != AnalysisSIDKey || kv.Value.AsString() != "sid-1" {
		t.Errorf("AnalysisSID = %v, want key %q value %q", kv, AnalysisSIDKey, "sid-1")
	}
	if kv := FileSHA256("abc"); kv.Key != FileSHA256Key || kv.Value.AsString() != "abc" {
		t.Errorf("FileSHA256 = %v", kv)
	}
	if kv := ServiceName("icap"); kv.Key != ServiceNameKey || kv.Value.AsString() != "icap" {
		t.Errorf("ServiceName = %v", kv)
	}
	if kv := AnalysisUUID("u"); kv.Key != AnalysisUUIDKey || kv.Value.AsString() != "u" {
		t.Errorf("AnalysisUUID = %v", kv)
	}
	if kv := AnalysisMalware(true); kv.Key != AnalysisMalwareKey || kv.Value.Type() != attribute.BOOL || !kv.Value.AsBool() {
		t.Errorf("AnalysisMalware = %v, want bool true", kv)
	}
	if kv := AnalysisScore(42); kv.Key != AnalysisScoreKey || kv.Value.Type() != attribute.INT64 || kv.Value.AsInt64() != 42 {
		t.Errorf("AnalysisScore = %v, want int 42", kv)
	}
}
