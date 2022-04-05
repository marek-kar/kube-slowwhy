package collector

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func collectEvents(ctx context.Context, client kubernetes.Interface, since time.Duration) ([]EventInfo, error) {
	cutoff := time.Now().Add(-since)

	list, err := client.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	events := make([]EventInfo, 0)
	for _, e := range list.Items {
		ts := e.LastTimestamp.Time
		if ts.IsZero() {
			ts = e.EventTime.Time
		}
		if ts.Before(cutoff) {
			continue
		}

		events = append(events, EventInfo{
			Namespace:      e.Namespace,
			Name:           e.Name,
			Reason:         e.Reason,
			Message:        e.Message,
			Type:           e.Type,
			InvolvedObject: fmt.Sprintf("%s/%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Namespace, e.InvolvedObject.Name),
			Count:          e.Count,
			FirstTimestamp: e.FirstTimestamp.Time,
			LastTimestamp:  ts,
		})
	}
	return events, nil
}
