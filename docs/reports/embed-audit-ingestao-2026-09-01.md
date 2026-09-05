# RelatÃ³rio â€” Auditoria do CÃ©rebro + IngestÃ£o do Veterano

**Data:** 2026-09-01
**Autor:** Cosca Kernel (aprovado pelo Don)
**Commits:** `2025779` (ORC no daemon), `61a24af` (fix audit + ingestÃ£o 692)
**Escopo:** Etapa 4 da migraÃ§Ã£o â€” anÃ¡lise, auditoria e ingestÃ£o do corpus do Veterano.

---

## 1. Arquitetura de cÃ©rebros (o quadro real)

| CÃ©rebro | Caminho | Tamanho | Papel |
|---|---|---|---|
| **Ãrea de ediÃ§Ã£o (Vivo)** | `.opencode/cosca/` | 3,8 MB / 988 arq | Onde o conhecimento Ã© criado/atualizado; fonte dos **templates do opencode** (`.opencode/opencode.json` referencia `KERNEL.md`, `engines/`, `workflows/`) |
| **CÃ©rebro canÃ´nico (Embed)** | `internal/embed/cosca/` | 9,8 MB / 2.005 arq | Compilado no binÃ¡rio via `go:embed`. Consumido de verdade: `agents.go`, `catalog.go`, `restcycle.go` (ORC), `init.go` (framework), `cli/embed.go` |
| **Derivado (Runtime)** | `.cosca/fallback/` + `knowledge.db` | 585 MB | Materializado do embed pelo `MaterializeFallback`; o ORC compila `fallback/knowledge` â†’ `knowledge.db` |

**DireÃ§Ã£o da verdade: embed â†’ disco** (init/fallback/restcycle leem o embed). O Vivo **nÃ£o alimenta o runtime** â€” Ã© Ã¡rea de ediÃ§Ã£o + fonte de templates.

## 2. Bug P2 corrigido â€” audit cego no Windows

`cosca embed audit` classificava **2.004 de 2.005 arquivos como "LEI/IDENTIDADE"**.

