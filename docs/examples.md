# Examples

Real-world patterns for using bootchain-operator. Each example includes the full YAML needed and explains what to expect.

---

## Dependency patterns

### Simple dependency

A single service waiting for one upstream.

```yaml
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: worker
  namespace: default
spec:
  dependsOn:
    - service: redis
      port: 6379
      timeout: 30s
```

When a `Deployment` named `worker` is created in the same namespace, the operator injects one init container that waits until `redis:6379` accepts TCP connections.

---

### Chain dependency (A → B → C)

Services that must start in sequence. Each `BootDependency` declares only its direct upstream.

```yaml
# database — no dependencies, starts immediately
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: database
  namespace: default
spec:
  dependsOn:
    - service: postgres
      port: 5432
      timeout: 120s
---
# api — waits for database
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: api
  namespace: default
spec:
  dependsOn:
    - service: database
      port: 8080
      timeout: 60s
---
# frontend — waits for api
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: frontend
  namespace: default
spec:
  dependsOn:
    - service: api
      port: 3000
      timeout: 60s
```

Start order enforced: `postgres` → `database` → `api` → `frontend`.

---

### Fan-in dependency (multiple upstreams)

A service that requires several independent services before starting.

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
      timeout: 120s
    - service: redis
      port: 6379
      timeout: 30s
    - service: kafka
      port: 9092
      timeout: 60s
```

Three init containers are injected. The application pod only starts once all three services are reachable.

---

### External host dependency

Use `host` instead of `service` when the dependency lives outside the cluster — a managed database, an external API, or any hostname/IP reachable from inside the cluster.

```yaml
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: payments-api
  namespace: default
spec:
  dependsOn:
    - host: db.example.com       # external managed database
      port: 5432
      timeout: 120s
    - service: redis             # in-cluster cache
      port: 6379
      timeout: 30s
```

`service` and `host` entries can be mixed freely. Each entry must specify **exactly one** of the two — the validating webhook rejects entries with both set or neither set.

!!! note
    `host` dependencies are resolved directly by the init container without DNS look-up via `svc.cluster.local`. They are also excluded from cycle detection since external hosts cannot form a `BootDependency` cycle.

---

### HTTP health check

Use `httpPath` to probe an HTTP endpoint instead of doing a raw TCP connection. The check performs an HTTP GET and requires a `2xx` response before the init container exits. This is useful when a service binds its port before it is fully initialised and exposes a dedicated health endpoint.

```yaml
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: frontend
  namespace: default
spec:
  dependsOn:
    - service: backend
      port: 8080
      httpPath: /healthz       # waits for HTTP 200 on http://backend:8080/healthz
      timeout: 60s
    - host: api.example.com    # external API with a health endpoint
      port: 443
      httpPath: /health
      timeout: 30s
    - service: postgres
      port: 5432               # no httpPath → plain TCP check
      timeout: 120s
```

`service` and `host` entries can freely mix TCP and HTTP checks within the same `BootDependency`.

---

### HTTPS health check

Use `httpScheme: https` together with `httpPath` to probe an HTTPS endpoint. By default TLS certificates are verified. Set `insecure: true` to skip verification (useful for services with self-signed certificates).

```yaml
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: frontend
  namespace: default
spec:
  dependsOn:
    - host: secure-api.example.com   # external HTTPS API — full TLS verification
      port: 443
      httpPath: /healthz
      httpScheme: https
      timeout: 60s
    - service: internal-api          # in-cluster service with self-signed cert
      port: 8443
      httpPath: /ready
      httpScheme: https
      insecure: true                 # skip TLS verification
      timeout: 30s
    - service: postgres              # plain TCP check — no httpPath
      port: 5432
      timeout: 120s
```

!!! note
    `insecure: true` only skips TLS certificate verification. The connection is still encrypted. Use it for services with self-signed or internal CA certificates that are not in the system trust store.

!!! warning
    Both `httpScheme` and `insecure` require `httpPath` to be set. The API server rejects resources that specify either field without `httpPath`.

---

### Circular dependency (rejected)

The validating webhook blocks cycles at admission time.

```yaml
# Apply svc-a first — succeeds
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: svc-a
  namespace: default
spec:
  dependsOn:
    - service: svc-b
      port: 8080
```

```yaml
# Apply svc-b — rejected
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: svc-b
  namespace: default
spec:
  dependsOn:
    - service: svc-a
      port: 8080
```

```
Error from server (Forbidden): admission webhook "vbootdependency-v1alpha1.kb.io" denied the request:
spec.dependsOn: Invalid value: ...: circular dependency detected: svc-b → svc-a → svc-b
```

---

## Common use cases

### Web application with database and cache

A typical three-tier application: PostgreSQL + Redis + app server.

=== "BootDependency"

    ```yaml
    apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
    kind: BootDependency
    metadata:
      name: web-app
      namespace: production
    spec:
      dependsOn:
        - service: postgres
          port: 5432
          timeout: 120s
        - service: redis
          port: 6379
          timeout: 30s
    ```

=== "Deployment"

    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: web-app        # must match BootDependency name
      namespace: production
    spec:
      replicas: 3
      selector:
        matchLabels:
          app: web-app
      template:
        metadata:
          labels:
            app: web-app
        spec:
          containers:
            - name: app
              image: my-org/web-app:1.0.0
              env:
                - name: DB_HOST
                  value: postgres
                - name: CACHE_HOST
                  value: redis
    ```

