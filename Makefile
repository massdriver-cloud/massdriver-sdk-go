# Massdriver Go SDK — common dev tasks.
#
# Run `make help` for a list of targets.

.DEFAULT_GOAL := help

GO ?= go
GOLANGCI_LINT ?= golangci-lint
GENQLIENT_DIR := massdriver/internal/gen
SCHEMA_URL ?= https://api.massdriver.cloud/graphql/v2/schema.graphql
SCHEMA_FILE := $(GENQLIENT_DIR)/schema.graphql

.PHONY: help
help: ## Show available targets and what they do
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage: make <target>\n\nTargets:\n"} \
		/^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' \
		$(MAKEFILE_LIST)

# ---------------------------------------------------------------------------
# Build / test / quality
# ---------------------------------------------------------------------------

.PHONY: build
build: ## Compile the SDK
	$(GO) build ./...

.PHONY: vet
vet: ## Run `go vet` on all packages
	$(GO) vet ./...

.PHONY: lint
lint: ## Run golangci-lint
	$(GOLANGCI_LINT) run

.PHONY: test
test: ## Run unit tests
	$(GO) test ./... -cover

.PHONY: test-integration
test-integration: ## Run integration tests (requires MASSDRIVER_API_KEY + MASSDRIVER_ORGANIZATION_ID)
	$(GO) test -tags=integration -count=1 ./...

.PHONY: check
check: vet lint test ## Run vet, lint, and tests — everything you'd want before pushing

# ---------------------------------------------------------------------------
# Code generation
# ---------------------------------------------------------------------------

.PHONY: schema
schema: ## Fetch the latest GraphQL schema into $(SCHEMA_FILE)
	curl --fail --silent --show-error -o $(SCHEMA_FILE) $(SCHEMA_URL)

.PHONY: generate
generate: schema ## Pull the schema and regenerate the genqlient client
	$(GO) generate ./$(GENQLIENT_DIR)/...
