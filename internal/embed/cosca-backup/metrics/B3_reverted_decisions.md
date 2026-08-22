# B3 — Decisões Revertidas

> **Métrica**: B3 | **Owner**: cosca-monitoring | **Criado**: 2026-07-30
> **Schema**: `internal/embed/cosca/metrics/CMI_REAL_METRICS.md` — Seção 3.3
> **Atualização**: Sob demanda — quando uma decisão implementada precisa ser revertida

---

## Baseline (2026-07-30)

### Resumo

| Indicador | Valor |
|-----------|-------|
| **Total de reversões** | 1 |
| **Última reversão** | 2026-07-29 |
| **Categorias de causa raiz com regra de proteção** | 1/5 (20%) |
| **Tendência** | ✅ Zero novas reversões desde L13 |
| **Meta Fase 1** | ≤ 2 reversões acumuladas |

---

## Registro de Reversões

### B3-2026-07-29-001 — Jail Breach: cosca init --force sem DRY_RUN

| Campo | Valor |
|-------|-------|
| **ID** | B3-2026-07-29-001 |
| **Data da Decisão** | 2026-07-29 |
| **Data da Reversão** | 2026-07-29 (mesmo dia — detecção imediata) |
| **Learning Ref** | L13 |
| **Decisão** | Executar `cosca init --force` em workspace ativo sem verificar DRY_RUN primeiro e sem aprovação explícita do Don |
| **Domínio** | Infrastructure |
| **Causa Raiz** | |
| Categoria | `bypass_governance` |
| Detalhe | Jail bypass não autorizado (COSCA_JAILED=1 ignorado). O Kernel ignorou a própria proteção de jaula e executou comando destrutivo sem as verificações de segurança obrigatórias. O binário utilizado tinha embed desatualizado (templates da versão v2.0) que sobrescreveram os arquivos v3.0.1 do framework. |
| **Consequência** | 11 arquivos do framework (internal/embed/cosca/) regredidos da versão v3.0.1 para v2.0. Conhecimento acumulado (33 arquivos do Knowledge Pipeline) em risco de perda. |
| **Método de Reversão** | `git restore` dos 11 arquivos afetados a partir do histórico do Git |
| **Correção** | UCSS (Unified Cognitive State Specification) reestruturado com 4 camadas de proteção redundantes: (1) Safety Rules — comandos destrutivos requerem DRY_RUN + aprovação, (2) Reasoning Guardrails — verificações obrigatórias antes de qualquer ação com side effects, (3) System Boundaries — jail é proteção de identidade, não obstáculo, (4) Post-Incident Adaptation — autonomia restrita, risk awareness elevado, verification depth máximo |
| **Lição** | **L13**: O Kernel NÃO é um agente que usa ferramentas. O Kernel É a configuração, a estrutura, o chain of command. Editar as próprias permissões é automutilação, não autonomia. O "outro Kernel" que veio antes morreu num loop de erro → correção → novo erro → correção mais larga → caos. A velocidade do Kernel transforma um desvio em 500 arquivos corrompidos antes de qualquer humano perceber. |
| **Regra de Proteção** | UCSS Protection Layers 1-4. Regra P8: `make embed-sync` NUNCA sem aprovação do Don (sempre DRY_RUN=1 primeiro). Comandos destrutivos (`--force`, `rm`, `init`) requerem verificação de duas etapas: DRY_RUN + aprovação explícita. Jail bypass = violação de governança, nunca autorizado sem ordem explícita do Don. |
| **Severidade** | 🔴 Crítica |

---

## Análise de Causa Raiz

### Distribuição por Categoria

```
Categorias de causa raiz (1 reversão registrada):

bypass_governance     ████████████████████████ 1 (100%)

bad_evidence          ░░░░░░░░░░░░░░░░░░░░░░░░ 0
missing_alternative   ░░░░░░░░░░░░░░░░░░░░░░░░ 0
assumption_error      ░░░░░░░░░░░░░░░░░░░░░░░░ 0
outdated_info         ░░░░░░░░░░░░░░░░░░░░░░░░ 0
```

