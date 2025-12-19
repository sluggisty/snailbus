.PHONY: build run test clean generate-openapi validate-openapi swag help

# Build the main application
build:
	go build -o snailbus .

# Run the application
run: build
	./snailbus

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f snailbus
	rm -f openapi.json
	rm -rf docs
	rm -f cmd/generate-openapi/generate-openapi
	rm -f cmd/validate-openapi/validate-openapi

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

# Help target
help:
	@echo "Available targets:"
	@echo "  build              - Build the main application"
	@echo "  run                - Build and run the application"
	@echo "  test               - Run tests"
	@echo "  clean              - Remove build artifacts"
	@echo "  swag               - Generate OpenAPI spec from code annotations"
	@echo "  generate-spec      - Alias for swag"
	@echo "  generate-openapi   - Build the OpenAPI generator tool"
	@echo "  validate-openapi   - Build the OpenAPI validator tool"
	@echo "  generate-json      - Generate openapi.json from openapi.yaml"
	@echo "  validate           - Validate the OpenAPI specification"
	@echo "  install-tools      - Build all OpenAPI tools"
	@echo "  help               - Show this help message"

