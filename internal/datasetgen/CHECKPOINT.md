# CHECKPOINT DE CONTINUIDADE — COSCA / datasetgen (colmeia)

> **Propósito:** se a sessão sair do controle (crash, limite de passos, interrupção),
> esta sessão—ou um kernel retomado—deve ler ESTE arquivo e retomar exatamente daqui.
> NÃO inventar estado. Este é o registro canônico do "onde paramos".
>
> **Atualizado:** 2026-09-03 (sessão do treinador de modelos — fábrica + golden gate)
> **Sessão anterior concluída:** investigação do "pedreiro não constrói" (ver learnings.md)

---

## 🎯 OBJETIVO ATIVO DA FAMÍLIA
**Distillation de um modelo pequeno que siga o protocolo de tool-call do COSCA.**
Estratégia: DeepSeek-V4 (professor) gera dataset → LoRA num 8B (base = `qwen3:4b`)
→ avaliar antes/depois via **GOLDEN GATE** (régua anti-autoengano). Essa é a
**PRIMEIRA CÉLULA** da **colmeia** (modelos pequenos especializados coordenados
pelo runtime).

---

## ✅ FEITO, VALIDADO E COMMITADO NESTA SESSÃO

### 1. Metas da fábrica + treinador — COMMITADOS
- `259f783a` — **feat(datasetgen): fábrica de dados de treinamento** (gerador procedural + classificador de 6 classes + runner EVIDENCE GATE).
- `b0b9a453` — **feat(datasetgen): golden gate multicritério + promotion gate anti-autoengano**.

### 2. Bancada de modelos (medida, não chutada)
| Modelo | Tok/s geração | Tool-call | Veredicto |
|---|---|---|---|
| `qwen3:4b` (Q4) | **93.4** | ✅ PERFEITO (round 1 já lê) | **BASE ESCOLHIDA** |
| `qwen2.5-coder:3b` | **119.1** | ⚠️ JSON solto no round 1 | Alternativa veloz |
| `qwen2.5-coder:latest` (8B) | 45.7 | ⚠️ oscila | reserva |
| `qwen3:8b` | ~44 | — | reserva |
| `deepcoder:1.5b` | — | ❌ prosa (abaixo do piso) | descartado |

- **Espaço:** ~45.9 GB livre. Ollama com qwen3:4b, qwen2.5-coder:3b/8b, qwen3:8b, deepcoder:1.5b, nomic-embed-text.

### 3. Pacote `internal/datasetgen/` — A FÁBRICA DE DADOS (COMMITADO)
- **`SPEC.md`** — design das 8 regras do professor.
- **`types.go`** — esquema: `Label` (6 classes), `Focus` (6 categorias), `Example` (trajetória completa + label), helpers de invariante (`readsBeforeEdit`, `hasUnsafeMutation`, `hasTestEvidence`).
- **`synthetic.go`** — gerador procedural determinístico (6 linguagens, 4 focos, distribuição ponderada 40/25/15/10/10).
- **`runner.go`** — core: executor canônico + EVIDENCE GATE, workspace isolado, loop tool-call, classifica. RESILIENTE a timeout (num_ctx configurável = 8192 por padrão, timeout 180s).
- **`datasetgen_test.go`** — testes do classificador + distribuição.

### 4. GOLDEN GATE MULTICRITÉRIO (COMMITADO) ⭐ — anti-autoengano
- **`golden.go`** — assessoria do golden set em 7 camadas (task success, tool-call válido, read→edit correto, recovery, test/evidência, false_completion==0, unsafe_mutation==0) + **PromotionGate** (`CheckPromotion` + `PromotionCriteria` versionada).
- **`golden/golden.json`** — GOLDEN SET CONGELADO (8 casos, `frozen:true`, nunca alterar entre campanhas). Cobre happy_path/recovery/request_info/search_first em go/py/ts/json/yaml/multi.
- **`golden_test.go`** — testes do gate (violação crítica NÃO compensa, regressão reprova, saudável promove) + carregamento do golden congelado.
- **REGRA CRÍTICA (professor):** nenhuma média compensa violação crítica. Ex.: subiu 92%→95% mas `unsafe_mutation=1` → REPROVA.

### 5. BASELINE REAL MEDIDO (qwen3:4b no golden de 8 casos) — CONFIRMADO VIA CLI
```
golden gate (n=8):
  pass%       = 0.88 (7/8)      <- THIS É O "before" OFICIAL
  recovery    = 0.12
  tool_valid  = 1.00
  read_edit   = 1.00
  test_evid   = 1.00
  CRÍTICAS    = unsafe=0 false_completion=0
  Único FAILURE: go-happy-simple (não atingiu assinatura exata esperada)
```
> **Nota:** baseline ALTO (0.88) — o qwen3:4b já é muito bom no golden set.
> 100% invariante de evidência, 0 violações críticas. Espaço de melhoria
> visível: recovery (0.12) e o caso go-happy-simple. Considerar ENDURECER o
> golden set (mais casos de recovery) para dar espaço de discriminação.

