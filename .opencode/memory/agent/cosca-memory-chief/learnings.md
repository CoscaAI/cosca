# Memory Chief — Aprendizados Registrados

> Memória semântica do agente cosca-memory-chief. Registro após cada tarefa de mineração/pesquisa.
> Evolução pessoal (nível/auto-evolução) e lições reutilizáveis.

## Nível de evolução
- Nível 3+ (objetivo): executar o protocolo de auto-evolução e registrar aprendizado estruturado.

---

## [2026-09-01] Mineração de diamantes: huggingface/datasets + peft → COSCa (Go)

### Contexto da tarefa
Minerei 2 repos clonados para achar padrões técnicos aplicáveis ao COSCa (assistente Go, 100% local,
memória episódica+semântica em disco, cérebro via Ollama/qwen2.5-coder, Windows/AMD). Objetivo: cache/armazenamento
eficiente de memória + caminho de fine-tune param-eficiente (LoRA) sem custo.

### Aprendizados-chave (reutilizáveis)

**Padrões de cache/fingerprint (datasets):**
- Fingerprint determinístico = hash (xxhash64) do estado do dataset + o `mtime` de cada cache_file.
  Um dataset só é "recomputável" se o fingerprint bate; transformações têm `@version` manual p/ invalidar cache.
  → Aplicar em COSCa: gerar um content-hash da "receita" de um artefato de memória (schema + query + versão de transform)
    e usar como nome de arquivo de cache. Windows: validar chars inválidos no path (filename limit 64 chars).
- `Hasher` aceita objetos python arbitrários via **dill**; falha ao serializar → cai p/ hash aleatório (consistência perdida).
  → Em Go não há dill: basta hashear bytes canônicos (JSON com chaves ordenadas) dos metadados.

**Armazenamento Arrow/Parquet (datasets):**
- `MemoryMappedTable` (table.py) só guarda `path` + lista de "replays" (transformações a re-aplicar ao ler).
  Pickle NÃO copia dados; custo de RAM ~zero, lê do disco sob demanda. Perfeito p/ não estourar RAM.
- `read_schema_from_file`: lê schema via `pa.memory_map` sem carregar o arquivo inteiro.
- Parquet: row groups ~100MB (uncompressed), shards ~500MB; default compression config.
- `Features` (features.py): schema tipado (Value/Sequence/List/ClassLabel/Json...) → `arrow_schema`.
  Round-trip: `Features.from_arrow_schema` reconstrói features a partir de metadados embutidos no schema.
  → COSCa pode versionar o schema da memória (episódica vs semântica) e distinguir colunas que exigem decode.

**Streaming para não estourar RAM (iterable_dataset.py):**
- Cadeia de `*ExamplesIterable` (lazy): cada transform é um iterável que emite um exemplo por vez.
- `BufferShuffledExamplesIterable`: buffer de N exemplos + amostragem aleatória incremental (shuffle aproximado
  online). Estratégia clássica p/ stream sem carregar tudo.
- `map`/`filter`/`shuffle` composição lazy; `_iter_arrow` processa em batch de RecordBatches.
- `__iter__` de Dataset usa `ARROW_READER_BATCH_SIZE_IN_DATASET_ITER = 10`.

**Fine-tune param-eficiente (peft):**
- LoRA: congela weights base; injeta `lora_A` (rank r, down) e `lora_B` (up) por módulo alvo.
  forward: `output += lora_B(lora_A(x)) * scaling`, com `scaling = lora_alpha/r` (ou `lora_alpha/sqrt(r)` p/ rslora).
  → Personalizar qwen2.5-coder treinando apenas ~0.1-1% dos parâmetros.
- `LoraConfig` (config.py): `r`, `target_modules` (regex/list/'all-linear'), `lora_alpha`, `lora_dropout`,
  `bias`, `fan_in_fan_out` (qwen/gpt2 Conv1D!), `layers_to_transform`, `rank_pattern`/`alpha_pattern` (por camada),
  `init_lora_weights`, `use_rslora`, `use_dora`, `modules_to_save`.
- Múltiplos adapters EMPILHÁVEIS no mesmo base model: `add_adapter`/`load_adapter`/`set_adapter` (ativa 1),
  `unload`/`merge`. → "Skins" por caso de uso (chat, code, visão) sem retreinar.
- `inject_adapter_in_model` (mapping.py): injeta camadas LoRA in-place sem retornar PeftModel.
- `PeftModel.from_pretrained`/`LoadAdapter`: guarda `adapter_config.json` + weights em safetensors.
- `AutoPeftModel` (auto.py): resolve a classe base a partir do `adapter_config.json` (auto_mapping) + import_allowlist.
- `set_peft_model_state_dict` (save_and_load.py): inserir pesos do adapter re-mapeando chaves por adapter_name.
- `MODEL_TYPE_TO_PEFT_MODEL_MAPPING` por `task_type` (CAUSAL_LM etc.).

### Próximas ações sugeridas
- Aprofundar em: arrow_writer (TyPodSequence/OptimizedTypedSequence, batched write), splits.py (SplitInfo),
  download/cache naming (hash_url_to_filename), e variants de LoRA (dora, piissa p/ low-rank sem SVD caro).
- Validar viabilidade de LoRA via Ollama/llama.cpp (em Go) — o PEFT é PyTorch; considerar GGUF adapters ou
  lm_head treinável. Registrar descoberta aqui.
