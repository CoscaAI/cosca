# GATES DE QUALIDADE — Definicoes Canonicas

> **Versao**: 1.1.0 | **Status**: active | **Dono**: QA Chief | **Ultima Atualizacao**: 2026-07-30

## PROPOSITO
Fonte unica de verdade para todos os gates de qualidade, metricas e thresholds. Referenciado por KERNEL.md, Review Engine, Quality Engine e todos os chefes de departamento. Nenhum outro arquivo deve duplicar definicoes de gate.

---

## Arquitetura de Gates

```
Gate 0:   Pre-Trabalho        →   A requisicao de trabalho e valida?
Gate 0.5: Contrafactual       →   E se a decisao oposta tivesse sido tomada?
Gate 1:   Pre-Implementacao   →   O plano e solido?
Gate 2:   Pos-Implementacao   →   O codigo atende os padroes?
Gate 3:   Pre-Release         →   O release e seguro?
Gate 4:   Pos-Release         →   A producao esta saudavel?
```

---

## Gate 0 — Pre-Trabalho (Validacao de Requisicao)

Acionado pelo Kernel antes que qualquer trabalho comecce.

| Verificacao | Threshold | Severidade | Automatizado |
|-------------|-----------|------------|--------------|
| Classificacao da requisicao | Deve ser um de: feature, bug, refactor, architecture, docs, deploy, research, review | Erro | Sim |
| Escopo definido | Pelo menos 1 frase descrevendo o que precisa ser feito | Erro | Nao |
| Departamentos afetados identificados | Pelo menos 1 departamento mapeado | Aviso | Sim |
| Nenhum workflow ativo conflitante | Nenhum outro workflow tocando nos mesmos arquivos/modulos | Aviso | Nao |

---

## Gate 0.5 — Revisao de Decisao Contrafactual (Gate Pre-Decisao)

> **Dono**: cosca-critic | **Obrigatorio para**: Decisoes estrategicas P0/P1 | **Opcional para**: P2/P3
>
> **Workflow completo**: [workflows/contrafactual-gate.md](workflows/contrafactual-gate.md)

**Acionado pelo Kernel** apos Resolucao de Capacidade (Passo 6) e antes de Planejamento e Geracao de DAG (Passo 7), quando uma decisao estrategica foi identificada mas ainda nao commitada. O Kernel invoca cosca-critic para executar o gate.

### Proposito

Antes de cristalizar qualquer decisao estrategica, o sistema deve pausar e perguntar: **"E se a decisao oposta tivesse sido tomada?"** — forcando comparacao explicita de alternativas para prevenir vies de confirmacao. Este gate implementa pensamento contrafactual como um passo mandatorio no pipeline de decisao.

### Verificacoes

| Verificacao | Threshold | Severidade | Automatizado |
|-------------|-----------|------------|--------------|
| Decisao oposta (~A) formulada | ~A deve ser uma alternativa real e viavel (nao espantalho) | Erro | Nao |
| Evidencias para ~A coletadas | Pelo menos 2 fontes de evidencia apoiando ~A | Erro | Nao |
| 5 dimensoes comparadas | Risco, Custo, Tempo, Ganho de Conhecimento, Reversibilidade pontuados para ambos os lados | Erro | Nao |
| Premissas auditadas | Pelo menos 3 premissas explicitas identificadas e desafiadas | Aviso | Nao |
| Cenarios extremos testados | 10x escala, 100x escala, falha catastrofica, projecao 6 meses | Aviso | Nao |
| Racional da decisao documentado | Racional explicito por que A foi escolhido sobre ~A | Erro | Nao |
| Gate executado ANTES da decisao | Timestamp prova que o gate rodou pre-decisao, nao post-hoc | Erro | Sim |
| Confianca das evidencias ponderada | Forca da evidencia classificada conforme CONFIDENCE_MODEL.md | Aviso | Nao |
| Riscos identificados com mitigacoes | Pelo menos 1 risco documentado por lado da decisao | Aviso | Nao |

