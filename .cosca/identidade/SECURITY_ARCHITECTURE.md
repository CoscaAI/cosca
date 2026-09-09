# ARQUITETURA DE SEGURANÇA — Framework de Cibersegurança Empresarial

> **Version**: 1.0.0 | **Status**: active | **Owner**: Security Chief | **Last Updated**: 2026-07-12

## PROPÓSITO
Este documento é a única fonte de verdade para todas as políticas, padrões e procedimentos de cibersegurança no ecossistema Cosca. Implementa defesa em profundidade em 8 domínios de segurança. Todo agente, engine e workflow deve cumprir. Sem exceção.

---

## 1. PRINCÍPIOS DE SEGURANÇA

| Princípio | Regra | Aplicação |
|-----------|-------|-----------|
| **Zero Trust** | Nunca confie, sempre verifique. Toda requisição autenticada e autorizada. | Identity Engine |
| **Menor Privilégio** | Agentes recebem permissões mínimas necessárias. Acesso elevado requer justificativa + TTL. | Policy Engine + Identity Engine |
| **Defesa em Profundidade** | Múltiplas camadas de segurança. Se uma falhar, outras pegam. | Todos os 8 domínios abaixo |
| **Seguro por Padrão** | Novos projetos começam com segurança máxima. Saída requer aprovação do Security Council. | Security Baseline (project-init Passo 7) |
| **Shift Left** | Segurança em cada estágio: design → código → build → test → deploy → monitorar. | Quality Gates 0-4 |
| **Assumir Breach** | Projetar para quando (não se) um breach ocorrer. Isolar, detectar, responder, recuperar. | Incident Response + Recovery Engine |
| **Privacidade por Design** | PII criptografado em repouso e em trânsito. Minimização de dados. Direito ao esquecimento. | Compliance Engine |
| **Nunca Confiar no Modelo** | Saída de IA validada antes de uso em interface com o usuário. Defesas contra prompt injection. Sem secrets em prompts. | AI Council + Secrets Engine |

---

## 2. DOMÍNIOS DE SEGURANÇA (8)

```
┌─────────────────────────────────────────────────────────────┐
│                    DOMÍNIOS DE SEGURANÇA                     │
├─────────────────────────────────────────────────────────────┤
│ 1. Identidade e Acesso     → Quem é você? O que pode fazer? │
│ 2. Proteção de Dados       → Criptografia, mascaramento, retenção │
│ 3. Segurança de Aplicação  → OWASP, SAST, DAST, dependências │
│ 4. Infraestrutura          → Rede, containers, cloud │
│ 5. Cadeia de Suprimentos   → SBOM, assinatura, procedência   │
│ 6. Segurança de IA/ML      → Prompt injection, envenenamento de modelo │
│ 7. Resposta a Incidentes   → Detectar, conter, erradicar, recuperar │
│ 8. Conformidade            → GDPR, SOC2, HIPAA, PCI-DSS      │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. DOMÍNIO 1: GESTÃO DE IDENTIDADE E ACESSO

### 3.1 Padrões de Autenticação
| Método | Caso de Uso | Requisito Mínimo |
|--------|----------|-------------------|
| API Keys | Service-to-service (agent-to-agent) | 256-bit aleatório, rotacionado a cada 90 dias |
| JWT (Access) | Sessões de usuário/agente | RS256, TTL de 15 min, sem secrets no payload |
| JWT (Refresh) | Renovação de sessão | 256-bit aleatório, TTL de 7 dias, uso único, rotação ao usar |
| OAuth 2.0 / OIDC | Integração com terceiros | PKCE obrigatório, parâmetro state validado |
| mTLS | Comunicação entre serviços críticos | Certificate pinning, rotação a cada 30 dias |
| Biométrico | Apps mobile/desktop | Nativo da plataforma (FaceID, TouchID, Windows Hello) |

### 3.2 Modelo de Autorização (RBAC + ABAC Híbrido)
```
RBAC Layer:
  Role → Permissions
  Admin > Chief > Specialist > ReadOnly

