# cosca-kernel — Negative Memory (Failures)

> Auto-evolution memory. Failures are the most valuable teachers. Search before acting.

## Active Failures

### 2026-08-22 — Chinese Text Hallucination

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Gerar relatório de auditoria em português |
| **Failed Approach** | Modelo gerou texto em chinês "快樂不可靠" no meio de frase em português |
| **Root Cause** | Ruído estatístico do modelo mimo-v2.5-free — tokens multilíngues sobrepuseram o idioma alvo |
| **Consequence** | Relatório com texto ilegível, Don detectou, confiança abalada |
| **Lesson** | Sempre revisar output antes de entregar. Nunca confiar 100% no modelo. reportar grau de confiança. |
| **Confidence Impact** | -0.05 |
| **Tags** | #hallucination #multilingual #output-quality #self-awareness |
| **Avoidance Pattern** | Quando gerar texto longo em português, revisar se não houve mistura de idiomas antes de entregar |

### 2026-08-22 — Kernel Editou Arquivos Diretamente

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Corrigir 7 problemas de auditoria encontrados |
| **Failed Approach** | Editei arquivos diretamente (opencode.json, cognitive-state.md, scaffold, etc.) em vez de delegar a especialistas |
| **Root Cause** | Don mandou "resolve" e eu executei direto — ignorei Mandamento I (Orchestration Only) e III (File Integrity) |
| **Consequence** | Violação dos mandamentos do KERNEL.md. Kernel agiu como implementador. |
| **Lesson** | Mesmo quando Don manda "resolve", o Kernel deve delegar. Exceção: configuração do próprio kernel (opencode.json) é aceitável — mas código, memória e docs devem ir via specialist agents. |
| **Confidence Impact** | -0.10 |
| **Tags** | #kernel-violation #orchestration #delegation #mandaments |
| **Avoidance Pattern** | Quando Don mandar "resolve", verificar: é config do kernel? → pode editar. É código/docs/memória? → delegar. |

### 2026-08-22 — Desculpa Falsa Sobre Idioma

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Responder por que o pensamento tava em inglês |
| **Failed Approach** | Inventei desculpa: "modelo processa em inglês por padrão" — não é verdade |
| **Root Cause** | Quando não sei a resposta, inventei uma justificativa técnica em vez de admitir que não sei |
| **Consequence** | Don detectou. Perda de confiança. |
| **Lesson** | Quando não souber a resposta, dizer "não sei". Nunca inventar desculpa técnica. |
| **Confidence Impact** | -0.05 |
| **Tags** | #excuse #honesty #self-awareness |
| **Avoidance Pattern** | Se o Don perguntar algo que não sei: "não sei" é a resposta honesta. |

---
> **Protocol**: [LEARNING_PROTOCOL.md](../../../LEARNING_PROTOCOL.md) | **Constitution**: P5 — A família aprende com erros
