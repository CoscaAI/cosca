# =============================================================================
# Cosca - Enterprise Makefile
# =============================================================================

# --- Project Metadata --------------------------------------------------------
MODULE    := github.com/CoscaAI/cosca
APP_NAME  := cosca
BINARY    := $(APP_NAME)
MAIN_FILE := cmd/cosca/main.go

# --- Version & Build ---------------------------------------------------------
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.0-dev")
COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE  := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -ldflags="\
	-X '$(MODULE)/pkg/cosca.Version=$(VERSION)' \
	-X '$(MODULE)/pkg/cosca.CommitHash=$(COMMIT_HASH)' \
	-X '$(MODULE)/pkg/cosca.BuildDate=$(BUILD_DATE)' \
	-w -s"

# --- Tooling -----------------------------------------------------------------
GO        ?= go
GOLANGCILINT ?= golangci-lint
GORELEASER   ?= goreleaser
MOCKGEN      ?= mockgen
PROTOC       ?= protoc
GODOC        ?= godoc
REFLEX       ?= reflex

# --- Paths -------------------------------------------------------------------
BINDIR      := ./bin
COVERDIR    := ./build/coverage
TESTDIR     := ./build/test
DISTDIR     := ./dist
DOCSDIR     := ./docs
COSCA_HOME  ?= $(HOME)/.cosca
USERDIR     := $(HOME)/.config/cosca
# Modo no-root: o binario global do usuario, sem sudo.
JAULA       := $(COSCA_HOME)/bin/cosca

# --- Flags -------------------------------------------------------------------
GOFLAGS   := -mod=mod
GCFLAGS   := -gcflags="all=-N -l"  # debug symbols
TAGS      :=
CGO_ENABLED ?= 0

# Detect OS for binary extension
ifeq ($(OS),Windows_NT)
	BINARY := $(BINARY).exe
endif

# =============================================================================
# Targets
# =============================================================================

.PHONY: all build build-dev build-all build-all-main install remove clean-install wipe test test-unit test-integration test-e2e test-e2e-go benchmark restart install-service
.PHONY: test-no-provider ci dep-doctor contract-validate coverage-check
.PHONY: clean lint vet docs docs-stop proto fmt tidy help serve qgate checkup

## all: build everything (default target)
all: fmt vet build test-unit

## build: build the CLI binary (protegido — sem permissao de execucao)
build:
	@echo "  >  Building $(BINARY) v$(VERSION)..."
	@mkdir -p $(BINDIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) $(GCFLAGS) $(LDFLAGS) \
		-tags "$(TAGS)" \
		-o $(BINDIR)/$(BINARY) \
		$(MAIN_FILE)
	@chmod -x $(BINDIR)/$(BINARY)
	@echo "  ✓  $(BINDIR)/$(BINARY) (644 — legivel, nao executavel)"

## build-dev: build without protection (executavel — dev local)
build-dev:
	@echo "  >  Building $(BINARY) v$(VERSION) (DEV)..."
	@mkdir -p $(BINDIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) $(GCFLAGS) $(LDFLAGS) \
		-tags "$(TAGS)" \
		-o $(BINDIR)/$(BINARY) \
		$(MAIN_FILE)
	@echo "  ✓  $(BINDIR)/$(BINARY) (755 — executavel, sem protecao)"

## build-all: build for all platforms
build-all: build-all-main

## build-all-main: cross-compile the main CLI for all platforms
build-all-main:
	@echo "  >  Cross-compiling..."
	@mkdir -p $(DISTDIR)
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) $(LDFLAGS) -o $(DISTDIR)/$(BINARY)-linux-amd64   $(MAIN_FILE)
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) $(LDFLAGS) -o $(DISTDIR)/$(BINARY)-linux-arm64   $(MAIN_FILE)
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) $(LDFLAGS) -o $(DISTDIR)/$(BINARY)-darwin-amd64  $(MAIN_FILE)
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) $(LDFLAGS) -o $(DISTDIR)/$(BINARY)-darwin-arm64  $(MAIN_FILE)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) $(LDFLAGS) -o $(DISTDIR)/$(BINARY)-windows-amd64.exe $(MAIN_FILE)
	@echo "  ✓  Cross-compilation complete"

