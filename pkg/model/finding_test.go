package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFindingJSONRoundTrip(t *testing.T) {
	ts := time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC)
	f := Finding{
		SchemaVersion: SchemaVersion,
		ID:            "slow-001",
		Title:         "High Pod Restart Count",
		Category:      "pod-health",
		Severity:      SeverityHigh,
		Confidence:    0.95,
		Summary:       "Pod nginx-abc has restarted 12 times in the last hour",
		Evidence: []Evidence{
			{
				Type:    EvidenceEvent,
				Ref:     "v1/Event/default/nginx-abc.restart",
				Message: "Back-off restarting failed container",
				Data:    map[string]string{"restartCount": "12"},
			},
		},
		NextSteps: []string{"Check container logs", "Review resource limits"},
		Timestamp: ts,
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Finding
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != f.ID {
		t.Errorf("ID mismatch: got %q, want %q", decoded.ID, f.ID)
	}
	if decoded.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion mismatch: got %q, want %q", decoded.SchemaVersion, SchemaVersion)
	}
	if decoded.Severity != SeverityHigh {
		t.Errorf("Severity mismatch: got %q, want %q", decoded.Severity, SeverityHigh)
	}
	if decoded.Confidence != 0.95 {
		t.Errorf("Confidence mismatch: got %f, want %f", decoded.Confidence, 0.95)
	}
	if len(decoded.Evidence) != 1 {
		t.Fatalf("Evidence count: got %d, want 1", len(decoded.Evidence))
	}
	if decoded.Evidence[0].Type != EvidenceEvent {
		t.Errorf("Evidence type: got %q, want %q", decoded.Evidence[0].Type, EvidenceEvent)
	}
	if !decoded.Timestamp.Equal(ts) {
		t.Errorf("Timestamp mismatch: got %v, want %v", decoded.Timestamp, ts)
	}
}

func TestNewReport(t *testing.T) {
	r := NewReport(nil)
	if r.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion: got %q, want %q", r.SchemaVersion, SchemaVersion)
	}
}
