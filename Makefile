GOLANGCI_LINT_VERSION=v1.52.2

all: prepare test lint
.PHONY: all

prepare:
	go mod download
	go mod tidy
.PHONY: prepare

test: ## Run tests with code coverage print
	@go test -short -coverprofile coverage.tmp.out ./...
	@cat coverage.tmp.out | grep -v ".gen.go" > coverage.out
	@go tool cover -func coverage.out
.PHONY: test

lint: ## Check source code with linter
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@golangci-lint --version
	@golangci-lint run -v .
.PHONY: lint