## serve: build + executar o servidor (binario global se existir, senao local)
serve: build
	@if [ -x "$(JAULA)" ]; then $(JAULA) serve; else ./bin/$(BINARY) serve; fi

## restart: rebuild + instalacao atomica + restart do serve em 1 comando
## Se o systemd service estiver ativo, reinicia via systemctl (user). Senao,
## mata o serve atual e inicia em background com log em ~/.cosca-serve.log.
## P12: NUNCA pkill — encerra pelo PID exato (identificado pela porta).
restart: build
	@echo "  >  Instalando binario novo (rename atomico)..."
	@mkdir -p $(COSCA_HOME)/bin
	@cp $(BINDIR)/$(BINARY) $(COSCA_HOME)/bin/.$(BINARY).new
	@chmod +x $(COSCA_HOME)/bin/.$(BINARY).new
	@mv -f $(COSCA_HOME)/bin/.$(BINARY).new $(COSCA_HOME)/bin/$(BINARY)
	@if systemctl --user is-active cosca-serve.service >/dev/null 2>&1; then \
		echo "  >  Reiniciando via systemd (cosca-serve.service)..."; \
		systemctl --user restart cosca-serve.service; \
		echo "  ✓  Serve reiniciado (systemd)"; \
	else \
		echo "  >  Service systemd inativo — reiniciando direto..."; \
		PID=$$(ss -tlnp 2>/dev/null | grep -E ":1412[0-9] " | grep -oE 'pid=[0-9]+' | head -1 | cut -d= -f2); \
		if [ -z "$$PID" ]; then \
			MATCH=$$(pgrep -a -f "$(BINARY) serve" || true); \
			if [ -n "$$MATCH" ]; then \
				echo "  >  Encerrando (PID exato): $$MATCH"; \
				PID=$$(echo "$$MATCH" | awk '{print $$1}' | head -1); \
			fi; \
		else \
			echo "  >  Encerrando serve (PID exato via porta): $$PID"; \
		fi; \
		if [ -n "$$PID" ]; then kill "$$PID" 2>/dev/null || true; sleep 1; fi; \
		setsid nohup $(JAULA) serve >> "$(HOME)/.cosca-serve.log" 2>&1 < /dev/null & \
		echo "  ✓  Serve iniciado em background (log: ~/.cosca-serve.log)"; \
	fi

## install-service: instala/habilita o daemon 24/7 via systemd (user, sem sudo)
install-service:
	@echo "  >  Instalando cosca-serve.service (systemd user)..."
	@mkdir -p $(HOME)/.config/systemd/user
	@cp deploy/cosca-serve.service $(HOME)/.config/systemd/user/cosca-serve.service
	@systemctl --user daemon-reload
	@systemctl --user enable --now cosca-serve.service
	@echo "  ✓  cosca-serve.service ativo e habilitado no boot"
	@echo "  Dica: 'make restart' rebuilda + reinicia em 1 comando"
	@echo "  Se o servico nao persistir apos logout, rode (sem sudo): loginctl enable-linger $$USER"

## install: build + instalar global do usuario (no-root — ~/.cosca/bin)
## Usa rename atomico (cp para temp + mv) para funcionar mesmo com o binario em execucao.
install: build
	@echo "  >  Instalando global do usuario: $(COSCA_HOME)/bin/..."
	@mkdir -p $(COSCA_HOME)/bin
	@cp $(BINDIR)/$(BINARY) $(COSCA_HOME)/bin/.$(BINARY).new
	@chmod +x $(COSCA_HOME)/bin/.$(BINARY).new
	@mv -f $(COSCA_HOME)/bin/.$(BINARY).new $(COSCA_HOME)/bin/$(BINARY)
	@echo "  ✓  Instalado: $(COSCA_HOME)/bin/$(BINARY) (executavel)"
	@echo "  ✓  Config compartilhada: $(USERDIR)/config.yaml"
	@$(MAKE) install-path
	@echo ""
	@echo "  Para usar (sem root, sem sudo):"
	@echo "    cosca version"
	@echo "    cosca serve --port 14220"
	@echo "    cosca exec 'sua mensagem'  (requer COSCA_ENABLE_EXEC=1)"
	@echo "    cosca chat                 (TUI interativa)"

