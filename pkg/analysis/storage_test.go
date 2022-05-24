package analysis

import (
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/marek-kar/kube-slowwhy/pkg/collector"
	"github.com/marek-kar/kube-slowwhy/pkg/model"
)

func TestStorageRule_NoIssues(t *testing.T) {
	snap := &collector.Snapshot{
		PVCs: []collector.PVCInfo{
			{Name: "data-pvc", Namespace: "default", Phase: corev1.ClaimBound, VolumeName: "pv-001"},
		},
		PVs: []collector.PVInfo{
			{Name: "pv-001", Phase: corev1.VolumeBound},
		},
	}

	rule := &StorageRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestStorageRule_PendingPVC(t *testing.T) {
	snap := &collector.Snapshot{
		PVCs: []collector.PVCInfo{
			{
				Name:             "stuck-pvc",
				Namespace:        "default",
				Phase:            corev1.ClaimPending,
				StorageClassName: "gp3",
			},
		},
	}

	rule := &StorageRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.ID != "storage-issue" {
		t.Errorf("id: got %q, want %q", f.ID, "storage-issue")
	}
	if f.Category != "storage" {
		t.Errorf("category: got %q, want %q", f.Category, "storage")
	}
	if f.Severity != model.SeverityMedium {
		t.Errorf("severity: got %q, want %q", f.Severity, model.SeverityMedium)
	}

	if len(f.Evidence) < 1 {
		t.Fatal("expected at least 1 evidence item")
	}
	if f.Evidence[0].Ref != "pvc/default/stuck-pvc" {
		t.Errorf("evidence ref: got %q, want %q", f.Evidence[0].Ref, "pvc/default/stuck-pvc")
	}
}

func TestStorageRule_PendingPVCWithEvents(t *testing.T) {
	snap := &collector.Snapshot{
		PVCs: []collector.PVCInfo{
			{
				Name:             "db-pvc",
				Namespace:        "prod",
				Phase:            corev1.ClaimPending,
				StorageClassName: "ebs-sc",
			},
		},
		Events: []collector.EventInfo{
			{
				Namespace:      "prod",
				Name:           "db-pvc.provisioning",
				Reason:         "ProvisioningFailed",
				Message:        "Failed to provision volume with StorageClass ebs-sc: insufficient capacity",
				Type:           "Warning",
				InvolvedObject: "PersistentVolumeClaim/prod/db-pvc",
				Count:          5,
				FirstTimestamp: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				LastTimestamp:  time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC),
			},
		},
	}

	rule := &StorageRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.Severity != model.SeverityHigh {
		t.Errorf("severity: got %q, want %q (pending + events = high)", f.Severity, model.SeverityHigh)
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
	if resourceEvidence < 1 {
		t.Errorf("resource evidence: got %d, want >= 1", resourceEvidence)
	}
	if eventEvidence < 1 {
		t.Errorf("event evidence: got %d, want >= 1", eventEvidence)
	}
}

func TestStorageRule_FailedPV(t *testing.T) {
	snap := &collector.Snapshot{
		PVs: []collector.PVInfo{
			{
				Name:             "pv-broken",
				Phase:            corev1.VolumeFailed,
				StorageClassName: "local-storage",
				ClaimRef:         "default/my-claim",
			},
		},
	}

	rule := &StorageRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	hasFailedPV := false
	for _, e := range findings[0].Evidence {
		if e.Ref == "pv/pv-broken" {
			hasFailedPV = true
		}
	}
	if !hasFailedPV {
		t.Error("expected evidence for failed PV")
	}
}

func TestStorageRule_CSIEvents(t *testing.T) {
	snap := &collector.Snapshot{
		Events: []collector.EventInfo{
			{
				Namespace:      "default",
				Name:           "app-pod.mount",
				Reason:         "FailedMount",
				Message:        "MountVolume.SetUp failed for volume pvc-123: CSI driver timeout",
				Type:           "Warning",
				InvolvedObject: "Pod/default/app-pod",
				Count:          3,
				FirstTimestamp: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				LastTimestamp:  time.Date(2024, 1, 1, 10, 15, 0, 0, time.UTC),
			},
			{
				Namespace:      "default",
				Name:           "app-pod.attach",
				Reason:         "FailedAttachVolume",
				Message:        "AttachVolume.Attach failed: volume not found",
				Type:           "Warning",
				InvolvedObject: "Pod/default/app-pod",
				Count:          2,
				FirstTimestamp: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				LastTimestamp:  time.Date(2024, 1, 1, 10, 10, 0, 0, time.UTC),
			},
		},
	}

	rule := &StorageRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	if len(findings[0].Evidence) < 2 {
		t.Errorf("expected at least 2 event evidence items, got %d", len(findings[0].Evidence))
	}
}

func TestStorageRule_ManyPendingCritical(t *testing.T) {
	pvcs := make([]collector.PVCInfo, 4)
	for i := range pvcs {
		pvcs[i] = collector.PVCInfo{
			Name:             fmt.Sprintf("pvc-%d", i),
			Namespace:        "default",
			Phase:            corev1.ClaimPending,
			StorageClassName: "gp3",
		}
	}

	snap := &collector.Snapshot{PVCs: pvcs}
	rule := &StorageRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != model.SeverityCritical {
		t.Errorf("severity: got %q, want %q", findings[0].Severity, model.SeverityCritical)
	}
}

func TestStorageRule_OnlyEventsNoStorage(t *testing.T) {
	snap := &collector.Snapshot{
		Events: []collector.EventInfo{
			{
				Namespace:      "default",
				Name:           "pod.attach",
				Reason:         "FailedAttachVolume",
				Message:        "AttachVolume.Attach failed for volume pvc-abc",
				Type:           "Warning",
				InvolvedObject: "Pod/default/my-pod",
				Count:          1,
			},
		},
	}

	rule := &StorageRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding from events alone, got %d", len(findings))
	}
	if findings[0].Severity != model.SeverityLow {
		t.Errorf("severity: got %q, want %q", findings[0].Severity, model.SeverityLow)
	}
}

func TestStorageRule_Name(t *testing.T) {
	rule := &StorageRule{}
	if rule.Name() != "storage-issues" {
		t.Errorf("name: got %q, want %q", rule.Name(), "storage-issues")
	}
}

var _ Rule = (*StorageRule)(nil)
