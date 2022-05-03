package analysis

import (
	"github.com/marek-kar/kube-slowwhy/pkg/collector"
	"github.com/marek-kar/kube-slowwhy/pkg/model"
)

type Rule interface {
	Name() string
	Evaluate(snap *collector.Snapshot) []model.Finding
}
