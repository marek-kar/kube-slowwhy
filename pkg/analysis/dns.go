package analysis

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/marek-kar/kube-slowwhy/pkg/collector"
	"github.com/marek-kar/kube-slowwhy/pkg/model"
)

type DNSRule struct{}

func (r *DNSRule) Name() string { return "dns-instability" }

const (
	maxLogLineLen    = 256
	restartThreshold = 3
)

var coreDNSLabels = []string{
	"coredns",
	"kube-dns",
}

var dnsLogPatterns = []string{
	"SERVFAIL",
	"timeout",
	"i/o timeout",
	"connection refused",
	"no such host",
	"NXDOMAIN",
}

var dnsEventReasons = []string{
	"BackOff",
	"CrashLoopBackOff",
	"Unhealthy",
	"Failed",
	"OOMKilled",
}

func (r *DNSRule) Evaluate(snap *collector.Snapshot) []model.Finding {
	dnsPods := findCoreDNSPods(snap)
	if len(dnsPods) == 0 {
		return nil
	}

	var evidence []model.Evidence
	var crashlooping, highRestarts int

	for _, pod := range dnsPods {
		for _, c := range pod.Containers {
			if c.State.Waiting != nil && c.State.Waiting.Reason == "CrashLoopBackOff" {
				crashlooping++
				evidence = append(evidence, model.Evidence{
					Type:    model.EvidenceResource,
					Ref:     fmt.Sprintf("pod/%s/%s", pod.Namespace, pod.Name),
					Message: fmt.Sprintf("Container %s is in CrashLoopBackOff", c.Name),
					Data: map[string]string{
						"container":    c.Name,
						"restartCount": fmt.Sprintf("%d", c.RestartCount),
					},
				})
			} else if c.RestartCount >= restartThreshold {
				highRestarts++
				evidence = append(evidence, model.Evidence{
					Type:    model.EvidenceResource,
					Ref:     fmt.Sprintf("pod/%s/%s", pod.Namespace, pod.Name),
					Message: fmt.Sprintf("Container %s has %d restarts", c.Name, c.RestartCount),
					Data: map[string]string{
						"container":    c.Name,
						"restartCount": fmt.Sprintf("%d", c.RestartCount),
					},
				})
			}
		}

		if pod.Phase != corev1.PodRunning {
			evidence = append(evidence, model.Evidence{
				Type:    model.EvidenceResource,
				Ref:     fmt.Sprintf("pod/%s/%s", pod.Namespace, pod.Name),
				Message: fmt.Sprintf("CoreDNS pod is in %s phase", pod.Phase),
			})
		}
	}

	dnsEvents := findDNSEvents(snap.Events, dnsPods)
	for _, ev := range dnsEvents {
		evidence = append(evidence, model.Evidence{
			Type:    model.EvidenceEvent,
			Ref:     ev.InvolvedObject,
			Message: truncate(ev.Message, maxLogLineLen),
			Data: map[string]string{
				"reason": ev.Reason,
				"count":  fmt.Sprintf("%d", ev.Count),
			},
		})
	}

	logEvidence := findDNSLogPatternEvidence(snap.Events)
	evidence = append(evidence, logEvidence...)

	if len(evidence) == 0 {
		return nil
	}

	confidence := dnsConfidence(crashlooping, highRestarts, len(dnsEvents), len(logEvidence))
	severity := dnsSeverity(crashlooping, highRestarts, len(dnsPods))

	return []model.Finding{
		{
			SchemaVersion: model.SchemaVersion,
			ID:            "dns-instability",
			Title:         "DNS instability detected",
			Category:      "dns",
			Severity:      severity,
			Confidence:    confidence,
			Summary:       dnsSummary(dnsPods, crashlooping, highRestarts, len(dnsEvents)),
			Evidence:      evidence,
			NextSteps:     dnsNextSteps(),
			Timestamp:     time.Now().UTC(),
		},
	}
}

