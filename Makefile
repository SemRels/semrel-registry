.PHONY: up down restart logs seed dev-web dev-admin build-api test-api clean help

ADMIN_TOKEN ?= dev-secret
API_DIR     := ./api
WEB_DIR     := ./web
ADMIN_DIR   := ./admin

##@ Infrastructure

up: ## Start PostgreSQL + Go API via Docker Compose
	docker compose up -d --build
	@echo ""
	@echo "  API:  http://localhost:8080/api/v1/plugins"
	@echo "  Web:  run 'make dev-web' to start the Astro dev server"
	@echo "  Admin: run 'make dev-admin' to start the React admin panel"

down: ## Stop all containers
	docker compose down

restart: down up ## Restart all containers

logs: ## Follow container logs
	docker compose logs -f

ps: ## Show running containers
	docker compose ps

##@ Data

seed: ## Import plugins.json into PostgreSQL
	@echo "Seeding plugins from plugins.json..."
	go -C api run cmd/seed/main.go \
		-db "$(or $(DATABASE_URL),postgres://dev:dev@localhost:5433/semrel_registry?sslmode=disable)" \
		-file ../plugins.json
	@echo "Done."

seed-direct: ## Seed directly via curl (requires API running on :8080)
	@for plugin in $$(cat plugins.json | python3 -c "import json,sys; [print(json.dumps(p)) for p in json.load(sys.stdin)['plugins']]"); do \
		curl -s -X POST http://localhost:8080/api/v1/plugins \
			-H "Content-Type: application/json" \
			-H "X-Admin-Token: $(ADMIN_TOKEN)" \
			-d "$$plugin" > /dev/null && echo "  ✓ $$plugin" || echo "  ✗ failed"; \
	done

##@ Development

dev-web: ## Start Astro web dev server (port 3000)
	cd $(WEB_DIR) && npm install && npm run dev

dev-admin: ## Start React admin dev server (port 5173)
	@if [ ! -d "$(ADMIN_DIR)" ]; then echo "Admin panel not yet created. Run: make scaffold-admin"; exit 1; fi
	cd $(ADMIN_DIR) && npm install && npm run dev

dev-api: ## Run Go API from project root (loads root .env automatically)
	go -C api run main.go

##@ Build

build-api: ## Build Go API binary to bin/api
	@mkdir -p bin
	go -C api build -o ../bin/api main.go

build-web: ## Build Astro web for production
	cd $(WEB_DIR) && npm ci && npm run build

build-admin: ## Build React admin for production
	cd $(ADMIN_DIR) && npm ci && npm run build

##@ Testing

test-api: ## Run Go API tests
	cd $(API_DIR) && go test ./... -count=1

test-api-watch: ## Run Go API tests in watch mode
	cd $(API_DIR) && gotestsum --watch ./...

##@ Cleanup

clean: ## Remove build artifacts
	rm -rf bin/
	rm -rf $(WEB_DIR)/dist/
	rm -rf $(ADMIN_DIR)/dist/ 2>/dev/null || true

clean-db: ## Drop and recreate PostgreSQL volume
	docker compose down -v

##@ Help

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\n\033[1mUsage:\033[0m make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
