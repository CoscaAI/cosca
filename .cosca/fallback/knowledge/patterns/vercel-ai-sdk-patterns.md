# Vercel AI SDK Patterns — Provider Abstraction, Tool Loop, Streaming

> **Version**: 1.0.0 | **Confidence**: 0.88 | **Category**: AI Agent Patterns | **Created**: 2026-08-22 | **Source**: https://github.com/vercel/ai (Apache-2.0)

> **Mined by**: cosca-kernel (ordem do Don). Extraído de `packages/provider`, `packages/ai`, `packages/sdk`. O SDK de IA mais adotado — o modelo de referência para **abstração de provider** e **tool-loop**.

## Purpose

O Vercel AI SDK resolve o problema de **unificar N providers de LLM sob interfaces tipadas** (chat, embeddings, images, speech, rerank) e de **orquestrar o loop modelo→tools→modelo** com parada controlada e streaming provider-agnóstico.

---

## 1. Provider Abstraction com Versionamento de Spec + Registry Global
- **O que resolve**: integrar N providers (OpenAI, Anthropic, Google...) sem que o core saiba nada sobre as APIs de cada um — unificando chat, completions, embeddings, images, speech, transcription, rerank sob interfaces tipadas.
- **Como funciona**: duas camadas. (a) Spec versionada (`specificationVersion: 'v2' | 'v3' | 'v4'`): cada `LanguageModelV4`, `EmbeddingModelV4`, `ProviderV4` declara uma versão de contrato, permitindo evolução sem quebrar retrocompatibilidade — o core resolve com adaptadores `asLanguageModelV4`/`asProviderV4`. (b) `ProviderV4` é uma factory: `languageModel(modelId)`, `embeddingModel(modelId)`, `imageModel(...)`, `speechModel?`, `rerankingModel?`. O core nunca instancia o provider; faz `resolveLanguageModel(model)` onde `model` pode ser **string** (resolvida via `getGlobalProvider().languageModel(modelId)`) ou **objeto** (adaptado). A string `AI_SDK_DEFAULT_PROVIDER` é o fallback global.
- **Onde**: `packages/provider/src/language-model/{v2,v3,v4}/language-model-v4.ts`, `packages/ai/src/model/resolve-model.ts:31`, `packages/ai/src/registry/provider-registry.ts`.
- **Aplicação no Cosca**: o módulo de `provider` deve espelhar isso: um `ProviderFactory` com `languageModel(id)`/`embeddingModel(id)`, `specificationVersion`, e `resolve-*` que aceita string ou objeto. Os agentes cosca registram providers num registry central; os adaptadores `as-*` normalizam versões de contrato.

## 2. Structured Output / Typed Generation via interface `Output`
- **O que resolve**: gerar saída tipada (não só texto) consistentemente entre `generateText` e `streamText`, com schema provider-agnóstico e streaming incremental do objeto.
- **Como funciona**: a interface `Output<OUTPUT, PARTIAL, ELEMENT>` tem 4 contratos: `responseFormat` (JSON-schema ao provider), `parseCompleteOutput` (parseia + valida com `safeParseJSON` + `safeValidateTypes`), `parsePartialOutput` (usa `parsePartialJson` para reconstruir JSON parcial/truncado) e `createElementStreamTransform`. Fabrica-se via `text()`, `object({schema})`, `array({element})`, `choice({options})`, `json()`.
- **Onde**: `packages/ai/src/generate-text/output.ts:22`, `packages/ai/src/generate-text/parse-partial-json.ts`.
- **Aplicação no Cosca**: o SDK/Cosca deve expor `output` tipado nos providers e agentes. Mapeia para os specialists que respondem JSON validado, e ao RAG (objetos estruturados parciais). O `parsePartialJson` com reparo é essencial para streaming do UI sem quebrar em JSON truncado.

