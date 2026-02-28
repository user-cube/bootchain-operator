# Quickstart

This guide walks through a complete example: declaring that `payments-api` depends on `payments-db` (PostgreSQL) and `redis`, and verifying that init containers are automatically injected.

## 1. Create a BootDependency

```yaml title="payments-api-deps.yaml"
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

```bash
kubectl apply -f payments-api-deps.yaml
```

Check the status:

```bash
kubectl get bootdependency payments-api
```

```
NAME           READY   RESOLVED   AGE
payments-api   False   0/2        5s
```

The `READY` column reflects reachability. `0/2` means neither dependency is reachable yet — the services don't exist. That's expected.

## 2. Deploy the application

Create a Deployment with the **same name** as the BootDependency, in the same namespace:

```yaml title="payments-api-deploy.yaml"
apiVersion: apps/v1
kind: Deployment
metadata:
  name: payments-api
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: payments-api
  template:
    metadata:
      labels:
        app: payments-api
    spec:
      containers:
      - name: app
        image: nginx
```

```bash
kubectl apply -f payments-api-deploy.yaml
```

## 3. Verify init containers were injected

```bash
kubectl get deployment payments-api \
  -o jsonpath='{.spec.template.spec.initContainers[*].name}'
```

```
wait-for-payments-db wait-for-redis
```

The operator automatically injected two init containers. Describe the pod to see them in action:

```bash
kubectl describe pod -l app=payments-api
```

The pod will stay in `Init:0/2` until both `payments-db:5432` and `redis:6379` are reachable.

## 4. External host dependency

Use `host` instead of `service` when the dependency lives outside the cluster:

```yaml title="payments-api-deps-external.yaml"
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

The init container for the `host` entry dials `db.example.com:5432` directly (no cluster DNS suffix). `service` and `host` entries can be mixed freely.

## 5. HTTP health check

Use `httpPath` to probe an HTTP endpoint instead of a raw TCP connection. Useful when a service binds its port before it is fully ready and exposes a `/healthz` or `/ready` endpoint:

```yaml title="payments-api-deps-http.yaml"
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
      httpPath: /healthz      # HTTP GET — waits for 2xx
      timeout: 30s
```

The injected init container for `auth-service` will use `wget --spider` instead of `nc -z`, and only exit once it receives a `2xx` response.

## 6. HTTPS health check

Add `httpScheme: https` to probe an HTTPS endpoint. Certificate verification is on by default — set `insecure: true` to accept self-signed certificates:

```yaml title="payments-api-deps-https.yaml"
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
    - host: secure-api.example.com   # external HTTPS API
      port: 443
      httpPath: /healthz
      httpScheme: https              # use HTTPS instead of HTTP
      timeout: 30s
    - service: internal-svc          # in-cluster service with self-signed cert
      port: 8443
      httpPath: /ready
      httpScheme: https
      insecure: true                 # skip TLS certificate verification
      timeout: 30s
```

## 7. Circular dependency detection

The validating webhook blocks dependency cycles. Try creating a cycle:

```yaml
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

```bash
kubectl apply -f svc-a.yaml
# bootdependency.core.bootchain-operator.ruicoelho.dev/svc-a created
```

Now try creating `svc-b` that depends back on `svc-a`:

```bash
kubectl apply -f - <<EOF
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: svc-b
  namespace: default
spec:
  dependsOn:
  - service: svc-a
    port: 8080
EOF
```

```
Error from server (Forbidden): admission webhook "vbootdependency-v1alpha1.kb.io" denied the request:
spec.dependsOn: Invalid value: ...: circular dependency detected: svc-b → svc-a → svc-b
```

The cycle is blocked with a clear error message.

## Next steps

- See more patterns (fan-in, chains, external hosts, HTTP/HTTPS checks) in the [Examples](../examples.md)
- Learn about all available fields in the [API Reference](../reference/api.md)
- Configure the Helm chart with [Helm Values](../reference/helm-values.md)
- Monitor the operator with [Metrics](../reference/metrics.md)
