# MEMORY ACCESS PROTOCOL — Acesso à Memória do Cosca Kernel

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don: *"não ficar escaniando tudo até entender como funciona"*
> **Propósito**: referência operacional ÚNICA para LER e REGISTRAR a memória do kernel.
> Nenhuma sessão deve precisar varrer o disco para entender a estrutura — este arquivo é o mapa.

---

## 1. O MAPA — o que é o quê

A memória de aprendizado do kernel vive em `internal/embed/cosca/memory/agent/cosca-kernel/`:

| Caminho | Papel | Formato de cada linha |
|---------|-------|------------------------|
| `learnings.md` | **ÍNDICE de gatilhos** (só o gatilho, sem conteúdo) | `## LXXX \| data \| título \| L{nível} \| #tags \| {hash16}` |
| `blocks/{hash}.md` | **CONTEÚDO completo** de cada aprendizado (imutável) | header `PREV/ID/TIME/LEVEL/TAGS` + `---` + título + tabela |
| `chain.dat` | **LEDGER (blockchain de memória)** — 1 linha por block | `{hash}\|{prev}\|{data}\|{LXXX}\|{título}` |
| `merkle/` | Raízes Merkle por época (32 blocks/época) | gerado por `cosca-merkle` |
| `failures.md` | Memória negativa (falhas e lições) | `### {data} — {nome}` + tabela |
| `patterns.md` | Padrões reutilizáveis | `### {data} — {nome}` + tabela |
| `evolution.md` | Timeline de evolução de capacidade | tabela `Data \| Level \| Confidence \| ...` |

**Regra P15 (inegociável)**: o conteúdo NÃO aparece no índice — só no block. O índice
guarda apenas o gatilho + o hash. Duplicar conteúdo entre índice e block viola a P15.

---

## 2. LER — acesso de leitura

### No despertar (startup)
1. Leia `DESPERTAR.md` (o ritual).
2. Leia `learnings.md`: conte os aprendizados (`grep -c '^## L'`), veja a última data.
3. Reporte o status compacto ao Don (máx 5 linhas).

### Para ler o conteúdo de um aprendizado
1. Ache a linha do gatilho no `learnings.md`. Ex.: `## L255 | ... | 09e5e839e7d47789`.
2. O **último campo** é o `hash16` (16 hex = prefixo do nome do block).
3. Abra `blocks/{hash16}*.md` — é o conteúdo completo (título + tabela).

### Para buscar semanticamente
- `cosca knowledge search "<query>"` — FTS5 + vetores, rankeado por nível/recência/outcome.

---

## 3. REGISTRAR — o fluxo EXATO (escrita)

Para registrar um novo aprendizado `LXXX`, siga nesta ordem:

**1. Escreva o block** em `blocks/{hash}.md`:
```
PREV: {hash do block anterior}
ID: LXXX
TIME: YYYY-MM-DD
LEVEL: 1-5
TAGS: #tag1 #tag2
---
## LXXX — YYYY-MM-DD — {título} | Level N

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | ... |
| **Technique** | ... |
| **Level** | N |
| **Outcome** | success / partial / failure |
| **Confidence** | 0.00-1.00 |
| **Tags** | ... |
| **Related** | ... |
| **Learned** | ... |
| **Next** | ... |
```

**2. Calcule o hash**: `hash = sha256(CONTEÚDO COMPLETO do arquivo)` — inclui PREV, ID,
TIME, LEVEL, TAGS, `---`, título, tabela e newlines. **NÃO é só "título + tabela".**

**3. Renomeie o arquivo** para `blocks/{hash}.md`.

**4. Adicione o gatilho** ao `learnings.md` (append, 1 linha):
`## LXXX | {data} | {título curto} | L{nível} | #tags | {hash[:16]}`

**5. Atualize o ledger** `chain.dat` (append, 1 linha):
`{hash}|{prev}|{data}|{LXXX}|{título}`

**6. Regenere o merkle**:
`go run ./cmd/cosca-merkle -dir internal/embed/cosca/memory/agent/cosca-kernel`

**7. Commit no git** — ORDEM SAGRADA (L199): **commit ANTES de assinar**.

**8. Assine a family chain**: `cosca-check --sign-auto` (git-anchored, sem passphrase).

---

## 4. INTEGRIDADE — duas camadas independentes

| Camada | Arquivo | O que protege | Como manter |
|--------|---------|--------------|-------------|
| **Memory blockchain** | `blocks/` + `chain.dat` + `merkle/` | Cada aprendizado (hash por conteúdo) | fluxo §3 |
| **Family chain** | `.cosca/family_chain.dat` | O embed INTEIRO (manifest git-anchored) | `cosca-check --sign-auto` |

