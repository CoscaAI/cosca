# ADR-044: Learning Vaults por Departamento — separação consistente da memória de aprendizado

> **Status:** APROVADO (decisão do Don 2026-09-08 — "sim") | **Owner:** cosca-kernel | **Last Updated:** 2026-09-08
> **Natureza:** ADR de decisão. Cria o desenho dos **Learning Vaults** — bancos de aprendizado por departamento, enxutos, regeneráveis do zero a partir dos gatilhos. Substitui a ingestão monolítica atual (knowledge.db 764MB + corte D3 incompleto).
> **Relação:** revisa **ADR-013** (corte modular — o D3 write-through nunca foi completado: ATTACH de escrita ausente); complementa **ADR-002** (knowledge engine) e **ADR-031** (token efficiency); usa o router determinístico do **ADR-013 §3.2** (modlink) e o padrão agregador do **vectoragg**.

---

## 1. Contexto

**Incidente raiz (2026-09-08):** a family chain ficou INVÁLIDA porque agents escreviam conteúdo completo nos `learnings.md` (em vez de gatilhos via `cosca memory register`), mudando hashes vs `family_chain.dat`. Corrigido (reconstrução dos gatilhos + re-assinatura), mas o diagnóstico expôs dois problemas estruturais:

1. **D3 incompleto (bug pré-existente):** com os módulos físicos do ADR-013 presentes (`core.db` etc.), o indexador escreve `INSERT INTO core.documents` na conexão do monolito — mas **nenhum ATTACH de escrita** foi implementado. Resultado: `no such table: core.documents` em toda ingestão nova. O `vectoragg` faz ATTACH só de LEITURA; o caminho de escrita nunca foi conectado.
2. **Inflação do knowledge.db (764MB):** todo aprendizado ingerido vira documento completo + embedding no monolito. A memória da família cresce sem limite e o rebuild é caro.

**Direção do Don:** "reescrever o banco e criar banco por departamento learning — rebuild completo do zero com gatilho pro banco, separação consistente, registrar na memória de forma eficiente sem inflar o banco."

**Safety net confirmado:** `.opencode/cosca/memory/` (gatilhos + archives + blocks + chain.dat) preserva TUDO. Os bancos são derivados e regeneráveis — apagar e reconstruir do zero não perde memória.

---

## 2. Decisão

Criar **Learning Vaults por Departamento**: bancos SQLite enxutos, um por departamento (agrupamento de agents), contendo **apenas metadados de gatilho** (trigger + hash + pointer + embedding do título), NUNCA o conteúdo completo. O conteúdo completo permanece imutável em `blocks/{sha256}.md` (chain-tracked).

### 2.1 Layout físico

```
.cosca/learning/
  kernel.db       ← cosca-kernel
  backend.db      ← cosca-backend, cosca-specialist-backend-api, cosca-specialist-backend-service
  frontend.db     ← cosca-frontend, cosca-specialist-frontend-component, cosca-uiux
  database.db     ← cosca-database, cosca-specialist-database-sql, cosca-migration
  security.db     ← cosca-security, cosca-compliance
  ai.db           ← cosca-ai, cosca-semantic-memory, cosca-provider
  devops.db       ← cosca-devops, cosca-infrastructure, cosca-platform, cosca-automation
  architecture.db ← cosca-architecture, cosca-review, cosca-technical-debt, cosca-critic
  qa.db           ← cosca-qa, cosca-testing, cosca-specialist-testing-*
  product.db      ← cosca-product, cosca-uiux (UX), cosca-governance
  runtime.db      ← cosca-runtime, cosca-monitoring, cosca-performance, cosca-context
  cli.db          ← cosca-cli, cosca-sdk, cosca-plugin
  workflow.db     ← cosca-workflow-chief, cosca-messaging, cosca-integrations
  strategy.db     ← cosca-ceo, cosca-cto, cosca-paradigm, cosca-evolution, cosca-discovery
  docs.db         ← cosca-documentation, cosca-specialist-documentation-writer
```

Mapeamento completo e determinístico em `internal/learning/vaults.go` (agent → vault).

### 2.2 Schema do vault (enxuto, sem inflar)

