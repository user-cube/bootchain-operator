# bootchain-operator

**bootchain-operator** is a Kubernetes operator that makes service boot dependencies declarative and automatic, eliminating hand-written init containers.

## The problem

In Kubernetes, services often need other services to be ready before they can start. The traditional solution is to manually write init containers for every Deployment:

```yaml
initContainers:
- name: wait-for-postgres
  image: busybox
  command: ['sh', '-c', 'until nc -z postgres 5432; do sleep 1; done']
- name: wait-for-redis
  image: busybox
  command: ['sh', '-c', 'until nc -z redis 6379; do sleep 1; done']
```

This is repetitive, easy to get wrong, and scattered across many manifests.

## The solution

With bootchain-operator, you declare dependencies once in a `BootDependency` resource:

```yaml
apiVersion: core.bootchain-operator.ruicoelho.dev/v1alpha1
kind: BootDependency
metadata:
  name: payments-api
  namespace: default
spec:
  dependsOn:
    - service: payments-db   # in-cluster Kubernetes Service
      port: 5432
    - host: cache.example.com  # external host (DNS / IP)
      port: 6379
```

The operator automatically injects the correct init containers into any `Deployment` with the same name, in the same namespace. No more boilerplate.

## Features

- **Automatic init container injection** — a mutating webhook injects `wait-for-*` init containers into matching Deployments
- **In-cluster and external dependencies** — use `service` for Kubernetes Services in the same namespace, or `host` for external hostnames and IP addresses
- **Circular dependency detection** — a validating webhook blocks any `BootDependency` that would create a dependency cycle
- **Status tracking** — the controller continuously checks TCP reachability and updates `status.resolvedDependencies` (e.g. `2/3`) and `status.conditions`
- **Prometheus metrics** — exposes reconciliation counters, duration histograms, and per-resource dependency gauges
- **Helm chart** — production-ready chart with cert-manager TLS, leader election, and optional ServiceMonitor

## Quick links

- [Installation](getting-started/installation.md)
- [Quickstart](getting-started/quickstart.md)
- [API Reference](reference/api.md)
- [Helm Values](reference/helm-values.md)
- [Metrics](reference/metrics.md)
