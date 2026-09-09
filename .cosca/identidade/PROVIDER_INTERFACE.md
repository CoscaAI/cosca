# INTERFACE DE PROVIDER — Abstracao de Provedores de IA

> **Versao**: 1.1.0 | **Status**: active | **Dono**: Cosca Kernel | **Ultima Atualizacao**: 2026-08-01

## PROPOSITO
Este documento define a interface padronizada para provedores de modelos de IA no ecossistema Cosca. Permite:
- **Agnosticismo de provider** — Trocar provedores de IA sem mudar logica de agentes
- **Failover de provider** — Fallback automatico quando o provedor primario falha
- **Otimizacao de custo** — Rotear tarefas para o provedor mais barato capaz
- **Orquestracao multi-modelo** — Usar provedores diferentes para tipos de tarefa diferentes

## ARQUITETURA

```
┌──────────────────────────────────────────────────┐
│              CAMADA DE AGENTES COSCA               │
│  (Chefes, Especialistas, Engines)                │
│  Agentes pedem: "complete esta tarefa"           │
│  Agentes NAO sabem qual provedor e usado          │
└──────────────────────┬───────────────────────────┘
                       │
              ┌────────▼────────┐
              │ ROUTER PROVIDER │
              │ (seleciona melhor│
              │  provider por   │
              │  tipo de tarefa)│
              └────────┬────────┘
                       │
     ┌─────────────────┼─────────────────┐
     │                 │                 │
┌────▼─────┐   ┌──────▼──────┐   ┌─────▼──────┐
│PRIMARIO  │   │SECUNDARIO   │   │ FALLBACK   │
│ Provider │   │ Provider    │   │ Provider   │
│ (OpenAI) │   │(Anthropic)  │   │ (LLM Local)│
└──────────┘   └─────────────┘   └────────────┘
```

## 1. REGISTRO DE PROVIDERS

### 1.1 Configuracao do Provider

```yaml
providers:
  - id: openai-gpt4
    name: OpenAI GPT-4o
    type: cloud
    priority: 1                    # Menor = maior prioridade
    models:
      - gpt-4o
      - gpt-4o-mini
    capabilities:
      - code_generation
      - code_review
      - architecture_design
      - creative_writing
      - analysis
    cost_per_1k_tokens:
      input: 0.005
      output: 0.015
    rate_limit:
      requests_per_minute: 500
      tokens_per_minute: 100000
    timeout_ms: 240000
    retry:
      max_attempts: 3
      backoff_ms: 5000
```

### 1.2 Matriz de Capacidades do Provider

| Capacidade | GPT-4o | Claude 3.5 | Llama 3 | DeepSeek | Gemini |
|-----------|--------|------------|---------|----------|--------|
| code_generation | Sim | Sim | Sim | Sim | Sim |
| code_review | Sim | Sim | Aviso | Sim | Aviso |
| architecture_design | Sim | Sim | Nao | Sim | Aviso |
| creative_writing | Sim | Sim | Aviso | Aviso | Sim |
| analysis | Sim | Sim | Sim | Sim | Sim |
| long_context | Aviso | Sim | Nao | Sim | Sim |
| security_audit | Sim | Sim | Nao | Sim | Nao |
| test_generation | Sim | Sim | Sim | Sim | Sim |
| documentation | Sim | Sim | Sim | Aviso | Sim |

### 1.3 Modalidade de Audio (TTS/STT)

O framework Cosca tambem abstrai uma modalidade de **audio** para interacao por voz local. Providers sao APENAS locais — nenhum servico de audio em nuvem:

| Provider | Modalidade | Modelo | Licenca | Notas |
|----------|-----------|--------|---------|-------|
| Kokoro | TTS | Kokoro-82M | Apache-2.0 | Tempo real em CPU, PT-BR nativo (voice code `p`) |
| whisper.cpp | STT | ggml-base / whisper models | MIT | CPU via OpenBLAS, mais rapido que tempo real |

- Roteamento e failover seguem a mesma abstracao de provider definida neste documento.
- Audio nunca sai da maquina — 100% local.
- Gerenciado pelo [Voice Engine](../engines/knowledge/SKILL.md); veja a capacidade `voice.local` no [KERNEL.md](KERNEL.md).

## 2. REGRAS DE ROTEAMENTO DE TAREFAS

### 2.1 Tipo de Tarefa → Selecao de Provider

| Tipo de Tarefa | Primario | Secundario | Fallback | Temperature |
|----------------|----------|-----------|----------|-------------|
| Estrategico (CEO) | GPT-4o | Claude 3.5 | — | 0.7 |
| Planejamento (CTO, Produto) | GPT-4o | Claude 3.5 | — | 0.5 |
| Arquitetura | Claude 3.5 | GPT-4o | — | 0.5 |
| Geracao de Codigo | Claude 3.5 | GPT-4o | Llama 3 | 0.3 |
| Revisao de Codigo | GPT-4o | Claude 3.5 | — | 0.2 |
| Auditoria de Seguranca | GPT-4o | Claude 3.5 | — | 0.1 |
| Testes | Claude 3.5 | GPT-4o | Llama 3 | 0.2 |
| Documentacao | GPT-4o | Claude 3.5 | Llama 3 | 0.3 |
| Tarefas Simples | Llama 3 | GPT-4o-mini | Claude Haiku | 0.1 |
| Criativo | GPT-4o | Claude 3.5 | — | 0.7 |

### 2.2 Logica de Failover

