# COSCA — RUNBOOK DE RECUPERAÇÃO E ESTADO GOLD (2026-09-05)

> **Objetivo:** eternizar o passo a passo para, se der problema de novo, a família voltar
> ao estado bom atual SEM sofrer. Foi difícil chegar aqui — este documento é a "memória
> de ouro" para não repetir o caminho às cegas.
>
> **Autor:** Cosca Kernel (nº de aprendizado do Don — "olha, foi difícil")
> **Data:** 2026-09-05 · **Estado de referência:** GOLD (módulos semânticos íntegros,
> busca funcionando, GOLD DB versionado)
> **Regra de segurança do Don:** kernel opera SOMENTE LEITURA no git (sem push/merge/
> commit/remover/rebase). Escrita só sob ordem explícita.

---

## 0. O QUE É O ESTADO GOLD (a meta do runbook)

É o estado em que **o sistema está operável e a busca semântica funciona de verdade**:
- Runtime (`cosca serve`) sobe e responde HTTP 200 em 127.0.0.1:14120.
- Family chain VÁLIDA (sem BREACH).
- Módulos semânticos `vector-*.db` POPULADOS com os vetores.
- Split ADR-013 ÍNTEGRO (`db verify` OK).
- Todos os bancos ABAIXO do teto de 100 MB (ADR-013 Decisão 1).

---

## 1. PONTO DE PARTIDA: O QUE QUEREMOS (o "estado bom")

| Métrica | Valor alvo |
|---------|-----------|
| Runtime | `running` (HTTP 200) |
| Chain | `valid` (blocks N, files 2005) |
| Vetores nos módulos | 15.773 (`vector-*.db` soma) |
| Split | `db verify` íntegro |
| Gate 100 MB | todos os bancos ok |
| GOLD POINT | um commit marcado `GOLD POINT` |
| GOLD DB | branch com bancos versionados |

---

## 2. DIAGNÓSTICO RÁPIDO (5 comandos de leitura — SEMPRE primeiro)

```powershell
# 1. A chain está ok?
cosca-check.exe --root "C:\Users\Henrique\Documents\cosca"
#   → "Chain valid" = ok | "FAMILY CHAIN BREACH" = re-assinar (ver §3)

# 2. O serve sobe?
cosca serve   # e em outro terminal:
# curl http://127.0.0.1:14120/health  → 200 = ok

# 3. Os módulos têm vetores?
#  (ver contagem por módulo)

# 4. O split está íntegro?
cosca db verify   # → "Split íntegro" = ok

# 5. Algum banco estourou o teto?
cosca db check --gate   # → exit 0 e "todos ok"
```

---

## 3. PASSO 1 — CONSERTAR A CHAIN (a causa nº 1 de "não sobe")

### Por que quebra
A chain usa **git-anchor**: cada bloco referencia o hash do commit. **Quando o HEAD do
git muda (qualquer commit novo), a chain quebra** → `GIT COMMIT MISMATCH / REWRITTEN`.
Isso NÃO é erro — é o fail-closed agindo certo. A solução é re-assinar.

### Passo a passo
```powershell
# Re-assinar a chain ancorando no HEAD atual (git-anchored, testemunho de imutabilidade)
cosca-check.exe --root "C:\Users\Henrique\Documents\cosca" --sign-auto
#   → "✅ Block N git-anchored"

# Verificar
cosca-check.exe --root "C:\Users\Henrique\Documents\cosca"
#   → "✅ Chain valid"
```

**IMPORTANTE (a lição que custou caro):** quando você FAZ UM COMMIT NOVO, a chain
quebra. Então a ordem correta é: **commit primeiro → re-assinar DEPOIS**. Se você
re-assinar antes do commit, quebra de novo (o commit muda o HEAD).

---

## 4. PASSO 2 — SUBIR O SERVE

O serve NÃO sobe sem 3 coisas (fail-closed em camadas):
1. **Chain válida** (senão `FAMILY CHAIN BREACH` bloqueia).
2. **`COSCA_ALLOW_NO_ROOT=1`** (jaula indisponível no Windows — opt-in explícito).
3. **`COSCA_JWT_SECRET`** (senão `refusing to start`).

```powershell
# Pode subir em dev mode (admin com senha aleatória) com o JWT do .env
$env:COSCA_ALLOW_NO_ROOT="1"
$env:COSCA_DEV_MODE="true"
$env:COSCA_JWT_SECRET="<segredo do .env>"
cosca serve

# Validar
# curl http://127.0.0.1:14120/health  → 200
# curl http://127.0.0.1:14120/ready   → {"ready":true,...}
```

**O JWT_SECRET deve estar persistido no `.env` (gitignored)** — senão o serve não sobe
de novo sem setar de novo.

---

## 5. PASSO 3 — RESTAURAR/POPULAR OS MÓDULOS SEMÂNTICOS (busca funcional)

