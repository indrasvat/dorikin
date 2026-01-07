# Drift Detection

> How dorikin compares your manifests against cluster state

This document explains the core drift detection algorithm, what it handles, known limitations, and future improvement areas.

---

## Overview

Dorikin performs **field-level comparison** between your YAML manifests (desired state) and the live Kubernetes cluster (actual state). The algorithm:

1. **Loads** manifests from files/directories
2. **Fetches** corresponding resources from the cluster (concurrently)
3. **Filters** out Kubernetes-managed fields
4. **Compares** remaining fields recursively
5. **Reports** differences with JSONPath locations

```
┌─────────────────┐     ┌─────────────────┐
│  YAML Manifest  │     │  Cluster State  │
│  (desired)      │     │  (actual)       │
└────────┬────────┘     └────────┬────────┘
         │                       │
         ▼                       ▼
    ┌────────────────────────────────┐
    │  Filter Kubernetes-managed     │
    │  fields (status, uid, etc.)    │
    └────────────────┬───────────────┘
                     │
                     ▼
    ┌────────────────────────────────┐
    │  Recursive field comparison    │
    │  with type normalization       │
    └────────────────┬───────────────┘
                     │
                     ▼
    ┌────────────────────────────────┐
    │  Generate diff report with     │
    │  JSONPath locations            │
    └────────────────────────────────┘
```

---

## Drift Statuses

| Status | Meaning |
|--------|---------|
| `IN_SYNC` | All compared fields match exactly |
| `DRIFTED` | Resource exists but one or more fields differ |
| `MISSING` | Resource defined in manifest but not found in cluster |
| `EXTRA` | Resource exists in cluster but not in manifests (requires `--include-extra`) |
| `ERROR` | Could not fetch or compare the resource |

---

## Comparison Algorithm

### Field-by-Field Comparison

The comparator walks both object trees recursively:

- **Maps**: Compares all keys; reports added/removed/modified keys
- **Slices**: Compares element-by-element by index (order matters)
- **Scalars**: Direct equality check after type normalization

### Type Normalization

JSON and YAML handle numbers differently:
- JSON unmarshals all numbers as `float64`
- YAML preserves integer types (`int`, `int64`)

Dorikin normalizes all numeric types to `float64` before comparison to prevent false positives:

```go
// Both compare as equal after normalization
manifest: replicas: 3      // YAML: int
cluster:  replicas: 3.0    // JSON: float64
```

### Diff Types

| Type | Meaning |
|------|---------|
| `modified` | Field exists in both but values differ |
| `added` | Field exists in cluster but not in manifest |
| `removed` | Field exists in manifest but not in cluster |

---

## Field Filtering

Kubernetes adds many server-side fields that would cause false drift detection. Dorikin filters these by default.

### Default Ignored Fields

**Metadata (always server-managed):**
- `metadata.resourceVersion` - Changes on every update
- `metadata.uid` - Unique identifier assigned by server
- `metadata.generation` - Incremented on spec changes
- `metadata.creationTimestamp` - Set at creation time
- `metadata.managedFields` - Server-side apply tracking
- `metadata.selfLink` - Deprecated self-reference
- `metadata.annotations.kubectl.kubernetes.io/last-applied-configuration`
- `metadata.annotations.deployment.kubernetes.io/revision`

**Status (always server-side):**
- `status` - Entire status subresource

**Deployment defaults:**
- `spec.progressDeadlineSeconds`
- `spec.revisionHistoryLimit`
- `spec.strategy`
- `spec.template.metadata.creationTimestamp`

**Pod spec defaults:**
- `spec.template.spec.dnsPolicy`
- `spec.template.spec.restartPolicy`
- `spec.template.spec.schedulerName`
- `spec.template.spec.terminationGracePeriodSeconds`
- `spec.template.spec.securityContext`

**Container defaults:**
- `spec.template.spec.containers.imagePullPolicy`
- `spec.template.spec.containers.terminationMessagePath`
- `spec.template.spec.containers.terminationMessagePolicy`
- `spec.template.spec.containers.ports.protocol`
- Probe thresholds and schemes

**Service defaults:**
- `spec.clusterIP` / `spec.clusterIPs` - Assigned by server
- `spec.internalTrafficPolicy`
- `spec.ipFamilies` / `spec.ipFamilyPolicy`
- `spec.sessionAffinity`
- `spec.ports.protocol`

### Custom Ignore Paths

Add custom paths via the `--ignore` flag:

```bash
# Ignore replica count (useful with HPA)
dorikin scan --ignore spec.replicas ./manifests/

# Ignore multiple paths
dorikin scan --ignore spec.replicas --ignore metadata.labels.version ./manifests/
```

Ignore paths support:
- Exact matches: `spec.replicas`
- Nested fields: `metadata.annotations.my-annotation`
- Array wildcards: `spec.template.spec.containers.resources` (matches all containers)

---

## What Dorikin Handles Well

### Supported Resource Types

Any Kubernetes resource with a standard structure:
- Deployments, StatefulSets, DaemonSets
- Services, Ingresses
- ConfigMaps, Secrets
- HPAs, PDBs
- CRDs (Custom Resource Definitions)
- RBAC resources
- And more...

### Accurate Detection

