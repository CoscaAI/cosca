# BRIDGE ARCHITECTURE — Referência: Ponte Host↔OpenCode (padrão Vercel AI SDK)

> **Status**: reference | **Date**: 2026-08-01 | **Owner**: Cosca Kernel
> **Fonte**: AI SDK da Vercel — `packages/harness-opencode` (Apache-2.0), copiado em
> `/home/cosca/Documents/ai-sdk`. Análise completa: `.cosca/memory/project/ai-sdk-analysis.md`.
> **Uso**: este documento é o **mapa da arquitetura** para o futuro bridge Cosca
> (Kernel ↔ OpenCode/agentes). NÃO é código a copiar — é o padrão a entender.

## Propósito

Como um orquestrador (host) conversa com um coding agent externo (OpenCode, Claude Code,
Codex) de forma **segura, versionada e retomável**, tratando o agente como se fosse um
modelo de linguagem (sessão → generate → detach/stop/destroy).

## Arquitetura em 3 camadas

```
┌─────────────────────────────┐          ┌────────────────────────────────┐         ┌───────────────┐
│  HOST (orquestrador)        │  WS +    │  BRIDGE (Node, no sandbox)     │  SDK    │  AGENTE       │
│  HarnessAgent (Vercel)      │ frames   │  bridge.mjs + tool-relay       │  v2     │  OpenCode     │
│  = futura Cosca Kernel      │  Zod     │  = nosso futuro bridge Cosca   │         │  (servidor)   │
└──────────────┬──────────────┘          └────────────────────────────────┘         └───────────────┘
               │ spawn: node bridge.mjs --workdir ... --bridge-state-dir ...
               │ env: BRIDGE_WS_PORT, BRIDGE_CHANNEL_TOKEN, HOME/XDG (isolados)
               └──────────────────────────────────────────────────────────────────────────┘
```

**Descoberta-chave**: o bridge **NÃO spawna o CLI do agente por PTY** — usa o **SDK
oficial do agente** (`@opencode-ai/sdk/v2` para OpenCode; `client.v2.session.create/get`
etc.). O agente roda como servidor; o bridge é o cliente do SDK. Isso é muito mais
simples e robusto que o padrão PTY do Traycer.

## Protocolo (frames WebSocket)

Frames validados por **Zod na borda** (`harnessV1BridgeOutboundMessageSchema`, union
discriminada por `type`). Duas famílias:

### 14 eventos de consumidor (bridge → host)
`stream-start` · `text-start` · `text-delta` · `text-end` · `reasoning-start/delta/end` ·
`tool-call` · `tool-approval-request` · `tool-result` · `finish-step` · `finish` ·
`file-change` · `compaction` · `error` · `raw`

### 5 frames de controle/transporte (bridge → host)
- `bridge-hello` — handshake (versão, capacidade, sanity check)
- `bridge-detach` — resposta ao detach, carrega payload de resume (adapter-specific)
- `bridge-thread` — coordenada de resume anunciada proativamente (ex: thread id do Codex)
- `sandbox-log` — linha de console capturada (stderr→warn, stdout→info)
- `debug-event` — evento estruturado de diagnóstico

### 6 comandos inbound (host → bridge)
`start` · `tool-result` · `tool-approval-response` · `user-message` · `abort` ·
`shutdown` · `resume` (replay de eventos com `seq > lastSeenEventId`) · `detach`

## Ciclo de vida — o trinômio de durabilidade

| Operação | O que faz |
|---|---|
| `doPromptTurn` | Envia `start { operation: 'prompt', prompt, tools, model, provider, ... }` |
| `doContinueTurn` | Envia `start { operation: 'prompt', prompt: 'Continue.', ... }` (ou resume) |
| `doDetach` | `stopped=true`, suspende canal, retorna `{ port, token, lastSeenEventId, sandboxId, openCodeSessionId }` |
| `doStop` | Persiste estado + para |
| `doDestroy` | Envia `shutdown`, espera processo (timeout 5s), limpa sandbox |
| `doSuspendTurn` | Boundary para time-slices (workflow ~800s) |
| `doCompact` | Compaction manual entre turnos (custom instructions = unsupported para OpenCode) |

### Estratégia de resume (attach → rerun → replay)
1. **attach**: bridge ainda vivo nas mesmas coordenadas `{port, token, lastSeenEventId}` → reconecta direto
2. **rerun**: bridge morto → nova instância do bridge com `BRIDGE_REPLAY_FROM_DISK=1`
   (replay do `event-log.ndjson` a partir do `lastSeenEventId`)
3. `classifyDiskLog(log)` decide se o log em disco é replaiável

### Tools do host
- Tools **declaradas no host** (ex: `deploy`, `weather`) executam no **processo host**
- Resultados voltam ao agente via `submitToolResult({ toolCallId, output, isError })`
- Aprovação de tools via `submitToolApproval({ approvalId, approved, reason })`
- `builtinTools` do OpenCode: read/write/edit/bash/glob/grep/webfetch/agent/todowrite/skill
- Tool relay com auth própria (`tool-relay-auth.ts`) — o bridge encaminha chamadas de tool
  entre o SDK do agente e o MCP do host (`host-tool-mcp.ts`)

## Segurança (o que a Cosca deve copiar no espírito)

1. **Token por sessão**: `randomBytes(32).toString('hex')` → `BRIDGE_CHANNEL_TOKEN`, nunca em log
2. **Ambiente isolado**: `HOME`/`USERPROFILE`/XDG dirs criados no sandbox, nunca no host
3. **Frames validados na borda**: schema (Zod) em cada mensagem — frame inválido ≠ processado
4. **Sandbox como fronteira**: o host nunca executa código do agente diretamente
5. **Capability unsupported explícito**: `HarnessCapabilityUnsupportedError` em vez de
   "capabilities object" estático — impossibilidade é erro tipado
6. **Diagnóstico estruturado**: `bridge-diagnostics.ts` + `classifyDiskLog` para debugar
   bridge sem acesso ao sandbox

## Aplicabilidade à Cosca

| Padrão Vercel | Equivalente/Plano Cosca |
|---|---|
| HarnessAgent (harness-v1) | Futuro Cosca AgentBridge (Kernel ↔ OpenCode/agentes) |
| Sandbox com bridge Node | Nosso jail/bwrap + bridge em Go |
| SDK do agente (`@opencode-ai/sdk/v2`) | Usar o MESMO SDK — OpenCode expõe API, não precisamos de PTY |
| Frames Zod na borda | Validação no nosso boundary (Go: structs + validação) |
| detach/stop/suspendTurn + resume | Computação retomável no Compute Fabric |
| Time-slices ~800s | ORC/circadian em escala cloud |
| `HarnessCapabilityUnsupportedError` | Erros tipados do nosso runtime |

## Referências
- Fonte: `packages/harness-opencode/src/opencode-harness.ts` (1.024 linhas) + `src/bridge/index.ts` (1.384 linhas) + `packages/harness/src/v1/harness-v1-bridge-protocol.ts` (325 linhas)
- Análise: `.cosca/memory/project/ai-sdk-analysis.md`
- Contrato: [RUNTIME_CONTRACT.md](../RUNTIME_CONTRACT.md) seção 9 (versionamento por método)