### O FLUXO CORRETO (a chave que resolveu — NÃO esqueça)
O `vectors-backfill` escreve os vetores NO MONOLITO BASE (`knowledge.db`). Depois o
`db build` SINCRONIZA para os módulos. **O backfill sozinho NÃO popula os módulos.**

```powershell
# 1. Re-embed dos chunks (escreve no knowledge.db base) — serve PARADO para não travar
cosca knowledge vectors-backfill
#   → meta: "faltam: 0" (todos os 15773 chunks com vetor)

# 2. Sincroniza knowledge.db -> vector-*.db (popula os módulos)
cosca db build
#   → "Módulos físicos criados (ADR-013 Fase C)"

# 3. Verifica o split
cosca db verify
#   → "Split íntegro — módulos equivalentes à fonte"
```

**ATENÇÃO (o erro que custou caro):** o `db build` é DESTRUTIVO nos módulos (recria
do zero). Se os módulos já têm dados do vetor nos `vector-*.db` e você roda `db build`,
ele APAGA e recria — e se o `knowledge.db` estiver com `vectors=0`, os vetores se perdem.
**SEMPRE rode o `vectors-backfill` ANTES do `db build`** quando precisar repopular.

---

## 6. PASSO 4 — GOLD POINT + GOLD DB (versionar a "foto" do estado)

### GOLD POINT (commit com marker na mensagem)
O `cosca recovery --restore` volta neste commit e reconstrói os módulos.
```powershell
# 1. Commit com a palavra "GOLD POINT" na mensagem
git commit -m "chore(gold): GOLD POINT 2026-09-05 - <descricao>"

# 2. RE-ASSINAR a chain (o commit mudou o HEAD!)
cosca-check.exe --root "C:\Users\Henrique\Documents\cosca" --sign-auto
```

### GOLD DB (bancos versionados — os módulos são gitignored, força o add)
```powershell
# Forçar o commit dos bancos (gitignored -> -f), SEM sidecars WAL/SHM
git add -f .cosca/knowledge.db .cosca/core.db .cosca/graph.db .cosca/projects.db .cosca/semantic.db .cosca/vector-*.db
git commit -m "db(gold): GOLD DB 2026-09-05 - <descricao>"
```

---

## 7. CHECK-UP FINAL (a prova de que chegou lá)

```powershell
cosca-check.exe --root "C:\Users\Henrique\Documents\cosca"   # Chain valid
cosca db check --gate                                        # todos ok (exit 0)
cosca db verify                                              # Split integro
cosca knowledge search "orquestracao de agentes"            # 10 resultados
cosca recovery                                               # Sistema integro + GOLD POINT disponivel
curl http://127.0.0.1:14120/health                          # 200
```

---

## 8. LIÇÕES ETERNAS (o que NÃO fazer)

1. **NUNCA** rodar `db build` sem antes rodar `vectors-backfill` (apaga os vetores dos módulos).
2. **NUNCA** re-assinar a chain ANTES do commit (o commit muda o HEAD e quebra de novo).
3. **NUNCA** reescrever histórico / rebase / squash em histórico já compartilhado — quebra a chain (REWRITTEN/REBASED).
4. **NUNCA** remover/commitar sem ordem do Don (regra imutável do Don).
5. **O `knowledge.db` NÃO é o cérebro** — é fonte + buffer de escrita. O cérebro/identidade é `.opencode/cosca/` (arquivos .md). O `knowledge.db` é apenas o índice derivado + fonte de dados; os vetores de verdade vivem nos módulos (`vector-*.db`).
6. **Doc (README/CLI) desatualiza** — P2: código executado é a verdade. Verificar com `cosca agent list` / `skill status` / `workflow list`.
7. **Fail-closed é seu amigo** — quando o serve "não sobe" ou o `verify` acusa, é um portão de segurança te dizendo que algo mudou. Diagnosticar (ler o log), não contornar.

---

## 9. REFERÊNCIAS (onde está cada pedaço)

| Assunto | Caminho |
|---------|---------|
| Runbook de recuperação (este) | `docs/reports/cosca-recovery-runbook-2026-09-05.md` |
| Diagrama modular ADR-013 | `docs/adr/ADR-013-modular-knowledge-databases.md` |
| Níveis de soberania (L1/L2/L3) | `docs/COSCA_LEVELS.md` |
| GOLD certification | `docs/gold-certification-2026-08-21.md` |
| Auditorias | `docs/reports/cosca-*.audit-*.md`, `cosca-master-audit-*.md` |
| Config do git | `.git/config` (remotes `origin` + `origin-readonly`) |

---

> **"A vergonha não é descer; descer é a sabedoria que evita o loop de morte."**
> — Léssico do COSCA (L434). Este runbook existe para que o próximo despertar
> **já saiba o caminho** e não precise sofrer de novo.
