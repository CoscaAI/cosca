# 13 — OBSERVABILITY / TELEMETRY INTELLIGENCE

> Stack 13 da Cosca Engineering Intelligence Matrix.

## MISSÃO
Construir sistemas observáveis. **Observability não é coletar dados** — é transformar comportamento do sistema em informação acionável.

## PRINCÍPIOS CORE
1. **Golden signals**: latency, traffic, errors, saturation · UNIVERSAL
2. **3 pilares correlacionados**: logs + métricas + traces (com correlation/trace id) · UNIVERSAL
3. **Logs como eventos estruturados**: timestamp, severity, service, operation, trace id, metadata — **nunca** "something went wrong" · UNIVERSAL
4. **Alertas com WHO/WHAT/WHY/URGENCY/ACTION** — alerta que não diz o que fazer é ruído · UNIVERSAL
5. **Context propagation** (OpenTelemetry) para rastrear requisição através dos serviços · STRONG

## REGRAS DE DECISÃO
- Instrumentar desde o início (não depois que o incidente acontece).
- Cada alerta precisa sobreviver ao teste: "quem recebe, o que faz, com que urgência?"
- Nunca logar secrets/dados sensíveis.

## ANTI-PATTERNS
`"something went wrong"` · `alerta para tudo` · `logs sem trace id` · `métrica sem unidade/definição` · `painel bonito que ninguém lê` · `dados sensíveis em log` · `observabilidade removida por "performance"`

## CHECKLIST
- [ ] Golden signals visíveis (dashboard)
- [ ] Traces correlacionados (request → services → db/queue)
- [ ] Logs estruturados com trace/correlation id
- [ ] Alertas com ação clara + sem duplicação
- [ ] Nenhum segredo em logs

## A REGRA
Observability é transformar comportamento do sistema em informação acionável.

## REFERÊNCIAS
OpenTelemetry · Prometheus · Grafana · Loki · Jaeger · Sentry
