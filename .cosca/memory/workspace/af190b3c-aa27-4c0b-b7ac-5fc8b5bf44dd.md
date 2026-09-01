---
id: af190b3c-aa27-4c0b-b7ac-5fc8b5bf44dd
type: session
layer: workspace
scope: security
created_at: 2026-08-31T09:25:23.7837507-03:00
updated_at: 2026-08-31T09:25:23.7837507-03:00
ttl: 0s
priority: 0
version: 0
metadata:
    agent: cosca-mcp
    provenance: P0
    source: EVIDENCE
---

2026-08-31 — Fechado o gap do engine_builder: conjunto LEGACY removido do caminho engine/chat, guard read-before-write ativo no write_file.

| Field | Value |
|-------|-------|
| Agent | cosca-security |
| Task | Migrar engine_builder.go do conjunto LEGACY (tool.NewReadTool/WriteTool/EditTool/GlobTool de internal/chat/tool/filesystem.go) para o conjunto SEGURO de internal/chat/tools/filesystem (com guard determinístico read-before-write, ADR-034) |
| Technique | Level 3 — substituição das 4 chamadas LEGACY por filesystem.Register no buildEngineWithMode + wrapper registerFilesystemTools (testável) + teste de regressão TestEngineFilesystemToolsUseReadBeforeWriteGuard em internal/cli; gofmt + go build ./... + go vet ./... + go test (cli/chat/tools) |
| Level | 3 |
| Outcome | success |
| Tags | #read-before-write #engine_builder #filesystem-guard #legacy-tools #guard-regression #adr-034 #security-gap-closed |
| Related | internal/cli/engine_builder.go L72-77 (registerFilesystemTools), L378-384 (helper); internal/chat/tools/filesystem/filesystem.go Register L747, WriteFileTool.Execute L165-174; internal/chat/tool/registry.go Execute L101-105; internal/chat/tool/filesystem.go (LEGACY — mantido) |
| Learned | (1) O caminho do engine/chat (buildEngineWithMode, usado por cosca exec/serve/chat) usava o conjunto LEGACY que sobrescrevia cegamente (só .bak). Migrado para filesystem.Register(toolRegistry, workspace) via wrapper registerFilesystemTools (nomeado p/ ser testável). (2) rails (sandbox.NewRails) e sbGate NÃO foram removidos: rails ainda é usado por tool.NewSearchTool(L95); sbGate por shell/search/git/build/test/MCP. Shell tool preservado. (3) O legacy internal/chat/tool/filesystem.go NÃO ficou órfão: ainda usado pelos próprios testes (tool/filesystem_test.go, tool/security_boundary_test.go, tool/tool_extreme_test.go) — mantido, NÃO deletado (follow-up: descontinuar quando os testes legacy forem migrados). (4) registry.Execute já propaga result.Error (L101-105), então a rejeição do guard é observável via engine path — crítico p/ o teste de regressão. (5) Teste de regressão TestEngineFilesystemToolsUseReadBeforeWriteGuard: asserta que o conjunto registrado tem read_file/write_file/edit_file/list_dir/glob e NÃO tem write/read/edit legacy; e que write_file em arquivo existente SEM leitura prévia recusa ('already exists') e NÃO sobrescreve; e que read_file antes autoriza o write_file. (6) Windows: symlink tests (TestSymlinkEscapeRejected etc.) skipam por falta de privilégio — esperado, zero impacto no guard. (7) Verificação: build=0, vet=0, go test ./internal/chat/tools/... tudo verde (incl. TestWriteFileGuard_*), cli full ok (97s), legacy chat/tool ok. |
| Next | Follow-up (NÃO fazer agora): descontinuar/remover os tools legacy (read/write/edit/glob de internal/chat/tool/filesystem.go) quando os testes legacy forem migrados p/ o conjunto novo; auditar se algum outro ponto do código ainda constrói engine com o conjunto LEGACY. |
