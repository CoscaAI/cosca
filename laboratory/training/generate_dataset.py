"""
Cosca Training Pipeline — Dataset Generator
===========================================
Uses teacher model (Ollama) to generate high-quality training data.
"""

import json
import os
import random
import time
from pathlib import Path
from typing import List, Dict, Any, Optional

import requests
from tqdm import tqdm

from config import get_config, CoscaTrainingConfig


# ============================================================================
# PROMPTS FOR DATA GENERATION
# ============================================================================

# Level 1: Basic instructions and facts
LEVEL_1_PROMPTS = [
    "Explique o que é uma API REST em poucas palavras.",
    "O que é um banco de dados relacional?",
    "Quais são os princípios SOLID?",
    "O que é Git e por que é usado?",
    "Explique a diferença entre HTTP e HTTPS.",
    "O que é um container Docker?",
    "Quais são os tipos principais de dados em Go?",
    "O que é uma função em programação?",
    "Explique o que é POO (Programação Orientada a Objetos).",
    "O que é um algoritmo?",
]

# Level 2: Multi-step tasks
LEVEL_2_PROMPTS = [
    "Crie uma função em Go que receba uma lista de inteiros e retorne a média.",
    "Escreva um script Python que leia um CSV e retorne as 5 maiores linhas.",
    "Projete um endpoint REST para cadastro de usuários com validação.",
    "Crie um teste unitário para uma função de soma.",
    "Escreva uma query SQL que retorne os 10 clientes mais ativos.",
    "Implemente uma fila simples com push e pop em Go.",
    "Crie um middleware de autenticação JWT em Go.",
    "Escreva uma função que valide CPF brasileiro.",
    "Projete um schema de banco para um sistema de pedidos.",
    "Crie um script de backup automático para SQLite.",
]

# Level 3: Code, math, reasoning
LEVEL_3_PROMPTS = [
    "Implemente um algoritmo de ordenação mergesort em Go com complexidade O(n log n).",
    "Resolva: Se f(x) = 2x² + 3x - 5, calcule f(3) e f(-2).",
    "Crie um programa que encontre o caminho mais curto em um grafo usando Dijkstra.",
    "Implemente uma árvore binária de busca com inserção, busca e remoção.",
    "Escreva uma função que determine se um número é primo de forma eficiente.",
    "Crie um parser de expressões matemáticas em Go.",
    "Implemente o padrão Observer em Go com channels.",
    "Resolva: Qual é o valor de x se log₂(x) + log₂(x-2) = 3?",
    "Crie um sistema de cache LRU com capacity limitada.",
    "Implemente um lock-free counter usando atomic operations em Go.",
]

# Level 4: Hard problems
LEVEL_4_PROMPTS = [
    "Implemente um compilador simples para uma linguagem de expressões aritméticas.",
    "Crie um sistema de transação distribuída com two-phase commit.",
    "Implemente um scheduler de tarefas com prioridades e preempção.",
    "Crie um sistema de rate limiter com sliding window.",
    "Implemente um algoritmo de compressão LZ77 simplificado.",
    "Crie um deadlock detector para sistemas de transação.",
    "Implemente um sistema de eventos com ordering guarantees.",
    "Crie um cache distribuído com consistência consistente.",
    "Implemente um sistema de streaming com backpressure.",
    "Crie um otimizador de query para SQL simplificado.",
]

# Level 5: Domain-specific (Cosca/AI)
LEVEL_5_PROMPTS = [
    "Explique como funciona um sistema de RAG (Retrieval Augmented Generation).",
    "Crie um agente de IA que utilize tool use para executar comandos.",
    "Implemente um sistema de memory para agentes de IA.",
    "Descreva como implementar um pipeline de evaluation para LLMs.",
    "Crie um sistema de content trust para validar respostas de IA.",
    "Implemente um orchestrador de agentes com fallback e retry.",
    "Crie um sistema de semantic memory com embeddings vetoriais.",
    "Descreva como implementar um sistema de capability discovery para agentes.",
    "Crie um pipeline de distillation de conhecimento entre modelos.",
    "Implemente um sistema de hard-example mining para treinamento.",
]

ALL_PROMPTS = {
    1: LEVEL_1_PROMPTS,
    2: LEVEL_2_PROMPTS,
    3: LEVEL_3_PROMPTS,
    4: LEVEL_4_PROMPTS,
    5: LEVEL_5_PROMPTS,
}


# ============================================================================
# TEACHER INTERFACE
# ============================================================================

class OllamaTeacher:
    """Interface for Ollama teacher model."""
    
    def __init__(self, config: CoscaTrainingConfig):
        self.endpoint = config.model.teacher_endpoint
        self.model = config.model.teacher_model
        self.temperature = config.dataset.temperature
        self.top_p = config.dataset.top_p
    
    def generate(self, prompt: str, system: str = None, max_tokens: int = 2048) -> Optional[str]:
        """Generate a response from the teacher."""
        payload = {
            "model": self.model,
            "prompt": prompt,
            "stream": False,
            "options": {
                "temperature": self.temperature,
                "top_p": self.top_p,
                "num_predict": max_tokens,
            }
        }
        
        if system:
            payload["system"] = system
        
        try:
            response = requests.post(
                f"{self.endpoint}/api/generate",
                json=payload,
                timeout=120
            )
            response.raise_for_status()
            return response.json().get("response", "")
        except Exception as e:
            print(f"Error generating response: {e}")
            return None
    
    def is_available(self) -> bool:
        """Check if teacher is available."""
        try:
            response = requests.get(f"{self.endpoint}/api/tags", timeout=5)
            return response.status_code == 200
        except:
            return False


