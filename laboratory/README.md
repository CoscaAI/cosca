# COSCA Laboratory

> **Profissional e autocontido.** Área de pesquisa isolada do COSCA de produção.
> Concentra **treinamento**, **experimentos**, **testes**, **diagnóstico** e **artefatos**,
> sem quebrar nem mover a estrutura existente de produção.

```
PROJECT_ROOT (repo COSCA — código estável, dados persistentes, runtime)
└── laboratory/          ← LAB_ROOT (esta área de pesquisa)
    ├── config/          ← paths resolver, environments, profiles
    ├── training/        ← datasets, runs, checkpoints, adapters/lora, exports, manifests
    ├── experiments/     ← active / completed / archived
    ├── tests/           ← unit / integration / e2e / regression / benchmark / fixtures
    ├── logs/            ← training / tests / runtime / build / errors
    ├── traces/          ← sessions / execution / performance / failures
    ├── reports/         ← training / tests / benchmarks / security / experiments
    ├── artifacts/       ← builds / packages / models / temporary
    ├── snapshots/       ← before / after / known-good
    └── tmp/             ← cache / work / staging   (descartável)
```

---

## 1. Path policy (canônico)

**Todas** as rotas derivam de `LAB_ROOT = PROJECT_ROOT/laboratory`. **Nunca** espalhe
strings `laboratory/...` pelo código — resolva pelo resolver único:

```python
import sys, os
sys.path.insert(0, os.path.join(os.path.dirname(__file__)))   # laboratory/
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "config"))  # config/
from paths import Paths, ensure_canonical_dirs   # vivem em laboratory/config/paths/

lab = Paths()                       # LAB_ROOT (inferido do git top-level, ou $COSCA_LAB_ROOT)
lab.training.datasets / "run-001"   # → <LAB_ROOT>/training/datasets/run-001
lab.experiments.active
lab.reports.training / "campaign-002.md"

ensure_canonical_dirs()             # idempotente
```

- Raiz do projeto: `$COSCA_LAB_PROJECT_ROOT`, senão `git rev-parse --show-toplevel`.
- Raiz do lab: `$COSCA_LAB_ROOT` (relativo ao projeto ou absoluto), senão `laboratory/`.
- **Nunca** paths absolutos hardcoded. **Nunca** `laboratory/...` fora do resolver.

---

## 2. Finalidade de cada diretório

| Diretório | Finalidade |
|---|---|
| `config/paths/` | Resolver canônico de paths (única fonte de verdade). |
| `config/environments/` | Perfis de ambiente (local / colab / gpu / cloud) — variáveis e tooling. |
| `config/profiles/` | Perfis de execução (treino/merge/aval) — hyperparams nomeados. |
| `training/datasets/` | Datasets SFT e splits (`.jsonl`). |
| `training/runs/` | Uma pasta por **run** de treino (id único), com manifest + config + resultado. |
| `training/checkpoints/` | Checkpoints de treino. |
| `training/adapters/lora/` | Adapters **LoRA** treinados (base + r/alpha). |
| `training/exports/` | Exports (GGUF, safetensors, etc.) gerados por `merge`. |
| `training/manifests/` | Manifests de reprodução (dataset+base+params+resultado). |
| `experiments/active/` | Experimentos **em andamento**. |
| `experiments/completed/` | Experimentos **concluídos** (com resultado). |
| `experiments/archived/` | Experimentos **arquivados** (concluídos e encerrados). |
| `tests/*` | Testes de unidade/integração/e2e/regressão/benchmark + fixtures. |
| `logs/*` | Logs por área (training/tests/runtime/build/errors). |
| `traces/*` | Traces de execução (sessions/execution/performance/failures). |
| `reports/*` | Relatórios finais por área. |
| `artifacts/builds/` | Builds gerados. |
| `artifacts/packages/` | Pacotes (zip, colab package, etc.). |
| `artifacts/models/` | Modelos base + derivados (pesados). |
| `artifacts/temporary/` | Artefatos **descartáveis**. |
| `snapshots/before/` | Estado **antes** de uma operação. |
| `snapshots/after/` | Estado **depois**. |
| `snapshots/known-good/` | **Conhecido-bom** — **somente leitura** para a rotina normal. |
| `tmp/cache|work|staging/` | **Descartável** — pode ser apagado a qualquer momento. |

> **Legado coexiste (não movido, regra 001):** `train/` (scripts LoRA) e `training/`
> (pipeline genérico) e `data/` e `models/` e `campaign-001|002/` **já existiam** e
> continuam no lugar. Mapeiam assim: `data/`→`training/datasets/`, `models/`→`artifacts/models/`
> e `training/adapters/lora/`, `campaign-*/`→`experiments/(completed|archived)/`. **Não foram
> movidos para preservar os paths usados pelos scripts atuais.** A migração é opt-in.

