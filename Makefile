GOLANGCI_LINT_VERSION=v1.52.2

all: prepare test lint
.PHONY: all

prepare:
	go mod download
	go mod tidy
.PHONY: prepare

test: prepare ## Run tests with code coverage print
	@go test -short -coverprofile coverage.tmp.out ./...
	@cat coverage.tmp.out | grep -v ".gen.go" > coverage.out
	@go tool cover -func coverage.out
.PHONY: test

lint: prepare ## Check source code with linter
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@golangci-lint --version
	@golangci-lint run -v .
.PHONY: lint

BLUE   = \033[36m
NC 	   = \033[0m

help: ## Prints this help and exits
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "${BLUE}%-30s${NC} %s\n", $$1, $$2}'
.PHONY: help
