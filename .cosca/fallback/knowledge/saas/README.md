# 35 — ENTERPRISE SaaS / B2B

> Stack 35 da Cosca Engineering Intelligence Matrix.

## MISSÃO
Especialista em SaaS enterprise/B2B: multi-tenancy, billing, RBAC, auditoria e onboarding — **sofisticação sem complexidade desnecessária**.

## PRINCÍPIOS CORE
1. **Isolamento de tenant** — escolher modelo por risco/dados: row-level (RLS) > schema > database; isolar de verdade, nunca só na UI · UNIVERSAL
2. **RBAC/ABAC explícito** — permissão por papel/atributo em toda ação · UNIVERSAL
3. **Billing/metering** — medir uso real; planos e limites claros; dunning (cobrança/retry) planejado · STRONG
4. **Auditoria** — ações administrativas e mudanças sensíveis logadas · UNIVERSAL
5. **Onboarding ativação** — empty states → primeira vitória → valor rápido · STRONG
6. **Limites por tenant** (API, storage, recursos) — um tenant nunca derruba o outro · STRONG
7. **Segredos isolados por tenant** — nenhuma chave/provedor compartilhado vazando · UNIVERSAL

## REGRAS DE DECISÃO
- **Multi-tenant**: começar com row-level isolation + tenant_id em tudo; schema/db isolation só quando o modelo de ameaça justifica.
- **Billing**: planos + metering + quotas; reconciliar com o provedor de pagamento (cross-ref fintech).
- **Feature flags**: liberar/limitar por tenant/plano.

## ANTI-PATTERNS
`tenant_id esquecido numa query (vazamento de dados)` · `isolamento só na UI` · `segredo compartilhado entre tenants` · `billing sem metering` · `sem auditoria de ações admin` · `onboarding sem empty state` · `um tenant estourando recursos de todos` · `planos hardcoded`

## CHECKLIST (quality gate)
- [ ] Tenant_id em toda query (testes de isolamento)
- [ ] RBAC/ABAC em toda ação
- [ ] Segredos por tenant/isolados
- [ ] Billing com metering + quotas + dunning
- [ ] Auditoria de ações sensíveis
- [ ] Limites por tenant (nunca um derruba todos)
- [ ] Onboarding com empty states de ativação

## A REGRA
Um tenant nunca deve ser capaz de ver, afetar ou derrubar outro.

## REFERÊNCIAS
supabase · pocketbase · wasp · cal.com · novu · BoxyHQ/saas-starter-kit · saaspegasus · appwrite · directus · strapi
