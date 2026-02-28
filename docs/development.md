# Development

## Prerequisites

| Tool | Purpose |
|---|---|
| Go 1.25+ | Build the operator |
| Docker | Build container images |
| kubectl | Interact with the cluster |
| Helm 3.10+ | Install / test the chart |
| task (Taskfile) | Developer workflow shortcuts |
| kubebuilder | Scaffold new APIs and webhooks |

Install `task`:
```bash
brew install go-task
```

## Repository layout

```
bootchain-operator/
├── api/v1alpha1/               # CRD Go types and deepcopy generation
│   ├── bootdependency_types.go # BootDependency spec / status structs
│   ├── groupversion_info.go    # API group registration
│   └── zz_generated.deepcopy.go  # generated — do not edit
├── cmd/
│   └── main.go                 # Entrypoint: flag parsing + manager setup
├── config/                     # Kustomize manifests (managed by kubebuilder)
│   ├── crd/bases/              # Generated CRD YAML — do not edit
│   ├── rbac/                   # Generated RBAC rules — do not edit
│   └── webhook/                # Generated webhook manifest — do not edit
├── charts/bootchain-operator/  # Helm chart for cluster deployment
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── crds/               # CRD (inline, duplicated from config/crd for Helm)
│       ├── deployment.yaml
│       ├── rbac.yaml
│       ├── webhook.yaml
│       ├── certmanager.yaml    # Issuer + Certificate for webhook TLS
│       └── servicemonitor.yaml # Prometheus ServiceMonitor (optional)
├── docs/                       # MkDocs documentation source
├── hack/                       # Dev helper scripts (certs, webhook registration)
├── internal/
│   ├── controller/
│   │   ├── bootdependency_controller.go  # Reconciliation loop
│   │   └── metrics.go                    # Custom Prometheus metrics
│   └── webhook/
│       ├── v1/             # Mutating webhook — injects init containers into Deployments
│       └── v1alpha1/       # Validating webhook — circular dependency detection
├── test/e2e/               # End-to-end tests
├── Makefile                # Kubebuilder scaffold — codegen, build, test, lint
├── Taskfile.yml            # Developer workflow — delegates to make + owns local dev tasks
└── mkdocs.yml              # Documentation site configuration
```

### Key areas in detail

#### `api/v1alpha1/`

The source of truth for the `BootDependency` API surface. This is where you define the spec fields, status fields, and validation markers (`// +kubebuilder:validation:...`). After any change here, run `task generate` to regenerate `zz_generated.deepcopy.go` and the CRD YAML.

The API group is `core.bootchain-operator.ruicoelho.dev`. The domain is defined in `PROJECT` and `api/v1alpha1/groupversion_info.go`.

#### `cmd/main.go`

The binary entrypoint. Sets up the controller-runtime manager, registers the controller and both webhooks, configures TLS for the webhook server, and wires up health/readiness probes. Flags are parsed here — see `--help` for the full list.

#### `config/`

Owned entirely by Kubebuilder tooling (`make manifests`). Contains Kustomize bases for the CRD, RBAC ClusterRoles, and webhook configurations. **Do not edit these files manually** — they are overwritten on every `make manifests` run. Use them as a reference or as input to Kustomize overlays for GitOps deployments.

#### `internal/controller/`

The reconciliation loop (`bootdependency_controller.go`) is the core of the operator. On each reconcile it:

1. Fetches the `BootDependency` object
2. Dials each dependency's `service:port` with a TCP connection
3. Updates the `Ready` condition and `resolvedDependencies` status field
4. Emits Kubernetes events

`metrics.go` registers four custom Prometheus counters/gauges/histograms via controller-runtime's shared metrics registry — they are picked up automatically by the `/metrics` endpoint.

#### `internal/webhook/`

Two separate packages, one per API version being intercepted:

- **`v1/`** — Mutating webhook on `apps/v1 Deployment`. Looks up a `BootDependency` with the same name in the same namespace and, if found, injects one `initContainer` per declared dependency using a `netcat`-based TCP probe.
- **`v1alpha1/`** — Validating webhook on `BootDependency` CREATE/UPDATE. Builds the full dependency graph for the namespace and runs a DFS cycle-detection algorithm before admitting the object.

#### `charts/bootchain-operator/`

