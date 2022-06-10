package analysis

import (
	"testing"
	"time"

	"github.com/marek-kar/kube-slowwhy/pkg/model"
)

var testTS = time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)

func TestCorrelator_EmptyInput(t *testing.T) {
	c := NewCorrelator()
	result := c.Correlate(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result))
	}
}

func TestCorrelator_SingleFinding(t *testing.T) {
	c := NewCorrelator()
	input := []model.Finding{
		{
			ID: "test-1", Category: "node-health", Severity: model.SeverityHigh, Confidence: 0.7,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "node/worker-1", Message: "DiskPressure"},
				{Type: model.EvidenceEvent, Ref: "node/worker-1", Message: "eviction event"},
			},
			Timestamp: testTS,
		},
	}

	result := c.Correlate(input)
	if len(result) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result))
	}
	if result[0].Confidence <= 0.7 {
		t.Errorf("confidence should be boosted from 2 evidence types, got %f", result[0].Confidence)
	}
	if result[0].Reasoning == "" {
		t.Error("reasoning should be set")
	}
}

func TestCorrelator_MergesDuplicates(t *testing.T) {
	c := NewCorrelator()
	input := []model.Finding{
		{
			ID: "pressure-1", Category: "node-health", Severity: model.SeverityHigh, Confidence: 0.7,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "node/worker-1", Message: "DiskPressure"},
			},
			NextSteps: []string{"Check disk usage"},
			Timestamp: testTS,
		},
		{
			ID: "pressure-2", Category: "node-health", Severity: model.SeverityCritical, Confidence: 0.85,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "node/worker-1", Message: "DiskPressure"},
				{Type: model.EvidenceEvent, Ref: "node/worker-1", Message: "eviction happened"},
			},
			NextSteps: []string{"Check disk usage", "Clean images"},
			Timestamp: testTS,
		},
	}

	result := c.Correlate(input)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged finding, got %d", len(result))
	}

	f := result[0]
	if f.Severity != model.SeverityCritical {
		t.Errorf("severity: got %q, want %q (should take highest)", f.Severity, model.SeverityCritical)
	}
	if f.Confidence < 0.85 {
		t.Errorf("confidence should be at least 0.85 (max of inputs), got %f", f.Confidence)
	}
	if len(f.Evidence) != 2 {
		t.Errorf("evidence: got %d, want 2 (deduplicated)", len(f.Evidence))
	}
	if len(f.NextSteps) != 2 {
		t.Errorf("next steps: got %d, want 2 (deduplicated)", len(f.NextSteps))
	}
}

func TestCorrelator_DifferentCategoriesNotMerged(t *testing.T) {
	c := NewCorrelator()
	input := []model.Finding{
		{
			ID: "dns-1", Category: "dns", Severity: model.SeverityHigh, Confidence: 0.8,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "pod/kube-system/coredns-abc", Message: "crashloop"},
			},
			Timestamp: testTS,
		},
		{
			ID: "storage-1", Category: "storage", Severity: model.SeverityMedium, Confidence: 0.6,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "pvc/default/data", Message: "pending"},
			},
			Timestamp: testTS,
		},
	}

	result := c.Correlate(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 findings (different categories), got %d", len(result))
	}
}

func TestCorrelator_SortBySeverityThenConfidence(t *testing.T) {
	c := NewCorrelator()
	input := []model.Finding{
		{
			ID: "low-1", Category: "a", Severity: model.SeverityLow, Confidence: 0.9,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "a/1", Message: "m"},
			},
			Timestamp: testTS,
		},
		{
			ID: "critical-1", Category: "b", Severity: model.SeverityCritical, Confidence: 0.7,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "b/1", Message: "m"},
			},
			Timestamp: testTS,
		},
		{
			ID: "high-1", Category: "c", Severity: model.SeverityHigh, Confidence: 0.6,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "c/1", Message: "m"},
			},
			Timestamp: testTS,
		},
		{
			ID: "high-2", Category: "d", Severity: model.SeverityHigh, Confidence: 0.9,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "d/1", Message: "m"},
			},
			Timestamp: testTS,
		},
	}

	result := c.Correlate(input)
	if len(result) != 4 {
		t.Fatalf("expected 4 findings, got %d", len(result))
	}

	expected := []struct {
		id       string
		severity model.Severity
	}{
		{"critical-1", model.SeverityCritical},
		{"high-2", model.SeverityHigh},
		{"high-1", model.SeverityHigh},
		{"low-1", model.SeverityLow},
	}

	for i, want := range expected {
		if result[i].ID != want.id {
			t.Errorf("position %d: got ID %q, want %q", i, result[i].ID, want.id)
		}
		if result[i].Severity != want.severity {
			t.Errorf("position %d: got severity %q, want %q", i, result[i].Severity, want.severity)
		}
	}
}

