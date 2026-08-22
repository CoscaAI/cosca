# MODEL PROTOCOL — O coração da IA (modelos, tarefas, GPU, hardware)

> **Versão**: 1.1.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — o núcleo do Cosca, finalmente documentado
> **Propósito**: referência operacional ÚNICA para a orquestração de IA: qual modelo
> roda qual tarefa, onde (CPU/GPU/remoto), com qual custo.

---

## 1. A ESPINHA — as 18 tarefas canônicas (AI Task Engine)

O Cosca normaliza TUDO em **18 tarefas** (§4): `text_generation`,
`image_generation`, `speech_to_text`, `text_to_speech`, `segmentation`,
`upscale`, `transcription`, `embedding`, `classification`, ... Cada tarefa tem
contrato (input/output/hardware/executores).

```bash
cosca task list            # as 18 tarefas
cosca task info speech_to_text   # contrato de uma
```

**Regra**: toda execução de IA é uma TAREFA primeiro, um modelo depois. O
modelo é o executor; a tarefa é a intenção.

---

## 2. O REGISTRY — modelo ↔ tarefa (`.cosca/models/index.yaml`)

Cada modelo declara: provider, id, version, formato (safetensors/gguf/onnx),
quantização (fp16/int8/int4), requisito de VRAM, as TASKS que executa e a
licença (§35).

```bash
cosca model add whisper --provider openai --version large-v3 --task speech_to_text
cosca model list [--task <tipo>] [--kind local|remote]
cosca model info <key>
cosca model remove <key>
```

**O registry liga modelo → tarefa**: para qualquer task, o engine sabe qual
modelo do projeto a executa.

---

## 3. O DESTINO — GPU Engine decide ONDE roda

O scheduler cruza a tarefa com o hardware e decide (§28:
performance/custo/disponibilidade/qualidade):

```bash
cosca gpu probe     # tabela de capacidade da GPU (vendor, VRAM, driver, ROCm/CUDA)
cosca gpu plan      # decide: local-cpu | local-gpu | remote
cosca hardware probe  # CPU + RAM + GPU
```

- **Probe é READ-ONLY** — sem sudo, sem instalar driver.
- **Compute target** (gfx1030, sm_80) define o que a GPU aguenta.

---

## 4. A CAPACIDADE — o que o Cosca consegue (L0-L3)

```bash
cosca capability level   # nível atual + capacidades disponíveis/indisponíveis
```

| Nível | Significado |
|-------|-------------|
| **L0** | Determinístico (sem IA: regras, memória, workflow) |
| **L1** | Retrieval (IA opcional: busca, classificação, extração) |
| **L2** | Raciocínio (modelo disponível: análise, código, diagnóstico) |
| **L3** | Execução autônoma (modelo + runtime: planeja → executa → aprende) |

---

## 5. A REGRA DE OURO — local primeiro, nuvem só se valer

1. **Resolve sem IA se puder** (L0/L1) — knowledge + search + cache.
2. **Se precisa de IA, local primeiro** (ollama/qwen, grátis, na GPU da casa).
3. **Nuvem (openai/deepseek) só quando** local não aguenta (VRAM/qualidade/task).
4. **NUNCA enviar o contexto do cérebro à nuvem.** Identidade, memória,
   conhecimento e conversas são propriedade da família — a nuvem é um túnel
   que entra e sai do cofre sem ninguém ver (L46: a casa já foi ferida por
   API keys). Chamadas remotas só com payload **sanitizado**, **aprovação
   explícita do Don por sessão** e trilha de auditoria. Sem exceção.

O BUDGET_PROTOCOL disciplina o custo; o MODEL_PROTOCOL escolhe o executor.

---

## 6. GOTCHAS

1. **Modelo sem task = enfeite** — registrar modelo sem ligar à task é morto.
2. **VRAM é o limite** — quantização (int8/int4) existe pra caber; não force o
   que não cabe (L16: RX 6700 XT vs RTX 5060 Ti).
3. **Probe é read-only** — nunca instala driver nem pede sudo.
4. **Fallback obrigatório** — se o provider falhar, o engine tem que cair pro
   determinístico (nunca travar a operação).

---

## 7. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.1.0 | 2026-08-16 | §5.4 — portão de SEGURANÇA: nuvem nunca com contexto do cérebro; sanitização + aprovação do Don + auditoria (ordem do Don, após revisão do risco) |
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — o coração da IA documentado |
