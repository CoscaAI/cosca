# COSCA — Auditoria Científico / Realtime / Vision-Action / Media

> **Data**: 2026-09-05
> **Autor**: Cosca Kernel + Auditor de Arquitetura (cosca-architecture)
> **Método**: READ-ONLY — leitura de código-fonte (`internal/*.go`, `laboratory/*.py`) + execução de build/test. Nenhum arquivo alterado.
> **Princípio**: código executado é a verdade absoluta (P2). Docs são a intenção; código é o fato.

---

## 1. Resumo executivo

| Verificação | Resultado |
|-------------|-----------|
| `go build ./...` dos pacotes do escopo | ✅ OK, sem erro |
| `go test -count=1` (sciengine, deliberate, decision, evidence, stallwatch, concurrency, visionact, grounding, compute, durable, task, aitask, training, screen, voice, cli) | ✅ Todos PASS |

**Fragmento crucial (honesto):** o cluster **Científico/Realtime/Vision-Action** é uma mistura de **primitivas reais e bem construídas** (stallwatch, durable, compute, grounding, deliberation determinística) com **contratos que ainda não foram conectados** (sciengine = registro passivo sem reproducibilidade; visionact = só perceber, não age; training = handoff sem orquestração). Nenhum é stub no sentido de "código vazio", mas vários são **primitivas aguardando integração** — e o Don deve saber disso.

---

## 2. O Scientific Engine — o ponto mais crítico

`internal/sciengine/sciengine.go`

**O que é:** um **registry YAML passivo** — CRUD sobre `<projeto>/.cosca/experiments.yaml`. Cada experimento registra INPUT · PARAMETERS · CODE VERSION · MODEL VERSION · ENVIRONMENT · RESULT · METRICS · TIMESTAMP, com `ResultKind` canônico (observed/calculated/simulated/generated/hypothesis, §32).

### 2.1 A verdade (path:linha)
- `sciengine.go:59` — `CodeVersion`, `:61` `ModelVersion`, `:63` `Environment` são **strings livres digitadas pelo chamador**. **Não há** captura automática do commit, do runtime, nem hash de entrada. `:86` Timestamp gravado em `New()`, não no momento do resultado.
- **Reproducibilidade = contrato/registro, NÃO garantida pelo código.** Não há re-execução de verificação nem validação de que o `Kind` corresponde ao resultado.
- **Não há comando CLI ligando o sciengine**: busca por `NewSciEngine`/`experiment` em `internal/cli/root.go` não retorna registro. Só existe o template `experiment.json` em `internal/cli/project_examples.go:81`. Ele é re-exportado via `pkg/engine/engine.go:298-333`.

### 2.2 Veredito honesto
> Se **reproducibilidade científica** é um requisito real para o Don, o sciengine **não a entrega hoje** — é um cadastro. Para garantir, seria preciso: captura automática de env + commit, hash de entrada, e re-run de verificação. **Nada disso está implementado.** O código atual apenas registra o que o usuário declara.

---

## 3. Vision-Action: perceber → decidir → agir

`internal/visionact/visionact.go`, `internal/cli/voice_chat_deliberate.go`

### 3.1 A verdade
- `visionact.go:167-185` — `LookAtScreen` (perceber) é real.
- `voice_chat_deliberate.go:228-262` — `respondWithDeliberation` usa LLM para deliberar.
- **"Agir" = gerar uma fala (TTS).** Não há loop de ação sobre o mundo/ferramentas.
- **O loop perceber→decidir→agir existe só PARCIALMENTE:** perceber (real), deliberar (LLM), agir (apenas fala). Não há ação corpórea/física.