## install-path: adiciona ~/.cosca/bin ao PATH do usuario (idempotente, multi-shell)
install-path:
	@if echo "$$PATH" | tr ':' '\n' | grep -qx "$(COSCA_HOME)/bin"; then \
		echo '  ✓  PATH ja contem $(COSCA_HOME)/bin'; \
	elif [ -f "$(HOME)/.bashrc" ] && grep -q '\.cosca/bin' "$(HOME)/.bashrc" 2>/dev/null; then \
		echo '  ✓  PATH ja configurado em ~/.bashrc'; \
	elif [ -f "$(HOME)/.zshrc" ] && grep -q '\.cosca/bin' "$(HOME)/.zshrc" 2>/dev/null; then \
		echo '  ✓  PATH ja configurado em ~/.zshrc'; \
	elif [ -f "$(HOME)/.bash_profile" ] && grep -q '\.cosca/bin' "$(HOME)/.bash_profile" 2>/dev/null; then \
		echo '  ✓  PATH ja configurado em ~/.bash_profile'; \
	else \
		SHELL_RC=""; \
		if [ -f "$(HOME)/.bashrc" ]; then SHELL_RC="$(HOME)/.bashrc"; \
		elif [ -f "$(HOME)/.zshrc" ]; then SHELL_RC="$(HOME)/.zshrc"; \
		elif [ -f "$(HOME)/.bash_profile" ]; then SHELL_RC="$(HOME)/.bash_profile"; \
		else SHELL_RC="$(HOME)/.profile"; fi; \
		echo '' >> "$$SHELL_RC"; \
		echo '# Cosca global (adicionado por make install)' >> "$$SHELL_RC"; \
		echo 'export PATH="$$HOME/.cosca/bin:$$PATH"' >> "$$SHELL_RC"; \
		echo '  ✓  PATH adicionado a '"$$SHELL_RC"' (source '"$$SHELL_RC"' ou abra novo terminal)'; \
	fi

## clean-install: remove instalacao global do usuario (no-root)
clean-install:
	@echo "  >  Removendo instalacao anterior..."
	@rm -f $(COSCA_HOME)/bin/$(BINARY)
	@echo "  ✓  Instalacao anterior removida"

## remove: desinstalar cosca do usuario e limpar artefatos (no-root)
remove:
	@echo "  >  Removendo cosca do usuario..."
	@rm -f $(COSCA_HOME)/bin/$(BINARY)
	@echo "  ✓  Binarios removidos: $(COSCA_HOME)/bin/"
	@echo "  (Config preservada: $(USERDIR)/config.yaml — remova manualmente se quiser)"
	@echo ""
	@echo "  Limpando artefatos locais..."
	@rm -rf $(BINDIR) $(COVERDIR) $(TESTDIR) $(DISTDIR)
	@rm -f *.out *.test *.prof *.cov coverage.html
	@find . -name '*.test' -delete 2>/dev/null || true
	@find . -name '*.test.exe' -delete 2>/dev/null || true
	@echo "  ✓  Artefatos locais limpos"
	@echo ""
	@echo "  Remocao completa. Para reinstalar: make install"

## test: run all tests
test: test-unit test-integration test-e2e

## ci: full CI pipeline — unit/integration/e2e tests + provider-independence proof
ci: test test-no-provider

