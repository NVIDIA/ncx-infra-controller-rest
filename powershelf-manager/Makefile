CURDATE := $(shell date +%s)

gen-pb: ## Generate protobuf files
	@echo "Starting generate protobuf"
	    @protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		internal/proto/v1/*.proto
	@echo "Successfully generated protobuf"

psm:
	mkdir -p build
	go build -C build ..

container: psm
	docker build -t carbide-psm:$$(cat version)-$(CURDATE)-$$(git rev-parse HEAD) -t carbide-psm:latest .

# test runs the tests that will be done as preintegration checks in the exact same way as the pipeline, including the use of a build container.
test:
	DOCKER_BUILDKIT=1 docker build --no-cache -f Dockerfile.test -t carbide-psm-test:$$(cat version)-$(CURDATE)-$(CI_COMMIT_SHORT_SHA) .
	DOCKER_API_VERSION=1.41 docker rmi carbide-psm-test:$$(cat version)-$(CURDATE)-$(CI_COMMIT_SHORT_SHA)

.PHONY: gen-pb psm container runcontainer test