| Scenario | Detection |
|----------|-----------|
| Image tag changed | `spec.template.spec.containers[0].image`: `nginx:1.25` → `nginx:1.26` |
| Replicas modified | `spec.replicas`: `3` → `5` |
| Env var added | `spec.template.spec.containers[0].env[2]`: added |
| ConfigMap data changed | `data.KEY`: `old-value` → `new-value` |
| Label removed | `metadata.labels.environment`: removed |
| Resource limits changed | `spec.template.spec.containers[0].resources.limits.memory`: `128Mi` → `256Mi` |

### Multi-Document YAML

Files with multiple resources (separated by `---`) are parsed correctly:

```yaml
# manifests/app.yaml
apiVersion: apps/v1
kind: Deployment
# ...
---
apiVersion: v1
kind: Service
# ...
```

---

## Limitations & Caveats

### HPA and Deployment Replicas

**Problem:** When an HPA manages a Deployment, it actively modifies `spec.replicas`. If your manifest says `replicas: 3` but HPA has scaled to 5, dorikin reports drift.

**Why:** Dorikin compares manifests to cluster state exactly. It doesn't know that the HPA is *supposed* to change replicas.

**Workaround:**
```bash
dorikin scan --ignore spec.replicas ./manifests/
```

**Future:** HPA-aware mode that validates replicas within `minReplicas`/`maxReplicas` range.

### No Range/Semantic Validation

**Problem:** Dorikin performs exact comparison, not semantic validation.

| Field | Current Behavior | Semantic Behavior (future) |
|-------|------------------|---------------------------|
| HPA `minReplicas: 3` | Exact match required | Could validate actual ∈ [min, max] |
| Resource `memory: 128Mi` | String comparison | Could normalize units (128Mi = 134217728) |
| Image `nginx:latest` | String comparison | Could resolve digest |

### Slice Ordering

**Problem:** Arrays are compared by index. If elements are reordered, dorikin sees multiple modifications.

```yaml
# Manifest                    # Cluster
env:                          env:
  - name: A                     - name: B    # Flagged as modified
  - name: B                     - name: A    # Flagged as modified
```

**Future:** Content-based matching for arrays with identifiable elements (e.g., match containers by `name`).

### Controller-Managed Fields

**Problem:** Some controllers (operators, admission webhooks) intentionally modify fields.

**Example:** An admission webhook that injects sidecar containers will always show as drift.

**Workaround:** Use `--ignore` for known controller-managed paths.

**Future:** Use `metadata.managedFields` to identify and optionally skip controller-owned fields.

### No Helm/Kustomize Rendering

**Current:** Dorikin reads raw YAML files only.

**Limitation:** If you use Helm or Kustomize, you must render templates first:

```bash
# Helm
helm template my-release ./chart > rendered.yaml
dorikin scan rendered.yaml

# Kustomize
kustomize build ./overlays/prod > rendered.yaml
dorikin scan rendered.yaml
```

**Future:** Native `--helm` and `--kustomize` flags for direct rendering.

### Quantity String Comparison

**Problem:** Kubernetes quantities like `128Mi`, `0.5`, `500m` are compared as strings, not values.

```yaml
# These are semantically equal but will show as drift:
memory: 128Mi    vs    memory: 134217728
cpu: 500m        vs    cpu: 0.5
```

**Future:** Quantity normalization for resource values.

---

## Future Improvements

Planned enhancements to address current limitations:

| Feature | Description | Status |
|---------|-------------|--------|
| HPA-aware comparison | Skip `spec.replicas` when HPA targets the deployment | Planned |
| Quantity normalization | Treat `128Mi` = `134217728` = `128M` | Planned |
| Content-based array matching | Match array elements by key field (e.g., container name) | Planned |
| Helm integration | `dorikin scan --helm ./chart` | Planned |
| Kustomize integration | `dorikin scan --kustomize ./overlay` | Planned |
| Controller ownership | Use `managedFields` to skip controller-owned fields | Planned |
| EXTRA resource detection | Find resources in cluster not in manifests | Partial |
| Drift remediation | `dorikin apply` to sync cluster to manifests | Roadmap |

---

## Best Practices

### 1. Start with baseline validation

```bash
# After deploying, verify zero drift
kubectl apply -f ./manifests/
dorikin scan ./manifests/
```

### 2. Customize ignore paths for your environment

```bash
# Create a wrapper script or alias
alias dorikin-scan='dorikin scan --ignore spec.replicas --ignore metadata.annotations.prometheus.io/scrape'
```

### 3. Use quiet mode in CI/CD

```bash
# Exit code 0 = no drift, 1 = drift detected
dorikin scan -o quiet ./manifests/ || exit 1
```

### 4. Organize manifests logically

```
manifests/
├── base/           # Core resources
├── apps/           # Application deployments
└── config/         # ConfigMaps, Secrets
```

### 5. Review drifted fields, not just status

The TUI (`dorikin ui`) shows exactly which fields drifted with expected vs actual values.

---

## Technical Details

### Concurrency

Resource fetching uses Go 1.25's `sync.WaitGroup.Go()` for concurrent cluster queries, significantly improving scan time for large manifest sets.

### Memory Efficiency

Resources are processed as `unstructured.Unstructured` objects, avoiding full type deserialization and reducing memory overhead.

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All resources in sync |
| `1` | Drift detected (DRIFTED, MISSING, or EXTRA) |
| `2` | Error during scan |