```
1. Tentar provider PRIMARIO
   ├── Sucesso → retornar resultado
   └── Falha (timeout, rate limit, erro)
       2. Tentar provider SECUNDARIO
          ├── Sucesso → retornar resultado, registrar evento failover
          └── Falha
              3. Tentar provider FALLBACK
                 ├── Sucesso → retornar resultado, registrar evento double-failover
                 └── Falha → escalar para Kernel
```

### 2.3 Circuit Breaker

```
Estado: CLOSED → (falhas > 5 em 60s) → OPEN (rejeitar tudo por 30s)
Estado: OPEN → (30s decorridos) → HALF_OPEN (permitir 1 requisicao sonda)
Estado: HALF_OPEN → (sucesso) → CLOSED | (falha) → OPEN
```

## 3. INTERFACE API DO PROVIDER

### 3.1 Formato da Requisicao (Agnostico de Provider)

```json
{
  "provider_id": "openai-gpt4",
  "model": "gpt-4o",
  "messages": [
    { "role": "system", "content": "Voce e o Chefe de Arquitetura Cosca..." },
    { "role": "user", "content": "Projete uma arquitetura de microsservicos para..." }
  ],
  "temperature": 0.5,
  "max_tokens": 4096,
  "tools": ["read_file", "write_file"],
  "metadata": {
    "agent": "cosca-architecture",
    "department": "architecture",
    "workflow_id": "wf-001",
    "task_id": "task-042"
  }
}
```

### 3.2 Formato da Resposta (Agnostico de Provider)

```json
{
  "provider_id": "openai-gpt4",
  "model": "gpt-4o",
  "content": "Baseado nos requisitos...",
  "tool_calls": [
    { "tool": "write_file", "arguments": { "path": "...", "content": "..." } }
  ],
  "usage": {
    "prompt_tokens": 1500,
    "completion_tokens": 800,
    "total_tokens": 2300,
    "cost_usd": 0.0195
  },
  "duration_ms": 4500,
  "finish_reason": "stop"
}
```

### 3.3 Resposta de Erro

```json
{
  "provider_id": "openai-gpt4",
  "error": {
    "code": "rate_limit_exceeded",
    "message": "Rate limit excedido. Retry apos 30s.",
    "retry_after_ms": 30000,
    "failover_triggered": true,
    "failover_provider": "anthropic-claude"
  }
}
```

## 4. RASTREAMENTO DE CUSTOS

### 4.1 Evento de Custo

```json
{
  "timestamp": "2026-07-12T10:30:00Z",
  "provider": "openai-gpt4",
  "model": "gpt-4o",
  "agent": "cosca-architecture",
  "task_type": "architecture_design",
  "tokens": { "prompt": 1500, "completion": 800 },
  "cost_usd": 0.0195,
  "session_id": "sess-001",
  "workflow_id": "wf-001"
}
```

### 4.2 Regras de Otimizacao de Custo

| Regra | Acao |
|-------|------|
| Tarefa simples (mudanca < 100 LOC) | Rotear para provider mais barato capaz |
| Tarefa repetida (mesmo padrao) | Usar resultado cacheado se < 1h |
| Tarefa de contexto longo (> 10k tokens) | Preferir Claude (custo de input menor) |
| Processamento em lote | Agendar em unica requisicao quando possivel |
| Retries idempotentes | Reutilizar resultado anterior no failover |

## 5. MONITORAMENTO DE SAUDE DO PROVIDER

| Metrica | Threshold | Acao |
|---------|-----------|------|
| Taxa de erro | > 5% em 5 min | Ativar failover |
| Latencia p95 | > 30s | Registrar alerta, considerar secundario |
| Hits de rate limit | > 10 em 1 min | Reduzir concorrencia |
| Custo por sessao | > $5.00 | Alertar CTO |
| Circuit breaker abre | Qualquer | Alertar Monitoring Chief |

## 6. ESTENDENDO COM NOVOS PROVIDERS

### 6.1 Para Adicionar um Novo Provider:

1. Implementar a interface do adaptador do provider:
   - `complete(requisicao) → resposta`
   - `health_check() → status`
   - `get_models() → lista_modelos`
   - `get_cost(modelo, tokens) → custo_usd`
2. Registrar na configuracao YAML do provider
3. Adicionar a matriz de capacidades
4. Configurar regras de roteamento
5. Testar cadeia de failover

## RELACIONADOS
- [KERNEL.md](KERNEL.md) — Ponto de entrada da orquestracao
- [RUNTIME_CONTRACT.md](RUNTIME_CONTRACT.md) — Interface do runtime que consome providers
- [departments/ai/SKILL.md](../engines/knowledge/SKILL.md) — Chefe de IA (estrategia de selecao de provider)
- [departments/monitoring/SKILL.md](../engines/knowledge/SKILL.md) — Monitoramento de saude do provider
- [engines/observability/SKILL.md](../engines/knowledge/SKILL.md) — Metricas de custo e latencia
- [engines/voice/SKILL.md](../engines/knowledge/SKILL.md) — Voice Engine (modalidade de audio TTS/STT)

## HISTORICO

| Versao | Data | Autor | Mudancas |
|--------|------|-------|----------|
| 1.0.0 | 2026-07-12 | Cosca Kernel | Interface inicial de provider — abstracao, failover, rastreamento de custos |
| 1.1.0 | 2026-08-01 | Voice Engine | Adicionada modalidade de audio (TTS/STT) com providers locais Kokoro e whisper.cpp |
