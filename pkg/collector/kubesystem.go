package collector

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const kubeSystemNS = "kube-system"

var criticalLabels = []string{
	"k8s-app=kube-dns",
	"k8s-app=kube-proxy",
	"k8s-app=calico-node",
	"k8s-app=cilium",
	"k8s-app=flannel",
	"app=csi-node-driver",
	"app.kubernetes.io/name=coredns",
	"app.kubernetes.io/name=aws-node",
}

func collectKubeSystemHealth(ctx context.Context, client kubernetes.Interface) (KubeSystemHealth, error) {
	var health KubeSystemHealth

	dsList, err := client.AppsV1().DaemonSets(kubeSystemNS).List(ctx, metav1.ListOptions{})
	if err != nil {
		return health, fmt.Errorf("list kube-system daemonsets: %w", err)
	}

	for _, ds := range dsList.Items {
		health.DaemonSets = append(health.DaemonSets, DaemonSetInfo{
			Name:                   ds.Name,
			DesiredNumberScheduled: ds.Status.DesiredNumberScheduled,
			CurrentNumberScheduled: ds.Status.CurrentNumberScheduled,
			NumberReady:            ds.Status.NumberReady,
			NumberMisscheduled:     ds.Status.NumberMisscheduled,
			NumberUnavailable:      ds.Status.NumberUnavailable,
		})
	}

	seen := make(map[string]bool)
	for _, sel := range criticalLabels {
		pods, err := client.CoreV1().Pods(kubeSystemNS).List(ctx, metav1.ListOptions{
			LabelSelector: sel,
		})
		if err != nil {
			continue
		}
		for _, p := range pods.Items {
			if seen[p.Name] {
				continue
			}
			seen[p.Name] = true

			containers := make([]ContainerInfo, 0, len(p.Spec.Containers))
			for i, c := range p.Spec.Containers {
				ci := ContainerInfo{
					Name:      c.Name,
					Resources: c.Resources,
				}
				if i < len(p.Status.ContainerStatuses) {
					ci.Ready = p.Status.ContainerStatuses[i].Ready
					ci.RestartCount = p.Status.ContainerStatuses[i].RestartCount
					ci.State = p.Status.ContainerStatuses[i].State
				}
				containers = append(containers, ci)
			}

			health.Pods = append(health.Pods, PodInfo{
				Name:       p.Name,
				Namespace:  p.Namespace,
				Phase:      p.Status.Phase,
				Conditions: p.Status.Conditions,
				Containers: containers,
				NodeName:   p.Spec.NodeName,
				QOSClass:   p.Status.QOSClass,
			})
		}
	}

	return health, nil
}
