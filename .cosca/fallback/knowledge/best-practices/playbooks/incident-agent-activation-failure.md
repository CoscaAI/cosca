---
id: playbook-006
title: "Resposta a Falha na Ativacao de Agente"
type: incident-response
severity: high
owner: cosca-kernel
created: 2026-07-29
tags:
  - agent-activation
  - orchestration
  - permission
  - dna
  - activation-gate
  - failures
  - kernel
related_incidents:
  - Onda 6: 7/8 agents activated, cosca-paradigm gated (2026-07-28)
  - Onda 2: 10-agent parallel activation (2026-07-28)
  - Onda 3: 9 specialists activation (2026-07-28)
  - opencode.json permission breach: stale paths, .* permissions (2026-07-29)
  - 51/55 agents without substantive failures.md
  - Semantic memory: only 3 agents with failure records
  - cosca-paradigm activation gate: 3 months Confidence Model data
  - kernel L12: security audit — permission hardening (2026-07-29)
---

# Resposta a Falha na Ativacao de Agente

## Trigger

Qualquer um destes eventos dispara o playbook:

1. **Agente spawna mas retorna erro** — `task` tool invoca agente, agente responde com erro ou sem deliverable
2. **Agente spawna mas output é vazio ou template-only** — agente responde sem executar tarefa real (padrão seed/template)
3. **Agente falha em produzir deliverable especificado** — prompt descreve output esperado, agente entrega algo diferente ou incompleto
4. **Agente recusa ativação por gate** — similar ao cosca-paradigm: agente verifica condição de ativação e reporta "gated"
5. **Múltiplos agentes falham simultaneamente** — 2+ agentes de uma mesma onda de ativação falham (indica problema sistêmico)

**Contexto real**: Entre 2026-07-28 e 2026-07-29, o kernel orquestrou 6 ondas de ativação com 54/55 agentes ativados (98%). Onda 6 ativou 7/8 agentes — cosca-paradigm recusou legitimamente: requer 3 meses de dados do Confidence Model (previsão Out/2026). Três problemas reais foram documentados: (1) activation gate legítimo (paradigm), (2) permissões stale em opencode.json apontando para diretório de outro usuário, (3) 51 de 55 agentes sem failures.md — agentes não registram falhas, então não aprendem com elas.

## Passos

### Passo 1: Verificar se o agente esta no opencode.json (DNA valido)

```bash
# Verificar se o agente tem entrada no opencode.json
grep -A 20 "\"name\": \"<agent-name>\"" .opencode/opencode.json

# Estrutura esperada de DNA (AGENT_DNA.md v3.0, 28 campos):
# - name: identificador unico
# - description: proposito e dominio
# - tools: [read, glob, grep, bash, write, edit, task]
# - permissions: regras de acesso a paths
# - systemPrompt: referencia ao prompt canonico (internal/embed/cosca/agents/<name>/PROMPT.md)
# - subagent_type: tipo no sistema de task
```

**Exemplo real**: Auditoria de segurança do kernel (L12, 2026-07-29) encontrou permissões stale no opencode.json — paths de permissão apontavam para `/home/henrique/Documents/...` (outro usuário) em vez do workspace real. Agentes com paths errados não conseguem acessar seus arquivos de memória ou prompts.

**Diagnóstico**:
- Se agente NÃO está no opencode.json → não é um agente registrado. Verificar se o nome está correto.
- Se agente está mas `systemPrompt` aponta para arquivo inexistente → criar prompt ou corrigir path.
- Se agente está mas `tools` estão vazios ou incompletos → agente não tem ferramentas para executar tarefas.

### Passo 2: Verificar activation gate

```bash
# Verificar se o agente tem condicao de ativacao (activation gate)
grep -ri "activation.*gate\|gated\|prerequisite\|required.*data" \
  internal/embed/cosca/memory/agent/<agent-name>/

# Exemplo real: cosca-paradigm
# Condicao: 3 meses de dados do Confidence Model acumulados
# Gate definido em: memory/roadmap/milestones.md, memory/ceo/activation-report.md
# Status: observation mode ate ~2026-10-28
```

**Padrão real** — Onda 6 (kernel L11, 2026-07-28):

| Agente | Gate | Status |
|--------|------|--------|
| cosca-paradigm | 3 meses de Confidence Model data | Gated até Out/2026 |
| cosca-ceo | Nenhum | Ativado (0.77) |
| cosca-cto | Nenhum | Ativado (0.72) |
| cosca-product | Nenhum | Ativado (0.65) |
| cosca-evolution | Nenhum | Ativado (0.72) |
| cosca-release | Nenhum | Ativado (0.75) |
| cosca-uiux | Nenhum | Ativado (0.50) |
| cosca-memory-chief | Nenhum | Ativado (0.62) |

**Ação**: Se o gate é legítimo, o agente está correto em recusar. Registrar no relatório de ativação e marcar como "gated — condição pendente". NÃO forçar ativação de agente com gate legítimo.

**Se o gate NÃO é legítimo** (agente deveria ativar mas não ativa):
- Verificar se a condição do gate já foi satisfeita (ex: 3 meses de dados já acumulados)
- Verificar se o agente está verificando a condição corretamente
- Se gate está stale/incorreto, atualizar a definição do gate

