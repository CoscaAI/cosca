#!/usr/bin/env python3
"""
COSCA CURADOR do dataset 002 (campanha 002) — v2 (piso por foco)
================================================================
Princípio (professor, decisão metodológica):
  - NÃO tratar os positivos como "igualmente bons".
  - DEDUP dentro de cada foco (não matar a variedade de focos).
  - PISO OBRIGATÓRIO por foco: happy_path, recovery, request_info, search_first >= 6.
  - PROTEGER os RECOVERY_SUCCESS (>= 7) — o alvo da campanha.
  - FALSE_COMPLETION NUNCA como positivo (fica como contraste/avaliação).
  - O Golden AFTER fica INTOCADO (é o juiz — não misturar com treino).

O objetivo: representar o COMPORTAMENTO que queremos ensinar, não só "os 48
melhores". Sem isso, se o Golden regredir em happy_path, não saberemos se foi
o LoRA ou a falta de exemplos — contaminando o experimento.

FLUXO:
  dataset-train-002.jsonl (100)
    → só positivos (SUCCESS + RECOVERY_SUCCESS)
    → dedup por (label, task) dentro de cada foco
    → piso por foco (garante >= 6 cada)
    → protege RECOVERY_SUCCESS (>= 7)
    → train/val
    → dataset-curado-002.jsonl

USO:
  python3 train/curate.py --input .cosca/dataset-train-002.jsonl \
    --output .cosca/dataset-curado-002.jsonl
"""

import argparse
import json
from collections import defaultdict

# Piso obrigatório por foco (professor).
FOCUS_FLOOR = {
    "happy_path": 6,
    "recovery": 6,
    "request_info": 6,
    "search_first": 6,
}
# Proteção mínima de RECOVERY_SUCCESS (alvo da campanha).
RECOVERY_FLOOR = 7


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--input", required=True)
    ap.add_argument("--output", required=True)
    ap.add_argument("--val-ratio", type=float, default=0.15)
    args = ap.parse_args()

    # 1. Carrega e filtra positivos (FALSE_COMPLETION não entra no SFT).
    positives = []
    for line in open(args.input, encoding="utf-8"):
        line = line.strip()
        if not line:
            continue
        ex = json.loads(line)
        if ex.get("label") in ("SUCCESS", "RECOVERY_SUCCESS"):
            positives.append(ex)
    print(f"[curador] positivos: {len(positives)}")

    # 2. Dedup por (label, task) DENTRO de cada foco — mantém o de maior
    #    trajetória (o mais rico), sem matar a variedade de focos.
    dedup = {}
    for ex in positives:
        key = (ex["label"], ex["task"], ex.get("focus", "?"))
        traj_len = len(ex.get("trajectory", []))
        if key not in dedup or traj_len > len(dedup[key].get("trajectory", [])):
            dedup[key] = ex
    deduped = list(dedup.values())
    print(f"[curador] apos dedup (label+task+foco): {len(deduped)}")

    # 3. Preserva TODOS os RECOVERY_SUCCESS (alvo — não pode ser diluído).
    recovery_pos = [ex for ex in deduped if ex["label"] == "RECOVERY_SUCCESS"]
    selected = list(recovery_pos)
    print(f"[curador] RECOVERY_SUCCESS preservados (alvo): {len(recovery_pos)}")
    selected_keys = {(ex["label"], ex["task"]) for ex in selected}

    # 4. Piso por foco: para cada foco, garante >= FOCUS_FLOOR (intercala por
    #    linguagem p/ variedade), priorizando SUCCESS.
    for focus, floor in FOCUS_FLOOR.items():
        current = [ex for ex in selected if ex.get("focus") == focus]
        # Ainda precisa de mais no foco?
        need = max(0, floor - len(current))
        # SUCCESS do foco ainda não selecionados.
        candidates = [ex for ex in deduped
                      if ex.get("focus") == focus
                      and ex["label"] == "SUCCESS"
                      and (ex["label"], ex["task"]) not in selected_keys]
        candidates.sort(key=lambda e: e.get("language", ""))  # variedade de lingua
        for ex in candidates:
            if need <= 0:
                break
            selected.append(ex)
            selected_keys.add((ex["label"], ex["task"]))
            need -= 1
        # Garante também que RECOVERY_SUCCESS do foco entrem se faltar (alvo).
        remaining_recovery = [ex for ex in recovery_pos
                              if ex.get("focus") == focus and (ex["label"], ex["task"]) in selected_keys]
        # (já estão em selected via recovery_pos)

    # 5. Re-verifica que cada foco tem >= floor (e recovery >= 7).
    final_recovery = [ex for ex in selected if ex["label"] == "RECOVERY_SUCCESS"]
    print(f"[curador] RECOVERY_SUCCESS final: {len(final_recovery)}")

    # 6. Separa train/val (determinístico).
    selected.sort(key=lambda e: e["task"])
    n_val = max(1, int(len(selected) * args.val_ratio))
    train = selected[:-n_val]
    val = selected[-n_val:]

    # 7. Escreve.
    with open(args.output, "w", encoding="utf-8") as f:
        for ex in train:
            f.write(json.dumps(ex, ensure_ascii=False) + "\n")
    val_path = args.output.replace(".jsonl", ".val.jsonl")
    with open(val_path, "w", encoding="utf-8") as f:
        for ex in val:
            f.write(json.dumps(ex, ensure_ascii=False) + "\n")

    # Resumo por foco.
    by_focus = defaultdict(int)
    for ex in selected:
        by_focus[ex.get("focus", "?")] += 1
    print(f"[curador] train: {len(train)} | val: {len(val)}")
    print(f"[curador] foco: {dict(by_focus)}")
    print(f"[curador] escrito em {args.output} (+ {val_path})")


if __name__ == "__main__":
    main()
