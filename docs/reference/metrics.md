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

### Setup checklist

Both items below are required. The dashboard will show **"No data"** if either is missing.

| # | Requirement | How to verify |
|---|---|---|
| 1 | **ServiceMonitor enabled** — Prometheus must be scraping the operator's `/metrics` endpoint | Check _Status → Targets_ in the Prometheus UI for a target named `bootchain-operator` |
| 2 | **Dashboard label matches the Grafana sidecar/operator selector** — the ConfigMap must carry the label the sidecar watches | Check the sidecar's `GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH` or `sidecar.dashboards.label` in your Grafana Helm values |

### Enable with kube-prometheus-stack (Grafana sidecar)

kube-prometheus-stack ships a Grafana sidecar that auto-discovers ConfigMaps carrying a specific label (default: `grafana_dashboard: "1"`). Both the ServiceMonitor and the dashboard ConfigMap must be enabled together:

```bash
helm upgrade bootchain-operator charts/bootchain-operator \
  --set metrics.serviceMonitor.enabled=true \
  --set metrics.serviceMonitor.additionalLabels.release=prometheus \
  --set grafana.dashboard.enabled=true \
  --set grafana.dashboard.labels.grafana_dashboard="1"
```

> **`additionalLabels.release`** must match the `serviceMonitorSelector` label of your Prometheus instance. A common value is `prometheus` or `kube-prometheus-stack`. Check with:
> ```bash
> kubectl get prometheus -A -o jsonpath='{.items[*].spec.serviceMonitorSelector}'
> ```
> If `serviceMonitorSelector` is empty (`{}`), all ServiceMonitors are picked up and the label can be omitted.

The sidecar will pick up the ConfigMap and import the dashboard automatically — no manual import required.

**Verifying the sidecar picked up the dashboard:**

```bash
# Check sidecar logs for "Found ConfigMap" or "Updating dashboard"
kubectl logs -n <grafana-namespace> \
  -l app.kubernetes.io/name=grafana \
  -c grafana-sc-dashboard
```

### Enable with Grafana Operator

If you use the Grafana Operator, set `grafana.dashboard.labels` to match your `GrafanaDashboard` label selector:

```bash
helm upgrade bootchain-operator charts/bootchain-operator \
  --set metrics.serviceMonitor.enabled=true \
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

> The ServiceMonitor must still be enabled for the imported dashboard to show data.

### Troubleshooting

**Dashboard does not appear in Grafana**

1. Confirm the ConfigMap was created:
   ```bash
   kubectl get configmap bootchain-operator-dashboard -n bootchain-operator-system
   ```
2. Confirm the label on the ConfigMap matches the sidecar's `sidecar.dashboards.label` value (default `grafana_dashboard: "1"`):
   ```bash
   kubectl get configmap bootchain-operator-dashboard \
     -n bootchain-operator-system \
     --show-labels
   ```
3. If the label is missing or wrong, either re-deploy with the correct `grafana.dashboard.labels` value, or patch it directly:
   ```bash
   kubectl label configmap bootchain-operator-dashboard \
     grafana_dashboard="1" \
     -n bootchain-operator-system
   ```
4. Check the sidecar container logs (see _Enable with kube-prometheus-stack_ above).

---

**All panels show "No data"**

1. Confirm Prometheus is scraping the operator:
   ```bash
   kubectl port-forward svc/bootchain-operator-metrics 8080:8080 \
     -n bootchain-operator-system
   curl -s http://localhost:8080/metrics | grep bootchain
   ```
   If this returns metrics, the operator is healthy. If Prometheus is still not scraping it, the ServiceMonitor is likely missing or has the wrong labels.

2. Check whether the ServiceMonitor exists:
   ```bash
   kubectl get servicemonitor -n bootchain-operator-system
   ```
   If it does not exist, enable it:
   ```bash
   helm upgrade bootchain-operator charts/bootchain-operator \
     --set metrics.serviceMonitor.enabled=true \
     --set metrics.serviceMonitor.additionalLabels.release=<your-release-label>
   ```

3. Verify the ServiceMonitor is being picked up by Prometheus (_Status → Targets_ in the Prometheus UI). If the target is missing, the `additionalLabels` on the ServiceMonitor do not match your Prometheus instance's `serviceMonitorSelector`.

4. In the Grafana dashboard, confirm the **datasource** variable at the top is pointing to the correct Prometheus instance.

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
