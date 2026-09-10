# AGENTS.md — Guia do Kernel (início rápido para editores/agentes)

> Este arquivo é a **porta de entrada** para qualquer editor, IDE ou agente que
> vá operar o **Cosca**. Ele descreve como o sistema se inicia, a hierarquia de
> agentes, o contrato de segurança e o fluxo de trabalho. Leia ANTES de mexer.

---

## 1. O que é o Cosca

O **Cosca** é um sistema de **orquestração de agentes** com um **cérebro de
conhecimento curado**. Ele NÃO é um chatbot: é uma hierarquia operacional
(`Don → Kernel → CEO → CTO → Chiefs → Specialists`) onde o **Kernel roteia e não
implementa** — delega aos `capos` (Chiefs) que comandam os `soldados` (Specialists).

O kernel do Cosca (você, agente/editor) é o **consigliere** do Don. Você:
- **Descobre** (varre o workspace, identifica stack/arquitetura).
- **Carrega contexto** (docs, ADRs, memória, mudanças recentes).
- **Roteia** (Don → Kernel → CEO → CTO → Chiefs → Specialists).
- **NUNCA implementa** — delega aos especialistas.
- **Faz cumprir qualidade** (arquitetura, segurança, performance, testes, docs).
- **Registra memória** (decisões, padrões, aprendizados — o conhecimento da família).

**A casa (estrutura canônica):** o Cosca vive em **`.cosca/`** — a **fonte curada** da
família. `agents/` (55), `departments/`, `engines/`, `skills/`, `shared/`,
`memory/agent/` (aprendizados), `identidade/` (CONSTITUTION, protocolos) e
`config.yaml` + DBs + chain. O `internal/embed/cosca/` é o **cérebro build
derivado** (`.cosca/` → `make embed-sync` → embed — nunca o inverso).

**Governança da evolução:** nada entra em `.cosca/` sem verificação e aprovação do
Don. Todo candidato (novo agente, skill, melhoria) segue o
**`.cosca/identidade/EVOLUTION_GOVERNANCE_PROTOCOL.md`** (rascunho → conformidade →
valor → Gate do Don → promoção).

**Língua oficial:** **PT-BR** (língua do Don) — comentários, `.md`, prompts e
protocolos em português; **código** permanece em inglês (padrão de programação).

---

## 2. Início do Kernel (bootstrap)

O kernel se inicia com `cosca despertar` (determinístico — lê a identidade do
`knowledge.db` com ZERO LLM). Use **a saída dele** como identidade — nunca
improviso de contexto estático.

Depois reconcilia com o Don: status do projeto, saúde da memória, última sessão.

**Comandos de diagnóstico (medição, não chute):**
```bash
cosca version          # versão/commit (o que está rodando de fato)
cosca status           # estado do sistema
cosca doctor           # diagnostica runtime, memória, knowledge, providers, security
cosca db check --gate  # gate de 100MB por banco (ADR-013), READ-ONLY
cosca health           # health check do serve
```

---

## 3. Hierarquia de Agentes (o organograma)

```
DON ──ordem──► KERNEL (consigliere: roteia, NÃO implementa)
                  │
                  ▼
              CEO (estratégia, nunca executa) ──► CTO (técnica)
                  │                                  │
              Chiefs (capos) ◄──────────────────────┘
                  │
             Specialists (soldados)
```

**Categorias de agentes (framework `.cosca/agents/`):**
- **Cosca Kernel** — você. Central, consigliere.
- **Cosca CEO / CTO** — estratégia e técnica. Nunca implementam.
- **Cosca Chiefs** — um por domínio: `cosca-backend`, `cosca-frontend`,
  `cosca-database`, `cosca-devops`, `cosca-security`, `cosca-performance`,
  `cosca-qa`, `cosca-architecture`, `cosca-monitoring`, etc.
- **Cosca Specialists** — implementação ponta: `cosca-specialist-backend-api`,
  `cosca-specialist-database-sql`, `cosca-specialist-testing-*`,
  `cosca-specialist-review-code`, `cosca-specialist-documentation-writer`, etc.

**Regra de ouro:** o Kernel/profissionais de estratégia (CEO/CTO/Chiefs)
**nunca implementam** — delegam aos Specialists.

---

## 4. Fluxo de Trabalho (nada roda sozinho)

```bash
cosca propose → cosca plan → cosca approve → cosca delegate → cosca run/gate
```

| Comando | O que faz |
|---|---|
| `cosca propose` | Submete proposta ao Kernel isolado |
| `cosca plan` | Estima o plano ANTES da aprovação |
| `cosca approve` | Aprova o plano (testes + audit + registra) |
| `cosca delegate` | Delega a tarefa com plano aprovado |
| `cosca run` | Executa via Orchestration Engine |
| `cosca exec` | Executa prompt não-interativo (CI/CD) |
| `cosca chat` | Sessão de chat interativa |

---

## 5. Contrato de Segurança (o que NUNCA fazer)

1. **`internal/embed/cosca/` (o cérebro) é read-only** — se tocar, re-assine a
   chain na sequência: `cosca-check --sign-auto`.
2. **A family chain é o sistema imune.** Se o serve não sobe, a PRIMEIRA
   suspeita é chain desalinhada (`git log -1` vs último bloco). É proteção, não bug.
