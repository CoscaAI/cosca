# CHECKPOINT DE CONTINUIDADE — COSCA / datasetgen (colmeia)

> **Propósito:** se a sessão sair do controle (crash, limite de passos, interrupção),
> esta sessão—ou um kernel retomado—deve ler ESTE arquivo e retomar exatamente daqui.
> NÃO inventar estado. Este é o registro canônico do "onde paramos".
>
> **Atualizado:** 2026-09-03 (sessão do datasetgen + tese da colmeia)
> **Sessão anterior concluída:** investigação do "pedreiro não constrói" (ver learnings.md)

---

## 🎯 OBJETIVO ATIVO DA FAMÍLIA
**Distillation de um modelo pequeno que siga o protocolo de tool-call do COSCA.**
Estratégia: DeepSeek-V4 (professor) gera dataset → LoRA num 8B (base = `qwen3:4b`)
→ avaliar antes/depois. Essa é a **PRIMEIRA CÉLULA** da **colmeia** (modelos pequenos
especializados coordenados pelo runtime).

---

## ✅ FEITO E VALIDADO NESTA SESSÃO

### 1. Bancada de modelos (medida, não chutada)
| Modelo | Tok/s geração | Tool-call | Veredicto |
|---|---|---|---|
| `qwen3:4b` (Q4) | **93.4** | ✅ PERFEITO (round 1 já lê) | **BASE ESCOLHIDA** |
| `qwen2.5-coder:3b` | **119.1** | ⚠️ JSON solto no round 1 | Alternativa veloz |
| `qwen2.5-coder:latest` (8B) | 45.7 | ⚠️ oscila | reserva |
| `qwen3:8b` | ~44 | — | reserva |
| `deepcoder:1.5b` | — | ❌ prosa (abaixo do piso) | descartado |

- **Espaço:** ~45.9 GB livre. Ollama com qwen3:4b, qwen2.5-coder:3b/8b, qwen3:8b, deepcoder:1.5b, nomic-embed-text.

### 2. Pacote `internal/datasetgen/` — A FÁBRICA DE DADOS (criado e COMPILANDO)
- **`SPEC.md`** — design das 8 regras do professor.
- **`types.go`** — esquema: `Label` (6 classes), `Focus` (6 categorias), `Example` (trajetória completa + label), helpers de análise.
- **`synthetic.go`** — gerador procedural determinístico (6 linguagens, 4 focos, distribuição ponderada 40/25/15/10/10).
- **`runner.go`** — o core: monta executor canônico (sandbox + policy + EVIDENCE GATE), cria workspace isolado por exemplo, roda o loop de tool-call com o modelo, captura trajetória e classifica.
- **`datasetgen_test.go`** — 6 testes, TODOS PASSANDO:
  - `TestGenerateBatchDistribution` (distribuição + variedade de linguagens) ✅
  - `TestClassifyLabels` (5 labels: SUCCESS, RECOVERY_SUCCESS, FAILURE, PROSE_INSTEAD_OF_ACTION, FALSE_COMPLETION) ✅
  - `TestGenerateExampleE2E` (opcional, roda com `COSCA_DATASETGEN_RUN_EVAL=1`)

### 3. PILOTO E2E (qwen3:4b) — a fábrica PRODUZIU o que precisamos
- Exemplo 0 (happy_path): `FAILURE` mas tools=[read_file, edit_file] → **OURO para contraste** (leu/editou, não atingiu expected).
- Exemplo 2 (recovery): `RECOVERY_SUCCESS` (list_dir→read_file) → **o "reflexo" que queremos destilar**.
- **OBSTÁCULO:** 2 timeouts no Ollama (`context deadline exceeded`) — janela 32768 grande demais pro qwen3:4b.
- **Correção identificada:** aumentar timeout por passo (>180s) OU reduzir janela (num_ctx). NÃO é bug do gerador.

### 4. TESE DA COLMEIA (arquitetural, aprovada pelo professor e Don)
- **NÃO precisa de um modelo gigante.** Ecossistema de modelos pequenos ESPECIALIZADOS, cada um ótimo em UM comportamento.
- **COSCA Runtime = sistema nervoso** (tools + memory + evidence) que os conecta.
- **Router de intenção** (detectTaskType/IsActionIntent) decide QUEM trata cada task.
- **Regra:** não se treina "qualquer coisa" — especializa-se um base que JÁ domina a modalidade (visão→VLM, áudio→base audio, código→coder base).
- **datasetgen (código) = primeira célula.** Visão/STT/TTS seguem o mesmo padrão com base + dataset diferentes.

---

## 🧱 ARQUIVOS NOVOS (não commitados ainda)
```
internal/datasetgen/SPEC.md
internal/datasetgen/types.go
internal/datasetgen/synthetic.go
internal/datasetgen/runner.go
internal/datasetgen/datasetgen_test.go
```
> **TODOS NOVOS.** Precisam de `git add` + commit.

---

## ⚠️ OBSTÁCULOS ABERTOS / DECISÕES PENDENTES
1. **Plataforma de treino do LoRA** — NÃO decidida. A 6700 XT é AMD; torch atual = build de CPU (`2.13.0+cpu`); ROCm/hipcc NÃO instalados. Opções:
   - (a) Nuvem/Colab NVIDIA (unsloth nativo, rápido) → exportar LoRA → aplicar local.
   - (b) Linux + ROCm (a 6700 XT funciona bem p/ treino no Linux).
   - (c) CPU (já instalado, mas MUITO lento p/ 8B LoRA).
2. **Timeout/janela do E2E** — necessário ajustar piloto (timeout >180s por passo OU num_ctx menor).
3. **Integrar professor** — o caminho `GenerateFromProfessor` (DeepSeek-V4) está só documentado no SPEC, NÃO implementado.
4. **Testes de visão/áudio** — células futuras, fora do escopo atual.

---

## 🎯 PRÓXIMO PASSO EXATO (ao retomar)
1. Ajustar `DefaultGeneratorConfig().Timeout` (para >180s) e/ou `opts.NumCtx` (de 32768 para menor, ex.: 8192) no `runner.go`.
2. Rodar E2E de novo: `$env:COSCA_DATASETGEN_RUN_EVAL="1"; go test ./internal/datasetgen/ -run TestGenerateExampleE2E -v -count=1 -timeout 900s`.
3. Validar que ETÁ produzindo `SUCCESS` e `RECOVERY_SUCCESS` consistentes (não só FAILURE).
4. `git add internal/datasetgen/` + commit ("feat(datasetgen): fabrica de dados de treinamento + classificador de 6 classes").
5. Implementar `GenerateFromProfessor` (DeepSeek-V4) e decidir plataforma de treino.

---

## 🧠 REFERÊNCIAS
- Ver `internal/datasetgen/SPEC.md` para o design completo (8 regras do professor).
- Ver `learnings.md` → bloco "2026-09-03 - FABRICA DE DADOS (datasetgen) + TESE DA COLMEIA".
- Commits anteriores da investigação do pedreiro: `64ffd364` (último) até `2c37940f`.
