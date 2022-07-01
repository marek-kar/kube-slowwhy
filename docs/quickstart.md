# Quickstart Guide

This guide walks you through installing kube-slowwhy, collecting your first cluster snapshot, and interpreting the analysis results.

## Prerequisites

- A running Kubernetes cluster (any version 1.24+)
- `kubectl` configured with a valid kubeconfig
- Read-only RBAC permissions (see [RBAC section](#set-up-rbac) below)

## Install

Pick one:

```bash
# Homebrew
brew install marek-kar/tap/kube-slowwhy

# Go install
go install github.com/marek-kar/kube-slowwhy/cmd/kube-slowwhy@latest

# From source
git clone https://github.com/marek-kar/kube-slowwhy.git
cd kube-slowwhy
go build -o kube-slowwhy ./cmd/kube-slowwhy
```

Verify the installation:

```bash
kube-slowwhy --help
```

## Set Up RBAC

kube-slowwhy only needs read access. Apply the minimal ClusterRole:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kube-slowwhy
rules:
  - apiGroups: [""]
    resources: [nodes, pods, events, persistentvolumeclaims, persistentvolumes]
    verbs: [get, list]
  - apiGroups: [apps]
    resources: [daemonsets]
    verbs: [get, list]
EOF
```

Bind it to your user or service account:

```bash
kubectl create clusterrolebinding kube-slowwhy \
  --clusterrole=kube-slowwhy \
  --user=$(kubectl config current-context)
```

## Step 1: Collect a Snapshot

Collect cluster state from the last 30 minutes:

```bash
kube-slowwhy collect --since 30m --out snapshot.json
```

You should see:

```
Collecting cluster snapshot (since 30m0s)...
Snapshot written to snapshot.json
```

### Options

| Flag | Default | Description |
|---|---|---|
| `--since` | `30m` | Look-back duration for events |
| `-n, --namespace` | _(all)_ | Filter by namespace |
| `-o, --out` | `snapshot.json` | Output file path |

### Examples

```bash
# Last hour, production namespace only
kube-slowwhy collect --since 1h -n production -o prod.json

# Last 2 hours, all namespaces
kube-slowwhy collect --since 2h -o full-cluster.json
```

## Step 2: Inspect the Snapshot

The snapshot is a self-contained JSON file. You can inspect it directly:

```bash
# Count nodes
jq '.nodes | length' snapshot.json

# List pending pods
jq '[.pods[] | select(.phase == "Pending")] | length' snapshot.json

# View warning events
jq '[.events[] | select(.type == "Warning")] | .[:5]' snapshot.json
```

The snapshot can be shared with teammates, attached to incident tickets, or stored for later comparison.

## Step 3: Understand the Findings

kube-slowwhy analyzes the snapshot and produces findings. Each finding includes:

| Field | Description |
|---|---|
| **Severity** | `critical`, `high`, `medium`, or `low` |
| **Confidence** | 0–100% — how certain the tool is about this finding |
| **Reasoning** | Short explanation of confidence score factors |
| **Category** | Grouping: `node-health`, `scheduling`, `dns`, `storage` |
| **Evidence** | References to specific nodes, pods, events, or metrics |
| **Next Steps** | Actionable remediation suggestions |

### Severity Guide

| Level | Meaning |
|---|---|
| **Critical** | Active outage or cascade failure likely (e.g., all CoreDNS pods crashlooping) |
| **High** | Significant degradation (e.g., node under memory pressure with evictions) |
| **Medium** | Potential issue (e.g., pods pending due to taints) |
| **Low** | Informational (e.g., single storage event) |

### Confidence Scoring

Confidence is boosted when multiple independent evidence sources agree:

- A single resource condition → base confidence (~0.5–0.7)
- Correlated events → +0.10–0.15
- Multiple evidence types (resource + event + log) → additional boost
- Findings with the same root cause are merged and confidence is recalculated

## Step 4: Built-in Analysis Rules

### Node Pressure

Detects `DiskPressure`, `MemoryPressure`, and `PIDPressure` conditions. Correlates with eviction-related events (`Evicted`, `OOMKilling`, `SystemOOM`). Escalates to critical when evictions are happening.

### Pending Pods

Groups pending pods by scheduling failure reason:

- **insufficient-cpu / insufficient-memory** — cluster is out of capacity
- **taint** — pods lack required tolerations
- **affinity** — node selector or affinity rules can't be satisfied
- **unschedulable** — nodes are cordoned

### DNS Instability

Checks CoreDNS pods for:

- CrashLoopBackOff state
- High restart counts (≥3)
- SERVFAIL, timeout, NXDOMAIN patterns in events

Critical when all CoreDNS replicas are down.

### Storage Issues

Detects:

- PVCs stuck in Pending
- PVs in Failed phase
- FailedAttachVolume, FailedMount, CSI errors in events

## Step 5: Share and Collaborate

Snapshots are portable. Common workflows:

```bash
# Collect on a production jump host
kube-slowwhy collect --since 1h -o incident-2022-06-15.json

# Transfer to your laptop
scp jumphost:incident-2022-06-15.json .

# Inspect locally with jq or any JSON viewer
jq '.findings[] | {id, severity, title}' incident-2022-06-15.json
```

## Troubleshooting

**"load kubeconfig" error**

Ensure your kubeconfig is accessible:

```bash
echo $KUBECONFIG
kubectl cluster-info
```

**Empty snapshot**

Verify RBAC permissions:

```bash
kubectl auth can-i list pods --all-namespaces
kubectl auth can-i list nodes
kubectl auth can-i list events --all-namespaces
```

**No findings generated**

Your cluster may be healthy. Try widening the event window:

```bash
kube-slowwhy collect --since 2h -o wider-snapshot.json
```

**macOS `signal: abort trap`**

Known Go 1.21 issue on macOS. Workaround:

```bash
CGO_ENABLED=0 go build -o kube-slowwhy ./cmd/kube-slowwhy
```

Or upgrade to Go 1.22+.

## Next Steps

- Add kube-slowwhy to your incident response runbook
- Run periodic snapshots via CronJob for trend analysis
- Contribute new analysis rules — see [Contributing](../README.md#contributing)