### Passo 3: Verificar permissoes do agente no opencode.json

```bash
# Extrair bloco de permissoes do agente no opencode.json
# Verificar se os paths de permissao existem no filesystem
# Exemplo de permissoes esperadas (kernel L12 security audit):

# Ferramentas por tipo de agente:
# - Chiefs (analiticos): read + glob + grep + task (restritas ao workspace)
# - Chiefs (implementacao): read + glob + grep + bash + write + edit + task
# - Specialists (implementacao): read + glob + grep + bash + write + edit
# - Kernel: read + glob + grep + bash + write + edit + task

# Verificar paths:
# - Workspace path esta correto? (nao apontar para outro usuario)
# - /tmp/opencode access: apenas bash e read precisam
# - glob/grep/read com ".*" violam least privilege
```

**Exemplo real de breach** (kernel L12, 2026-07-29):

| Ferramenta | Path configurado | Problema |
|-----------|-----------------|---------|
| read, glob, grep | `.*` (irrestrito) | Qualquer subagente pode ler qualquer arquivo do sistema |
| edit, write | `/tmp/opencode` (desnecessário) | edit/write não precisam de acesso a tmp |
| bash | `/tmp/opencode` (correto) | Necessário para compilações temporárias |
| Todos | Path apontando para `/home/henrique/...` | Diretório de outro usuário — stale path |

**Ação**:
- Se `.*` → restringir para workspace + `/tmp/opencode` (bash/read) ou workspace-only (edit/write)
- Se path stale → corrigir para o workspace real
- Se agente não tem ferramenta necessária → adicionar com escopo mínimo

### Passo 4: Verificar dependencias — o agente tem acesso aos arquivos que precisa?

```bash
# Verificar se o diretorio de memoria do agente existe
ls internal/embed/cosca/memory/agent/<agent-name>/

# Estrutura esperada (DNA v3.0):
# INDEX.md               — indice de memoria do agente
# capability-profile.md  — perfil de capacidade
# learnings.md           — aprendizados semanticos
# failures.md            — memoria negativa (falhas)
# evolution.md           — timeline de evolucao
# patterns.md            — padroes reutilizaveis

# Verificar se o PROMPT.md existe
ls internal/embed/cosca/agents/<agent-name>/PROMPT.md

# Verificar se ha arquivos de dominio que o agente precisa
# Exemplo: cosca-database precisa de acesso a internal/sqlite/
ls internal/sqlite/
```

**Dependências comuns que falham**:

| Dependência | Sintoma | Exemplo real |
|------------|---------|-------------|
| Prompt canonico ausente | Agente spawna com prompt generico/default | opencode.json referencia PROMPT.md que não existe |
| Memoria do agente vazia (seed-only) | Agente não tem contexto para execução real | 41/55 agentes com template-only seed, sem execution history (semantic memory audit, 2026-07-29) |
| Arquivos de codigo inacessiveis | Agente reporta "file not found" | Permissões stale apontando para diretório errado (kernel L12) |
| Falta de failures.md | Agente não aprende com erros passados | 51/55 agentes sem failures.md substancial — padrão "avoided failures" só existe em cosca-backend (2 avoided) |

### Passo 5: Se falha de runtime — verificar logs de falha do agente

```bash
# Verificar se ha registro de falha anterior
cat internal/embed/cosca/memory/agent/<agent-name>/failures.md

# Verificar o que o agente reportou na sessao atual
# (logs do proprio agente na sessao, se disponivel)

# Verificar se ha padrao de falha em outros agentes
grep -r "activation.*fail\|failed.*spawn\|error.*agent" \
  internal/embed/cosca/memory/agent/cosca-kernel/learnings.md
```

**Padrão real de falha documentada** — cosca-backend (único agente com failures.md substancial):

| Falha | Data | Causa Raiz | Evitada depois? |
|-------|------|-----------|:---:|
| Aggressive caching em auth-gated endpoints | 2026-07-15 | Cache key sem user identity → authorization leak | ✅ 2026-07-28 |
| Handler error format inconsistency | 2026-07-10 | Migração parcial: 12/19 handlers no novo formato | ✅ 2026-07-28 |
| Premature CRUD abstraction | 2026-06-20 | Generalizou de 2-3 exemplos, each domain tinha operações diferentes | ✅ (revertido) |

**Problema sistêmico**: 94% dos agentes (51/55) não têm failures.md substancial. O semantic memory audit (2026-07-29) identificou isso como "negative memory severely underutilized despite being the most valuable learning resource per AUTO_EVOLUTION protocol."

### Passo 6: Reportar ao cosca-kernel com diagnostico

Estrutura do relatório de falha de ativação:

