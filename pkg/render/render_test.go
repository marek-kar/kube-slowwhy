package render

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marek-kar/kube-slowwhy/pkg/model"
)

var update = flag.Bool("update", false, "update golden files")

func testReport() model.Report {
	ts := time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC)
	return model.NewReport([]model.Finding{
		{
			SchemaVersion: model.SchemaVersion,
			ID:            "slow-001",
			Title:         "High Pod Restart Count",
			Category:      "pod-health",
			Severity:      model.SeverityHigh,
			Confidence:    0.95,
			Summary:       "Pod nginx-abc has restarted 12 times in the last hour",
			Evidence: []model.Evidence{
				{
					Type:    model.EvidenceEvent,
					Ref:     "v1/Event/default/nginx-abc.restart",
					Message: "Back-off restarting failed container",
					Data:    map[string]string{"restartCount": "12"},
				},
			},
			NextSteps: []string{"Check container logs", "Review resource limits"},
			Timestamp: ts,
		},
		{
			SchemaVersion: model.SchemaVersion,
			ID:            "slow-002",
			Title:         "CPU Throttling Detected",
			Category:      "resource-pressure",
			Severity:      model.SeverityMedium,
			Confidence:    0.80,
			Summary:       "Container web in pod frontend-xyz is being CPU throttled",
			Evidence: []model.Evidence{
				{
					Type:    model.EvidenceMetric,
					Ref:     "container_cpu_cfs_throttled_periods_total",
					Message: "CPU throttle ratio at 45%",
				},
			},
			NextSteps: []string{"Increase CPU limits"},
			Timestamp: ts,
		},
	})
}

func TestTableRenderer(t *testing.T) {
	goldenPath := filepath.Join("testdata", "single_finding.table.golden")
	r := New(FormatTable)
	var buf bytes.Buffer
	if err := r.Render(&buf, testReport()); err != nil {
		t.Fatalf("render: %v", err)
	}

	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update to create)", err)
	}

	if !bytes.Equal(buf.Bytes(), golden) {
		t.Errorf("output mismatch.\n--- got ---\n%s\n--- want ---\n%s", buf.String(), string(golden))
	}
}

func TestJSONRenderer(t *testing.T) {
	goldenPath := filepath.Join("testdata", "single_finding.json.golden")
	r := New(FormatJSON)
	var buf bytes.Buffer
	if err := r.Render(&buf, testReport()); err != nil {
		t.Fatalf("render: %v", err)
	}

	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update to create)", err)
	}

	if !bytes.Equal(buf.Bytes(), golden) {
		t.Errorf("output mismatch.\n--- got ---\n%s\n--- want ---\n%s", buf.String(), string(golden))
	}
}
