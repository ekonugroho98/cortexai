BINARY     = cortexai
BUILD_DIR  = ./bin
CMD_PATH   = ./cmd/cortexai
IMAGE_NAME = cortexai
IMAGE_TAG  = latest

.PHONY: build run dev test test-security test-coverage lint fmt vet tidy clean \
        docker-build docker-run k8s-apply k8s-delete health help

## build: compile the binary
build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags="-w -s" -o $(BUILD_DIR)/$(BINARY) $(CMD_PATH)
	@echo "Binary: $(BUILD_DIR)/$(BINARY)"

## run: build and run the server
run: build
	$(BUILD_DIR)/$(BINARY)

## dev: run with local config (config/cortexai.json), falls back to example
dev: build
	@if [ -f config/cortexai.json ]; then \
		CORTEXAI_CONFIG=config/cortexai.json $(BUILD_DIR)/$(BINARY); \
	else \
		echo "config/cortexai.json not found, using example config"; \
		CORTEXAI_CONFIG=config/cortexai.example.json $(BUILD_DIR)/$(BINARY); \
	fi

## test: run all tests with race detector
test:
	go test ./... -v -race -count=1

## test-security: run security package tests only
test-security:
	go test ./internal/security/... -v -race

## test-coverage: generate HTML coverage report
test-coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
	@go tool cover -func=coverage.out | tail -1

## fmt: format all Go files
fmt:
	go fmt ./...
	@echo "Formatted"

## vet: run go vet (static analysis)
vet:
	go vet ./...

## lint: run golangci-lint (install: https://golangci-lint.run/usage/install/)
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed. See https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run ./...

## tidy: tidy and verify go modules
tidy:
	go mod tidy
	go mod verify

## clean: remove build artifacts and coverage files
clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html

## docker-build: build Docker image (multi-stage, ~15MB)
docker-build:
	docker build -f deploy/docker/Dockerfile -t $(IMAGE_NAME):$(IMAGE_TAG) .
	@echo "Image size: $$(docker image inspect $(IMAGE_NAME):$(IMAGE_TAG) --format='{{.Size}}' | awk '{printf "%.1fMB", $$1/1024/1024}')"

## docker-run: run Docker container
docker-run:
	docker run --rm -p 8000:8000 \
		-e ANTHROPIC_API_KEY=$(ANTHROPIC_API_KEY) \
		-e GCP_PROJECT_ID=$(GCP_PROJECT_ID) \
		-e CORTEXAI_API_KEYS=$(CORTEXAI_API_KEYS) \
		$(IMAGE_NAME):$(IMAGE_TAG)

## k8s-apply: create namespace and apply all k8s manifests
k8s-apply:
	kubectl apply -f deploy/k8s/namespace.yaml
	kubectl apply -f deploy/k8s/configmap.yaml
	kubectl apply -f deploy/k8s/secret.yaml
	kubectl apply -f deploy/k8s/deployment.yaml
	kubectl apply -f deploy/k8s/service.yaml
	kubectl apply -f deploy/k8s/hpa.yaml

## k8s-delete: remove all k8s resources including namespace
k8s-delete:
	kubectl delete namespace cortexai --ignore-not-found

## health: check server health
health:
	curl -s http://localhost:8000/health | jq .

## help: show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