After applying both, the operator injects init containers automatically:

```bash
kubectl get deployment web-app \
  -o jsonpath='{.spec.template.spec.initContainers[*].name}'
# wait-for-postgres wait-for-redis
```

---

### Message queue consumer

A worker that must wait for both the queue broker and its database before consuming messages.

=== "BootDependency"

    ```yaml
    apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
    kind: BootDependency
    metadata:
      name: order-consumer
      namespace: default
    spec:
      dependsOn:
        - service: rabbitmq
          port: 5672
          timeout: 90s
        - service: orders-db
          port: 5432
          timeout: 120s
    ```

=== "Deployment"

    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: order-consumer
      namespace: default
    spec:
      replicas: 2
      selector:
        matchLabels:
          app: order-consumer
      template:
        metadata:
          labels:
            app: order-consumer
        spec:
          containers:
            - name: consumer
              image: my-org/order-consumer:1.0.0
    ```

---

### Microservices with staged rollout

In a microservices architecture, declare dependencies explicitly to enforce a safe rollout order.

```yaml
# infrastructure layer
---
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: auth-service
  namespace: apps
spec:
  dependsOn:
    - service: auth-db
      port: 5432
      timeout: 120s
---
# business logic layer — waits for auth
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: order-service
  namespace: apps
spec:
  dependsOn:
    - service: auth-service
      port: 8080
      timeout: 60s
    - service: order-db
      port: 5432
      timeout: 120s
---
# gateway layer — waits for all business services
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: api-gateway
  namespace: apps
spec:
  dependsOn:
    - service: auth-service
      port: 8080
      timeout: 60s
    - service: order-service
      port: 8080
      timeout: 60s
```

---

## Troubleshooting

### Pod stuck in `Init:0/N`

The init containers are running but the upstream services are not yet reachable.

```bash
# Check which init containers are waiting
kubectl get pod -l app=my-app

# NAME                     READY   STATUS     RESTARTS   AGE
# my-app-xxx-yyy           0/1     Init:0/2   0          45s

# Describe the pod to see which dependency is blocking
kubectl describe pod -l app=my-app
```

Look for lines like:

```
Init Containers:
  wait-for-payments-db:
    State: Running
    ...
  wait-for-redis:
    State: Waiting
      Reason: PodInitializing
```

The first init container runs serially — if `wait-for-payments-db` is still running, it means `payments-db:5432` is not yet accepting connections.

---

### BootDependency shows `Ready: False`

```bash
kubectl get bootdependency my-app
# NAME     READY   RESOLVED   AGE
# my-app   False   1/2        2m
```

`1/2` means one dependency is reachable and one is not. Check which:

```bash
kubectl describe bootdependency my-app
```

Look at the `Conditions` and `Events` sections. The controller emits an event per unreachable dependency.

---

### Service exists but dependency never becomes ready

Verify the service name and port match exactly:

```bash
# Check the service exists and its port
kubectl get service payments-db
kubectl get service payments-db -o jsonpath='{.spec.ports[*].port}'
```

Common mistakes:

| Mistake | Symptom |
|---|---|
| Wrong service name (typo) | Init container loops forever |
| Wrong port | Init container loops forever |
| Service in different namespace | Init container loops forever (cross-namespace not supported) |
| `timeout` too short | Pod restarts repeatedly |

!!! note
    `BootDependency` and the `Deployment` must be in the **same namespace**. The operator looks up the `BootDependency` by matching its name to the `Deployment` name within the same namespace.

---

### Admission webhook rejected a valid resource

If you get an unexpected rejection:

```bash
# Check webhook is running
kubectl get pods -n bootchain-operator-system

# Check webhook logs
kubectl logs -n bootchain-operator-system -l app.kubernetes.io/name=bootchain-operator
```

If the cycle detection false-positives, describe all `BootDependency` objects in the namespace to visualise the graph:

```bash
kubectl get bootdependency -A \
  -o custom-columns='NAMESPACE:.metadata.namespace,NAME:.metadata.name,SERVICES:.spec.dependsOn[*].service,HOSTS:.spec.dependsOn[*].host'
```

---

### Webhook not injecting init containers

If a `Deployment` exists but no init containers were injected:

1. Confirm a `BootDependency` with the **same name** exists in the **same namespace**
2. Check the operator pod is running and webhooks are registered:

```bash
kubectl get mutatingwebhookconfigurations
kubectl get validatingwebhookconfigurations
```

3. If using `ENABLE_WEBHOOKS=false` (local dev mode), webhooks are disabled by design — use `task run-with-webhook` instead.
