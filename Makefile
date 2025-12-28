.PHONY: build run test test-unit test-integration test-coverage test-docker test-integration-docker test-docker-up test-docker-down test-docker-clean clean generate-openapi validate-openapi swag help

# Build the main application
build:
	go build -o snailbus .

# Run the application
run: build
	./snailbus

# Run all tests (unit and integration)
test: test-unit test-integration

# Run unit tests only (tests that don't require a database)
# Uses -short flag to skip integration tests
test-unit:
	go test -short -v ./...

# Run integration tests (tests that require a database)
# Requires TEST_DATABASE_URL environment variable or will use default
# Note: Integration tests should be named with "Integration" in the test name
# or use build tags to separate them
test-integration:
	@if [ -z "$$TEST_DATABASE_URL" ]; then \
		echo "Warning: TEST_DATABASE_URL not set, using default: postgres://snail:snail_secret@localhost:5432/snailbus_test?sslmode=disable"; \
		echo "Tip: Use 'make test-docker-up' to start a test database with docker-compose"; \
	fi
	go test -v -run Integration ./...

# Start test database with docker-compose
test-docker-up:
	docker-compose -f docker-compose.test.yml up -d
	@echo "Waiting for test database to be ready..."
	@timeout=30; \
	while [ $$timeout -gt 0 ]; do \
		if docker-compose -f docker-compose.test.yml exec -T postgres-test pg_isready -U snail -d snailbus_test > /dev/null 2>&1; then \
			echo "Test database is ready!"; \
			echo "Set TEST_DATABASE_URL=postgres://snail:snail_secret@localhost:5434/snailbus_test?sslmode=disable"; \
			exit 0; \
		fi; \
		sleep 1; \
		timeout=$$((timeout-1)); \
	done; \
	echo "Warning: Database may not be ready yet"; \
	exit 1

# Stop test database
test-docker-down:
	docker-compose -f docker-compose.test.yml down

# Stop and remove test database (cleans up completely)
test-docker-clean:
	docker-compose -f docker-compose.test.yml down -v

# Run integration tests with docker-compose (starts DB, runs tests, stops DB)
test-integration-docker: test-docker-up
	@TEST_DATABASE_URL=postgres://snail:snail_secret@localhost:5434/snailbus_test?sslmode=disable \
	$(MAKE) test-integration; \
	EXIT_CODE=$$?; \
	$(MAKE) test-docker-down; \
	exit $$EXIT_CODE

# Run all tests (unit + integration) with docker-compose (starts DB, runs tests, stops DB)
test-docker: test-docker-up
	@TEST_DATABASE_URL=postgres://snail:snail_secret@localhost:5434/snailbus_test?sslmode=disable \
	$(MAKE) test; \
	EXIT_CODE=$$?; \
	$(MAKE) test-docker-down; \
	exit $$EXIT_CODE

# Run tests with coverage report
test-coverage:
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	go tool cover -func=coverage.out


# Build the OpenAPI generator tool
generate-openapi:
	go build -o cmd/generate-openapi/generate-openapi ./cmd/generate-openapi

# Build the OpenAPI validator tool
validate-openapi:
	go build -o cmd/validate-openapi/validate-openapi ./cmd/validate-openapi

# Generate JSON from YAML OpenAPI spec
generate-json: generate-openapi
	./cmd/generate-openapi/generate-openapi openapi.yaml openapi.json

# Validate OpenAPI spec
validate: validate-openapi
	./cmd/validate-openapi/validate-openapi openapi.yaml

# Install tools
install-tools: generate-openapi validate-openapi
	@echo "✓ OpenAPI tools built successfully"

# Generate OpenAPI spec from code annotations using swag
swag:
	@GOPATH=$$(go env GOPATH); \
	if [ -f "$$GOPATH/bin/swag" ]; then \
		$$GOPATH/bin/swag init -g main.go -o docs --parseDependency --parseInternal; \
	elif command -v swag > /dev/null; then \
		swag init -g main.go -o docs --parseDependency --parseInternal; \
	else \
		echo "swag not found. Installing..."; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
		$$(go env GOPATH)/bin/swag init -g main.go -o docs --parseDependency --parseInternal; \
	fi

# Generate OpenAPI spec (alias for swag)
generate-spec: swag

# Clean coverage reports
clean:
	rm -f snailbus
	rm -f openapi.json
	rm -rf docs
	rm -f cmd/generate-openapi/generate-openapi
	rm -f cmd/validate-openapi/validate-openapi
	rm -f coverage.out coverage.html

# Help target
help:
	@echo "Available targets:"
	@echo "  build              - Build the main application"
	@echo "  run                - Build and run the application"
	@echo "  test               - Run all tests (unit and integration)"
	@echo "  test-unit          - Run unit tests only (no database required)"
	@echo "  test-integration   - Run integration tests (requires database)"
	@echo "  test-docker        - Run all tests (unit + integration) with docker-compose (auto-starts/stops DB)"
	@echo "  test-integration-docker - Run integration tests with docker-compose (auto-starts/stops DB)"
	@echo "  test-coverage      - Run tests and generate coverage report"
	@echo "  test-docker-up     - Start test database with docker-compose"
	@echo "  test-docker-down   - Stop test database"
	@echo "  test-docker-clean  - Stop and remove test database (cleanup)"
	@echo "  clean              - Remove build artifacts and coverage reports"
	@echo "  swag               - Generate OpenAPI spec from code annotations"
	@echo "  generate-spec      - Alias for swag"
	@echo "  generate-openapi   - Build the OpenAPI generator tool"
	@echo "  validate-openapi   - Build the OpenAPI validator tool"
	@echo "  generate-json      - Generate openapi.json from openapi.yaml"
	@echo "  validate           - Validate the OpenAPI specification"
	@echo "  install-tools      - Build all OpenAPI tools"
	@echo "  help               - Show this help message"

