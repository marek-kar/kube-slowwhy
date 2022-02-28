package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/marek-kar/kube-slowwhy/pkg/model"
)

type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
)

type Renderer interface {
	Render(w io.Writer, report model.Report) error
}

func New(f Format) Renderer {
	switch f {
	case FormatJSON:
		return &jsonRenderer{}
	default:
		return &tableRenderer{}
	}
}

type jsonRenderer struct{}

func (r *jsonRenderer) Render(w io.Writer, report model.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

type tableRenderer struct{}

func (r *tableRenderer) Render(w io.Writer, report model.Report) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	fmt.Fprintf(tw, "SEVERITY\tID\tCATEGORY\tTITLE\tCONFIDENCE\n")
	for _, f := range report.Findings {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%.0f%%\n",
			strings.ToUpper(string(f.Severity)),
			f.ID,
			f.Category,
			f.Title,
			f.Confidence*100,
		)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	for _, f := range report.Findings {
		fmt.Fprintf(w, "\n--- %s ---\n", f.ID)
		fmt.Fprintf(w, "Summary: %s\n", f.Summary)
		if len(f.Evidence) > 0 {
			fmt.Fprintf(w, "Evidence:\n")
			for _, e := range f.Evidence {
				fmt.Fprintf(w, "  [%s] %s\n", e.Type, e.Message)
				if e.Ref != "" {
					fmt.Fprintf(w, "         ref: %s\n", e.Ref)
				}
			}
		}
		if len(f.NextSteps) > 0 {
			fmt.Fprintf(w, "Next Steps:\n")
			for i, s := range f.NextSteps {
				fmt.Fprintf(w, "  %d. %s\n", i+1, s)
			}
		}
	}
	return nil
}
