# Relatório de Identidade — Árvores Go (FASE 2)

> **Data:** 2026-08-24
> **Autor:** cosca-kernel
> **Decisão do Don:** Árvore canônica = RAIZ (`C:\Users\Henrique\Documents\cosca`)

---

## 1. Identidade da Árvore Canônica (RAIZ)

| Item | Valor | Status |
|------|-------|--------|
| Caminho | `C:\Users\Henrique\Documents\cosca` | ✅ canônica |
| Git repo | SIM (`.git`) | ✅ |
| Branch | `main` | ✅ |
| HEAD | `43cefe7` (Day/Night cycle) | ✅ mais recente |
| Remote | `github.com/CoscaAI/cosca-v4` | ✅ |
| `unreal/` | presente | ✅ |
| Contém Living World (`worldmodel`) | presente | ✅ |
| Contém bridge (`internal/bridge`) | presente | ✅ |

## 2. Identidade da `cosca/` (subpasta)

| Item | Valor | Status |
|------|-------|--------|
| Caminho | `cosca/` | ⚠️ árvore candidata |
| Git repo | NÃO (sem `.git`) | ❌ |
| Trackeada na raiz | **0 arquivos** | ❌ gitignored |
| No `.gitignore` da raiz | `/cosca` | ✅ confirmado |
| `go.mod` | IDÊNTICO à raiz | ⚠️ cópia |
| `unreal/` | NÃO tem | ❌ |
| `internal/worldmodel` | NÃO tem | ❌ |
| `internal/bridge` | NÃO tem | ❌ |

## 3. Trabalho EXCLUSIVO da RAIZ (não existe na cosca/)

Estes 7 pacotes são o trabalho real que NÃO pode ser perdido — só existem na raiz:

```
internal/bridge        ← WebSocket Unreal (nervoso/corpo)
internal/worldmodel    ← Living World (tipos, orchestrator)
internal/catalog
internal/deliberate
internal/grounding
internal/results
internal/skilleval
```

## 4. Trabalho EXCLUSIVO da cosca/ (candidato a perda)

A `cosca/` tem 109 subdirs em `internal/`, mas **nenhum** exclusivo relevante:
- Todos os pacotes que existem na `cosca/` também existem na raiz (ou são versões mais antigas)
- A `cosca/` NÃO tem `unreal/`, `worldmodel`, `bridge` — exatamente o trabalho novo

## 5. VEREDICTO

> **A `cosca/` é uma árvore ÓRFÃ mais antiga/incompleta.**
> - NÃO contém trabalho exclusivo de valor (falta exatamente o Living World + bridge).
> - É gitignored (nunca foi versionada).
> - É uma cópia antiga do framework, provavelmente de uma instalação/snapshot anterior.
>
> **Nenhum trabalho exclusivo será perdido** ao tratar a raiz como canônica e a
> `cosca/` como candidata a remoção/arquivamento.

## 6. Recomendação (aguardando aprovação do Don)

A `cosca/` NÃO deve ser apagada ainda (regra do Don). Recomendo:
1. **Arquivar** mover para `C:\Users\Henrique\AppData\Local\Temp\opencode\cosca-orphan-backup\` (fora do workspace, preservado) — OU
2. **Congelar** deixar como está, apenas ignorar definitivamente.

## 7. Provenance

Registro no ledger `.cosca/provenance.yaml`:
- `arch-unreal-module` (Unreal = módulo separado, hypothesis, 0.9)
- `arch-kernel-memory` (memória Kernel = knowledge.db + chain, observed, 0.95)
