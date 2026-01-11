COMMON_MOCKERY_PARAMS=--disable-version-string --with-expecter --exported

.PHONY: help
help: ## Display this help message
	@echo "Available commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""

.DEFAULT_GOAL := help

.PHONY: check-go
check-go:
	@which go > /dev/null || (echo "Go is not installed.. Please install and try again."; exit 1)

.PHONY: check-lint
check-lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint is not installed. Please install and try again."; exit 1)

.PHONY: check-git
check-git:
	@which git > /dev/null || (echo "git is not installed. Please install and try again."; exit 1)

.PHONY: check-anvil
check-anvil:
	@which anvil > /dev/null || (echo "anvil is not installed. Please install Foundry (https://book.getfoundry.sh/getting-started/installation) and try again."; exit 1)

# Targets with required dependencies
lint: check-go check-lint

.PHONY: lint
lint: ## Runs the linter
	export "GOROOT=$$(go env GOROOT)" && $$(go env GOPATH)/bin/golangci-lint run --timeout 5m

.PHONY: generate-mocks
generate-mocks: ## Generate mock files for testing
	mockery ${COMMON_MOCKERY_PARAMS}

.PHONY: test
test: check-go ## Run all unit tests
	@echo "Running all tests..."
	@go test $$(go list ./... | grep -v /tests) -v -race

.PHONY: test-coverage
test-coverage: check-go ## Run all unit tests with coverage report
	@echo "Running tests with coverage..."
	@go test $$(go list ./... | grep -v /tests | grep -v /examples) -coverprofile=coverage.out -coverpkg=./... -v
	@echo ""
	@echo "📊 Coverage Summary:"
	@echo "===================="
	@go tool cover -func=coverage.out | grep -v "mocks" | awk ' \
		BEGIN { \
			print sprintf("%-80s %10s", "File", "Coverage"); \
			print "--------------------------------------------------------------------------------------------"; \
		} \
		/.go:/ { \
			split($$1, parts, ":"); \
			file = parts[1]; \
			coverage = $$NF; \
			gsub(/%/, "", coverage); \
			if (!(file in files)) { \
				files[file] = 1; \
				cmd = "go tool cover -func=coverage.out | grep \"" file ":\" | grep -v mocks | awk \"{sum+=\\$$NF; gsub(/%/, \\\"\\\", \\$$NF); total+=\\$$NF; count++} END {if(count>0) print total/count; else print 0}\""; \
				cmd | getline result; \
				close(cmd); \
				file_coverage[file] = result; \
			} \
		} \
		END { \
			sum = 0; \
			count = 0; \
			for (file in file_coverage) { \
				cov = file_coverage[file]; \
				sum += cov; \
				count++; \
				marker = ""; \
				if (cov >= 90) marker = " ✅"; \
				else if (cov >= 70) marker = " ✓"; \
				else if (cov < 50) marker = " ⚠️"; \
				printf("%-80s %9.1f%%%s\n", file, cov, marker); \
			} \
			print "============================================================================================"; \
			if (count > 0) printf("%-80s %9.1f%%\n", "TOTAL", sum/count); \
		}' | sort
	@echo ""
	@echo "💡 Tip: Run 'go tool cover -html=coverage.out' to view detailed coverage in browser"

.PHONY: test-quick
test-quick: check-go ## Run all unit tests without race detector (faster)
	@echo "Running tests (quick mode)..."
	@go test ./... -short

.PHONY: test-integration
test-integration: check-go check-anvil ## Run integration tests (requires Anvil/Foundry)
	@echo "Running integration tests..."
	@go test -tags=integration -v ./tests/... -timeout 5m

.PHONY: build-codegen
build-codegen: check-go ## Build the indexer code generator tool
	@echo "Building indexer-gen..."
	@go build -o bin/indexer-gen ./cmd/indexer-gen
	@echo "✅ Code generator built successfully: bin/indexer-gen"

.PHONY: build
build: check-go ## Build the ChainIndexor binary with built-in indexers
	@echo "Building ChainIndexor..."
	@go build -o bin/indexer ./cmd/indexer
	@echo "✅ ChainIndexor built successfully: bin/indexer"

.PHONY: build-all
build-all: build-codegen build ## Build all binaries
	@echo "✅ All binaries built successfully"

.PHONY: docs
docs: check-go ## Generate Swagger API documentation
	@echo "Generating Swagger API documentation..."
	@go run github.com/swaggo/swag/cmd/swag@latest init -g pkg/api/server.go --output ./pkg/api/docs
	@echo "✅ Swagger documentation generated successfully"
	@echo "   Access the API docs at: http://localhost:8080/swagger/index.html (when server is running)"
	@echo "   Spec files: pkg/api/docs/swagger.{json,yaml}"