# F1.3 PROACTIVE GAP DETECTION ENGINE

> **Versão**: 1.0.0 | **Status**: active | **Owner**: Architecture Chief | **Criado**: 2026-07-30
>
> **Autoridade constitucional**: [CONSTITUTION.md](../../CONSTITUTION.md) — Implementa P2 (código é a verdade), P4 (intervalo de confiança), P5 (aprender com erros).
>
> **Dependências**: [F1.1 DDNA](../../knowledge/architecture/DECISION_DNA.md) · [F1.2 Contrafactual Gate](../../workflows/contrafactual-gate.md) · [QUALITY_GATES.md](../../QUALITY_GATES.md)
>
> **Lema**: *"Detecte antes que doa. Um gap hoje é um incidente amanhã."*

---

## SUMÁRIO

1. [Definição](#definição)
2. [6 Dimensões de Gap](#6-dimensões-de-gap)
3. [Pipeline de Detecção](#pipeline-de-detecção)
4. [Gap Score](#gap-score)
5. [Integrações](#integrações)
6. [Configuração de Thresholds](#configuração-de-thresholds)
7. [Automação CLI e CI](#automação-cli-e-ci)
8. [Exemplo Real](#exemplo-real)
9. [Formato de Dados](#formato-de-dados)
10. [Qualidade e Métricas](#qualidade-e-métricas)

---

## DEFINIÇÃO

### O que é

A **Proactive Gap Detection Engine (F1.3)** é o mecanismo que varre o código e o repositório **antes** que gaps de qualidade se transformem em incidentes. Diferente de ferramentas reativas (que alertam depois que o problema aconteceu — ex: pipeline quebrou, bug em produção), a F1.3 opera **antes do commit** e **antes do deploy**, calculando um **Gap Score** que reflete o risco acumulado de qualidade.

### Proativo vs Reativo

| Aspecto | Reativo (tradicional) | Proativo (F1.3) |
|---------|-----------------------|-----------------|
| **Quando** | Depois do deploy / após falha | Antes do commit / no planejamento |
| **Postura** | "Apaga incêndio" | "Previne incêndio" |
| **Métrica** | MTTR, incidentes abertos | Gap Score, tendência de débito |
| **Exemplo** | CI falha porque teste quebrou | Pré-commit detecta: "este diff reduz cobertura em 5%" |
| **Custo** | Alto (rollback, hotfix, retrabalho) | Baixo (feedback imediato no diff) |
| **Dívida** | Acumula silenciosamente | Detectada no commit, documentada no DDNA |

### Filosofia

```
gap_hoje = dívida_amanhã
gap_score > 30 → bloqueia com sugestões
gap P0 → gera DDNA automaticamente
gap arquitetural → aciona Contrafactual Gate
```

---

## 6 DIMENSÕES DE GAP

Cada gap é classificado em uma das 6 dimensões. A severidade é calculada por regras determinísticas que combinam magnitude do desvio + impacto potencial.

### Tabela de Dimensões

| # | Dimensão | Código | Regra de Detecção | Severidade | Ação Padrão | Threshold Config |
|---|----------|--------|--------------------|------------|-------------|------------------|
| 1 | **Cobertura** | `GAP_COVERAGE` | Queda ≥ threshold_points em N commits consecutivos (ex: -5% em 3 commits) | **HIGH** se > -5%, **CRITICAL** se > -10% | Bloquear commit, sugerir testes para arquivos descobertos | `coverage.drop_pct: 5`, `coverage.commits: 3` |
| 2 | **Dívida Técnica** | `GAP_DEBT` | `TODO`/`FIXME`/`HACK`/`XXX` sem DDNA ID vinculado. Arquivos com mais de K marcações sem triagem | **MEDIUM** se 3-5 marcações, **HIGH** se > 5 | Bloquear, exigir DDNA ou link para issue. Registrar no DDNA automático se > 5 | `debt.max_unlinked: 3` |
| 3 | **Documentação** | `GAP_DOCS` | Arquivo novo (diff) com > threshold_lines sem docstring/bloco de documentação no topo | **MEDIUM** se > 100 linhas sem doc, **HIGH** se > 300 linhas sem doc | Avisar, sugerir template de doc do módulo | `docs.min_lines_per_file: 300` |
| 4 | **Segurança** | `GAP_SECURITY` | `import`/`require` de package que faz chamada de rede (HTTP, socket, DNS) sem camada de isolamento ou validação de entrada | **HIGH** se import de rede sainte, **CRITICAL** se import + sem sanitização | Bloquear, exigir revisão de segurança | `security.block_network_imports: true` |
| 5 | **Arquitetura** | `GAP_ARCH` | Arquivo > threshold_lines (default 500) e/ou > threshold_methods (default 15) e/ou alta complexidade ciclomática (> 20) | **MEDIUM** se > 500 linhas, **HIGH** se > 800, **CRITICAL** se > 1200 | Bloquear, sugerir refatoração, acionar Contrafactual Gate se > 1000 | `arch.max_lines: 500`, `arch.max_methods: 15`, `arch.max_cyclomatic: 20` |
| 6 | **Memória de Agente** | `GAP_MEMORY` | Agente (especialista ou chief) sem learnings registradas há > threshold_days. Ausência de entrada no `learnings.md` | **LOW** se 7-14 dias, **MEDIUM** se 15-30, **HIGH** se > 30 | Notificar agente, sugerir sessão de aprendizado | `memory.agent_gap_days: 7` |

### Detalhamento por Dimensão

#### 1. GAP_COVERAGE — Cobertura de Testes

**Detecção**:
- Pré-commit: diff adiciona/modifica linhas sem teste correspondente → marca como descoberto
- Pós-commit: varredura semanal compara cobertura atual vs baseline (últimos 7 dias)
- Regra: queda de threshold% (ex: 5%) em threshold_commits (ex: 3 commits) → GAP_COVERAGE

**Cálculo**:
```
file_coverage = lines_tested / lines_total
if file_coverage < baseline - threshold → gap
if trend[3_commits] é decrescente → severidade sobe um nível
```

**Exemplo**:
- Commit A: cobertura 85%
- Commit B: cobertura 82%
- Commit C: cobertura 78%
- Threshold = 5%, commits = 3
- Queda acumulada = 7% (> 5%) → GAP_COVERAGE HIGH

#### 2. GAP_DEBT — Dívida Técnica

**Detecção**:
- Varre arquivos modificados no diff por `TODO`/`FIXME`/`HACK`/`XXX`
- Verifica se cada marcação tem: `// DDNA-YYYY-MM-DD-NNN` ou `// issue: #NNN` ou `// ref: link`
- Se não tiver → gap

**Regra de triagem**:
```yaml
debt_markers:
  - pattern: "TODO"
    require: "DDNA ID ou issue link"
  - pattern: "FIXME"
    require: "DDNA ID (obrigatório)"
    severity_boost: HIGH  # FIXME sem DDNA é automaticamente HIGH
  - pattern: "HACK"
    require: "DDNA ID + data de expiração"
  - pattern: "XXX"
    require: "DDNA ID"
```

#### 3. GAP_DOCS — Documentação Ausente

**Detecção**:
- Arquivo novo > threshold_lines sem docblock no topo
- Arquivos públicos (exportados) sem docstring
- Pacote sem `package.go` ou `doc.go`

**Regras**:
```
GAP_DOCS se:
  - file.line_count > docs.min_lines AND no docblock no início
  - exported_function/func sem comentário
  - diretório com >5 arquivos .go e sem doc.go
```

#### 4. GAP_SECURITY — Segurança

**Detecção**:
- Import de pacotes que fazem chamadas de rede no diff
- Uso de `http.Get`, `net.Dial`, `tls.Dial`, etc.
- Sem validação de input (regex, sanitização) no mesmo arquivo
- Lista de packages monitorados configurável

**Packages monitorados (default)**:
```
net/http, net, net/url, crypto/tls, golang.org/x/net, google.golang.org/grpc
```

#### 5. GAP_ARCH — Arquitetura

**Detecção**:
- Linhas por arquivo (> threshold)
- Métodos por arquivo (> threshold)
- Complexidade ciclomática (> threshold)
- Profundidade de aninhamento (> 5 níveis)

**Cálculo de gap**:
```
arch_score = (
  (line_count / max_lines * 0.4) +
  (method_count / max_methods * 0.3) +
  (cyclomatic / max_cyclomatic * 0.3)
) * 100

if arch_score > 60 → GAP_ARCH (severidade baseado no score)
```

#### 6. GAP_MEMORY — Memória de Agente

**Detecção**:
- Varre diretórios de agentes em `internal/embed/cosca/agents/` e `departments/`
- Verifica timestamp do último learning entry em `learnings.md`
- Se `hoje - ultimo_learning > memory.agent_gap_days` → gap

**Regra**:
```
GAP_MEMORY:
  agentes_obrigados: [cosca-architecture, cosca-backend, cosca-frontend, ...]
  dias_limite: 7 (configurável)
  severidade:
    7-14 dias: LOW
    15-30 dias: MEDIUM
    > 30 dias: HIGH
```

---

## PIPELINE DE DETECÇÃO

### Pipeline Completo

```
┌─────────────────────────────────────────────────────────────────┐
│                    F1.3 GAP DETECTION PIPELINE                    │
└─────────────────────────────────────────────────────────────────┘

                              ▼
┌─────────────────────────────────────────────────────────┐
│ FASE 0: GATILHO                                           │
│   ├── GIT_PRE_COMMIT (hook)                               │
│   ├── GIT_POST_COMMIT (hook, assíncrono)                  │
│   ├── CI_PR_OPEN (GitHub/GitLab webhook)                  │
│   ├── CI_SCHEDULED (cron semanal)                         │
│   └── CLI_TRIGGER (comando manual `cosca gap scan`)       │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│ FASE 1: DIFF ANALYSIS (< 2 segundos)                     │
│   ├── 1.1 Calcular diff do commit/PR                     │
│   ├── 1.2 Identificar arquivos novos, modificados        │
│   ├── 1.3 Extrair métricas por arquivo                    │
│   │     ├── lines_added / lines_deleted                  │
│   │     ├── funções exportadas novas                     │
│   │     ├── imports de rede                              │
│   │     ├── TODO/FIXME/XXX sem DDNA                     │
│   │     ├── docstrings ausentes                          │
│   │     └── complexidade ciclomática                     │
│   └── 1.4 Comparar com baseline histórica                │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│ FASE 2: SCORE & SEVERIDADE                              │
│   ├── 2.1 Para cada dimensão, calcular gap_score (0-100)│
│   ├── 2.2 Gap Score Total = média ponderada das 6       │
│   ├── 2.3 Pesos configuráveis (ver seção thresholds)     │
│   └── 2.4 Classificar: < 15 = OK, 15-30 = warning,       │
│                         > 30 = bloqueio (pré-commit)     │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│ FASE 3: AÇÃO (pré-commit x pós-commit)                   │
│                                                           │
│  PRÉ-COMMIT (síncrono, < 2s):                             │
│   ├── gap_score ≤ 15 → ✅ permitir commit                 │
│   ├── gap_score 16-30 → ⚠️ warning + sugestões           │
│   └── gap_score > 30 → 🚫 BLOQUEAR commit + explicação   │
│                                                           │
│  PÓS-COMMIT (assíncrono, ~30s):                           │
│   ├── gap_score 15-30 → 💤 gap silencioso (log + registro)│
│   ├── gap_score > 30 → 📝 gap registrado                 │
│   ├── gap P0 (severidade CRITICAL) → 📌 abrir DDNA        │
│   │   + notificar no dashboard                            │
│   └── gap arquitetural → 🔄 acionar Contrafactual Gate    │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│ FASE 4: REGISTRO & APRENDIZADO                          │
│   ├── 4.1 Registrar gap no GAP_REGISTRY.md              │
│   ├── 4.2 Gap P0 → criar DDNA entry                     │
│   ├── 4.3 Gap arquitetural → acionar Contrafactual Gate │
│   ├── 4.4 Atualizar gap-trend.md com score histórico    │
│   ├── 4.5 Se mesma dimensão de gap > 3x em 30 dias      │
│   │   → flag para revisão de processo                   │
│   └── 4.6 Notificar agentes/departamentos afetados       │
└─────────────────────────────────────────────────────────┘
```

### Pré-commit (Rápido — < 2s)

| Etapa | O quê | Tempo Máximo |
|-------|-------|-------------|
| Diff parse | Extrair diff do staged | 200ms |
| Arquivos novos | Verificar se > threshold_lines sem doc | 100ms |
| Imports de rede | Detectar packages de rede no diff | 100ms |
| TODO/FIXME scan | Marcações sem DDNA | 150ms |
| Complexidade | Cálculo rápido (linhas/métodos) | 200ms |
| Score | Calcular gap score total | 50ms |
| Decisão | Bloquear? Warning? Permitir? | 50ms |
| **Total** | | **< 850ms** (garantia < 2s) |

**Estratégia de timeout**: Se > 2s, liberar commit com warning parcial e escalar para pós-commit.

### Pós-commit (Completo — Semanal)

| Etapa | O quê | Frequência |
|-------|-------|-----------|
| Deep diff | Comparação completa com baseline | A cada commit (async) |
| Cobertura | Rodar testes com -coverprofile e comparar | A cada commit (async) |
| Dívida técnica | Varredura de TODO/FIXME em toda a base | Semanal |
| Arquitetura | Análise de complexidade completa | Semanal |
| Memória | Verificar learnings de todos os agentes | Diária |
| Gap silencioso | Gaps < 30 que não bloquearam o commit | Registrar sempre |
| Gap P0 → DDNA | Severidade CRITICAL → DDNA automático | Imediato |

---

## GAP SCORE

### Fórmula

```
gap_score_total = Σ(gap_dimensão_n × peso_n) / Σ(pesos)
```

Onde cada `gap_dimensão_n` é calculado de 0 a 100 pela dimensão específica.

### Pesos Default

| Dimensão | Peso | Justificativa |
|----------|------|---------------|
| GAP_COVERAGE | 1.0 | Cobertura baixa afeta diretamente a confiança no código |
| GAP_DEBT | 0.8 | Dívida não gerenciada cresce exponencialmente |
| GAP_DOCS | 0.4 | Baixo impacto imediato, alto custo a longo prazo |
| GAP_SECURITY | 1.2 | Risco de segurança tem peso maior (confidencialidade) |
| GAP_ARCH | 0.6 | Dívida arquitetural é cara de reverter |
| GAP_MEMORY | 0.3 | Impacto indireto (aprendizado do sistema) |

### Thresholds de Decisão

| Gap Score | Pré-commit | Pós-commit | Significado |
|-----------|-----------|------------|-------------|
| **≤ 15** | ✅ Permitir | — | Código saudável |
| **16–30** | ⚠️ Warning + sugestões | 💤 Gap silencioso (log) | Atenção, mas não bloqueia |
| **31–50** | 🚫 Bloquear | 📝 Registrar gap | Qualidade comprometida |
| **51–80** | 🚫 Bloquear + exigir revisão | 📌 Abrir DDNA | Crítico — precisa ação |
| **> 80** | 🚫🚫 Bloquear + escalar | 🔄 Acionar Contrafactual Gate | Arquitetura em risco |

---

## INTEGRAÇÕES

### Com F1.1 DDNA

Quando um gap recebe severidade **CRITICAL (P0)**, a engine cria automaticamente uma entrada no formato DDNA:

```yaml
# Gatilho: gap_score > 80 ou severidade CRITICAL
# Ação: criar DDNA entry

gap → DDNA:
  condição: gap.severity == "CRITICAL" OR gap.score > 80
  template: DECISION_DNA_FORMAT.md
  campos_preenchidos:
    Decisão: "Resolver gap [dimensão] em [arquivo]"
    Contexto: "Gap detectado por F1.3 em [timestamp]. Score: [score]"
    Domínio: [dimensão]
    Confiança: "0.XX" (baseado no gap score)
    Nível: "3" (arquitetura)
    Riscos: Preenchidos a partir da severidade/descrição do gap
    Alternativas: "Não resolver (gap persiste)", "Resolver agora (custo imediato)"
  fluxo:
    - "Criar DDNA entry em memory/decisions/"
    - "Registrar no learnings.md do Architecture Chief"
    - "Relacionar no GAP_REGISTRY.md"
```

### Com F1.2 Contrafactual Gate

Quando um gap arquitetural é detectado (GAP_ARCH com score > 80), a engine dispara o Contrafactual Gate:

```yaml
# Gatilho: GAP_ARCH com score > 80
# Ação: invocar contrafactual-gate.md

gap → gate:
  condição:
    dimensão: GAP_ARCH
    score: "> 80"
    ou: "arquivo com > 1000 linhas no diff"
  invocação:
    workflow: "workflows/contrafactual-gate.md"
    decision: "Refatorar [arquivo] ou manter como está"
    decision_priority: "P0"
    context:
      gap_id: "GAP-XXX"
      arquivo: "path/to/file.go"
      linhas: 1050
      métodos: 18
      score: 82
    evidence_for_a: "Funcional. Código legado sem bugs conhecidos."
    alternatives_considered:
      - "Extrair módulo principal"
      - "Dividir em 3 arquivos"
      - "Manter e registrar dívida técnica"
```

### Com QUALITY_GATES.md

A F1.3 se alinha com os Quality Gates do Cosca:

| Quality Gate | F1.3 Trigger | Ação |
|---|---|---|
| QG-1: Code Review | gap.score > 30 no diff | Bloquear merge até review |
| QG-2: Test Coverage | GAP_COVERAGE detectado | Exigir testes nos arquivos descobertos |
| QG-3: Documentation | GAP_DOCS detectado | Exigir docblock |
| QG-4: Security Review | GAP_SECURITY detectado | Acionar Security Chief |
| QG-5: Architecture Review | GAP_ARCH score > 80 | Acionar Contrafactual Gate |

---

## CONFIGURAÇÃO DE THRESHOLDS

### Formato YAML

```yaml
# internal/embed/cosca/engines/gap-detection/config.yaml

gap_detection:
  enabled: true
  pre_commit_timeout_ms: 2000

  # Pesos das dimensões
  weights:
    coverage: 1.0
    debt: 0.8
    docs: 0.4
    security: 1.2
    arch: 0.6
    memory: 0.3

  # Thresholds por dimensão
  dimensions:
    coverage:
      drop_pct: 5           # Queda % para disparar gap
      commits_to_check: 3    # Número de commits para tendência
      min_target: 80         # Cobertura mínima aceitável

    debt:
      max_unlinked_markers: 3    # Máx de TODO/FIXME sem DDNA
      block_at: 5                # Bloquear se > N marcações sem DDNA
      required_for:
        - "FIXME"
        - "HACK"

    docs:
      min_lines_for_docblock: 50     # Mín de linhas para exigir doc
      require_package_doc: true      # Exigir doc.go em pacotes > 5 arquivos
      block_at_lines: 300            # Bloquear se > N sem doc

    security:
      block_network_imports: true
      monitored_packages:
        - "net/http"
        - "net"
        - "net/url"
        - "crypto/tls"
        - "golang.org/x/net"
        - "google.golang.org/grpc"
      require_input_sanitization: true

    arch:
      max_lines_per_file: 500
      max_methods_per_file: 15
      max_cyclomatic_complexity: 20
      max_nesting_depth: 5

    memory:
      agent_gap_days: 7
      agent_gap_severity_days:
        low: 7
        medium: 15
        high: 30

  # Scores
  scoring:
    block_threshold: 30       # Gap score para bloquear commit
    warn_threshold: 15        # Gap score para warning
    p0_threshold: 80          # Gap score para P0 (abrir DDNA)
    gate_threshold: 80        # Gap score para acionar Contrafactual Gate
```

---

## AUTOMAÇÃO CLI E CI

### CLI

```bash
# Escanear working directory completo
cosca gap scan

# Escanear diff staged (pré-commit)
cosca gap scan --staged

# Escanear diff entre branches
cosca gap scan --base main --head feature

# Escanear arquivo específico
cosca gap scan --file path/to/file.go

# Modo CI (JSON output, exit code reflete bloqueio)
cosca gap scan --ci --format json

# Configurar thresholds (cria config.yaml)
cosca gap init

# Ver relatório de gaps ativos
cosca gap report

# Ver histórico de gaps
cosca gap history --days 30
```

### Exit Codes

| Exit Code | Significado |
|-----------|-------------|
| 0 | Nenhum gap ou apenas warnings |
| 1 | Gap score > 30 (bloqueio) |
| 2 | Gap P0 detectado (DDNA criado) |
| 3 | Gap arquitetural (Gate acionado) |

### CI (GitHub Actions)

```yaml
# .github/workflows/gap-detection.yml
name: Gap Detection
on:
  pull_request:
    types: [opened, synchronize]

jobs:
  gap-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 10  # Para baseline de commits

      - name: Gap Scan
        run: |
          cosca gap scan --ci --base origin/main \
            --format json --output gap-report.json

      - name: Upload Gap Report
        uses: actions/upload-artifact@v4
        with:
          name: gap-report
          path: gap-report.json

      - name: Check Gap Score
        run: |
          SCORE=$(jq '.gap_score_total' gap-report.json)
          if [ "$(echo "$SCORE > 30" | bc)" -eq 1 ]; then
            echo "❌ Gap score $SCORE exceeds threshold 30"
            exit 1
          fi
          echo "✅ Gap score $SCORE is acceptable"
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit (instalado por `cosca gap init`)

cosca gap scan --staged --format json --output /tmp/gap-precommit.json
EXIT_CODE=$?

if [ $EXIT_CODE -eq 0 ]; then
    echo "✅ Gap Detection: clean"
    exit 0
fi

SCORE=$(jq '.gap_score_total' /tmp/gap-precommit.json)
echo "⚠️  Gap Detection: score $SCORE (threshold: 30)"

if [ "$(echo "$SCORE > 30" | bc)" -eq 1 ]; then
    echo ""
    echo "❌ COMMIT BLOQUEADO por Gap Detection"
    echo "   Gap Score: $SCORE (máximo permitido: 30)"
    echo "   Use --no-verify para ignorar (não recomendado)"
    echo ""
    jq -r '.gaps[] | "   • [\(.dimension)] \(.description)"' /tmp/gap-precommit.json
    echo ""
    echo "   Sugestões:"
    jq -r '.suggestions[] | "   • \(.)"' /tmp/gap-precommit.json
    exit 1
fi

exit 0
```

---

## EXEMPLO REAL

### Cenário

Um desenvolvedor adiciona um arquivo novo de **600 linhas** que implementa um cliente HTTP para uma API externa, **sem testes**, **sem docblock**, e com **3 TODO/FIXME** sem DDNA.

```go
// api/client.go — NOVO ARQUIVO, 600 linhas

package api

import (
    "net/http"    // ← GAP_SECURITY: import de rede
    "encoding/json"
)

type Client struct { /* ... */ }  // ← GAP_DOCS: sem docblock na struct

func (c *Client) Fetch(url string) (*Response, error) {
    // TODO: implementar retry com backoff   ← GAP_DEBT: sem DDNA
    // FIXME: timeout está hardcoded         ← GAP_DEBT: sem DDNA
    resp, err := http.Get(url)               // ← GAP_SECURITY: sem sanitização
    // ...
}

// 600 linhas, 0 testes                          ← GAP_COVERAGE
// 14 métodos, complexidade média 18              ← GAP_ARCH: > 15 métodos
```

### Execução do Pré-commit

```bash
$ cosca gap scan --staged
```

### Saída

```json
{
  "gap_score_total": 65,
  "threshold_block": 30,
  "blocked": true,
  "gaps": [
    {
      "dimension": "GAP_COVERAGE",
      "score": 85,
      "severity": "CRITICAL",
      "description": "api/client.go: 600 linhas adicionadas, 0 testes correspondentes",
      "details": {
        "lines_added": 600,
        "test_lines": 0,
        "test_files": [],
        "suggestion": "Criar api/client_test.go com testes para Fetch() e Parse()"
      }
    },
    {
      "dimension": "GAP_DOCS",
      "score": 70,
      "severity": "HIGH",
      "description": "api/client.go: 600 linhas sem docblock no topo; struct Client sem docstring; função Fetch sem comentário",
      "details": {
        "lines_without_doc": 600,
        "exported_symbols_without_doc": ["Client", "Fetch"],
        "suggestion": "Adicionar docblock com descrição do pacote, documentar Client e Fetch"
      }
    },
    {
      "dimension": "GAP_DEBT",
      "score": 60,
      "severity": "HIGH",
      "description": "2 marcações sem DDNA: TODO (L42, sem issue), FIXME (L43, sem DDNA)",
      "details": {
        "markers": [
          {"type": "TODO", "line": 42, "text": "implementar retry com backoff", "linked": false},
          {"type": "FIXME", "line": 43, "text": "timeout está hardcoded", "linked": false}
        ],
        "suggestion": "Vincular cada TODO/FIXME a um DDNA ou issue. Use: // TODO DDNA-2026-07-30-001: ..."
      }
    },
    {
      "dimension": "GAP_SECURITY",
      "score": 55,
      "severity": "HIGH",
      "description": "Import de net/http (chamada de rede) sem camada de validação de entrada visível no diff",
      "details": {
        "network_imports": ["net/http"],
        "has_sanitization": false,
        "suggestion": "Adicionar validação de URL antes de http.Get (url.Parse, validação de scheme/host)"
      }
    },
    {
      "dimension": "GAP_ARCH",
      "score": 45,
      "severity": "MEDIUM",
      "description": "600 linhas (threshold: 500), 14 métodos (threshold: 15)",
      "details": {
        "line_count": 600,
        "method_count": 14,
        "cyclomatic_max": 18,
        "suggestion": "Considere extrair a lógica HTTP para um transport.go separado"
      }
    }
  ],
  "suggestions": [
    "📋 TESTES: Crie api/client_test.go com testes para Fetch() e Parse() — reduza GAP_COVERAGE de 85 para 0",
    "📖 DOCS: Adicione comentário de pacote // Package api implements ... — reduza GAP_DOCS de 70 para 20",
    "🔗 DDNA: Vincule TODO e FIXME a DDNA entries existentes ou crie novas — reduza GAP_DEBT de 60 para 20",
    "🔒 SEGURANÇA: Adicione validação de URL em Fetch() — reduza GAP_SECURITY de 55 para 10",
    "🏗️ ARQUITETURA: Extraia transport.go — reduza GAP_ARCH de 45 para 15",
    "",
    "💡 Resolver todos os gaps reduziria o Gap Score de 65 para ~8 (✅ permitido)"
  ]
}
```

### Resultado

```
❌ COMMIT BLOQUEADO (Gap Score: 65 > 30)

Gaps encontrados:
  • GAP_COVERAGE  [CRITICAL]: 600 linhas, 0 testes
  • GAP_DOCS      [HIGH]:    600 linhas sem docblock
  • GAP_DEBT      [HIGH]:    2 marcações sem DDNA
  • GAP_SECURITY  [HIGH]:    import de rede sem validação
  • GAP_ARCH      [MEDIUM]:  600 linhas, 14 métodos

Para resolver:
  1. Adicione api/client_test.go (testes)
  2. Adicione docblock e docstrings
  3. Vincule TODO/FIXME a DDNA
  4. Adicione validação de entrada
  5. Considere extrair transport.go

Use `cosca gap scan --format json` para ver o relatório completo.
```

---

## FORMATO DE DADOS

### Estrutura de um Gap

```yaml
gap:
  id: "GAP-{YYYYMMDD}-{NNNN}"       # ID único
  timestamp: "ISO8601"               # Detecção
  detected_by: "cosca-gap-detection" # Engine
  trigger: "pre-commit | post-commit | scheduled | manual"
  dimension: "GAP_COVERAGE | GAP_DEBT | GAP_DOCS | GAP_SECURITY | GAP_ARCH | GAP_MEMORY"
  severity: "LOW | MEDIUM | HIGH | CRITICAL"
  score: 0-100                       # Score específico da dimensão

  # Descrição
  file: "path/to/file.go"
  lines_affected: 600
  description: "texto legível"

  # Detalhes por dimensão
  details:
    coverage_drop: 7.0               # GAP_COVERAGE: % de queda
    unlinked_markers: 2              # GAP_DEBT: qtd sem DDNA
    lines_without_doc: 600           # GAP_DOCS
    network_imports: ["net/http"]    # GAP_SECURITY
    line_count: 600                  # GAP_ARCH
    agent_days_since_learning: 12    # GAP_MEMORY

  # Ações
  action: "block | warn | log | silent"
  suggestions: ["sugestão 1", "sugestão 2"]

  # Integrações
  ddna_created: false
  ddna_id: ""                        # Se P0
  gate_triggered: false
  gate_outcome: ""                   # Se Gate acionado

  # Resolução
  resolution:
    status: "open | in_progress | resolved | dismissed"
    resolved_by: ""
    resolved_at: ""
    resolution_notes: ""
```

### Gap Registry (GAP_REGISTRY.md)

```markdown
## GAP-20260730-0001

| Campo | Valor |
|-------|-------|
| **Data** | 2026-07-30T10:00:00Z |
| **Dimensão** | GAP_ARCH |
| **Severidade** | MEDIUM |
| **Score** | 45 |
| **Arquivo** | `api/client.go` |
| **Descrição** | 600 linhas (threshold: 500), 14 métodos (threshold: 15) |
| **Trigger** | pre-commit |
| **Status** | open |
| **DDNA** | — |
| **Gate** | — |
| **Sugestão** | Extrair transport.go |
```

---

## QUALIDADE E MÉTRICAS

### Critérios de Qualidade

- [ ] Varredura pré-commit < 2 segundos (garantido por timeout com fallback)
- [ ] Thresholds configuráveis via YAML sem alteração de código
- [ ] Gap Score calculado deterministicamente (mesmo input → mesmo score)
- [ ] Zero falsos negativos em cenários de diff com imports de rede
- [ ] Bloqueio pré-commit reversível com `--no-verify` (mas registrado)
- [ ] DDNA gerado automaticamente para gaps P0
- [ ] Contrafactual Gate acionado para gaps arquiteturais > 80
- [ ] CLI funcional (`cosca gap scan`) com output JSON e exit codes
- [ ] CI integration via GitHub Actions (template fornecido)
- [ ] Gap silencioso registrado mesmo quando não bloqueia

### Métricas da Engine

| Métrica | Tipo | Descrição | Meta |
|---------|------|-----------|------|
| `f13.scan.count` | Counter | Total de scans executados | — |
| `f13.scan.duration_ms` | Histogram | Duração do scan | p99 < 2000ms |
| `f13.blocked` | Counter | Commits bloqueados | > 0 (prevenção) |
| `f13.gaps_by_dimension` | Counter | Gaps detectados por dimensão | — |
| `f13.gap_score.avg` | Gauge | Média do gap score semanal | < 20 |
| `f13.gap_score.max` | Gauge | Máximo do gap score semanal | < 50 |
| `f13.ddna_created` | Counter | DDNA entries criadas por gap P0 | — |
| `f13.gate_triggered` | Counter | Contrafactual Gates acionados | — |
| `f13.false_positive` | Counter | Gaps que não representavam risco real | < 10% |
| `f13.gap_resolution_time` | Histogram | Tempo médio para resolver gap | < 7 dias |
| `f13.gap_resolution_rate` | Gauge | Taxa de gaps resolvidos / fechados | > 80% |

---

## RESTRIÇÕES E GOVERNANÇA

### Restrições

1. **Performance**: Scan pré-commit NUNCA excede 2 segundos. Se exceder, libera commit com warning parcial e agenda scan completo em background.
2. **Transparência**: Todo bloqueio inclui explicação clara e sugestões acionáveis.
3. **Proporcionalidade**: Gap Score reflete impacto real — não superdimensionar.
4. **Reversibilidade**: Bloqueio pode ser ignorado com `--no-verify`, mas o gap fica registrado.
5. **Evolução**: Thresholds mudam com o tempo — baseline histórica ajusta expectativas.
6. **Autonomia**: Cada dimensão pode ser desabilitada independentemente via config.

### Governança

| Papel | Responsabilidade |
|-------|-----------------|
| **Architecture Chief** | Dono da engine, mantém thresholds, revisa gaps recorrentes |
| **CTO** | Aprova mudanças em pesos/thresholds da engine |
| **Cada Chief de dimensão** | Responsável por resolver gaps na sua dimensão |
| **Quality Chief** | Monitora métricas da engine, reporta tendências |
| **Kernel** | Invoca engine nos pontos de gatilho definidos |

---

## RELACIONADOS

- [F1.1 DECISION_DNA_FORMAT.md](../../knowledge/architecture/DECISION_DNA.md) — DDNA entries para gaps P0
- [F1.2 Contrafactual Gate](../../workflows/contrafactual-gate.md) — Gate acionado por gaps arquiteturais
- [QUALITY_GATES.md](../../QUALITY_GATES.md) — Alinhamento com gates de qualidade
- [GAP_REGISTRY.md](../../memory/GAP_REGISTRY.md) — Registro global de gaps
- [Cognitive Gap Detection](./COGNITIVE.md) — Engine complementar de gaps de conhecimento ("não sei")
- [CONSTITUTION.md](../../CONSTITUTION.md) — P2 (código é a verdade), P4 (confiança), P5 (aprender com erros)
- [internal/embed/cosca/engines/gap-detection/config.yaml](./config.yaml) — Configuração de thresholds

---

## HISTÓRICO

| Versão | Data | Autor | Alterações |
|--------|------|-------|------------|
| 1.0.0 | 2026-07-30 | Architecture Chief | Criação inicial da F1.3 Proactive Gap Detection Engine. 6 dimensões: cobertura, dívida, docs, segurança, arquitetura, memória. Pipeline pré-commit + pós-commit. Gap Score (0-100) com bloqueio > 30. Integrações DDNA, Gate, Quality Gates. Exemplo real. |