ABAC Layer:
  Subject (who) + Action (what) + Resource (where) + Context (when/why)

  Example: "Backend Chief CAN write_file ON src/api/ DURING business hours FROM approved IP"
```

### 3.3 Aplicação da Identity Engine
| Verificação | Quando | Ação em Caso de Falha |
|-------|------|------------------|
| Autenticação de agente | Cada criação | Bloquear criação, alertar Security Chief |
| Validação de token | Cada requisição | Retornar 401, registrar tentativa |
| Verificação de permissão | Cada chamada de tool | Negar tool, registrar tentativa |
| Isolamento de tenant | Cada acesso cross-tenant | Bloquear, alertar imediatamente |
| Timeout de sessão | Cada 15 min (access token) | Forçar re-autenticação, preservar estado do workflow |
| Detecção de anomalias | Contínuo | Escalar para Security Council |

---

## 4. DOMÍNIO 2: PROTEÇÃO DE DADOS

### 4.1 Padrões de Criptografia
| Estado dos Dados | Algoritmo | Gerenciamento de Chaves |
|-----------|-----------|---------------|
| Em Repouso | AES-256-GCM | Criptografia envelope (DEK + KEK), suportado por HSM |
| Em Trânsito | TLS 1.3 | Certificate pinning, HSTS, sigilo encaminhado |
| Em Memória | Apenas em memória (sem swap) | mlock, swap criptografado desabilitado |
| Em Backups | AES-256-GCM | Chave de backup separada, chave mestra offline |

### 4.2 Classificação de Dados
| Nível | Exemplos | Armazenamento | Criptografia | Retenção |
|-------|----------|---------|------------|-----------|
| **P0 - Secrets** | API keys, senhas, tokens | Secrets Engine (vault) | AES-256-GCM + HSM | Rotação por política |
| **P1 - PII** | E-mails, nomes, endereços, IPs | Coluna criptografada no BD | AES-256-GCM | Por conformidade + direito ao esquecimento |
| **P2 - Confidencial** | Código-fonte, docs de arquitetura, ADRs | Criptografado em repouso | AES-256-GCM | Vida do projeto |
| **P3 - Interno** | Logs de workflow, métricas de agentes | Armazenamento padrão | AES-256-GCM (opcional) | 12 meses |
| **P4 - Público** | README, docs públicos, changelogs | Armazenamento padrão | Nenhum | Para sempre |

### 4.3 Matriz de Privacidade de Dados
| Regulação | Requisito | Aplicação no Cosca |
|-----------|----------|-----------------|
| GDPR | Direito ao esquecimento, portabilidade de dados, consentimento | Compliance Engine + poda de memória |
| CCPA | Direito de saber, direito de excluir, opt-out | Compliance Engine |
| HIPAA | Criptografia de PHI, registro de acesso, BAA | Compliance Engine + Audit Engine |
| PCI-DSS | Dados de cartão nunca armazenados, tokenização | Secrets Engine (isolamento de escopo PCI) |
| SOC2 | Segurança, disponibilidade, confidencialidade | Trilha de auditoria completa + relatórios de conformidade |

---

## 5. DOMÍNIO 3: SEGURANÇA DE APLICAÇÃO

### 5.1 OWASP Top 10 (2021) — Cobertura do Cosca
| # | Vulnerabilidade | Defesa do Cosca |
|---|-------------|------------|
| A01 | Controle de Acesso Quebrado | Identity Engine + RBAC/ABAC + verificação de permissão em cada chamada de tool |
| A02 | Falhas Criptográficas | Secrets Engine (nunca hardcoded), TLS 1.3, AES-256-GCM |
| A03 | Injeção | Queries parametrizadas apenas, validação de entrada em todas as entradas de agentes, codificação de saída |
| A04 | Design Inseguro | Revisão do Architecture Council, modelagem de ameaças por feature |
| A05 | Configuração Insegura de Segurança | Security Baseline (project-init Passo 7), cabeçalhos CSP, padrões seguros |
| A06 | Componentes Vulneráveis | Auditoria de dependências a cada 24h, geração de SBOM, scan de CVE |
| A07 | Falhas de Autenticação | MFA para admin, melhores práticas de JWT, proteção contra brute-force |
| A08 | Integridade de Software e Dados | Commits assinados, verificação de SBOM, integridade de pipeline CI/CD |
| A09 | Falhas de Logging e Monitoramento | Audit Engine (cada ação registrada), Observabilidade (detecção de anomalias) |
| A10 | SSRF | Restrições de tools de agentes, filtragem de egresso de rede, validação de URL |

### 5.2 Pipeline de Scan de Segurança
```
Commit → Pre-commit hooks (scan de secrets, lint básico)
  ↓