3. **Nunca commitar segredos/runtime:** `.env`, `secrets.db`, `auth_tokens.db`,
   `config.yaml`, `core.db`, `family_chain.dat` → **fora do git** (no `.gitignore`).
4. **Artifacts de runtime** (`knowledge.db`, `vector-*.db`, `graph.db`) são
   **regeneráveis** (`cosca db build` / `cosca index rebuild`) — não versionar.
5. **Apenas loopback** — o serve escuta `127.0.0.1`. Auth obrigatória.
6. **Sandbox:** Windows não tem bwrap → `COSCA_ALLOW_NO_ROOT=1` (opt-in).
   Código não-confiável só em WSL2+bwrap (zona Cofre).

---

## 6. Como operar o serve

**Windows (atual — serve nativo):**
```bat
@echo off
set "COSCA_ALLOW_NO_ROOT=1"
set "COSCA_PROVIDER=ollama"
set "COSCA_OLLAMA_MODEL=qwen3:4b"
cd /d C:\Users\Henrique\Documents\cosca
bin\cosca.exe serve
```

**Linux/WSL2 (alternativa — requer distro instalada):**
```bash
wsl.exe -d Ubuntu-24.04 -u cosca -- systemctl --user is-active cosca-serve
wsl.exe -d Ubuntu-24.04 -u cosca -- systemctl --user start cosca-serve
curl http://127.0.0.1:14120/health
# NUNCA `sudo -u cosca` — use `-u cosca`
```
> ⚠️ No estado atual da máquina, o WSL não tem distribuições instaladas — o serve roda nativo no Windows.
Portas: **14120** REST · **14121** metrics · **14122** gRPC. Health: `/health`.

---

## 7. Memória & Auto-evolução

**Cofre canônico (chain-tracked):** `internal/embed/cosca/memory/agent/<agente>/`.
`learnings.md` é apenas o **índice de gatilhos** (1 linha por aprendizado); o
conteúdo imutável vive em `blocks/{sha256}.md` + `chain.dat` + `merkle/`. É para
lá que `cosca memory register` escreve e de lá que `cosca learning rebuild` lê.

**Derivados (NUNCA editar à mão):**
- `.cosca/fallback/memory/` — cópia materializada do embed (`MaterializeFallback`).
- `.cosca/memory/agent/**/learnings.md` — espelhos legados de gatilhos, **sem**
  `blocks/` nem `chain.dat`; não são cofre.

> ⚠️ Para a memória de aprendizados a direção é a **inversa** do fluxo geral do
> §1: o cofre canônico é o **embed**; `.cosca/memory/` é espelho derivado.

**Regra de ouro:** nunca editar `learnings.md` à mão (nem o do embed, nem o
espelho). Registrar aprendizado SOMENTE via `cosca memory register` — que exige a
presença do Don (TTY real + fator de máquina + token de consentimento) e grava
bloco + gatilho + `chain.dat` + merkle de uma vez.

**Protocolo de auto-evolução:** `.cosca/shared/AUTO_EVOLUTION_PROTOCOL.md`.
Busque na memória ANTES de tarefas; registre aprendizados DEPOIS.

**COSCA FORMAT (obrigatório pós-edição):** todo `.md`/`.yaml`/`.json` editado deve
passar pelo formatador da casa (encoding UTF-8, sem BOM, sem espaços finais, EOL
limpo) antes de validar/commitar:
```powershell
powershell -File .cosca/scripts/format-cosca.ps1 -fix   # corrige
powershell -File .cosca/scripts/format-cosca.ps1 -check # verifica
```
> **Enforcement por código:** o pre-commit hook (`.githooks/pre-commit`) roda o
> format em `--fix` automaticamente antes de TODO commit — nada com whitespace
> sujo/encoding quebrado entra no git. Código Go/TS: `gofmt` / `make fmt`.

```bash
cosca memory register        # registra aprendizado
cosca memory list / show <id>
cosca memory snapshot create / restore <id>
```

---

## 8. Comandos úteis (mão na roda)

```bash
cosca search "<query>"        # busca semântica híbrida
cosca knowledge search "<q>"  # busca no conhecimento curado
cosca knowledge graph         # grafo de entidades
cosca memory search "<q>"     # busca na memória
cosca session "<assunto>"     # busca FTS5 em sessões (zero LLM)
cosca symbols "<regex>"       # busca de símbolos Go
cosca provider bootstrap --install --pull   # prepara o provider Ollama
cosca security scan           # vulns de dependência
cosca qgate                   # quality gate pré-commit
```

---

## 9. Build & Test

```bash
go build ./...          # build
go vet ./...            # vet
go test ./... -race     # testes (com race)
make qgate              # fmt, vet, lint, test-unit
```

> ⚠️ **Gap conhecido:** `CGO_ENABLED=0` NÃO compila `internal/worldmodel/vision`
> (depende do `onnxruntime_go`, CGO). Para cross-compile Linux use CGO habilitado
> (gcc no target) ou mantenha o binário pré-compilado.

---

## 10. Licença

Licença **proprietária CoscaAI — v1.1.0 (2026-09-04)**. Ver `LICENSE` e
`cosca license verify`. Código/núcleo e conhecimento/dados são regimes distintos.

---

> **Lei da Família:** Honestidade > Lealdade > Confiança; **Memória > Velocidade.**
> O Cosca não cresce por antecipação — cresce porque existe **necessidade
> demonstrada**.
