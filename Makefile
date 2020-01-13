

build: ## Run tests
	go build ./...

test: ## Run tests
	go test ./... -v

fmt: ## Format go files
	go fmt ./...


setup: ## Setup the precommit hook
	@which pre-commit > /dev/null 2>&1 || (echo "pre-commit not installed see README." && false)
	@pre-commit install



.PHONY: build test fmt setup
