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
