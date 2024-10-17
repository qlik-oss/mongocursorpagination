REPO_NAME := mongocursorpagination
DOCKER_TEST_IMAGE := $(REPO_NAME)-test
VERSION ?= latest
ARGS ?= ""

# Rebuild dependencies
mod:
	@go mod tidy

# Update dependencies
mod-update:
	@go get -u ./bson
	@go get -u ./mgo
	@go get -u ./mongo
	@go get -u ./test/integration
	@$(MAKE) mod

LINT_VER := 1.61.0
LINT_NAME := "golangci-lint_$(LINT_VER)"

lint:
	if [ ! -e ./bin/$(LINT_NAME) ]; then (curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v$(LINT_VER)) && mv ./bin/golangci-lint ./bin/$(LINT_NAME); fi
	./bin/$(LINT_NAME) run --timeout 5m

# Build the Docker test image
build-test-docker:
	./scripts/build-docker.sh $(DOCKER_TEST_IMAGE) $(VERSION) Dockerfile.test

# Run unit tests
test-unit:
	./scripts/test-unit.sh

# Run unit tests with Code Climate coverage
test-unit-code-climate:
	./scripts/test-unit-code-climate.sh

# Run integration tests
test-integration:
	./scripts/test-integration.sh $(ARGS)

# Run integration tests
test-integration-code-climate:
	./scripts/test-integration-code-climate.sh $(ARGS)

.PHONY: mod
.PHONY: lint
.PHONY: build-test-docker
.PHONY: test-unit test-integration-code-climate
.PHONY: test-integration test-unit-code-climate
