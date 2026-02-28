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
   - If `httpPath` is set: performs an HTTP(S) request to `{httpScheme}://{target}:{port}{httpPath}`. The method defaults to `GET` (override with `httpMethod`). Custom headers can be injected via `httpHeaders`. The accepted status codes default to any `2xx`; override with `httpExpectedStatuses`. When `insecure: true`, TLS certificate verification is skipped
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

The init containers use the `ghcr.io/user-cube/bootchain-operator/minimal-tools` image — a custom minimal image that bundles `netcat`, `wget`, and `curl`. The polling command depends on whether `httpPath` is set and which advanced fields are in use:

- **TCP check** (default): `timeout {timeout} sh -c 'until nc -z {target} {port}; do sleep 1; done'`
- **HTTP/HTTPS check** (basic — `httpPath` set, no advanced fields): uses `wget --spider`. With `insecure: true`, adds `--no-check-certificate`
- **Advanced HTTP/HTTPS check** (`httpMethod`, `httpHeaders`, or `httpExpectedStatuses` set): switches to `curl`, which supports custom methods (`-X`), headers (`--header`), and status code extraction (`-w '%{http_code}'`). With `insecure: true`, adds `-k`

### Validating Webhook (`internal/webhook/v1alpha1`)

The `BootDependencyCustomValidator` fires on `CREATE` and `UPDATE` of any `BootDependency`:

1. Validates that each `spec.dependsOn` entry specifies **exactly one** of `service` or `host`
2. Builds a directed dependency graph from all `BootDependency` resources in the namespace (`service` entries only — `host` entries are external leaf nodes and cannot form a `BootDependency` cycle)
3. Adds the incoming resource to the graph
4. Runs a depth-first search (DFS) from the incoming resource's name
5. Rejects the request if a back-edge (cycle) is detected, including the full cycle path in the error message

## Init container image: minimal-tools

The init containers injected by the mutating webhook use a custom image — `ghcr.io/user-cube/bootchain-operator/minimal-tools` — instead of a generic `busybox`. This image is purpose-built to include exactly the tools needed for health probing:

| Tool | Used for |
|---|---|
| `netcat` (`nc`) | TCP connection checks (default probe) |
| `wget` | HTTP and HTTPS health checks (`httpPath`) |
| `curl` | Available for manual debugging inside init containers |

Using a dedicated image rather than a large general-purpose one keeps the image footprint small while providing all the probing primitives the operator needs. The image is versioned and published to GitHub Container Registry alongside the operator.

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