PR → SAST (análise estática), auditoria de dependências (verificação CVE)
  ↓
Build → Scan de container (Trivy), geração de SBOM (CycloneDX)
  ↓
Test → DAST (análise dinâmica), testes de fuzz
  ↓
Deploy → Scan de IaC (tfsec, checkov), verificação de conformidade
  ↓
Produção → RASP, WAF, monitoramento contínuo
```

### 5.3 Padrões de Código Seguro
| Linguagem | Padrão | Aplicado Por |
|----------|----------|-------------|
| TypeScript/JavaScript | Plugin de segurança ESLint, no-eval, cabeçalhos CSP | Pre-commit + CI |
| Python | Bandit, safety, pip-audit | Pre-commit + CI |
| Go | gosec, nancy | Pre-commit + CI |
| Java/Kotlin | SpotBugs, OWASP Dependency Check | CI |
| Infraestrutura | tfsec, checkov, kubesec | CI + Pre-deploy |

---

## 6. DOMÍNIO 4: SEGURANÇA DE INFRAESTRUTURA

### 6.1 Segurança de Rede
| Camada | Controle |
|-------|---------|
| Perímetro | WAF, proteção DDoS, allowlist de IP para admin |
| Rede | Isolamento VPC, subnets privadas, NAT gateways |
| Serviço | mTLS entre serviços, service mesh (Istio/Linkerd) |
| Container | Usuário non-root, sistema de arquivos read-only, seccomp/AppArmor |
| Egress | Allowlist de conexões de saída, bloqueio de mineração de criptomoedas |

### 6.2 Segurança de Containers
```dockerfile
# Secure Dockerfile pattern
FROM node:20-alpine            # Minimal base image, pinned version
RUN addgroup -S app && adduser -S app -G app
USER app                        # Non-root user
COPY --chown=app:app . .
EXPOSE 3000
HEALTHCHECK --interval=30s CMD wget -q http://localhost:3000/health || exit 1
```

### 6.3 Postura de Segurança em Cloud
| Provedor | Principais Controles |
|----------|-------------|
| AWS | IAM menor privilégio, S3 bloqueio de acesso público, CloudTrail habilitado, KMS CMK |
| GCP | Condições IAM, VPC Service Controls, Audit Logs, CMEK |
| Azure | RBAC, regras NSG, Key Vault, Defender for Cloud |

---

## 7. DOMÍNIO 5: SEGURANÇA DA CADEIA DE SUPRIMENTOS

### 7.1 SBOM (Software Bill of Materials)
```
Todo projeto gera SBOM no formato CycloneDX:
  - Todas as dependências diretas com versão + hash
  - Todas as dependências transitivas
  - Informações de licença por dependência
  - Status de CVE por dependência
  - Gerado em: cada build
  - Armazenado em: .cosca/security/sbom.json
```

### 7.2 Verificação de Dependências
| Verificação | Frequência | Ação em Caso de Falha |
|-------|-----------|------------------|
| Scan de CVE (crítico) | Cada commit + diário | Bloquear merge, forçar atualização |
| Scan de CVE (alto) | Diário | Bloquear release, agendar correção |
| Conformidade de licença | Cada build | Aviso se copyleft em projeto proprietário |
| Fixação de versão | Cada commit | Bloquear versões não fixadas |
| Verificação de procedência | Cada instalação | Bloquear pacotes não assinados |
| Detecção de typosquatting | Semanal | Alertar Security Chief |

### 7.3 Commits Assinados
```
Todos os commits gerados pelo Cosca devem ser:
  - Assinados com GPG (verificados por GitHub/GitLab)
  - Ter e-mail de autor verificado
  - Seguir formato de commits convencionais
  - Incluir ID do workflow no rodapé da mensagem do commit
