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

## Project structure

```
bootchain-operator/
├── api/v1alpha1/           # CRD Go types
├── cmd/main.go             # Entrypoint — flags, manager setup
├── config/                 # Kustomize manifests (CRD, RBAC, webhooks)
├── charts/bootchain-operator/  # Helm chart
├── docs/                   # MkDocs documentation
├── hack/                   # Dev helper scripts
├── internal/
│   ├── controller/         # Reconciliation loop + metrics
│   └── webhook/
│       ├── v1/             # Mutating webhook (Deployment)
│       └── v1alpha1/       # Validating webhook (BootDependency)
└── test/e2e/               # End-to-end tests
```

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
