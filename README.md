# kube-slowwhy

**Fast, read-only root cause analysis for slow Kubernetes clusters.**

kube-slowwhy collects a point-in-time snapshot of your cluster state — nodes, pods, events, storage, and critical system components — then runs a set of analysis rules to surface actionable findings ranked by severity and confidence.

## Features

- **Snapshot collection** — nodes, pods, events, PVCs/PVs, kube-system health in a single JSON file
- **Offline analysis** — run rules against saved snapshots without cluster access
- **Built-in rules:**
  - Node pressure detection (DiskPressure, MemoryPressure, PIDPressure) with eviction event correlation
  - Pending pod classification (insufficient cpu/memory, taints, affinity)
  - DNS instability (CoreDNS crashloops, SERVFAIL/timeout patterns)
  - Storage issues (Pending PVCs, FailedMount, CSI errors)
- **Finding correlation** — merges duplicates, boosts confidence from cross-signal agreement, deterministic severity-ranked output
- **Multiple output formats** — human-readable table or machine-readable JSON
- **Extensible rule engine** — implement a single interface to add new rules

## Installation

### Binary releases

### Homebrew

```bash
brew install marek-kar/tap/kube-slowwhy
```

### Go install

```bash
go install github.com/marek-kar/kube-slowwhy/cmd/kube-slowwhy@latest
```

### Build from source

```bash
git clone https://github.com/marek-kar/kube-slowwhy.git
cd kube-slowwhy
go build -o kube-slowwhy ./cmd/kube-slowwhy
```

## Quickstart

```bash
# Collect a cluster snapshot (uses current kubeconfig context)
kube-slowwhy collect --since 30m --out snapshot.json

# Collect from a specific namespace
kube-slowwhy collect --since 1h -n production -o prod-snapshot.json
```

The snapshot file can be shared with teammates, attached to incidents, or analyzed later without cluster access.

For a longer walkthrough, see [docs/quickstart.md](docs/quickstart.md).

## Example Output

### Table (default)

```
SEVERITY  ID        CATEGORY           TITLE                    CONFIDENCE
HIGH      slow-001  pod-health         High Pod Restart Count   95%
MEDIUM    slow-002  resource-pressure  CPU Throttling Detected  80%

--- slow-001 ---
Summary: Pod nginx-abc has restarted 12 times in the last hour
Evidence:
  [event] Back-off restarting failed container
         ref: v1/Event/default/nginx-abc.restart
Next Steps:
  1. Check container logs
  2. Review resource limits

--- slow-002 ---
Summary: Container web in pod frontend-xyz is being CPU throttled
Evidence:
  [metric] CPU throttle ratio at 45%
         ref: container_cpu_cfs_throttled_periods_total
Next Steps:
  1. Increase CPU limits
```

### JSON

```json
{
  "schemaVersion": "v1",
  "findings": [
    {
      "schemaVersion": "v1",
      "id": "node-pressure-worker-1-memorypressure",
      "title": "Node worker-1 has MemoryPressure",
      "category": "node-health",
      "severity": "critical",
      "confidence": 0.95,
      "reasoning": "confidence=95%; 1 resource source(s); 2 event source(s); cross-signal agreement",
      "summary": "Node worker-1 reports MemoryPressure (reason: KubeletHasInsufficientMemory). 2 eviction-related event(s) correlated.",
      "evidence": [
        {
          "type": "resource",
          "ref": "node/worker-1",
          "message": "Condition MemoryPressure is True: kubelet has insufficient memory available"
        },
        {
          "type": "event",
          "ref": "Pod/default/app-pod",
          "message": "The node worker-1 was low on resource: memory."
        }
      ],
      "nextSteps": [
        "Review pod memory requests and limits",
        "Check for memory leaks in workloads",
        "Consider adding nodes or increasing node memory"
      ],
      "timestamp": "2022-06-15T10:30:00Z"
    }
  ]
}
```

## Required Permissions (RBAC)

kube-slowwhy is **strictly read-only**. It requires the following cluster-scoped permissions:

```yaml
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
```

Bind it to a service account or your user:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kube-slowwhy
subjects:
  - kind: User
    name: your-user
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: kube-slowwhy
  apiGroup: rbac.authorization.k8s.io
```

## Security

- **No writes** — every API call is a read-only `get` or `list`
- **No secret access** — kube-slowwhy never reads Secrets or ConfigMaps
- **Log truncation** — any log or event message included in findings is truncated to 256 characters
- **Local output** — snapshot data is written to a local file; nothing is sent externally
- **No exec** — kube-slowwhy never runs commands inside containers

## Limitations

- Snapshot is a point-in-time view; intermittent issues may not be captured
- Event history depends on the cluster's event TTL (default 1 hour)
- Log-based detection relies on events containing log fragments; direct log streaming is not yet supported
- Analysis rules use heuristics — confidence scores indicate certainty, not guarantees
- Currently supports core Kubernetes resources; CRD-based analysis is planned

## Troubleshooting

| Problem | Solution |
|---|---|
| `load kubeconfig` error | Ensure `KUBECONFIG` is set or `~/.kube/config` exists |
| Empty snapshot | Check RBAC permissions; run `kubectl auth can-i list pods --all-namespaces` |
| No findings | Your cluster may be healthy! Try increasing `--since` duration |
| `signal: abort trap` on macOS | Known Go 1.21 issue; run with `CGO_ENABLED=0` or upgrade Go |

## Contributing

1. Fork the repo and create a feature branch
2. Implement changes with tests (`go test ./...`)
3. Ensure `go vet ./...` and `go fmt ./...` pass
4. Submit a PR with a clear description

To add a new analysis rule:

1. Create a file in `pkg/analysis/`
2. Implement the `Rule` interface (`Name()` + `Evaluate(*Snapshot) []Finding`)
3. Register it in `DefaultEngine()` in `pkg/analysis/engine.go`
4. Add tests with synthetic snapshots

## License

MIT
