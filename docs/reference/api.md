# API Reference

## BootDependency

**Group:** `core.bootchain-operator.ruicoelho.dev`
**Version:** `v1alpha1`
**Scope:** Namespaced

A `BootDependency` declares the set of services that must be reachable before a `Deployment` with the same name (in the same namespace) is allowed to start. The operator injects `wait-for-*` init containers automatically. Each dependency can be probed via a raw TCP check or an HTTP health endpoint.

### Spec

```yaml
spec:
  dependsOn:
    - service: <string>        # exactly one of service or host is required
      port: <integer>          # required
      httpPath: <string>       # optional, enables HTTP check (e.g. /healthz)
      timeout: <string>        # optional, default: "60s"

    - host: <string>           # use for external dependencies (DNS / IP)
      port: <integer>
      httpPath: <string>       # optional
      timeout: <string>
```

#### `spec.dependsOn`

List of dependencies. At least one entry is required. Each entry must specify **exactly one** of `service` or `host`.

| Field | Type | Required | Description |
|---|---|---|---|
| `service` | string | one of `service`/`host` | Name of the Kubernetes `Service` in the same namespace to wait for |
| `host` | string | one of `service`/`host` | External hostname or IP address to wait for (e.g. a managed database, an external API) |
| `port` | integer (1–65535) | yes | TCP port to probe |
| `httpPath` | string | no | HTTP path to probe instead of a raw TCP check (e.g. `/healthz`). Must start with `/`. When set, the check performs an HTTP GET and requires a `2xx` response. When omitted, a plain TCP connection check is used |
| `timeout` | duration string | no | How long to wait per dependency. Defaults to `60s` |

### Status

The operator updates the status after each reconciliation loop.

| Field | Type | Description |
|---|---|---|
| `conditions` | []Condition | Standard Kubernetes conditions. The `Ready` condition reflects overall reachability |
| `resolvedDependencies` | string | Human-readable summary, e.g. `"2/3"` |

#### Ready condition

| Status | Reason | Description |
|---|---|---|
| `True` | `AllDependenciesReady` | All declared dependencies are reachable |
| `False` | `DependenciesNotReady` | One or more dependencies are not reachable |

### Printer columns

```bash
kubectl get bootdependencies
```

```
NAME           READY   RESOLVED   AGE
payments-api   True    2/2        5m
svc-a          False   0/1        1m
```

### Examples

In-cluster services:

```yaml
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
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

External dependencies (outside the cluster):

```yaml
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: payments-api
  namespace: default
spec:
  dependsOn:
    - host: db.example.com
      port: 5432
      timeout: 120s
    - service: redis
      port: 6379
      timeout: 30s
```

HTTP health check (probe an endpoint instead of raw TCP):

```yaml
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: payments-api
  namespace: default
spec:
  dependsOn:
    - service: payments-db
      port: 5432
      timeout: 60s
    - service: auth-service
      port: 8080
      httpPath: /healthz
      timeout: 30s
    - host: api.example.com
      port: 443
      httpPath: /health
      timeout: 60s
```

### Naming convention

The `BootDependency` name must match the `Deployment` name it targets. The operator looks up a `BootDependency` whose `metadata.name` equals the Deployment's `metadata.name` in the same namespace.

```
Deployment: payments-api  →  BootDependency: payments-api  (same namespace)
```

### Injected init containers

For each entry in `spec.dependsOn`, the mutating webhook prepends an init container to the Deployment's pod template. The target address is the `service` name (resolved via cluster DNS) or the `host` value (used directly).

**TCP check** (default, when `httpPath` is omitted):

```yaml
initContainers:
- name: wait-for-payments-db
  image: busybox:1.36
  imagePullPolicy: IfNotPresent
  command:
  - sh
  - -c
  - "echo 'Waiting for payments-db:5432...'; timeout 60s sh -c 'until nc -z payments-db 5432; do sleep 1; done'; echo 'payments-db:5432 is ready'"
```

**HTTP check** (when `httpPath` is set):

```yaml
initContainers:
- name: wait-for-auth-service
  image: busybox:1.36
  imagePullPolicy: IfNotPresent
  command:
  - sh
  - -c
  - "echo 'Waiting for http://auth-service:8080/healthz...'; timeout 30s sh -c 'until wget -q --spider http://auth-service:8080/healthz; do sleep 1; done'; echo 'http://auth-service:8080/healthz is ready'"
```

Init containers are injected idempotently — re-applying a Deployment will not duplicate them.