- **Causa:** `filepath.Rel` retorna `\` no Windows, mas `classifyEmbedPath` e o manifest da chain comparam com `/` canÃ´nico.
- **CorreÃ§Ã£o:** `rel = filepath.ToSlash(rel)` nos 2 pontos (`internal/cli/embed.go`: classificaÃ§Ã£o + `provenanceCount`).
- **Resultado:** classificaÃ§Ã£o real agora: MEMÃ“RIA 1329, CONHECIMENTO 180, LEI/IDENTIDADE 158, AGENTES 110, SKILLS 89, DEPARTAMENTOS 61, WORKFLOWS 39, TEMPLATES 19, etc. Provenance e duplicaÃ§Ã£o passam a funcionar.

## 3. IngestÃ£o do Veterano (692 ediÃ§Ãµes publicadas)

**Problema:** o embed estava **defasado** â€” 692 arquivos do Vivo mais novos que o embed (memory 09-01 vs 08-25, shared, skills, workflows). O binÃ¡rio operava com conhecimento antigo.

**AÃ§Ã£o:** 692 ediÃ§Ãµes mais novas do Vivo **publicadas no embed** (backup em `Temp/opencode/embed-backup-20260901`). Preservados os **1.274 exclusivos do embed** (memÃ³rias geradas, protocolos â€” zero deletes).

**DivergÃªncia restante:** 39 arquivos onde o **embed Ã© mais novo** â€” intencional (canÃ´nico lidera; o Vivo nÃ£o tem essas ediÃ§Ãµes).

## 4. DecisÃµes P8 (destino do Veterano)

| Item | DecisÃ£o | Motivo |
|---|---|---|
| `internal/embed/cosca` | **KEEP** (nÃ£o tocar em P8) | Ã‰ o cÃ©rebro canÃ´nico compilado; tudo consome ele |
| `.opencode/cosca` (Vivo) | **KEEP como Ã¡rea de ediÃ§Ã£o** | `.opencode/opencode.json` referencia seus paths nos templates; deletar quebra o opencode |
| `keys/` | KEEP | ContÃ©m apenas `kernel_public.key` (chave pÃºblica de verificaÃ§Ã£o â€” seguro para embed) |

## 5. Fluxo futuro de publicaÃ§Ã£o (quando o Vivo evoluir)

1. Editar no Vivo (Ã¡rea de ediÃ§Ã£o).
2. Publicar no embed: copiar arquivos do Vivo mais novos que o embed (mesmo procedimento desta auditoria).
3. Rebuild + `cosca embed audit` para validar.
4. Commit.

> Regra: **nunca** deletar exclusivos do embed (1274 arquivos) ao sincronizar; e **nunca** regerar o embed a partir do Vivo cegamente (o embed tem ediÃ§Ãµes prÃ³prias mais recentes).

## 6. VerificaÃ§Ã£o

- `go build ./...` âœ… Â· `go vet` âœ… Â· `go test ./internal/cli/` âœ… (81,8s)
- `cosca embed audit` roda Ã­ntegro no Windows âœ…
- BinÃ¡rio `bin/cosca.exe` rebuilt: `v1.5.0-345-g61a24af` âœ…

## 7. Atualização — Chain re-assinada com autoridade do Don (2026-09-01)

A ingestão dos 692 arquivos disparou a Family Chain (fail-closed funcionando). Os blocks 61/62 foram git-anchored (testemunho de imutabilidade — fallback), mas o DON re-assinou pessoalmente: **Block 63 Ed25519, autoridade real** (fator máquina DPAPI + TTY + consentimento-ao-conteúdo). Chain validada: 63 blocks, 2.005 arquivos.

**Regra operacional (aprovada pelo Don):** mudou o embed ? commit ? DON assina com `cosca-check --sign`. O `--sign-auto` é apenas fallback emergencial, sempre seguido do `--sign`. O `--sign` não quebra com commits (não depende do git HEAD); o git-anchor quebra (lição: os blocks 61/62 quebraram após commits de docs/gitignore).

## 8. Incidente — queda do índice vetorial + recuperação (2026-09-01)

**Sintoma:** a tabela de vetores (que o Don celebrou como "vector é minoria") revelou na verdade uma QUEDA: o knowledge.db tinha 6.031 vetores (11% de cobertura) contra 58.756 no backup de 13:35 (índice completo de 22/08).

**Causa raiz:** o reindex de hoje (18:00-18:12) vetorizou 6.031 chunks e PAROU — o índice ficou caído pela metade. A busca semântica operava com 11% do índice. Não foi perda de dados (o conteúdo/chunks estava intacto; o backup preservou o índice antigo).

**Recuperação:** `cosca knowledge vectors-backfill` (idempotente, aditivo, não-destrutivo) re-embebeu os chunks faltantes em 2 passadas:
- Passada 1: 6.031 ? 54.304 (15.869 embebidos, 400 falharam)
- Passada 2: ? 54.507 (200 embebidos, 200 falharam)
- **Final: 54.507/54.709 chunks = 99,6% de cobertura**

**Gap residual (202):** chunks de 12-53 chars ("## Strengths", headers pequenos) — curtos demais para o modelo de embedding (nomic-embed-text 768-dim). Não é conhecimento perdido; é ruído de fragmentação rejeitado legitimamente pelo provider.

**Lições:**
1. A tabela de vetores é métrica de SAÚDE do índice, não de design — o Don deve pedir `cosca db mirror`/`vectors-backfill --dry-run` para auditar cobertura.
2. O backup automático salvou o índice antigo — a retenção de backups (decisão da Fase A: NÃO apagar) provou valor.
3. Sempre verificar cobertura (vetores/chunks) após qualquer operação de reindex ou housekeeping.