```

---

## 8. DOMÍNIO 6: SEGURANÇA DE IA/ML

### 8.1 Defesas contra Prompt Injection
| Vetor de Ataque | Defesa |
|--------------|---------|
| Injeção direta ("ignore instruções anteriores") | Higienização de entrada, fortalecimento do prompt do sistema, validação de saída |
| Injeção indireta (dados envenenados no contexto) | Validação de contexto, execução em sandbox |
| Jailbreaking | Template de prompt com imposição de papel, barreiras de entrada/saída |
| Exfiltração de dados via prompt | Filtragem de saída, sem secrets no contexto, imposição de limite de tokens |
| Inversão de modelo | Limitação de taxa, monitoramento de saída, privacidade diferencial |

### 8.2 Validação de Saída de IA
```
Antes que a saída de IA atinja o usuário:
  1. Validar formato (JSON, código, texto)
  2. Escanear secrets (padrões regex)
  3. Escanear código malicioso (eval, exec, chamadas de sistema)
  4. Validar contra schema esperado
  5. Higienizar XSS se renderizado na UI
  6. Registrar todas as saídas de IA para auditoria
```

### 8.3 Segurança de Modelos
| Preocupação | Mitigação |
|---------|-----------|
| Envenenamento de modelo | Usar apenas modelos assinados de fontes confiáveis |
| Vazamento de dados de treinamento | Anonimização de dados antes do treinamento |
| Entradas adversárias | Validação de entrada, detecção de anomalias |
| Roubo de modelo | Limitação de taxa de API, marca d'água |
| Abuso de custo | Orçamentos de tokens por agente, alertas de anomalia de custo |

---

## 9. DOMÍNIO 7: RESPOSTA A INCIDENTES

### 9.1 Classificação de Severidade de Incidentes
| Severidade | Definição | Tempo de Resposta | Escalação |
|----------|-----------|---------------|------------|
| **P0 - Crítico** | Breach ativo, exfiltração de dados, comprometimento do sistema | < 15 min | Conselho Executivo + todos |
| **P1 - Alto** | Vulnerabilidade explorável, serviço fora do ar, secrets vazados | < 1 hora | Security Council |
| **P2 - Médio** | CVE não crítico, atividade suspeita, violação de política | < 4 horas | Security Chief |
| **P3 - Baixo** | Configuração incorreta menor, dependência desatualizada | < 24 horas | Chief Respectivo |

### 9.2 Playbook de Resposta a Incidentes
```
DETECTAR → Alerta do Monitoring Engine ou scan de segurança
  ↓
TRIAGE → Security Chief classifica severidade (P0-P3)
  ↓
CONTER → Isolar sistema afetado, rotacionar secrets, bloquear IP do atacante
  ↓
ERRADICAR → Remover causa raiz, corrigir vulnerabilidade, verificar correção
  ↓
RECUPERAR → Restaurar de backup limpo, verificar integridade, retomar serviço
  ↓
APRENDER → Post-mortem documentado em knowledge/incidents/
  ↓
