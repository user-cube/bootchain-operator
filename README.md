# bootchain-operator

A Kubernetes operator that makes service boot dependencies declarative and automatic, eliminating hand-written init containers.

## Documentation

Full documentation is available at **https://user-cube.github.io/bootchain-operator**

- [Installation](https://user-cube.github.io/bootchain-operator/getting-started/installation/)
- [Quickstart](https://user-cube.github.io/bootchain-operator/getting-started/quickstart/)
- [API Reference](https://user-cube.github.io/bootchain-operator/reference/api/)
- [Helm Values](https://user-cube.github.io/bootchain-operator/reference/helm-values/)
- [Metrics](https://user-cube.github.io/bootchain-operator/reference/metrics/)

## Quick install

```bash
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager --create-namespace \
  --set crds.enabled=true --wait

helm install bootchain-operator oci://ghcr.io/user-cube/charts/bootchain-operator \
  --namespace bootchain-operator-system --create-namespace --wait
```

## License

Apache License 2.0