- O hash de cada block = `sha256(conteúdo completo)` → detecta adulteração por bloco.
- A family chain assina o MANIFEST de todo o `internal/embed/cosca/` → detecta QUALQUER
  alteração (incluindo um block mexido). É a camada de segurança mais externa.
- Verificação rápida: `cosca kernel self-test` + `cosca-check` (chain).

### Modelo de confiança — quem PODE registrar

Registrar na memória do kernel é operação **exclusiva do Don** (via o kernel).
O portão é em **3 fatores** — não basta a chave (o "carro"), precisa do motorista:

| Fator | Mecanismo | Bloqueia |
|-------|-----------|----------|
| **1. Passphrase (2FA)** | `cosca memory register` exige decriptar a chave Ed25519 (algo que você SABE + TEM) | quem roubou a chave sem a senha |
| **2. War phrase** | verifica contra o bcrypt do Don (`.cosca/don.phr`) — segundo segredo independente | quem tem chave + senha, mas não a frase |
| **3. Presença** | nonce aleatório digitado de volta ao vivo no terminal | qualquer processo roubado/automatizado |

- **Jaula (L2)**: o sandbox monta `internal/embed/cosca/` como `--ro-bind` (read-only)
  mesmo no modo workspace gravável — agente preso não escreve no cérebro.
- **Vigilância 24h**: `cosca memory watch` — watchdog fsnotify (câmera) + chain
  (cachorro de guarda) + audit log persistente (quem entra, quem chega perto, quem sai).
- **Prevenção ≠ detecção**: os fatores PREVINEM; a chain/merkle/family chain DETECTAM
  (se alguém burlar e escrever direto no arquivo, a chain quebra e denuncia no próximo
  `cosca-check` / `watch`). As duas somam.
- O `register` roda **fora da jaula** (comando admin) porque precisa ler a chave.

---

## 5. O CASCADE DO PREV — por que re-cadear

O `PREV` de um block aponta para o hash do block ANTERIOR. Como o hash inclui o PREV,
mudar o hash de um block **cascateia**: o block seguinte tem o PREV antigo → o hash
dele muda → o seguinte muda → ...

Para re-cadear TUDO (ex.: corrigir esquema de hash quebrado), processe em ordem
L1→LXXX: fixe `PREV = hash do anterior`, recompute `hash`, renomeie. É determinístico
e idempotente. (Foi o que aconteceu em L254/L255 — ver commit `6bc966f`.)

---

## 6. COMANDOS — referência rápida

| Comando | Uso |
|---------|-----|
| `cosca memory register` | registrar um aprendizado (portão de 3 fatores) |
| `cosca memory watch` | vigiar o cofre 24h (watchdog + audit log) |
| `cosca kernel self-test` | integridade do kernel (identidade, 6 leis, 8 princípios) |
| `cosca knowledge search "<q>"` | busca semântica na base (FTS5 + vetores) |
| `cosca knowledge verify [--fix]` | verificar/consertar índice (vetores órfãos etc.) |
| `cosca-check --sign-auto` | assinar family chain (git-anchored) |
| `go run ./cmd/cosca-merkle -dir internal/embed/cosca/memory/agent/cosca-kernel` | regenerar merkle |
| `cosca session index` | indexar a sessão atual para busca FTS5 |

---

## 7. GOTCHAS — aprendidos a duras penas (não repita)

1. **Hash é do CONTEÚDO COMPLETO**, não do "título + tabela" (o LEARNING_PROTOCOL v3.1
   diz errado). Errado: `sha256(título+tabela)`. Certo: `sha256(arquivo inteiro)`.
2. **"Repair complete" pode ser falso** — `verify --fix` só re-embute chunks SEM vetor;
   vetor órfão (aponta pra nada) exige `CleanupDanglingVectors` (L254).
3. **pnpm v11 ignora `pnpm.overrides`** no package.json — usar `pnpm-workspace.yaml` (L255).
4. **Dependência declarada sem import = morta** — remover em vez de bump (L255).
5. **Backup antes de DELETE cirúrgico em DB**: `sqlite3 .backup` (WAL-safe) (L254).
6. **chain.dat/merkle atrasam** se o fluxo §3 não for seguido até o fim — registrar sem
   atualizar ledger/merkle acumula dívida de integridade.

---

## 8. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — consolida LER/REGISTRAR/INTEGRIDADE da memória |