The production Helm chart. The CRD template is a copy of the generated CRD from `config/crd/bases/` — keep them in sync when the API changes (run `task generate` then copy). The chart integrates with cert-manager via the `cert-manager.io/inject-ca-from` annotation on the webhook configurations, which eliminates the need to manage CA bundles manually.

#### `hack/`

Shell scripts for local development only — none of these are included in the Docker image:

| Script | Purpose |
|---|---|
| `gen-dev-certs.sh` | Generates a self-signed CA + TLS cert/key for local webhook dev |
| `register-dev-webhook.sh` | Registers `MutatingWebhookConfiguration` and `ValidatingWebhookConfiguration` pointing at the local process |

#### `Makefile` vs `Taskfile.yml`

The `Makefile` is the Kubebuilder scaffold and owns code generation, building, testing, and linting. It should not be customised — Kubebuilder may overwrite parts of it on `kubebuilder init` or plugin upgrades.

`Taskfile.yml` is the developer-facing interface. It delegates to `make` for codegen/build/test targets and directly owns everything else: local cluster setup, webhook dev workflow, Helm, and docs. Run `task --list` to see all available tasks.

### What lives where

| Area | Owned by | Notes |
|---|---|---|
| `api/` | Kubebuilder + developer | Edit types here; run `task generate` after |
| `config/` | Kubebuilder (`make manifests`) | Never edit manually |
| `internal/controller/` | Developer | Reconciliation logic and metrics |
| `internal/webhook/` | Developer | Admission webhook handlers |
| `charts/` | Developer | Helm chart; CRD kept in sync with `config/crd/bases/` |
| `hack/` | Developer | Local dev scripts; not shipped in the image |
| `Makefile` | Kubebuilder scaffold | Codegen targets; avoid adding custom tasks here |
| `Taskfile.yml` | Developer | All local dev / Helm / docs tasks |

## Common tasks

```bash
task --list         # show all available tasks
task build          # compile the operator binary
task test           # run unit and controller tests
task lint           # run golangci-lint
task generate       # regenerate DeepCopy methods and CRD manifests
task fmt            # format Go source files
task vet            # run go vet
```

## Running locally (controller only)

```bash
task install-crd    # apply CRD to current cluster
task run            # run controller with ENABLE_WEBHOOKS=false
```

## Running locally with webhooks

Webhooks require a valid TLS certificate reachable from the cluster. The scripts in `hack/` automate the setup for a local Colima/k3s cluster:

```bash
task webhook-setup        # generates certs + registers webhooks + labels namespace
task webhook-proxy        # socat proxy: TCP4:9444 → TCP6:[::1]:9443 (Colima networking)
task run-with-webhook     # start operator with webhook server
```

!!! note "Colima networking"
    When using Colima with a self-managed k3s cluster, the Mac host is reachable from the cluster at `192.168.64.1` (bridge100 interface). Always set `HOST_IP=192.168.64.1` when registering webhooks:
    ```bash
    HOST_IP=192.168.64.1 task webhook-setup
    ```

## Code generation

After modifying types in `api/v1alpha1/`, regenerate:

```bash
task generate   # regenerates zz_generated.deepcopy.go
                # and config/crd/bases/*.yaml
```

!!! warning
    Never edit `zz_generated.deepcopy.go` or `config/crd/bases/*.yaml` manually — they are overwritten by `make generate manifests`.

## Testing

Tests use [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) with `envtest` (a real API server + etcd, no cluster needed).

```bash
task test
```

Test files:
- `internal/controller/bootdependency_controller_test.go` — controller reconciliation
- `internal/webhook/v1/deployment_webhook_test.go` — mutating webhook
- `internal/webhook/v1alpha1/bootdependency_webhook_test.go` — validating webhook (cycle detection)

## Adding a new API

```bash
kubebuilder create api \
  --group core \
  --version v1alpha1 \
  --kind MyResource
```

## Adding a new webhook

```bash
kubebuilder create webhook \
  --group apps \
  --version v1 \
  --kind Deployment \
  --defaulting
```

## Helm chart development

```bash
task helm-lint                  # lint the chart
task helm-template              # render templates to stdout
task helm-install               # install into current cluster
task helm-uninstall             # remove from cluster
```
