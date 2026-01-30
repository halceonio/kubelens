SHELL := /bin/bash

.PHONY: help backend-build backend-run backend-tidy frontend-install frontend-dev frontend-build frontend-preview kube-sa

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*##"}; {printf "%-22s %s\n", $$1, $$2}'

backend-build: ## Build backend server binary
	cd backend && go build ./cmd/server

backend-run: ## Run backend with test config
	KUBELENS_CONFIG=backend/config.test.yaml go run ./backend/cmd/server

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

kube-sa: ## Provision service account and export kubeconfig (NS and SA required)
	@if [ -z "$(NS)" ] || [ -z "$(SA)" ]; then \
		echo "Usage: make kube-sa NS=<namespace> SA=<serviceaccount> [OUT=./kubelens.kubeconfig]"; \
		exit 1; \
	fi
	./scripts/provision-kubelens-sa.sh $(NS) $(SA) $(OUT)
