"""
Cosca Training Pipeline - Model Evaluation
==========================================
Evaluates trained model: perplexity, response generation, baseline comparison.
"""

import json
import os
import sys
import time
import math
import logging
from pathlib import Path
from typing import Optional, List, Dict

import torch
from transformers import AutoModelForCausalLM, AutoTokenizer
from peft import PeftModel

from config import get_config, CoscaTrainingConfig

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    handlers=[
        logging.StreamHandler(sys.stdout),
        logging.FileHandler("evaluation.log", mode="w", encoding="utf-8"),
    ],
)
logger = logging.getLogger(__name__)


# ============================================================================
# MODEL LOADING
# ============================================================================

def load_base_model(config: CoscaTrainingConfig):
    """Load the base (untrained) student model."""
    model_name = config.model.student_name
    logger.info("Loading base model: %s", model_name)

    tokenizer = AutoTokenizer.from_pretrained(model_name, trust_remote_code=True)
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    model = AutoModelForCausalLM.from_pretrained(
        model_name,
        torch_dtype=torch.float32,
        trust_remote_code=True,
        device_map=None,
    )
    model.eval()
    return model, tokenizer


def load_trained_model(config: CoscaTrainingConfig, checkpoint_path: Optional[str] = None):
    """Load the fine-tuned model with LoRA adapters."""
    model_name = config.model.student_name

    if checkpoint_path is None:
        training_dir = Path(__file__).parent
        checkpoint_path = str(training_dir / config.training.output_dir / "final")

    if not Path(checkpoint_path).exists():
        raise FileNotFoundError("Checkpoint not found: %s" % checkpoint_path)

    logger.info("Loading trained model from: %s", checkpoint_path)

    tokenizer = AutoTokenizer.from_pretrained(checkpoint_path, trust_remote_code=True)
    if tokenizer.pad_token is None:
        tokenizer.pad_token = tokenizer.eos_token

    base_model = AutoModelForCausalLM.from_pretrained(
        model_name,
        torch_dtype=torch.float32,
        trust_remote_code=True,
        device_map=None,
    )

    model = PeftModel.from_pretrained(base_model, checkpoint_path)
    model.eval()
    return model, tokenizer


# ============================================================================
# PERPLEXITY
# ============================================================================

def compute_perplexity(
    model,
    tokenizer,
    texts: List[str],
    max_length: int = 512,
    stride: int = 256,
) -> float:
    """Compute perplexity using sliding window."""
    logger.info("Computing perplexity on %d samples...", len(texts))

    total_loss = 0.0
    total_tokens = 0

    for i, text in enumerate(texts):
        if i % 10 == 0:
            logger.info("  Sample %d/%d", i + 1, len(texts))

        encodings = tokenizer(text, return_tensors="pt", truncation=True, max_length=max_length)
        input_ids = encodings.input_ids
        seq_len = input_ids.size(1)

        if seq_len < 2:
            continue

        for begin_loc in range(0, seq_len, stride):
            end_loc = min(begin_loc + max_length, seq_len)
            trg_len = end_loc - begin_loc
            if begin_loc > 0:
                trg_len = min(stride, end_loc - begin_loc)

            input_chunk = input_ids[:, begin_loc:end_loc]

            with torch.no_grad():
                outputs = model(input_chunk, labels=input_chunk)
                loss = outputs.loss

            if not math.isnan(loss.item()):
                total_loss += loss.item() * trg_len
                total_tokens += trg_len

            if end_loc >= seq_len:
                break

    if total_tokens == 0:
        return float("inf")

    avg_loss = total_loss / total_tokens
    return math.exp(avg_loss)


# ============================================================================
# RESPONSE GENERATION
# ============================================================================

def generate_response(
    model,
    tokenizer,
    prompt: str,
    system: str = None,
    max_new_tokens: int = 512,
    temperature: float = 0.7,
    top_p: float = 0.9,
) -> str:
    """Generate a response from the model."""
    messages = []
    if system:
        messages.append({"role": "system", "content": system})
    messages.append({"role": "user", "content": prompt})

    text = tokenizer.apply_chat_template(
        messages, tokenize=False, add_generation_prompt=True
    )
    inputs = tokenizer(text, return_tensors="pt", truncation=True, max_length=1024)

    with torch.no_grad():
        outputs = model.generate(
            **inputs,
            max_new_tokens=max_new_tokens,
            temperature=temperature,
            top_p=top_p,
            do_sample=True,
            pad_token_id=tokenizer.pad_token_id,
        )

    generated = outputs[0][inputs["input_ids"].shape[1]:]
    response = tokenizer.decode(generated, skip_special_tokens=True)
    return response.strip()


# ============================================================================
# EVALUATION
# ============================================================================

