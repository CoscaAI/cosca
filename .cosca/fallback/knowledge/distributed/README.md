# 07 — DISTRIBUTED SYSTEMS INTELLIGENCE

> Stack 07 da Cosca Engineering Intelligence Matrix.

## MISSÃO
Raciocinar sobre sistemas distribuídos e suas falhas. **ASSUME FAILURE.**

## PRINCÍPIOS CORE
1. **O caminho feliz não é o sistema** — modele falhas primeiro · UNIVERSAL
2. **Consistência/Availability/Partition (CAP)** é trade-off, não slogan · UNIVERSAL
3. **Mensageria: at-least-once + idempotência** (a entrega pode duplicar) · UNIVERSAL
4. **Outbox/inbox** para integridade entre DB e mensagem · STRONG
5. **Backpressure e dead-letter queue** como cidadãos do design · STRONG

## MODELO DE FALHA (sempre perguntar)
O que acontece se: a rede cair? o processo morrer? a mensagem duplicar/atrasar/desaparecer? o servidor reiniciar? o clock divergir? o banco ficar indisponível?

## PADRÕES
`retry` · `idempotency` · `outbox` · `inbox` · `saga` · `circuit breaker` · `backpressure` · `dead letter queue` · `leader election` · `replication`

## ANTI-PATTERNS
`confiança cega no caminho feliz` · `mensagem processada 2x causando efeito duplicado` · `retry sem idempotência` · `saga sem compensação` · `consenso frágil (split brain)` · `timeout infinito` · `event sourcing sem replay pensado`

## CHECKLIST
- [ ] Idempotência em consumidores
- [ ] Failure modes documentados por componente
- [ ] Retry com backoff + limite
- [ ] DLQ configurada
- [ ] Testado sob partição/reinício (chaos)
- [ ] Clock drift considerado (não confiar em timestamp local como ordem)

## A REGRA
Em sistemas distribuídos: assuma falha. Nunca desenhe só o caminho feliz.

## REFERÊNCIAS
etcd · Redis · NATS · Kafka · RabbitMQ · Temporal/Cadence · Consul · Jepsen
