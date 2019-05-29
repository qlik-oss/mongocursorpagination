REPO_NAME := mongocursorpagination
DOCKER_TEST_IMAGE := $(REPO_NAME)-test
VERSION ?= latest
ARGS ?= ""

# Install dependencies using go get
dep:
	go get -v -t -d ./...

# Lint the code
lint: dep
	./scripts/lint.sh

# Build the Docker test image
build-test-docker: dep
	./scripts/build-docker.sh $(DOCKER_TEST_IMAGE) $(VERSION) Dockerfile.test

# Run unit tests
test-unit: dep
	./scripts/test-unit.sh

# Run unit tests with Code Climate coverage
test-unit-code-climate: dep
	./scripts/test-unit-code-climate.sh

# Run integration tests
test-integration: dep
	./scripts/test-integration.sh $(ARGS)

# Run integration tests
test-integration-code-climate: dep
	./scripts/test-integration-code-climate.sh $(ARGS)

.PHONY: dep
.PHONY: lint
.PHONY: build-test-docker
.PHONY: test-unit test-integration-code-climate
.PHONY: test-integration test-unit-code-climate
