package analysis

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marek-kar/kube-slowwhy/pkg/collector"
	"github.com/marek-kar/kube-slowwhy/pkg/model"
)

func TestNodePressureRule_NoFindings(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []collector.NodeInfo{
			{
				Name: "node-1",
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
					{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
				},
			},
		},
	}

	rule := &NodePressureRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestNodePressureRule_DiskPressure(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []collector.NodeInfo{
			{
				Name: "worker-1",
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					{
						Type:    corev1.NodeDiskPressure,
						Status:  corev1.ConditionTrue,
						Reason:  "KubeletHasDiskPressure",
						Message: "kubelet has disk pressure",
					},
				},
			},
		},
	}

	rule := &NodePressureRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.Severity != model.SeverityHigh {
		t.Errorf("severity: got %q, want %q", f.Severity, model.SeverityHigh)
	}
	if f.Category != "node-health" {
		t.Errorf("category: got %q, want %q", f.Category, "node-health")
	}
	if f.ID != "node-pressure-worker-1-diskpressure" {
		t.Errorf("id: got %q, want %q", f.ID, "node-pressure-worker-1-diskpressure")
	}
	if len(f.Evidence) < 1 {
		t.Fatal("expected at least 1 evidence item")
	}
	if f.Evidence[0].Ref != "node/worker-1" {
		t.Errorf("evidence ref: got %q, want %q", f.Evidence[0].Ref, "node/worker-1")
	}
}

func TestNodePressureRule_MemoryPressureWithEvictions(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []collector.NodeInfo{
			{
				Name: "worker-2",
				Conditions: []corev1.NodeCondition{
					{
						Type:    corev1.NodeMemoryPressure,
						Status:  corev1.ConditionTrue,
						Reason:  "KubeletHasInsufficientMemory",
						Message: "kubelet has insufficient memory available",
					},
				},
			},
		},
		Events: []collector.EventInfo{
			{
				Namespace:      "default",
				Name:           "app-pod.eviction",
				Reason:         "Evicted",
				Message:        "The node worker-2 was low on resource: memory.",
				Type:           "Warning",
				InvolvedObject: "Pod/default/app-pod",
				Count:          3,
				FirstTimestamp: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				LastTimestamp:  time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC),
			},
			{
				Namespace:      "monitoring",
				Name:           "prom.oom",
				Reason:         "OOMKilling",
				Message:        "Memory cgroup out of memory on worker-2",
				Type:           "Warning",
				InvolvedObject: "Pod/monitoring/prometheus-0",
				Count:          1,
				FirstTimestamp: time.Date(2024, 1, 1, 10, 15, 0, 0, time.UTC),
				LastTimestamp:  time.Date(2024, 1, 1, 10, 15, 0, 0, time.UTC),
			},
			{
				Namespace:      "default",
				Name:           "unrelated.event",
				Reason:         "Scheduled",
				Message:        "Successfully assigned default/other-pod to worker-3",
				Type:           "Normal",
				InvolvedObject: "Pod/default/other-pod",
				Count:          1,
				FirstTimestamp: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				LastTimestamp:  time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			},
		},
	}

	rule := &NodePressureRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.Severity != model.SeverityCritical {
		t.Errorf("severity: got %q, want %q", f.Severity, model.SeverityCritical)
	}
	if f.Confidence < 0.85 {
		t.Errorf("confidence too low: got %f, want >= 0.85", f.Confidence)
	}

	if len(f.Evidence) != 3 {
		t.Fatalf("evidence count: got %d, want 3 (1 condition + 2 events)", len(f.Evidence))
	}

	eventEvidence := 0
	for _, e := range f.Evidence {
		if e.Type == model.EvidenceEvent {
			eventEvidence++
		}
	}
	if eventEvidence != 2 {
		t.Errorf("event evidence count: got %d, want 2", eventEvidence)
	}
}

func TestNodePressureRule_MultiplePressures(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []collector.NodeInfo{
			{
				Name: "worker-3",
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeDiskPressure, Status: corev1.ConditionTrue, Reason: "DiskPressure", Message: "disk pressure"},
					{Type: corev1.NodePIDPressure, Status: corev1.ConditionTrue, Reason: "PIDPressure", Message: "pid pressure"},
				},
			},
		},
	}

	rule := &NodePressureRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
}

func TestNodePressureRule_Name(t *testing.T) {
	rule := &NodePressureRule{}
	if rule.Name() != "node-pressure" {
		t.Errorf("name: got %q, want %q", rule.Name(), "node-pressure")
	}
}

var _ Rule = (*NodePressureRule)(nil)

func init() {
	_ = metav1.Now()
}