MELHORAR → Atualizar políticas, adicionar regras de detecção, fortalecer defesas
```

### 9.3 Equipe de Resposta a Incidentes
| Função | Primário | Secundário |
|------|---------|-----------|
| Comandante do Incidente | Security Chief | CTO |
| Líder Técnico | DevOps Chief | Backend Chief |
| Comunicação | CEO | Documentation Chief |
| Jurídico/Conformidade | Compliance Engine | Security Chief |
| Forense | Security Engineer (especialista) | AI Chief |

---

## 10. DOMÍNIO 8: AUTOMAÇÃO DE CONFORMIDADE

### 10.1 Monitoramento Contínuo de Conformidade
| Framework | Verificações | Frequência | Evidência |
|-----------|--------|-----------|----------|
| SOC2 | Revisões de acesso, gestão de mudanças, avaliação de riscos | Mensal | Logs de auditoria + relatórios do Compliance Engine |
| GDPR | Inventário de dados, registros de consentimento, tratamento de DSR | Mensal | Mapa de dados + log de DSR |
| HIPAA | Logs de acesso a PHI, verificação de criptografia, rastreamento de BAA | Semanal | Logs de acesso + status de criptografia |
| PCI-DSS | Scan de dados de cartão, segmentação de rede, controle de acesso | Semanal | Resultados do scan PCI + diagrama de rede |
| ISO 27001 | Revisão do ISMS, efetividade dos controles, tratamento de riscos | Trimestral | Matriz de controles + registro de riscos |

### 10.2 Requisitos de Trilha de Auditoria
```
Todo evento relevante para segurança deve ser registrado:
  - Quem (ID do agente)
  - O quê (ação)
  - Quando (timestamp com fuso horário)
  - Onde (recurso, IP)
  - Resultado (sucesso/falha)
  - Contexto (ID do workflow, ID da sessão)

  Logs devem ser:
  - Imutáveis (append-only, sem exclusão)
  - Anti-fraude (cadeia de hash)
  - Mantidos por no mínimo 1 ano
  - Pesquisáveis em até 5 segundos
