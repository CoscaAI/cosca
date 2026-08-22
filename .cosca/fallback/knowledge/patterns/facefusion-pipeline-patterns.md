# FaceFusion Pipeline & Job System Patterns

> **Source**: https://github.com/facefusion/facefusion — MIT/OpenRAIL-AS, ~246 arquivos, Python 3, ONNX Runtime  
> **Analyzed**: 2026-08-09 — Análise profunda cross-agent (2 agentes em paralelo)  
> **Confidence**: 0.93 (validado por leitura direta de código fonte)

## Intent

Extrair padrões reutilizáveis de uma plataforma de manipulação facial com pipeline de processadores plugáveis, sistema de jobs baseado em arquivos, e aceleração GPU multi-provider.

## Context

FaceFusion é a plataforma líder de manipulação facial (face swap, enhancement, lip sync, age modification, etc.) com:
- **Pipeline pipe-and-filter**: 11 processadores plugáveis com contrato rígido
- **Sistema de jobs**: Fila persistente baseada em arquivos (JSON em diretórios de status)
- **Multi-GPU**: 9 execution providers (CUDA, TensorRT, ROCm, CoreML, OpenVINO, DirectML, MIGraphX, QNN, CPU)
- **Dual mode**: CLI headless/batch + Gradio UI
- **ONNX Runtime**: Inferência cross-platform sem dependência de framework específico

## Patterns Extraídos (Top 10 para Cosca)

### 1. Processor Pipeline — Pipe-and-Filter Sequencial

**Como funciona**: Cada processador implementa 9 métodos obrigatórios. O pipeline executa `process_frame()` em sequência: a saída (frame, máscara) de um processador é a entrada do próximo. Carregamento dinâmico via `importlib`.

**Código-chave** (`workflows/core.py`):
```python
for processor_module in get_processors_modules(state_manager.get_item('processors')):
    temp_vision_frame, temp_vision_mask = processor_module.process_frame({
        'temp_vision_frame': temp_vision_frame,
        'temp_vision_mask': temp_vision_mask,
        ...
    })
```

**Contrato** (`processors/core.py`):
```python
PROCESSORS_METHODS = [
    'get_inference_pool', 'clear_inference_pool', 'register_args',
    'apply_args', 'get_common_modules', 'pre_check',
    'pre_process', 'post_process', 'process_frame'
]
```

### 2. File-Based Job Queue — Zero Dependencies

**Como funciona**: Jobs são arquivos JSON movidos entre diretórios de status:
```
.jobs/
  drafted/   →  queued/   →  completed/
                         →  failed/  →  queued/ (retry)
```

**Por que é genial**: Zero dependência de banco de dados. Operação atômica = `mv` entre diretórios. Humano-legível. Resumível após crash. Step individual tracking dentro do job.

### 3. Step/Job Key Split Registry

**Como funciona**: Cada arg de CLI se registra como `job_key` (global: execution, memory, download) ou `step_key` (por passo: source, target, processors). Na execução, `reduce_step_args()` e `reduce_job_args()` separam automaticamente.

### 4. Dual-Context State Management

**Como funciona**: Estado dividido em `cli` e `ui`. Detecção automática de contexto por stack frame inspection (`sys._getframe`). `init_item` escreve em ambos, `set_item`/`get_item` no contexto atual, `sync_item` copia UI→CLI.

### 5. ONNX Runtime Multi-Provider com Fallback

**Como funciona**: 9 execution providers mapeados. Cada processador pode `override_inference_providers()` (trocar GPU) ou `adjust_inference_providers()` (tunar parâmetros). Provider ordering com fallback automático. Cache de engine TensorRT/CoreML.

### 6. FFmpeg Functional Command Builder

**Como funciona**: Funções puras que retornam `List[str]`, compostas com `chain()` (concatena) e `concat()` (merge de flags duplicadas por vírgula). Mapeamento de presets por encoder (NVENC/AMF/QSV).

### 7. 3-Tier Memory Strategy

**Como funciona**: `strict` (limpa tudo após cada step), `moderate` (limpa processador atual), `tolerant` (mantém tudo em memória). Controla ciclo de vida das inference sessions ONNX.

### 8. Error Code Propagation (0-4)

**Como funciona**: Cada task retorna `ErrorCode` (0=ok, 1=falha, 2=pre-check, 3=NSFW, 4=stopped). Pipeline aborta no primeiro erro > 0. `process_manager.end()` garante limpeza mesmo em abort.

### 9. CRC32 Model Integrity + Curl Resume Download

**Como funciona**: Cada `.onnx` tem `.hash` (CRC32). Download via curl com `--continue-at -` (resume), `--retry 5`, `--connect-timeout 5`. Providers: GitHub + HuggingFace com fallback automático.

### 10. argparse Modular Composition

**Como funciona**: ~25 funções `create_*_program()` cada uma retornando `ArgumentParser(add_help=False)`. Compostas via `parents=[...]`. Processadores registram seus próprios args dinamicamente.

## Tags

`#facefusion` `#pipeline` `#processors` `#job-system` `#onnx` `#gpu` `#ffmpeg` `#state-management` `#patterns` `#cross-agent-analysis` `#level-4`

---

*Análise conduzida por 2 agentes Cosca em paralelo. Extração e síntese pelo cosca-kernel. 2026-08-09.*
