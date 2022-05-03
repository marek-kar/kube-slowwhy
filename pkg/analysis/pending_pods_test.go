package analysis

import (
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/marek-kar/kube-slowwhy/pkg/collector"
	"github.com/marek-kar/kube-slowwhy/pkg/model"
)

func TestPendingPodsRule_NoPending(t *testing.T) {
	snap := &collector.Snapshot{
		Pods: []collector.PodInfo{
			{Name: "app-1", Namespace: "default", Phase: corev1.PodRunning},
			{Name: "app-2", Namespace: "default", Phase: corev1.PodRunning},
		},
	}

	rule := &PendingPodsRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestPendingPodsRule_InsufficientCPU(t *testing.T) {
	snap := &collector.Snapshot{
		Pods: []collector.PodInfo{
			{Name: "running-1", Namespace: "default", Phase: corev1.PodRunning},
			{
				Name: "pending-1", Namespace: "default", Phase: corev1.PodPending,
				Conditions: []corev1.PodCondition{
					{
						Type:    corev1.PodScheduled,
						Status:  corev1.ConditionFalse,
						Reason:  "Unschedulable",
						Message: "0/3 nodes are available: 3 Insufficient cpu.",
					},
				},
			},
			{
				Name: "pending-2", Namespace: "default", Phase: corev1.PodPending,
				Conditions: []corev1.PodCondition{
					{
						Type:    corev1.PodScheduled,
						Status:  corev1.ConditionFalse,
						Reason:  "Unschedulable",
						Message: "0/3 nodes are available: 3 Insufficient cpu.",
					},
				},
			},
		},
		Events: []collector.EventInfo{
			{
				Namespace:      "default",
				Name:           "pending-1.scheduling",
				Reason:         "FailedScheduling",
				Message:        "0/3 nodes are available: 3 Insufficient cpu.",
				Type:           "Warning",
				InvolvedObject: "Pod/default/pending-1",
				Count:          5,
				FirstTimestamp: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				LastTimestamp:  time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC),
			},
		},
	}

	rule := &PendingPodsRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.ID != "pending-pods-insufficient-cpu" {
		t.Errorf("id: got %q, want %q", f.ID, "pending-pods-insufficient-cpu")
	}
	if f.Category != "scheduling" {
		t.Errorf("category: got %q, want %q", f.Category, "scheduling")
	}
	if f.Severity != model.SeverityHigh {
		t.Errorf("severity: got %q, want %q", f.Severity, model.SeverityHigh)
	}

	resourceEvidence := 0
	eventEvidence := 0
	for _, e := range f.Evidence {
		switch e.Type {
		case model.EvidenceResource:
			resourceEvidence++
		case model.EvidenceEvent:
			eventEvidence++
		}
	}
	if resourceEvidence != 2 {
		t.Errorf("resource evidence: got %d, want 2", resourceEvidence)
	}
	if eventEvidence < 1 {
		t.Errorf("event evidence: got %d, want >= 1", eventEvidence)
	}
}

func TestPendingPodsRule_TaintIssues(t *testing.T) {
	snap := &collector.Snapshot{
		Pods: []collector.PodInfo{
			{
				Name: "taint-pod", Namespace: "prod", Phase: corev1.PodPending,
				Conditions: []corev1.PodCondition{
					{
						Type:    corev1.PodScheduled,
						Status:  corev1.ConditionFalse,
						Reason:  "Unschedulable",
						Message: "0/5 nodes are available: 5 node(s) had untolerated taint {gpu=true: NoSchedule}.",
					},
				},
			},
		},
	}

	rule := &PendingPodsRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.ID != "pending-pods-taint" {
		t.Errorf("id: got %q, want %q", f.ID, "pending-pods-taint")
	}
	if f.Severity != model.SeverityMedium {
		t.Errorf("severity: got %q, want %q", f.Severity, model.SeverityMedium)
	}
}

func TestPendingPodsRule_AffinityIssues(t *testing.T) {
	snap := &collector.Snapshot{
		Pods: []collector.PodInfo{
			{
				Name: "affinity-pod", Namespace: "default", Phase: corev1.PodPending,
				Conditions: []corev1.PodCondition{
					{
						Type:    corev1.PodScheduled,
						Status:  corev1.ConditionFalse,
						Reason:  "Unschedulable",
						Message: "0/3 nodes are available: 3 node(s) didn't match Pod's node affinity/selector.",
					},
				},
			},
		},
	}

	rule := &PendingPodsRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	if findings[0].ID != "pending-pods-affinity" {
		t.Errorf("id: got %q, want %q", findings[0].ID, "pending-pods-affinity")
	}
}

func TestPendingPodsRule_MixedReasons(t *testing.T) {
	snap := &collector.Snapshot{
		Pods: []collector.PodInfo{
			{
				Name: "cpu-pod", Namespace: "default", Phase: corev1.PodPending,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Message: "Insufficient cpu"},
				},
			},
			{
				Name: "mem-pod", Namespace: "default", Phase: corev1.PodPending,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Message: "Insufficient memory"},
				},
			},
			{Name: "running", Namespace: "default", Phase: corev1.PodRunning},
		},
	}

	rule := &PendingPodsRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (cpu + memory), got %d", len(findings))
	}

	ids := make(map[string]bool)
	for _, f := range findings {
		ids[f.ID] = true
	}
	if !ids["pending-pods-insufficient-cpu"] {
		t.Error("missing finding for insufficient-cpu")
	}
	if !ids["pending-pods-insufficient-memory"] {
		t.Error("missing finding for insufficient-memory")
	}
}

func TestPendingPodsRule_HighCountCritical(t *testing.T) {
	pods := make([]collector.PodInfo, 6)
	for i := range pods {
		pods[i] = collector.PodInfo{
			Name: fmt.Sprintf("pending-%d", i), Namespace: "default", Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Message: "Insufficient memory"},
			},
		}
	}

	snap := &collector.Snapshot{Pods: pods}
	rule := &PendingPodsRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != model.SeverityCritical {
		t.Errorf("severity: got %q, want %q", findings[0].Severity, model.SeverityCritical)
	}
}

func TestPendingPodsRule_Name(t *testing.T) {
	rule := &PendingPodsRule{}
	if rule.Name() != "pending-pods" {
		t.Errorf("name: got %q, want %q", rule.Name(), "pending-pods")
	}
}

var _ Rule = (*PendingPodsRule)(nil)

func init() {
	_ = metav1.Now()
}
