GOLANGCI_LINT_VERSION=v1.52.2

all: test lint
.PHONY: all

test: ## Run tests with code coverage print
	@go test -short -coverprofile coverage.out.tmp ./...
	@cat coverage.out.tmp | grep -v ".gen.go" > coverage.out
	@go tool cover -func coverage.out
.PHONY: test

lint: ## Check source code with linter
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@golangci-lint --version
	@golangci-lint run -v .
.PHONY: lint