func findCoreDNSPods(snap *collector.Snapshot) []collector.PodInfo {
	var pods []collector.PodInfo

	for _, p := range snap.KubeSystem.Pods {
		if isCoreDNSPod(p.Name) {
			pods = append(pods, p)
		}
	}

	for _, p := range snap.Pods {
		if p.Namespace == "kube-system" && isCoreDNSPod(p.Name) {
			if !podInSlice(pods, p.Name, p.Namespace) {
				pods = append(pods, p)
			}
		}
	}

	return pods
}

func isCoreDNSPod(name string) bool {
	lower := strings.ToLower(name)
	for _, label := range coreDNSLabels {
		if strings.Contains(lower, label) {
			return true
		}
	}
	return false
}

func podInSlice(pods []collector.PodInfo, name, ns string) bool {
	for _, p := range pods {
		if p.Name == name && p.Namespace == ns {
			return true
		}
	}
	return false
}

func findDNSEvents(events []collector.EventInfo, dnsPods []collector.PodInfo) []collector.EventInfo {
	podRefs := make(map[string]bool)
	for _, p := range dnsPods {
		podRefs[fmt.Sprintf("Pod/%s/%s", p.Namespace, p.Name)] = true
	}

	var matched []collector.EventInfo
	for _, ev := range events {
		if !podRefs[ev.InvolvedObject] {
			continue
		}
		for _, reason := range dnsEventReasons {
			if strings.EqualFold(ev.Reason, reason) {
				matched = append(matched, ev)
				break
			}
		}
	}
	return matched
}

func findDNSLogPatternEvidence(events []collector.EventInfo) []model.Evidence {
	var evidence []model.Evidence
	for _, ev := range events {
		if !strings.Contains(strings.ToLower(ev.InvolvedObject), "coredns") &&
			!strings.Contains(strings.ToLower(ev.InvolvedObject), "kube-dns") {
			continue
		}
		for _, pattern := range dnsLogPatterns {
			if strings.Contains(strings.ToLower(ev.Message), strings.ToLower(pattern)) {
				evidence = append(evidence, model.Evidence{
					Type:    model.EvidenceLog,
					Ref:     ev.InvolvedObject,
					Message: truncate(ev.Message, maxLogLineLen),
					Data: map[string]string{
						"pattern": pattern,
					},
				})
				break
			}
		}
	}
	return evidence
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func dnsConfidence(crashlooping, highRestarts, eventCount, logPatterns int) float64 {
	base := 0.5
	if crashlooping > 0 {
		base += 0.25
	}
	if highRestarts > 0 {
		base += 0.10
	}
	if eventCount > 0 {
		base += 0.10
	}
	if logPatterns > 0 {
		base += 0.10
	}
	if base > 1.0 {
		base = 1.0
	}
	return base
}

func dnsSeverity(crashlooping, highRestarts, totalDNSPods int) model.Severity {
	if crashlooping > 0 && crashlooping >= totalDNSPods {
		return model.SeverityCritical
	}
	if crashlooping > 0 {
		return model.SeverityHigh
	}
	if highRestarts > 0 {
		return model.SeverityMedium
	}
	return model.SeverityLow
}

func dnsSummary(pods []collector.PodInfo, crashlooping, highRestarts, eventCount int) string {
	parts := []string{fmt.Sprintf("%d CoreDNS pod(s) inspected.", len(pods))}
	if crashlooping > 0 {
		parts = append(parts, fmt.Sprintf("%d crashlooping.", crashlooping))
	}
	if highRestarts > 0 {
		parts = append(parts, fmt.Sprintf("%d with high restart count.", highRestarts))
	}
	if eventCount > 0 {
		parts = append(parts, fmt.Sprintf("%d warning event(s).", eventCount))
	}
	return strings.Join(parts, " ")
}

func dnsNextSteps() []string {
	return []string{
		"Check CoreDNS pod logs for errors",
		"Review CoreDNS Corefile configuration",
		"Verify upstream DNS resolver connectivity",
		"Check CoreDNS resource limits and OOM kills",
		"Test DNS resolution from within pods",
	}
}
