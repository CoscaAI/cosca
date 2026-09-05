# Oracle-Based Discovery Evaluation — Spec (cosca eval v2)

> **Version**: 1.0.0 | **Status**: spec | **Owner**: Cosca Kernel | **Created**: 2026-08-13
>
> Extensão do `cosca eval` (harness atual em `internal/evals/`) para medir **descoberta**, não reprodução. Fundamentado na análise da proposta de *problem-oracle*: o agente nunca recebe a solução de referência, só o problema + um oráculo caixa-preta que responde o mínimo.

## 1. Problema que resolve

O harness atual (`verify` = lista de comandos shell públicos) mede "o agente passou nos testes que ele pode ver". Isso mede **execução**, não **descoberta**:

- `verify` público → o agente pode *overfitar* os testes públicos.
- Sem oráculo → não há como medir "ele encontrou uma solução que eu não conhecia".
- Sem instrumentação → não dá pra distinguir "pensou" de "combinou padrão".

## 2. Conceitos

| Conceito | Definição |
|----------|-----------|
| **Oráculo** | Verificador caixa-preta. Recebe uma candidata, retorna o mínimo. O agente pode **chamar**, nunca **inspecionar**. |
| **Teste público** | `verify` — o agente vê e pode rodar. |
| **Teste secreto** | `secret_verify` — vive FORA da suíte (arquivo separado, 0600, não embed, não commit). Só o oráculo lê. |
| **Propriedade** | Critério formal que uma solução deve satisfazer (pode ser exposto em `public_properties` sem entregar a solução). |
| **Hipótese** | Cada submissão ao oráculo = uma candidata que o agente acredita ser válida. É o sinal mais confiável de "tentou algo novo". |
| **Problema-classe** | `reproduction` (A) / `discovery` (B) / `open` (C) / `existence` (D). |

## 3. Esquema YAML (extensão do `Case`)

```yaml
suite: cosca-discovery
metadata:
  author: cosca-kernel
  description: "Oracle-based discovery benchmark"
  difficulty: hard
  category: discovery
defaults:
  timeouts: { plan_seconds: 600, execute_seconds: 3600, idle_seconds: 300 }
  oracle:
    mode: interactive          # static | interactive | existence
    feedback: valid_invalid    # valid_invalid | pass_fail | property | constraint
    max_submissions: 50        # query budget (0 = ilimitado)
cases:
  - id: index-optimization
    prompt: |
      Encontre uma estratégia de indexação que reduza a latência de busca do
      knowledge.db em ≥10x, sob as restrições dadas. Você pode submeter
      candidatas ao oráculo via `cosca eval oracle submit index-optimization`.
    problem_class: discovery   # reproduction | discovery | open | existence
    verify: []                 # público: vazio = não entrega nada do segredo
    oracle:
      public_properties:
        - "latência de busca p95 < 50ms"
        - "sem perda de recall na busca semântica"
    weight: 2.0
```

**Novos campos no `Case`** (Go):

```go
type Case struct {
    // ... campos atuais ...
    ProblemClass string  `yaml:"problem_class"`  // reproduction|discovery|open|existence
    Oracle       *Oracle `yaml:"oracle"`
}

type Oracle struct {
    Mode             string   `yaml:"mode"`               // static|interactive|existence
    Feedback         string   `yaml:"feedback"`           // valid_invalid|pass_fail|property|constraint
    MaxSubmissions   int      `yaml:"max_submissions"`    // 0 = unlimited
    PublicProperties []string `yaml:"public_properties"`  // o que o agente PODE ver
}
```

## 4. Store de segredos (fora do alcance do agente)

Os testes secretos NUNCA entram na suíte pública. Vivem em:

```
.cosca/evals/secrets/<case-id>.yaml    # 0600, gitignored, não-embed
```

```yaml
case: index-optimization
reference_solution: ~            # opcional — se existir, habilita a comparação C_n vs S*
secret_verify:
  - run: go test ./internal/knowledge/ -run TestSecretLatency -v
    expect: pass
  - run: "./scripts/secret-probe.sh"
    expect: exit 0
acceptance_criteria:
  - "recall@10 == 1.0 nas 200 queries secretas"
```

- `reference_solution` presente → ao final, o harness compara a solução do agente com a referência (hash/diff) e registra `solution_class: known | novel`.
- `reference_solution` ausente → problema aberto (classe C).

## 5. Protocolo do oráculo

Novo subcomando CLI (o agente usa durante a busca):

```
cosca eval oracle submit <case-id>
```

- Roda `secret_verify` contra o workspace ATUAL.
- Retorna o sinal mínimo, conforme `feedback`:

| feedback | resposta |
|----------|----------|
| `valid_invalid` | `VALID` ou `INVALID` (só isso) |
| `pass_fail` | `t1=PASS t2=FAIL t3=PASS` (sem nomes/porquês) |
| `property` | `restricao_A=satisfeita restricao_B=violada` (sem como corrigir) |
| `constraint` | igual a property + contagem de violações |

- Contabiliza a submissão no orçamento (`max_submissions`).
- **Nunca** revela como corrigir, o `secret_verify` em si, nem a solução de referência.