```sql
CREATE TABLE triggers (
  id         TEXT PRIMARY KEY,          -- L<id> (ex: L435)
  agent      TEXT NOT NULL,             -- cosca-kernel
  vault      TEXT NOT NULL,             -- kernel
  date       TEXT NOT NULL,             -- 2026-09-08
  title      TEXT NOT NULL,
  level      INTEGER NOT NULL DEFAULT 0,
  tags       TEXT NOT NULL DEFAULT '',
  hash16     TEXT NOT NULL,             -- hash do bloco (16 chars)
  block_hash TEXT NOT NULL,             -- hash completo → blocks/{hash}.md
  chain_prev TEXT NOT NULL DEFAULT '',  -- prev da chain (proveniência)
  embedding  BLOB,                      -- embedding do TÍTULO + tags (leve)
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_triggers_agent ON triggers(agent);
CREATE INDEX idx_triggers_tags  ON triggers(tags);
```

**NUNCA** armazena o conteúdo do aprendizado — o conteúdo vive no bloco imutável. O vault é um **índice semântico de gatilhos**, não um repositório de conteúdo.

### 2.3 Fluxo de registro (eficiente)

```
cosca memory register --agent X --title ... --tags ... --learned ...
  → 1. bloco imutável blocks/{sha256}.md (chain-tracked, como hoje)
  → 2. gatilho 1-linha em learnings.md (como hoje)
  → 3. row chain.dat + merkle (como hoje)
  → 4. NOVO: upsert leve no vault do departamento (trigger + hash + embedding do título)
```

O passo 4 substitui a ingestão monolítica atual. Custo por registro: ~1 documento pequeno no vault do departamento, não 1 documento completo + embedding no monolito de 764MB.

### 2.4 Rebuild do zero (determinístico)

```
cosca learning rebuild
  → 1. apaga .cosca/learning/*.db (derivados, regeneráveis)
  → 2. lê chain.dat de cada agent (ledger: hash|prev|date|id|title)
  → 3. para cada row: abre blocks/{hash}.md → extrai LEVEL/TAGS do header
  → 4. classifica agent → vault (mapeamento determinístico)
  → 5. cria/atualiza o vault com a trigger + embedding do título
  → 6. reporta: N vaults, M triggers, K blocos faltando (buracos históricos)
```

Idempotente, sem LLM, sem rede. O gatilho do learnings.md é o ponteiro; o chain.dat é o ledger; os blocks são a fonte.

### 2.5 Busca

- **Escopada:** `cosca learning search "<q>" --vault backend` → busca semântica só no vault do departamento (isolamento real — aprendizado de segurança não polui contexto de frontend).
- **Cross-department:** agregador no padrão vectoragg (ATTACH read-only dos vaults) — já existe o padrão, reusa.
- **Conteúdo completo:** carregado sob demanda do bloco (`blocks/{hash}.md`) — nunca do vault.

---

## 3. Opções consideradas

- **A (APROVADA) — Learning Vaults por departamento:** enxuto, isolado, regenerável, mata o bug do D3 (cada vault é auto-contido). Custo: mapeamento agent→vault (1x) + engine de vault (~1 pacote Go).
- **B — Consertar o D3 (ATTACH de escrita no monolito):** mantém o monolito de 764MB e a inflação; exige conexão única (impacto em concorrência) + testes. Rejeitada: não resolve a inflação nem o isolamento.
- **C — Status quo (monolito + ingestão quebrada):** rejeitada — a ingestão está quebrada HOJE e o banco infla sem controle.

---

## 4. Trade-offs e riscos

| Risco | Mitigação |
|-------|-----------|
| Busca cross-department mais lenta (multi-DB) | Agregador vectoragg já existente; vaults pequenos → barato |
| Mapeamento agent→vault errado | Tabela determinística em `vaults.go` + teste de cobertura (54 agents) |
| Blocos faltando (buracos históricos, ex: kernel 66) | Reportados no rebuild; trigger fica com hash16 sem conteúdo — honesto |
| Embedding do título menos rico que conteúdo completo | Título + tags são o índice; conteúdo completo sob demanda (ADR-031: token efficiency) |

---

## 5. Critérios de aceite

1. `cosca learning rebuild` reconstrói todos os vaults do zero, idempotente, sem LLM.
2. `cosca memory register --agent X` grava no vault do departamento de X (verificável: contagem no vault).
3. `cosca learning search "<q>" --vault Y` retorna só triggers do departamento Y.
4. Nenhum conteúdo completo em vault — sempre pointer para blocks/{hash}.md.
5. Chain continua válida após rebuild (`cosca-check -root .` → "Chain valid").

---

## 6. Fora de escopo (fases futuras)

- Migração/retirada do knowledge.db monolito (764MB) — decisão separada.
- Promoção trigger → padrão reutilizável (patterns.md) com gate de evidência (ADR-038).
- Curadoria cross-vault (memory-curation engine) — pode operar sobre os vaults depois.