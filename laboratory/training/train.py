"""
Cosca Training Pipeline — Model Training
========================================
Fine-tunes Qwen2.5-1.5B-Instruct using LoRA/PEFT on CPU.
Student learns from teacher-generated dataset via SFT (Supervised Fine-Tuning).
"""

import json
import os
import sys
import time
import logging
from pathlib import Path
from typing import Optional

import torch
from datasets import load_dataset, Dataset
from transformers import (
    AutoModelForCausalLM,
    AutoTokenizer,
    TrainingArguments,
    BitsAndBytesConfig,
    DataCollatorForSeq2Seq,
)
from peft import LoraConfig, get_peft_model, TaskType, PeftModel
from trl import SFTTrainer, SFTConfig

from config import get_config, CoscaTrainingConfig

# ============================================================================
# LOGGING
# ============================================================================

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    handlers=[
        logging.StreamHandler(sys.stdout),
        logging.FileHandler("training.log", mode="a", encoding="utf-8"),
    ],
)
logger = logging.getLogger(__name__)


# ============================================================================
# DATA LOADING
# ============================================================================

def load_jsonl(filepath: str) -> Dataset:
    """Load a JSONL file into a HuggingFace Dataset."""
    path = Path(filepath)
    if not path.exists():
        raise FileNotFoundError(f"Dataset not found: {filepath}")

    data = []
    with open(path, "r", encoding="utf-8") as f:
        for line_num, line in enumerate(f, 1):
            line = line.strip()
            if not line:
                continue
            try:
                data.append(json.loads(line))
            except json.JSONDecodeError as e:
                logger.warning(f"Skipping invalid JSON on line {line_num}: {e}")
                continue

    if not data:
        raise ValueError(f"No valid samples in {filepath}")

    return Dataset.from_list(data)


def format_messages(example: dict) -> str:
    """Format messages into chat template string for Qwen2.5-Instruct."""
    messages = example.get("messages", [])
    if not messages:
        return ""
    # Return as a list of dicts - SFTTrainer will handle tokenization
    return messages


# ============================================================================
# MODEL LOADING
# ============================================================================

def load_student_model(config: CoscaTrainingConfig):
    """Load the student model and tokenizer."""
    model_name = config.model.student_name
    logger.info(f"Loading student model: {model_name}")

    # Tokenizer
    tokenizer = AutoTokenizer.from_pretrained(
        model_name,
        trust_remote_code=True,
        padding_side=config.model.padding_side,
    )
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    # Model - load in float32 for CPU (no quantization needed for 1.5B)
    model = AutoModelForCausalLM.from_pretrained(
        model_name,
        torch_dtype=torch.float32,
        trust_remote_code=True,
        device_map=None,  # CPU
    )

    logger.info(f"Model loaded. Parameters: {model.num_parameters():,}")
    return model, tokenizer


# ============================================================================
# LORA CONFIGURATION
# ============================================================================

def apply_lora(model, config: CoscaTrainingConfig):
    """Apply LoRA adapters to the model."""
    lora_config = LoraConfig(
        task_type=TaskType.CAUSAL_LM,
        r=16,
        lora_alpha=32,
        lora_dropout=0.05,
        target_modules=["q_proj", "v_proj"],
        bias="none",
    )

    model = get_peft_model(model, lora_config)
    model.print_trainable_parameters()
    return model


# ============================================================================
# TRAINING
# ============================================================================

def train(config: CoscaTrainingConfig, resume_from: Optional[str] = None):
    """Run SFT training loop."""
    logger.info("=" * 60)
    logger.info("COSCA TRAINING — SFT WITH LORA")
    logger.info("=" * 60)

    # Paths (relative to training/ directory)
    training_dir = Path(__file__).parent
    data_dir = training_dir / config.dataset.data_dir
    train_path = data_dir / config.dataset.train_file
    val_path = data_dir / config.dataset.val_file
    output_dir = training_dir / config.training.output_dir

    # Load data
    logger.info("Loading datasets...")
    train_dataset = load_jsonl(str(train_path))
    logger.info(f"Training samples: {len(train_dataset)}")

    val_dataset = None
    if val_path.exists():
        val_dataset = load_jsonl(str(val_path))
        logger.info(f"Validation samples: {len(val_dataset)}")

    # Load model and tokenizer
    model, tokenizer = load_student_model(config)

    # Apply LoRA
    model = apply_lora(model, config)

    # Training arguments
    training_args = SFTConfig(
        output_dir=str(output_dir),
        num_train_epochs=config.training.num_train_epochs,
        per_device_train_batch_size=config.training.per_device_train_batch_size,
        gradient_accumulation_steps=config.training.gradient_accumulation_steps,
        learning_rate=config.training.learning_rate,
        weight_decay=config.training.weight_decay,
        lr_scheduler_type=config.training.lr_scheduler_type,
        bf16=config.training.bf16,
        fp16=config.training.fp16,
        logging_steps=config.training.logging_steps,
        save_strategy=config.training.save_strategy,
        save_steps=config.training.save_steps,
        save_total_limit=config.training.save_total_limit,
        eval_strategy=config.training.eval_strategy if val_dataset else "no",
        eval_steps=config.training.eval_steps if val_dataset else None,
        seed=config.training.seed,
        dataloader_num_workers=config.training.dataloader_num_workers,
        report_to=config.training.report_to,
        dataset_text_field=None,
        packing=False,
        remove_unused_columns=False,
    )

    # Initialize trainer
    trainer = SFTTrainer(
        model=model,
        args=training_args,
        train_dataset=train_dataset,
        eval_dataset=val_dataset,
        processing_class=tokenizer,
    )

    # Resume or start fresh
    total_start = time.time()

    if resume_from and Path(resume_from).exists():
        logger.info(f"Resuming from checkpoint: {resume_from}")
        trainer.train(resume_from_checkpoint=resume_from)
    else:
        # Check for latest checkpoint in output_dir
        latest_checkpoint = find_latest_checkpoint(output_dir)
        if latest_checkpoint:
            logger.info(f"Found existing checkpoint: {latest_checkpoint}")
            logger.info("Resuming training from checkpoint...")
            trainer.train(resume_from_checkpoint=latest_checkpoint)
        else:
            logger.info("Starting training from scratch...")
            trainer.train()

    total_time = time.time() - total_start
    logger.info(f"Training completed in {total_time:.1f}s ({total_time/60:.1f}min)")

    # Save final model
    final_dir = output_dir / "final"
    final_dir.mkdir(parents=True, exist_ok=True)
    logger.info(f"Saving final model to {final_dir}")
    model.save_pretrained(str(final_dir))
    tokenizer.save_pretrained(str(final_dir))

    # Save training metrics
    metrics = trainer.evaluate() if val_dataset else {}
    metrics_path = output_dir / "training_metrics.json"
    with open(metrics_path, "w") as f:
        json.dump(
            {
                "trainable_params": sum(p.numel() for p in model.parameters() if p.requires_grad),
                "total_params": model.num_parameters(),
                "training_time_seconds": total_time,
                "final_metrics": metrics,
                "config": {
                    "model": config.model.student_name,
                    "lora_r": 16,
                    "lora_alpha": 32,
                    "epochs": config.training.num_train_epochs,
                    "lr": config.training.learning_rate,
                    "batch_size": config.training.per_device_train_batch_size,
                    "grad_accum": config.training.gradient_accumulation_steps,
                },
            },
            f,
            indent=2,
        )

    logger.info(f"Training metrics saved to {metrics_path}")
    return trainer, metrics


