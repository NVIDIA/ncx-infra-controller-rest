# SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
# SPDX-License-Identifier: Apache-2.0
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.PHONY: test postgres-up postgres-down ensure-postgres postgres-wait clean
.PHONY: build docker-build docker-build-local
.PHONY: test-ipam test-site-agent test-site-manager test-workflow test-db test-api test-auth test-common test-cert-manager test-site-workflow migrate carbide-mock-server-build carbide-mock-server-start carbide-mock-server-stop rla-mock-server-build rla-mock-server-start rla-mock-server-stop
.PHONY: validate-openapi preview-openapi generate-client
.PHONY: pre-commit-install pre-commit-run pre-commit-update

# Build configuration
BUILD_DIR := build/binaries
IMAGE_REGISTRY := localhost:5000
IMAGE_TAG := latest
DOCKERFILE_DIR := docker/production

# PostgreSQL container configuration
POSTGRES_CONTAINER_NAME := project-test
POSTGRES_PORT := 30432
POSTGRES_USER := postgres
POSTGRES_PASSWORD := postgres
POSTGRES_DB := forgetest
POSTGRES_IMAGE := postgres:14.4-alpine

postgres-up:
	docker run -d --rm \
		--name $(POSTGRES_CONTAINER_NAME) \
		-p $(POSTGRES_PORT):5432 \
		-e POSTGRES_USER=$(POSTGRES_USER) \
		-e POSTGRES_PASSWORD=$(POSTGRES_PASSWORD) \
		-e POSTGRES_DB=$(POSTGRES_DB) \
		$(POSTGRES_IMAGE)

postgres-down:
	-docker rm -f $(POSTGRES_CONTAINER_NAME)

clean:
	@echo "Cleaning up test resources..."
	-$(MAKE) postgres-down
	-$(MAKE) carbide-mock-server-stop
	-$(MAKE) rla-mock-server-stop
	@echo "Stopping kind cluster..."
	-$(MAKE) kind-down
	@echo "Stopping colima..."
	-colima stop
	@echo "Removing build artifacts..."
	-rm -rf $(BUILD_DIR)
	-rm -rf build
	-rm -f db/cmd/migrations/migrations
	@echo "Clean complete"

ensure-postgres:
	@docker inspect $(POSTGRES_CONTAINER_NAME) > /dev/null 2>&1 || $(MAKE) postgres-up
	@$(MAKE) postgres-wait

