# COSCA TREINADOR — pipeline de treino do LoRA (Campanha 001)

> **Objetivo:** treinar um adaptador LoRA/QLoRA no **Qwen3-4B** para destilar o
> comportamento operacional do COSCA (seguir o protocolo `read→edit→verify→recover`),
> mantendo as invariantes de evidência (read→edit=1.00, 0 violações críticas).

## Arquivos

| Arquivo | Papel |
|---|---|
| `lora_qwen3_4b.py` | Script de treino (unsloth/QLoRA) — pronto para Colab/NVIDIA |
| `(dataset SFT)` | `.cosca/dataset-train.sft.jsonl.{train,val}.jsonl` (gerado pelo `cosca dataset convert`) |

## Como rodar (Colab / GPU NVIDIA)

**1. Instale o unsloth:**
```bash
pip install "unsloth[colab-new] @ git+https://github.com/unslothai/unsloth.git"
```

**2. Valide o dataset e a config (dry-run, sem treinar):**
```bash
python train/lora_qwen3_4b.py --data .cosca/dataset-train.sft.jsonl.train.jsonl --dry-run
```
> Isso confirma que o dataset carrega e a config é válida, sem gastar GPU.

**3. Treine (exige GPU NVIDIA com ~8GB+ VRAM):**
```bash
python train/lora_qwen3_4b.py --data .cosca/dataset-train.sft.jsonl.train.jsonl \
  --epochs 3 --lora-r 16 --lora-alpha 32
```
> Salva o adaptador em `./output/cosca-qwen3-4b-lora` e exporta GGUF (q4_k_m)
> para uso no Ollama.

## Dataset (o que entrou no forno)

- **treino**: 18 exemplos SFT (positivos: SUCCESS + RECOVERY_SUCCESS)
- **validação**: 2 exemplos SFT
- **origem**: `dataset-train.jsonl` (30 exemplos — 20 positivos + 10 contrastes)
- Os **contrastes** NÃO entram no SFT (seriam material de preferência/DPO).

## Mapa de dados (campanha 001)

```
dataset-train.jsonl (30)  →  converte  →  SFT (20 pos)  →  split 18 train / 2 val
   (16 SUCCESS + 4 RECOVERY)                (18 train)
   (9 FAILURE + 1 FALSE_COMPLETION)  ──────> contraste (NÃO entra no SFT)
```

## Próximo passo (após treinar)

1. Aplicar o adaptador LoRA no Qwen3-4B.
2. Rodar `cosca dataset golden` → **AFTER**.
3. Comparar **BEFORE (0.88) × AFTER** via `CheckPromotion`.
4. Se `0.88 → 1.00` mantendo 0 críticas: investigar overfitting comportamental
   (prova inédita/oculta).

## Decisão de plataforma (o bloqueio atual)

- **Recomendado:** Colab/NVIDIA (unsloth nativo, treino em minutos).
- Alternativa local: Linux + ROCm (a 6700 XT funciona no Linux; exige dual-boot/WSL).
- CPU (torch atual): inviável para LoRA de 4B (lentíssimo).
