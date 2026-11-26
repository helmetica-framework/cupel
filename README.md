# Cupel

**Cupel**: a shallow, porous vessel used in assaying to separate precious metals and reveal their true content.

Cupel **weighs** two sources against each other — OCI-hosted Helm charts or claim resources in your cluster: it renders each to its Kubernetes manifests and shows the difference side by side in your terminal.

Documentation: https://docs.helmetica.io/cupel/

## Quickstart

```bash
go run . weigh \
  oci://ghcr.io/stefanprodan/charts/podinfo:6.5.0 \
  oci://ghcr.io/stefanprodan/charts/podinfo:6.6.0
```

## Development

```bash
make build
make test
make docs   # preview the docs
```
