# COSCA — Campanha 002 no Kaggle (GPU free)

Este guia destrava a **Campanha 002** (LoRA Qwen3-4B + merge GGUF `q4_k_m`) que está
bloqueada pela cota do Colab. O notebook `kaggle_campaign002.ipynb` é **autocontido**
(não depende de upload de dataset — os 15 exemplos SFT estão embutidos inline) e roda de
ponta a ponta no **GPU free do Kaggle** (~30h/semana).

> **Arquivos desta campanha**
> - `campaign-002.json` — config canônica (não alterar)
> - `kaggle_campaign002.ipynb` — **novo**: notebook autocontido p/ Kaggle
> - `kaggle_HOWTO.md` — **novo**: este guia
> - `worker_colab.py` / `dataset.jsonl` — worker Colab (referência)

---

## 1. Requisitos de conta

- Conta **Kaggle** gratuita (https://www.kaggle.com). Funciona até *without verification*,
  mas verifique sua conta (Settings → **Phone verification**) para **garantir 30h/semana de GPU**
  (T4x2). Sem verificação o limite é menor.
- Nenhuma chave é necessária para rodar no **browser** (caminho default).

## 2. Opção A — Rodar no browser (recomendado, sem chave)

1. Acesse https://www.kaggle.com/code → **New Notebook**.
2. No editor, **File → Import Notebook** e selecione `kaggle_campaign002.ipynb`.
   - _(alternativa) copie o conteúdo `.ipynb` e cole/faça upload do arquivo._
3. Configure as **Settings** (painel à direita):
   - **Accelerator:** `GPU T4x2` (ou `GPU P100`). Ambos 16 GB — suficiente p/ QLoRA de 4B.
   - **Internet:** `ON` (para `pip install` e download dos pesos de `Qwen/Qwen3-4B`).
   - **Language:** `Python 3`.
4. Rode as células **em ordem** (`Run all`):
   - `Setup` (instala deps) → `Data` (embute dataset) → `Train` (LoRA/QLoRA) → `Merge+GGUF` → `Save`.
5. Ao terminar, o artefato está em `/kaggle/working/output/`.

Tempo estimado: dependendo do T4x2, o treino de 15 exemplos × 3 epochs é rápido
(minutos a dezenas de minutos) — domina o download inicial do base + instalação do `unsloth`.

## 3. Opção B — `kaggle kernels push` (automação, opcional)

Você precisará de credenciais `/secrets` (decida se quer):

1. Instale & autentique:
   ```bash
   pip install -q kaggle
   echo '{"username":"SEU_USER","key":"SUA_KEY"}' > ~/.kaggle/kaggle.json
   chmod 600 ~/.kaggle/kaggle.json
   ```
2. Crie um `kernel-metadata.json` na pasta do notebook:
   ```json
   {
     "id": "SEU_USER/campaign-002-qwen3-4b-lora",
     "title": "campaign-002-qwen3-4b-lora",
     "code_file": "kaggle_campaign002.ipynb",
     "language": "python",
     "kernel_type": "notebook",
     "is_private": true,
     "enable_gpu": true,
     "enable_internet": true,
     "competition_sources": []
   }
   ```
3. Push:
   ```bash
   kaggle kernels push -p .    # pasta onde está o metadata + .ipynb
   ```
4. Acompanhe: `kaggle kernels status SEU_USER/campaign-002-qwen3-4b-lora` (ou no site).

> ⚠️ Para a Opção B você precisa de `KAGGLE_USERNAME`/`KAGGLE_KEY`. Por segurança, **NÃO** coloque
> chave dentro do notebook — o run é da *sua* conta. Se preferir, mantenha o caminho default (browser).

## 4. Como baixar o output

- **Browser:** no notebook aberto, painel **Output** (direita) → selecione
  `output/campaign-002-qwen3-4b-lora.gguf` → **Download** (ou marque + *Download selected*).
- **CLI (se usou push):**
  ```bash
  kaggle kernels output SEU_USER/campaign-002-qwen3-4b-lora -p ./kaggle-out
  ```
- Baixe também `output/adapter-campaign-002/` e `output/WORKER_REPORT.json` se quiser o adaptador/relatório.

Salve o `.gguf` em `laboratory/campaign-002/output/` no seu PC.

## 5. Depois do download (validação AFTER)

1. Crie o `Modelfile` do Ollama com o modelo fundido (base `Qwen3-4B` = `campaign-002-qwen3-4b-lora.gguf`).
2. Carregue: `ollama create cosca-qwen3-4b-lora-002 -f Modelfile`.
3. Rode o **golden gate** do COSCA (`cosca dataset golden --model ...`).
   - O Kaggle **não decide promoção** — o worker só entrega `TRAINING_COMPLETE`; o COSCA valida.

---

## 6. Notas honestas / riscos

- **OOM (`max_seq_len=8192` no T4/P100):** `max_length=8192` é o **teto de truncamento** do
  `SFTConfig`, não o comprimento real dos exemplos. Os 15 exemplos são curtos (centenas de tokens),
  então o treino usa o comprimento real e **não há pad até 8192** — com
  `use_gradient_checkpointing="unsloth"` o risco de OOM é baixo. **Fallback:** se um dia um exemplo
  longo estourar a VRAM, reduza a variável `MAX_SEQ` (ex.: `2048`) na célula `Train`. Está documentado
  em `# [kaggle]` dentro da célula e no markdown do próprio notebook.
- **bf16:** T4/P100 (Turing/Pascal) **não suportam `bf16`**. O notebook força a fusão em
  `dtype=torch.float16` no merge (espelho do `merge_gguf.py`); o treino usa 4-bit com compute FP16.
- **Instabilidade de versão TRL:** o `SFTConfig` mudou o nome de `max_seq_length`→`max_length`
  (e `SFTTrainer` ganhou `processing_class`). O notebook usa **shims de compat** (`# [kaggle]`) que
  tentam a assinatura atual e caem para a antiga — cobre Kaggle e upgrades.
- **GPU é "1 destino":** uso `CUDA_VISIBLE_DEVICES=0` (1 GPU do T4x2). QLoRA de 4B cabe em um T4
  16 GB; não precisa de DDP.
- **Tempo:** o primeiro run baixa `Qwen/Qwen3-4B` (~8 GB em 4-bit) + instala `unsloth`
  (pode recompilar/baixar muitos wheels). Depois disso é rápido.

## 7. Ajustes únicos a olhar (`# [kaggle]`)

Só **2 pontos** são manuais no notebook:
1. `MAX_SEQ = 8192` → reduzir se OOM.
2. Nenhuma chave/segredo no notebook por design (**NÃO** adicione `KAGGLE_KEY` nele).