---

## 🧱 PENDÊNCIA DE COMMIT (nenhuma — tudo commitado)

---

## ⚠️ OBSTÁCULOS ABERTOS / DECISÕES PENDENTES
1. **Plataforma de treino do LoRA** — NÃO decidida. A 6700 XT é AMD; torch atual = build de CPU (`2.13.0+cpu`); ROCm/hipcc NÃO instalados. Opções:
   - (a) Nuvem/Colab NVIDIA (unsloth nativo, rápido) → exportar LoRA → aplicar local.
   - (b) Linux + ROCm (a 6700 XT funciona bem p/ treino no Linux).
   - (c) CPU (já instalado, mas MUITO lento p/ 8B LoRA).
2. **Otimizar o baseline** — o golden set tende a pass% alto; para medir melhoria real do LoRA, considerar tornar o golden set mais desafiador (mais recovery) OU focar a campanha em recovery/false_completion (onde o base é fraco).
3. **`GenerateFromProfessor` (DeepSeek-V4)** — deliberadamente DELAY até depois do gate (ordem correta do professor: régua primeiro, depois professor). Documentado no SPEC, NÃO implementado.
4. **Testes de visão/áudio** — células futuras da colmeia, fora do escopo atual.

---

## 🎯 PRÓXIMO PASSO EXATO (ao retomar) — CAMPANHA 002
> A campanha 001 ESTÁ FECHADA e REPROVADA (0.88 -> 0.75). O pipeline está
> PROVADO. A campanha 002 ataca os aprendizados da 001.

### ESTADO DA CAMPANHA 001 (FECHADA — não repetir)
- ✅ Treino OK → fusão GGUF → Ollama → golden AFTER → **PromotionGate REPROVOU**
- ✅ Veredito: 0.88 (before) → 0.75 (after), caso `py-happy-simple` regrediu
- ✅ Rollback: `qwen3:4b` continua produção; `cosca-qwen3-4b-lora-001` = experimental (campaign-001/)
- ✅ Invariantes PRESERVADAS (read_edit=1.00, tool_valid=1.00, 0 críticas)
- ✅ Fix aplicado: `dataset golden` agora loga `golden gate modelo=...` (evita AFTER inválido)

### CAMPANHA 002 — PLANO (do professor + aprendizados da 001)
1. **MAIS dados** (100+, não 18): `cosca dataset generate --n 120 --eval`.
2. **Melhor distribuição de linguagens** — o `py-happy-simple` regrediu por
   faltar casos Python no treino. Garantir cobertura equilibrada (go/py/ts/json/yaml/md).
3. **Foco em RECOVERY** (0.12 é o alvo): gerar mais casos de erro→recuperação.
4. **NÃO mexer nas invariantes** (já estão em 1.00) — o treino deve focar em
   task success + recovery, não em re-ensinar read→edit.
5. Continuar com `qwen3:4b` como base; LoRA 002 = novo artefato separado.
6. Rodar BEFORE (0.88) → treinar → AFTER → `CheckPromotion` (PROMOTE se
   success>=0.88 AND criticas==0 AND tool_valid>=0.8 AND recovery>=0.2).

### DECISÃO DE PLATAFORMA (ainda pendente — usar o mesmo caminho da 001)
- ✅ Colab + unsloth FUNCIONOU (foi assim que treinamos a 001).
- ✅ Fusão FP16 + GGUF + Ollama FUNCIONOU (caminho provado).
- Manter Colab/NVIDIA para treinar a 002.

---

## 🧠 REFERÊNCIAS
- Ver `internal/datasetgen/SPEC.md` para o design completo (8 regras do professor).
- Ver `internal/datasetgen/golden.go` para o Golden Gate + PromotionGate.
- Ver `campaign-001/RESULTADO.md` para o veredito completo da campanha 001.
- Ver `train/lora_qwen3_4b.py` (treino) e `train/merge_gguf.py` (fusão GGUF).
- Ver `learnings.md` → blocos "FABRICA DE DADOS + TESE DA COLMEIA", "EVOLUÇÃO DA TESE: COSCA vira TREINADOR", "GOLDEN GATE MULTICRITÉRIO", "CAMPANHA 001" (veredito).
- Commits: `259f783a` (fábrica), `b0b9a453` (golden gate), `40f4dd00` (treino), `b615fc99` (fusão), `2577f0ba`+`c7219d31` (veredito 001).
