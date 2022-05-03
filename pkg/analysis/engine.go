package analysis

import (
	"github.com/marek-kar/kube-slowwhy/pkg/collector"
	"github.com/marek-kar/kube-slowwhy/pkg/model"
)

type Engine struct {
	rules []Rule
}

func NewEngine(rules ...Rule) *Engine {
	return &Engine{rules: rules}
}

func DefaultEngine() *Engine {
	return NewEngine(
		&NodePressureRule{},
		&PendingPodsRule{},
		&DNSRule{},
		&StorageRule{},
	)
}

func (e *Engine) Register(r Rule) {
	e.rules = append(e.rules, r)
}

func (e *Engine) Analyze(snap *collector.Snapshot) model.Report {
	var findings []model.Finding
	for _, r := range e.rules {
		findings = append(findings, r.Evaluate(snap)...)
	}
	return model.NewReport(findings)
}
