package analysis

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/marek-kar/kube-slowwhy/pkg/collector"
	"github.com/marek-kar/kube-slowwhy/pkg/model"
)

type NodePressureRule struct{}

func (r *NodePressureRule) Name() string { return "node-pressure" }

var pressureConditions = []corev1.NodeConditionType{
	corev1.NodeDiskPressure,
	corev1.NodeMemoryPressure,
	corev1.NodePIDPressure,
}

var evictionReasons = []string{
	"Evicted",
	"eviction",
	"NodeHasDiskPressure",
	"NodeHasMemoryPressure",
	"NodeHasPIDPressure",
	"OOMKilling",
	"SystemOOM",
}

func (r *NodePressureRule) Evaluate(snap *collector.Snapshot) []model.Finding {
	var findings []model.Finding

	for _, node := range snap.Nodes {
		for _, cond := range node.Conditions {
			if !isPressureCondition(cond.Type) || cond.Status != corev1.ConditionTrue {
				continue
			}

			evidence := []model.Evidence{
				{
					Type:    model.EvidenceResource,
					Ref:     fmt.Sprintf("node/%s", node.Name),
					Message: fmt.Sprintf("Condition %s is True: %s", cond.Type, cond.Message),
					Data: map[string]string{
						"condition": string(cond.Type),
						"status":    string(cond.Status),
						"reason":    cond.Reason,
					},
				},
			}

			relatedEvents := findEvictionEvents(snap.Events, node.Name)
			for _, ev := range relatedEvents {
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

			confidence := pressureConfidence(cond, len(relatedEvents))
			severity := pressureSeverity(cond.Type, len(relatedEvents))

			findings = append(findings, model.Finding{
				SchemaVersion: model.SchemaVersion,
				ID:            fmt.Sprintf("node-pressure-%s-%s", node.Name, conditionSlug(cond.Type)),
				Title:         fmt.Sprintf("Node %s has %s", node.Name, cond.Type),
				Category:      "node-health",
				Severity:      severity,
				Confidence:    confidence,
				Summary:       buildPressureSummary(node.Name, cond, len(relatedEvents)),
				Evidence:      evidence,
				NextSteps:     pressureNextSteps(cond.Type),
				Timestamp:     time.Now().UTC(),
			})
		}
	}

	return findings
}

func isPressureCondition(ct corev1.NodeConditionType) bool {
	for _, p := range pressureConditions {
		if ct == p {
			return true
		}
	}
	return false
}

func findEvictionEvents(events []collector.EventInfo, nodeName string) []collector.EventInfo {
	var matched []collector.EventInfo
	for _, ev := range events {
		if !isEvictionRelated(ev.Reason, ev.Message) {
			continue
		}
		if strings.Contains(ev.InvolvedObject, nodeName) || strings.Contains(ev.Message, nodeName) {
			matched = append(matched, ev)
		}
	}
	return matched
}

func isEvictionRelated(reason, message string) bool {
	combined := reason + " " + message
	for _, keyword := range evictionReasons {
		if strings.Contains(strings.ToLower(combined), strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func pressureConfidence(cond corev1.NodeCondition, evictionEventCount int) float64 {
	base := 0.7
	if evictionEventCount > 0 {
		base += 0.15
	}
	if evictionEventCount > 3 {
		base += 0.10
	}
	if base > 1.0 {
		base = 1.0
	}
	return base
}

func pressureSeverity(ct corev1.NodeConditionType, evictionEventCount int) model.Severity {
	if evictionEventCount > 0 {
		return model.SeverityCritical
	}
	switch ct {
	case corev1.NodeMemoryPressure, corev1.NodeDiskPressure:
		return model.SeverityHigh
	default:
		return model.SeverityMedium
	}
}

func buildPressureSummary(nodeName string, cond corev1.NodeCondition, evCount int) string {
	s := fmt.Sprintf("Node %s reports %s (reason: %s).", nodeName, cond.Type, cond.Reason)
	if evCount > 0 {
		s += fmt.Sprintf(" %d eviction-related event(s) correlated.", evCount)
	}
	return s
}

func pressureNextSteps(ct corev1.NodeConditionType) []string {
	switch ct {
	case corev1.NodeDiskPressure:
		return []string{
			"Check disk usage on the node",
			"Review pod ephemeral storage usage",
			"Consider expanding node disk or cleaning images",
		}
	case corev1.NodeMemoryPressure:
		return []string{
			"Review pod memory requests and limits",
			"Check for memory leaks in workloads",
			"Consider adding nodes or increasing node memory",
		}
	case corev1.NodePIDPressure:
		return []string{
			"Check for fork bombs or runaway processes",
			"Review pod PID limits",
			"Inspect node process table",
		}
	default:
		return []string{"Investigate node conditions"}
	}
}

func conditionSlug(ct corev1.NodeConditionType) string {
	return strings.ToLower(strings.ReplaceAll(string(ct), " ", "-"))
}