# ============================================================================
# DATASET GENERATOR
# ============================================================================

class DatasetGenerator:
    """Generate training dataset using teacher model."""
    
    def __init__(self, config: CoscaTrainingConfig):
        self.config = config
        self.teacher = OllamaTeacher(config)
        self.data_dir = Path(config.dataset.data_dir)
        self.data_dir.mkdir(parents=True, exist_ok=True)
    
    def generate_sample(self, prompt: str, level: int) -> Optional[Dict[str, Any]]:
        """Generate a single training sample."""
        system = """Você é um assistente de IA altamente competente. 
Responda de forma clara, precisa e completa.
Se o pedido envolver código, forneça o código completo com explicações.
Se envolver matemática, mostre os passos da resolução."""
        
        response = self.teacher.generate(prompt, system=system)
        
        if not response or len(response.strip()) < 50:
            return None
        
        return {
            "messages": [
                {"role": "user", "content": prompt},
                {"role": "assistant", "content": response}
            ],
            "level": level,
            "prompt": prompt,
            "response_length": len(response),
        }
    
    def generate_dataset(self, num_samples: int = None) -> List[Dict[str, Any]]:
        """Generate the full dataset."""
        if num_samples is None:
            num_samples = self.config.dataset.num_samples
        
        if not self.teacher.is_available():
            print("ERROR: Teacher model not available at", self.config.model.teacher_endpoint)
            print("Please start Ollama with: ollama serve")
            return []
        
        print(f"Generating {num_samples} samples using teacher: {self.config.model.teacher_model}")
        
        samples = []
        prompts_per_level = num_samples // len(ALL_PROMPTS)
        
        for level, prompts in ALL_PROMPTS.items():
            print(f"\n--- Level {level} ---")
            level_samples = 0
            
            for prompt in tqdm(prompts, desc=f"Level {level}"):
                if level_samples >= prompts_per_level:
                    break
                
                sample = self.generate_sample(prompt, level)
                if sample:
                    samples.append(sample)
                    level_samples += 1
                
                # Rate limiting
                time.sleep(0.5)
        
        random.shuffle(samples)
        return samples
    
    def split_dataset(self, samples: List[Dict[str, Any]]) -> Dict[str, List]:
        """Split dataset into train/val/test/holdout."""
        n = len(samples)
        train_end = int(n * self.config.dataset.train_ratio)
        val_end = train_end + int(n * self.config.dataset.val_ratio)
        test_end = val_end + int(n * self.config.dataset.test_ratio)
        
        return {
            "train": samples[:train_end],
            "val": samples[train_end:val_end],
            "test": samples[val_end:test_end],
            "holdout": samples[test_end:],
        }
    
    def save_dataset(self, splits: Dict[str, List]):
        """Save dataset splits to files."""
        for split_name, split_data in splits.items():
            filepath = self.data_dir / f"{split_name}.jsonl"
            with open(filepath, "w", encoding="utf-8") as f:
                for sample in split_data:
                    f.write(json.dumps(sample, ensure_ascii=False) + "\n")
            print(f"Saved {len(split_data)} samples to {filepath}")
    
    def run(self, num_samples: int = None):
        """Run the full dataset generation pipeline."""
        print("=" * 60)
        print("COSCA TRAINING — DATASET GENERATION")
        print("=" * 60)
        
        # Check teacher
        if not self.teacher.is_available():
            print("\nERROR: Teacher model not available!")
            print("Please start Ollama: ollama serve")
            return
        
        print(f"\nTeacher: {self.config.model.teacher_model}")
        print(f"Endpoint: {self.config.model.teacher_endpoint}")
        print(f"Target samples: {num_samples or self.config.dataset.num_samples}")
        
        # Generate
        samples = self.generate_dataset(num_samples)
        
        if not samples:
            print("\nERROR: No samples generated!")
            return
        
        print(f"\nGenerated {len(samples)} samples")
        
        # Split and save
        splits = self.split_dataset(samples)
        self.save_dataset(splits)
        
        # Stats
        print("\n" + "=" * 60)
        print("DATASET STATISTICS")
        print("=" * 60)
        for split_name, split_data in splits.items():
            levels = {}
            for s in split_data:
                level = s.get("level", 0)
                levels[level] = levels.get(level, 0) + 1
            print(f"{split_name}: {len(split_data)} samples, levels: {levels}")
        
        print("\nDataset generation complete!")


# ============================================================================
# MAIN
# ============================================================================

if __name__ == "__main__":
    config = get_config()
    generator = DatasetGenerator(config)
    
    # Generate a small dataset first (50 samples)
    generator.run(num_samples=50)
