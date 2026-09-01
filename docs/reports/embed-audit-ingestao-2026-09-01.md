# Relatório — Auditoria do Cérebro + Ingestão do Veterano

**Data:** 2026-09-01
**Autor:** Cosca Kernel (aprovado pelo Don)
**Commits:** `2025779` (ORC no daemon), `61a24af` (fix audit + ingestão 692)
**Escopo:** Etapa 4 da migração — análise, auditoria e ingestão do corpus do Veterano.

---

## 1. Arquitetura de cérebros (o quadro real)

| Cérebro | Caminho | Tamanho | Papel |
|---|---|---|---|
| **Área de edição (Vivo)** | `.opencode/cosca/` | 3,8 MB / 988 arq | Onde o conhecimento é criado/atualizado; fonte dos **templates do opencode** (`.opencode/opencode.json` referencia `KERNEL.md`, `engines/`, `workflows/`) |
| **Cérebro canônico (Embed)** | `internal/embed/cosca/` | 9,8 MB / 2.005 arq | Compilado no binário via `go:embed`. Consumido de verdade: `agents.go`, `catalog.go`, `restcycle.go` (ORC), `init.go` (framework), `cli/embed.go` |
| **Derivado (Runtime)** | `.cosca/fallback/` + `knowledge.db` | 585 MB | Materializado do embed pelo `MaterializeFallback`; o ORC compila `fallback/knowledge` → `knowledge.db` |

**Direção da verdade: embed → disco** (init/fallback/restcycle leem o embed). O Vivo **não alimenta o runtime** — é área de edição + fonte de templates.

## 2. Bug P2 corrigido — audit cego no Windows

`cosca embed audit` classificava **2.004 de 2.005 arquivos como "LEI/IDENTIDADE"**.

- **Causa:** `filepath.Rel` retorna `\` no Windows, mas `classifyEmbedPath` e o manifest da chain comparam com `/` canônico.
- **Correção:** `rel = filepath.ToSlash(rel)` nos 2 pontos (`internal/cli/embed.go`: classificação + `provenanceCount`).
- **Resultado:** classificação real agora: MEMÓRIA 1329, CONHECIMENTO 180, LEI/IDENTIDADE 158, AGENTES 110, SKILLS 89, DEPARTAMENTOS 61, WORKFLOWS 39, TEMPLATES 19, etc. Provenance e duplicação passam a funcionar.

## 3. Ingestão do Veterano (692 edições publicadas)

**Problema:** o embed estava **defasado** — 692 arquivos do Vivo mais novos que o embed (memory 09-01 vs 08-25, shared, skills, workflows). O binário operava com conhecimento antigo.

**Ação:** 692 edições mais novas do Vivo **publicadas no embed** (backup em `Temp/opencode/embed-backup-20260901`). Preservados os **1.274 exclusivos do embed** (memórias geradas, protocolos — zero deletes).

**Divergência restante:** 39 arquivos onde o **embed é mais novo** — intencional (canônico lidera; o Vivo não tem essas edições).

## 4. Decisões P8 (destino do Veterano)

| Item | Decisão | Motivo |
|---|---|---|
| `internal/embed/cosca` | **KEEP** (não tocar em P8) | É o cérebro canônico compilado; tudo consome ele |
| `.opencode/cosca` (Vivo) | **KEEP como área de edição** | `.opencode/opencode.json` referencia seus paths nos templates; deletar quebra o opencode |
| `keys/` | KEEP | Contém apenas `kernel_public.key` (chave pública de verificação — seguro para embed) |

## 5. Fluxo futuro de publicação (quando o Vivo evoluir)

1. Editar no Vivo (área de edição).
2. Publicar no embed: copiar arquivos do Vivo mais novos que o embed (mesmo procedimento desta auditoria).
3. Rebuild + `cosca embed audit` para validar.
4. Commit.

> Regra: **nunca** deletar exclusivos do embed (1274 arquivos) ao sincronizar; e **nunca** regerar o embed a partir do Vivo cegamente (o embed tem edições próprias mais recentes).

## 6. Verificação

- `go build ./...` ✅ · `go vet` ✅ · `go test ./internal/cli/` ✅ (81,8s)
- `cosca embed audit` roda íntegro no Windows ✅
- Binário `bin/cosca.exe` rebuilt: `v1.5.0-345-g61a24af` ✅
