# Bare Metal Manager REST Helm Charts

Helm charts for deploying the Bare Metal Manager REST API platform services.

## Charts

| Chart | Description |
|-------|-------------|
| `carbide-rest-site-manager` | Site lifecycle manager (TLS on port 8100) |
| `carbide-rest-db` | Database migration job (Bun ORM, idempotent) |
| `carbide-rest-workflow` | Temporal workers (cloud-worker + site-worker) |
| `carbide-rest-api` | REST API server (port 8388) |
| `carbide-rest-site-agent` | Elektra site agent (deployed independently per-site) |

## Prerequisites

The following must be running before installing charts:

- **PostgreSQL** database
- **Temporal** server with `cloud` and `site` namespaces
- **Keycloak** authentication server
- **cert-manager** with ClusterIssuer `carbide-rest-ca-issuer`
- **Site CRD**: `kubectl apply -f deploy/kustomize/base/site-manager/site-crd.yaml`
- **Secrets**: `db-creds`, `keycloak-client-secret`, `temporal-encryption-key`, `temporal-client-cloud-certs`

## Install

Charts are deployed individually in dependency order:

```bash
# Set common variables
REPO=ghcr.io/your/registry
TAG=latest
NS=carbide-rest

# 1. Site Manager
helm upgrade --install carbide-rest-site-manager charts/carbide-rest-site-manager/ \
  --namespace $NS --set global.image.repository=$REPO --set global.image.tag=$TAG

# 2. DB Migration
helm upgrade --install carbide-rest-db charts/carbide-rest-db/ \
  --namespace $NS --set global.image.repository=$REPO --set global.image.tag=$TAG

# 3. Workflow Workers
helm upgrade --install carbide-rest-workflow charts/carbide-rest-workflow/ \
  --namespace $NS --set global.image.repository=$REPO --set global.image.tag=$TAG

# 4. REST API
helm upgrade --install carbide-rest-api charts/carbide-rest-api/ \
  --namespace $NS --set global.image.repository=$REPO --set global.image.tag=$TAG
```

## Site Agent

Site agent is deployed separately because it requires a registered site (UUID + OTP).

```bash
# 1. Install chart (will CrashLoop until bootstrapped)
helm upgrade --install carbide-rest-site-agent charts/carbide-rest-site-agent/ \
  --namespace $NS --set global.image.repository=$REPO --set global.image.tag=$TAG || true

# 2. Bootstrap site registration (creates site via API, patches ConfigMap/Secret) e.g. ./scripts/setup-local.sh site-agent

# 3. Site agent will stabilize after bootstrap
kubectl -n $NS rollout status statefulset/carbide-rest-site-agent --timeout=120s
```

## Uninstall

```bash
helm uninstall carbide-rest-site-agent -n $NS
helm uninstall carbide-rest-api -n $NS
helm uninstall carbide-rest-workflow -n $NS
helm uninstall carbide-rest-db -n $NS
helm uninstall carbide-rest-site-manager -n $NS
```

## Configuration

All charts accept a `global` section:

```yaml
global:
  image:
    repository: nvcr.io/0837451325059433/carbide-dev
    tag: "1.0.5"
    pullPolicy: IfNotPresent
  imagePullSecrets:
    - name: image-pull-secret
  certificate:  # only for site-manager and site-agent
    issuerRef:
      kind: ClusterIssuer
      name: carbide-rest-ca-issuer
      group: cert-manager.io
```

See each chart's `values.yaml` for full configuration options.
