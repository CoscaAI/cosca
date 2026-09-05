#!/usr/bin/env python3
"""
COSCA REMOTE TRAINING WORKER v1 — Colab (worker genérico)
================================================================
O Colab é um WORKER DESCARTÁVEL. Ele:
  1. lê o job (manifest.json do Git / .cosca/jobs/<campaign>.json)
  2. baixa dataset + modelo base (referência imutável via commit)
  3. treina o LoRA
  4. salva adapter/checkpoint
  5. retorna o WORKER_REPORT (SÓ artefatos — NUNCA decide promoção)

REGRA DE OURO (professor): o worker NUNCA decide promoção. Ele entrega
TRAINING_COMPLETE (adapter, checkpoint, metrics, logs). O COSCA valida
via Golden Gate → PromotionGate. Mesmo que o treino erre, não ganha acesso
ao "portão da casa".

USO (Colab):
  1. Monte o drive e suba/baixe o job (manifest).
  2. Rode:  !python worker_colab.py --job /content/campaign-002.json --out /content/output
"""

import argparse
import json
import os
import sys


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--job", required=True, help="caminho do manifest.json (job do COSCA)")
    ap.add_argument("--out", default="/content/output", help="diretório de saída dos artefatos")
    ap.add_argument("--dataset", default="", help="caminho do dataset (sobrepõe o do job, se necessário)")
    args = ap.parse_args()

    # 1. Lê o job (manifest imutável).
    with open(args.job, "r", encoding="utf-8") as f:
        job = json.load(f)
    print("[cosca-worker] JOB:", job.get("campaign_id"), "| base:", job.get("base"))

    dataset_path = args.dataset or job.get("dataset")
    if not dataset_path or not os.path.exists(dataset_path):
        print(f"[cosca-worker] ERRO: dataset não encontrado: {dataset_path}")
        sys.exit(1)

    cfg = job.get("config", {})
    lora = cfg.get("lora", {})
    base = job.get("base")
    os.makedirs(args.out, exist_ok=True)

    try:
        import torch
        from datasets import Dataset
        from transformers import AutoTokenizer
        from trl import SFTTrainer, SFTConfig
        from unsloth import FastLanguageModel
    except ImportError:
        print("[cosca-worker] ERRO: instale unsloth/trl/transformers/datasets")
        sys.exit(1)

    # 2. Carrega dataset (SFT ChatML do COSCA).
    recs = []
    for line in open(dataset_path, encoding="utf-8"):
        line = line.strip()
        if line:
            recs.append(json.loads(line))
    print(f"[cosca-worker] dataset: {len(recs)} exemplos")
    dataset = Dataset.from_list(recs)

    # 3. Carrega base (referência imutável) e aplica LoRA.
    max_seq = cfg.get("max_seq_len", 8192)
    model, tokenizer = FastLanguageModel.from_pretrained(
        model_name=base, max_seq_length=max_seq, dtype=None, load_in_4bit=True)
    tokenizer.add_tokens(["<|im_start|>", "<|im_end|>"], special_tokens=True)
    tokenizer.eos_token = "<|im_end|>"
    tokenizer.pad_token = "<|im_end|>"
    model.config.eos_token_id = tokenizer.convert_tokens_to_ids("<|im_end|>")
    model.generation_config.eos_token = "<|im_end|>"
    model.generation_config.eos_token_id = tokenizer.eos_token_id

    model = FastLanguageModel.get_peft_model(
        model, r=lora.get("r", 16), target_modules=lora.get("targets"),
        lora_alpha=lora.get("alpha", 32), lora_dropout=0, bias="none",
        use_gradient_checkpointing="unsloth", random_state=42)

    # 4. Template manual (sem <EOS_TOKEN> do apply_chat_template).
    def pt(messages):
        out = []
        for m in messages:
            r = m["role"]
            if r == "system": out.append("<|im_start|>system\n" + m["content"] + "<|im_end|>\n")
            elif r == "user": out.append("<|im_start|>user\n" + m["content"] + "<|im_end|>\n")
            elif r == "assistant": out.append("<|im_start|>assistant\n" + m["content"] + "<|im_end|>\n")
        return "".join(out)
    dataset = dataset.map(lambda ex: {"text": [pt(m) for m in ex["messages"]]}, batched=True)

    # 5. Treina.
    sft_cfg = SFTConfig(
        output_dir=args.out,
        per_device_train_batch_size=2, gradient_accumulation_steps=4,
        num_train_epochs=cfg.get("epochs", 3),
        learning_rate=cfg.get("learning_rate", 2e-4),
        lr_scheduler_type="linear", warmup_steps=10,
        max_length=max_seq, eos_token="<|im_end|>",
        logging_steps=5, save_strategy="epoch", report_to="none",
        bf16=False, fp16=True)
    trainer = SFTTrainer(model=model, processing_class=tokenizer, train_dataset=dataset, args=sft_cfg)
    print("[cosca-worker] TREINANDO...")
    trainer.train()

    # 6. Salva adapter/checkpoint (artefatos para o COSCA validar).
    model.save_pretrained(args.out)
    tokenizer.save_pretrained(args.out)

    # 7. WorkerReport — SÓ artefatos, SEM decisão de promoção.
    report = {
        "campaign_id": job.get("campaign_id"),
        "status": "TRAINING_COMPLETE",
        "adapter": args.out,
        "checkpoint": args.out,
        "metrics": {"epochs": cfg.get("epochs", 3)},
        "finished_at": __import__("datetime").datetime.now().isoformat(),
    }
    with open(os.path.join(args.out, "WORKER_REPORT.json"), "w", encoding="utf-8") as f:
        json.dump(report, f, indent=2)
    print("[cosca-worker] WORKER_REPORT escrito. Treinamento completo — o COSCA decide a promoção.")


if __name__ == "__main__":
    main()
