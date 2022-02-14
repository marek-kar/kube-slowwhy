package model

import "time"

const SchemaVersion = "v1"

type EvidenceType string

const (
	EvidenceMetric   EvidenceType = "metric"
	EvidenceEvent    EvidenceType = "event"
	EvidenceLog      EvidenceType = "log"
	EvidenceResource EvidenceType = "resource"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

type Evidence struct {
	Type    EvidenceType      `json:"type"`
	Ref     string            `json:"ref"`
	Message string            `json:"message"`
	Data    map[string]string `json:"data,omitempty"`
}

type Finding struct {
	SchemaVersion string     `json:"schemaVersion"`
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Category      string     `json:"category"`
	Severity      Severity   `json:"severity"`
	Confidence    float64    `json:"confidence"`
	Reasoning     string     `json:"reasoning,omitempty"`
	Summary       string     `json:"summary"`
	Evidence      []Evidence `json:"evidence"`
	NextSteps     []string   `json:"nextSteps"`
	Timestamp     time.Time  `json:"timestamp"`
}

type Report struct {
	SchemaVersion string    `json:"schemaVersion"`
	Findings      []Finding `json:"findings"`
}

func NewReport(findings []Finding) Report {
	return Report{
		SchemaVersion: SchemaVersion,
		Findings:      findings,
	}
}
