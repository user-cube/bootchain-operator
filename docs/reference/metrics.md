# Metrics

bootchain-operator exposes Prometheus metrics via an HTTP endpoint on port `8080` (default).

## Custom metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `bootchain_reconcile_total` | Counter | `result` | Total reconciliations, partitioned by result |
| `bootchain_reconcile_duration_seconds` | Histogram | `result` | Duration of each reconciliation in seconds |
| `bootchain_dependencies_total` | Gauge | `namespace`, `name` | Total declared dependencies per BootDependency |
| `bootchain_dependencies_ready` | Gauge | `namespace`, `name` | Currently reachable dependencies per BootDependency |

### Label values

**`result`**: `success` | `error`

### Example output

```
# HELP bootchain_reconcile_total Total number of BootDependency reconciliations, partitioned by result.
# TYPE bootchain_reconcile_total counter
bootchain_reconcile_total{result="success"} 42
bootchain_reconcile_total{result="error"} 1

# HELP bootchain_reconcile_duration_seconds Duration of BootDependency reconciliation in seconds.
# TYPE bootchain_reconcile_duration_seconds histogram
bootchain_reconcile_duration_seconds_bucket{result="success",le="0.01"} 38
bootchain_reconcile_duration_seconds_sum{result="success"} 0.412
bootchain_reconcile_duration_seconds_count{result="success"} 42

# HELP bootchain_dependencies_total Total number of declared dependencies for a BootDependency resource.
# TYPE bootchain_dependencies_total gauge
bootchain_dependencies_total{name="payments-api",namespace="default"} 2

# HELP bootchain_dependencies_ready Number of dependencies currently reachable for a BootDependency resource.
# TYPE bootchain_dependencies_ready gauge
bootchain_dependencies_ready{name="payments-api",namespace="default"} 1
```

## controller-runtime metrics

In addition to the custom metrics above, controller-runtime exposes standard Kubernetes controller metrics:

| Metric | Description |
|---|---|
| `controller_runtime_reconcile_total` | Reconcile calls by controller and result |
| `controller_runtime_reconcile_time_seconds` | Reconcile duration by controller |
| `controller_runtime_webhook_requests_total` | Webhook requests by webhook and HTTP status |
| `controller_runtime_webhook_latency_seconds` | Webhook handler latency |
| `workqueue_*` | Work queue depth, latency, and throughput |

## Accessing metrics

### Via port-forward

```bash
kubectl port-forward svc/bootchain-operator-metrics 8080:8080 \
  -n bootchain-operator-system
curl http://localhost:8080/metrics
```

### Via Prometheus Operator

Enable the ServiceMonitor in the Helm chart:

```bash
helm upgrade bootchain-operator charts/bootchain-operator \
  --set metrics.serviceMonitor.enabled=true \
  --set metrics.serviceMonitor.additionalLabels.release=prometheus
```

The `additionalLabels` must match your Prometheus instance's `serviceMonitorSelector`.

## Grafana dashboard

The Helm chart ships a pre-built Grafana dashboard that can be deployed as a ConfigMap and auto-discovered by the Grafana sidecar or Grafana Operator.

### Panels

| Section | Panel | Description |
|---|---|---|
| Overview | BootDependency Resources | Count of BootDependency objects being tracked |
| Overview | Dependencies Ready (total) | Sum of all reachable dependencies across all resources |
| Overview | Dependencies Not Ready | Sum of unresolved dependencies (red when > 0) |
| Overview | Reconcile Error Rate | Fraction of reconciliations that ended in error |
| Overview | Reconcile Latency p99 | 99th-percentile reconcile duration |
| Dependency Health | Dependency Readiness Ratio | Gauge showing ready/total per resource |
| Dependency Health | Ready vs Total Over Time | Time-series of ready and total counts per resource |
| Dependency Health | BootDependency Status Table | Per-resource table with ready / total counts |
| Reconciliation | Reconcile Throughput | Reconcile rate (success vs error) over time |
| Reconciliation | Reconcile Duration (p50/p95/p99) | Latency percentiles by result |
| Webhook | Webhook Request Rate | Mutating and validating webhook request rates |
| Webhook | Webhook Latency (p95/p99) | Webhook handler latency percentiles |

The dashboard includes two template variables — **Namespace** and **BootDependency** — that filter all panels to the selected resources.

### Enable with kube-prometheus-stack (Grafana sidecar)

kube-prometheus-stack uses the Grafana sidecar, which discovers ConfigMaps labelled `grafana_dashboard: "1"` (default).

```bash
helm upgrade bootchain-operator charts/bootchain-operator \
  --set grafana.dashboard.enabled=true \
  --set grafana.dashboard.labels.grafana_dashboard="1"
```

The sidecar will pick up the ConfigMap and import the dashboard automatically — no manual import required.

### Enable with Grafana Operator

If you use the Grafana Operator, set `grafana.dashboard.labels` to match your `GrafanaDashboard` label selector:

```bash
helm upgrade bootchain-operator charts/bootchain-operator \
  --set grafana.dashboard.enabled=true \
  --set grafana.dashboard.labels.app=grafana
```

### Manual import

If you prefer to import the dashboard manually, extract the JSON from the ConfigMap and paste it into **Grafana → Dashboards → Import**:

```bash
kubectl get configmap bootchain-operator-dashboard \
  -n bootchain-operator-system \
  -o jsonpath='{.data.bootchain-operator\.json}' > bootchain-operator.json
```

Then open Grafana, go to **Dashboards → Import**, upload `bootchain-operator.json`, and select your Prometheus datasource.

## Suggested alerts

```yaml
groups:
- name: bootchain-operator
  rules:
  - alert: BootDependencyReconcileErrors
    expr: rate(bootchain_reconcile_total{result="error"}[5m]) > 0
    for: 5m
    annotations:
      summary: "bootchain-operator reconcile errors"

  - alert: BootDependencyUnresolved
    expr: bootchain_dependencies_ready < bootchain_dependencies_total
    for: 10m
    annotations:
      summary: "BootDependency {{ $labels.namespace }}/{{ $labels.name }} has unresolved dependencies"
```
