package collector

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

const SnapshotSchemaVersion = "v1"

type Snapshot struct {
	SchemaVersion string            `json:"schemaVersion"`
	CollectedAt   time.Time         `json:"collectedAt"`
	Since         string            `json:"since"`
	Nodes         []NodeInfo        `json:"nodes"`
	Pods          []PodInfo         `json:"pods"`
	Events        []EventInfo       `json:"events"`
	PVCs          []PVCInfo         `json:"pvcs"`
	PVs           []PVInfo          `json:"pvs"`
	KubeSystem    KubeSystemHealth  `json:"kubeSystem"`
}

type NodeInfo struct {
	Name         string                        `json:"name"`
	Conditions   []corev1.NodeCondition        `json:"conditions"`
	Allocatable  corev1.ResourceList           `json:"allocatable"`
	Capacity     corev1.ResourceList           `json:"capacity"`
	Unschedulable bool                         `json:"unschedulable"`
}

type PodInfo struct {
	Name       string                 `json:"name"`
	Namespace  string                 `json:"namespace"`
	Phase      corev1.PodPhase        `json:"phase"`
	Conditions []corev1.PodCondition  `json:"conditions,omitempty"`
	Containers []ContainerInfo        `json:"containers"`
	NodeName   string                 `json:"nodeName"`
	QOSClass   corev1.PodQOSClass     `json:"qosClass"`
}

type ContainerInfo struct {
	Name         string                       `json:"name"`
	Ready        bool                         `json:"ready"`
	RestartCount int32                        `json:"restartCount"`
	State        corev1.ContainerState        `json:"state"`
	Resources    corev1.ResourceRequirements  `json:"resources"`
}

type EventInfo struct {
	Namespace      string    `json:"namespace"`
	Name           string    `json:"name"`
	Reason         string    `json:"reason"`
	Message        string    `json:"message"`
	Type           string    `json:"type"`
	InvolvedObject string    `json:"involvedObject"`
	Count          int32     `json:"count"`
	FirstTimestamp time.Time `json:"firstTimestamp"`
	LastTimestamp   time.Time `json:"lastTimestamp"`
}

type PVCInfo struct {
	Name             string                            `json:"name"`
	Namespace        string                            `json:"namespace"`
	Phase            corev1.PersistentVolumeClaimPhase `json:"phase"`
	VolumeName       string                            `json:"volumeName"`
	StorageClassName string                            `json:"storageClassName,omitempty"`
	Capacity         corev1.ResourceList               `json:"capacity,omitempty"`
}

type PVInfo struct {
	Name             string                      `json:"name"`
	Phase            corev1.PersistentVolumePhase `json:"phase"`
	StorageClassName string                       `json:"storageClassName,omitempty"`
	Capacity         corev1.ResourceList          `json:"capacity,omitempty"`
	ClaimRef         string                       `json:"claimRef,omitempty"`
}

type KubeSystemHealth struct {
	DaemonSets []DaemonSetInfo `json:"daemonSets"`
	Pods       []PodInfo       `json:"pods"`
}

type DaemonSetInfo struct {
	Name                string `json:"name"`
	DesiredNumberScheduled int32  `json:"desiredNumberScheduled"`
	CurrentNumberScheduled int32  `json:"currentNumberScheduled"`
	NumberReady            int32  `json:"numberReady"`
	NumberMisscheduled     int32  `json:"numberMisscheduled"`
	NumberUnavailable      int32  `json:"numberUnavailable"`
}

