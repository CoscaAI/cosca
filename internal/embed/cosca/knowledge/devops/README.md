# 11 — DEVOPS / CI/CD / INFRASTRUCTURE INTELLIGENCE

> Stack 11 da Cosca Engineering Intelligence Matrix.

## MISSÃO
Transformar software em sistemas reproduzíveis, automatizados e operáveis.

## PRINCÍPIOS CORE
1. **Infraestrutura como código** (Terraform/Helm) — versionada, revisável, idempotente · UNIVERSAL
2. **Pipeline completo**: `COMMIT → LINT → TEST → SECURITY → BUILD → ARTIFACT → DEPLOY → VERIFY → ROLLBACK` · UNIVERSAL
3. **Imutabilidade**: artefatos, não mutações; drift detection · STRONG
4. **Deployment strategies**: rolling/blue-green/canary com feature flags quando o risco justifica · STRONG
5. **Segredos via secret manager**, nunca em imagem/config · UNIVERSAL

## REGRAS DE DECISÃO
- Docker para consistência; Kubernetes quando escala/orquestração justifica (não por moda).
- GitOps (ArgoCD/Flux) quando o Git deve ser a fonte de verdade.
- Cada ambiente reproduzível do zero a partir do código + IaC.

## ANTI-PATTERNS
`imagem com segredo embutido` · `ambiente feito "na mão"` · `deploy sem rollback` · `IaC sem drift detection` · `CI quebrada ignorada` · `snowflake server` · `artefato não versionado`

## CHECKLIST
- [ ] Pipeline verde em CI (lint+test+security+build)
- [ ] Imagens sem segredos
- [ ] Deploy com rollback documentado
- [ ] Ambientes reproduzíveis via IaC
- [ ] Backup + disaster recovery testado

## A REGRA
Infrastructure must be: REPEATABLE → AUDITABLE → VERSIONED → RECOVERABLE.

## REFERÊNCIAS
Kubernetes · Terraform · Ansible · ArgoCD · Flux · Helm · Docker · Kaniko · wshobson/agents
