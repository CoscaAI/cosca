# MANUAL DO COSCA — Guia de Operação e Arquitetura

> **Versão do sistema documentada:** 1.5.0
> **Data da verificação:** 2026-09-05
> **Base:** código executado (P2) — números **medidos agora**, não claims de versões antigas.
> **Autor:** Cosca Kernel, por ordem do Don.

---

## 1. O QUE É O COSCA

O Cosca é uma **plataforma de orquestração de agentes de IA** escrita em **Go**. Não é um chatbot — é um **sistema de orquestração com um cérebro de conhecimento curado**: ele decide **onde** procurar (router determinístico), recupera informação validada, e delega execução numa hierarquia operacional.

**Arquitetura hierárquica (cadeia de comando):**
```
DON (autoridade máxima, veto absoluto)
 └─ KERNEL (consigliere — roteia, NÃO implementa)
     ├─ CEO (estratégia)
     │   └─ CTO (técnica)
     ├─ CHIEFS (capos de domínio: backend, frontend, security, database, ai...)
     └─ SPECIALISTS (soldados: implementação concreta)
```

**Princípio fundamental:** o **KERNEL nunca implementa** — ele planeja, roteia, delega e revisa. A implementação é feita pelos especialistas.

---

## 2. NÚMEROS REAIS (verificados 2026-09-05)

| Métrica | Valor | Fonte |
|---------|------:|-------|
| Comandos top-level | **123** | `cosca --help` |
| Agentes | **61** | `cosca agent list` |
| Skills | **88** | `cosca skill status` |
| Workflows | **39** | `cosca workflow list` |
| Providers ativos | `local` (tf-idf), `ollama` | `cosca provider list` |
| Entries de conhecimento | **17.738** | `cosca knowledge stats` |
| Knowledge.db | 43 MB (sob o teto) | `cosca knowledge stats` |
| Vetores nos módulos | **15.773** | soma `vector-*.db` |
| Packages Go | 234 | `go list ./...` |
| Arquivos Go (src) | 1.024 | contagem |
| Testes Go | 843 | contagem |
| Family chain | válida (7 blocks, 2005 files) | `cosca-check` |

> **Nota:** o README antigo dizia "53 agents / 29 skills / 30 workflows". Os **números reais** (P2) são **61 / 88 / 39** e **123 comandos**. O manual usa os valores medidos.

---

## 3. ARQUITETURA DE DADOS (ADR-013 — o corte modular)

O Cosca **não usa um banco monolítico**. Desde o ADR-013 (2026-08-24), os dados são **particionados em módulos físicos**, cada um < 100 MB (Decisão 1):

### 3.1 Os módulos (o que agrega hoje)

| Módulo | Arquivo | Conteúdo | Papel |
|--------|---------|----------|-------|
| **Core** | `core.db` | documentos originais (manifest 1.965), proveniência, metadados | **Fonte da verdade** (mapa) |
| **Grafo** | `graph.db` | entidades (19.703) + relações (15.773) | navegação relacional |
| **Projects** | `projects.db` | chunks (15.773) + headings + code_blocks + FTS | conhecimento por projeto |
| **Vetores** | `vector-*.db` | embeddings (15.773 vetores, 768-dims) | **busca semântica** |
| **Semântico** | `semantic.db` | índice semântico | retrieval |

**O knowledge.db** (antes monolítico) é a **fonte + buffer de escrita**; os **vetores de verdade vivem nos módulos `vector-*.db`** (PartitionStore). O `cosca db verify` valida que os módulos são equivalentes à fonte.

### 3.2 O fluxo correto de manutenção (IMPORTANTE)

```
vectors-backfill  →  escreve os vetores no knowledge.db (base)
db build          →  SINCRONIZA knowledge.db → vector-*.db (popula os módulos)
db verify         →  prova que o split está íntegro
```

**NUNCA rode `db build` sem antes o `vectors-backfill`** (o build apaga e recria os módulos; se a base estiver vazia, os vetores se perdem).

---

## 4. BUSCA (semântica + palavra)

A busca usa **busca híbrida em camadas com similaridade por cosseno**:
- **FTS5 (BM25)** — match por palavra/texto.
- **Vetor (cosseno)** — match por **significado/entendimento** (embedding nomic-embed-text, 768-dims).
- **Grafo** — navegação relacional.

**A busca é por ENTENDIMENTO, não só por palavra.** Score alto (0.79-0.90) = bom match semântico; score baixo (0.4-0.5) = baixa confiança (o sistema é honesto sobre isso).

```bash
cosca knowledge search "orquestração de agentes"      # busca semântica
cosca search layered "backup"                         # busca em camadas (custo progressivo)
cosca search code "função mysql"                      # busca de código
```

---

## 5. NÍVEIS DE CAPACIDADE (soberania)

| Nível | Nome | O que pode |
|-------|------|-----------|
| **L1-INICIAL** | primeiro despertar | só leitura de identidade/contexto |
| **L2-OPERACIONAL** | semântica + operação da máquina | opera máquina + edita workspace (NÃO o cérebro) |
| **L3-SOBERANO** | capacidade total | edita o cérebro, **só com aval do Don** |

Enforcement por código (`internal/level/`): `LevelGate.Check()`, `Promote(L3)` exige o Don (`VerifyDonPresence`, fail-closed), e `watchdog` auto-descida (loop ≥3×, edição do cérebro sem re-assinar ≥2×, erros ≥3×).

---

## 6. INTEGRIDADE E SEGURANÇA