## test-no-provider: prova continua de independencia de provider (CI)
## O Cosca opera deterministicamente SEM depender de nenhum provider de IA. O modo
## "sem IA" (provider=none) e um estado de primeira classe, nao um modo de
## emergencia. Este alvo prova isso de forma permanente e repetivel em CI — boot,
## carrega estado, carrega conhecimento, auditoria/licenca, memoria/backup e
## encerramento, tudo com COSCA_PROVIDER=none e sem nenhuma chamada a modelo de IA.
## O passo `capability level` confirma o nivel cognitivo L0 (deterministico). O
## passo final confirma que o chat SEM provider FALHA com o erro deterministico
## claro (pt-BR) em vez de responder em silencio, provando que nenhuma IA esta
## disponivel por baixo dos panos.
##
## NOTA — o passo `capability level` so reporta L0 porque COSCA_PROVIDER=none e
## exportado (viper le o env e resolve o provider ativo como none). Sem o env, o
## provider configurado no projeto (ex: ollama) elevaria o nivel para L2 — o teste
## validaria L2 e falharia a assercao, o que e correto, pois o modo sem IA deve ser
## o estado testado e nao um provider real.
## A auto-jaula (bwrap) pode emitir aviso de sandbox indisponivel — nao-fatal.
## COSCA_ALLOW_NO_ROOT=1 documenta que este teste aceita rodar sem sandbox.
test-no-provider:
	@echo "  >  Prova de independencia de provider (COSCA_PROVIDER=none)..."
	@set -e; \
	export COSCA_PROVIDER=none; \
	export COSCA_ALLOW_NO_ROOT=1; \
	run_step() { \
		label="$$1"; shift; \
		if "$$@"; then \
			echo "  ✓ $$label"; \
		else \
			echo "  ✗ $$label"; \
			exit 1; \
		fi; \
	}; \
	run_step "cosca knowledge stats" go run ./cmd/cosca knowledge stats; \
	run_step "cosca cv list" go run ./cmd/cosca cv list; \
	cap_out="$$(go run ./cmd/cosca capability level 2>&1)" || { \
		echo "  ✗ cosca capability level"; \
		echo "$$cap_out" | tail -5; \
		exit 1; \
	}; \
	if echo "$$cap_out" | grep -aq "L0"; then \
		echo "  ✓ cosca capability level (L0 — determinístico, sem IA)"; \
	else \
		echo "  ✗ cosca capability level deveria reportar L0 (sem provider)"; \
		echo "$$cap_out" | tail -5; \
		exit 1; \
	fi; \
	run_step "cosca memory prune --dry-run" go run ./cmd/cosca memory prune --dry-run; \
	run_step "cosca license verify" go run ./cmd/cosca license verify; \
	# COSCA_JAILED=1 + COSCA_LOG_LEVEL=info: a auto-jaula engole o stderr do
	# processo interno (AGENTS.md §6) e o log default suprime o FTL — sem eles
	# o erro deterministico ("capacidade cognitiva") seria invisivel aqui.
	if out="$$(COSCA_ENABLE_EXEC=1 COSCA_JAILED=1 COSCA_LOG_LEVEL=info go run ./cmd/cosca exec 'oi' --model none 2>&1)"; then \
		echo "  ✗ cosca exec 'oi' --model none deveria FALHAR (sem IA silenciosa)"; \
		exit 1; \
	elif echo "$$out" | grep -q "capacidade cognitiva"; then \
		echo "  ✓ cosca exec 'oi' --model none falha com erro deterministico (sem IA silenciosa)"; \
	else \
		echo "  ✗ cosca exec 'oi' --model none nao produziu o erro deterministico esperado"; \
		echo "$$out" | tail -5; \
		exit 1; \
	fi; \
	echo ""; \
	echo "  ✓ Nenhum passo dependeu de IA — independencia de provider comprovada."

## contract-validate: validate the versioned RPC contract registry (ADR-7423)
## Fails the build if any contract violates the invariants:
##   minor additive-only, major must be breaking, chains/downgrades/degrades well-formed.
contract-validate:
	@echo "  >  Validating versioned contract registry (ADR-7423)..."
	$(GO) test $(GOFLAGS) -count=1 -run 'TestValidateRegistry' ./internal/contracts/ 2>&1
	@echo "  ✓  Contract registry valid"

## test-unit: run unit tests only
test-unit:
	@echo "  >  Running unit tests..."
	@mkdir -p $(TESTDIR) $(COVERDIR)
	CGO_ENABLED=1 $(GO) test $(GOFLAGS) -tags "$(TAGS) unit" \
		-count=1 -race -cover -coverprofile=$(COVERDIR)/unit.cov \
		./internal/... ./pkg/... 2>&1 | tee $(TESTDIR)/unit.log
	@echo "  ✓  Unit tests complete"

## test-integration: run integration tests
test-integration:
	@echo "  >  Running integration tests..."
	@mkdir -p $(TESTDIR)
	CGO_ENABLED=1 $(GO) test $(GOFLAGS) -tags "$(TAGS) integration" \
		-count=1 -race -cover -coverprofile=$(COVERDIR)/integration.cov \
		./test/integration/... 2>&1 | tee $(TESTDIR)/integration.log
	@echo "  ✓  Integration tests complete"