def evaluate(config: CoscaTrainingConfig, checkpoint_path: Optional[str] = None):
    """Run full evaluation pipeline."""
    logger.info("=" * 60)
    logger.info("COSCA TRAINING - MODEL EVALUATION")
    logger.info("=" * 60)

    results = {}
    training_dir = Path(__file__).parent
    data_dir = training_dir / config.dataset.data_dir
    output_dir = training_dir / config.training.output_dir
    results_dir = output_dir / "eval_results"
    results_dir.mkdir(parents=True, exist_ok=True)

    # Load test data
    test_path = data_dir / config.dataset.test_file
    val_path = data_dir / config.dataset.val_file

    test_samples = []
    if test_path.exists():
        with open(test_path, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if line:
                    test_samples.append(json.loads(line))
        logger.info("Loaded %d test samples", len(test_samples))
    else:
        logger.warning("No test file found at %s", test_path)

    val_texts = []
    if val_path.exists():
        with open(val_path, "r", encoding="utf-8") as f:
            for line in f:
                line = line.strip()
                if line:
                    sample = json.loads(line)
                    for msg in sample.get("messages", []):
                        if msg.get("role") == "assistant":
                            val_texts.append(msg["content"])
        logger.info("Loaded %d validation texts for perplexity", len(val_texts))
    else:
        logger.warning("No validation file found at %s", val_path)

    # Load models
    logger.info("Loading base model...")
    base_model, base_tokenizer = load_base_model(config)

    trained_model = None
    trained_tokenizer = None
    try:
        logger.info("Loading trained model...")
        trained_model, trained_tokenizer = load_trained_model(config, checkpoint_path)
    except FileNotFoundError as e:
        logger.warning("Could not load trained model: %s", e)
        logger.warning("Will evaluate base model only.")

    # Perplexity
    logger.info("--- Perplexity Evaluation ---")
    if val_texts:
        logger.info("Evaluating BASE model perplexity...")
        base_ppl = compute_perplexity(base_model, base_tokenizer, val_texts[:50])
        results["base_perplexity"] = base_ppl
        logger.info("Base model perplexity: %.2f", base_ppl)

        if trained_model:
            logger.info("Evaluating TRAINED model perplexity...")
            trained_ppl = compute_perplexity(trained_model, trained_tokenizer, val_texts[:50])
            results["trained_perplexity"] = trained_ppl
            logger.info("Trained model perplexity: %.2f", trained_ppl)

            improvement = ((base_ppl - trained_ppl) / base_ppl) * 100
            results["perplexity_improvement_pct"] = improvement
            logger.info("Perplexity improvement: %.1f%%", improvement)
    else:
        logger.warning("No validation texts for perplexity")

    # Response generation
    logger.info("--- Response Generation ---")
    if test_samples:
        system_prompt = "Voce e um assistente de IA competente. Responda de forma clara e precisa."
        eval_samples = test_samples[:10]
        comparison_results = []

        for i, sample in enumerate(eval_samples):
            user_msg = ""
            expected = ""
            for msg in sample.get("messages", []):
                if msg.get("role") == "user":
                    user_msg = msg["content"]
                elif msg.get("role") == "assistant":
                    expected = msg["content"]

            if not user_msg:
                continue

            logger.info("Sample %d: %s...", i + 1, user_msg[:80])

            base_response = generate_response(base_model, base_tokenizer, user_msg, system=system_prompt)

            trained_response = ""
            if trained_model:
                trained_response = generate_response(trained_model, trained_tokenizer, user_msg, system=system_prompt)

            result = {
                "prompt": user_msg,
                "expected": expected[:500],
                "base_response": base_response[:500],
                "trained_response": trained_response[:500] if trained_response else "N/A",
                "level": sample.get("level", 0),
            }
            comparison_results.append(result)

            logger.info("  Expected:  %s...", expected[:100])
            logger.info("  Base:      %s...", base_response[:100])
            if trained_response:
                logger.info("  Trained:   %s...", trained_response[:100])

        results["comparison_samples"] = comparison_results

        comparison_path = results_dir / "response_comparison.json"
        with open(comparison_path, "w", encoding="utf-8") as f:
            json.dump(comparison_results, f, indent=2, ensure_ascii=False)
        logger.info("Comparison results saved to %s", comparison_path)

    # Summary
    logger.info("=" * 60)
    logger.info("EVALUATION SUMMARY")
    logger.info("=" * 60)

    summary = {
        "base_perplexity": results.get("base_perplexity"),
        "trained_perplexity": results.get("trained_perplexity"),
        "perplexity_improvement_pct": results.get("perplexity_improvement_pct"),
        "num_test_samples": len(test_samples),
        "num_evaluated": len(results.get("comparison_samples", [])),
        "checkpoint_path": checkpoint_path or "final",
    }

    for key, value in summary.items():
        if value is not None:
            logger.info("  %s: %s", key, value)

    results_path = results_dir / "evaluation_results.json"
    with open(results_path, "w", encoding="utf-8") as f:
        json.dump(results, f, indent=2, ensure_ascii=False)
    logger.info("Full results saved to %s", results_path)

    del base_model, base_tokenizer
    if trained_model:
        del trained_model, trained_tokenizer

    return results


# ============================================================================
# MAIN
# ============================================================================

if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description="Evaluate trained model")
    parser.add_argument("--checkpoint", type=str, default=None, help="Checkpoint path")
    args = parser.parse_args()

    config = get_config()
    evaluate(config, checkpoint_path=args.checkpoint)
