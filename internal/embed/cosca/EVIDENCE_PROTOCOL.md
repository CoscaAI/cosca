# EVIDENCE PROTOCOL — A epistemologia operacional (quarentena, evidência, conflito)

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — *"analise todos os comandos, falta algum protocolo?"*
> **Propósito**: referência operacional ÚNICA para o pipeline de VERIFICAÇÃO do conhecimento —
> a porta dos fundos epistemológica. Complementa `KNOWLEDGE_SEARCH_PROTOCOL.md` (busca) e
> `PROFESSOR_PROTOCOL.md` (ensino): busca acha, professor ensina, ESTE protocolo garante
> que **nada entra no conhecimento sem passar pela quarentena**.

---

## 1. O PRINCÍPIO — nada nasce confiável

A casa tem uma lei epistemológica (P13): **nenhum fato sem verificação, nenhuma
métrica sem medição**. Na prática:

> **EXTERNAL DATA ≠ TRUSTED DATA** — tudo o que vem de fora (internet, IA, fontes
> divergentes) nasce **UNTRUSTED**. Só a promoção manual do Don (ou do kernel com
> aprovação) move algo para o conhecimento.

Três mecanismos protegem o conhecimento:

| Mecanismo | O que protege | Comando |
|-----------|---------------|---------|
| **Quarentena** | o que a IA inventa | `cosca quarantine` |
| **Evidência externa** | o que a internet traz | `cosca evidence` |
| **Conflito** | fontes que divergem | `cosca conflict` |

---

## 2. A QUARENTENA — o que a IA inventa não entra direto

Fluxo de vida de uma proposal (invenção da IA):

```
proposal → quarantine → validation → evidence → promotion (ou archival)
pending  →  pending   →  validating  → validated → promoted / archived
```

```bash
cosca quarantine add --title "X" --content "..." --source agent-x   # Q-XXXX (pending)
cosca quarantine list                                               # ativas
cosca quarantine validate Q-0001                                    # pending → validating
cosca quarantine promote Q-0001 --to K-06                           # validating → promoted
cosca quarantine discard Q-0001                                     # arquiva (NUNCA deleta)
```

**Regras de ouro:**
- Cada proposal é um arquivo `.cosca/quarantine/Q-XXXX.json` (append-only).
- **Nunca deletar** — o que não sobrevive é ARQUIVADO em `.cosca/quarantine/archive/`
  (princípio do curador: a falha também é evidência).
- A promoção na quarentena NÃO grava em laws/knowledge — promover ao conhecimento
  é uma etapa manual/aprovada SEPARADA (dupla portaria).

---

## 3. A EVIDÊNCIA EXTERNA — o que a internet traz

Fluxo de aquisição (ACQUIRE → VERIFY/HASH → QUARANTINE):

```
fetch <url> ──> SHA-256 ──> A-XXXX (artefato) + Q-XXXX (proposal external, pending)
                                        │
                                        └──> promote A-XXXX --item K-XX (P2/P4) [manual]
```

```bash
cosca evidence fetch https://raw.githubusercontent.com/CoscaAI/cosca/main/README.md --allow-remote
cosca evidence list
cosca evidence show A-0001
cosca evidence promote A-0001 --item K-01
```

**Regras do Don (fail-closed):**
- **SSRF fail-closed**: `--allow-remote` explícito; apenas http/https; IPs
  privados/loopback/link-local são REJEITADOS por padrão
  (`COSCA_ACQUISITION_ALLOW_PRIVATE=1` é o opt-in explícito para mocks/LAN).
- O corpo fica em `.cosca/quarantine/artifacts/A-XXXX` — **NUNCA** entra direto
  em knowledge/laws/memory.
- Conteúdo externo **nunca é executado** durante a aquisição.
- Nada é promovido automaticamente: `promote` exige `--item` explícito.

---

## 4. O CONFLITO — fontes que divergem

Quando duas fontes divergem sobre o mesmo item, a casa **NÃO escolhe uma**
(A + B → IA escolhe uma é proibido). Registra o conflito e **recusa promover**
o item para lei enquanto o conflito estiver aberto:

```
Fonte A ──┐
          ├── CONFLICT (bloqueia promoção)
Fonte B ──┘
```

```bash
cosca conflict new --item K-27 --claim-a "K-27:E-101" --claim-b "K-27:E-203" --desc "A diz X, B diz o contrário"
cosca conflict list              # abertos (default)
cosca conflict list --resolved
cosca conflict show CONFLICT-001
cosca conflict resolve CONFLICT-001
```

- Cada conflito vive em `.cosca/conflict.db` (SQLite dedicada).
- Convenção das alegações: `"item:evidence"` (ex.: `K-27:E-101`).
- Resposta honesta possível: *"Existem duas fontes conflitantes. Não vou
  promover isso para uma lei."* — integridade > velocidade.

---

## 5. O CLAIM — classificar afirmações

Toda afirmação pode ser classificada antes de virar conhecimento:

```bash
cosca knowledge claim add --text "..." --type FACT --source ...
```

| Tipo | Significado |
|------|-------------|
| **FACT** | verificado, com evidência |
| **EVIDENCE** | observado, rastreável à fonte |
| **INFERENCE** | derivado de fatos (não observado direto) |
| **ASSUMPTION** | premissa não verificada |
| **HYPOTHESIS** | proposta para testar |
| **UNKNOWN** | honestidade — não sei |

**Regra**: o tipo é parte do contrato da afirmação (CL-XXXX) — nunca promover
um ASSUMPTION como FACT sem antes passar pela quarentena/evidência.

---

## 6. AS LEIS (CKL) — o topo da hierarquia

O item do CKL (Cosca Knowledge Law) é o que `promote` mira:

```bash
cosca knowledge law list          # leis com nível, confiança e evidências
cosca knowledge law add ...       # registrar lei (só após evidência suficiente)
```

Uma lei legítima tem: nível (L1–L5), confiança, evidências (P0–P5) — ver
`KNOWLEDGE_SEARCH_PROTOCOL.md` §4 (explicabilidade) e o PROFESSOR (epistemologia).

---

## 7. QUANDO USAR O QUÊ — a árvore de decisão

| Situação | Use |
|----------|-----|
| "A IA propôs algo novo" | `quarantine add` → validate → promote |
| "Preciso de dado externo (RFC, docs, GitHub)" | `evidence fetch --allow-remote` → promote --item |
| "Duas fontes dizem coisas diferentes" | `conflict new` (bloqueia promoção até resolver) |
| "Classificar uma afirmação" | `knowledge claim add --type ...` |
| "Promover algo a lei" | `knowledge law` (após evidência suficiente) |

**Ordem canônica do pipeline**: CLAIM (classifica) → QUARANTINE (protege) →
EVIDENCE (verifica externo) → CONFLICT (resolve divergência) → LAW (promove).

---

## 8. RELAÇÃO COM OUTROS PROTOCOLOS

| Protocolo | Complemento |
|-----------|-------------|
| `KNOWLEDGE_SEARCH_PROTOCOL.md` | busca/rank/explicabilidade do que JÁ é conhecimento |
| `PROFESSOR_PROTOCOL.md` | epistemologia conceitual (o PORQUÊ das regras) |
| `MEMORY_ACCESS_PROTOCOL.md` | learnings do kernel (memória ≠ conhecimento) |
| `SECURITY_PROTOCOL.md` | SSRF/quarentena de código executável (esta é a de conteúdo) |