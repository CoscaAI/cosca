# Cosca — Enterprise AI Orchestration Platform

> **Versão:** 1.5.0 · **Go 1.26.5** · Windows/Linux/macOS
> **O Cosca não é um chatbot — é um sistema de orquestração de agentes com um
> cérebro de conhecimento curado.** Ele decide onde procurar (roteador
> determinístico), recupera informação validada, e delega execução numa
> hierarquia operacional onde cada agente tem papel definido.

---

## Índice

- [O que é o Cosca](#o-que-é-o-cosca)
- [Números reais](#números-reais)
- [Arquitetura](#arquitetura)
- [Instalação](#instalação)
- [Uso básico](#uso-básico)
- [Como tirar proveito máximo](#como-tirar-proveito-máximo)
- [Segurança e integridade](#segurança-e-integridade)
- [Solução de problemas](#solução-de-problemas)

---

## O que é o Cosca

O Cosca é uma **plataforma de orquestração de agentes de IA** em Go. Ele não é
um framework genérico — é uma infraestrutura de produção com:

- **Conhecimento modular** com proveniência (claims FACT / EVIDENCE / INFERENCE).
- **Memória semântica** auto-evolutiva (o kernel aprende com cada sessão).
- **Busca híbrida**: texto (BM25) + semântica (vetor) + grafo relacional.
- **Quality gates, sandboxing e chain de integridade** Ed25519.
- **Modularidade de dados** (ADR-013): sem banco monolítico — cada domínio em
  um módulo < 100 MB.

### Hierarquia operacional

```
DON (autoridade máxima)
 └─ KERNEL (consigliere — roteia, NÃO implementa)
     ├─ CEO (estratégia) → CTO (técnica)
     ├─ CHIEFS (capos de domínio: backend, frontend, security, database, ai…)
     └─ SPECIALISTS (soldados: implementação concreta)
```

> O **Kernel nunca implementa** — planeja, roteia, delega e revisa.

---

## Números reais

Métricas verificadas no estado atual (2026-09-05):

| Métrica | Valor |
|---------|------:|
| Comandos CLI | **123** |
| Agentes | **61** |
| Skills | **88** |
| Workflows / Pipelines | **39** |
| Entries de conhecimento | **17.738** |
| Vetores nos módulos | **15.773** |
| Providers | 10 (ollama, local ativos) |
| Packages Go | 234 |
| Arquivos Go (src) | 1.024 |
| Testes | 843 |

---

## Arquitetura

### Dados (ADR-013 — corte modular)

O Cosca **não usa** um banco monolítico. Os dados são particionados em módulos
físicos, cada um < 100 MB:

| Módulo | Arquivo | Conteúdo |
|--------|---------|----------|
| Core | `core.db` | documentos originais (manifest), proveniência, metadados — **fonte da verdade** |
| Grafo | `graph.db` | entidades (19.703) + relações (15.773) |
| Projects | `projects.db` | chunks (15.773) + elementos + FTS |
| Vetores | `vector-*.db` | embeddings (15.773, 768-dims) — **busca semântica** |
| Semântico | `semantic.db` | índice semântico |

**A busca usa similaridade por cosseno (entendimento) + BM25 (palavra) + grafo.**

### Integração externa

- **Unreal Engine**: ponte via WebSocket (cliente real `nhooyr.io/websocket`).
- **Blender**: pipeline end-to-end real (`scripts/blender/*.py`).
- **Embeddings**: `nomic-embed-text` local (Ollama, 768-dims, offline).
- **Providers**: 10 suportados (llama3 via Ollama ativo; OpenAI/Anthropic/etc.
  como no_key).

---

## Instalação

### Pré-requisitos

- **Go 1.26+** (para compilar) **ou** binário pré-compilado.
- **Git** (para integridade da chain e versionamento).
- **Ollama** (opcional, para embeddings locais) — recomendo para busca semântica.

### 1. Build (a partir do código)

```bash
# Clonar/entrar no diretório do projeto
cd cosca

# Compilar o binário principal
go build -o cosca ./cmd/cosca

# Compilar os utilitários de integridade (opcional)
go build -o cosca-check.exe ./cmd/cosca-check
go build -o cosca-indexer.exe ./cmd/cosca-indexer
go build -o cosca-merkle.exe ./cmd/cosca-merkle
```

### 2. Inicializar o projeto

```bash
# Cria a estrutura .cosca/, configuração e descobre o ambiente
cosca init

# OU — fluxo completo de preparação (recomendado):
cosca start
#   → detecta estado da pasta
#   → verifica dependências
#   → executa init + install
#   → executa doctor --fix (diagnóstico + correções seguras)
#   → executa sync (indexação)
```

### 3. Diagnóstico

```bash
cosca doctor        # diagnóstico do sistema (runtime, knowledge, providers)
cosca setup         # provisiona a máquina até COSCA_READY (idempotente)
```

### 4. Subir o serviço (REST API)

```powershell
# Windows: a jaula bwrap não existe → opt-in explícito
$env:COSCA_ALLOW_NO_ROOT="1"
$env:COSCA_DEV_MODE="true"
$env:COSCA_JWT_SECRET="<segredo forte, ≥32 bytes>"   # coloque no .env (gitignored)

cosca serve
# health: curl http://127.0.0.1:14120/health → 200
# ready:  curl http://127.0.0.1:14120/ready  → {"ready":true,...}
```

---

## Uso básico

```bash
# Consultar conhecimento (busca semântica)
cosca knowledge search "orquestração de agentes"

# Ver estatísticas do conhecimento
cosca knowledge stats

# Orquestrar uma tarefa via IA (o engine roteia para o melhor agente)
cosca run "refatore o módulo de autenticação"
cosca run "explique a arquitetura do pipeline" --stream

# Criar uma memória de aprendizado (auto-evolução)
cosca memory register --title "..." --tags "#learned #pattern" --learned "..."

# Ver agentes/skills/workflows disponíveis
cosca agent list
cosca skill status
cosca workflow list

# Nível de capacidade cognitiva
cosca capability level
```

---

## Como tirar proveito máximo

### 1. Use o pipeline de decisão (Kernel-first)

O Cosca tem um ciclo de decisão. Para tarefas de desenvolvimento, o fluxo
aprovado é:

```bash
cosca plan "melhorar a performance do search"    # estima o plano
cosca approve --plan plano.json                  # aprova (roda testes + auditoria)
cosca delegate                                    # delega a execução
cosca run/gate                                    # executa e valida
```

### 2. Explore os agentes por domínio

Não use só o `run`. Delega a domínios específicos:

```bash
cosca agent show security-chief      # especialista de segurança
cosca agent show database-chief      # especialista de banco
cosca agent search "performance"     # encontra o agente certo
```

### 3. Alimente o conhecimento (o cérebro cresce)

```bash
# Ingerir conhecimento de uma fonte
cosca knowledge index <diretório>    # indexa documentos (FTS + embeddings)
cosca knowledge add <lib|repo>       # registra um Knowledge Package
cosca knowledge evidence             # proveniência (P0-P5) — nunca inventar
cosca knowledge claim                # classifica afirmações (FACT/EVIDENCE/...)
```

### 4. Memória semântica (o kernel lembra)

```bash
cosca memory semantic "como resolver conflito de dependências"   # busca por significado
cosca memory snapshot create        # snapshot do estado
cosca memory guard                  # valida contra inflação/auto-promoção
```

### 5. Gerencie o banco de forma modular

```bash
cosca db check --gate               # todos os bancos < 100 MB
cosca db build                      # materializa os módulos (aditivo)
cosca db verify                     # valida o split
```

> **⚠️ Regra de ouro:** rode `cosca knowledge vectors-backfill` **antes** de
> `cosca db build` — o build recria os módulos a partir da fonte; sem o
> backfill os vetores se perdem.

### 6. Use os utilitários de produtividade

```bash
cosca machine probe                 # perfil da máquina (GPU/CPU)
cosca model list                    # modelos registrados
cosca vision                        # visão (ONNX)
cosca media extract-audio video.mp4 a.wav   # mídia (ffmpeg)
cosca qgate                         # quality gate pré-commit
```

---

## Segurança e integridade

- **Family chain:** Ed25519 + DPAPI (Windows) + git-anchor. Assinatura
  machine-bound. Se o histórico for reescrito, a chain **bloqueia** o boot
  (fail-closed correto).
- **JWT:** `COSCA_JWT_SECRET` obrigatório (≥32 bytes). Sem ele o serve não sobe.
- **Jaula:** bwrap (Linux/WSL2). Windows exige `COSCA_ALLOW_NO_ROOT=1` (opt-in).
- **Loopback only:** `cosca serve` escuta só em `127.0.0.1:14120`.

---

## Solução de problemas

### "Serve não sobe"
Checar (em ordem):
1. **Chain válida?** → `cosca-check` → se `BREACH`, re-assinar:
   `cosca-check --sign-auto`
2. **`COSCA_JWT_SECRET`?** → setar no `.env` (≥32 bytes).
3. **`COSCA_ALLOW_NO_ROOT=1`?** (Windows).

### "Busca semântica não retorna"
1. **Vetores populados?** → `cosca knowledge vectors-backfill`
2. **Módulos sincronizados?** → `cosca db build` → `cosca db verify`

### "Fail-closed bloqueando"
O Cosca **bloqueia de propósito** quando detecta alteração de integridade.
Isso é proteção, não bug. Diagnosticar (ler log) e **re-assinar a chain**
quando a mudança é legítima.

---

## Documentação relacionada

| Documento | Caminho |
|-----------|---------|
| Manual completo | `docs/MANUAL_COSCA.md` |
| Runbook de recuperação | `docs/reports/cosca-recovery-runbook-2026-09-05.md` |
| Níveis de capacidade | `docs/COSCA_LEVELS.md` |
| ADR-013 (corte modular) | `docs/adr/ADR-013-modular-knowledge-databases.md` |

---

> **"Honestidade > Lealdade > Confiança; Memória > Velocidade."**
> — Lei da Família Cosca
