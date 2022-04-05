package collector

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func collectPods(ctx context.Context, client kubernetes.Interface, namespace string) ([]PodInfo, error) {
	list, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	pods := make([]PodInfo, 0, len(list.Items))
	for _, p := range list.Items {
		containers := make([]ContainerInfo, 0, len(p.Spec.Containers))
		for i, c := range p.Spec.Containers {
			var state = p.Status.ContainerStatuses
			ci := ContainerInfo{
				Name:      c.Name,
				Resources: c.Resources,
			}
			if i < len(state) {
				ci.Ready = state[i].Ready
				ci.RestartCount = state[i].RestartCount
				ci.State = state[i].State
			}
			containers = append(containers, ci)
		}

		pods = append(pods, PodInfo{
			Name:       p.Name,
			Namespace:  p.Namespace,
			Phase:      p.Status.Phase,
			Conditions: p.Status.Conditions,
			Containers: containers,
			NodeName:   p.Spec.NodeName,
			QOSClass:   p.Status.QOSClass,
		})
	}
	return pods, nil
}
