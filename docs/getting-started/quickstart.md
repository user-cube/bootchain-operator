# Quickstart

This guide walks through a complete example: declaring that `payments-api` depends on `payments-db` (PostgreSQL) and `redis`, and verifying that init containers are automatically injected.

## 1. Create a BootDependency

```yaml title="payments-api-deps.yaml"
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

The `READY` column reflects TCP reachability. `0/2` means neither dependency is reachable yet — the services don't exist. That's expected.

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

## 4. Circular dependency detection

The validating webhook blocks dependency cycles. Try creating a cycle:

```yaml
apiVersion: core.bootchain.ruicoelho.dev/v1alpha1
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
# bootdependency.core.bootchain.ruicoelho.dev/svc-a created
```

Now try creating `svc-b` that depends back on `svc-a`:

```bash
kubectl apply -f - <<EOF
apiVersion: core.bootchain.ruicoelho.dev/v1alpha1
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

- Learn about all available fields in the [API Reference](../reference/api.md)
- Configure the Helm chart with [Helm Values](../reference/helm-values.md)
- Monitor the operator with [Metrics](../reference/metrics.md)
