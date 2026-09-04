#!/usr/bin/env python3
"""
Gera o worker_self_contained.py embutindo dataset + job em base64.

O colab run so envia ESTE script. Dataset e job sao materializados na VM.
Worker: COSCA -> Job Manifest -> Colab Worker -> GPU/Unsloth -> LoRA ->
Artifact -> COSCA / Golden V2 / PromotionGate.

POLITICA: o worker NAO promove modelo, NAO altera dataset/Golden, apenas
treina e entrega artifact + report.
"""
import base64
import os

ROOT = "/mnt/c/Users/Henrique/Documents/cosca"

DATASET_PATH = os.path.join(ROOT, ".cosca", "campaign-002-package", "dataset.jsonl")
JOB_PATH = os.path.join(ROOT, ".cosca", "campaign-002-package", "campaign-002.json")

with open(DATASET_PATH, "rb") as f:
    DS_BYTES = f.read()
with open(JOB_PATH, "rb") as f:
    JOB_BYTES = f.read()

DS_B64_STR = base64.b64encode(DS_BYTES).decode("ascii")
JOB_B64_STR = base64.b64encode(JOB_BYTES).decode("ascii")

worker = r'''#!/usr/bin/env python3
"""COSCA REMOTE TRAINING WORKER — self-contained.
Dataset e job embutidos em Base64. FASES: PROVISIONING, DEPENDENCIES, CONFIG,
IMPORT, DATASET, MODEL, TOKENIZER, TRAINING, ARTIFACT, VALIDATION, DONE.
O worker marca cada etapa; NAO promove; NAO altera Golden/dataset.
"""
import argparse
import base64
import json
import os
import subprocess
import sys
import time

DATASET_B64 = "__DS__"
JOB_B64 = "__JOB__"


def phase(name, msg=""):
    print(f"[cosca-worker] PHASE={name} {msg}", flush=True)


def log(msg):
    print(f"[cosca-worker] {msg}", flush=True)


# ── PROVISIONING ──────────────────────────────────────────────

def materialize():
    phase("PROVISIONING")
    ds_path = "/content/dataset.jsonl"
    job_path = "/content/campaign-002.json"
    with open(ds_path, "wb") as f:
        f.write(base64.b64decode(DATASET_B64))
    with open(job_path, "wb") as f:
        f.write(base64.b64decode(JOB_B64))
    log("dados materializados")
    try:
        with open(job_path, "r", encoding="utf-8-sig") as f:
            job = json.load(f)
        log("job.json VALIDO (fail-fast)")
        missing = [k for k in ["campaign_id", "base"] if not job.get(k)]
        if missing:
            raise ValueError("job.json sem campos obrigatorios: %s" % missing)
    except Exception as e:
        log("ERRO FATAL: job.json invalido: %s" % e)
        raise
    return ds_path, job_path


# ── DEPENDENCIES ──────────────────────────────────────────────

def install_deps():
    phase("DEPENDENCIES", "instalando stack de treinamento...")
    packages = ["unsloth", "trl", "transformers", "datasets", "peft", "accelerate"]
    cmd = [sys.executable, "-m", "pip", "install", "--quiet", *packages]
    log("pip: " + " ".join(packages))
    try:
        p = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                             text=True, bufsize=1)
        import threading
        def pump():
            for line in p.stdout:
                line = line.strip()
                if line:
                    print("  [pip] %s" % line[:160], flush=True)
        thread = threading.Thread(target=pump, daemon=True)
        thread.start()
        while p.poll() is None:
            log("instalando deps... (heartbeat)")
            time.sleep(6)
        thread.join(timeout=5)
        if p.returncode != 0:
            raise RuntimeError("pip falhou com exit=%s" % p.returncode)
        log("deps instaladas")
    except Exception as e:
        log("ERRO FATAL na instalacao: %s" % e)
        raise


# ── TOKENIZER EOS COMPATIBILITY (professor) ───────────────────
# NAO adiciona '<EOS_TOKEN>' ao vocabulario (evita redimensionar
# embeddings). Se qualquer camada chamar convert_tokens_to_ids('<EOS_TOKEN>'),
# resolve para o EOS real (<|im_end|> / 151645).

def install_eos_compat(tokenizer):
    real_eos = tokenizer.eos_token
    real_eos_id = tokenizer.eos_token_id
    if not real_eos:
        raise RuntimeError("Tokenizer sem eos_token.")
    if real_eos_id is None:
        raise RuntimeError("Tokenizer sem eos_token_id.")
    original_convert = tokenizer.convert_tokens_to_ids

    def convert_tokens_to_ids_compat(tokens):
        if isinstance(tokens, str):
            if tokens == "<EOS_TOKEN>":
                return real_eos_id
            return original_convert(tokens)
        if isinstance(tokens, (list, tuple)):
            converted = []
            for token in tokens:
                if token == "<EOS_TOKEN>":
                    converted.append(real_eos_id)
                else:
                    converted.append(original_convert(token))
            return converted
        return original_convert(tokens)

    tokenizer.convert_tokens_to_ids = convert_tokens_to_ids_compat
    log("EOS compatibility instalada: <EOS_TOKEN> -> %r (id=%s)" % (real_eos, real_eos_id))
    return tokenizer


# ── TOKENIZER VALIDATION ──────────────────────────────────────

def validate_tokenizer(tokenizer, model):
    phase("TOKENIZER")
    eos = tokenizer.eos_token
    eos_id = tokenizer.eos_token_id
    log("EOS: %r" % eos)
    log("EOS_ID: %s" % eos_id)
    if eos_id is None:
        raise RuntimeError("eos_token_id e' None.")
    try:
        vocab = tokenizer.get_vocab()
        if eos not in vocab:
            raise RuntimeError("EOS %r nao esta' no vocabulario." % eos)
        log("EOS existe no vocab: True")
    except Exception as e:
        raise RuntimeError("Falha validando EOS no vocab: %s" % e)
    tokenizer.pad_token = eos
    model.config.eos_token_id = eos_id
    model.config.pad_token_id = eos_id
    if hasattr(model, "generation_config"):
        model.generation_config.eos_token_id = eos_id
        model.generation_config.pad_token_id = eos_id
        if hasattr(model.generation_config, "eos_token"):
            model.generation_config.eos_token = eos
    log("tokenizer.pad_token=%r" % tokenizer.pad_token)
    log("model.config.eos_token_id=%s" % model.config.eos_token_id)
    template = getattr(tokenizer, "chat_template", None)
    if template:
        log("chat_template presente")
        log("chat_template contem <EOS_TOKEN>: %s" % ("<EOS_TOKEN>" in str(template)))
    else:
        log("chat_template ausente")


# ── DATASET FORMAT ────────────────────────────────────────────

def manual_chat_template(messages):
    out = []
    for message in messages:
        role = message["role"]
        content = message["content"]
        out.append("<|im_start|>%s\n%s<|im_end|>\n" % (role, content))
    return "".join(out)


def format_dataset(dataset, tokenizer):
    phase("DATASET")
    def format_example(example):
        messages = example["messages"]
        if getattr(tokenizer, "chat_template", None):
            try:
                text = tokenizer.apply_chat_template(messages, tokenize=False,
                                                     add_generation_prompt=False)
                return {"text": text}
            except Exception as e:
                log("AVISO: chat_template nativo falhou; usando fallback: %s" % e)
        return {"text": manual_chat_template(messages)}
    cols = [c for c in dataset.column_names if c != "text"]
    dataset = dataset.map(format_example, remove_columns=cols, desc="Formatando dataset COSCA")
    log("dataset formatado: %d exemplos" % len(dataset))
    if len(dataset) > 0:
        first = dataset[0]["text"]
        log("primeiro exemplo:")
        print(first[:1200], flush=True)
        log("primeiro exemplo contem <EOS_TOKEN>: %s" % ("<EOS_TOKEN>" in first))
        log("primeiro exemplo contem <|im_end|>: %s" % ("<|im_end|>" in first))
    return dataset


# ── DIAGNOSTICS ───────────────────────────────────────────────

def inspect_environment():
    phase("DIAGNOSTICS")
    import torch, transformers, trl, unsloth
    log("Python: %s" % sys.version.split()[0])
    log("PyTorch: %s" % torch.__version__)
    log("Transformers: %s" % transformers.__version__)
    log("TRL: %s" % getattr(trl, "__version__", "unknown"))
    log("Unsloth: %s" % getattr(unsloth, "__version__", "unknown"))
    log("CUDA available: %s" % torch.cuda.is_available())
    if torch.cuda.is_available():
        log("GPU: %s" % torch.cuda.get_device_name(0))
    try:
        import inspect
        from trl import SFTTrainer
        path = inspect.getsourcefile(SFTTrainer)
        log("SFTTrainer source: %s" % path)
        if path and os.path.exists(path):
            with open(path, "r", encoding="utf-8") as f:
                lines = f.readlines()
            log("trecho do SFTTrainer (linhas 595-640):")
            for i in range(595, min(640, len(lines))):
                print("[cosca-worker] SFT:%d: %s" % (i + 1, lines[i].rstrip()), flush=True)
    except Exception as e:
        log("inspect SFTTrainer falhou: %s" % e)


# ── MAIN ──────────────────────────────────────────────────────

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--job", default="/content/campaign-002.json")
    ap.add_argument("--dataset", default="/content/dataset.jsonl")
    ap.add_argument("--out", default="/content/output")
    args = ap.parse_args()

    ds_path, job_path = materialize()
    install_deps()

    phase("CONFIG")
    with open(job_path, "r", encoding="utf-8-sig") as f:
        job = json.load(f)
    campaign_id = job.get("campaign_id")
    base = job.get("base")
    cfg = job.get("config", {})
    lora = cfg.get("lora", {})
    if not base:
        raise RuntimeError("job.base nao definido.")
    log("JOB: %s | base: %s" % (campaign_id, base))
    log("config: %s" % json.dumps(cfg, ensure_ascii=False))
    os.makedirs(args.out, exist_ok=True)

    phase("IMPORT")
    from unsloth import FastLanguageModel
    from datasets import Dataset
    from trl import SFTTrainer, SFTConfig
    inspect_environment()

    records = []
    with open(ds_path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if line:
                records.append(json.loads(line))
    log("dataset: %d exemplos" % len(records))
    if not records:
        raise RuntimeError("Dataset vazio.")
    dataset = Dataset.from_list(records)

    phase("MODEL")
    max_seq = cfg.get("max_seq_len", 8192)
    model, tokenizer = FastLanguageModel.from_pretrained(
        model_name=base, max_seq_length=max_seq, dtype=None, load_in_4bit=True)
    validate_tokenizer(tokenizer, model)

    tokenizer = install_eos_compat(tokenizer)
    compat_eos_id = tokenizer.convert_tokens_to_ids("<EOS_TOKEN>")
    if compat_eos_id != tokenizer.eos_token_id:
        raise RuntimeError("EOS compatibility falhou: %s != %s" % (compat_eos_id, tokenizer.eos_token_id))
    log("EOS compatibility VALIDADA")

    log("configurando LoRA...")
    model = FastLanguageModel.get_peft_model(
        model, r=lora.get("r", 16), target_modules=lora.get("targets"),
        lora_alpha=lora.get("alpha", 32), lora_dropout=0, bias="none",
        use_gradient_checkpointing="unsloth", random_state=42)

    dataset = format_dataset(dataset, tokenizer)

    phase("TRAINING")
    real_eos = tokenizer.eos_token
    log("SFT EOS final: %r" % real_eos)
    log("SFT EOS ID final: %s" % tokenizer.eos_token_id)
    sft_cfg = SFTConfig(
        output_dir=args.out,
        per_device_train_batch_size=2,
        gradient_accumulation_steps=4,
        num_train_epochs=cfg.get("epochs", 3),
        learning_rate=cfg.get("learning_rate", 2e-4),
        lr_scheduler_type="linear",
        warmup_steps=10,
        max_length=max_seq,
        eos_token=real_eos,
        logging_steps=5,
        save_strategy="epoch",
        report_to="none",
        bf16=False,
        fp16=True,
        seed=42,
        data_seed=42,
    )
    log("SFTConfig.eos_token=%r" % sft_cfg.eos_token)
    try:
        log("SFTConfig eos_token field=%r" % sft_cfg.to_dict().get("eos_token"))
    except Exception as e:
        log("SFTConfig.to_dict erro: %s" % e)
    log("TOKENIZER EOS=%r" % tokenizer.eos_token)
    log("TOKENIZER EOS ID=%s" % tokenizer.eos_token_id)
    log("MODEL CONFIG EOS ID=%s" % model.config.eos_token_id)

    log("criando SFTTrainer...")
    trainer = SFTTrainer(model=model, processing_class=tokenizer,
                         train_dataset=dataset, args=sft_cfg)
    log("SFTTrainer criado com sucesso.")

    log("TREINANDO...")
    train_result = trainer.train()
    log("trainer.train() terminou.")

    phase("ARTIFACT")
    model.save_pretrained(args.out)
    tokenizer.save_pretrained(args.out)

    metrics = {"epochs": cfg.get("epochs", 3)}
    try:
        if train_result is not None:
            metrics.update(train_result.metrics)
    except Exception:
        pass

    phase("VALIDATION")
    report = {
        "campaign_id": campaign_id,
        "status": "TRAINING_COMPLETE",
        "adapter": args.out,
        "checkpoint": args.out,
        "base_model": base,
        "metrics": metrics,
        "tokenizer": {
            "eos_token": tokenizer.eos_token,
            "eos_token_id": tokenizer.eos_token_id,
            "pad_token": tokenizer.pad_token,
            "pad_token_id": tokenizer.pad_token_id,
        },
        "policy": {"promotion": "COSCA_ONLY", "golden": "COSCA_ONLY"},
        "finished_at": __import__("datetime").datetime.now().isoformat(),
    }
    report_path = os.path.join(args.out, "WORKER_REPORT.json")
    with open(report_path, "w", encoding="utf-8") as f:
        json.dump(report, f, indent=2, ensure_ascii=False)
    log("WORKER_REPORT escrito: %s" % report_path)

    phase("DONE")
    log("CAMPAIGN CONCLUIDA.")


if __name__ == "__main__":
    main()
'''

worker = worker.replace("__DS__", DS_B64_STR).replace("__JOB__", JOB_B64_STR)
out = os.path.join(ROOT, "train", "worker_self_contained.py")
with open(out, "w", encoding="utf-8") as f:
    f.write(worker)
print("gerado:", out, len(worker), "bytes")
