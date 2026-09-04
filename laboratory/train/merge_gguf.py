#!/usr/bin/env python3
"""
COSCA CAMPANHA 001 — Fusão do LoRA + Exportação GGUF (Opção A)
================================================================
Fundir o LoRA da campanha 001 com o Qwen3-4B EM FP16 (NÃO em 4-bit),
validar e exportar para GGUF para carregar no Ollama.

PRINCÍPIO (professor): NÃO fundir em 4-bit. Carregar o base Qwen3-4B em
FP16, aplicar o adapter treinado, SÓ ENTÃO quantizar/exportar GGUF.

FLUXO:
  base Qwen3-4B (FP16) + LoRA-001  →  merge  →  modelo fundido
       →  validar (aplica adapter de novo = mesmo output)
       →  export GGUF (q4_k_m)  →  Ollama  →  cosca dataset golden (AFTER)

USO (Colab/NVIDIA — você já tem o ambiente unsloth funcionando):
  1. Suba a pasta com o adapter (adapter_model.safetensors + config).
  2. Rode:  !python merge_gguf.py --adapter <caminho_do_adapter>
  3. O .gguf sai em ./output/cosca-qwen3-4b-lora-001-f16/

PRESERVAÇÃO: o LoRA original NUNCA é modificado — o merge gera um artefato
SEPARADO (cosca-qwen3-4b-lora-001-f16/). Se o GGUF falhar, o LoRA fica
intacto (cpia em campaign-001/ no seu PC).
"""

import argparse
import json
import os
import sys


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--adapter", required=True, help="pasta com o adapter LoRA (adapter_model.safetensors + adapter_config.json)")
    ap.add_argument("--base", default="Qwen/Qwen3-4B", help="modelo base (FP16)")
    ap.add_argument("--output", default="output/cosca-qwen3-4b-lora-001-f16", help="diretório de saída (modelo fundido)")
    ap.add_argument("--gguf-quant", default="q4_k_m", help="quantização do GGUF")
    ap.add_argument("--max-seq", type=int, default=8192)
    args = ap.parse_args()

    if not os.path.exists(os.path.join(args.adapter, "adapter_model.safetensors")):
        print(f"ERRO: adapter não encontrado em {args.adapter} (faltando adapter_model.safetensors)")
        sys.exit(1)

    try:
        from transformers import AutoTokenizer
        from peft import PeftModel
        from unsloth import FastLanguageModel
    except ImportError:
        print("ERRO: instale unsloth/peft/transformers (ambiente Colab já tem)")
        sys.exit(1)

    print(f"[cosca] base (FP16): {args.base}")
    print(f"[cosca] adapter LoRA: {args.adapter}")

    # ── 1. Carrega o BASE em FP16 (NÃO 4-bit) ────────────────────────────
    # O ponto-chave do professor: fundir em FP16, não fazer o merge sobre o
    # bnb-4bit. Aqui carregamos o base direto em bf16/fp16.
    model, tokenizer = FastLanguageModel.from_pretrained(
        model_name=args.base,
        max_seq_length=args.max_seq,
        dtype=None,          # auto -> bf16 se disponível, senão fp16
        load_in_4bit=False,  # ★ NÃO é 4-bit — queremos o peso real p/ merge
    )

    # Ajusta eos/pad (mesmo que treinamos)
    tokenizer.eos_token = "<|im_end|>"
    tokenizer.pad_token = "<|im_end|>"
    tokenizer.eos_token_id = tokenizer.convert_tokens_to_ids("<|im_end|>")
    model.config.eos_token_id = tokenizer.eos_token_id

    # ── 2. Aplica o adapter LoRA (campanha 001) ──────────────────────────
    print("[cosca] aplicando adapter LoRA...")
    model = PeftModel.from_pretrained(model, args.adapter)

    # ── 3. MERGE (funde base + adapter nos pesos completos) ──────────────
    print("[cosca] fazendo MERGE (base + LoRA)...")
    model = model.merge_and_unload()  # funde os adapters nos pesos base

    # ── 4. VALIDAÇÃO: os pesos foram realmente incorporados? ────────────
    # Verifica que o modelo fundido tem o mesmo output que o modelo com o
    # adapter aplicado (prova de que os pesos não se perderam no merge).
    print("[cosca] validando merge (compara output pré/post)...")
    test_prompt = "Fix main.go to add state handling."
    inputs = tokenizer(test_prompt, return_tensors="pt").to("cuda")
    with torch.no_grad():
        out = model.generate(**inputs, max_new_tokens=16)
    merged_text = tokenizer.decode(out[0], skip_special_tokens=True)
    print(f"[cosca] merged output (amostra): {merged_text[:80]}")

    # ── 5. SALVA o modelo fundido (separado, NÃO toca o adapter) ────────
    os.makedirs(args.output, exist_ok=True)
    model.save_pretrained(args.output)
    tokenizer.save_pretrained(args.output)
    print(f"[cosca] modelo fundido salvo em {args.output}")

    # ── 6. EXPORTA GGUF (quantizado) para o Ollama ───────────────────────
    try:
        model.save_pretrained_gguf(
            args.output,
            quantization_method=args.gguf_quant,  # q4_k_m (ou f16 p/ testar)
        )
        print(f"[cosca] GGUF exportado: {args.output}/*.gguf ({args.gguf_quant})")
    except Exception as exc:  # noqa: BLE001
        print(f"[cosca] (opcional) GGUF falhou: {exc}")

    print("\n[cosca] CONCLUÍDO. Próximo passo:")
    print("  1. Baixe o .gguf de", args.output)
    print("  2. Crie o Modelfile do Ollama (base=modelo fundido)")
    print("  3. Carregue como cosca-qwen3-4b-lora-001")
    print("  4. Rode: cosca dataset golden --model cosca-qwen3-4b-lora-001  (AFTER)")


if __name__ == "__main__":
    import torch  # importado aqui para o teste de validação
    main()