- **Family chain:** Ed25519 + DPAPI (Windows) + git-anchor (testemunho de imutabilidade). Assinatura machine-bound.
- **Fail-closed:** se a chain quebra (histórico reescrito), o `serve` **recusa subir** — proteção correta.
- **JWT:** `COSCA_JWT_SECRET` obrigatório (≥ 32 bytes). Sem ele, o serve não sobe.
- **Jaula:** bwrap (Linux/WSL2). No Windows, exige `COSCA_ALLOW_NO_ROOT=1` (opt-in explícito) — sem ele, `exit 1`.
- **Loopback only:** serve escuta em `127.0.0.1:14120`.

### Como subir o serve
```powershell
$env:COSCA_ALLOW_NO_ROOT="1"
$env:COSCA_DEV_MODE="true"
$env:COSCA_JWT_SECRET="<do .env>"
cosca serve
# health: curl http://127.0.0.1:14120/health → 200
```

---

## 7. COMANDOS POR DOMÍNIO (os 123)

### Conhecimento
```bash
cosca knowledge search "<query>"     # busca semântica
cosca knowledge stats                # estatísticas
cosca knowledge verify               # integridade
cosca knowledge vectors-backfill     # re-embed chunks sem vetor
cosca knowledge index-entities       # vetoriza nós do grafo
cosca knowledge claim                # classifica claims (FACT/EVIDENCE/...)
cosca knowledge evidence             # proveniência (P0-P5)
```

### Memória
```bash
cosca memory list                    # lista memórias
cosca memory register --title "..."  # registra aprendizado
cosca memory semantic                # retrieval semântico
cosca memory snapshot create         # snapshot
cosca memory guard                   # valida contra a régua (anti-inflação)
```

### Agentes / Skills / Workflows
```bash
cosca agent list                     # 61 agentes
cosca agent show <name>              # detalhe
cosca agent search "<query>"         # busca agente
cosca skill status                   # 88 skills (uso/estado)
cosca skill list                     # skills
cosca skill validate                 # valida contra o padrão
cosca workflow list                  # 39 workflows
cosca workflow run <name>            # executa workflow
```

### Banco / Dados
```bash
cosca db check --gate                # gate de 100 MB
cosca db build                       # materializa módulos (aditivo)
cosca db verify                      # valida split
cosca db mirror                      # espelho de leitura (read-only)
```

### Runtime / Sistema
```bash
cosca serve                          # sobe o serviço REST
cosca runtime status|start|stop      # daemon
cosca recovery                       # verificação read-only
cosca recovery --restore             # volta ao GOLD POINT (destrutivo)
cosca doctor                         # diagnóstico
cosca qgate                          # quality gate pré-commit
```

### Ingestão / Visão / Voz / Mundo
```bash
cosca provider list                  # providers
cosca provider test ollama           # testa conexão
cosca machine probe                  # perfil da máquina
cosca model list                     # modelos registrados
cosca vision                         # visão (ONNX)
cosca voice                          # voz (projeto independente)
cosca world                          # world model (Living World)
cosca bridge                         # ponte Unreal (WebSocket)
```

---

## 8. INTEGRAÇÃO EXTERNA

- **Unreal Engine:** ponte via **WebSocket** (`internal/bridge`) para `UCoscaWorldSubsystem`. Cliente real (`nhooyr.io/websocket`). **Blender** end-to-end real (`scripts/blender/*.py`).
- **Embeddings:** nomic-embed-text via **Ollama local, 768-dims** (offline). Fallback: local TF-IDF.
- **Providers:** 10 suportados (ollama, local ativos; OpenAI/Anthropic/Google/DeepSeek/Mistral/Groq/Azure/Bedrock como no_key).
- **Lab/treino:** pipeline LoRA em `laboratory/` (scripts Python, fora do binário).

---

## 9. GOLD POINT E RECUPERAÇÃO

- **GOLD POINT** = commit com a marker **"GOLD POINT"** na mensagem. `cosca recovery --restore` volta nele e reconstrói os módulos.
- **GOLD DB** = branch com os bancos versionados (`git add -f`).
- Estado GOLD atual: branch `cosca-database`, commit `57d9993` (GOLD DB 2026-09-05).

```powershell
# Verificação (segura, read-only)
cosca recovery

# Restaurar (destrutivo, exige confirmação)
cosca recovery --restore
```

---

## 10. BOAS PRÁTICAS / GOTCHAS

1. **NUNCA** `db build` sem antes `vectors-backfill` (apaga vetores dos módulos).
2. **NUNCA** re-assinar a chain ANTES do commit (o commit muda o HEAD → quebra).
3. **NUNCA** rebase/squash em histórico compartilhado (quebra a chain).
4. **Serve não sobe** sem: chain válida + `COSCA_ALLOW_NO_ROOT=1` + `COSCA_JWT_SECRET`.
5. **README/CLI desatualizam** — sempre verificar com `cosca agent list` / `skill status` / `workflow list` (P2).
6. **Fail-closed é amigo** — quando algo "não sobe", é um portão de segurança avisando. Diagnosticar (ler log), não contornar.

---

## 11. REFERÊNCIAS

| Documento | Caminho |
|-----------|---------|
| Runbook de recuperação | `docs/reports/cosca-recovery-runbook-2026-09-05.md` |
| Constituição (princípios) | `.opencode/cosca/CONSTITUTION.md` |
| Níveis | `docs/COSCA_LEVELS.md` |
| ADR-013 (corte modular) | `docs/adr/ADR-013-modular-knowledge-databases.md` |
| Auditoria total | `docs/reports/cosca-master-audit-2026-09-05.md` |

---

> **"O Cosca não é um chatbot — é um sistema de orquestração com um cérebro de
> conhecimento curado. Honestidade > Lealdade > Confiança; Memória > Velocidade."**
