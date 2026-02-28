# API Reference

## BootDependency

**Group:** `core.bootchain.ruicoelho.dev`
**Version:** `v1alpha1`
**Scope:** Namespaced

A `BootDependency` declares the set of TCP services that must be reachable before a `Deployment` with the same name (in the same namespace) is allowed to start. The operator injects `wait-for-*` init containers automatically.

### Spec

```yaml
spec:
  dependsOn:
    - service: <string>        # required
      port: <integer>          # required
      timeout: <string>        # optional, default: "60s"
```

#### `spec.dependsOn`

List of service dependencies. At least one entry is required.

| Field | Type | Required | Description |
|---|---|---|---|
| `service` | string | yes | Name of the Kubernetes `Service` to wait for |
| `port` | integer (1–65535) | yes | TCP port that must accept connections |
| `timeout` | duration string | no | How long to wait per dependency. Defaults to `60s` |

### Status

The operator updates the status after each reconciliation loop.

| Field | Type | Description |
|---|---|---|
| `conditions` | []Condition | Standard Kubernetes conditions. The `Ready` condition reflects overall TCP reachability |
| `resolvedDependencies` | string | Human-readable summary, e.g. `"2/3"` |

#### Ready condition

| Status | Reason | Description |
|---|---|---|
| `True` | `AllDependenciesReady` | All declared services are TCP-reachable |
| `False` | `DependenciesNotReady` | One or more services are not reachable |

### Printer columns

```bash
kubectl get bootdependencies
```

```
NAME           READY   RESOLVED   AGE
payments-api   True    2/2        5m
svc-a          False   0/1        1m
```

### Example

```yaml
apiVersion: core.bootchain.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: payments-api
  namespace: default
spec:
  dependsOn:
    - service: payments-db
      port: 5432
      timeout: 60s
    - service: redis
      port: 6379
      timeout: 30s
```

### Naming convention

The `BootDependency` name must match the `Deployment` name it targets. The operator looks up a `BootDependency` whose `metadata.name` equals the Deployment's `metadata.name` in the same namespace.

```
Deployment: payments-api  →  BootDependency: payments-api  (same namespace)
```

### Injected init containers

For each entry in `spec.dependsOn`, the mutating webhook prepends an init container to the Deployment's pod template:

```yaml
initContainers:
- name: wait-for-payments-db
  image: busybox:1.36
  imagePullPolicy: IfNotPresent
  command:
  - sh
  - -c
  - "echo 'Waiting for payments-db:5432...'; until nc -z payments-db 5432; do sleep 1; done; echo 'payments-db:5432 is ready'"
```

Init containers are injected idempotently — re-applying a Deployment will not duplicate them.
