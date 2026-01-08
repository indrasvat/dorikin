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

### Configuration File

Dorikin looks for `.dorikin.yaml` in your project root (or any parent directory). This is the recommended way to configure ignore paths and other settings.

```yaml
# .dorikin.yaml
ignore:
  # Extend built-in defaults (default: true)
  extend_defaults: true

  # Additional global paths to ignore
  paths:
    - metadata.annotations.my-org.io/managed-by

  # Resource-type-specific paths (for CRDs, etc.)
  resources:
    MyCustomResource:
      - spec.internalState

# HPA awareness mode
hpa_aware: manifests  # manifests | cluster | disabled
```

See `.dorikin.yaml.example` in the repository for a complete reference.

### Default Ignored Fields

The built-in defaults are defined in [`internal/config/defaults.go`](../internal/config/defaults.go). Categories include:

| Category | Examples |
|----------|----------|
| **Metadata** | `resourceVersion`, `uid`, `generation`, `creationTimestamp`, `managedFields` |
| **Annotations** | `kubectl.kubernetes.io/last-applied-configuration`, `deployment.kubernetes.io/revision`, `autoscaling.alpha.kubernetes.io/*` |
| **Status** | Entire `status` subresource |
| **Deployment** | `progressDeadlineSeconds`, `revisionHistoryLimit`, `strategy` |
| **Pod spec** | `dnsPolicy`, `restartPolicy`, `schedulerName`, `terminationGracePeriodSeconds` |
| **Container** | `imagePullPolicy`, `terminationMessagePath`, probe thresholds |
| **Service** | `clusterIP`, `clusterIPs`, `internalTrafficPolicy`, `ipFamilies` |
| **StatefulSet** | `persistentVolumeClaimRetentionPolicy` |

### CLI Override

The `--ignore` flag overrides the config file entirely:

```bash
# Use only these paths (ignores config file)
dorikin scan --ignore spec.replicas --ignore metadata.labels.version ./manifests/
```

### Ignore Path Syntax

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

**Solved:** Dorikin now supports **HPA-aware drift detection**. When an HPA manages a scalable resource (Deployment, StatefulSet, ReplicaSet), dorikin automatically skips `spec.replicas` comparison.

#### HPA Awareness Modes

| Mode | Flag | Behavior |
|------|------|----------|
| **Manifests** (default) | `--hpa-aware=manifests` | Extracts HPAs from manifest files |
| **Cluster** | `--hpa-aware=cluster` | Also queries cluster for HPAs |
| **Disabled** | `--hpa-aware=disabled` | Original behavior (compare replicas exactly) |

#### How It Works

1. Dorikin scans manifests for `HorizontalPodAutoscaler` resources
2. Extracts `scaleTargetRef` to identify managed Deployments/StatefulSets
3. When comparing those resources, `spec.replicas` is dynamically ignored

```bash
# Default: HPA awareness from manifests
dorikin scan ./manifests/

# Also check cluster for HPAs not in manifests
dorikin scan --hpa-aware=cluster ./manifests/

# Disable HPA awareness (original behavior)
dorikin scan --hpa-aware=disabled ./manifests/
```

#### Supported Scalable Resources

- Deployments
- StatefulSets
- ReplicaSets

#### Edge Cases Handled

| Scenario | Handling |
|----------|----------|
| HPA missing `minReplicas` | Defaults to 1 (Kubernetes default) |
| `autoscaling/v1` HPAs | Fully supported |
| `autoscaling/v2` HPAs | Fully supported |
| Resource not managed by HPA | Normal replica comparison |

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
| HPA-aware comparison | Skip `spec.replicas` when HPA targets the deployment | ✅ Implemented |
| Comprehensive unit tests | 213 tests with 80%+ coverage on core packages | ✅ Implemented |
| CI/CD pipeline | GitHub Actions with `make ci`, pre-push hooks | ✅ Implemented |
| Helm integration | `dorikin scan --helm ./chart` | 🚧 In Progress |
| Kustomize integration | `dorikin scan --kustomize ./overlay` | 🚧 In Progress |
| Quantity normalization | Treat `128Mi` = `134217728` = `128M` | Planned |
| Content-based array matching | Match array elements by key field (e.g., container name) | Planned |
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
