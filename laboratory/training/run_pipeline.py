"""
Cosca Training Pipeline - Orchestrator
======================================
Runs the full pipeline: generate data -> train -> evaluate -> report.
"""

import json
import os
import sys
import time
import logging
from pathlib import Path
from datetime import datetime

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    handlers=[logging.StreamHandler(sys.stdout)],
)
logger = logging.getLogger(__name__)

# Ensure we can import from the training directory
TRAINING_DIR = Path(__file__).parent
sys.path.insert(0, str(TRAINING_DIR))


def step_generate_data(config):
    """Step 1: Generate dataset using teacher model."""
    logger.info("=" * 60)
    logger.info("STEP 1: GENERATE DATASET")
    logger.info("=" * 60)

    training_dir = Path(__file__).parent
    data_dir = training_dir / config.dataset.data_dir
    train_path = data_dir / config.dataset.train_file

    if train_path.exists():
        # Count existing samples
        with open(train_path, "r", encoding="utf-8") as f:
            count = sum(1 for line in f if line.strip())
        logger.info("Dataset already exists with %d training samples", count)
        logger.info("Skipping generation. Delete %s to regenerate.", data_dir)
        return True

    try:
        from generate_dataset import DatasetGenerator

        generator = DatasetGenerator(config)
        generator.run(num_samples=50)  # Small initial dataset

        if not train_path.exists():
            logger.error("Dataset generation failed - no train.jsonl produced")
            return False

        logger.info("Dataset generation complete!")
        return True

    except Exception as e:
        logger.error("Dataset generation failed: %s", e)
        import traceback
        traceback.print_exc()
        return False


def step_train(config, quick_only=False):
    """Step 2: Train model with LoRA."""
    logger.info("=" * 60)
    logger.info("STEP 2: TRAIN MODEL")
    logger.info("=" * 60)

    training_dir = Path(__file__).parent
    data_dir = training_dir / config.dataset.data_dir
    train_path = data_dir / config.dataset.train_file

    if not train_path.exists():
        logger.error("No training data found at %s", train_path)
        logger.error("Run dataset generation first.")
        return False

    try:
        from train import quick_test, train

        # Always run quick test first
        logger.info("Running quick test to verify pipeline...")
        success = quick_test(config)
        if not success:
            logger.error("Quick test failed! Fix issues before proceeding.")
            return False

        if quick_only:
            logger.info("Quick test passed. Skipping full training.")
            return True

        # Full training
        logger.info("Starting full training...")
        start_time = time.time()
        trainer, metrics = train(config)
        elapsed = time.time() - start_time

        logger.info("Training completed in %.1f minutes", elapsed / 60)
        return True

    except Exception as e:
        logger.error("Training failed: %s", e)
        import traceback
        traceback.print_exc()
        return False


def step_evaluate(config):
    """Step 3: Evaluate trained model."""
    logger.info("=" * 60)
    logger.info("STEP 3: EVALUATE MODEL")
    logger.info("=" * 60)

    training_dir = Path(__file__).parent
    final_dir = training_dir / config.training.output_dir / "final"
    if not final_dir.exists():
        logger.warning("No trained model found at %s", final_dir)
        logger.warning("Skipping evaluation.")
        return None

    try:
        from evaluate import evaluate

        results = evaluate(config)
        return results

    except Exception as e:
        logger.error("Evaluation failed: %s", e)
        import traceback
        traceback.print_exc()
        return None


def step_report(config, results):
    """Step 4: Generate final report."""
    logger.info("=" * 60)
    logger.info("STEP 4: REPORT")
    logger.info("=" * 60)

    report = {
        "timestamp": datetime.now().isoformat(),
        "config": {
            "student_model": config.model.student_name,
            "teacher_model": config.model.teacher_model,
            "epochs": config.training.num_train_epochs,
            "lr": config.training.learning_rate,
            "batch_size": config.training.per_device_train_batch_size,
            "grad_accum": config.training.gradient_accumulation_steps,
            "lora_r": 16,
            "lora_alpha": 32,
        },
        "evaluation": results,
    }

    # Save report
    training_dir = Path(__file__).parent
    report_path = training_dir / config.training.output_dir / "pipeline_report.json"
    with open(report_path, "w", encoding="utf-8") as f:
        json.dump(report, f, indent=2, ensure_ascii=False)

    logger.info("Pipeline report saved to %s", report_path)

    # Print summary
    logger.info("")
    logger.info("=" * 60)
    logger.info("PIPELINE COMPLETE")
    logger.info("=" * 60)
    logger.info("Student: %s", config.model.student_name)
    logger.info("Teacher: %s", config.model.teacher_model)

    if results:
        if "base_perplexity" in results:
            logger.info("Base perplexity: %.2f", results["base_perplexity"])
        if "trained_perplexity" in results:
            logger.info("Trained perplexity: %.2f", results["trained_perplexity"])
        if "perplexity_improvement_pct" in results:
            logger.info("Improvement: %.1f%%", results["perplexity_improvement_pct"])

    logger.info("Checkpoints: %s", config.training.output_dir)
    logger.info("Report: %s", report_path)

    return report


# ============================================================================
# MAIN
# ============================================================================

def main():
    """Run the full pipeline."""
    import argparse

    parser = argparse.ArgumentParser(description="Cosca Training Pipeline")
    parser.add_argument("--quick-test", action="store_true", help="Quick test only")
    parser.add_argument("--skip-generate", action="store_true", help="Skip data generation")
    parser.add_argument("--skip-train", action="store_true", help="Skip training")
    parser.add_argument("--skip-eval", action="store_true", help="Skip evaluation")
    parser.add_argument("--generate-only", action="store_true", help="Only generate data")
    parser.add_argument("--train-only", action="store_true", help="Only train (data must exist)")
    parser.add_argument("--eval-only", action="store_true", help="Only evaluate")
    args = parser.parse_args()

    from config import get_config

    config = get_config()
    results = None
    start_time = time.time()

    logger.info("=" * 60)
    logger.info("COSCA TRAINING PIPELINE")
    logger.info("Started at: %s", datetime.now().isoformat())
    logger.info("=" * 60)

    # Step 1: Generate dataset
    if not args.skip_generate and not args.train_only and not args.eval_only:
        if not step_generate_data(config):
            if not args.quick_test:
                logger.error("Pipeline aborted at data generation.")
                sys.exit(1)

    if args.generate_only:
        logger.info("Generate-only mode. Done.")
        return

    # Step 2: Train
    if not args.skip_train and not args.eval_only:
        if not step_train(config, quick_only=args.quick_test):
            if not args.quick_test:
                logger.error("Pipeline aborted at training.")
                sys.exit(1)

    if args.train_only:
        logger.info("Train-only mode. Done.")
        return

    # Step 3: Evaluate
    if not args.skip_eval:
        results = step_evaluate(config)

    if args.eval_only:
        logger.info("Eval-only mode. Done.")
        return

    # Step 4: Report
    step_report(config, results)

    elapsed = time.time() - start_time
    logger.info("Total pipeline time: %.1f minutes", elapsed / 60)


if __name__ == "__main__":
    main()
