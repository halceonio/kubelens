SHELL := /bin/bash

.PHONY: help backend-build backend-run backend-tidy frontend-install frontend-dev frontend-build frontend-preview dev kill-dev kube-sa keycloak-token keycloak-device-token keycloak-device-token-py docs-preview docs-build release-tag release

DEV_CONFIG ?= backend/config.test.yaml
DEV_KUBECONFIG ?= refs/kubelens-test.kubeconfig
DEV_FRONTEND_PORT ?= 3000
ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
DEV_CONFIG_ABS := $(abspath $(DEV_CONFIG))
DEV_KUBECONFIG_ABS := $(abspath $(DEV_KUBECONFIG))
REGISTRY ?= halceon
REGISTRY_ALT ?= nudevco
DOCKER_TAG ?= latest

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*##"}; {printf "%-22s %s\n", $$1, $$2}'

backend-build: ## Build backend server binary
	cd backend && go build ./cmd/server

backend-run: ## Run backend with test config
	mkdir -p backend/data
	cd backend && KUBELENS_CONFIG="$(DEV_CONFIG_ABS)" KUBECONFIG="$(DEV_KUBECONFIG_ABS)" go run ./cmd/server

backend-tidy: ## Tidy backend Go modules
	cd backend && go mod tidy

frontend-install: ## Install frontend dependencies
	cd frontend && npm install

frontend-dev: ## Run frontend dev server
	cd frontend && npm run dev

frontend-build: ## Build frontend
	cd frontend && npm run build

frontend-preview: ## Preview frontend build
	cd frontend && npm run preview

docs-preview: ## Preview docs locally (requires uv + mkdocs)
	uv run mkdocs serve --dev-addr 0.0.0.0:4000

docs-build: ## Build docs site
	uv run mkdocs build

dev: ## Run backend + frontend together (Ctrl+C to stop)
	@bash -c 'set -e; trap "kill 0" EXIT; mkdir -p backend/data; \
	cd backend && KUBELENS_CONFIG="$(DEV_CONFIG_ABS)" KUBECONFIG="$(DEV_KUBECONFIG_ABS)" go run ./cmd/server & \
	cd frontend && npm run dev -- --host 0.0.0.0 --port $(DEV_FRONTEND_PORT) & wait'

kill-dev: ## Kill all dev processes
	@killport $(DEV_FRONTEND_PORT) || true
	@killport 8080 || true

kube-sa: ## Provision service account and export kubeconfig (NS and SA required)
	@if [ -z "$(NS)" ] || [ -z "$(SA)" ]; then \
		echo "Usage: make kube-sa NS=<namespace> SA=<serviceaccount> [OUT=./kubelens.kubeconfig]"; \
		exit 1; \
	fi
	./scripts/provision-kubelens-sa.sh $(NS) $(SA) $(OUT)

keycloak-token: ## Fetch a Keycloak access token via password grant (requires env vars)
	./scripts/get-keycloak-token.sh

keycloak-device-token: ## Fetch a Keycloak access token via device authorization flow
	./scripts/get-keycloak-device-token.sh

keycloak-device-token-py: ## Fetch a Keycloak access token via device flow (python)
	uv run ./scripts/get_keycloak_device_token.py

release-tag: ## Create and push a git tag (VERSION required, e.g. v0.0.1)
	@if [ -z "$(VERSION)" ]; then \
		echo "Usage: make release-tag VERSION=v0.0.1"; \
		exit 1; \
	fi
	git tag -a "$(VERSION)" -m "Release $(VERSION)"
	git push origin "$(VERSION)"

release: ## Trigger release (runs ci-e2e before publishing)
	@if [ -z "$(VERSION)" ]; then \
		echo "Usage: make release VERSION=v0.0.1"; \
		exit 1; \
	fi
	$(MAKE) release-tag VERSION=$(VERSION)


.PHONY: docker-build
docker-build: ## Build and push Docker image to registry
	@docker buildx build \
		--platform linux/amd64 \
		--builder cloud-nudevco-default \
		-t $(REGISTRY)/kubelens:$(DOCKER_TAG) \
		-t $(REGISTRY_ALT)/kubelens:$(DOCKER_TAG) \
		--build-arg VITE_USE_MOCKS=false \
		-f docker/Dockerfile \
		--push .
	