postgres-wait:
	@until docker exec $(POSTGRES_CONTAINER_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "SELECT 1" > /dev/null 2>&1; do :; done

migrate:
	docker exec $(POSTGRES_CONTAINER_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "CREATE EXTENSION IF NOT EXISTS pg_trgm;"
	cd db/cmd/migrations && go build -o migrations .
	PGHOST=localhost \
	PGPORT=$(POSTGRES_PORT) \
	PGDATABASE=$(POSTGRES_DB) \
	PGUSER=$(POSTGRES_USER) \
	PGPASSWORD=$(POSTGRES_PASSWORD) \
	./db/cmd/migrations/migrations db init_migrate

lint-go:
	go vet ./...
	golangci-lint run --issues-exit-code 0 --output.code-climate.path=stdout | jq .
	go list ./... | xargs -L1 revive -config .revive.toml -set_exit_status

test-ipam:
	$(MAKE) ensure-postgres
	cd ipam && go test ./... -count=1

test-site-manager:
	cd site-manager && CGO_ENABLED=1 go test -race -p 1 ./... -count=1

test-workflow:
	$(MAKE) ensure-postgres
	cd workflow && go test -p 1 ./... -count=1

test-db:
	$(MAKE) ensure-postgres
	cd db && go test -p 1 ./... -count=1

carbide-mock-server-build:
	mkdir -p build
	cd site-agent/cmd/elektraserver && go build -o ../../../build/elektraserver .

carbide-mock-server-start: carbide-mock-server-build
	-lsof -ti:11079 | xargs kill -9 2>/dev/null
	./build/elektraserver -tout 0 > build/elektraserver.log 2>&1 & echo $$! > build/elektraserver.pid
	@echo "Waiting for gRPC server to start..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if grep -q "Started API server" build/elektraserver.log 2>/dev/null; then \
			sleep 0.1; \
			echo "gRPC server is ready"; \
			exit 0; \
		fi; \
		sleep 0.2; \
	done; \
	echo "Timeout waiting for gRPC server to start"; \
	exit 1

carbide-mock-server-stop:
	-kill $$(cat build/elektraserver.pid) 2>/dev/null
	-rm -f build/elektraserver.pid

rla-mock-server-build:
	mkdir -p build
	cd site-agent/cmd/rlaserver && go build -o ../../../build/rlaserver .

rla-mock-server-start: rla-mock-server-build
	-lsof -ti:11080 | xargs kill -9 2>/dev/null
	./build/rlaserver -tout 0 > build/rlaserver.log 2>&1 & echo $$! > build/rlaserver.pid
	@echo "Waiting for RLA gRPC server to start..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if grep -q "Started RLA API server" build/rlaserver.log 2>/dev/null; then \
			sleep 0.1; \
			echo "RLA gRPC server is ready"; \
			exit 0; \
		fi; \
		sleep 0.2; \
	done; \
	echo "Timeout waiting for RLA gRPC server to start"; \
	exit 1

rla-mock-server-stop:
	-kill $$(cat build/rlaserver.pid) 2>/dev/null
	-rm -f build/rlaserver.pid

test-site-agent: carbide-mock-server-start rla-mock-server-start
	cd site-agent/pkg/components && CGO_ENABLED=1 go test -race -p 1 ./... -count=1 ; \
	ret=$$? ; cd ../../.. && $(MAKE) carbide-mock-server-stop rla-mock-server-stop ; exit $$ret

test-api:
	$(MAKE) ensure-postgres
	cd api && go test -p 1 ./... -count=1

test-auth:
	$(MAKE) ensure-postgres
	cd auth && go test -p 1 ./... -count=1

test-common:
	cd common && go test -p 1 ./... -count=1

test-cert-manager:
	cd cert-manager && go test -p 1 ./... -count=1

test-site-workflow:
	cd site-workflow && go test -p 1 ./... -count=1

test: test-ipam test-db test-api test-auth test-common test-cert-manager test-site-workflow test-site-manager test-workflow test-site-agent

build:
	mkdir -p $(BUILD_DIR)
	go mod download
	cd api && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-extldflags '-static'" -o ../$(BUILD_DIR)/api ./cmd/api
	cd workflow && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-extldflags '-static'" -o ../$(BUILD_DIR)/workflow ./cmd/workflow
	cd site-manager && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-extldflags '-static'" -o ../$(BUILD_DIR)/sitemgr ./cmd/sitemgr
	cd site-agent && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-extldflags '-static'" -o ../$(BUILD_DIR)/elektra ./cmd/elektra
	cd db && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-extldflags '-static'" -o ../$(BUILD_DIR)/migrations ./cmd/migrations
	cd cert-manager && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-extldflags '-static'" -o ../$(BUILD_DIR)/credsmgr ./cmd/credsmgr
	cd cli && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-extldflags '-static'" -o ../$(BUILD_DIR)/bmmcli ./cmd/bmmcli

INSTALL_DIR ?= $(shell go env GOPATH)/bin

install-bmmcli:
	cd cli && go build -o $(INSTALL_DIR)/bmmcli ./cmd/bmmcli
	@echo "Installed bmmcli to $(INSTALL_DIR)/bmmcli"

docker-build:
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-api:$(IMAGE_TAG) -f $(DOCKERFILE_DIR)/Dockerfile.carbide-rest-api .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-workflow:$(IMAGE_TAG) -f $(DOCKERFILE_DIR)/Dockerfile.carbide-rest-workflow .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-site-manager:$(IMAGE_TAG) -f $(DOCKERFILE_DIR)/Dockerfile.carbide-rest-site-manager .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-site-agent:$(IMAGE_TAG) -f $(DOCKERFILE_DIR)/Dockerfile.carbide-rest-site-agent .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-db:$(IMAGE_TAG) -f $(DOCKERFILE_DIR)/Dockerfile.carbide-rest-db .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-cert-manager:$(IMAGE_TAG) -f $(DOCKERFILE_DIR)/Dockerfile.carbide-rest-cert-manager .
	docker build -t $(IMAGE_REGISTRY)/carbide-rla:$(IMAGE_TAG) -f $(DOCKERFILE_DIR)/Dockerfile.carbide-rla .
	docker build -t $(IMAGE_REGISTRY)/carbide-psm:$(IMAGE_TAG) -f $(DOCKERFILE_DIR)/Dockerfile.carbide-psm .

carbide-proto:
	if [ -d "carbide-core" ]; then cd carbide-core && git pull; else git clone ssh://git@github.com/nvidia/carbide-core.git; fi
	ls carbide-core/rpc/proto
	@for file in carbide-core/rpc/proto/*.proto; do \
		cp "$$file" "workflow-schema/site-agent/workflows/v1/$$(basename "$$file" .proto)_carbide.proto"; \
		echo "Copied: $$file"; \
		./workflow-schema/scripts/add-go-package-option.sh "workflow-schema/site-agent/workflows/v1/$$(basename "$$file" .proto)_carbide.proto" "github.com/nvidia/bare-metal-manager-rest/workflow-schema/proto"; \
	done
	rm -rf carbide-core

carbide-protogen:
	echo "Generating protobuf for Carbide"
	cd workflow-schema && buf generate

rla-proto:
	RLA_DIR=rla \
	ls "$${RLA_DIR}/proto/v1"; \
	for file in "$${RLA_DIR}"/proto/v1/*.proto; do \
		cp "$$file" "workflow-schema/rla/proto/v1/"; \
		echo "Copied: $$file"; \
		./workflow-schema/scripts/add-go-package-option.sh "workflow-schema/rla/proto/v1/$$(basename "$$file")" "github.com/nvidia/bare-metal-manager-rest/workflow-schema/rla"; \
	done

rla-protogen:
	echo "Generating protobuf for RLA"
	cd workflow-schema/rla && buf generate

# =============================================================================
# Kind Local Deployment Targets
# =============================================================================

.PHONY: kind-up kind-down kind-deploy kind-load kind-apply kind-redeploy kind-status kind-logs kind-reset kind-verify setup-site-agent

# Kind cluster configuration
KIND_CLUSTER_NAME := carbide-rest-local
KUSTOMIZE_OVERLAY := deploy/kustomize/overlays/local
LOCAL_DOCKERFILE_DIR := docker/local

# Recommended colima configuration for full stack with Temporal:
#   colima start --cpu 8 --memory 8 --disk 100

# Build images using local Dockerfiles (public base images for local dev)
docker-build-local:
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-api:$(IMAGE_TAG) -f $(LOCAL_DOCKERFILE_DIR)/Dockerfile.carbide-rest-api .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-workflow:$(IMAGE_TAG) -f $(LOCAL_DOCKERFILE_DIR)/Dockerfile.carbide-rest-workflow .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-site-manager:$(IMAGE_TAG) -f $(LOCAL_DOCKERFILE_DIR)/Dockerfile.carbide-rest-site-manager .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-site-agent:$(IMAGE_TAG) -f $(LOCAL_DOCKERFILE_DIR)/Dockerfile.carbide-rest-site-agent .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-mock-core:$(IMAGE_TAG) -f $(LOCAL_DOCKERFILE_DIR)/Dockerfile.carbide-rest-mock-core .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-db:$(IMAGE_TAG) -f $(LOCAL_DOCKERFILE_DIR)/Dockerfile.carbide-rest-db .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-cert-manager:$(IMAGE_TAG) -f $(LOCAL_DOCKERFILE_DIR)/Dockerfile.carbide-rest-cert-manager .

# Create kind cluster with port mappings
kind-up:
	kind create cluster --name $(KIND_CLUSTER_NAME) --config deploy/kind/cluster-config.yaml
	kubectl apply -f deploy/kustomize/base/crds/

# Delete kind cluster
kind-down:
	kind delete cluster --name $(KIND_CLUSTER_NAME)

# Full deployment: build images, load into kind, apply manifests
kind-deploy: docker-build-local kind-load kind-apply
	@echo "Deployment complete. Run 'make kind-verify' to verify."

# Load all images into kind cluster
kind-load:
	kind load docker-image $(IMAGE_REGISTRY)/carbide-rest-api:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(IMAGE_REGISTRY)/carbide-rest-workflow:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(IMAGE_REGISTRY)/carbide-rest-site-manager:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(IMAGE_REGISTRY)/carbide-rest-site-agent:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(IMAGE_REGISTRY)/carbide-rest-mock-core:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(IMAGE_REGISTRY)/carbide-rest-db:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(IMAGE_REGISTRY)/carbide-rest-cert-manager:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)

# Apply kustomize manifests and wait for readiness
kind-apply:
	kubectl apply -k $(KUSTOMIZE_OVERLAY)
	@echo "Waiting for PostgreSQL..."
	kubectl -n carbide-rest wait --for=condition=ready pod -l app=postgres --timeout=120s
	@echo "Waiting for Cert Manager..."
	kubectl -n carbide-rest wait --for=condition=ready pod -l app=carbide-rest-cert-manager --timeout=180s
	@echo "Waiting for Temporal..."
	kubectl -n temporal wait --for=condition=ready pod -l app.kubernetes.io/name=temporal,app.kubernetes.io/component=frontend --timeout=120s
	@echo "Waiting for Keycloak..."
	kubectl -n carbide-rest wait --for=condition=ready pod -l app=keycloak --timeout=180s
	@echo "Running database migrations..."
	kubectl -n carbide-rest wait --for=condition=complete job/db --timeout=120s
	@echo "Waiting for API service..."
	kubectl -n carbide-rest wait --for=condition=ready pod -l app=carbide-rest-api --timeout=120s || true
	@echo "Waiting for Site Manager..."
	kubectl -n carbide-rest rollout restart deployment/carbide-rest-site-manager || true
	kubectl -n carbide-rest wait --for=condition=ready pod -l app=carbide-rest-site-manager --timeout=120s || true
	@echo "Waiting for Site Agent..."
	kubectl -n carbide-rest rollout restart deployment/carbide-rest-site-agent || true
	kubectl -n carbide-rest wait --for=condition=ready pod -l app=carbide-rest-site-agent --timeout=120s || true

# Rebuild and redeploy apps only (faster iteration)
kind-redeploy: docker-build-local kind-load
	kubectl -n carbide-rest rollout restart deployment/carbide-rest-api
	kubectl -n carbide-rest rollout restart deployment/cloud-worker
	kubectl -n carbide-rest rollout restart deployment/site-worker
	kubectl -n carbide-rest rollout restart deployment/carbide-rest-site-agent
	kubectl -n carbide-rest rollout restart deployment/carbide-rest-mock-core
	kubectl -n carbide-rest rollout restart deployment/carbide-rest-cert-manager
	kubectl -n carbide-rest rollout restart deployment/carbide-rest-site-manager

# Show status of all pods and services
kind-status:
	kubectl -n carbide-rest get pods,svc,jobs

# View logs from API service
kind-logs:
	kubectl -n carbide-rest logs -l app=carbide-rest-api -f --tail=100

# Full reset: tear down cluster, rebuild images, and redeploy everything
kind-reset:
	-kind delete cluster --name $(KIND_CLUSTER_NAME)
	kind create cluster --name $(KIND_CLUSTER_NAME) --config deploy/kind/cluster-config.yaml
	kubectl apply -f deploy/kustomize/base/crds/
	@echo "Installing cert-manager.io..."
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.14.4/cert-manager.yaml
	@echo "Waiting for cert-manager deployments..."
	kubectl -n cert-manager rollout status deployment/cert-manager --timeout=240s
	kubectl -n cert-manager rollout status deployment/cert-manager-webhook --timeout=240s
	kubectl -n cert-manager rollout status deployment/cert-manager-cainjector --timeout=240s
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-api:$(IMAGE_TAG) -f $(LOCAL_DOCKERFILE_DIR)/Dockerfile.carbide-rest-api .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-workflow:$(IMAGE_TAG) -f $(LOCAL_DOCKERFILE_DIR)/Dockerfile.carbide-rest-workflow .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-site-manager:$(IMAGE_TAG) -f $(LOCAL_DOCKERFILE_DIR)/Dockerfile.carbide-rest-site-manager .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-site-agent:$(IMAGE_TAG) -f $(LOCAL_DOCKERFILE_DIR)/Dockerfile.carbide-rest-site-agent .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-mock-core:$(IMAGE_TAG) -f $(LOCAL_DOCKERFILE_DIR)/Dockerfile.carbide-rest-mock-core .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-db:$(IMAGE_TAG) -f $(LOCAL_DOCKERFILE_DIR)/Dockerfile.carbide-rest-db .
	docker build -t $(IMAGE_REGISTRY)/carbide-rest-cert-manager:$(IMAGE_TAG) -f $(LOCAL_DOCKERFILE_DIR)/Dockerfile.carbide-rest-cert-manager .
	kind load docker-image $(IMAGE_REGISTRY)/carbide-rest-api:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(IMAGE_REGISTRY)/carbide-rest-workflow:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(IMAGE_REGISTRY)/carbide-rest-site-manager:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(IMAGE_REGISTRY)/carbide-rest-site-agent:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(IMAGE_REGISTRY)/carbide-rest-mock-core:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(IMAGE_REGISTRY)/carbide-rest-db:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(IMAGE_REGISTRY)/carbide-rest-cert-manager:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	@echo "Setting up PKI secrets for cert-manager..."
	NAMESPACE=carbide-rest ./scripts/setup-local.sh pki
	kubectl apply -k $(KUSTOMIZE_OVERLAY)
	kubectl -n carbide-rest wait --for=condition=ready pod -l app=postgres --timeout=240s
	kubectl -n carbide-rest wait --for=condition=ready pod -l app=carbide-rest-cert-manager --timeout=360s
	@echo "Configuring cert-manager.io ClusterIssuer..."
	kubectl apply -k deploy/kustomize/base/cert-manager-io/
	@echo "Creating temporal namespace..."
	kubectl create namespace temporal || true
	@echo "Creating Temporal certificates..."
	kubectl apply -k deploy/kustomize/base/temporal-helm/
	@echo "Waiting for Temporal certificates to be issued..."
	kubectl -n temporal wait --for=condition=Ready certificate/server-interservice-cert --timeout=240s || true
	kubectl -n temporal wait --for=condition=Ready certificate/server-cloud-cert --timeout=240s || true
	kubectl -n temporal wait --for=condition=Ready certificate/server-site-cert --timeout=240s || true
	kubectl -n carbide-rest wait --for=condition=Ready certificate/temporal-client-cert --timeout=240s || true
	@echo "Creating postgres-auth secret for Temporal Helm chart..."
	kubectl -n temporal create secret generic postgres-auth --from-literal=password=temporal || true
	@echo "Granting temporal user CREATEDB permission..."
	kubectl -n carbide-rest exec -it postgres-0 -- psql -U postgres -c "ALTER USER temporal CREATEDB; ALTER DATABASE temporal OWNER TO temporal; ALTER DATABASE temporal_visibility OWNER TO temporal;" || true
	@echo "Updating Helm chart dependencies..."
	helm dependency update temporal-helm/temporal/
	@echo "Installing Temporal via Helm chart..."
	helm upgrade --install temporal ./temporal-helm/temporal \
		--namespace temporal \
		--values ./temporal-helm/temporal/values-kind.yaml \
		--wait --timeout 16m || true
	@echo "Waiting for Temporal services..."
	kubectl -n temporal wait --for=condition=ready pod -l app.kubernetes.io/name=temporal,app.kubernetes.io/component=frontend --timeout=360s || true
	kubectl -n temporal wait --for=condition=ready pod -l app.kubernetes.io/name=temporal,app.kubernetes.io/component=history --timeout=360s || true
	kubectl -n temporal wait --for=condition=ready pod -l app.kubernetes.io/name=temporal,app.kubernetes.io/component=matching --timeout=360s || true
	kubectl -n temporal wait --for=condition=ready pod -l app.kubernetes.io/name=temporal,app.kubernetes.io/component=worker --timeout=360s || true
	@echo "Creating Temporal namespaces with TLS..."
	kubectl -n temporal exec deploy/temporal-admintools -- temporal operator namespace create cloud --address temporal-frontend:7233 \
		--tls-cert-path /var/secrets/temporal/certs/server-interservice/tls.crt \
		--tls-key-path /var/secrets/temporal/certs/server-interservice/tls.key \
		--tls-ca-path /var/secrets/temporal/certs/server-interservice/ca.crt \
		--tls-server-name interservice.server.temporal.local || true
	kubectl -n temporal exec deploy/temporal-admintools -- temporal operator namespace create site --address temporal-frontend:7233 \
		--tls-cert-path /var/secrets/temporal/certs/server-interservice/tls.crt \
		--tls-key-path /var/secrets/temporal/certs/server-interservice/tls.key \
		--tls-ca-path /var/secrets/temporal/certs/server-interservice/ca.crt \
		--tls-server-name interservice.server.temporal.local || true
	@echo "Temporal Helm deployment ready"
	kubectl -n carbide-rest wait --for=condition=ready pod -l app=keycloak --timeout=360s
	kubectl -n carbide-rest wait --for=condition=complete job/db --timeout=240s
	-kubectl -n carbide-rest wait --for=condition=ready pod -l app=carbide-rest-api --timeout=240s
	@echo "Waiting for workflow workers..."
	-kubectl -n carbide-rest wait --for=condition=ready pod -l app=cloud-worker --timeout=240s
	-kubectl -n carbide-rest wait --for=condition=ready pod -l app=site-worker --timeout=240s
	@echo "Waiting for Site Manager..."
	kubectl -n carbide-rest wait --for=condition=ready pod -l app=carbide-rest-site-manager --timeout=360s
	./scripts/setup-local.sh site-agent
	@echo ""
	@echo "================================================================================"
	@echo "Deployment complete!"
	@echo ""
	@echo "Temporal UI: http://localhost:8233"
	@echo "  - View running workflows and their execution history"
	@echo ""
	@echo "API: http://localhost:8388"
	@echo "Keycloak: http://localhost:8080"
	@echo "================================================================================"
	@echo ""
	@echo "Example: Get a token and list all sites:"
	@echo ""
	@echo '  TOKEN=$$(curl -s -X POST "http://localhost:8080/realms/carbide-dev/protocol/openid-connect/token" \'
	@echo '    -H "Content-Type: application/x-www-form-urlencoded" \'
	@echo '    -d "client_id=carbide-api" \'
	@echo '    -d "client_secret=carbide-local-secret" \'
	@echo '    -d "grant_type=password" \'
	@echo '    -d "username=admin@example.com" \'
	@echo '    -d "password=adminpassword" | jq -r .access_token)'
	@echo ""
	@echo '  curl -s "http://localhost:8388/v2/org/test-org/carbide/site" \'
	@echo '    -H "Authorization: Bearer $$TOKEN" | jq ".[].name"'
	@echo ""

# Setup site-agent with a real site created via the API
setup-site-agent:
	./scripts/setup-local.sh site-agent

# Verify the local deployment (health checks)
kind-verify:
	./scripts/setup-local.sh verify

# Run PKI E2E tests
test-pki:
	./scripts/test-pki.sh

# Run Temporal mTLS and rotation tests
test-temporal-e2e:
	./scripts/test-temporal.sh all

# =============================================================================
# Generated Go API Client
# =============================================================================

# Generate Go API SDK from OpenAPI spec using openapi-generator
# Requires: brew install openapi-generator
generate-sdk:
	openapi-generator generate \
		-i openapi/spec.yaml \
		-g go \
		-o sdk/standard \
		--package-name standard \
		--additional-properties=isGoSubmodule=true,enumClassPrefix=true \
		--global-property=apis,models,supportingFiles
	rm -rf sdk/standard/docs sdk/standard/api sdk/standard/README.md sdk/standard/test sdk/standard/.openapi-generator
	@echo "Client generated in sdk/standard/"
	go build ./sdk/standard/...
	@echo "Client compiles successfully"

# =============================================================================
# OpenAPI Spec Validation
# =============================================================================

# Validate OpenAPI spec using Redocly CLI (Docker)
lint-openapi:
	npx @redocly/cli lint ./openapi/spec.yaml

# Preview OpenAPI spec in Redoc UI (Docker)
preview-openapi:
	@echo "Starting Redoc UI at http://127.0.0.1:8090"
	docker run -it --rm -p 8090:80 -v ./openapi/spec.yaml:/usr/share/nginx/html/openapi.yaml -e SPEC_URL=openapi.yaml redocly/redoc

publish-openapi:
	npx @redocly/cli build-docs ./openapi/spec.yaml -o ./docs/index.html

# =============================================================================
# Pre-commit Hooks (TruffleHog Secret Detection)
# =============================================================================

# Install pre-commit hooks
pre-commit-install:
	@command -v pre-commit >/dev/null 2>&1 || { echo "Installing pre-commit..."; pip install pre-commit; }
	@command -v trufflehog >/dev/null 2>&1 || { echo "Installing trufflehog..."; brew install trufflehog || go install github.com/trufflesecurity/trufflehog/v3@latest; }
	pre-commit install
	pre-commit install --hook-type pre-push
	@echo "Pre-commit hooks installed successfully!"

# Run pre-commit on all files
pre-commit-run:
	pre-commit run --all-files

# Update pre-commit hooks to latest versions
pre-commit-update:
	pre-commit autoupdate