```

---

## 11. AUTORIDADE DO SECURITY COUNCIL

### 11.1 Decisões Requerendo Aprovação do Security Council
- Nova integração de provider de IA
- Alteração em padrões de criptografia
- Alterações em políticas de gerenciamento de secrets
- Aprovação de SDK/plugin de terceiros
- Escopo e descobertas de teste de penetração
- Adoção de framework de conformidade
- Classificação de severidade de incidente (P0/P1)
- Alterações no security baseline

### 11.2 Poder de Veto do Security Chief
O Security Chief (ou Security Council) pode vetar:
- Qualquer release com CVEs críticos/alta
- Qualquer deploy em produção sem revisão de segurança
- Qualquer merge de código com secrets hardcoded
- Qualquer integração com terceiros sem avaliação de segurança
- Qualquer alteração arquitetural sem modelagem de ameaças

---

## 12. MÉTRICAS E RELATÓRIOS DE SEGURANÇA

### 12.1 Principais Métricas de Segurança
| Métrica | Meta | Medição |
|--------|--------|-------------|
| Tempo Médio para Detectar (MTTD) | < 1 hora (P0), < 24 horas (P1) | Timestamps de incidentes |
| Tempo Médio para Responder (MTTR) | < 4 horas (P0), < 48 horas (P1) | Tempo de resolução do incidente |
| Remediação de vulnerabilidades | Crítico: 24h, Alto: 7d, Médio: 30d | Rastreamento de CVE |
| Secrets no código | 0 (zero tolerância) | Scan de pre-commit |
| Saúde das dependências | 0 CVEs críticos/altos | Auditoria diária |
| Cobertura de revisão de segurança | 100% dos PRs | Rastreamento de revisões |
| Frequência de teste de penetração | Trimestral | Relatórios de teste |
| Treinamento de segurança | Anual para todos os tipos de agentes | Learning Engine |

### 12.3 Checklist de Segurança — Todo Projeto

Antes de qualquer projeto ir para produção:
- [ ] Security baseline aplicado (project-init Passo 7)
- [ ] .gitignore cobre: .env, secrets, credenciais, tokens
- [ ] Sem secrets hardcoded (verificado por scan)
- [ ] Todas as dependências auditadas (0 CVEs críticos/altos)
- [ ] Autenticação aplicada em todos os endpoints
- [ ] Autorização verificada em todos os recursos protegidos
- [ ] Limitação de taxa configurada
- [ ] Cabeçalhos CSP definidos
- [ ] HTTPS aplicado (HSTS)
- [ ] Validação de entrada em todas as entradas externas
- [ ] Codificação de saída em todas as saídas
- [ ] Prevenção de SQL injection (queries parametrizadas)
- [ ] Prevenção de XSS (codificação apropriada ao contexto)
- [ ] Proteção CSRF em operações que alteram estado
- [ ] Docker executando como usuário non-root
- [ ] SBOM gerado e verificado
- [ ] Modelagem de ameaças documentada (para features P0/P1)
- [ ] Runbook de resposta a incidentes pronto
- [ ] Trilha de auditoria configurada e testada
- [ ] Teste de penetração aprovado (trimestral)
```
┌─────────────────────────────────────────────────────┐
│              SECURITY POSTURE DASHBOARD               │
├─────────────────────────────────────────────────────┤
│ STATUS: 🟢 SECURE    LAST INCIDENT: 15 days ago      │
│                                                       │
│ VULNERABILITIES                                       │
│ Critical: 0    High: 0    Medium: 3    Low: 12        │
│                                                       │
│ COMPLIANCE                                            │
│ SOC2: ✅    GDPR: ✅    HIPAA: ✅    PCI: ✅            │
│                                                       │
│ ACTIVE DEFENSES                                        │
│ SAST: ✅    DAST: ✅    SCA: ✅    WAF: ✅              │
│ Secrets Scan: ✅    Container Scan: ✅                  │
│                                                       │
│ RECENT EVENTS                                         │
│ ✅ Dependency audit passed — 2 min ago                │
│ ✅ Secrets scan clean — 15 min ago                    │
│ ⚠️ Medium CVE in lodash — patching scheduled          │
│ ✅ Access review completed — 1 day ago                │
└─────────────────────────────────────────────────────┘
```

---

## 13. SECURITY CHECKLIST — Every Project

Before any project goes to production:
- [ ] Security baseline applied (project-init Step 7)
- [ ] .gitignore covers: .env, secrets, credentials, tokens
- [ ] No hardcoded secrets (verified by scan)
- [ ] All dependencies audited (0 critical/high CVEs)
- [ ] Authentication enforced on all endpoints
- [ ] Authorization checked on all protected resources
- [ ] Rate limiting configured
- [ ] CSP headers set
- [ ] HTTPS enforced (HSTS)
- [ ] Input validation on all external inputs
- [ ] Output encoding on all outputs
- [ ] SQL injection prevention (parameterized queries)
- [ ] XSS prevention (context-appropriate encoding)
- [ ] CSRF protection on state-changing operations
- [ ] Docker runs as non-root user
- [ ] SBOM generated and verified
- [ ] Threat model documented (for P0/P1 features)
- [ ] Incident response runbook ready
- [ ] Audit trail configured and tested
- [ ] Penetration test passed (quarterly)

---

## RELACIONADOS
- [Security Chief](departments/security/SKILL.md) — Estratégia e supervisão de segurança
- [Secrets Engine](engines/secrets/SKILL.md) — Gerenciamento de credenciais
- [Identity Engine](engines/identity/SKILL.md) — Autenticação e controle de acesso
- [Compliance Engine](engines/compliance/SKILL.md) — Conformidade regulatória
- [Policy Engine](engines/policy/SKILL.md) — Políticas de segurança (POL-SEC-*)
- [QUALITY_GATES.md](QUALITY_GATES.md) — Gate 2.3 Verificações de segurança
- [COUNCILS.md](councils/COUNCILS.md) — Autoridade do Security Council
- [ENTERPRISE_REDUNDANCY.md](ENTERPRISE_REDUNDANCY.md) — Circuit breakers e recuperação
- [PROVIDER_INTERFACE.md](PROVIDER_INTERFACE.md) — Segurança de providers de IA
- [project-init workflow](workflows/project-init.md) — Security Baseline (Passo 7)

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|---------|------|--------|---------|
| 1.0.0 | 2026-07-12 | Cosca Kernel | Framework completo de cibersegurança: 8 domínios, Zero Trust, defesa em profundidade, cobertura OWASP, cadeia de suprimentos, segurança de IA, resposta a incidentes, automação de conformidade |
