package collector

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func collectNodes(ctx context.Context, client kubernetes.Interface) ([]NodeInfo, error) {
	list, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	nodes := make([]NodeInfo, 0, len(list.Items))
	for _, n := range list.Items {
		nodes = append(nodes, NodeInfo{
			Name:          n.Name,
			Conditions:    n.Status.Conditions,
			Allocatable:   n.Status.Allocatable,
			Capacity:      n.Status.Capacity,
			Unschedulable: n.Spec.Unschedulable,
		})
	}
	return nodes, nil
}
