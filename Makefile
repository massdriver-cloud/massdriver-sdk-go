

.PHONY: check
check: clean generate test ## Run tests and linter locally
	golangci-lint run

.PHONY: clean
clean:
	rm -rf ${API_DIR}/schema.graphql
	rm -rf ${API_DIR}/zz_generated.go

.PHONY: generate
generate:
	./scripts/graphql-gen.sh
	cd ${API_DIR} && go generate

.PHONY: test
test:
	go test ./... -cover

.PHONY: lint
lint:
	golangci-lint run
