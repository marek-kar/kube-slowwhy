package collector

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
)

func Collect(ctx context.Context, client kubernetes.Interface, opts Options) (*Snapshot, error) {
	snap := &Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		CollectedAt:   time.Now().UTC(),
		Since:         opts.Since.String(),
	}

	var errs []error

	nodes, err := collectNodes(ctx, client)
	if err != nil {
		errs = append(errs, err)
	}
	snap.Nodes = nodes

	pods, err := collectPods(ctx, client, opts.Namespace)
	if err != nil {
		errs = append(errs, err)
	}
	snap.Pods = pods

	events, err := collectEvents(ctx, client, opts.Since)
	if err != nil {
		errs = append(errs, err)
	}
	snap.Events = events

	pvcs, err := collectPVCs(ctx, client, opts.Namespace)
	if err != nil {
		errs = append(errs, err)
	}
	snap.PVCs = pvcs

	pvs, err := collectPVs(ctx, client)
	if err != nil {
		errs = append(errs, err)
	}
	snap.PVs = pvs

	ksHealth, err := collectKubeSystemHealth(ctx, client)
	if err != nil {
		errs = append(errs, err)
	}
	snap.KubeSystem = ksHealth

	if len(errs) > 0 {
		return snap, fmt.Errorf("collection had %d errors; first: %w", len(errs), errs[0])
	}
	return snap, nil
}
