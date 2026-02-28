# bootchain-operator

A Kubernetes operator that makes service boot dependencies **declarative and automatic**, eliminating hand-written init containers and ad-hoc wait scripts.

## Features

- **Declarative dependencies** — Define which services a Deployment must wait for via a `BootDependency` custom resource
- **Automatic wiring** — The operator injects init logic so your Deployment only starts when dependencies are reachable (TCP health)
- **Per-dependency timeouts** — Configure wait timeouts per service (e.g. 60s for DB, 30s for cache)
- **Status and metrics** — Ready conditions, resolved dependency counts, and Prometheus metrics for observability

## How it works

1. You create a `BootDependency` with the **same name and namespace** as your Deployment.
2. You list the services (and ports) that must be reachable before the Deployment should start.
3. The operator ensures the Deployment is only rolled out when all dependencies are satisfied, and keeps status up to date.

No init containers to maintain — just a small CR and the operator does the rest.

## Prerequisites

- Kubernetes 1.24+
- [cert-manager](https://cert-manager.io/) (required for the operator's webhooks)

## Quick install

```bash
# Install cert-manager (required for webhooks)
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager --create-namespace \
  --set crds.enabled=true --wait

# Install bootchain-operator
helm install bootchain-operator oci://ghcr.io/user-cube/bootchain-operator/bootchain-operator \
  --namespace bootchain-operator-system --create-namespace --wait
```

## Example

Create a `BootDependency` that matches your Deployment name and namespace, and list the services to wait for:

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

The operator will ensure the `payments-api` Deployment in `default` only starts once `payments-db:5432` and `redis:6379` are reachable.

## Documentation

Full documentation is available at **https://user-cube.github.io/bootchain-operator**

- [Installation](https://user-cube.github.io/bootchain-operator/getting-started/installation/)
- [Quickstart](https://user-cube.github.io/bootchain-operator/getting-started/quickstart/)
- [API Reference](https://user-cube.github.io/bootchain-operator/reference/api/)
- [Helm Values](https://user-cube.github.io/bootchain-operator/reference/helm-values/)
- [Metrics](https://user-cube.github.io/bootchain-operator/reference/metrics/)

## License

Apache License 2.0