## 3. Tool Abstraction: `FunctionTool`/`ProviderExecutedTool` + `ToolSet`
- **O que resolve**: definir ferramentas ricas que o modelo pode chamar, separando "tool declarada pelo usuário e executada pelo SDK" de "tool executada pelo provider", com schemas fortemente tipados.
- **Como funciona**: `Tool` é uma union discriminada por `type`: `FunctionTool` (schemas usuário, executado pelo SDK), `DynamicTool` (runtime, ex. MCP), `ProviderDefinedTool` e `ProviderExecutedTool` (ex. web-search/code-exec no servidor), com `supportsDeferredResults` (resultado pode voltar em turno futuro). Todo tool tem `inputSchema`, `outputSchema`?, `contextSchema` + `toolsContext`, callbacks `onInputStart/onInputDelta/onInputAvailable`, `toModelOutput`, `needsApproval`. `prepareTools()` converte o `ToolSet` em `LanguageModelV4FunctionTool` respeitando `toolOrder`.
- **Onde**: `packages/provider-utils/src/types/tool.ts:328`, `packages/ai/src/prompt/prepare-tools.ts:15`, `packages/ai/src/generate-text/execute-tool-call.ts:43`.
- **Aplicação no Cosca**: cada agente registra tools via `tool({inputSchema, outputSchema, contextSchema, execute})`. O `toolsContext` vira o "runtime context" que o Cosca injeta (workdir, papéis). O `providerExecuted` + `supportsDeferredResults` permite ferramentas nativas de provider com resultado diferido — ideal para specialists num pipeline multi-agente.

## 4. Multi-Step Tool Loop (o coração do `generateText`/`streamText`)
- **O que resolve**: orquestrar iterações modelo→tools→modelo até terminar, mantendo um histórico ordenado de steps, sem loop infinito.
- **Como funciona**: um `do { ... } while(...)`. Cada iteração: monta prompt, chama `doGenerate`/`doStream`, extrai `tool-call` parts, aplica aprovação (`resolveToolApproval`), executa tools em paralelo via `executeToolCall`, converte resultados em `tool-result` parts, acumula em `steps[]` e `messagesForNextStep`. O loop continua **somente se** `clientToolOutputs.length + denied === clientToolCalls.length` E `clientToolCalls.length > 0` E a condição `stopWhen` (default `isStepCount(1)` em generate; `isStepCount(20)` no agent) não está satisfeita. `pendingDeferredToolCalls` rastreia provider-executed calls com resultado em turno futuro.
- **Onde**: `packages/ai/src/generate-text/generate-text.ts:844`, `stop-condition.ts`, `packages/ai/src/agent/tool-loop-agent.ts:39`.
- **Aplicação no Cosca**: o cosca-workflow-chief usa exatamente a técnica de `steps[]` + `stopWhen` + aprovação de tools. Cada agente de domínio é um `ToolLoopAgent`; a paragem por `isStepCount(max)` e `hasToolCall` mapeia para limites de profundidade (evitar loops infinitos). O `notify()` (callbacks + telemetria) é o modelo para o sistema de eventos do Cosca.

## 5. Streaming em 3 camadas: stream parts → text stream → UI/RSC
- **O que resolve**: um único streaming provider-agnóstico que alimenta texto bruto, outputs estruturados parciais, tool-events e UI de forma reutilizável.
- **Como funciona**: o provider retorna `ReadableStream<LanguageModelV4StreamPart>`. O core normaliza para `TextStreamPart<TOOLS>` (disc. union: `text-delta`, `reasoning-delta`, `tool-input-delta`, `tool-call-*`, `partial-output`, `finish-reason`, `error`) com um **finish-reason unificado** (`finishReason.unified` vs `.raw`). Adapters triviais: `toTextStream()`, `pipeTextStreamToResponse()`, `createTextStreamResponse()`, e `ui-message-stream`/RSC para mensagens de UI. O `smooth-stream` estabiliza deltas de texto.
- **Onde**: `packages/ai/src/generate-text/stream-text.ts`, `.../text-stream/*`, `packages/rsc/src/streamable-value/create-streamable-value.ts`.
- **Aplicação no Cosca**: o cosca-frontend usa a corrente `TextStreamPart → UI`; cosca-sdk reexporta `toTextStream`/`pipeToResponse`. O finish-reason unificado permite ao runtime tratar terminação/erro igual entre providers.