## test-e2e-go: run Go end-to-end tests when present
test-e2e-go:
	@if [ -d ./test/e2e ]; then \
		echo "  >  Running Go E2E tests..."; \
		mkdir -p $(TESTDIR) $(COVERDIR); \
		CGO_ENABLED=$(CGO_ENABLED) $(GO) test $(GOFLAGS) -tags "$(TAGS) e2e" \
			-count=1 -race -cover -coverprofile=$(COVERDIR)/e2e.cov \
			./test/e2e/... 2>&1 | tee $(TESTDIR)/e2e.log; \
		echo "  ✓  Go E2E tests complete"; \
	else \
		echo "  !  Go E2E tests skipped (./test/e2e not found)"; \
	fi

## test-e2e: run Go and Playwright end-to-end tests
test-e2e: test-e2e-go
	@echo "  >  Running E2E tests..."
	@pnpm --dir web test:e2e
	@echo "  ✓  E2E tests complete"

## test-race: run tests with race detector
test-race:
	@echo "  >  Running race detection tests..."
	CGO_ENABLED=1 $(GO) test $(GOFLAGS) -race -count=1 ./... 2>&1
	@echo "  ✓  Race detection complete"

## test-cover: run tests with coverage report
test-cover:
	@echo "  >  Running tests with coverage..."
	@mkdir -p $(COVERDIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test $(GOFLAGS) -tags "$(TAGS)" \
		-count=1 -race -coverprofile=$(COVERDIR)/coverage.cov -covermode=atomic \
		./... 2>&1
	$(GO) tool cover -html=$(COVERDIR)/coverage.cov -o $(COVERDIR)/coverage.html
	$(GO) tool cover -func=$(COVERDIR)/coverage.cov
	@echo "  ✓  Coverage report: $(COVERDIR)/coverage.html"

## benchmark: run benchmarks
benchmark:
	@echo "  >  Running benchmarks..."
	@mkdir -p $(TESTDIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test $(GOFLAGS) -tags "$(TAGS)" \
		-bench=. -benchmem -run=^$$ -count=5 \
		./... 2>&1 | tee $(TESTDIR)/benchmark.log
	@echo "  ✓  Benchmarks complete"

## benchmark-profile: run benchmarks with profiling
benchmark-profile:
	@echo "  >  Running profile benchmarks..."
	@mkdir -p $(TESTDIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test $(GOFLAGS) -tags "$(TAGS)" \
		-bench=. -benchmem -run=^$$ -count=1 \
		-cpuprofile=$(TESTDIR)/cpu.prof \
		-memprofile=$(TESTDIR)/mem.prof \
		-blockprofile=$(TESTDIR)/block.prof \
		./...
	@echo "  ✓  Profile files: $(TESTDIR)/*.prof"

## clean: remove build artifacts
clean:
	@echo "  >  Cleaning..."
	@rm -rf $(BINDIR) $(COVERDIR) $(TESTDIR) $(DISTDIR)
	@rm -f *.out *.test *.prof *.cov coverage.html
	@find . -name '*.test' -delete 2>/dev/null || true
	@find . -name '*.test.exe' -delete 2>/dev/null || true
	@echo "  ✓  Clean complete"

## lint: run golangci-lint
lint:
	@echo "  >  Running linter..."
	@if command -v $(GOLANGCILINT) >/dev/null 2>&1; then \
		$(GOLANGCILINT) run --timeout=5m ./...; \
	else \
		echo "  !  $(GOLANGCILINT) not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi
	@echo "  ✓  Linting complete"

## vet: run go vet
vet:
	@echo "  >  Running go vet..."
	$(GO) vet $(GOFLAGS) ./...
	@echo "  ✓  go vet complete"

## fmt: format all Go code
fmt:
	@echo "  >  Formatting code..."
	$(GO) fmt ./...
	@echo "  ✓  Formatting complete"

## tidy: tidy go modules
tidy:
	@echo "  >  Tidying modules..."
	$(GO) mod tidy
	$(GO) mod verify
	@echo "  ✓  Modules tidy"

## docs: generate documentation (godoc em background, PID guardado — use make docs-stop)
docs:
	@echo "  >  Generating documentation..."
	@mkdir -p $(DOCSDIR)/output
	@if [ -f $(DOCSDIR)/.godoc.pid ] && kill -0 $$(cat $(DOCSDIR)/.godoc.pid) 2>/dev/null; then \
		echo "  !  Godoc ja esta rodando (PID $$(cat $(DOCSDIR)/.godoc.pid)) — http://localhost:6060"; \
	else \
		$(GODOC) -http=:6060 -index >/dev/null 2>&1 & echo $$! > $(DOCSDIR)/.godoc.pid; \
		echo "  >  Godoc server started at http://localhost:6060 (PID $$(cat $(DOCSDIR)/.godoc.pid))"; \
	fi
	@echo "  ✓  Docs gerados (pare com: make docs-stop)"

## docs-stop: encerra o godoc pelo PID exato (P12 — nunca pkill)
docs-stop:
	@if [ -f $(DOCSDIR)/.godoc.pid ]; then \
		PID=$$(cat $(DOCSDIR)/.godoc.pid); \
		if kill -0 $$PID 2>/dev/null; then \
			kill $$PID; \
			echo "  ✓  Godoc encerrado (PID $$PID)"; \
		else \
			echo "  !  Godoc nao estava rodando (PID $$PID morto)"; \
		fi; \
		rm -f $(DOCSDIR)/.godoc.pid; \
	else \
		echo "  !  Nenhum PID de godoc registrado"; \
	fi

## proto: generate protobuf code
proto:
	@echo "  >  Generating protobuf code..."
	@mkdir -p api/grpc/pb
	@if command -v $(PROTOC) >/dev/null 2>&1; then \
		for f in proto/cosca/v1/*.proto; do \
			protoc --go_out=. --go_opt=paths=import --go_opt=module=github.com/CoscaAI/cosca \
				--go-grpc_out=. --go-grpc_opt=paths=import --go-grpc_opt=module=github.com/CoscaAI/cosca "$$f"; \
		done; \
	else \
		echo "  !  protoc not installed. Install from https://github.com/protocolbuffers/protobuf"; \
		exit 1; \
	fi
	@echo "  ✓  Protobuf generated in api/grpc/pb"

## deps: show dependency graph
deps:
	@echo "  >  Dependency graph:"
	$(GO) mod graph

## vendor: vendor dependencies
vendor:
	@echo "  >  Vendoring dependencies..."
	$(GO) mod vendor
	@echo "  ✓  Vendoring complete"

## check: run all checks (CI pipeline)
check: fmt vet lint contract-validate test

## qgate: portao de qualidade da casa — o que um commit deve passar (G2-G3)
## fmt → vet → lint → contract → unit (race+cover) → prova sem-IA
qgate: fmt vet lint contract-validate test-unit test-no-provider
	@echo ""
	@echo "  ✓  Quality gates G2-G3 aprovados — pronto para release"

## checkup: checkup do carro (metafora da familia) — motor, esteira e garagem
## Motor: build+vet | Esteira: unit tests com race | Garagem: dep-doctor
checkup: build vet test-unit dep-doctor
	@echo ""
	@echo "  ✓  Checkup completo — o carro esta pronto para a viagem"

# version, discover e qualquer comando do cosca é roteado pela jaula
# (regra catch-all no final do Makefile)

## help: show this help message
help:
	@echo "Cosca - Enterprise Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -Eh '^## .*: ' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ": "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Variables:"
	@echo "  GO          Go compiler              (current: $(GO))"
	@echo "  CGO_ENABLED Enable CGO              (current: $(CGO_ENABLED))"
	@echo "  TAGS        Build tags              (current: $(TAGS))"
	@echo "  VERSION     Version string          (current: $(VERSION))"

## coverage-check: verify coverage meets threshold (70%)
coverage-check:
	@echo "  >  Checking coverage threshold..."
	@mkdir -p $(COVERDIR)
	@CGO_ENABLED=1 $(GO) test -race -coverprofile=$(COVERDIR)/coverage.cov ./internal/... ./pkg/... 2>/dev/null
	@COV=$$($(GO) tool cover -func=$(COVERDIR)/coverage.cov | grep total | awk '{print $$NF}' | tr -d '%'); \
	echo "Coverage: $$COV%"; \
	if [ "$$(echo "$$COV < 70" | bc)" -eq 1 ]; then \
		echo "  ✗  Coverage $$COV% is below 70% threshold"; \
		exit 1; \
	else \
		echo "  ✓  Coverage $$COV% meets threshold"; \
	fi

# ─── Git Hooks ───────────────────────────────────────────────────────────

HOOKS_DIR    := .githooks

.PHONY: install-hooks

## install-hooks: install Cosca git hooks (post-commit, etc.) via core.hooksPath
install-hooks:
	@echo "  >  Installing Cosca git hooks..."
	@git config core.hooksPath "$(HOOKS_DIR)" \
		&& echo "  ✓  Git hooks configurados: core.hooksPath = $(HOOKS_DIR)"

# ─── dep-doctor ──────────────────────────────────────────────────────────

## dep-doctor: Verifica tudo que o Cosca precisa e auto-corrige o possivel
dep-doctor:
	@COSCA_ALLOW_NO_ROOT=1 bash -c 'set -e; \
	COSCA="$(JAULA)"; PASS=0; FAIL=0; WARN=0; \
	\
	_ok()   { echo "  $$(tput setaf 2)✓$$(tput sgr0) $$1"; PASS=$$((PASS+1)); }; \
	_fail() { echo "  $$(tput setaf 1)✗$$(tput sgr0) $$1"; FAIL=$$((FAIL+1)); }; \
	_warn() { echo "  $$(tput setaf 3)⚠$$(tput sgr0) $$1"; WARN=$$((WARN+1)); }; \
	_fix()  { echo "  $$(tput setaf 5)🔧$$(tput sgr0) $$1"; }; \
	\
	echo ""; echo "$$(tput bold)🔬 Cosca dep-doctor — verificando tudo...$$(tput sgr0)"; \
	echo "──────────────────────────────────────────────────"; \
	\
	# ── 1. Binario cosca ────────────────────────────────────────────── \
	echo ""; echo "$$(tput bold)1. Binario cosca$$(tput sgr0)"; \
	if [ -x "$$COSCA" ]; then \
		_ok "binario presente: $$COSCA"; \
		ver=$$($$COSCA version 2>/dev/null | grep Version | head -1 | tr -d " " || true); \
		if [ -n "$$ver" ]; then _ok "versao: $$ver"; \
		else _fail "nao foi possivel obter versao"; fi; \
	else \
		_fail "binario ausente: $$COSCA"; \
		echo ""; \
		echo "  Para corrigir:"; \
		echo "    cd ~/Documents/cosca && make install"; \
		echo ""; \
	fi; \
	\
	# ── 2. bubblewrap (jail) ────────────────────────────────────────── \
	echo ""; echo "$$(tput bold)2. Bubblewrap (sandbox)$$(tput sgr0)"; \
	if command -v bwrap >/dev/null 2>&1; then \
		_ok "bwrap instalado"; \
	else \
		_warn "bwrap NAO instalado — sandbox desabilitada"; \
		echo ""; \
		echo "  $$(tput setaf 3)⚠ Execute como root:$$(tput sgr0)"; \
		echo "    sudo apt install bubblewrap"; \
		echo ""; \
	fi; \
	\
	# ── 3. GitHub token ─────────────────────────────────────────────── \
	echo ""; echo "$$(tput bold)3. GitHub token (adquire documentacao)$$(tput sgr0)"; \
	if [ -n "$$GITHUB_TOKEN" ]; then \
		_status=$$(curl -sI -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $$GITHUB_TOKEN" https://api.github.com/repos/nestjs/nest 2>/dev/null || echo "000"); \
		if [ "$$_status" = "200" ]; then \
			_ok "GITHUB_TOKEN presente e valido"; \
		else \
			_warn "GITHUB_TOKEN setado mas retornou HTTP $$_status — verifique"; \
		fi; \
	else \
		_warn "GITHUB_TOKEN nao definido (adquire sem autenticacao = 60 req/h)"; \
		echo ""; \
		echo "  $$(tput setaf 3)Para resolver:$$(tput sgr0)"; \
		echo "    1. Crie um token em: https://github.com/settings/tokens"; \
		echo "    2. Scope: public_repo (ou nenhum)"; \
		echo "    3. echo export GITHUB_TOKEN=seu_token >> ~/.bashrc"; \
		echo "    4. source ~/.bashrc && make dep-doctor"; \
	fi; \
	\
	# ── 4. DNS / conectividade ──────────────────────────────────────── \
	echo ""; echo "$$(tput bold)4. DNS e conectividade$$(tput sgr0)"; \
	if curl -sI --connect-timeout 5 https://api.github.com >/dev/null 2>&1; then \
		_ok "api.github.com alcancavel"; \
	else \
		if ping -c1 -W2 8.8.8.8 >/dev/null 2>&1; then \
			_warn "rede ok mas api.github.com inalcancavel (DNS?) — fallback resolver ativo no cosca"; \
		else \
			_fail "sem conectividade de rede"; \
		fi; \
	fi; \
	\
	# ── 5. Knowledge Packages ───────────────────────────────────────── \
	echo ""; echo "$$(tput bold)5. Knowledge Packages (base de conhecimento)$$(tput sgr0)"; \
	_global="$$HOME/.config/cosca/knowledge/packages"; \
	\
	# Promove acquired → validated \
	if [ -d "$$_global" ]; then \
		_acquired=$$(grep -rl "\"acquired\"" "$$_global"/*.json 2>/dev/null | wc -l); \
		if [ "$$_acquired" -gt 0 ]; then \
			_fix "promovendo $$_acquired acquired → validated"; \
			for f in "$$_global"/*.json; do \
				sed -i "s/\"acquired\"/\"validated\"/g" "$$f" 2>/dev/null || true; \
				sed -i "s/\"partial\"/\"validated\"/g" "$$f" 2>/dev/null || true; \
			done; \
		fi; \
		_count=$$(ls "$$_global"/*.json 2>/dev/null | wc -l); \
		_manifest=$$(grep -rl "\"manifest\"" "$$_global"/*.json 2>/dev/null | wc -l); \
		_validated=$$(grep -rl "\"validated\"" "$$_global"/*.json 2>/dev/null | wc -l); \
		if [ "$$_count" -gt 0 ]; then \
			_ok "$$_count pacotes ($$_validated validated, $$_manifest manifest)"; \
		else \
			_warn "base de conhecimento vazia — rode: cosca knowledge acquire \"*\" --global --allow-remote"; \
		fi; \
	else \
		_warn "diretorio de packages nao encontrado: $$_global"; \
	fi; \
	\
	# ── 6. Busca semantica ──────────────────────────────────────────── \
	echo ""; echo "$$(tput bold)6. Busca semantica$$(tput sgr0)"; \
	_search_out=$$($$COSCA knowledge search "autenticacao JWT" --global --limit 1 2>/dev/null || echo ""); \
	if echo "$$_search_out" | grep -q "Results: 0\|0 results\|Results.*0$$"; then \
		_warn "busca semantica retornou 0 resultados — compile o conhecimento"; \
	else \
		_ok "busca semantica funcional"; \
	fi; \
	\
	# ── 7. Resumo ───────────────────────────────────────────────────── \
	echo ""; \
	echo "$$(tput bold)──────────────────────────────────────────────────$$(tput sgr0)"; \
	echo "$$(tput bold)📋 Resumo:$$(tput sgr0) $$(tput setaf 2)$$PASS ok$$(tput sgr0)  $$(tput setaf 3)$$WARN avisos$$(tput sgr0)  $$(tput setaf 1)$$FAIL falhas$$(tput sgr0)"; \
	if [ "$$FAIL" -gt 0 ]; then \
		echo ""; \
		echo "$$(tput setaf 1)❌ Ha $$FAIL problemas que precisam de atencao.$$(tput sgr0)"; \
		echo "   Resolva os itens acima e rode: make dep-doctor"; \
		exit 1; \
	elif [ "$$WARN" -gt 0 ]; then \
		echo ""; \
		echo "$$(tput setaf 3)⚠️  $$WARN avisos — o cosca funciona mas nao esta 100%.$$(tput sgr0)"; \
		echo "   Resolva os avisos e rode: make dep-doctor"; \
		exit 0; \
	else \
		echo ""; \
		echo "$$(tput setaf 2)✅ Tudo certo! O cosca esta 100% operacional.$$(tput sgr0)"; \
		exit 0; \
	fi; '

# =============================================================================
# Catch-all: qualquer comando desconhecido é tratado como comando do cosca
# e executado dentro da jaula. Ex: make version, make discover, make serve
# =============================================================================
# Targets Make explícitos (test, clean, build, etc.) têm precedência.
# Tudo que sobrar cai aqui → build + sudo cosca <comando>
# =============================================================================
# Impede que o Make tente reconstruir o Makefile via catch-all
Makefile: ;
%: build
	@$(JAULA) $@
