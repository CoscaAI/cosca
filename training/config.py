"""
Cosca Training Pipeline — Configuration
=======================================
Student: Qwen2.5-1.5B-Instruct (1.5B params)
Teacher: qwen2.5-coder:14b via Ollama (14.8B params, Q4_K_M)
Hardware: AMD Ryzen 7 5700X3D, 32GB RAM, RX 6700 XT (CPU training)
"""

import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional


@dataclass
class ModelConfig:
    """Model configuration."""
    # Student
    student_name: str = "Qwen/Qwen2.5-1.5B-Instruct"
    student_revision: Optional[str] = None
    
    # Teacher (Ollama)
    teacher_endpoint: str = "http://localhost:11434"
    teacher_model: str = "qwen2.5-coder:14b"
    
    # Tokenizer
    max_seq_length: int = 2048
    padding_side: str = "right"


@dataclass
class TrainingConfig:
    """Training hyperparameters."""
    # Core
    output_dir: str = "./checkpoints"
    num_train_epochs: int = 3
    per_device_train_batch_size: int = 2
    gradient_accumulation_steps: int = 8
    
    # Learning rate
    learning_rate: float = 2e-5
    weight_decay: float = 0.01
    warmup_ratio: float = 0.1
    lr_scheduler_type: str = "cosine"
    
    # Precision
    bf16: bool = False  # CPU training
    fp16: bool = False  # CPU training
    
    # Logging
    logging_steps: int = 10
    save_strategy: str = "steps"
    save_steps: int = 100
    save_total_limit: int = 3
    
    # Evaluation
    eval_strategy: str = "steps"
    eval_steps: int = 100
    
    # Misc
    seed: int = 42
    dataloader_num_workers: int = 4
    report_to: str = "none"  # No wandb for now


@dataclass
class DatasetConfig:
    """Dataset configuration."""
    # Paths
    data_dir: str = "./data"
    train_file: str = "train.jsonl"
    val_file: str = "val.jsonl"
    test_file: str = "test.jsonl"
    holdout_file: str = "holdout.jsonl"
    
    # Generation
    num_samples: int = 1000
    temperature: float = 0.7
    top_p: float = 0.9
    
    # Splits
    train_ratio: float = 0.8
    val_ratio: float = 0.1
    test_ratio: float = 0.05
    holdout_ratio: float = 0.05


@dataclass
class DistillationConfig:
    """Distillation configuration."""
    # Response distillation
    response_distillation: bool = True
    
    # Logit distillation (if teacher logits available)
    logit_distillation: bool = False
    alpha: float = 0.5  # Weight for distillation loss
    
    # Temperature
    temperature: float = 2.0


@dataclass
class CurriculumConfig:
    """Curriculum learning configuration."""
    # Levels
    levels: list = field(default_factory=lambda: [
        {"name": "basic", "description": "Simple instructions and facts"},
        {"name": "intermediate", "description": "Multi-step tasks"},
        {"name": "advanced", "description": "Code, math, reasoning"},
        {"name": "hard", "description": "Complex problems"},
        {"name": "domain", "description": "Target domain tasks"},
    ])
    
    # Progression
    current_level: int = 0
    level_threshold: float = 0.8  # Accuracy to advance


@dataclass
class ExperimentConfig:
    """Experiment tracking."""
    experiment_id: str = "exp_001"
    description: str = "First training cycle"
    git_commit: Optional[str] = None
    
    # Hardware
    hardware: str = "AMD Ryzen 7 5700X3D, 32GB RAM"
    
    # Checkpoints
    best_loss: Optional[str] = None
    best_general: Optional[str] = None
    best_reasoning: Optional[str] = None
    best_code: Optional[str] = None
    best_overall: Optional[str] = None


@dataclass
class CoscaTrainingConfig:
    """Main training configuration."""
    model: ModelConfig = field(default_factory=ModelConfig)
    training: TrainingConfig = field(default_factory=TrainingConfig)
    dataset: DatasetConfig = field(default_factory=DatasetConfig)
    distillation: DistillationConfig = field(default_factory=DistillationConfig)
    curriculum: CurriculumConfig = field(default_factory=CurriculumConfig)
    experiment: ExperimentConfig = field(default_factory=ExperimentConfig)
    
    def __post_init__(self):
        """Create directories."""
        os.makedirs(self.training.output_dir, exist_ok=True)
        os.makedirs(self.dataset.data_dir, exist_ok=True)


def get_config() -> CoscaTrainingConfig:
    """Get training configuration."""
    return CoscaTrainingConfig()
