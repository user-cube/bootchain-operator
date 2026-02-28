# Architecture

## Overview

bootchain-operator is a Kubernetes operator built with [Kubebuilder](https://book.kubebuilder.io/) and [controller-runtime](https://pkg.go.dev/sigs.k8s.io/controller-runtime). It consists of three components that work together:

```
┌─────────────────────────────────────────────────────────┐
│                   Kubernetes API Server                 │
└──────────┬──────────────────────┬───────────────────────┘
           │ watch                │ admission review
           ▼                      ▼
┌─────────────────┐    ┌─────────────────────────────────┐
│   Controller    │    │         Webhook Server          │
│                 │    │                                 │
│  Reconcile loop │    │  MutatingWebhook  (Deployment)  │
│  TCP/HTTP check │    │  ValidatingWebhook (BootDep)    │
│  Status update  │    │                                 │
└─────────────────┘    └─────────────────────────────────┘
```

## Components

### Controller (`internal/controller`)

The `BootDependencyReconciler` runs a reconciliation loop that:

1. Fetches the `BootDependency` resource
2. Probes each declared dependency (3-second timeout per check):
   - If `httpPath` is set: performs an HTTP(S) GET to `{httpScheme}://{target}:{port}{httpPath}` and requires a `2xx` response. The scheme defaults to `http`; set `httpScheme: https` for HTTPS. When `insecure: true`, TLS certificate verification is skipped (useful for self-signed certificates)
   - Otherwise: TCP-dials the address — `service` entries resolve as `{service}.{namespace}.svc.cluster.local:{port}`, `host` entries are dialled as `{host}:{port}`
3. Updates `status.resolvedDependencies` (e.g. `"2/3"`) and the `Ready` condition
4. Emits Kubernetes events for reachable/unreachable dependencies
5. Records Prometheus metrics
6. Requeues after **30s** if all ready, **10s** if not

### Mutating Webhook (`internal/webhook/v1`)

The `DeploymentCustomDefaulter` fires on `CREATE` and `UPDATE` of any `apps/v1 Deployment`:

1. Looks up a `BootDependency` with the same `name` and `namespace` as the Deployment
2. If found, prepends a `wait-for-{target}` init container for each `spec.dependsOn` entry
3. The init container target is the `service` name (cluster DNS) or `host` value (used directly)
4. Injection is **idempotent** — existing init containers with the same name are skipped

The init containers use `busybox:1.36`. The polling command depends on whether `httpPath` is set:

- **TCP check** (default): `timeout {timeout} sh -c 'until nc -z {target} {port}; do sleep 1; done'`
- **HTTP check**: `timeout {timeout} sh -c 'until wget -q --spider http://{target}:{port}{httpPath}; do sleep 1; done'`
- **HTTPS check** (`httpScheme: https`): same as HTTP but with `https://`. When `insecure: true`, `--no-check-certificate` is added to skip TLS verification

### Validating Webhook (`internal/webhook/v1alpha1`)

The `BootDependencyCustomValidator` fires on `CREATE` and `UPDATE` of any `BootDependency`:

1. Validates that each `spec.dependsOn` entry specifies **exactly one** of `service` or `host`
2. Builds a directed dependency graph from all `BootDependency` resources in the namespace (`service` entries only — `host` entries are external leaf nodes and cannot form a `BootDependency` cycle)
3. Adds the incoming resource to the graph
4. Runs a depth-first search (DFS) from the incoming resource's name
5. Rejects the request if a back-edge (cycle) is detected, including the full cycle path in the error message

## TLS and cert-manager

The webhook server requires TLS. In production (Helm install), cert-manager automatically provisions a self-signed `Certificate` and injects the CA bundle into both `MutatingWebhookConfiguration` and `ValidatingWebhookConfiguration` via the `cert-manager.io/inject-ca-from` annotation.

```
cert-manager Issuer (self-signed)
       │
       └─► Certificate → Secret (bootchain-operator-webhook-tls)
                                  │
                          Deployment (volume mount)
                                  │
                          Webhook Server (:9443)
```

## Data flow: init container injection

```
kubectl apply -f deployment.yaml
        │
        ▼
API Server → MutatingWebhookConfiguration
        │
        ▼
bootchain-operator webhook handler
        │
        ├─ GET BootDependency (same name, same namespace)
        │         │
        │    found? ──yes──► inject wait-for-* init containers
        │         │
        │    not found? ──► pass through unchanged
        │
        ▼
API Server stores Deployment with injected init containers
```

## Data flow: cycle detection

```
kubectl apply -f bootdependency.yaml
        │
        ▼
API Server → ValidatingWebhookConfiguration
        │
        ▼
bootchain-operator webhook handler
        │
        ├─ LIST all BootDependencies in namespace
        ├─ Build directed graph (name → [dependsOn services])
        ├─ Add incoming resource to graph
        ├─ DFS from incoming resource's name
        │
        ├─ cycle found? ──► 403 Forbidden with cycle path
        └─ no cycle? ──────► 200 Allowed
```
