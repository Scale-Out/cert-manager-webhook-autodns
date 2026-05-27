# Changelog

## 0.2.0 — Scale-Out fork

- Modernized Go module to 1.25 and dependencies (cert-manager 1.20.2, client-go 0.35.2, klog/v2 2.140).
- Switched import path from `github.com/jetstack/cert-manager` to `github.com/cert-manager/cert-manager`.
- Updated `k8s.io/apiextensions-apiserver` to `v1` (removed `v1beta1` usage).
- Solver name lowercased: `autoDNS` -> `autodns`.
- Replaced plain `username`/`password` config with `usernameSecretRef`/`passwordSecretRef` referencing a Kubernetes Secret.
- Helm chart `Chart.yaml` bumped to `apiVersion: v2`, version `0.2.0`.
- Added `replicaCount` default in `values.yaml`.
- Fixed broken `secret-reader` RoleBinding (missing subject); whitelist comes from `values.secretRefs`.
- Default image registry: `ghcr.io/scale-out/cert-manager-webhook-autodns`.
- Dockerfile rebased on `golang:1.25-alpine` and `alpine:3.20`.
- Removed `.gitlab-ci.yml`; builds are manual via `make image` / `make push`.
- Test setup migrated to `setup-envtest`.

## 1.0.0 (2021-10-17)


### Bug Fixes

* Switch to semantic release pipeline ([9e0cb06](https://gitlab.der-jd.de/containers/cert-manager-webhook-autodns/commit/9e0cb06c797221a0efd1202ea2834b9c6a5200c9))
