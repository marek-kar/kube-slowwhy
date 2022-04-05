package collector

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func collectPVCs(ctx context.Context, client kubernetes.Interface, namespace string) ([]PVCInfo, error) {
	list, err := client.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pvcs: %w", err)
	}

	pvcs := make([]PVCInfo, 0, len(list.Items))
	for _, p := range list.Items {
		sc := ""
		if p.Spec.StorageClassName != nil {
			sc = *p.Spec.StorageClassName
		}
		pvcs = append(pvcs, PVCInfo{
			Name:             p.Name,
			Namespace:        p.Namespace,
			Phase:            p.Status.Phase,
			VolumeName:       p.Spec.VolumeName,
			StorageClassName: sc,
			Capacity:         p.Status.Capacity,
		})
	}
	return pvcs, nil
}

func collectPVs(ctx context.Context, client kubernetes.Interface) ([]PVInfo, error) {
	list, err := client.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pvs: %w", err)
	}

	pvs := make([]PVInfo, 0, len(list.Items))
	for _, p := range list.Items {
		claimRef := ""
		if p.Spec.ClaimRef != nil {
			claimRef = fmt.Sprintf("%s/%s", p.Spec.ClaimRef.Namespace, p.Spec.ClaimRef.Name)
		}
		pvs = append(pvs, PVInfo{
			Name:             p.Name,
			Phase:            p.Status.Phase,
			StorageClassName: p.Spec.StorageClassName,
			Capacity:         p.Spec.Capacity,
			ClaimRef:         claimRef,
		})
	}
	return pvs, nil
}
