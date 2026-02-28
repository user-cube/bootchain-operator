# API Reference

## BootDependency

**Group:** `core.bootchain-operator.ruicoelho.dev`
**Version:** `v1alpha1`
**Scope:** Namespaced

A `BootDependency` declares the set of services that must be reachable before a `Deployment` with the same name (in the same namespace) is allowed to start. The operator injects `wait-for-*` init containers automatically. Each dependency can be probed via a raw TCP check, an HTTP health endpoint, or an HTTPS health endpoint.

### Spec

```yaml
spec:
  dependsOn:
    - service: <string>              # exactly one of service or host is required
      port: <integer>                # required
      httpPath: <string>             # optional, enables HTTP(S) check (e.g. /healthz)
      httpScheme: <string>           # optional, "http" or "https" (default: "http")
      insecure: <boolean>            # optional, skip TLS verification (default: false)
      httpMethod: <string>           # optional, HTTP verb (default: "GET")
      httpHeaders:                   # optional, custom request headers
        - name: <string>
          value: <string>
      httpExpectedStatuses: [<int>]  # optional, accepted status codes (default: 2xx)
      timeout: <string>              # optional, default: "60s"

    - host: <string>                 # use for external dependencies (DNS / IP)
      port: <integer>
      httpPath: <string>
      httpScheme: <string>
      insecure: <boolean>
      httpMethod: <string>
      httpHeaders:
        - name: <string>
          value: <string>
      httpExpectedStatuses: [<int>]
      timeout: <string>
```

#### `spec.dependsOn`

List of dependencies. At least one entry is required. Each entry must specify **exactly one** of `service` or `host`.

| Field | Type | Required | Description |
|---|---|---|---|
| `service` | string | one of `service`/`host` | Name of the Kubernetes `Service` in the same namespace to wait for |
| `host` | string | one of `service`/`host` | External hostname or IP address to wait for (e.g. a managed database, an external API) |
| `port` | integer (1–65535) | yes | TCP port to probe |
| `httpPath` | string | no | HTTP(S) path to probe instead of a raw TCP check (e.g. `/healthz`). Must start with `/`. When set, the check performs an HTTP GET and requires a `2xx` response. When omitted, a plain TCP connection check is used |
| `httpScheme` | `http` \| `https` | no | URL scheme to use when `httpPath` is set. Defaults to `http`. Requires `httpPath` to be set |
| `insecure` | boolean | no | When `true`, TLS certificate verification is skipped for HTTPS probes (accepts self-signed certificates). Defaults to `false`. Requires `httpPath` to be set |
| `httpMethod` | string | no | HTTP verb to use for the probe (e.g. `GET`, `POST`, `HEAD`). Must be uppercase. Defaults to `GET`. Requires `httpPath` to be set |
| `httpHeaders` | `[{name, value}]` | no | List of custom HTTP headers to include in the probe request (e.g. `Authorization`). Requires `httpPath` to be set |
| `httpExpectedStatuses` | `[]integer` | no | List of HTTP status codes accepted as healthy. Defaults to any `2xx` (200–299). Useful for endpoints that return `204 No Content`. Requires `httpPath` to be set |
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

Advanced HTTP check (custom method, headers, and accepted status codes):

```yaml
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: payments-api
  namespace: default
spec:
  dependsOn:
    - service: auth-service
      port: 8080
      httpPath: /healthz
      httpMethod: POST                       # use POST instead of GET
      httpHeaders:
        - name: Authorization
          value: Bearer my-token             # required by the endpoint
        - name: X-Probe-Source
          value: bootchain
      httpExpectedStatuses: [200, 204]       # accept 200 or 204
      timeout: 30s
```

HTTPS health check (with TLS certificate verification):

```yaml
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: payments-api
  namespace: default
spec:
  dependsOn:
    - host: secure-api.example.com   # external HTTPS API
      port: 443
      httpPath: /healthz
      httpScheme: https
      timeout: 60s
    - service: internal-https-svc    # in-cluster service with self-signed cert
      port: 8443
      httpPath: /ready
      httpScheme: https
      insecure: true                 # skip TLS verification
      timeout: 30s
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
  image: ghcr.io/user-cube/bootchain-operator/minimal-tools:1.2.0
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
  image: ghcr.io/user-cube/bootchain-operator/minimal-tools:1.2.0
  imagePullPolicy: IfNotPresent
  command:
  - sh
  - -c
  - "echo 'Waiting for http://auth-service:8080/healthz...'; timeout 30s sh -c 'until wget -q --spider http://auth-service:8080/healthz; do sleep 1; done'; echo 'http://auth-service:8080/healthz is ready'"
```

**HTTPS check** (when `httpPath` and `httpScheme: https` are set):

```yaml
initContainers:
- name: wait-for-secure-api
  image: ghcr.io/user-cube/bootchain-operator/minimal-tools:1.2.0
  imagePullPolicy: IfNotPresent
  command:
  - sh
  - -c
  - "echo 'Waiting for https://secure-api:443/healthz...'; timeout 60s sh -c 'until wget -q --spider https://secure-api:443/healthz; do sleep 1; done'; echo 'https://secure-api:443/healthz is ready'"
```

With `insecure: true`, `--no-check-certificate` is added to the `wget` command to skip TLS verification.

**Advanced HTTP check** (when `httpMethod`, `httpHeaders`, or `httpExpectedStatuses` are set — switches to `curl`):

```yaml
initContainers:
- name: wait-for-auth-service
  image: ghcr.io/user-cube/bootchain-operator/minimal-tools:1.2.0
  imagePullPolicy: IfNotPresent
  command:
  - sh
  - -c
  - "echo 'Waiting for http://auth-service:8080/healthz...'; timeout 30s sh -c 'until STATUS=$(curl -s -o /dev/null -w \"%{http_code}\" -X POST --header \"Authorization: Bearer my-token\" http://auth-service:8080/healthz) && case \"$STATUS\" in 200|204) true ;; *) false ;; esac; do sleep 1; done'; echo 'http://auth-service:8080/healthz is ready'"
```

`wget` is used by default for simple HTTP(S) probes. When any of `httpMethod`, `httpHeaders`, or `httpExpectedStatuses` are set, the init container switches to `curl` which supports all three options.

Init containers are injected idempotently — re-applying a Deployment will not duplicate them.