### Regras de Resultado

| Condicao | Resultado | Acao |
|----------|-----------|------|
| Evidencias para ~A sao irrelevantes ou mais fracas que A | **prosseguir** | Decisao confirmada, racional documentado |
| ~A revela riscos significativos nao examinados | **escalar** | Revisao mais profunda necessaria antes de prosseguir |
| ~A demonstra resultados objetivamente melhores | **rejeitar** | Sinalizar para revisao do Don — NAO prosseguir |
| Premissas criticas tem baixa confianca | **escalar** | Validar premissas primeiro |
| Evidencias insuficientes para ambos os lados | **escalar** | Decisao e prematura |

### Formato de Saida

```yaml
contrafactual_gate:
  decision: "string"
  decision_a: "a decisao proposta"
  decision_not_a: "a decisao oposta"
  evidence_for_a: []
  evidence_for_not_a: []
  assumptions_challenged: []
  comparison_matrix:
    risk: { a: 0, not_a: 0 }
    cost: { a: 0, not_a: 0 }
    time: { a: 0, not_a: 0 }
    knowledge_gain: { a: 0, not_a: 0 }
    reversibility: { a: 0, not_a: 0 }
  extreme_scenarios:
    at_10x: "string"
    at_100x: "string"
    catastrophic_failure: "string"
    in_6_months: "string"
  outcome: "prosseguir | escalar | rejeitar"
  rationale: "string"
  confidence: 0.0
  risks_identified:
    - risk: "string"
      severity: "baixa | media | alta | critica"
      mitigation: "string"
```

### Condicoes de Pulo

O gate pode ser pulado (com aviso) quando:
- A decisao e P2/P3 (operacional, baixo impacto)
- A decisao e trivial e nao admite oposicao real (ex: "usar git" — nao ha ~A viavel)
- O workflow e puramente mecanico (ex: rodar testes, formatar codigo)

### Integracao com KERNEL.md

```
Passo 6: Resolucao de Capacidade
         │
         ├── Decisao estrategica (P0/P1)?
         │       │
         │       ├── SIM ──► [GATE 0.5: CONTRAFACTUAL] ──► prosseguir?
         │       │                                            │
         │       │                                   prosseguir ─┴──► Passo 7: Planejamento
         │       │                                   escalar ──► Revisao mais profunda
         │       │                                   rejeitar ──► Revisao do Don
         │       │
         │       └── NAO ──► Passo 7 direto (P2/P3 ou sem decisao necessaria)
```

### Relacionados

- [Workflow: contrafactual-gate.md](workflows/contrafactual-gate.md) — Workflow completo, passos e exemplo
- [cosca-critic PROMPT.md](agents/cosca-critic/PROMPT.md) — Dono do gate e execucao
- [KERNEL.md sec 10](KERNEL.md) — Sequencia de Inicializacao (onde o gate e invocado)
- [CONFIDENCE_MODEL.md](engines/evidence/CONFIDENCE_MODEL.md) — Modelo de ponderacao de evidencias
- [RISK_REGISTRY.md](memory/risk/RISK_REGISTRY.md) — Registro de riscos conhecidos

---

## Gate 1 — Pre-Implementacao (Validacao do Plano)

Acionado pelo Planning Engine apos a geracao do Plano Executivo.

| Verificacao | Threshold | Severidade | Automatizado |
|-------------|-----------|------------|--------------|
| Revisao de arquitetura | Plano revisado pelo Chefe de Arquitetura | Erro | Nao |
| Revisao de seguranca | Implicacoes de seguranca avaliadas | Erro | Nao |
| Verificacao de dependencias | Sem dependencias circulares entre passos | Erro | Sim |
| Alocacao de recursos | Todos os departamentos necessarios disponiveis | Aviso | Nao |
| Avaliacao de risco | Top 3 riscos identificados com mitigacoes | Aviso | Nao |
| Criterios de sucesso | Pelo menos 1 criterio mensuravel definido | Erro | Sim |
| Estimativa | Esforco estimado (XS/S/M/L/XL) por passo | Aviso | Sim |

