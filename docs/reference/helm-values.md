# Helm Values

Full reference for all configurable values in the `bootchain-operator` Helm chart.

## Image

| Value | Default | Description |
|---|---|---|
| `image.repository` | `ghcr.io/user-cube/bootchain-operator` | Container image repository |
| `image.tag` | `""` | Image tag. Defaults to `.Chart.AppVersion` |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy |
| `imagePullSecrets` | `[]` | List of pull secret names |

## Deployment

| Value | Default | Description |
|---|---|---|
| `replicaCount` | `1` | Number of operator replicas |
| `nameOverride` | `""` | Override the chart name |
| `fullnameOverride` | `""` | Override the full release name |

## Service Account

| Value | Default | Description |
|---|---|---|
| `serviceAccount.name` | `""` | ServiceAccount name. Defaults to release fullname |
| `serviceAccount.annotations` | `{}` | Annotations to add to the ServiceAccount |

## RBAC

| Value | Default | Description |
|---|---|---|
| `rbac.create` | `true` | Create ClusterRole and ClusterRoleBinding |

## Leader Election

| Value | Default | Description |
|---|---|---|
| `leaderElection.enabled` | `true` | Enable leader election (recommended for production) |

## Resources

| Value | Default | Description |
|---|---|---|
| `resources.requests.cpu` | `10m` | CPU request |
| `resources.requests.memory` | `64Mi` | Memory request |
| `resources.limits.cpu` | `500m` | CPU limit |
| `resources.limits.memory` | `128Mi` | Memory limit |

## Metrics

| Value | Default | Description |
|---|---|---|
| `metrics.enabled` | `true` | Enable the metrics endpoint |
| `metrics.secure` | `false` | Serve metrics over HTTPS |
| `metrics.port` | `8080` | Metrics server port |
| `metrics.service.type` | `ClusterIP` | Metrics Service type |
| `metrics.service.annotations` | `{}` | Annotations on the metrics Service |
| `metrics.serviceMonitor.enabled` | `false` | Create a Prometheus Operator ServiceMonitor |
| `metrics.serviceMonitor.interval` | `30s` | Scrape interval |
| `metrics.serviceMonitor.scrapeTimeout` | `10s` | Scrape timeout |
| `metrics.serviceMonitor.additionalLabels` | `{}` | Extra labels on the ServiceMonitor |

## Webhooks

| Value | Default | Description |
|---|---|---|
| `webhook.enabled` | `true` | Enable admission webhooks |
| `webhook.port` | `9443` | Webhook server port |
| `webhook.service.type` | `ClusterIP` | Webhook Service type |
| `webhook.certManager.enabled` | `true` | Use cert-manager for TLS certificate provisioning |
| `webhook.certManager.duration` | `8760h` | Certificate duration (1 year) |
| `webhook.certManager.renewBefore` | `720h` | Certificate renewal window (30 days) |

## CRDs

| Value | Default | Description |
|---|---|---|
| `crds.install` | `true` | Install the BootDependency CRD |
| `crds.keep` | `true` | Retain the CRD when the release is uninstalled |

## Scheduling

| Value | Default | Description |
|---|---|---|
| `nodeSelector` | `{}` | Node selector for the operator pod |
| `tolerations` | `[]` | Tolerations for the operator pod |
| `affinity` | `{}` | Affinity rules for the operator pod |

## Pod

| Value | Default | Description |
|---|---|---|
| `podAnnotations` | `{}` | Annotations added to the operator pod |
| `podLabels` | `{}` | Extra labels added to the operator pod |

## Examples

### Minimal install (webhooks disabled)

```bash
helm install bootchain-operator charts/bootchain-operator \
  --set webhook.enabled=false
```

### Enable Prometheus scraping

```bash
helm install bootchain-operator charts/bootchain-operator \
  --set metrics.serviceMonitor.enabled=true \
  --set metrics.serviceMonitor.additionalLabels.release=prometheus
```

### Custom image

```bash
helm install bootchain-operator charts/bootchain-operator \
  --set image.repository=my-registry/bootchain-operator \
  --set image.tag=v1.2.3
```