### 3.2 DESVIO no pipeline de visão (`internal/worldmodel/vision`)
- `pipeline.go:173-180` — Step 2 "Classify each detection (CLIP)" é **no-op**: monta `candidates` e faz `_ = candidates`. A classificação CLIP zero-shot **nunca roda**.
- `adapters.go:212-214` — `SAM.Segment` exige `PromptPoint`, que o `DefaultPipelineConfig` (`pipeline.go:57-67`) **não seta**; mesmo setado, o código **ignora o ponto** (`_ = img`, builda só tensor de imagem, sem `point_coords`). **O SAM degrada sempre (vazio)**.
- Comentário obsoleto em `pipeline.go:13` ("compunica com Python subprocesses") **contradiz** o motor ONNX nativo Go real. Documentação desalinhada do código.

---

## 4. Realtime / Durabilidade / Concorrência (os fortes)

| Pacote | #src | #test | Real? | Path:linha |
|--------|-----:|------:|-------|-----------|
| `internal/stallwatch` | 1 | 2 | ✅ **Real, wired** | timeout + retry + backoff jitter + fallback; wired em `orchestration/executor.go:1072` para `llm.chat` |
| `internal/durable` | 1 | 2 | ✅ **Real, wired** | fencing/lease/idempotency/migração; wired via bootstrap + pipeline_wiring |
| `internal/concurrency` | 1 | 2 | ⚠️ Real, **mas wired em 1 ponto só** | `semantic_router.go:139` |
| `internal/compute` | 17 | 15 | ✅ **Real** | Compute Fabric — scheduler, pools, backpressure |

**Leitura:** os garantidores de realtime **existem e funcionam**. `stallwatch` e `durable` estão **efetivamente conectados** ao runtime. `concurrency` existe mas só é usado em **1 ponto** — não está generalizado em todo o sistema.

---

## 5. Raciocínio científico / Deliberação

| Pacote | #src | #test | Real? | Nota |
|--------|-----:|------:|-------|------|
| `internal/deliberate` | 6 | 1 | ✅ Real | `deliberation.go:218-248` gates determinísticos zero-LLM (convergence/confidence/emit), **integrados** ao Deliberator. `Enabled=false` por padrão (fail-closed). |
| `internal/decision` | 1 | 1 | ⚠️ Standalone | `decision.go:102-137` log event-sourced real, mas `NewDecisionLog(nil)` só nos testes; adapter de persistência "vive na borda" — **não existe no código** |
| `internal/evidence` | 2 | 2 | ⚠️ Conceito duplo | `internal/evidence` (ObservationState+ReplayHash, usada em `evals/world.go:300`) ≠ `internal/cli/evidence.go` (aquisição externa→quarantine→knowledge) |
| `internal/provenance` | 1 | 1 | ⚠️ Passivo | `provenance.go:26-156` registry YAML real via `cosca provenance`, mas **registra o que o usuário declara**, sem verificar correspondência kind/resultado nem licença real (§35) |

---

## 6. Training (pipeline LoRA)

| | Real? | Path |
|-|-------|------|
| `internal/training/job.go` | ⚠️ **Só contrato** | Job manifest / WorkerReport — sem execução |
| `laboratory/*.py` (`lora_qwen3_4b.py`, `merge_gguf.py`, `curate.py`, `gen_worker.py`, `worker_colab.py`, `worker_self_contained.py`, `colab_auth.py`) | ✅ **Reais e funcionais** | importam torch/transformers/peft/trl/unsloth, têm `main` |

**Verdade:** os scripts Python de treino LoRA existem, têm `main()`, e importam as bibliotecas certas — **mas estão fora do binário Go e nada no Go os invoca.** É um handoff sem orquestração.

---

## 7. Perception / Screen / Voice (os "soldados" reais)

- `internal/screen` (10 src/6 test): **real** — captura GDI + CLIP + métricas estéticas + OCR (via subprocesso PowerShell no Windows).
- `internal/voice` (8 src/2 test): **real** — TTS concatenativo de dífonos (Go puro).
- `internal/grounding` (9 src/7 test): **real** — grounding linguagem→mundo.

---

## 8. CLI: o que está wired

Reais: `eval`, `benchmark`, `dataset`, `confidence`. ⚠️ `benchmark` usa `diagnostics.RunAll` como proxy de "index/memory benchmark" (naif). **Nenhum** usa o sciengine.

