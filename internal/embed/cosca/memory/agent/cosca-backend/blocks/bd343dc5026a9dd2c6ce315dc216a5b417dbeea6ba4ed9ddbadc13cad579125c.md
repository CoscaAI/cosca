PREV: 87a53acb7c5ccfefe6104e7a0fe3e4ef6929b0cffa76ffcab829498be885df29
ID: 2026-07-31
TIME: 2026-07-31
LEVEL:
TAGS: #g1 #singleton #registry #per-request #dependency-injection #no-leak #handler #rest
---
### 2026-07-31 — resolveRegistry no RunHandler elimina Select mutante no global
| Field | Value |
|-------|-------|
| **Agent** | cosca-backend |
| **Task** | Eliminar o vazamento de estado do singleton global chat.GetRegistry() no RunHandler — um `POST /v1/run?provider=X` mutava o provider selecionado GLOBALMENTE, afetando requests subsequentes e simultâneos. |
| **Technique** | Level 3 — Injeção per-request: `registryFactory func() *chat.ChatRegistry` no RunHandler (default: `chat.NewChatRegistry()`), `SetRegistryFactory()` para override, e `resolveRegistry(ctx, req)` que cria registry fresco + RegisterChatProviders + Select quando `req.Provider != ""`. O singleton fica read-only para o handler (usado apenas como default quando não há override). |
| **Level** | 3 |
| **Outcome** | success — TestExecuteProviderOverrideDoesNotLeak prova que o override não vaza; build/vet/test verdes, -race limpo |
| **Tags** | #g1 #singleton #registry #per-request #dependency-injection #no-leak #handler #rest |
| **Related** | api/rest/handler/run.go, internal/chat/registry.go (NewChatRegistry), internal/chat/provider/register_chat.go |
| **Learned** | **(1) O padrão de correção é: factory injetada + registry novo por request + registrar + selecionar + descartar.** O singleton global NUNCA recebe Select com override. **(2) ARMADILHA: `collectNonStreamResponse` do ProviderAdapter engole erros** — quando o provider não existe, o teste antigo `TestExecuteWithProviderSelection` passava com 200 e resposta vazia (fallback silencioso para ollama), mascarando que o mock era pulado. Corrigido registrando o mock no registry fresco via SetRegistryFactory. Sempre desconfiar de teste que "passa por razão errada". **(3) `Select` nunca falha com RegisterChatProviders porque a factory do ollama sempre sucede** — o caminho determinístico de erro do resolveRegistry é factory retornando nil → RegisterChatProviders retorna erro. Documentado no teste. |
| **Next** | Considerar variante do TestExecuteProviderOverrideDoesNotLeak para Stream. G5 (unificação engines) permanece pendente de decisão estratégica. |
