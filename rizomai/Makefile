# RIZOMAI — comandos do monorepo (ADR-004). Uso: make <alvo>

GO      ?= go
BIN_DIR := bin

.PHONY: help run-api run-worker test vet lint build openapi-lint gen

help: ## Lista os alvos disponíveis
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

# --- Execução ---------------------------------------------------------------
run-api: ## Sobe o gateway HTTP (env PORT opcional; default 8080)
	$(GO) run ./api/cmd/rizomai-api

run-worker: ## Sobe o publish-worker (scaffold — fila River na Fase 2)
	$(GO) run ./workers/cmd/publish-worker

# --- Qualidade --------------------------------------------------------------
test: ## Roda todos os testes do módulo
	$(GO) test ./...

vet: ## go vet em todo o módulo
	$(GO) vet ./...

lint: vet ## Lint mínimo sem ferramentas globais (go vet). golangci-lint na Fase 2
	@echo "lint ok (go vet)"

build: ## Compila todos os binários
	$(GO) build -o $(BIN_DIR)/ ./api/cmd/rizomai-api ./workers/cmd/publish-worker ./workers/cmd/webhook-worker

# --- OpenAPI (ADR-005: a spec é a fonte da verdade) --------------------------
openapi-lint: ## Valida openapi/rizomai.yaml (requer node/npx)
	npx --yes @redocly/cli lint openapi/rizomai.yaml

# --- Geração de SDKs ----------------------------------------------------------
gen: ## Gera SDKs a partir da spec (Fase 2 — openapi-generator/hey-api)
	@echo "TODO(Fase 2): openapi-generator -> sdk/gen/{typescript,python,go} (ADR-004 §3, ADR-005 §1.6)"
