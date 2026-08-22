# cosca-runtime — Reusable Patterns

> Discovered patterns that can be reapplied. Search before acting.

## Pattern P1 — Validar formato de vetores antes de buscar
**Sintoma:** busca vetorial retorna 0 resultados sem erro.
**Checagem:** verificar dimensão do store vs dimensão do provider vs bytes reais dos blobs no banco (`hexdump`/`struct.unpack`): float32 = 4 bytes/valor, float64 = 8.
**Correção:** alinhar `SQLiteVecConfig.Dimension` com `embRegistry.Dimensions()` (nunca fixar dim no código); serializar float32.

## Pattern P2 — Auditar o escopo do RootDir em adapters
**Sintoma:** banco de conhecimento incha muito além do esperado, indexando projetos vizinhos.
**Checagem:** `RootDir: filepath.Dir(a.dir)` vs `RootDir: a.dir`. O workspace é o diretório que contém `.cosca/`.
**Correção:** sempre `RootDir: a.dir`; nunca o pai.

## Pattern P3 — Banco novo exige AutoMigrate
**Sintoma:** "no such table: X" no primeiro rebuild/index.
**Checagem:** `knowledge.Config` literal sem `AutoMigrate: true` → `sqlite.Open` não roda migrações.
**Correção:** `AutoMigrate: true` no Config literal, ou comparar com `DefaultConfig()`.

## Pattern P4 — Detectar binário stale
**Sintoma:** comportamento estranho que "o código não faz" (vetores ilegíveis, features mortas).
**Checagem:** `cosca version` vs HEAD; `strings bin | grep <commit>`; rebuildar antes de culpar o código.
**Correção:** `go build -o cosca ./cmd/cosca` e repetir o teste.

## Pattern P5 — Verificar wiring de subsistemas (código morto em produção)
**Sintoma:** componente documentado nunca aparece em `/v1/status`.
**Checagem:** grep por `Register<X>` / `New<X>` em `internal/cli/serve.go`; hooks `init<X>` que começam com `if r.x == nil { return nil }`.
**Correção:** adicionar a linha de wiring no ponto de integração (padrão adapter + `var _ rt.Subsystem = ...`).

## Pattern P6 — Erro engolido = bug invisível
**Sintoma:** falha silenciosa, exit 0, sem mensagem.
**Checagem:** procurar `(result, nil)` onde `result.Error != ""`; chamadores que só leem `Content`.
**Correção:** propagar erro; imprimir mensagem clara; exit != 0.

## Pattern P7 — Dry-run deve ser read-only de verdade
**Sintoma:** `--dry-run` ainda escreve arquivos.
**Checagem:** procurar efeitos colaterais ANTES do branch de dry-run.
**Correção:** mover toda escrita para dentro de `if !dryRun { ... }`.

## Pattern P8 — Divergência binário × source
**Sintoma:** banco escrito por processo que o source atual não lê.
**Checagem:** formato de bytes dos blobs (float32 vs float64), dimensões, versão do processo vs source.
**Correção:** decidir um formato canônico; migrar; nunca deixar dois formatos convivendo.

## Pattern P9 — Gate fail-closed no entrypoint
**Sintoma:** runtime carrega antes de uma auditoria de governança, ou um env var silencioso contorna a proteção.
**Checagem:** validar o manifesto no entrypoint antes de providers/memória; classificar comandos por allowlist explícita (`init`, `version`, `memory integrity init|verify`).
**Correção:** falhar com mensagem acionável em PT-BR e exit não-zero; usar o mesmo resolvedor de raiz no gate e no comando offline; nunca aceitar bypass por ambiente.

> Protocol: [LEARNING_PROTOCOL.md](../../LEARNING_PROTOCOL.md) | Auto-evolution: search before acting.