def find_latest_checkpoint(output_dir: Path) -> Optional[str]:
    """Find the latest checkpoint in the output directory."""
    checkpoints = sorted(
        [d for d in output_dir.iterdir() if d.is_dir() and d.name.startswith("checkpoint-")],
        key=lambda x: int(x.name.split("-")[-1]),
    )
    if checkpoints:
        return str(checkpoints[-1])
    return None


# ============================================================================
# QUICK TEST RUN
# ============================================================================

def quick_test(config: CoscaTrainingConfig):
    """Run a minimal training to verify pipeline works."""
    logger.info("=" * 60)
    logger.info("QUICK TEST — Verifying pipeline works")
    logger.info("=" * 60)

    # Create tiny test dataset
    data_dir = Path(config.dataset.data_dir)
    test_data_dir = data_dir / "test_run"
    test_data_dir.mkdir(parents=True, exist_ok=True)

    tiny_samples = [
        {
            "messages": [
                {"role": "user", "content": "O que e Git?"},
                {"role": "assistant", "content": "Git e um sistema de controle de versao distribuido usado para rastrear mudancas no codigo-fonte durante o desenvolvimento de software."},
            ]
        },
        {
            "messages": [
                {"role": "user", "content": "O que e REST?"},
                {"role": "assistant", "content": "REST (Representational State Transfer) e um estilo de arquitetura para sistemas distribuidos hipermidia, amplamente utilizado para APIs web."},
            ]
        },
        {
            "messages": [
                {"role": "user", "content": "Escreva uma funcao soma em Go."},
                {"role": "assistant", "content": "func soma(a, b int) int { return a + b }"},
            ]
        },
    ]

    # Save tiny dataset
    for split in ["train", "val"]:
        path = test_data_dir / f"{split}.jsonl"
        with open(path, "w", encoding="utf-8") as f:
            for s in tiny_samples:
                f.write(json.dumps(s, ensure_ascii=False) + "\n")

    # Override config for quick test
    test_config = CoscaTrainingConfig()
    test_config.dataset.data_dir = str(test_data_dir)
    test_config.dataset.train_file = "train.jsonl"
    test_config.dataset.val_file = "val.jsonl"
    training_dir = Path(__file__).parent
    test_config.training.output_dir = str(training_dir / "checkpoints" / "quick_test")
    test_config.training.num_train_epochs = 1
    test_config.training.per_device_train_batch_size = 1
    test_config.training.gradient_accumulation_steps = 1
    test_config.training.logging_steps = 1
    test_config.training.save_steps = 9999
    test_config.training.eval_steps = 9999
    test_config.training.dataloader_num_workers = 0

    try:
        train(test_config)
        logger.info("QUICK TEST PASSED — Pipeline is functional!")
        return True
    except Exception as e:
        logger.error(f"QUICK TEST FAILED: {e}")
        import traceback
        traceback.print_exc()
        return False


# ============================================================================
# MAIN
# ============================================================================

if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description="Train Qwen2.5-1.5B with LoRA")
    parser.add_argument("--quick-test", action="store_true", help="Run minimal test first")
    parser.add_argument("--resume", type=str, default=None, help="Resume from checkpoint path")
    parser.add_argument("--full", action="store_true", help="Run full training (skip quick test)")
    args = parser.parse_args()

    config = get_config()

    if args.quick_test or not args.full:
        success = quick_test(config)
        if not success:
            logger.error("Quick test failed. Fix issues before full training.")
            sys.exit(1)

        if args.quick_test:
            logger.info("Quick test complete. Run with --full for full training.")
            sys.exit(0)

    # Full training
    train(config, resume_from=args.resume)
    logger.info("All training complete!")