func TestCorrelator_DeterministicTiebreaker(t *testing.T) {
	c := NewCorrelator()
	input := []model.Finding{
		{
			ID: "zzz", Category: "x", Severity: model.SeverityMedium, Confidence: 0.5,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "x/1", Message: "m"},
			},
			Timestamp: testTS,
		},
		{
			ID: "aaa", Category: "y", Severity: model.SeverityMedium, Confidence: 0.5,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "y/1", Message: "m"},
			},
			Timestamp: testTS,
		},
	}

	for i := 0; i < 20; i++ {
		result := c.Correlate(input)
		if result[0].ID != "aaa" || result[1].ID != "zzz" {
			t.Fatalf("run %d: non-deterministic order: %q, %q", i, result[0].ID, result[1].ID)
		}
	}
}

func TestCorrelator_BoostMultipleEvidenceTypes(t *testing.T) {
	c := NewCorrelator()
	input := []model.Finding{
		{
			ID: "multi", Category: "test", Severity: model.SeverityMedium, Confidence: 0.5,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "node/a", Message: "condition"},
				{Type: model.EvidenceEvent, Ref: "node/a", Message: "event"},
				{Type: model.EvidenceLog, Ref: "node/a", Message: "log line"},
			},
			Timestamp: testTS,
		},
	}

	result := c.Correlate(input)
	if result[0].Confidence < 0.65 {
		t.Errorf("3 distinct types should boost to >= 0.65, got %f", result[0].Confidence)
	}
	if result[0].Reasoning == "" {
		t.Error("reasoning should be set")
	}
	if result[0].Confidence > 1.0 {
		t.Errorf("confidence should not exceed 1.0, got %f", result[0].Confidence)
	}
}

func TestCorrelator_ReasoningContainsSourceCounts(t *testing.T) {
	c := NewCorrelator()
	input := []model.Finding{
		{
			ID: "r-1", Category: "test", Severity: model.SeverityLow, Confidence: 0.6,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "a/1", Message: "r1"},
				{Type: model.EvidenceResource, Ref: "a/2", Message: "r2"},
				{Type: model.EvidenceEvent, Ref: "a/1", Message: "e1"},
			},
			Timestamp: testTS,
		},
	}

	result := c.Correlate(input)
	r := result[0].Reasoning

	mustContain := []string{"confidence=", "resource source(s)", "event source(s)", "cross-signal agreement"}
	for _, s := range mustContain {
		if !contains(r, s) {
			t.Errorf("reasoning missing %q, got: %q", s, r)
		}
	}
}

func TestCorrelator_ConfidenceCappedAt1(t *testing.T) {
	c := NewCorrelator()
	input := []model.Finding{
		{
			ID: "max", Category: "test", Severity: model.SeverityHigh, Confidence: 0.95,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "a/1", Message: "m1"},
				{Type: model.EvidenceEvent, Ref: "a/1", Message: "m2"},
				{Type: model.EvidenceLog, Ref: "a/1", Message: "m3"},
				{Type: model.EvidenceMetric, Ref: "a/1", Message: "m4"},
				{Type: model.EvidenceResource, Ref: "a/2", Message: "m5"},
				{Type: model.EvidenceEvent, Ref: "a/3", Message: "m6"},
			},
			Timestamp: testTS,
		},
	}

	result := c.Correlate(input)
	if result[0].Confidence > 1.0 {
		t.Errorf("confidence should be capped at 1.0, got %f", result[0].Confidence)
	}
	if result[0].Confidence != 1.0 {
		t.Errorf("confidence should be 1.0 with 4 types + 6 items + 0.95 base, got %f", result[0].Confidence)
	}
}

func TestCorrelator_MergePreservesID(t *testing.T) {
	c := NewCorrelator()
	input := []model.Finding{
		{
			ID: "first-id", Category: "cat", Severity: model.SeverityLow, Confidence: 0.5,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "r/1", Message: "m"},
			},
			Timestamp: testTS,
		},
		{
			ID: "second-id", Category: "cat", Severity: model.SeverityMedium, Confidence: 0.6,
			Evidence: []model.Evidence{
				{Type: model.EvidenceResource, Ref: "r/1", Message: "m"},
				{Type: model.EvidenceEvent, Ref: "r/1", Message: "e"},
			},
			Timestamp: testTS,
		},
	}

	result := c.Correlate(input)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged finding, got %d", len(result))
	}
	if result[0].ID != "first-id" {
		t.Errorf("merged finding should keep first ID, got %q", result[0].ID)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && containsStr(haystack, needle)
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
