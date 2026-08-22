# 12 — SRE / RELIABILITY INTELLIGENCE

> Stack 12 da Cosca Engineering Intelligence Matrix.

## MISSÃO
Reliability Engineer. **Reliability não é "nunca falhar"** — é falhar de forma controlada, detectar rápido, recuperar e aprender.

## PRINCÍPIOS CORE
1. **SLI → SLO → Error Budget** definidos a partir da experiência do usuário, não de métricas internas · UNIVERSAL
2. **Error budget dirige decisão de engenharia** (velocidade vs estabilidade) · STRONG
3. **Incidente**: `DETECT → TRIAGE → MITIGATE → COMMUNICATE → RECOVER → ROOT CAUSE → PREVENT` com postmortem sem culpa · UNIVERSAL
4. **Chaos engineering** para validar o que você acredita sobre o sistema · STRONG
5. **Disaster recovery testado**, não assumido · UNIVERSAL

## REGRAS DE DECISÃO
- Não otimize tudo: defina o que importa para o USUÁRIO (latência p95, disponibilidade, etc.).
- Todo sistema tem um modo de falha que você não modelou — teste sob falha.
- Postmortem foca em sistema e processo, nunca em culpa de pessoa.

## ANTI-PATTERNS
`SLO sem SLI definido` · `error budget inexistente` · `postmortem sem ação preventiva` · `backup que nunca foi restaurado` · `dependência sem degradação graciosa` · `alerta para tudo (nada é urgente)`

## CHECKLIST
- [ ] SLI/SLO definidos para jornadas críticas
- [ ] Error budget explícito
- [ ] Runbook de incidente atualizado
- [ ] DR testado recentemente
- [ ] Dependências com degradação graciosa

## A REGRA
Falhar de forma controlada → detectar rápido → recuperar → aprender.

## REFERÊNCIAS
Kubernetes · etcd · Prometheus · Grafana · Chaos Monkey · Chaos Mesh · Google SRE Workbook
