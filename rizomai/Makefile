# RIZOMAI — comandos do monorepo (ADR-004). Uso: make <alvo>

GO      ?= go
BIN_DIR := bin

.PHONY: help db-up db-down run-api run-worker seed test vet lint build openapi-lint gen

help: ## Lista os alvos disponíveis
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

# --- Banco (dev local) --------------------------------------------------------
db-up: ## Sobe o Postgres local (docker compose)
	docker compose -f deploy/docker-compose.yml up -d

db-down: ## Derruba o Postgres local (dados preservados no volume)
	docker compose -f deploy/docker-compose.yml down

# --- Execução ---------------------------------------------------------------
run-api: ## Sobe o gateway HTTP (env: DATABASE_URL, PORT; default 8080)
	$(GO) run ./api/cmd/rizomai-api

run-worker: ## Sobe o publish-worker (scaffold — fila River na Fase 3)
	$(GO) run ./workers/cmd/publish-worker

seed: ## Bootstrap dev: team + API key + profile + contas fictícias (imprime a chave)
	$(GO) run ./cmd/seed

# --- Qualidade --------------------------------------------------------------
test: ## Roda todos os testes do módulo
	$(GO) test ./...

vet: ## go vet em todo o módulo
	$(GO) vet ./...

lint: vet ## Lint mínimo sem ferramentas globais (go vet). golangci-lint na Fase 3
	@echo "lint ok (go vet)"

build: ## Compila todos os binários
	$(GO) build -o $(BIN_DIR)/ ./api/cmd/rizomai-api ./workers/cmd/publish-worker ./workers/cmd/webhook-worker ./cmd/seed

# --- OpenAPI (ADR-005: a spec é a fonte da verdade) --------------------------
openapi-lint: ## Valida openapi/rizomai.yaml (requer node/npx)
	npx --yes @redocly/cli lint openapi/rizomai.yaml

# --- Geração de SDKs ----------------------------------------------------------
gen: ## Gera SDKs a partir da spec (Fase 3 — openapi-generator/hey-api)
	@echo "TODO(Fase 3): openapi-generator -> sdk/gen/{typescript,python,go} (ADR-004 §3, ADR-005 §1.6)"
