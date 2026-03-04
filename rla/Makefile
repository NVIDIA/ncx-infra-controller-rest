#
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
#

CURDATE := $(shell date +%s)

gen-pb: ## Generate protobuf files
	@echo "Starting generate protobuf"
	@protoc -I proto/v1 \
		--go_out=pkg/proto/v1 --go_opt=paths=source_relative \
		--go-grpc_out=pkg/proto/v1 --go-grpc_opt=paths=source_relative \
		proto/v1/*.proto
	@echo "Successfully generated protobuf"

gen-carbideapi-pb: ## Generate protobuf files for carbide-api access, only needed for new messages or fields that we will actually use
	rm -rf internal/carbideapi/carbideproto/bare-metal-manager
	git clone ssh://git@github.com:NVIDIA/bare-metal-manager.git
	rm -f internal/carbideapigrpc/carbideproto/*.proto
	cp bare-metal-manager/crates/rpc/proto/*.proto internal/carbideapi/carbideproto/
	rm -rf metal-manager
# The protodefs are a bit unclean, and we need to make some tweaks to make them usable.
	cd internal/carbideapi && ./tweak_protodefs.sh
# Specify version of buf for consistency in pipeline builds
	which buf || go install github.com/bufbuild/buf/cmd/buf@v1.57.2
# Clean old generated files before regenerating
	rm -f internal/carbideapi/gen/*.pb.go
	cd internal/carbideapi && buf generate

gen-psmapi-pb: ## Generate protobuf files for PSM client access
	cp powershelf_manager/internal/proto/v1/powershelf-manager.proto internal/psmapi/psmproto/
# Specify version of buf for consistency in pipeline builds
	which buf || go install github.com/bufbuild/buf/cmd/buf@v1.57.2
	cd internal/psmapi && buf generate

gen-nsmapi-pb: ## Generate protobuf files for NSM client access
# Specify version of buf for consistency in pipeline builds
	which buf || go install github.com/bufbuild/buf/cmd/buf@v1.57.2
	rm -f internal/nsmapi/gen/*.pb.go
	cd internal/nsmapi && buf generate

gen-doc: ## Generate gRPC API documentation (Markdown for GitLab, HTML for local)
	@which protoc-gen-doc > /dev/null || go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest
	@mkdir -p docs
	protoc --doc_out=./docs --doc_opt=markdown,grpc-api.md \
		-I proto/v1 proto/v1/rla.proto
	protoc --doc_out=./docs --doc_opt=html,grpc-api.html \
		-I proto/v1 proto/v1/rla.proto
	@echo "Generated docs/grpc-api.md (for GitLab) and docs/grpc-api.html (for local)"

rla:
	mkdir -p build
	VERSION=$$(cat ../VERSION 2>/dev/null || echo "dev") && \
	BUILD_TIME=$$(date -u +"%Y-%m-%dT%H:%M:%SZ") && \
	GIT_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") && \
	go build -C build \
		-ldflags "-X 'github.com/nvidia/bare-metal-manager-rest/rla/pkg/metadata.Version=$${VERSION}' \
		          -X 'github.com/nvidia/bare-metal-manager-rest/rla/pkg/metadata.BuildTime=$${BUILD_TIME}' \
		          -X 'github.com/nvidia/bare-metal-manager-rest/rla/pkg/metadata.GitCommit=$${GIT_COMMIT}'" \
		..

container: rla
	docker build -t carbide-rla:$$(cat ../VERSION)-$(CURDATE)-$$(git rev-parse HEAD) -t carbide-rla:latest .

# test-local runs tests locally with a postgres container
# Requires: ./scripts/start-test-postgres.sh to be running
test-local:
	@echo "Running tests locally..."
	@echo "Make sure postgres is running: ./scripts/start-test-postgres.sh"
	@bash -c 'go fmt ./... > /tmp/gofmt; if grep -q . /tmp/gofmt; then echo "go fmt was unclean on the following files:"; cat /tmp/gofmt; exit 1; fi; rm -f /tmp/gofmt'
	go vet ./...
	DB_ADDR=localhost DB_PORT=30432 DB_USER=postgres DB_PASSWORD=postgres DB_DATABASE=rla_test go run . db migrate
	DB_ADDR=localhost DB_PORT=30432 DB_USER=postgres DB_PASSWORD=postgres DB_DATABASE=rla_test go test -v -p 1 ./...

# test runs tests in a docker container with PostgreSQL (used by CI)
# Includes: go fmt, go vet, migrations, and go test
test:
	DOCKER_BUILDKIT=1 docker build --no-cache -f Dockerfile.test -t carbide-rla-test:$$(cat ../VERSION)-$(CURDATE)-$(CI_COMMIT_SHORT_SHA) .
	docker rmi carbide-rla-test:$$(cat version)-$(CURDATE)-$(CI_COMMIT_SHORT_SHA)

.PHONY: gen-pb gen-psmapi-pb gen-nsmapi-pb gen-doc rla container releasecontainer runcontainer releasetest test test-local
