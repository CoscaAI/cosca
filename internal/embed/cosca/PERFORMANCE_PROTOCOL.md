# PERFORMANCE PROTOCOL — Fritar e medir (a régua da performance)

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — *"coloca pra fritar quero ver se realmente tem desempenho bom"*
> **Propósito**: quando o Don fala **"fritar"** ou **"performance"**, o kernel vai DIRETO ao ponto —
> medir, isolar, achar o gargalo, corrigir com evidência (P13). Nasceu da investigação que
> derrubou a busca de conhecimento de **2.2s → 0.46s** e expôs 3 gargalos que nenhum teste pegava.

---

## 1. O QUE É "fritar"

A metáfora da casa: **colocar o sistema sob carga real e MEDIR**. É o checkup de
**PERFORMANCE** — complementar ao CARRO_PROTOCOL (que é saúde/integridade). "Fritar" =
auditoria de performance medida, não narrativa. "A esteira verde prova correção; a fritura
prova velocidade."

---

## 2. GATILHO — quando executar

| Fala do Don | Ação |
|-------------|------|
| `fritar` / `coloca pra fritar` | executar o §3 (medição + diagnóstico + fix) |
| `performance` / "ta lento" | idem — carregar ESTE arquivo e executar o §3 |

**Regra de ouro**: ir direto ao §3. Medir PRIMEIRO, otimizar DEPOIS. Nenhuma otimização
sem medição (P13). Nenhuma afirmação de "melhorou" sem número antes/depois.

---

## 3. CAMINHO RÁPIDO (runbook, na ordem)

```bash
# 1. MEDIR O TODO — a operação real (não o componente)
time cosca knowledge search "q"          # ex.: comando CLI lento
time curl -s http://127.0.0.1:14120/ready # API do serve
free -h; uptime                           # hardware: sobra RAM/CPU? (gargalo não é falta)

# 2. ISOLAR CAMADAS — o custo fixo vs variável
/usr/bin/time -f "%es" cosca version        # custo-base do binário (admin, sem jaula)
/usr/bin/time -f "%es" cosca knowledge stats  # mesmo família, sem busca
COSCA_JAILED=1 /usr/bin/time -f "%es" cosca knowledge search "q"  # sem jaula

# 3. INSTRUMENTAR O CAMINHO REAL — timings temporários no RunE
#    (fmt.Fprintf(os.Stderr, "PROBE_T x=%v", time.Since(t0))) → rodar → REMOVER

# 4. BISECT — probe de cada fase fora do CLI (go run em cmd/probe_*)
#    config.Load? New+Init? search? Close? global? → o probe aponta o componente

# 5. CORRIGIR O COMPONENTE (não o sintoma) — ver §4 para os fixes conhecidos

# 6. MEDIR DEPOIS — antes → depois com números reais
# 7. Commit + sign (ordem sagrada L199) + binário reinstalado (make install)
```

**Só otimizar com medição na mão** — o probe é a régua. Se o componente não for o
gargalo, NÃO mexa nele.

---

## 4. FALHAS CONHECIDAS → FIX (estudo de caso de 2026-08-16)

| Sintoma | Causa raiz | Fix |
|---------|-----------|-----|
| Serve lento sob carga, memória estranha | **cgroup `MemoryMax=1G`** no cosca-serve.service (máquina com 31GB) | `MemoryMax=16G` + daemon-reload + restart graceful (kill -TERM, L56). Conferir cgroup real: `cat /sys/fs/cgroup$CGP/memory.max` |
| CLI search ~2.2s (busca em si é 4ms) | **`Close()` re-persistia o grafo inteiro** no SQL (DELETE + re-insert de 13k entidades) a cada invocação, mesmo read-only | **dirty-flag no `graph.Graph`** (Add/Remove marcam; `loadGraph` chama `MarkClean()` após ler o cache; `saveGraph` pula se limpo) |
| `SearchSymbols` 41s CPU | **`json.Unmarshal` por linha** sobre 60k embeddings (code_symbols) | **decode binário** float64 (LittleEndian) no insert e no read (`encodeVec`/`decodeVec`) |
| Busca vetorial serial | brute-force O(60k) cosine em Go serial | **`scoreParallel`** (NumCPU goroutines) + **`SearchWithCandidates`** (L3 refina candidatos FTS/BM25 + pool recente em vez de 60k brutos — híbrido-first) |
| `cosca knowledge search` 6s → 2.2s → 0.46s | soma dos acima (Init frio + Close caro) | Fixes acima; o piso atual ≈ jaula 0.2s + Init 114ms + busca 4ms |

---

## 5. AS VERDADES DURAS (lições que custaram o dia)

1. **Esteira verde ≠ performance.** Testes de correção não medem tempo. O `go test ./...`
   verde convivia com uma busca de 2.2s — nenhum teste pegava.
2. **Medir primeiro, sempre.** O componente "óbvio" (o brute-force) era 400ms — o gargalo
   real estava no `Close()`. Sem o probe, teríamos otimizado o lugar errado.
3. **Gargalos escondidos clássicos:** (a) custo por invocação CLI (Init/Close/jaula), (b)
   cgroups do systemd (MemoryMax invisível no código), (c) formato de serialização
   (JSON por linha vs binário — 100x).
4. **Performance se ganha DENTRO da casa** — zero dependência externa (Regra de Libs L98;
   nada de HNSW/hnswlib sem o Don). O brute-force otimizado (paralelo + binário +
   híbrido-first) basta para ~60-100k vetores.
5. **O custo fixo é o piso, o variável é onde caçar.** Todo comando CLI paga: jaula
   (~0.2s) + Init (~114ms) + base. O que fazer rápido é o resto.
6. **A resposta do daemon é o caminho do futuro** (5ms vs 0.46s do CLI) — quando o auth
   JWT do `/v1/knowledge/search` for resolvido, o CLI busca pelo daemon 24/7.

---

## 6. GOTCHAS

1. **A jaula engole o stderr** — erro real do processo: `COSCA_JAILED=1`.
2. **`go run` vs binário instalado podem diferir** — medir o binário REAL
   (`~/.cosca/bin/cosca`), não o `go run` (que compila + roda).
3. **Task de agente cancelada pode deixar trabalho parcial no working tree** — antes de
   commitar, `git status`; revisar o que ficou (pode ser bom e completo).
4. **`time` com pipe pega o exit do pipe**, não do processo — usar `/usr/bin/time -f`
   sem pipe, ou capturar `${PIPESTATUS[0]}`.
5. **Probe em `cmd/probe_*` temporário** — criar, rodar, REMOVER (não deixar rastro).
6. **Reinstalar o binário após commit** (`make install` ou `go build -o bin/cosca && cp`),
   senão o teste mede a versão antiga.

---

## 7. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — a régua da fritura (nasceu da investigação 2.2s → 0.46s: cgroup 1G→16G, dirty-flag do grafo, decode binário, busca paralela híbrida) |
