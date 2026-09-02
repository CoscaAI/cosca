# ADR-009: Webhooks — **evento + id**, assinatura **HMAC-SHA256**, timeout 5 s, retry com mesmo id, delivery logs, endpoint de teste

> **Status:** Proposta 🔵 (aguarda aprovação do Don)
> **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-09-01
> **Projeto:** RIZOMAI — plataforma API-first de gestão de redes sociais
> **Referência (base):** `arquitetura-zernio.md` §7 (eventos/entrega) e §10 P0; `integracoes-zernio.md` §4.7 (assinatura, eventos); zernflow route.ts (ack-antes-processa-depois).

---

## 0. Contexto

Webhooks são o que permite construir apps sobre a API sem perder evento (P0 da mineração — o app
zernflow foi construído sobre o padrão de webhook da Zernio). Sem um contrato de entrega confiável,
qualquer consumidor perde eventos, duplica processamento ou fica sem depurar. Requisitos:
**autenticidade** (só quem tem o secret envia), **idempotência na entrega** (retry não duplica),
**auditabilidade** (logs de entrega) e **testabilidade** (endpoint de teste). Risco conhecido (R6):
timeout curto + retry podem gerar retry-storm se o consumidor processar **antes** de confirmar.

## 1. Decisão

**Contrato de webhook com 7 elementos obrigatórios:**

1. **Evento + id**: payload `{ "id": "<event_id UUID>", "event": "<tipo>", "timestamp": "<ISO>",
   "data": { ... } }`. O `event id` é único por evento gerado (gerado pelo RIZOMAI) e é o ponto de
   dedup do consumidor.
2. **Assinatura HMAC-SHA256**: header **`X-Rizomai-Signature`** = `sha256=<hex do HMAC-SHA256 do body
   bruto com o secret>`. Verificação com comparação de tempo constante (`crypto.subtle.timingSafeEqual`
   / `hmac.Equal`). Assinatura inválida → o consumidor deve **rejeitar (401)** antes de processar.
3. **Timeout de entrega de 5 s**: se o endpoint não responder 2xx em 5 s, a entrega é abortada e
   entra em retry. **Documentar** isso no contrato (consumidor lento = retry-storm).
4. **Retry com o mesmo event id**: política `5 s × 2^n`, janela **24 h** (máx ~12 tentativas);
   depois, `failed` definitivo + log. Como o id nunca muda, o consumidor faz **dedup por event id**
   (unique constraint na tabela de eventos processados — `23505`/duplicate = já processado, responde
   200). **Ack antes, processa depois**: o consumidor deve confirmar a entrega **antes** de trabalho
   pesado e enfileirar o processamento na própria fila (padrão zernflow `after()`).
5. **Delivery logs**: `GET /v1/webhooks/logs?status=success|failed&event=...&webhookId=...` —
   histórico de cada tentativa (status HTTP, tempo, erro). **Redelivery manual** por evento
   (`POST /v1/webhooks/{id}/redeliver/{eventId}`).
6. **Endpoint de teste**: `POST /v1/webhooks/test` — envia um evento `webhook.test` real, com o
   mesmo fluxo de assinatura/retry, para o usuário validar a integração.
7. **Configuração**: `{ name, url (HTTPS obrigatório), secret, events[], isActive,
   customHeaders? }`. **Nunca retornar o secret** em `GET` (só `masked`); gerar secret forte no
   POST. Default de eventos: `["post.published", "post.failed"]`.

### Eventos (catálogo inicial)

| Evento | Quando |
|---|---|
| `post.scheduled` | post agendado na fila |
| `post.published` | todos os targets publicados |
| `post.partial` | **alguns targets falharam** (o mais importante — ADR-007) |
| `post.failed` | todos falharam |
| `account.connected` / `account.disconnected` | ciclo de vida da conta (health — ADR-006) |
| `webhook.test` | endpoint de teste |

(Fase 2+: `message.received`, `comment.received` para inbox unificado.)

## 2. Consequências

**Prós**
- Confiança: consumidor verifica autenticidade e deduplica com segurança.
- Sem perda de evento: retry + logs + redelivery manual = auditável e suportável.
- Contrato idêntico ao que o mercado valida (Zernio/Stripe/Twilio) — SDKs e apps de terceiros
  entendem na hora.
- `webhook.test` reduz atrito de integração (e dá "besteira-teste" p/ debugar).

**Contras**
- Retry-storm se consumidor for lento (mitigado: timeout 5 s + **documentar ack-antes-processa** no
  guia; sem isso, R6 se materializa).
- Gerenciamento de segredos (secret nunca em GET; rotação manual; armazenar hash? — decisão de
  segurança na implementação).
- Fila de delivery própria (worker River dedicado — mais um consumidor para operar).
- Payloads de eventos precisam de schema no OpenAPI (ADR-005) para SDKs tiparem os eventos.

## 3. Alternativas consideradas

1. **Polling (cliente busca mudanças)** — **rejeitado**: latência, custo de polling, não é
   event-driven; inviável para AI agents e apps de inbox (o valor do produto).
2. **Webhooks sem assinatura** — **rejeitado**: qualquer um pode forjar eventos; inseguro e
   incompatível com compliance.
3. **Timeout longo (30 s+)** — **rejeitado**: trava o worker de delivery; o padrão de mercado é
   curto + retry com mesmo id.
4. **Sem delivery logs / redelivery** — **rejeitado**: suporte e depuração impossíveis (P0).

## 4. Referências

- `arquitetura-zernio.md` §7 (headers, payloads, política de entrega) e §10 P0.
- `integracoes-zernio.md` §4.7 (assinatura `x-*-signature`, eventos, `webhook.test`).
- `arquitetura-zernio.md` §11 R6 (retry-storm → ack-antes-processa-depois).
- ADR-003 (worker de delivery via River), ADR-005 (schema dos eventos no spec), ADR-007 (post.*).

---

*ADR de Fase 1 — RIZOMAI. Decisão de stack, aguardando aprovação do Don.*
