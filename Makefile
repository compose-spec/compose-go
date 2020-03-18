IMAGE_PREFIX=composespec/conformance-tests-

.PHONY: build
build: ## Run tests
	go build ./...

.PHONY: test
test: ## Run tests
	go test ./... -v

.PHONY: fmt
fmt: ## Format go files
	go fmt ./...

.PHONY: build-validate-image
build-validate-image:
	docker build . -f ci/Dockerfile -t $(IMAGE_PREFIX)validate

.PHONY: lint
lint: build-validate-image
	docker run --rm $(IMAGE_PREFIX)validate bash -c "golangci-lint run --config ./golangci.yml ./..."

.PHONY: setup
setup: ## Setup the precommit hook
	@which pre-commit > /dev/null 2>&1 || (echo "pre-commit not installed see README." && false)
	@pre-commit install
