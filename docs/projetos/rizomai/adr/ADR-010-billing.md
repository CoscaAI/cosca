# ADR-010: Billing **usage-based por conta conectada (graduado)** — metering próprio + **Stripe** (cartão + PIX)

> **Status:** Proposta 🔵 (aguarda aprovação do Don)
> **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-09-01
> **Projeto:** RIZOMAI — plataforma API-first de gestão de redes sociais (foco LATAM/PT-BR)
> **Referência (base):** `produto-zernio.md` §4 (modelo atual vs legado), §9 (recomendação de preço); `arquitetura-zernio.md` §9 (billing metered, account-day) e §11 R7.

---

## 0. Contexto

O modelo de pricing da Zernio validou: **meter por conta conectada (graduado), posts ilimitados
sempre, tudo incluso, sem feature tiers** — o modelo legado deles com cotas de posts foi um erro
abandonado (R7). Para nós (LATAM/PT-BR, ~R$1.000/mês de infra, público dev + agências): precisamos de
um medidor de uso confiável, um gateway que aceite **cartão internacional E PIX** (barreira de
entrada do mercado brasileiro), invoices itemizados e transparência (cap de gasto, sem surpresa).

## 1. Decisão

**Billing usage-based por conta conectada, com faixas graduadas, metering próprio (eventos
account-day) e Stripe como gateway primário (cartão + PIX via Stripe Brasil).** Sem cotas de posts,
sem feature tiers.

### 1.1 Modelo de preço (base — valores a validar com o Don)

| Faixa de contas conectadas | Preço por conta/mês | Referência BRL |
|---|---|---|
| 1–3 | **Grátis para sempre** (sem cartão) | R$ 0 |
| 4–10 | **US$ 4** | ~R$ 24 |
| 11–100 | **US$ 2** | ~R$ 12 |
| 101+ | **US$ 1** | ~R$ 6 |
| Agência (assinatura, futura) | US$ 49/mês base (até 10 contas + sets + white-label + suporte PT-BR) | ~R$ 290 |

- **Graduado, não flat** (20 contas = 3 grátis + 7×4 + 10×2 = US$ 48/mês) — âncora psicológica
  validada pela Zernio; **undercut de $6→$4** no tier de entrada (produto §9.2).
- Gratuidade modelada como **crédito** contábil (facilita fatura/contabilidade).
- **Posts ilimitados** sempre; limite é throughput (rate limit por key/tenant), não post.
- **Aditivos metered** (primeiro X grátis): mensagens outbound 10 k/mês grátis depois US$1/10 k;
  X/Twitter pass-through **subsidiado no free** (ex.: 300 req/mês grátis; depois repasse) — política
  de subsídio é decisão do Don (§4).

### 1.2 Metering (implementação)

- **Evento account-day**: 1 unidade por conta conectada por dia (job diário `metering` via River —
  ADR-003). Instrumentar desde o dia 1 (eventos `account.connected/disconnected` alimentam o medidor).
- **Agregação idempotente** (job pode rodar 2× sem duplicar): tabela `billing_usage(team, account,
  date, units)` com unique `(team, account, date)`; prorata diário simples.
- **Exposição na API**: `GET /v1/usage` → `{ planName, billingPeriod, billingAnchorDay,
  usage: { accounts, lastReset }, limits: {...} }` — pre-flight de quota na UI/dashboard e para
  agentes (padrão Zernio `usage-stats`; `lastReset` + `billingAnchorDay` dizem quando o ciclo vira).
- **Metering próprio, não Metronome** (R7): Metronome é poderoso mas caro e complexo; nosso modelo
  é um só meter (account-day) → próprio em SQL é simples, auditável e zero custo extra.

### 1.3 Gateway de pagamento

- **Stripe** como gateway primário: cartão (internacional) **e PIX** (via Stripe Brasil, com
  webhooks `payment_intent.*`). Um só provedor cobre os dois mundos LATAM/global — evita 2 billers
  (a complexidade telephony-style da Zernio que decidimos não copiar).
- **Abstração de gateway** (interface `internal/billing/gateway`) para alternativas (Asaas/Pagar.me)
  se o custo/taxa do PIX da Stripe não for competitivo no BR — decisão de negócio futura, sem mudar
  o modelo.
- **Invoices itemizados** por conta (transparência radical — produto §4.3), cap de gasto opcional
  (ex.: X pass-through com aviso a 80% e pausa a 100%), cobrança por acúmulo (1º charge a partir de
  ~US$10 — anti-fraud).

## 2. Consequências

**Prós**
- **Alinhamento de incentivo**: receita cresce com contas conectadas (automação), não com post
  (gesto básico) — mesmo modelo validado pela Zernio.
- **Simples de explicar e operar**: 1 meter, 1 gateway, prorata diário; suporte barato.
- Transparência (usage na API + invoices itemizados) gera confiança de infra B2B.
- PIX + cartão = cobre Brasil e exterior com um único provedor; free "para sempre" sem cartão
  remove atrito de adoção (produto-vírus).
- Metering idempotente em SQL = auditável e zero custo extra de infra.

**Contras**
- Account-day exige job de agregação **confiável e idempotente** (mitigado: unique constraint +
  replay; webhook `account.disconnected` cobre o prorata do dia).
- Risco de disputa (mitigado: invoices itemizados + uso visível na API + política de crédito clara).
- X/Twitter pass-through = custo variável real por post (decisão de política de repasse/subsídio —
  §4).
- Dependência do Stripe (mitigado: interface de gateway; 1 provedor a menos que 2-biller, ainda
  assim lock parcial).

## 3. Alternativas consideradas

1. **Metronome + Stripe (padrão Zernio)** — **rejeitado agora**: poderoso, mas complexo/caro para 1
   meter; R7 diz "começar simples, instrumentar desde o dia 1". Gate para reabrir: billing com
   múltiplos meters (msg + ads + telephony) e volume alto.
2. **Planos com cotas (Free/Build/Accelerate/Unlimited)** — **rejeitado**: erro comprovado do modelo
   legado Zernio (cota de posts estrangula o gesto básico; tier de $667 cria teto de mercado).
3. **Flat per-seat (X$/usuário)** — **rejeitado**: não reflete o valor (contas conectadas) e penaliza
   automação; usage-based é o padrão de mercado API-first.
4. **Asaas/Pagar.me como único gateway** — **rejeitado como primário**: cobre PIX/boleto mas não
   cartão internacional com a mesma maturidade; Stripe+PIX é o melhor de ambos (validação de taxa a
   fazer — §4).
5. **Sem medidor (faturar manual)** — **rejeitado**: insustentável, anti-transparência.

## 4. Pontos que exigem decisão do Don (relacionados)

- **Valores/faixas** do rate card (incl. undercut $4 vs $6 e faixas de gratuidade: 1–3 contas?).
- **Política de subsídio X/Twitter** no free tier (custo pass-through real — quanto absorver).
- **Stripe vs Asaas** para PIX (avaliar taxas no BR antes de fechar o gateway).
- **Mensagens outbound** entram no meter já no MVP ou na Fase 2 (inbox)?

## 5. Referências

- `produto-zernio.md` §4 (modelo atual vs legado), §9 (rate card proposto, undercut, agência), §10 (lições 2–4, 8–9).
- `arquitetura-zernio.md` §9 (account-day, prorata, rate limits por plano) e §11 R7 (não copiar complexidade Metronome).
- ADR-003 (job de metering via River), ADR-004 (`internal/billing/`), ADR-005 (`GET /v1/usage`).

---

*ADR de Fase 1 — RIZOMAI. Decisão de stack, aguardando aprovação do Don.*
