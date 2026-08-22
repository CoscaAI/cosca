# Playbook: Auditoria de Conformidade GDPR

> **Versão**: 1.0.0 | **Workflow**: compliance-audit | **Duração típica**: 3-5 dias

## Cenário
Realizar auditoria de conformidade GDPR para uma plataforma SaaS que processa dados pessoais de usuários europeus.

## Pré-requisitos
- [ ] Mapeamento de dados (data inventory) atualizado
- [ ] Acesso a todos os sistemas que processam dados pessoais
- [ ] Documentação de processos de segurança
- [ ] DPAs de fornecedores (se aplicável)

## Passo a Passo

### 1. Escopo (Dia 1, 2h)
Definir quais sistemas e dados estão no escopo:
```yaml
escopo:
  sistemas:
    - auth-service (cadastro, login, perfis)
    - billing-service (pagamentos, faturas)
    - analytics-service (eventos, tracking)
    - notification-service (emails, push)
  dados_pessoais:
    - nome, email, telefone
    - endereço IP, user-agent
    - histórico de compras
    - preferências de comunicação
  exclusões:
    - dados anonimizados (> 90 dias)
    - logs de infraestrutura (sem PII)
```

### 2. Coleta de Evidências (Dia 1-2, 6h)
Para cada controle, coletar evidências:

| Controle | Evidência | Onde buscar |
|----------|-----------|-------------|
| Consentimento | Registro de consentimento | Tabela user_consents |
| Exclusão | Procedimento de exclusão | docs/gdpr/deletion.md |
| Portabilidade | Export de dados | API /v1/users/:id/export |
| Criptografia | Config TLS/SSL | TerraState, LB config |
| Notificação | Breach notification plan | docs/security/breach.md |
| DPA | Contratos com fornecedores | docs/compliance/dpas/ |

### 3. Gap Analysis (Dia 3, 4h)
```yaml
gaps_encontrados:
  - controle: exclusão de dados
    status: partial
    problema: "Soft delete implementado, mas hard delete não automatizado"
    risco: medium
    remediacao: "Agendar script de hard delete semanal (prazo: 30 dias)"
    
  - controle: portabilidade
    status: fail
    problema: "API de export não inclui histórico de eventos"
    risco: high
    remediacao: "Adicionar eventos à export API (prazo: 15 dias)"
    
  - controle: DPA com provedor de email
    status: fail
    problema: "SendGrid DPA não assinado"
    risco: critical
    remediacao: "Solicitar DPA ao SendGrid (prazo: 7 dias)"
```

### 4. Relatório Final (Dia 4, 3h)
```markdown
# Relatório de Auditoria GDPR
**Data**: 2026-07-23
**Auditor**: Compliance Chief
**Resultado**: 82/100 (Condicional — 3 gaps encontrados)

## Score por Domínio
- Consentimento: 95% ✅
- Acesso e Retificação: 90% ✅
- Portabilidade: 60% ❌
- Exclusão: 70% ⚠️
- Segurança: 88% ✅
- Notificação de Violação: 95% ✅
- DPA com Fornecedores: 50% ❌

## Próximos Passos
1. Assinar DPA com SendGrid (7 dias)
2. Implementar hard delete automático (30 dias)
3. Expandir export API com eventos (15 dias)
4. Re-auditar em 60 dias
```

### 5. Aprovação (Dia 5, 1h)
- [ ] Relatório revisado pelo Compliance Chief
- [ ] Gaps aceitos pelo CTO (com prazos)
- [ ] Plano de remediação aprovado
- [ ] Relatório arquivado em memory/architecture

## Templates Úteis

### Template de DPIA (Data Privacy Impact Assessment)
```markdown
## DPIA: [Feature Name]
**Data**: YYYY-MM-DD
**DPIA Owner**: [Name]

### 1. Descreva o processamento
[O que será feito com os dados?]

### 2. Necessidade e proporcionalidade
[Por que é necessário? Existe alternativa menos invasiva?]

### 3. Riscos cosca direitos dos titulares
[Listar riscos identificados]

### 4. Mitigações
[Listar medidas de proteção]

### 5. Aprovação
[ ] Aprovado  [ ] Aprovado com condições  [ ] Rejeitado
```

## Related
- [Compliance Audit workflow](../../../workflows/compliance-audit.md)
- [Compliance Chief](../../../engines/tools/SKILL.md)
- [Compliance Validation skill](../../../skills/security/COMPLIANCE_VALIDATION.md)
