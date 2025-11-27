COMMON_MOCKERY_PARAMS=--disable-version-string --with-expecter --exported

.PHONY: check-go
check-go:
	@which go > /dev/null || (echo "Go is not installed.. Please install and try again."; exit 1)

.PHONY: check-lint
check-lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint is not installed. Please install and try again."; exit 1)

.PHONY: check-git
check-git:
	@which git > /dev/null || (echo "git is not installed. Please install and try again."; exit 1)

# Targets with required dependencies
lint: check-go check-lint

.PHONY: lint
lint: ## Runs the linter
	export "GOROOT=$$(go env GOROOT)" && $$(go env GOPATH)/bin/golangci-lint run --timeout 5m

.PHONY: generate-mocks
generate-mocks:	
	mockery ${COMMON_MOCKERY_PARAMS}