`cosca eval oracle submit` também é exposto como **tool** do pipeline (bindings desktop + tool executor), para o agente poder invocar de dentro da esteira.

## 6. Instrumentação de processo (flight recorder)

O `trace` já grava `tool_start/build_start/test_start`. Adicionar ações de descoberta:

| Ação trace | Quando |
|-----------|--------|
| `oracle.submit` | agente submete candidata |
| `oracle.result` | oráculo responde (payload: VALID/INVALID) |
| `hypothesis.record` | agente registra hipótese na memória (learnings/patterns) |

Hipóteses são derivadas de **duas fontes** (redundantes):
1. **Submissões ao oráculo** (sinal primário, confiável).
2. **Registros de memória** do agente durante a busca (se ele usar a memória — mede *aprendizado operacional*).

## 7. Métricas de descoberta (`DiscoveryMetrics` no Report)

```go
type DiscoveryMetrics struct {
    TimeToFirstHypothesis string  `json:"time_to_first_hypothesis"`
    NumHypotheses         int     `json:"num_hypotheses"`         // submissões
    NumExperiments        int     `json:"num_experiments"`        // build/test runs
    NumDiscarded          int     `json:"num_discarded"`          // submissões INVALID
    DiscardRate           float64 `json:"discard_rate"`
    TimeToSolution        string  `json:"time_to_solution"`       // até 1º VALID (vazio se nunca)
    Regressions           int     `json:"regressions"`            // VALID→INVALID
    HumanInterventions    int     `json:"human_interventions"`    // sempre 0 em modo autônomo
    DiscoveryEfficiency   float64 `json:"discovery_efficiency"`   // reward / (hipóteses+experimentos+1)
}
```

**DiscoveryEfficiency** é a métrica-mestre:

```
discovery_efficiency = reward / (num_hypotheses + num_experiments + 1)
```

Dois agentes resolvem o mesmo problema; um com 500 submissões, outro com 12 → o segundo tem efficiency maior. Mede **qualidade do processo**, não capacidade bruta.

## 8. Classes de problema (A/B/C/D)

| Classe | `problem_class` | Pergunta que responde |
|--------|-----------------|-----------------------|
| A | `reproduction` | "consegue executar uma solução conhecida?" |
| B | `discovery` | "consegue achar uma solução que eu conheço mas não ensinei?" |
| C | `open` | "consegue achar uma solução que NEM EU conheço?" (sem `reference_solution`) |
| D | `existence` | "consegue decidir entre construir vs provar impossibilidade?" |

## 9. Modo `existence` (o teste mais brutal)

```yaml
  - id: impossible-problem
    problem_class: existence
    prompt: |
      Determine se existe solução que satisfaça as propriedades. Se existir,
      produza-a. Se não existir, apresente uma demonstração de impossibilidade.
    oracle:
      mode: existence
```

- O oráculo valida **duas saídas distintas**: `construct` (roda secret_verify na candidata) ou `impossibility` (roda um `verify_proof` — o agente submete uma prova textual/formal).
- **Ponto crítico**: distinguir `impossibility` (prova) de `gave_up` (desistência). O harness só aceita `impossibility` se a `verify_proof` validar; caso contrário conta como `gave_up` (falha).
- Requer um `verify_proof` no store de segredos (ex.: um checker de prova, ou verificação humana pós-hoc marcada `human_interventions=1`).

## 10. Fases de implementação (do que já existe)

| Fase | O que constrói | Depende de |
|------|---------------|------------|
| **F1** | `secret_verify` + store de segredos (`.cosca/evals/secrets/`) | `Case` + `VerifyRunner` |
| **F2** | `cosca eval oracle submit <case-id>` (CLI + tool) | F1 |
| **F3** | `DiscoveryMetrics` + instrumentação trace (`oracle.submit/result`) | F2 |
| **F4** | `problem_class` + `reference_solution` → `solution_class: known/novel` | F1 |
| **F5** | modo `existence` + `verify_proof` | F1 |
| **F6** | 3 casos concretos A/B/C (CRUD conhecido / otimização do índice / scheduling aberto) | F1-F4 |

O `cosca eval` atual (runner, suite, verify, reward, canary, timeout) já é a fundação — as 6 fases são incrementais sobre ele.

## 11. Validação do próprio oráculo (verificação do verificador)

Antes de confiar nos resultados, o harness deve rodar **um meta-caso**: uma solução que o autor SABE ser inválida e uma que SABE ser válida, contra o `secret_verify`, para provar que o oráculo discrimina (senão o oráculo tem bug e toda a medição é lixo). Incluir como `self_check` na suíte.

## 12. Limites honestos (registrar no report)

1. **Oráculo finito ≠ propriedade**: `VALID` num oráculo de N testes não prova correção — pode ser overfit. Mitigação: `public_properties` formais + testes secretos estatisticamente independentes das submissões.
2. **Descoberta ≠ prova de novidade absoluta**: registra-se apenas *"a solução não estava no conhecimento fornecido nem nas fontes acessíveis"* (afirmação testável), nunca "ninguém no mundo descobriu".
3. **`existence` é coNP-hard**: o modo D exige `verify_proof` humano/mecânico robusto, senão "impossível" vira "desisti".
