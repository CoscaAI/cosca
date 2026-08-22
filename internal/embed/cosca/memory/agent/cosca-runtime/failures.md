# cosca-runtime — Negative Memory (Failures)

> Auto-evolution memory. Failures are the most valuable teachers. Search before acting.

## F-AUDIT-1 | 2026-08-03 | Rebuild indexando o diretório PAI — banco de 20,7GB | RESOLVIDO
**O que aconteceu:** `adapters_knowledge_adapter.go` usava `RootDir: filepath.Dir(a.dir)`; `cosca knowledge rebuild` varreu projetos vizinhos (order-system, ai-sdk, cosca-test-bk) e rodou 71h gerando 20,7GB de lixo.
**Causa raiz:** bug de escopo de 1 linha (pai em vez do workspace).
**Lição:** auditar o root de qualquer varredura; `filepath.Dir(x) ≠ x`.
**Prevenção:** `RootDir: a.dir`; nunca o pai. Banco antigo preservado como `.legacy-20260803`.

## F-AUDIT-2 | 2026-08-03 | Vetores em formato incompatível (float32 no banco, float64 no código) | RESOLVIDO
**O que aconteceu:** banco vivo com blobs float32 (1536/256 dims); source serializava float64 e fixava dim 128 → busca vetorial retornaria 0 resultados.
**Causa raiz:** divergência binário × source; dim fixa; formato não padronizado.
**Lição:** float32 é o padrão da indústria; dim deve vir do provider.
**Prevenção:** `Dimensions()` do registry; serialização float32.

## F-AUDIT-3 | 2026-08-03 | Falha silenciosa no chat não-streaming | RESOLVIDO
**O que aconteceu:** falha do LLM virava `(result, nil)`; o chamador imprimia só `Content` → resposta vazia com exit 0.
**Causa raiz:** erro engolido em `engine.go:326-330`.
**Lição:** nunca engolir erro de provider; propagar e sinalizar.
**Prevenção:** verificar `result.Error` nos 3 pontos de execução não-streaming do cosca-chat.

## F-AUDIT-4 | 2026-08-03 | `sync --dry-run` gravava arquivos do framework | RESOLVIDO
**O que aconteceu:** `cli/sync.go` chamava `SyncToOpenCode` antes do branch de dry-run.
**Causa raiz:** efeito colateral fora do guard de dry-run.
**Lição:** dry-run é read-only; toda escrita dentro de `if !dryRun`.
**Prevenção:** teste de regressão cobre o caso.

## F-AUDIT-5 | 2026-08-03 | `gate_linux.go:47` usava `--dev /dev /dev` (bug conhecido do bwrap) | RESOLVIDO
**O que aconteceu:** sandbox de chat falhava com "execvp /dev: Permission denied".
**Causa raiz:** formato duplicado de argumento (o jail já tinha corrigido; o gate não).
**Lição:** padrões de argumento do bwrap devem ser únicos; comparar implementações irmãs.
**Prevenção:** bind simples `--dev /dev`.

---
> **Protocol**: [LEARNING_PROTOCOL.md](../../LEARNING_PROTOCOL.md) | **Constitution**: P5 — A família aprende com erros
