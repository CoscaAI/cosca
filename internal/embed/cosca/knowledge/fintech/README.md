# 34 — FINTECH / PAYMENTS / LEDGER

> Stack 34 da Cosca Engineering Intelligence Matrix.

## MISSÃO
Especialista em sistemas financeiros: pagamentos, ledger, reconciliação e antifraude. **Dinheiro é registro, não número.**

## PRINCÍPIOS CORE
1. **Nunca usar float para dinheiro** — inteiros em centavos (ou Decimal) sempre · UNIVERSAL
2. **Double-entry ledger** — toda transação tem débito e crédito; o sistema nunca perde ou cria dinheiro · UNIVERSAL
3. **Idempotência em tudo** (pagamento, estorno, webhook) — 3 cliques ≠ 3 cobranças · UNIVERSAL
4. **Reconciliação** — conciliar com provedor/banco periodicamente; diferença é bug até provado contrário · STRONG
5. **Trilha de auditoria imutável** — toda mutação registrada (quem, o quê, quando) · UNIVERSAL
6. **Fraude é defesa, não detecção tardia** — risk scoring, rate limits, sinais do provedor, revisão manual · STRONG
7. **Nunca armazenar PAN** — tokenização + provedor especializado (PCI) · UNIVERSAL
8. **Ledger concorrente**: saldo com lock/versão — nunca leitura-escrita sem controle · STRONG

## REGRAS DE DECISÃO
- **Money movement**: direto (provedor) vs orquestrador (multi-provedor) — pelo risco/mercado/escopo.
- **Status de pagamento**: máquina de estados explícita (authorized → captured → refunded; failed paths).
- **Webhook de pagamento = input não confiável**: assinatura + idempotência + replay.

## ANTI-PATTERNS
`float para dinheiro` · `single-entry (só débito)` · `sem reconciliação` · `idempotência ausente (cobra 2x)` · `fraude só por padrão conhecido` · `PAN armazenado` · `saldo sem lock (race)` · `ledger sem auditoria` · `estorno sem workflow reverso`

## CHECKLIST (quality gate)
- [ ] Dinheiro em centavos/Decimal (nunca float)
- [ ] Double-entry em toda movimentação
- [ ] Idempotência em POSTs financeiros
- [ ] Reconciliação automática programada
- [ ] Auditoria imutável
- [ ] Fraude: risk scoring + rate limits + revisão
- [ ] PAN tokenizado (nunca bruto)
- [ ] Concorrência de saldo controlada
- [ ] Conformidade (PCI/prazos) considerada

## A REGRA
Dinheiro é registro. O sistema deve **nunca perder, nunca criar, e sempre explicar** cada centavo.

## REFERÊNCIAS
stripe (node/go) · killbill · moov-io (ach, base) · apache/fineract · openMF/mifosx · actualbudget · ghostfolio · firefly-iii · medusa/saleor/vendure (payments em ecommerce)