## 6. Capability Ads / Media & URL Routing (`supportedUrls` + download)
- **O que resolve**: cada modelo/provedor declarar o que suporta nativamente (URLs por media type), para o core decidir se envia a URL como está ou faz download.
- **Como funciona**: `LanguageModelV4.supportedUrls` é `PromiseLike<Record<mediaTypePattern, RegExp[]>>`. O core faz `supportedUrls: await stepModel.supportedUrls` e passa ao `convertToLanguageModelPrompt`, que decide `download` para partes `file`/`reasoning-file`/`tool-result` cuja URL não casa com os padrões. `experimental_download` permite customizar.
- **Onde**: `packages/provider/src/language-model/v4/language-model-v4.ts:42`, `packages/ai/src/prompt/convert-to-language-model-prompt.ts`, `packages/gateway/src/gateway-language-model.ts:35`.
- **Aplicação no Cosca**: mapeia para o pipeline de upload-file + media/routing (imagens, PDFs via specialist-frontend e knowledge upload). Declarar "quais media/URLs eu suporto" por provedor evita downloads desnecessários.

## 7. Model Routing & Capacity-Aware Batching (RAG/Embedding)
- **O que resolve**: usar um único modelo de embedding e ainda atingir throughput e corretude máximos — quebrar lotes grandes, respeitar limite por chamada e paralelismo, com retries.
- **Como funciona**: `EmbeddingModelV4` declara `maxEmbeddingsPerCall` e `supportsParallelCalls`. `embedMany()` os resolve; se sem limite faz uma chamada; senão `splitArray(values, maxEmbeddingsPerCall)` para sub-lotes e `splitArray(valueChunks, supportsParallelCalls ? maxParallelCalls : 1)` para paralelismo condicionado, executando cada chunk com `retry` (backoff exponencial) e somando tokens. O `gateway` é a camada de roteamento de modelo (proxy com headers de spec, `ai-language-model-id`, fallback/load-balance).
- **Onde**: `packages/provider/src/embedding-model/v4/embedding-model-v4.ts:35`, `packages/ai/src/embed/embed-many.ts:49`, `packages/ai/src/util/retry-with-exponential-backoff.ts`, `packages/gateway/src/gateway-language-model.ts`.
- **Aplicação no Cosca**: o cosca-semantic-memory segue o `splitArray` + condição de paralelismo por `supportsParallelCalls` para separar chunks de embedding do RAG, com retry. O gateway vira a estratégia de roteamento de modelo do cosca-provider (proxy que escolhe provider/modelo por headers e faz fallback).

---

## Synthesis — o que o Cosca deveria copiar

| # | Padrão | Ganho |
|---|--------|-------|
| 1 | Provider factory + spec versionada + registry | plugar provider novo em 1 arquivo sem tocar no core |
| 2 | `Output` tipado + `parsePartialJson` | streaming de JSON sem quebrar em truncado |
| 3 | Tool union (SDK vs provider-executed) + deferred | ferramentas nativas com resultado diferido |
| 4 | Multi-step loop com `stopWhen` | limites de profundidade, aprovação em tools |
| 5 | Streaming em 3 camadas + finish-reason unificado | UI/SDK reutilizáveis, terminação igual entre providers |
| 6 | Capacity-aware batching de embedding | throughput máximo respeitando limites |

## Related Patterns

- [`openai-agents-sdk-patterns.md`](openai-agents-sdk-patterns.md) — loop/tool (o mesmo conceito, visão openai)
- [`deepseek-harness-patterns.md`](deepseek-harness-patterns.md) — skill registry/catálogo (D1/D2)
- [`aws-agent-toolkit-patterns.md`](aws-agent-toolkit-patterns.md) — segredos na borda/gateway (E4)
