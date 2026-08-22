# GRAPH PROTOCOL — Navegar o grafo do conhecimento (o mapa da família)

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — *"crie protocolo graph"*, logo após o conserto
> do knowledge graph (L335: o grafo mostrava 0 nós com 9.110 no cofre)
> **Propósito**: referência operacional ÚNICA para LER e OPERAR o knowledge graph — o mapa
> das relações entre entidades da família. Complementa `KNOWLEDGE_SEARCH_PROTOCOL.md` (busca) —
> a busca acha os DOCUMENTOS; o grafo mostra como as COISAS se RELACIONAM.

---

## 1. O QUE É o grafo

O **knowledge graph** é o mapa das relações entre entidades do conhecimento:

- **Nós** = entidades (docs, skills, ADRs, patterns, configs, bugs, chunks...)
- **Arestas** = relações (majoritariamente `contains`: doc → seus chunks)

O grafo é **persistido no SQLite** (`.cosca/knowledge.db`, tabelas `entities` +
`relationships`) e **espelhado em memória** pelo adapter quando um comando roda.
**O SQLite é a fonte da verdade** — o grafo em memória é só uma projeção de leitura.

> ⚠️ **A lição do L335**: o adapter ANTIGO criava `graph.New()` vazio e nunca lia o
> SQLite — o CLI mostrava 0 nós com 9.110 no cofre. O adapter DEVE carregar
> `entities` + `relationships`; erro de Open/Query NUNCA é ignorado com `_ =`
> (devolve grafo vazio sem diagnóstico).

---

## 2. OS COMANDOS — como navegar

```bash
# Visão geral (nós + arestas)
cosca knowledge graph                     # 9110 nós, 8544 arestas
cosca graph show                          # idem (alias)
cosca graph stats                         # + densidade, componentes conexos

# Relações de uma entidade (por NOME ou path — os IDs são UUIDs)
cosca graph query "CONSTITUTION.md" --depth 1
cosca graph query "bug-008-metrics-misdocumented.md" --depth 2

# Exportar o grafo (json, dot, graphml)
cosca graph export -f json -o graph.json
cosca graph export -f dot -o graph.dot
cosca graph export -f graphml -o graph.graphml

# Evidências (documented_in, tested_by, affected_by, contradicted_by)
cosca graph evidence <entity>
cosca graph link <source> <target> --type documented_in
```

---

## 3. O MODELO DE DADOS — o que está no cofre

```sql
-- entities: cada linha é um NÓ
SELECT id, entity_type, name, path FROM entities;
--  id = UUID (nunca exibir ao Don — resolver nome → id)
--  entity_type ∈ {chunk, doc, skill, config, pattern, adr, bug, ...}

-- relationships: cada linha é uma ARESTA dirigida
SELECT source_id, target_id, rel_type, weight FROM relationships;
--  rel_type majoritário = contains (doc → chunk)
```

**Números de referência** (16/08/2026): 9.110 entidades, 8.544 relações,
566 componentes conexos. Entidades: chunk 8.544, doc 283, skill 183, config 33,
pattern 25, adr 18, bug 8...

---

## 4. A ARQUITETURA — quem carrega o quê

```
CLI (graph.go) ──> NewGraph(dir) ──> newGraphAdapter(dir)
                                        │
                                        ├── sql.Open("sqlite", dir/.cosca/knowledge.db)
                                        │       ↑ driver modernc (NUNCA "sqlite3" — mattn não existe aqui)
                                        │
                                        └── graph.New() ← AddNode/AddEdge (em memória, read-only)
```

- **`NewGraph(".")`** — o ponto de entrada de TODOS os comandos graph (show, stats,
  query, export). NUNCA `graph.New()` direto no CLI — é o bug que o L335 matou.
- **Driver**: `modernc.org/sqlite` (puro Go, sem cgo). O mattn (`sqlite3`) não é
  dependência da casa — usar `sql.Open("sqlite", ...)`.
- **Query**: resolve nome/path → UUID antes do BFS (IDs são UUIDs; o Don busca
  por nome). Resultados exibem NOMES legíveis, não UUIDs.
- **Export**: JSON via `Serialize()`, DOT/GraphML via conversores do adapter.
  Sem `-o` → `cosca-graph.<format>` no diretório atual.

---

## 5. A JAULA — armadilhas do sandbox (lições do L335)

| Armadilha | Sintoma | Solução |
|-----------|---------|---------|
| Caminho absoluto do host (`-o /home/...`) não existe na bolha | `export failed` silencioso | O CLI reescreve absoluto → relativo via `COSCA_PROJECT_DIR` |
| `--setenv` ANTES do `--clearenv` no bwrap | env varrida — `COSCA_PROJECT_DIR` vazio | Ordem sagrada do jail: `--clearenv` PRIMEIRO, todos os `--setenv` DEPOIS |
| stderr do processo interno engolido pelo bwrap | erro invisível (só "Export failed") | Debug: escrever log em arquivo DENTRO do workspace (`.cosca/export-debug.log`) — visível do host |
| `SilenceErrors: true` no root | `cosca graph export` falha mudo | Reproduzir via `go run` com main de teste imprimindo `err` |

**Regra de ouro**: se um comando falhar mudo dentro da jaula, o erro REAL está
no stderr do processo interno — capture via arquivo no workspace ou rode fora
da jaula (`COSCA_JAILED=1`).

---

## 6. COMO MANUTER — quando tocar no grafo

1. **Código do adapter**: `internal/cli/adapters_graph_adapter.go` (carrega do DB,
   Query, Export, Stats, Overview). Comandos: `internal/cli/graph.go` + `knowledge.go`.
2. **Testes**: `internal/cli/adapters_graph_adapter_test.go` — DB fake em `t.TempDir()`
   (NUNCA path absoluto da máquina — frágil). Cobre: carregamento, query por nome,
   export (json/dot/graphml + formato inválido), stats, dir vazio.
3. **Pós-mudança**: `go build ./...` + `go test ./internal/cli/ -run "TestNewGraphAdapter|TestGraphAdapter"`
   + teste REAL do CLI (a jaula engole erros que o teste unitário não vê):
   ```bash
   go build -o /tmp/cosca ./cmd/cosca && /tmp/cosca graph stats   # deve mostrar 9k+ nós
   ```
4. **Registrar**: aprendizado no formato canônico (block → chain → gatilho → merkle → commit → sign).

---

## 7. RELAÇÃO COM OUTROS PROTOCOLOS

| Protocolo | Complemento |
|-----------|-------------|
| `KNOWLEDGE_SEARCH_PROTOCOL.md` | busca acha os DOCUMENTOS; o grafo mostra as RELAÇÕES |
| `MEMORY_ACCESS_PROTOCOL.md` | memória = learnings + chain; grafo = knowledge.db |
| `PERFORMANCE_PROTOCOL.md` | carregar 9k nós ~50ms — se ficar lento, fritar |
| `CLEANUP_PROTOCOL.md` | fantasmas (entidade sem arquivo) — limpar no DB direto |