ARGS ?= ""

# Lint the code
lint:
	./scripts/lint.sh

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

.PHONY: lint
.PHONY: test-unit test-integration-code-climate
.PHONY: test-integration test-unit-code-climate
