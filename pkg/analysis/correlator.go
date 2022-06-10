package analysis

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/marek-kar/kube-slowwhy/pkg/model"
)

type Correlator struct{}

func NewCorrelator() *Correlator {
	return &Correlator{}
}

func (c *Correlator) Correlate(findings []model.Finding) []model.Finding {
	merged := mergeFindings(findings)
	for i := range merged {
		merged[i].Confidence = boostConfidence(merged[i])
		merged[i].Reasoning = buildReasoning(merged[i])
	}
	sortFindings(merged)
	return merged
}

func rootCauseKey(f model.Finding) string {
	parts := []string{f.Category}
	refs := make(map[string]bool)
	for _, e := range f.Evidence {
		if e.Type == model.EvidenceResource && !refs[e.Ref] {
			refs[e.Ref] = true
			parts = append(parts, e.Ref)
		}
	}
	sort.Strings(parts[1:])
	return strings.Join(parts, "|")
}

func mergeFindings(findings []model.Finding) []model.Finding {
	type bucket struct {
		primary model.Finding
		extras  []model.Finding
	}

	order := make([]string, 0)
	buckets := make(map[string]*bucket)

	for _, f := range findings {
		key := rootCauseKey(f)
		if b, ok := buckets[key]; ok {
			b.extras = append(b.extras, f)
		} else {
			order = append(order, key)
			buckets[key] = &bucket{primary: f}
		}
	}

	result := make([]model.Finding, 0, len(order))
	for _, key := range order {
		b := buckets[key]
		merged := b.primary
		for _, extra := range b.extras {
			merged = mergePair(merged, extra)
		}
		result = append(result, merged)
	}
	return result
}

func mergePair(primary, secondary model.Finding) model.Finding {
	if severityRank(secondary.Severity) > severityRank(primary.Severity) {
		primary.Severity = secondary.Severity
	}
	if secondary.Confidence > primary.Confidence {
		primary.Confidence = secondary.Confidence
	}

	existingRefs := make(map[string]bool)
	for _, e := range primary.Evidence {
		existingRefs[string(e.Type)+":"+e.Ref+":"+e.Message] = true
	}
	for _, e := range secondary.Evidence {
		key := string(e.Type) + ":" + e.Ref + ":" + e.Message
		if !existingRefs[key] {
			primary.Evidence = append(primary.Evidence, e)
			existingRefs[key] = true
		}
	}

	existingSteps := make(map[string]bool)
	for _, s := range primary.NextSteps {
		existingSteps[s] = true
	}
	for _, s := range secondary.NextSteps {
		if !existingSteps[s] {
			primary.NextSteps = append(primary.NextSteps, s)
			existingSteps[s] = true
		}
	}

	return primary
}

func boostConfidence(f model.Finding) float64 {
	typeCounts := make(map[model.EvidenceType]int)
	for _, e := range f.Evidence {
		typeCounts[e.Type]++
	}

	distinctTypes := len(typeCounts)
	totalEvidence := len(f.Evidence)
	base := f.Confidence

	if distinctTypes >= 3 {
		base += 0.15
	} else if distinctTypes >= 2 {
		base += 0.10
	}

	if totalEvidence >= 5 {
		base += 0.05
	}

	base = math.Round(base*100) / 100
	if base > 1.0 {
		base = 1.0
	}
	return base
}

func buildReasoning(f model.Finding) string {
	typeCounts := make(map[model.EvidenceType]int)
	for _, e := range f.Evidence {
		typeCounts[e.Type]++
	}

	parts := []string{fmt.Sprintf("confidence=%.0f%%", f.Confidence*100)}

	typeLabels := []model.EvidenceType{
		model.EvidenceResource,
		model.EvidenceEvent,
		model.EvidenceLog,
		model.EvidenceMetric,
	}
	for _, t := range typeLabels {
		if n, ok := typeCounts[t]; ok {
			parts = append(parts, fmt.Sprintf("%d %s source(s)", n, t))
		}
	}

	distinctTypes := len(typeCounts)
	if distinctTypes >= 3 {
		parts = append(parts, "strong cross-signal agreement")
	} else if distinctTypes >= 2 {
		parts = append(parts, "cross-signal agreement")
	}

	return strings.Join(parts, "; ")
}

var severityOrder = map[model.Severity]int{
	model.SeverityLow:      0,
	model.SeverityMedium:   1,
	model.SeverityHigh:     2,
	model.SeverityCritical: 3,
}

func severityRank(s model.Severity) int {
	return severityOrder[s]
}

func sortFindings(findings []model.Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		ri := severityRank(findings[i].Severity)
		rj := severityRank(findings[j].Severity)
		if ri != rj {
			return ri > rj
		}
		if findings[i].Confidence != findings[j].Confidence {
			return findings[i].Confidence > findings[j].Confidence
		}
		return findings[i].ID < findings[j].ID
	})
}