---

## 3. Convenção de nomes

- **kebab-case** para dirs (`known-good`, `train-run`, `campaign-002`).
- **Run de treino:** `run-<campaign>-<YYYYMMDD>-<hash6>` (ex.: `run-002-20260904-a1b2c3`).
- **Dataset:** `<campaign>.<split>.jsonl` (ex.: `campaign-002.train.jsonl`).
- **Adapter LoRA:** `<base>-<r>r<alpha>a` (ex.: `qwen3-4b-16r32a`).
- **Manifest/pacote por run:** mesmo id do run.

---

## 4. Como iniciar um experimento

1. `mkdir laboratory/experiments/active/<exp-id>` (ou crie via resolver).
2. Colete config em `chemical/config/`? **Não** — cole no run: `config/profiles/<exp-id>.yaml`.
3. Registre em `experiments/active/<exp-id>/` um `manifest.yaml` com: meta, config,
   dataset, seed, dependências.
4. Ao concluir: mova para `experiments/completed/` e escreva `report.md` + resultado.

## 5. Como iniciar um treinamento (ex.: LoRA Campaign 002)

```bash
# 1. Resolver a árvore (idempotente) — opcional, os dirs já existem
python -c "import sys; sys.path.insert(0,'laboratory/config'); from paths import ensure_canonical_dirs; ensure_canonical_dirs()"

# 2. Validar dataset+config (dry-run, sem GPU)
python laboratory/train/lora_qwen3_4b.py \
  --data laboratory/data/dataset-curado-002.sft.jsonl.train.jsonl --dry-run

# 3. Treinar (GPU NVIDIA ~8GB)
python laboratory/train/lora_qwen3_4b.py \
  --data laboratory/data/dataset-curado-002.sft.jsonl.train.jsonl \
  --epochs 3 --lora-r 16 --lora-alpha 32
```

Cada run grava em `training/runs/<run-id>/`: `config.yaml`, `manifest.yaml`, logs e o
resultado (`metrics.json`). **Não sobrescreve** runs anteriores (um run = um id).

## 6. Onde ficam checkpoints e LoRA

- **Checkpoints:** `training/checkpoints/<run-id>/`
- **LoRA (adapter):** `training/adapters/lora/<base>-<r>r<alpha>a/`
- **Exports (GGUF/safetensors):** `training/exports/<run-id>/`
- **Modelos base/derivados (pesados):** `artifacts/models/`

## 7. Onde ficam logs e traces

- **Logs:** `logs/{training,tests,runtime,build,errors}/` — nome `<run-id>.<YYYYMMDD>.log`,
  sempre com timestamp e id da execução.
- **Traces:** `traces/{sessions,execution,performance,failures}/` — permitem reconstruir a
  sequência de execução de um experimento/teste.

## 8. Como reproduzir uma execução

Cada diretório de run/experimento contém `manifest.yaml` com tudo que é preciso
para reproduzir: dataset (hash), base/model, hyperparams (r/alpha/epochs/lr), seed e
hash do código (`git rev-parse HEAD`). Basta: `checkout` no commit do manifest →
`python <script> --data <dataset> <params>`.

## 9. O que pode ser apagado com segurança (descartável)

- `tmp/` (cache, work, staging) — a qualquer momento.
- `logs/`, `traces/` antigos (se não precisar do histórico).
- `artifacts/temporary/`.
- `artifacts/builds/` e `artifacts/packages/` (regeneráveis).

## 10. O que é protegido

- **`snapshots/known-good/`** — **somente leitura** para a rotina normal do laboratório.
  Nunca sobrescrever sem decisão explícita.
- **`snapshots/before/` e `after/`** — referência de estado; não regravar
  silenciosamente.
- **`training/manifests/`** e `manifest.yaml` de cada run — fonte de reprodução.
- **`experiments/completed/`** e `archived/` — resultados; não sobrescrever.
- **`config/paths/`** — o resolver; não mover.

## 11. Retenção

- **Runs de treino:** manter todos (`un run = um id`, nunca sobrescrever).
- **Checkpoints:** manter os 3 melhores (ou `save_total_limit`), descartar intermediários.
- **Logs/traces:** rotacionar; manter o período ativo da campanha.
- **tmp/**: 0 retenção (descartável).
- **artifacts/packages**: manter o pacote da campanha, descartar o resto.
- **known-good**: retenção indefinida (é a base de referência).

---

> **Regra de ouro:** produção (código estável, dados persistentes, runtime) **não** é
> tocada por este laboratório. O laboratório é limpo/movido **sem risco** para produção —
> nada aqui referencia paths de produção, e nada de produção referencia o laboratório
> (auditado: nenhum `laboratory/` no código de `cmd/internal/pkg/api/scripts`).