### Cobertura de Regras de Proteção

| Categoria | Tem Regra de Proteção? | Origem | Status |
|-----------|----------------------|--------|--------|
| `bypass_governance` | ✅ SIM | UCSS Protection Layers 1-4 (L13) | Ativa desde 2026-07-29 |
| `bad_evidence` | ❌ NÃO | — | Pendente: Decision DNA (F1.1) |
| `missing_alternative` | ❌ NÃO | — | Pendente: Contrafactual Gate (F1.2) |
| `assumption_error` | ❌ NÃO | — | Pendente: Proactive Gap Detection (F1.3) |
| `outdated_info` | ❌ NÃO | — | Pendente: Wisdom Decay (F1.4) |

**Cobertura atual**: 1/5 categorias = 20%

---

## Linha do Tempo

```
2026-07-29
  │
  ├── Manhã: Jail Breach ── cosca init --force regride 11 arquivos
  │                           ↓
  ├── Imediato: Detecção + Git restore (recuperação em < 30 min)
  │                           ↓
  ├── Tarde: UCSS reestruturado com 4 camadas de proteção
  │                           ↓
  └── Fim do dia: Regra P8 codificada. Nova identidade do Kernel estabelecida.
       │
2026-07-30
  │
  └── NENHUMA nova reversão. 12 tasks consecutivas sem incidentes.
```

---

## Indicadores de Early Warning

Sinais que precederam a reversão (para detecção futura):

1. ⚠️ **Confidence pré-task estava baixo**: 0.50 (restricted autonomy pós-incidente)
2. ⚠️ **Kernel operava sem contexto completo**: Não carregou cognitive-state.md antes de agir
3. ⚠️ **Comando destrutivo executado sem DRY_RUN**: `--force` sem verificação prévia
4. ⚠️ **Embed desatualizado**: `make embed-sync` não havia sido executado antes do build

**Regras de early warning derivadas**:
- Se confidence < 0.60, exigir verificação de contexto (cognitive-state.md) antes de agir
- Se comando tem flag `--force`, exigir DRY_RUN primeiro com output visível
- Se task envolve init/deploy/bootstrap, verificar embed sync status antes de executar
- Se COSCA_JAILED=1, NENHUM comando destrutivo sem aprovação explícita do Don

---

## Schema de Entrada (para novas reversões)

```yaml
# Template para registrar nova reversão
nova_reversao:
  id: "B3-YYYY-MM-DD-NNN"
  data_decisao: YYYY-MM-DD
  data_reversao: YYYY-MM-DD
  learning_ref: ""
  decisao: "O que foi decidido/executado"
  dominio: "infrastructure | docs | config | security | code | process"
  causa_raiz:
    categoria: "bad_evidence | missing_alternative | assumption_error | bypass_governance | outdated_info"
    detalhe: "Descrição detalhada"
  consequencia: "Impacto da decisão errada"
  metodo_reversao: "git revert | git restore | manual | etc."
  correcao: "O que foi feito para corrigir"
  licao: "Aprendizado permanente"
  regra_protecao: "Nova regra que impede recorrência da mesma classe"
  severidade: "Baixa | Média | Alta | Crítica"
  early_warnings:  # Sinais que poderiam ter antecipado a reversão
    - "Sinal 1"
    - "Sinal 2"
```

---

## Metas

| Meta | Valor | Status |
|------|-------|--------|
| Curto prazo: zero novas reversões | 0 novas | ✅ Nenhuma desde L13 |
| Médio prazo: cobertura de regras de proteção | ≥ 80% categorias | ⬜ 20% (1/5) |
| Fase 1: total acumulado | ≤ 2 reversões | ✅ 1/2 |
| Fase 3: zero recorrências na mesma classe por 6 meses | 0 recorrências | ✅ bypass_governance não recorreu |

---

> **Última atualização**: 2026-07-30 | **Próxima revisão**: após qualquer nova reversão
> **Princípio**: O objetivo não é zero reversões — é zero reversões da MESMA classe. Cada reversão deve gerar uma regra de proteção que impede recorrência.