```markdown
# Agent Activation Failure Report

**Agent**: <nome>
**Onda**: <numero da onda de ativacao>
**Timestamp**: 2026-07-29
**Severity**: high | medium | low

## Sintoma
[O que foi observado: erro, sem output, output errado, recusa]

## Diagnostico
[Qual dos 5 passos revelou o problema]

## Causa Raiz
- [ ] DNA invalido / ausente (Passo 1)
- [ ] Activation gate legitimo (Passo 2)
- [ ] Permissoes insuficientes (Passo 3)
- [ ] Dependencias inacessiveis (Passo 4)
- [ ] Falha de runtime / erro do agente (Passo 5)

## Acao
[Correcao aplicada ou recomendada]

## Confidence Impact
[Impacto estimado na confianca do agente: -0.05 a -0.15]
```

**Exemplo real — Relatório Onda 6** (kernel L11):

```
7/8 ativados com sucesso: cto (0.72), product (0.65), memory-chief (0.62),
ceo (0.77), evolution (0.72), release (0.75), uiux (0.50)
1 gated: cosca-paradigm — activation gate legitimo (3 meses Confidence Model)
Resultado: 54/55 agentes ativos (98%)
```

## Escalação

| Gatilho | Escalar para | Razão |
|---------|-------------|-------|
| Falha sistêmica (múltiplos agentes simultâneos, 3+) | `cosca-ceo` | Indica problema de infraestrutura ou governance, não de agente individual |
| Activation gate bloqueia agente crítico para milestone | `cosca-ceo` + `cosca-product` | Requer decisão de prioridade: acelerar condição do gate ou redefinir milestone |
| Permissões com `.*` (irrestritas) em qualquer agente | `cosca-security` + `cosca-kernel` | Violação de least privilege — padrão encontrado em 2026-07-29 |
| Paths de permissão stale (apontando para outro usuário) | `cosca-kernel` | Requer auditoria completa de todos os paths no opencode.json |
| Agente sem failures.md após 3+ tarefas reais | `cosca-kernel` + `cosca-governance` | Viola AUTO_EVOLUTION_PROTOCOL — agentes devem registrar falhas |
| Agente com confidence < 0.25 após ativação | `cosca-kernel` | Agente seed sem execução real não deve ser tratado como ativo |

## Prevencao

1. **Validation pre-flight na ativação** — antes de spawnar um agente, verificar: (a) existe no opencode.json, (b) PROMPT.md existe, (c) diretório de memória existe, (d) paths de permissão são válidos. Onda 6 estabeleceu o padrão: contexto + escopo + deliverables + formato.

2. **Activation gates documentados e visíveis** — todo agente com gate deve ter a condição documentada em: `capability-profile.md`, `memory/roadmap/milestones.md`, e `memory/ceo/activation-report.md`. O caso cosca-paradigm (gated até Out/2026) é o exemplo canônico.

3. **Failures.md obrigatório após primeira tarefa real** — se um agente executa uma tarefa e falha, DEVE registrar em failures.md. Se executa e tem sucesso, registrar em learnings.md. O padrão "avoided failures" do cosca-backend (2 avoided em 2026-07-28) mostra o valor da memória negativa.

4. **Auditoria periódica de permissões** — similar à auditoria de segurança L12 (2026-07-29): verificar se paths de permissão estão corretos, se `.*` foi substituído por paths restritos, se `/tmp/opencode` só está acessível a bash/read.

5. **DNA compliance validator** — automatizar verificação de que todo agente no opencode.json tem os 28 campos do DNA v3.0, PROMPT.md válido, diretório de memória com arquivos obrigatórios (INDEX.md, capability-profile.md, learnings.md, failures.md, evolution.md).

6. **Cross-agent failure pattern detection** — o semantic memory audit (2026-07-29) revelou que falhas se agrupam por domínio (Auth/Security, Documentation Integrity, Runtime Lifecycle, Database Reality). Agentes no mesmo cluster devem compartilhar failures.md para evitar repetição de erros.

## Referencias

- **Onda 6 ativação (7/8, paradigm gated)**: `internal/embed/cosca/memory/agent/cosca-kernel/learnings.md` L11 (2026-07-28)
- **Onda 2 + 3 + 5 ativações**: `internal/embed/cosca/memory/agent/cosca-kernel/learnings.md` L2, L3, L10
- **Security audit — permission hardening**: `internal/embed/cosca/memory/agent/cosca-kernel/learnings.md` L12 (2026-07-29)
- **cosca-paradigm activation gate**: `internal/embed/cosca/memory/roadmap/milestones.md`, `internal/embed/cosca/memory/ceo/activation-report.md`
- **Failures.md — cosca-backend (canonical)**: `internal/embed/cosca/memory/agent/cosca-backend/failures.md` (63 linhas, 3 falhas)
- **Semantic memory: failure underutilization**: `internal/embed/cosca/memory/agent/cosca-semantic-memory/learnings.md` (2026-07-29)
- **Agent DNA v3.0 (28 fields)**: `internal/embed/cosca/AGENT_DNA.md`
- **AUTO_EVOLUTION_PROTOCOL**: `internal/embed/cosca/shared/AUTO_EVOLUTION_PROTOCOL.md`
- **Technical debt: AGT-02 (51/55 without failures)**: `internal/embed/cosca/memory/technical-debt/scorecard.md`
- **Workspace state: 54/55 activated**: `internal/embed/cosca/memory/context/workspace-state.md`
