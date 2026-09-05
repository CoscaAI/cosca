# Cosca Benchmark Contract (F9 — Hardware & Performance Brain)

> O contrato de benchmark do Cosca — a camada que produz **MEASURED** sem
> nunca alterar **FACT**. Fase 9 do Hardware Brain (L319, L316, L317, L320).

## 1. A LEI

> **"Benchmark não altera FACT. Benchmark produz MEASURED."** (L319)

Se o FACT diz 31 GB RAM e o benchmark mede 37 GB/s (e amanhã 42), a RAM não
virou 42 GB/s — o MEASURED mudou. Esta separação é o que evita corrupção
epistemológica: o conhecimento do Cosca só muda com evidência, nunca com
repetição ou palpite.

## 2. O CONTRATO (L316)

O executor/agente não inventa métricas a cada rodada — segue o contrato fixo:

```
INPUT      → workload + dataset + hardware + restrições
EXPERIMENT → UMA variável por vez
OUTPUT     → latência, throughput, bandwidth, allocations, variance, confidence
DECISION   → KEEP / REJECT / INCONCLUSIVE
```

Regras de disciplina:
- **Uma mudança por vez** — nunca combinar otimizações na mesma medição.
- **Múltiplas amostras** — nunca declarar vitória por uma execução.
- **Diferença mínima** — abaixo do limiar (default 2%) = INCONCLUSIVE (ruído).
- **Hipótese explícita** — cada experimento declara o que espera observar;
  refutação também é vitória (elimina direção com dado real).

## 3. Classes epistemológicas (L317)

| Classe | Origem | Exemplo |
|--------|--------|---------|
| `fact` | o sistema declarou | CPU = Ryzen, RAM = 31 GB |
| `measured` | o benchmark observou | 12.4 Mvec/s, ~38 GB/s |
| `inferred` | o sistema concluiu | provável memory-bound |
| `evidence` | prova registrada | benchmark id, commit, fingerprint |
| `profile` | perfil consolidado | hardware + workload + evidência |
| `decision` | decisão tomada | status superseded quando contradita |

**Invariante (L320)**: uma etapa não fabrica evidência para a anterior.
Um MEASURED não vira FACT por declaração — só por **execução observada**.

## 4. Memória experimental (L316)

Experimentos **REJEITADOS** são guardados (JSONL append-only, como o
chain.dat). Um futuro agente que pensar "vamos pré-calcular normas!" consulta
o histórico e encontra o registro REJECTED — não gasta tokens redescobrindo.

```
EXPERIMENTO ENCONTRADO
  Norms · 100K × 768
  Baseline: 12.4 Mvec/s · Norms: 10.6 Mvec/s · Delta: -14.5%
  STATUS: REJECTED · MOTIVO: overhead de memória supera benefício
```

## 5. Confiança estatística (L316)

- mean / min / max / stddev / CoV (coeficiente de variação)
- p50 / p95 / p99 (interpolação linear arredondada)
- Confidence: high (CoV<5%) / medium (<15%) / low (≥15%) / inconclusive

Nunca "NOVO RECORDE!" por 0.01 de diferença — o CoV e o limiar de ruído
decidem.

## 6. Modo conservador (L316)

> *"Performance optimization is optional. Correct execution is mandatory."*

Se o Brain não sabe → configuração segura. Inconclusivo → configuração
conhecida. GPU duvidosa → CPU fallback. O sistema nunca fica incapaz de
executar por não conseguir otimizar.

## 7. Uso

```go
r := performance.NewRunner()
res, err := r.Run(
    performance.WorkloadProfile{Name: "vector-search", Dimension: 768, DatasetSize: 100000},
    performance.Experiment{Name: "workers", Value: "12", Hypothesis: "mais workers melhora"},
    baselineFn, // funções que medem UMA operação
    candidateFn,
    "autopsy-2", // proveniência
)
// res.Decision: keep | reject | inconclusive
store.Record(res) // guarda inclusive os rejeitados
```

## 8. O que NÃO é

- Não é o harness de evals (`internal/benchmark/` — qualidade de resposta de
  agentes). Este é microbenchmark de **performance**.
- Não executa benchmarks sozinho — só provê o contrato e o runner.
- Não altera produção — produz recomendação com confiança (o runtime decide
  dentro da política de segurança).
