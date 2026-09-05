# cosca-testing — Reusable Patterns

> Discovered patterns that can be reapplied. Grows with agent evolution.

## Patterns Discovered

### P-WIN-1: Triage cross-platform de falhas de teste (Windows port)

| Campo | Valor |
|-------|-------|
| **Agente** | cosca-testing |
| **Data** | 2026-08-21 |
| **Tags** | #windows #cross-platform #testing #port |
| **Origem** | Frente B — 12 pacotes portados Ubuntu→Windows |

**Fluxo**: para cada falha, classificar em (a) específico de plataforma → `t.Skip(runtime.GOOS == "windows")` com justificativa pt-BR; (b) bug real de produção Windows → corrigir produção (fixes de produção PRIMEIRO, comportamento Linux inalterado); (c) teste desatualizado → corrigir o teste (separadores via `filepath.Join`/`filepath.FromSlash`, não strings hardcoded).

**Checklist de causas raiz conhecidas no Windows**:
1. `f.Sync()` em handle O_RDONLY → `ERROR_ACCESS_DENIED`. Sempre abrir temp com O_WRONLY antes de Sync.
2. `filepath.IsAbs("/etc")` = false → semântica POSIX só em Unix.
3. Chaves lógicas com `/` (paths de documento) → usar pacote `path`, nunca `path/filepath`.
4. Clock grosseiro → `time.Since()` pode medir 0; não asserar `> 0` em timing.
5. `os.Symlink` → requer admin; skip se criação falhar.
6. Permissões POSIX (0600/0755/0000) → mapeiam para 0666/no-op no NTFS; guard `runtime.GOOS != "windows"`.
7. Shell → `COMSPEC`/cmd.exe é o fallback; limpar nos testes de isolamento.
8. Shebang shell scripts → não executam; skip de fake-executáveis.
9. Paths de teste → construir com `filepath.Join` (separador nativo) para validar o comportamento real.

**Reuso**: aplicar ao revisar qualquer pacote com asserções de path/permissão/timing/exec. Validar com `GOOS=windows go build ./...` + `GOOS=linux go build ./...` após mudanças de produção.
