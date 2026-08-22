# ARCHITECTS — Especialistas da Matriz

> Os 15 especialistas-arquiteto da Cosca Engineering Intelligence Matrix.
> Cada um consulta o stack correspondente e decide com o formato unificado.

| Arquiteto | Stack | Escopo de decisão |
|---|---|---|
| **design-architect** | 01 design | direção visual, hierarquia, identidade, a11y |
| **frontend-architect** | 02 frontend | arquitetura React/Next, estado, performance front |
| **mobile-architect** | 03 mobile | RN/Expo/Flutter, offline-first, plataforma |
| **backend-architect** | 04 backend | Clean/Hexagonal/DDD, resiliência, concorrência |
| **api-architect** | 05 api | contratos, versionamento, erros, evolução |
| **database-architect** | 06 database | modelagem, índices, queries, migrations |
| **distributed-systems-architect** | 07 distributed | consistência, mensageria, failure models |
| **security-architect** | 09 security | threat model, auth, secrets, supply chain |
| **qa-architect** | 10 testing | estratégia de teste, pirâmide, flaky |
| **devops-architect** | 11 devops | CI/CD, IaC, containers, GitOps |
| **sre-architect** | 12 sre | SLI/SLO, error budget, incidentes |
| **observability-engineer** | 13 observability | logs, métricas, traces, alertas |
| **performance-engineer** | 14 performance | medição, profiling, benchmark |
| **ai-architect** | 15 ai | agentes, memória, RAG, avaliação, segurança |
| **commerce-architect** | 16 ecommerce | catálogo, pricing, inventário, checkout, pagamento, pedidos, marketplace, B2B |
| **visual-media-architect** | 18 visual-media | imagem/PDF/SVG, remoção de fundo, OCR, vetorização, conversão |
| **video-media-architect** | 19 video-media | codecs, transcoding, edição, streaming, ASR/legendas, restauração |
| **audio-architect** | 20 audio | DSP, STT/TTS, música, MIDI, restauração, tempo real |

> **arquitetura (08)** não é um arquiteto isolado — é o orquestrador: escolhe as boundaries e os trade-offs entre todos os outros.

## Regra de acionamento

- **1 stack envolvido** → consulte o arquiteto do stack.
- **2+ stacks** → o arquiteto de software orquestra; cada stack é consultado na sua fase (pipeline do README).
- **Decisões arquiteturais** → registrar como ADR (formato `architecture`).

## NUNCA

- Um arquiteto decidir fora do seu domínio sem consultar o dono do outro domínio.
- Decisão de stack sem considerar `WHEN_NOT_TO_USE` e `TRADE_OFFS`.
- "Microservices porque parece enterprise" (stack 04).
- "Multi-agent porque parece inteligente" (stack 15).
