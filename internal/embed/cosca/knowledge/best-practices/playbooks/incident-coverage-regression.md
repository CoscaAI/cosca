---
id: playbook-004
title: "Resposta a Regressao de Cobertura de Testes"
type: incident-response
severity: critical
owner: cosca-qa
created: 2026-07-29
tags:
  - testing
  - coverage
  - ci
  - threshold
  - jail
  - go
  - regression
related_incidents:
  - audit/coverage-audit-2026-07-29
  - memory/testing/coverage.md (threshold crisis)
  - memory/qa/quality-gates.md (G5: coverage < 70%)
  - kernel L19: CLI Coverage Breakthrough (2026-07-29)
  - kernel L18: Runtime Coverage Audit (2026-07-29)
---

# Resposta a Regressao de Cobertura de Testes

## Trigger

**CI falha no Gate G5** — qualquer um destes eventos dispara o playbook:

1. `make coverage-check` retorna coverage global < 70% (threshold canônico unificado, audit 2026-07-29)
2. `.github/workflows/ci.yml` falha no job `test-coverage`
3. Alerta manual: auditoria periódica detecta pacote crítico abaixo do piso

**Contexto real**: Em 2026-07-29, uma auditoria completa do runtime (Don's order) revelou 4 thresholds conflitantes para o mesmo gate — Makefile (40%), CI (55%), docs (70%), embed (80%). O CI executava a 55% com baseline real de ~78%. O playbook assume o threshold unificado de 70%.

## Passos

### Passo 1: Identificar quais pacotes cairam

```bash
# Cobertura por pacote com breakdown de funcoes
go tool cover -func=coverage.out | grep -E "^github.com/CoscaAI/cosca/" | awk '{print $1, $NF}'

# Para ver apenas pacotes abaixo do threshold
go tool cover -func=coverage.out | grep -v "100.0%" | grep -v "total:" | sort -t: -k2 -n

# Ou via target do Makefile para todos os pacotes
make test-unit -race 2>&1 | grep "coverage:"
```

**Padrão**: A auditoria de 2026-07-29 usou `go tool cover -func` para analisar 114 funções em 5 pacotes. Funções com 100% eram 104/114 (91.2%). Apenas 2 funções abaixo de 80% no core runtime.

**Output esperado**: Lista de pacotes com cobertura percentual, ordenada do menor para o maior.

### Passo 2: Isolar o commit causador

```bash
# Bisect para encontrar o commit que introduziu a queda
git bisect start
git bisect bad HEAD
git bisect good <ultimo-commit-com-ci-verde>

# Em cada passo do bisect, rodar cobertura do pacote afetado
make test-unit -race 2>&1 | grep -E "coverage:.*<previous_value>"

# Marcar bom/ruim
git bisect good  # ou git bisect bad
```

**Padrão real**: No kernel L19 (2026-07-29), a CLI estava travada em 68.5% — abaixo do threshold de 70%. O problema não era um commit específico, mas um padrão estrutural: o monolito `runServe` (699 linhas) impedia cobertura significativa.

### Passo 3: Categorizar — codigo novo sem teste vs refactor que removeu teste

```bash
# Ver diff do commit causador
git diff <commit>^..<commit> -- '*_test.go' | head -50

# Verificar se remocoes de teste foram intencionais
git log --all --full-history -- "*_test.go" | head -20
```

Classifique a regressão em uma de duas categorias:

| Categoria | Sintoma | Exemplo real |
|-----------|---------|-------------|
| **Codigo novo sem teste** | Novas funções/lógica sem `_test.go` correspondente | `jail.go`: 6 funções a 0% (InsideJail, ReexecInJail, createJailBinary, createMemfd, createTempFile, loadConstraints) — audit 2026-07-29 |
| **Refactor que removeu cobertura** | Testes existentes foram deletados ou alterados, cobertura do pacote caiu | `serve.go`: refactor de runServe alterou assinatura interna, testes antigos quebraram e não foram atualizados — kernel L19 |
| **Monolito estrutural** | Função ou arquivo muito grande (500+ linhas) que nunca teve boa cobertura | `runServe` (699 linhas) a 0.5% antes do refactor — kernel L19 |

### Passo 4: Se codigo novo — aplicar padrão extrair-para-testar

**Padrão comprovado** (kernel L19, 2026-07-29):

```bash
# Identificar blocos extraiveis no arquivo problematico
# Procure por: inicializacao de config, resolucao de paths, parsing de env
grep -n "func\|os.Getenv\|path.Join\|flag\." <arquivo_problematico>

# Extrair cada bloco como funcao independente
# Exemplo real (de serve.go):
# Antes: runServe() de 699 linhas com .env loading inline
# Depois: loadDotEnv() (15 linhas), resolveDataDir() (10 linhas), configureCORSFromEnv() (3 linhas)
```

**Regra comprovada**: 
- Função pequena (10-20 linhas) = trivial de cobrir
- Função grande (500+ linhas) = impossível de cobrir
- Extraia antes de testar — NUNCA tente "testar para cobrir" sem refatorar

```bash
# Apos extrair, escrever testes table-driven para cada funcao
# Exemplo de teste para funcao extraida:
# TestLoadDotEnv: .env existe, .env nao existe, .env malformado
# TestResolveDataDir: padrao, custom, path relativo, path absoluto

# Rodar cobertura apos cada extracao
go test -coverprofile=coverage.out ./<pacote>/ && go tool cover -func=coverage.out | grep "total:"
```

**Resultado real**: `runServe` saltou de 0.5% → 61.8%, CLI total de 68.5% → 71.5%. 3 funções extraídas de 10-15 linhas cada destravaram todo o pacote.

### Passo 5: Se refactor — restaurar testes equivalentes

```bash
# Listar funcoes que perderam cobertura apos refactor
diff <(git show HEAD~1:coverage_old.txt) <(cat coverage_new.txt)

# Para cada funcao afetada, verificar se o teste antigo ainda compila
go test -run "^Test<NomeFuncao>$" ./<pacote>/

# Reconstruir testes equivalentes cobrindo a nova assinatura
# ATENCAO: testes devem validar COMPORTAMENTO, nao implementacao
```

**Exemplo real** (kernel L18, 2026-07-29): `Start()` (78.3%) e `Restart()` (78.6%) — as funções existem com testes, mas caminhos de erro de inicialização de subsistemas não são exercitados. O refactor que alterou a assinatura de inicialização não atualizou os testes de cenários de falha.

### Passo 6: Verificar com suite completa

```bash
# Suite completa com race detector (obrigatorio)
make test-unit -race

# Coverage check com threshold unificado (70%)
make coverage-check

# Verificar coverage por funcao no pacote corrigido
go tool cover -func=coverage.out | grep "<pacote>/"
```

**Gate de verificacao**: Todos os pacotes devem estar ≥ 70%. Se pacote de segurança (jail, auth, middleware) estiver abaixo → escalar (ver seção Escalação).

### Passo 7: Atualizar memoria de cobertura

Editar `internal/embed/cosca/memory/testing/coverage.md` com os novos valores:

```markdown
## Coverage por Pacote — Atualizado 2026-07-29

| Pacote | Cobertura | Delta |
|--------|-----------|-------|
| internal/cli | 71.5% | +3.0% |
| pkg/cosca/jail.go | 0.0% → XX% | +XX% |
```

**Padrão real**: A propria `coverage.md` estava desatualizada na auditoria de 2026-07-29 — dizia que runtime estava "High Coverage >70%" quando estava em 97.9%. A memória subestimava a qualidade real por falta de atualização.

## Escalação

| Gatilho | Escalar para | Razão |
|---------|-------------|-------|
| Pacote de segurança afetado (jail, auth, middleware) | `cosca-security` | jail.go a 0% por semanas — código de segurança sem teste é invisível (kernel L19) |
| API handler afetado (rest, grpc) | `cosca-backend` + `cosca-security` | Handlers REST a 14.5% expõem endpoints sem cobertura (audit 2026-07-29) |
| Múltiplos pacotes abaixo do threshold | `cosca-qa` + `cosca-cto` | Indica problema sistêmico, não pontual |
| Threshold conflitante (CI != docs) | `cosca-qa` + `cosca-kernel` | 4 valores diferentes encontrados em 2026-07-29 — requer correção de governança |
| Regressão > 5pp em pacote L3+ | `cosca-kernel` | Compromete qualidade de agentes com alta confiança |

## Prevenção

1. **CI já barra regressão no merge** — threshold unificado em 70% executado como gate blocking (corrigido de `continue-on-error: true` → `false` em 2026-07-29, kernel L10)

2. **Revisar PRs que reduzem cobertura** — diff coverage no PR deve ser ≥ 80% (G5.1 do quality-gates.md)

3. **Código de segurança exige teste no PR** — jail.go a 0% não pode se repetir. Regra: para todo arquivo em `pkg/cosca/jail.go`, `internal/auth/`, `api/middleware/`: PR sem teste = reject (kernel L19 meta-learning #2)

4. **Padrão extrair-para-testar como default** — monolitos são o inimigo da cobertura. Identificar funções >200 linhas como débito técnico de cobertura (scorecard item)

5. **Auditoria periódica automatizada** — script de auditoria similar ao executado em 2026-07-29 para detectar degradação antes que o CI falhe:
   ```bash
   go tool cover -func=coverage.out | awk -F'\t' '{print $2, $3}' | sort -t: -k2 -n | head -20
   ```

## Referencias

- **Auditoria**: `internal/embed/cosca/memory/audit/coverage-audit-2026-07-29.md` — 183 linhas, análise completa de 5 pacotes
- **Coverage heatmap**: `internal/embed/cosca/memory/testing/coverage.md` — linha de base atualizada
- **Quality Gates**: `internal/embed/cosca/memory/qa/quality-gates.md` — G5 e G5.1
- **Kernel L19**: `internal/embed/cosca/memory/agent/cosca-kernel/learnings.md` L19 — CLI breakthrough, padrão extrair-para-testar
- **Threshold crisis**: `internal/embed/cosca/memory/testing/coverage.md` — 4 valores conflitantes documentados