---

## 9. Tabela-resumo de honestidade

| Componente | #src | #test | Implementação real | Estado | Nota honesta |
|------------|-----:|------:|--------------------|--------|--------------|
| sciengine | 1 | 1 | Registry YAML passivo | ⚠️ Contrato | **Não garante reproducibilidade** — é cadastro |
| visionact | 1 | 1 | Perceber+deliberar | ⚠️ Parcial | "Agir" = só TTS; sem ação no mundo |
| worldmodel/vision | 7 | 4 | ONNX nativo Go real | ⚠️ Desvios | CLIP-classify no-op; SAM degrada sempre |
| deliberate | 6 | 1 | Gates determinísticos | ✅ Real | `Enabled=false` por padrão |
| decision | 1 | 1 | Event-sourced log | ⚠️ Standalone | Sem adapter de persistência no código |
| evidence | 2 | 2 | ObservationState+ReplayHash | ⚠️ Duplo conceito | ≠ `cli/evidence.go` |
| provenance | 1 | 1 | Registry YAML | ⚠️ Passivo | Não verifica kind/resultado/licença |
| stallwatch | 1 | 2 | Timeout+retry+backoff | ✅ Real, wired | wired em `executor.go:1072` |
| durable | 1 | 2 | Fencing/lease/idempotency | ✅ Real, wired | wired via bootstrap |
| concurrency | 1 | 2 | Primitivas | ⚠️ 1 ponto | Só `semantic_router.go:139` |
| compute | 17 | 15 | Compute Fabric | ✅ Real | Scheduler, pools, backpressure |
| grounding | 9 | 7 | Grounding | ✅ Real | — |
| screen | 10 | 6 | Captura GDI+CLIP+OCR | ✅ Real | OCR via subprocesso PowerShell |
| voice | 8 | 2 | TTS concatenativo | ✅ Real | dífonos Go puro |
| training/job.go | 1 | 1 | Contrato Job | ⚠️ Só contrato | Nenhum Go chama os .py |
| laboratory/*.py | — | — | LoRA real | ✅ Real | Fora do binário, sem orquestração |

---

## 10. Conclusão honesta

O COSCA, no cluster científico/realtime/vision-action, **não é um monólito pronto** — é um conjunto de **primitivas fortes** (stallwatch, durable, compute, grounding, deliberação determinística) cercadas por **contratos ainda não fiados** (sciengine sem reproducibilidade, visionact que não age, training sem orquestração, decision sem persistência wired, provenance passivo).

**O que o Don deve saber antes de levar ao mundo:**
1. **Reproducibilidade científica NÃO está garantida** — sciengine é um registro, não um verificador. Se o valor central é científico, isso é o gap nº 1.
2. **Vision-action não age sobre o mundo** — só percebe e fala. O "agir" físico requer integração com a ponte Unreal/Blender (que está em outro cluster).
3. **Os garantidores de realtime são reais e conectados** — stallwatch e durable são os pilares confiáveis.
4. **O pipeline LoRA existe mas é um handoff** — os scripts estão em `laboratory/`, funcionais, mas o Go não os orquestra.

---

## 11. Referências

- `internal/sciengine/sciengine.go` (registry YAML)
- `internal/visionact/visionact.go` (perceber→deliberar)
- `internal/worldmodel/vision/pipeline.go`, `adapters.go` (ONNX; CLIP no-op, SAM degrada)
- `internal/deliberate/deliberation.go` (gates determinísticos)
- `internal/stallwatch/`, `internal/durable/`, `internal/concurrency/` (realtime)
- `internal/compute/` (Compute Fabric)
- `internal/grounding/`, `internal/screen/`, `internal/voice/` (percepção real)
- `laboratory/*.py` (pipeline LoRA)
- `internal/training/job.go` (contrato)
- `internal/cli/{eval,benchmark,dataset,confidence}.go` (CLI wired)
