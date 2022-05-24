package analysis

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/marek-kar/kube-slowwhy/pkg/collector"
	"github.com/marek-kar/kube-slowwhy/pkg/model"
)

type StorageRule struct{}

func (r *StorageRule) Name() string { return "storage-issues" }

var storageEventReasons = []string{
	"FailedAttachVolume",
	"FailedMount",
	"FailedDetach",
	"VolumeFailedRecycle",
	"VolumeFailedDelete",
	"ProvisioningFailed",
	"ExternalProvisioning",
}

var storageEventKeywords = []string{
	"AttachVolume",
	"MountVolume",
	"CSI",
	"csi",
	"volume",
	"disk",
	"FailedMount",
}

func (r *StorageRule) Evaluate(snap *collector.Snapshot) []model.Finding {
	var evidence []model.Evidence
	var pendingPVCs int

	for _, pvc := range snap.PVCs {
		if pvc.Phase == corev1.ClaimPending {
			pendingPVCs++
			evidence = append(evidence, model.Evidence{
				Type:    model.EvidenceResource,
				Ref:     fmt.Sprintf("pvc/%s/%s", pvc.Namespace, pvc.Name),
				Message: fmt.Sprintf("PVC is Pending (storageClass: %s)", storageClassOrNone(pvc.StorageClassName)),
				Data: map[string]string{
					"namespace":    pvc.Namespace,
					"storageClass": pvc.StorageClassName,
					"volumeName":   pvc.VolumeName,
				},
			})
		}
	}

	for _, pv := range snap.PVs {
		if pv.Phase == corev1.VolumeFailed {
			evidence = append(evidence, model.Evidence{
				Type:    model.EvidenceResource,
				Ref:     fmt.Sprintf("pv/%s", pv.Name),
				Message: fmt.Sprintf("PV is in Failed phase (storageClass: %s)", storageClassOrNone(pv.StorageClassName)),
				Data: map[string]string{
					"storageClass": pv.StorageClassName,
					"claimRef":     pv.ClaimRef,
				},
			})
		}
	}

	storageEvents := findStorageEvents(snap.Events)
	for _, ev := range storageEvents {
		evidence = append(evidence, model.Evidence{
			Type:    model.EvidenceEvent,
			Ref:     ev.InvolvedObject,
			Message: truncateStorage(ev.Message, maxLogLineLen),
			Data: map[string]string{
				"reason":    ev.Reason,
				"count":     fmt.Sprintf("%d", ev.Count),
				"namespace": ev.Namespace,
			},
		})
	}

	if len(evidence) == 0 {
		return nil
	}

	confidence := storageConfidence(pendingPVCs, len(storageEvents))
	severity := storageSeverity(pendingPVCs, len(storageEvents))

	return []model.Finding{
		{
			SchemaVersion: model.SchemaVersion,
			ID:            "storage-issue",
			Title:         "Storage provisioning/attachment issue",
			Category:      "storage",
			Severity:      severity,
			Confidence:    confidence,
			Summary:       storageSummary(pendingPVCs, len(storageEvents)),
			Evidence:      evidence,
			NextSteps:     storageNextSteps(),
			Timestamp:     time.Now().UTC(),
		},
	}
}

func findStorageEvents(events []collector.EventInfo) []collector.EventInfo {
	var matched []collector.EventInfo
	seen := make(map[string]bool)

	for _, ev := range events {
		key := ev.Namespace + "/" + ev.Name
		if seen[key] {
			continue
		}
		if isStorageEvent(ev) {
			seen[key] = true
			matched = append(matched, ev)
		}
	}
	return matched
}

func isStorageEvent(ev collector.EventInfo) bool {
	for _, reason := range storageEventReasons {
		if strings.EqualFold(ev.Reason, reason) {
			return true
		}
	}
	combined := ev.Reason + " " + ev.Message
	for _, kw := range storageEventKeywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

func storageClassOrNone(sc string) string {
	if sc == "" {
		return "<none>"
	}
	return sc
}

func truncateStorage(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func storageConfidence(pendingPVCs, eventCount int) float64 {
	base := 0.5
	if pendingPVCs > 0 {
		base += 0.20
	}
	if pendingPVCs > 2 {
		base += 0.10
	}
	if eventCount > 0 {
		base += 0.15
	}
	if eventCount > 3 {
		base += 0.10
	}
	if base > 1.0 {
		base = 1.0
	}
	return base
}

func storageSeverity(pendingPVCs, eventCount int) model.Severity {
	if pendingPVCs >= 3 || eventCount >= 5 {
		return model.SeverityCritical
	}
	if pendingPVCs > 0 && eventCount > 0 {
		return model.SeverityHigh
	}
	if pendingPVCs > 0 || eventCount > 2 {
		return model.SeverityMedium
	}
	return model.SeverityLow
}

func storageSummary(pendingPVCs, eventCount int) string {
	parts := []string{}
	if pendingPVCs > 0 {
		parts = append(parts, fmt.Sprintf("%d PVC(s) stuck in Pending.", pendingPVCs))
	}
	if eventCount > 0 {
		parts = append(parts, fmt.Sprintf("%d storage-related warning event(s).", eventCount))
	}
	return strings.Join(parts, " ")
}

func storageNextSteps() []string {
	return []string{
		"Check PVC events with kubectl describe pvc",
		"Verify StorageClass provisioner is running",
		"Check CSI driver pod health",
		"Review node volume attachment limits",
		"Verify cloud provider permissions for volume operations",
	}
}
