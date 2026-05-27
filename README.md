# cert-manager-webhook-autodns

ACME DNS-01 solver webhook for [cert-manager](https://cert-manager.io/) that uses the
[InterNetX AutoDNS JSON API](https://help.internetx.com/display/APIXMLEN/JSON+API+Basics)
to provision `_acme-challenge` TXT records.

Scale-Out fork of [derJD/cert-manager-webhook-autodns](https://github.com/derJD/cert-manager-webhook-autodns)
with modernized dependencies, SecretRef-based credentials, and a manual build flow.

## Requirements

- Go 1.25+
- Helm 3
- Kubernetes 1.27+
- cert-manager 1.16+

## Build & push image

No CI is configured. Build and push manually:

```bash
docker build -t ghcr.io/scale-out/cert-manager-webhook-autodns:0.2.0 .
docker push ghcr.io/scale-out/cert-manager-webhook-autodns:0.2.0
```

Or via Makefile:

```bash
make push IMAGE_TAG=0.2.0
```

## Install

```bash
helm install --namespace cert-manager \
  cert-manager-webhook-autodns \
  deploy/cert-manager-webhook-autodns \
  --set groupName=acme.yourdomain.tld
```

The webhook reads AutoDNS credentials from Kubernetes Secrets in its release
namespace (`cert-manager`). List allowed Secret names under `values.secretRefs`
so RBAC permits reading them:

```yaml
secretRefs:
  - autodns-credentials
```

## Credentials Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: autodns-credentials
  namespace: cert-manager
type: Opaque
stringData:
  username: your-autodns-user
  password: your-autodns-password
```

## ClusterIssuer

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: you@example.com
    privateKeySecretRef:
      name: letsencrypt-prod-account-key
    solvers:
      - dns01:
          webhook:
            groupName: acme.yourdomain.tld
            solverName: autodns
            config:
              url: https://api.autodns.com/v1
              context: "4"               # PersonalAutoDNS context, "1" = demo
              nameserver: ns1.pns.de     # authoritative NS for the zone
              usernameSecretRef:
                name: autodns-credentials
                key: username
              passwordSecretRef:
                name: autodns-credentials
                key: password
        selector:
          dnsZones:
            - yourdomain.tld
```

## Wildcard certificate

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: wildcard-yourdomain-tld
  namespace: default
spec:
  secretName: wildcard-yourdomain-tld-tls
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  dnsNames:
    - "*.yourdomain.tld"
    - "yourdomain.tld"
```

## Configuration reference

| Field | Required | Description |
|-------|----------|-------------|
| `url` | yes | AutoDNS endpoint. Live: `https://api.autodns.com/v1`, Demo: `https://api.demo.autodns.com/v1` |
| `context` | yes | AutoDNS context (string). `"1"` = demo, `"4"` or PersonalAutoDNS context number for live |
| `nameserver` | yes | Authoritative nameserver for the zone (e.g. `ns1.pns.de`) |
| `usernameSecretRef.name/key` | yes | Secret reference for the AutoDNS username |
| `passwordSecretRef.name/key` | yes | Secret reference for the AutoDNS password |
| `zone` | no | Override resolved zone. Usually leave empty. |

## License

Apache License 2.0. Original work by [derJD](https://github.com/derJD).
