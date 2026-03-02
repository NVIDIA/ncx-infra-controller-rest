<!--
SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
SPDX-License-Identifier: Apache-2.0
-->

# NVIDIA Bare Metal Manager REST API

A collection of microservices that comprise the management backend for NVIDIA Bare Metal Manager, exposed as a REST API.

In deployments, NVIDIA Bare Metal Manager REST requires [NVIDIA Bare Metal Manager Core](https://github.com/NVIDIA/bare-metal-manager-core) to function.

The REST layer can be deployed in the datacenter with Bare Metal Manager Core, or deployed anywhere in Cloud and allow Site Agent to connect from the datacenter. Multiple Bare Metal Manager Cores running in different datacenters can also connect to Bare Metal Manager REST through respective Site Agents.

View latest OpenAPI schema on [Github pages](https://nvidia.github.io/bare-metal-manager-rest/).

## Prerequisites

- Go 1.25.4 or later
- Docker 20.10+ with BuildKit enabled
- Make
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) (for local deployment)
- [kubectl](https://kubernetes.io/docs/tasks/tools/) (for local deployment)
- [jq](https://stedolan.github.io/jq/) (optional, for parsing JSON responses)

## Quick Start

### Run Unit Tests

```bash
make test
```

Tests require PostgreSQL. The Makefile automatically manages a test container.

Test database configuration:
- Host: `localhost`
- Port: `30432`
- User/Password: `postgres` / `postgres`

### Local Deployment with Kind

```bash
make kind-reset
```

This command:
1. Creates a Kind Kubernetes cluster
2. Builds all Docker images
3. Deploys all services (PostgreSQL, Temporal, Keycloak, cert-manager, etc.)
4. Runs database migrations
5. Configures PKI and site-agent
6. Deploys a mock Bare Metal Manager Core

Once complete, services are available at:

| Service | URL |
|---------|-----|
| API | http://localhost:8388 |
| Keycloak | http://localhost:8080 |
| Temporal UI | http://localhost:8233 |
| Adminer (DB UI) | http://localhost:8081 |

Other useful commands:

```bash
make kind-status    # Check pod status
make kind-logs      # Tail API logs
make kind-redeploy  # Rebuild and restart after code changes
make kind-verify    # Run health checks
make kind-down      # Tear down cluster
```

## Using the API

### Get an Access Token

```bash
TOKEN=$(curl -s -X POST "http://localhost:8080/realms/carbide-dev/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "client_id=carbide-api" \
  -d "client_secret=carbide-local-secret" \
  -d "grant_type=password" \
  -d "username=admin@example.com" \
  -d "password=adminpassword" | jq -r .access_token)
```

### Example API Requests

```bash
# Health check
curl -s http://localhost:8388/healthz -H "Authorization: Bearer $TOKEN" | jq .

# Get current tenant (auto-creates on first access)
curl -s "http://localhost:8388/v2/org/test-org/carbide/tenant/current" \
  -H "Authorization: Bearer $TOKEN" | jq .

# List sites
curl -s "http://localhost:8388/v2/org/test-org/carbide/site" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

### Test Users

| Email | Password | Roles |
|-------|----------|-------|
| `admin@example.com` | `adminpassword` | FORGE_PROVIDER_ADMIN, FORGE_TENANT_ADMIN |
| `testuser@example.com` | `testpassword` | FORGE_TENANT_ADMIN |
| `provider@example.com` | `providerpassword` | FORGE_PROVIDER_ADMIN |

All users have the `test-org` organization assigned.

## Building Docker Images

### Build All Images

```bash
make docker-build
```

Images are tagged with `localhost:5000` registry and `latest` tag by default.

### Build with Custom Registry and Tag

```bash
make docker-build IMAGE_REGISTRY=my-registry.example.com/carbide IMAGE_TAG=v1.0.0
```

### Push to Your Registry

1. Authenticate with your registry:

```bash
# Docker Hub
docker login

# AWS ECR
aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 123456789.dkr.ecr.us-east-1.amazonaws.com

# Google Container Registry
gcloud auth configure-docker

# Azure Container Registry
az acr login --name myregistry
```

2. Build and push:

```bash
REGISTRY=my-registry.example.com/bare-metal-manager-rest
TAG=v1.0.0

make docker-build IMAGE_REGISTRY=$REGISTRY IMAGE_TAG=$TAG

for image in carbide-rest-api carbide-rest-workflow carbide-rest-site-manager carbide-rest-site-agent carbide-rest-db carbide-rest-cert-manager; do
    docker push "$REGISTRY/$image:$TAG"
done
```

### Available Images

| Image | Description |
|-------|-------------|
| `carbide-rest-api` | Main REST API (port 8388) |
| `carbide-rest-workflow` | Temporal workflow worker |
| `carbide-rest-site-manager` | Site management worker |
| `carbide-rest-site-agent` | On-site agent |
| `carbide-rest-db` | Database migrations (run to completion) |
| `carbide-rest-cert-manager` | Native PKI certificate manager |


## Architecture

| Service | Binary | Description |
|---------|--------|-------------|
| carbide-rest-api | `api` | Main REST API server |
| carbide-rest-workflow | `workflow` | Temporal workflow service |
| carbide-rest-site-manager | `sitemgr` | Site management service |
| carbide-site-agent | `elektra` | On-site agent |
| carbide-rest-db | `migrations` | Database migrations |
| carbide-rest-cert-manager | `credsmgr` | Native PKI certificate manager |
| carbide-cli | `bmmcli` | [CLI client](cli/README.md) for the REST API |

Supporting modules:
- **common** - Shared utilities and configurations
- **auth** - Authentication and authorization
- **ipam** - IP Address Management

## OpenAPI Schema Development

OpenAPI schema must be updated whenever the API endpoints are added/updated. Please view instructions at [OpenAPI README](openapi/README.md)

## Pre-commit Hooks

This project uses [pre-commit](https://pre-commit.com/) with [TruffleHog](https://github.com/trufflesecurity/trufflehog) for secret detection to prevent accidentally committing sensitive information like API keys, passwords, or tokens.

### Setup

```bash
# Install pre-commit hooks (first time setup)
make pre-commit-install
```

This will:
1. Install `pre-commit` if not already installed
2. Install `trufflehog` if not already installed
3. Configure git hooks for pre-commit and pre-push

### Usage

Once installed, TruffleHog automatically scans your changes on every `git commit` and `git push`.

To manually run the scan on all files:

```bash
make pre-commit-run
```

Example output:

```
❯ make pre-commit-run
pre-commit run --all-files
[INFO] Initializing environment for https://github.com/trufflesecurity/trufflehog.
TruffleHog Secret Scan...................................................Passed
```

### Other Commands

```bash
make pre-commit-update  # Update hooks to latest versions
```

## Experimental Notice

This software is considered *experimental* and is a preview release. Use at
your own risk in production environments. The software is provided "as is"
without warranties of any kind. Features, APIs, and configurations may change
without notice in future releases. For production deployments, thoroughly test
in non-critical environments first.

## License

See [LICENSE](LICENSE) for details.
This project will download and install additional third-party open source software projects. Review the license terms of these open source projects before use.