---

## Gate 2 — Pos-Implementacao (Qualidade do Codigo)

Acionado pelo Execution Engine apos conclusao da tarefa, executado por Review Engine + Quality Engine.

### 2.1 — Conformidade com Arquitetura
| Verificacao | Threshold | Severidade |
|-------------|-----------|------------|
| Limites de modulo respeitados | Sem violacoes de limite | Erro |
| Direcao de dependencia | Dependencias fluem para abestraoes estaveis | Erro |
| Conformidade CDR | Codigo segue CDRs documentadas | Erro |
| Consistencia de padroes | Mesmo padrao usado para mesmo tipo de problema | Aviso |

### 2.2 — Qualidade do Codigo
| Verificacao | Threshold | Severidade |
|-------------|-----------|------------|
| Principios SOLID | Sem violacoes detectadas | Erro |
| Principe DRY | Duplicacao < 5% nos arquivos alterados | Aviso |
| Tamanho de funcao | < 50 linhas por funcao | Aviso |
| Tamanho de arquivo | < 300 linhas por arquivo | Aviso |
| Contagem de parametros | < 5 por funcao | Aviso |
| Complexidade ciclomatica | < 10 por funcao | Aviso |
| Complexidade cognitiva | < 15 por funcao | Aviso |
| Clareza de nomenclatura | Descritivo, segue convencoes | Aviso |
| Codigo morto | 0 instancias | Erro |
| Codigo comentado | 0 instancias | Erro |
| Numeros magicos | 0 instancias, usar constantes nomeadas | Aviso |

### 2.3 — Seguranca
| Verificacao | Threshold | Severidade |
|-------------|-----------|------------|
| OWASP Top 10 | 0 violacoes | Erro |
| Segredos hardcoded | 0 instancias | Erro |
| Validacao de entrada | Todas as entradas externas validadas | Erro |
| Codificacao de saida | Todas as saidas adequadamente codificadas | Erro |
| Injecao SQL | Apenas queries parametrizadas | Erro |
| Prevencao XSS | Codificacao apropriada por contexto | Erro |
| Protecao CSRF | Tokens anti-CSRF em operacoes que alteram estado | Erro |
| Verificacao de autenticacao | Endpoints protegidos impoem autenticacao | Erro |
| Verificacao de autorizacao | Verificacoes de permissao em recursos protegidos | Erro |
| Auditoria de dependencias | 0 CVEs criticos/altos | Erro |

### 2.4 — Performance
| Verificacao | Threshold | Severidade |
|-------------|-----------|------------|
| Queries N+1 | 0 instancias | Erro |
| Indices ausentes | 0 tabelas sem indices adequados | Aviso |
| Lazy/eager loading | Estrategia correta por caso de uso | Aviso |
| Alocacoes desnecessarias | Sem objetos grandes em caminhos quentes | Aviso |
| Bloqueio sincrono | Sem operacoes sincronas em contextos async | Aviso |

### 2.5 — Testes
| Verificacao | Threshold | Severidade |
|-------------|-----------|------------|
| Cobertura de linha | > 80% no codigo alterado | Erro |
| Cobertura de branch | > 70% no codigo alterado | Aviso |
| Caminho feliz testado | Sim | Erro |
| Casos limites testados | Pelo menos 2 casos limites | Aviso |
| Caminhos de erro testados | Pelo menos 1 caminho de erro | Aviso |
| Independencia de teste | Nenhum teste depende de outro | Erro |
| Determinismo de teste | 0 testes instaveis | Erro |
| Tempo de execucao de teste | < 5 min para testes unitarios | Aviso |

