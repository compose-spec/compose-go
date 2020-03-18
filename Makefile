
.PHONY: build
build: ## Run tests
	go build ./...

.PHONY: test
test: ## Run tests
	go test ./... -v

.PHONY: fmt
fmt: ## Format go files
	go fmt ./...


.PHONY: setup
setup: ## Setup the precommit hook
	@which pre-commit > /dev/null 2>&1 || (echo "pre-commit not installed see README." && false)
	@pre-commit install
