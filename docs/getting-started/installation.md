# Installation

## Prerequisites

| Requirement | Version |
|---|---|
| Kubernetes | >= 1.25 |
| Helm | >= 3.10 |
| cert-manager | >= 1.13 (for webhook TLS) |

## Install cert-manager

The operator uses cert-manager to automatically provision and rotate the TLS certificate for its admission webhooks.

```bash
helm repo add jetstack https://charts.jetstack.io --force-update
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set crds.enabled=true \
  --wait
```

## Install bootchain-operator

```bash
helm install bootchain-operator oci://ghcr.io/user-cube/bootchain-operator/bootchain-operator \
  --namespace bootchain-operator-system \
  --create-namespace \
  --wait
```

Verify the installation:

```bash
kubectl get all -n bootchain-operator-system
```

Expected output:

```
NAME                                     READY   STATUS    RESTARTS   AGE
pod/bootchain-operator-xxx-yyy           1/1     Running   0          30s

NAME                                 TYPE        CLUSTER-IP    PORT(S)    AGE
service/bootchain-operator-metrics   ClusterIP   10.x.x.x      8080/TCP   30s
service/bootchain-operator-webhook   ClusterIP   10.x.x.x      443/TCP    30s

NAME                                 READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/bootchain-operator   1/1     1            1           30s
```

## Install from source

```bash
git clone https://github.com/user-cube/bootchain-operator
cd bootchain-operator

helm install bootchain-operator charts/bootchain-operator \
  --namespace bootchain-operator-system \
  --create-namespace \
  --wait
```

## Uninstall

```bash
helm uninstall bootchain-operator --namespace bootchain-operator-system
```

!!! warning "CRD retention"
    By default, the CRD is annotated with `helm.sh/resource-policy: keep` and will **not** be deleted on uninstall. This protects your existing `BootDependency` resources.

    To also delete the CRD, run:
    ```bash
    kubectl delete crd bootdependencies.core.bootchain-operator.ruicoelho.dev
    ```