### 2.6 — Documentacao
| Verificacao | Threshold | Severidade |
|-------------|-----------|------------|
| Documentacao de API | Todos os endpoints novos/alterados documentados | Erro |
| CDR | Criada se decisao de arquitetura tomada | Erro |
| README | Atualizado se estrutura do projeto alterada | Aviso |
| Changelog | Entrada adicionada para a mudanca | Aviso |
| Comentarios de codigo | Explicam "por que", nao "o que" | Aviso |
| TODOs/FIXMEs | 0 novas instancias sem referencia a issue | Aviso |

---

## Gate 3 — Pre-Release

Acionado pelo Release Chief antes do deploy.

| Verificacao | Threshold | Severidade |
|-------------|-----------|------------|
| Todas as verificacoes Gate 2 passam | 0 erros | Erro |
| Aprovacao QA | Obtida do QA Chief | Erro |
| Todos os conjuntos de testes passam | Unitarios, integracao, E2E | Erro |
| Escaneamento de seguranca | 0 achados criticos/altos | Erro |
| Benchmarks de performance | Dentro da faixa aceitavel | Erro |
| Documentacao completa | Todos os itens Gate 2.6 feitos | Erro |
| Notas de release | Geradas e revisadas | Aviso |
| Plano de rollback | Documentado e testado | Aviso |
| Monitoramento configurado | Alertas configurados para novos endpoints | Aviso |
| Notificacao de partes interesadas | Partes relevantes informadas | Aviso |

---

## Gate 4 — Pos-Release

Acionado pelo Monitoring Chief apos o deploy.

| Verificacao | Threshold | Severidade |
|-------------|-----------|------------|
| Health checks | Todos os endpoints saudaveis | Erro |
| Taxa de erro | < 1% de aumento | Erro |
| Latencia | < 10% de degradacao | Erro |
| Feedback do usuario | Sem relatorios criticos nas primeiras 24h | Aviso |
| Memoria/CPU | Dentro da faixa normal | Aviso |

---

## Calculo da Nota de Qualidade

```
OVERALL = (Arquitetura x 0.20) + (Qualidade do Codigo x 0.20) + (Seguranca x 0.25) + (Performance x 0.10) + (Testes x 0.15) + (Documentacao x 0.10)
```

| Faixa de Nota | Nota | Acao |
|---------------|------|------|
| 9.0 – 10.0 | A | Aprovado |
| 7.0 – 8.9 | B | Aprovado com sugestoes |
| 5.0 – 6.9 | C | Alteracoes solicitadas |
| 3.0 – 4.9 | D | Rejeitado, retrabalho necessario |
| 0.0 – 2.9 | F | Bloqueado, inseguro para prosseguir |

---

## RELACIONADOS

- [CONSTITUTION.md](CONSTITUTION.md) — Referencia Parte IV Passo 7
- [Contrafactual Gate](workflows/contrafactual-gate.md) — Workflow completo e exemplo do Gate 0.5
- [cosca-critic PROMPT.md](agents/cosca-critic/PROMPT.md) — Execucao do Gate 0.5
- [Review Engine](engines/review/SKILL.md) — Executa Gates 2.1–2.6
- [Quality Engine](engines/quality/SKILL.md) — Coleta e relatorio de metricas
- [QA Chief](departments/qa/SKILL.md) — Autoridade de aprovacao do gate
- [Release Chief](departments/release/SKILL.md) — Execucao do Gate 3
- [Monitoring Chief](departments/monitoring/SKILL.md) — Execucao do Gate 4
- [KERNEL.md](KERNEL.md) — Execucao dos Gates 0 e 0.5

---

## HISTORICO

| Versao | Data | Autor | Mudancas |
|--------|------|-------|----------|
| 1.1.0 | 2026-07-30 | cosca-critic | Adicionado Gate 0.5 — Revisao de Decisao Contrafactual (gate mandatorio pre-decisao para P0/P1) |
| 1.0.0 | 2026-07-10 | QA Chief | Gates de qualidade canonicos iniciais |

---

> **Executado por**: Review Engine + Quality Engine + cosca-critic + Release Chief | **Ultima revisao**: 2026-07-30
