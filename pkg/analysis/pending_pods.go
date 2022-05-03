package analysis

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/marek-kar/kube-slowwhy/pkg/collector"
	"github.com/marek-kar/kube-slowwhy/pkg/model"
)

type PendingPodsRule struct{}

func (r *PendingPodsRule) Name() string { return "pending-pods" }

type schedulingReason struct {
	Category string
	Keywords []string
}

var schedulingReasons = []schedulingReason{
	{Category: "insufficient-cpu", Keywords: []string{"Insufficient cpu", "cpu"}},
	{Category: "insufficient-memory", Keywords: []string{"Insufficient memory", "memory"}},
	{Category: "taint", Keywords: []string{"had taint", "untolerated taint", "NoSchedule", "NoExecute"}},
	{Category: "affinity", Keywords: []string{"node affinity", "node(s) didn't match", "affinity", "anti-affinity"}},
	{Category: "unschedulable", Keywords: []string{"unschedulable", "SchedulingDisabled"}},
}

func (r *PendingPodsRule) Evaluate(snap *collector.Snapshot) []model.Finding {
	type podReason struct {
		pod    collector.PodInfo
		reason string
		events []collector.EventInfo
	}

	buckets := make(map[string][]podReason)

	for _, pod := range snap.Pods {
		if pod.Phase != corev1.PodPending {
			continue
		}

		events := findPodEvents(snap.Events, pod.Namespace, pod.Name)
		cat := classifySchedulingReason(pod, events)

		buckets[cat] = append(buckets[cat], podReason{
			pod:    pod,
			reason: cat,
			events: events,
		})
	}

	if len(buckets) == 0 {
		return nil
	}

	var findings []model.Finding

	for cat, pods := range buckets {
		evidence := make([]model.Evidence, 0, len(pods)*2)

		for _, pr := range pods {
			evidence = append(evidence, model.Evidence{
				Type:    model.EvidenceResource,
				Ref:     fmt.Sprintf("pod/%s/%s", pr.pod.Namespace, pr.pod.Name),
				Message: fmt.Sprintf("Pod is Pending (reason: %s)", cat),
				Data: map[string]string{
					"namespace": pr.pod.Namespace,
					"nodeName":  pr.pod.NodeName,
				},
			})
			for _, ev := range pr.events {
				evidence = append(evidence, model.Evidence{
					Type:    model.EvidenceEvent,
					Ref:     ev.InvolvedObject,
					Message: ev.Message,
					Data: map[string]string{
						"reason": ev.Reason,
						"count":  fmt.Sprintf("%d", ev.Count),
					},
				})
			}
		}

		confidence := pendingConfidence(len(pods), len(snap.Pods))
		severity := pendingSeverity(len(pods), cat)

		findings = append(findings, model.Finding{
			SchemaVersion: model.SchemaVersion,
			ID:            fmt.Sprintf("pending-pods-%s", cat),
			Title:         fmt.Sprintf("%d Pending pod(s) due to %s", len(pods), cat),
			Category:      "scheduling",
			Severity:      severity,
			Confidence:    confidence,
			Summary:       buildPendingSummary(len(pods), cat),
			Evidence:      evidence,
			NextSteps:     pendingNextSteps(cat),
			Timestamp:     time.Now().UTC(),
		})
	}

	return findings
}

func findPodEvents(events []collector.EventInfo, namespace, name string) []collector.EventInfo {
	ref := fmt.Sprintf("Pod/%s/%s", namespace, name)
	var matched []collector.EventInfo
	for _, ev := range events {
		if ev.InvolvedObject == ref || (ev.Namespace == namespace && strings.Contains(ev.Name, name)) {
			matched = append(matched, ev)
		}
	}
	return matched
}

func classifySchedulingReason(pod collector.PodInfo, events []collector.EventInfo) string {
	messages := collectMessages(pod, events)

	for _, sr := range schedulingReasons {
		for _, kw := range sr.Keywords {
			for _, msg := range messages {
				if strings.Contains(strings.ToLower(msg), strings.ToLower(kw)) {
					return sr.Category
				}
			}
		}
	}

	return "unknown"
}

func collectMessages(pod collector.PodInfo, events []collector.EventInfo) []string {
	var msgs []string
	for _, c := range pod.Conditions {
		if c.Message != "" {
			msgs = append(msgs, c.Message)
		}
		if c.Reason != "" {
			msgs = append(msgs, c.Reason)
		}
	}
	for _, ev := range events {
		msgs = append(msgs, ev.Message, ev.Reason)
	}
	return msgs
}

func pendingConfidence(pendingCount, totalPods int) float64 {
	if totalPods == 0 {
		return 0.5
	}
	ratio := float64(pendingCount) / float64(totalPods)
	base := 0.6
	if pendingCount >= 3 {
		base += 0.15
	}
	if ratio > 0.1 {
		base += 0.15
	}
	if base > 1.0 {
		base = 1.0
	}
	return base
}

func pendingSeverity(count int, category string) model.Severity {
	if count >= 5 {
		return model.SeverityCritical
	}
	switch category {
	case "insufficient-cpu", "insufficient-memory":
		if count >= 2 {
			return model.SeverityHigh
		}
		return model.SeverityMedium
	case "taint", "affinity":
		return model.SeverityMedium
	default:
		return model.SeverityLow
	}
}

func buildPendingSummary(count int, category string) string {
	return fmt.Sprintf("%d pod(s) stuck in Pending state. Most common scheduling failure: %s.", count, category)
}

func pendingNextSteps(category string) []string {
	switch category {
	case "insufficient-cpu":
		return []string{
			"Review CPU requests across the cluster",
			"Consider adding nodes or increasing node size",
			"Check for pods with excessive CPU requests",
		}
	case "insufficient-memory":
		return []string{
			"Review memory requests across the cluster",
			"Consider adding nodes or increasing node memory",
			"Check for pods with excessive memory requests",
		}
	case "taint":
		return []string{
			"Review node taints and pod tolerations",
			"Check if taints were recently added",
			"Verify pod tolerations match node taints",
		}
	case "affinity":
		return []string{
			"Review pod affinity and anti-affinity rules",
			"Check node labels match pod nodeSelector",
			"Consider relaxing affinity constraints",
		}
	case "unschedulable":
		return []string{
			"Check if nodes are cordoned",
			"Uncordon nodes if maintenance is complete",
		}
	default:
		return []string{
			"Inspect pod events with kubectl describe",
			"Check scheduler logs for details",
		}
	}
}
