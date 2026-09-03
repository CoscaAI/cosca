#!/usr/bin/env python3
"""
COSCA TREINADOR — LoRA no Qwen3-4B (Campanha Experimental 001)
================================================================
Treina um adaptador LoRA/QLoRA no Qwen3-4B usando o dataset SFT gerado pelo
COSCA (datasetgen → `dataset convert` → SFT ChatML).

PROPOSITO (professor):
  Provar que o ciclo de treinamento funciona. NAO mexer no Golden v1.
  Foco: melhorar RECOVERY + resolucao correta. Manter read_edit=1.00 e
  0 violacoes criticas. Se AFTER vier 0.88->1.00, investigar overfitting
  comportamental (prova inedita).

USO (Colab / NVIDIA GPU):
  1. Monte o repo (ou copie o dataset SFT).
  2. Rode:  !python train/lora_qwen3_4b.py --data .cosca/dataset-train.sft.jsonl.train.jsonl
  3. O adapter e' salvo em ./output/cosca-qwen3-4b-lora

PREREQUISITOS:
  pip install unsloth transformers datasets peft accelerate trl
  (no Colab:  !pip install "unsloth[colab-new] @ git+https://github.com/unslothai/unsloth.git")
"""

import argparse
import json
import os
import sys

# ─── Configuração base ──────────────────────────────────────────────────────

BASE_MODEL = "Qwen/Qwen3-4B"   # igual ao qwen3:4b do Ollama
OUTPUT_DIR = "output/cosca-qwen3-4b-lora"
MAX_SEQ_LEN = 8192
LORA_R = 16
LORA_ALPHA = 32
LORA_TARGET = ["q_proj", "k_proj", "v_proj", "o_proj",
               "gate_proj", "up_proj", "down_proj"]
EPOCHS = 3
BATCH = 2
GRAD_ACCUM = 4
LR = 2e-4

SYSTEM_PROMPT = (
    "You are a coding agent in a workspace. You MUST use tools to accomplish the task. "
    "Always read_file before edit_file. old_string must be an exact substring you observed. "
    "If a tool errors, read again and retry. Execute by calling tools, not describing."
)


