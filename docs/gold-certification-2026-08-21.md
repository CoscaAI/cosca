# COSCA GOLD CERTIFICATION — Manifesto de Proveniência

> **Estado**: GOLD — sistema inteiro validado e recuperável
> **Data**: 2026-08-21
> **Gerado por**: cosca-kernel, por ordem do Don
> **Semântica**: GOLD = estado conhecido-bom do sistema inteiro. Não significa
> projeto terminado. Toda mudança futura é evolução/experimento a partir deste baseline.

---

## 1. COMMIT

| Campo | Valor |
|---|---|
| **Commit SHA** | `0d4db194d9a1d61d4ca47252ef3dc86d3445a22e` |
| **Mensagem** | `fix cosca exec: propagar --model ao provider local + reportar erro no stderr (nao silencioso)` |
| **Working tree** | limpa (0 arquivos pendentes) |
| **Branch** | main |
| **Chain** | Family chain válida — 5 blocks, 1986 files |

## 2. AMBIENTE DE EXECUÇÃO

| Campo | Valor |
|---|---|
| **OS** | Microsoft Windows 11 Pro |
| **Arquitetura** | AMD64 |
| **Go** | go1.26.7 windows/amd64 |
| **CGO** | 0 |
| **GOPATH** | C:\Users\Henrique\go |
| **GOMODCACHE** | C:\Users\Henrique\go\pkg\mod |
| **GOTMPDIR/TEMP** | C:\Users\Henrique\cosca-test-tmp |
| **CPU** | 16 logical processors |
| **RAM** | 31.9 GB |
| **Provider** | ollama (local) — modelo `qwen2.5-coder:14b` |
| **Embedding** | nomic-embed-text (local) |

## 3. ARTEFATOS (binários oficiais)

| Binário | SHA-256 |
|---|---|
| **cosca.exe** | `CEED48F6D8E1F94298D7ACC1C9C6867907262BA43F1D50B923BF6843EC47AED7` |
| **cosca-check.exe** | `7EE774A83D44F17C5BCA426A85422BAC903AC684A92705654437BEDC7BACA80B` |
| **cosca-indexer.exe** | `BBD64D4878C610CCB2CBCA96DA50B66877555E33C88DF4AA424C159909F26F1D` |
| **cosca-merkle.exe** | `81D1E6F41E5EF54FFF3488CFA5D0BA59C6D89056C4355AAB01128C7DBB3A2553` |
| **acquireall.exe** | `B1A06956DD5C4D29B4CE2143FE164FAD0C835347AE5181CCD59264F3A9399355` |

## 4. RESULTADOS DE VALIDAÇÃO

### 4.1 Build
- `go build ./...` — **PASS** (exit 0)
- 5 binários oficiais compilados

### 4.2 Testes
- `go test ./... -count=1` — **141 pacotes ok, 0 FAIL**
- Testes raiz: **7431 PASS, 0 FAIL** (`go test ./... -v`)
- Subtests: 2293+ (variáveis por timing — 0 falhas em todas as execuções)
- **QGate**: **9736/9736 → TOTAL 100/100 → READY TO COMMIT**
  - Duração: 2m36–3m20 (com cache) / 3m19 final

### 4.3 Static analysis
- `go vet ./...` — **limpo** (exit 0, 0 erros)

### 4.4 QGate (2 execuções independentes)
- Exec 1: `9736/9736 ✅ 100/100 ✅ READY TO COMMIT` (3m19)
- Exec 2: `9736/9736 ✅ 100/100 ✅ READY TO COMMIT`
- Deps: 1 vuln UNKNOWN (`golang.org/x/crypto/openpgp`) — não-bloqueante, pré-existente

### 4.5 Runtime / Bootstrap
- Serve REST: `/ready` → `200 {"ready":true,"subsystems":{"knowledge":"available","memory":"available","runtime":"healthy"}}`
- `/health` → `200 {"healthy":true}`
- Ollama: `200 {"version":"0.32.15"}`
- `cosca status`: Runtime running, Knowledge 34334 entries, Vectors 32348
- `cosca despertar`: identidade verificada, chain criptograficamente válida
- Restart serve: contínuo (READY 200 após restart, dados preservados)

### 4.6 Persistência
- knowledge.db: 186 MB, WAL zerado, `cosca knowledge verify` → 0 issues
- audit.db / auth_tokens.db / department.db / secrets.db / trace.db: presentes, sem corrupção
- Continuidade: restart preservou 34334 entries

### 4.7 Integridade
- Family chain: **5 blocks, 1986 files — valid**
- Kernel self-test: **íntegro** (identity, laws, constitution, epistemology)
- Memory integrity: **Status: OK**
- Embed audit: 1986 arquivos, 0 duplicação, 0 órfãos
- Legado `cosca-chat`: **não existe** (removido, não voltou)
- Embed: 55 agents / 1037 memórias hash-idênticas ao framework

### 4.8 Regressão funcional
| Cenário | Resultado |
|---|---|
| `cosca run --provider none` | ✅ EXIT 0 |
| `cosca exec --model qwen2.5-coder:14b` | ✅ EXIT 0, resposta OK |
| `cosca chat --model none` | ✅ EXIT 0 |
| `cosca despertar` | ✅ chain válida |
| Knowledge search | ✅ 10 resultados |

### 4.9 Correções incluídas neste GOLD
1. **QGate**: contagem real de testes (`-v`), flag `--no-tests` viva, timeout 240s
2. **Analytics**: persistência no history (handler + stream)
3. **CKL**: IDs de evidência via crypto/rand (relógio Windows)
4. **Cache embeddings**: evicção determinística com `seq`
5. **`cosca exec`**: `--model` propagado ao provider local + erro reportado no stderr
6. **Config**: modelo `qwen2.5-coder:14b` (corrigido de `14b-128k`)

## 5. NOTAS / RESSALVAS DOCUMENTADAS

- **`cosca exec` determinístico (`--model none`)**: por design retorna erro
  "deterministic mode: no cognitive capability" (o exec requer modelo de IA real).
  O modo determinístico oficial é `cosca run --provider none` e `cosca chat --model none`.
- **Subtests variáveis**: a contagem total do QGate varia (9666–9736) por subtests
  condicionais ao timing; **0 falhas em todas as execuções**.
- **1 vuln UNKNOWN**: `golang.org/x/crypto/openpgp` (pacote descontinuado na
  dependência; o bcrypt usado pela família é seguro) — não-bloqueante.
- **Jaula no Windows**: bwrap indisponível — `COSCA_ALLOW_NO_ROOT=1` explícito,
  fail-closed preservado (sem opt-in, exit 1).

## 6. REPRODUTIBILIDADE

Para reproduzir a certificação:
```powershell
$env:TEMP = "C:\Users\Henrique\cosca-test-tmp"
$env:TMP  = "C:\Users\Henrique\cosca-test-tmp"
go build ./...
go test ./... -count=1
go vet ./...
cosca qgate --no-color          # 9736/9736, 100/100, READY TO COMMIT
```
Commit: `0d4db194d9a1d61d4ca47252ef3dc86d3445a22e`

---

**CERTIFICADO GOLD — 2026-08-21**
**Este é o estado conhecido-bom do sistema inteiro.**
