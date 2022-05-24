package analysis

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/marek-kar/kube-slowwhy/pkg/collector"
	"github.com/marek-kar/kube-slowwhy/pkg/model"
)

func TestDNSRule_NoCoreDNSPods(t *testing.T) {
	snap := &collector.Snapshot{
		KubeSystem: collector.KubeSystemHealth{
			Pods: []collector.PodInfo{
				{Name: "kube-proxy-abc", Namespace: "kube-system", Phase: corev1.PodRunning},
			},
		},
	}

	rule := &DNSRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestDNSRule_HealthyCoreDNS(t *testing.T) {
	snap := &collector.Snapshot{
		KubeSystem: collector.KubeSystemHealth{
			Pods: []collector.PodInfo{
				{
					Name: "coredns-abc123", Namespace: "kube-system", Phase: corev1.PodRunning,
					Containers: []collector.ContainerInfo{
						{Name: "coredns", Ready: true, RestartCount: 0},
					},
				},
				{
					Name: "coredns-def456", Namespace: "kube-system", Phase: corev1.PodRunning,
					Containers: []collector.ContainerInfo{
						{Name: "coredns", Ready: true, RestartCount: 1},
					},
				},
			},
		},
	}

	rule := &DNSRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 0 {
		t.Errorf("expected 0 findings for healthy CoreDNS, got %d", len(findings))
	}
}

func TestDNSRule_CrashLooping(t *testing.T) {
	snap := &collector.Snapshot{
		KubeSystem: collector.KubeSystemHealth{
			Pods: []collector.PodInfo{
				{
					Name: "coredns-abc123", Namespace: "kube-system", Phase: corev1.PodRunning,
					Containers: []collector.ContainerInfo{
						{
							Name:         "coredns",
							Ready:        false,
							RestartCount: 15,
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "CrashLoopBackOff",
								},
							},
						},
					},
				},
				{
					Name: "coredns-def456", Namespace: "kube-system", Phase: corev1.PodRunning,
					Containers: []collector.ContainerInfo{
						{Name: "coredns", Ready: true, RestartCount: 0},
					},
				},
			},
		},
		Events: []collector.EventInfo{
			{
				Namespace:      "kube-system",
				Name:           "coredns-abc123.backoff",
				Reason:         "BackOff",
				Message:        "Back-off restarting failed container",
				Type:           "Warning",
				InvolvedObject: "Pod/kube-system/coredns-abc123",
				Count:          10,
			},
		},
	}

	rule := &DNSRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.ID != "dns-instability" {
		t.Errorf("id: got %q, want %q", f.ID, "dns-instability")
	}
	if f.Category != "dns" {
		t.Errorf("category: got %q, want %q", f.Category, "dns")
	}
	if f.Severity != model.SeverityHigh {
		t.Errorf("severity: got %q, want %q", f.Severity, model.SeverityHigh)
	}
	if f.Confidence < 0.8 {
		t.Errorf("confidence too low: got %f, want >= 0.8", f.Confidence)
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

func TestDNSRule_AllCrashLoopingCritical(t *testing.T) {
	snap := &collector.Snapshot{
		KubeSystem: collector.KubeSystemHealth{
			Pods: []collector.PodInfo{
				{
					Name: "coredns-abc", Namespace: "kube-system", Phase: corev1.PodRunning,
					Containers: []collector.ContainerInfo{
						{
							Name: "coredns", Ready: false, RestartCount: 8,
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
							},
						},
					},
				},
				{
					Name: "coredns-def", Namespace: "kube-system", Phase: corev1.PodRunning,
					Containers: []collector.ContainerInfo{
						{
							Name: "coredns", Ready: false, RestartCount: 12,
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
							},
						},
					},
				},
			},
		},
	}

	rule := &DNSRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != model.SeverityCritical {
		t.Errorf("severity: got %q, want %q", findings[0].Severity, model.SeverityCritical)
	}
}

func TestDNSRule_HighRestarts(t *testing.T) {
	snap := &collector.Snapshot{
		KubeSystem: collector.KubeSystemHealth{
			Pods: []collector.PodInfo{
				{
					Name: "coredns-abc", Namespace: "kube-system", Phase: corev1.PodRunning,
					Containers: []collector.ContainerInfo{
						{Name: "coredns", Ready: true, RestartCount: 10},
					},
				},
			},
		},
	}

	rule := &DNSRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != model.SeverityMedium {
		t.Errorf("severity: got %q, want %q", findings[0].Severity, model.SeverityMedium)
	}
}

func TestDNSRule_LogPatterns(t *testing.T) {
	snap := &collector.Snapshot{
		KubeSystem: collector.KubeSystemHealth{
			Pods: []collector.PodInfo{
				{
					Name: "coredns-abc", Namespace: "kube-system", Phase: corev1.PodRunning,
					Containers: []collector.ContainerInfo{
						{Name: "coredns", Ready: true, RestartCount: 5},
					},
				},
			},
		},
		Events: []collector.EventInfo{
			{
				Namespace:      "kube-system",
				Name:           "coredns-abc.log",
				Reason:         "Unhealthy",
				Message:        "Liveness probe failed: SERVFAIL for upstream dns query",
				Type:           "Warning",
				InvolvedObject: "Pod/kube-system/coredns-abc",
				Count:          3,
			},
		},
	}

	rule := &DNSRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	hasLogEvidence := false
	for _, e := range findings[0].Evidence {
		if e.Type == model.EvidenceLog {
			hasLogEvidence = true
			break
		}
	}
	if !hasLogEvidence {
		t.Error("expected log evidence for SERVFAIL pattern")
	}
}

func TestDNSRule_TruncatesLongMessages(t *testing.T) {
	longMsg := ""
	for i := 0; i < 300; i++ {
		longMsg += "x"
	}

	snap := &collector.Snapshot{
		KubeSystem: collector.KubeSystemHealth{
			Pods: []collector.PodInfo{
				{
					Name: "coredns-abc", Namespace: "kube-system", Phase: corev1.PodRunning,
					Containers: []collector.ContainerInfo{
						{Name: "coredns", Ready: true, RestartCount: 5},
					},
				},
			},
		},
		Events: []collector.EventInfo{
			{
				Namespace:      "kube-system",
				Name:           "coredns-abc.event",
				Reason:         "BackOff",
				Message:        longMsg,
				Type:           "Warning",
				InvolvedObject: "Pod/kube-system/coredns-abc",
				Count:          1,
			},
		},
	}

	rule := &DNSRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	for _, e := range findings[0].Evidence {
		if len(e.Message) > maxLogLineLen {
			t.Errorf("message not truncated: len %d > %d", len(e.Message), maxLogLineLen)
		}
	}
}

func TestDNSRule_PodFromMainPodList(t *testing.T) {
	snap := &collector.Snapshot{
		Pods: []collector.PodInfo{
			{
				Name: "coredns-xyz", Namespace: "kube-system", Phase: corev1.PodRunning,
				Containers: []collector.ContainerInfo{
					{Name: "coredns", Ready: true, RestartCount: 7},
				},
			},
		},
	}

	rule := &DNSRule{}
	findings := rule.Evaluate(snap)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding from main pod list, got %d", len(findings))
	}
}

func TestDNSRule_Name(t *testing.T) {
	rule := &DNSRule{}
	if rule.Name() != "dns-instability" {
		t.Errorf("name: got %q, want %q", rule.Name(), "dns-instability")
	}
}

var _ Rule = (*DNSRule)(nil)