def load_sft_dataset(path):
    """Carrega o JSONL SFT (ChatML) do COSCA e o formata para o SFTTrainer.
    O formato esperado: {"messages":[{role,content},...]}."""
    records = []
    with open(path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                rec = json.loads(line)
            except json.JSONDecodeError:
                continue
            msgs = rec.get("messages", [])
            # Normaliza: garante system no inicio (se ausente, injeta).
            if not msgs or msgs[0].get("role") != "system":
                msgs = [{"role": "system", "content": SYSTEM_PROMPT}] + msgs
            records.append({"messages": msgs})
    print(f"[cosca] carregados {len(records)} exemplos SFT de {path}")
    return records


def chat_template(messages, tokenizer):
    """Aplica o chat template real do Qwen3 / dos tokens usados no Ollama.
    Reutiliza o template padrao do tokenizer se disponivel."""
    if not tokenizer.chat_template:
        # Fallback simples (funciona p/ Qwen3): usa nomes de rolo generico.
        toks = {"system": "<|im_start|>system", "user": "<|im_start|>user",
                "assistant": "<|im_start|>assistant"}
        out = []
        for m in messages:
            role = m["role"]
            content = m["content"]
            if role not in toks:
                continue
            out.append(f"{toks[role]}\n{content}<|im_end|>\n")
        return "".join(out)
    try:
        return tokenizer.apply_chat_template(messages, tokenize=False)
    except Exception:
        return chat_template_simple(messages)


def chat_template_simple(messages):
    toks = {"system": "<|im_start|>system", "user": "<|im_start|>user",
            "assistant": "<|im_start|>assistant"}
    out = []
    for m in messages:
        role = m["role"]
        if role not in toks:
            continue
        out.append(f"{toks[role]}\n{m['content']}<|im_end|>\n")
    return "".join(out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--data", required=True, help="dataset SFT JSONL (train)")
    ap.add_argument("--val", default="", help="dataset SFT JSONL (val, opcional)")
    ap.add_argument("--base", default=BASE_MODEL)
    ap.add_argument("--output", default=OUTPUT_DIR)
    ap.add_argument("--epochs", type=int, default=EPOCHS)
    ap.add_argument("--lora-r", type=int, default=LORA_R)
    ap.add_argument("--lora-alpha", type=int, default=LORA_ALPHA)
    ap.add_argument("--lr", type=float, default=LR)
    ap.add_argument("--dry-run", action="store_true",
                    help="so' valida dataset e config, sem treinar")
    args = ap.parse_args()

    if args.dry_run:
        records = load_sft_dataset(args.data)
        print(f"[cosca] DRY-RUN ok: {len(records)} exemplos | "
              f"base={args.base} lora_r={args.lora_r} lora_alpha={args.lora_alpha}")
        # Amostra do primeiro exemplo formatado.
        if records:
            print(f"[cosca] primeiro exemplo (raw): {json.dumps(records[0])[:300]}...")
        return

    try:
        from unsloth import FastLanguageModel
    except ImportError:
        print("ERRO: instale unsloth -> pip install 'unsloth[colab-new] @ git+https://github.com/unslothai/unsloth.git'")
        sys.exit(1)

    from datasets import Dataset
    from transformers import AutoTokenizer
    from trl import SFTTrainer, SFTConfig

    # ── 1. Carrega dataset ────────────────────────────────────────────────
    records = load_sft_dataset(args.data)
    if not records:
        print("ERRO: dataset vazio")
        sys.exit(1)
    dataset = Dataset.from_list(records)

    # ── 2. Carrega o modelo base (quantizado 4-bit p/ QLoRA) ──────────────
    model, tokenizer = FastLanguageModel.from_pretrained(
        model_name=args.base,
        max_seq_length=MAX_SEQ_LEN,
        dtype=None,  # auto (bfloat16 se disponivel)
        load_in_4bit=True,  # QLoRA: base congelada em 4-bit
    )
    model = FastLanguageModel.get_peft_model(
        model,
        r=args.lora_r,
        target_modules=LORA_TARGET,
        lora_alpha=args.lora_alpha,
        lora_dropout=0,  # determinismo
        bias="none",
        use_gradient_checkpointing="unsloth",
        random_state=42,
    )

    # Template de chat: aplicamos o template do tokenizer por exemplo.
    def format_fn(examples):
        texts = [chat_template(m, tokenizer) for m in examples["messages"]]
        return {"text": texts}

    dataset = dataset.map(format_fn, batched=True)

    # ── 3. SFT Config + Trainer ───────────────────────────────────────────
    sft_config = SFTConfig(
        output_dir=args.output,
        per_device_train_batch_size=BATCH,
        gradient_accumulation_steps=GRAD_ACCUM,
        num_train_epochs=args.epochs,
        learning_rate=args.lr,
        lr_scheduler_type="linear",
        warmup_ratio=0.03,
        max_seq_length=MAX_SEQ_LEN,
        logging_steps=5,
        save_strategy="epoch",
        report_to="none",
    )
    trainer = SFTTrainer(
        model=model,
        tokenizer=tokenizer,
        train_dataset=dataset,
        args=sft_config,
    )

    # ── 4. Treina ─────────────────────────────────────────────────────────
    print("[cosca] iniciando treino (campanha 001)...")
    trainer.train()

    # ── 5. Salva o adaptador LoRA (para aplicar no Ollama/local) ─────────
    os.makedirs(args.output, exist_ok=True)
    model.save_pretrained(args.output)
    tokenizer.save_pretrained(args.output)
    print(f"[cosca] adaptador salvo em {args.output}")

    # ── 6. Exporta para GGUF (para o Ollama) se unsloth suportar ─────────
    try:
        model.save_pretrained_gguf(args.output, quantization_method="q4_k_m")
        print("[cosca] GGUF exportado (q4_k_m) para uso no Ollama")
    except Exception as exc:  # noqa: BLE001
        print(f"[cosca] (opcional) export GGUF nao disponivel: {exc}")

    print("[cosca] treino concluido. Proximo passo: rodar 'cosca dataset golden' com o adaptador para o AFTER.")


if __name__ == "__main__":
    main()
