# ADR-010: Painéis de Governança como Views do Workbench (Trust / Control / Map)

> **Status:** Accepted ✅ | **Owner:** Cosca Kernel | **Last Updated:** 2026-08-14
> **Implementation:** L302 (Trust), L305 (Control Room), L306 (Project Map)
> **Fonte:** professor (2026-08-14) — pilares 6, 8, 9 da visão Enterprise

## Context

O professor descreveu 10 pilares Enterprise. A verificação mostrou que o
backend do cosca-code JÁ TEM a infraestrutura de metade deles (execpolicy,
jail, auth, intel, memory, skills, git). A decisão: **expor o que existe em
leitura, nunca criar API falsa** (P14).

### Decisão

Três painéis de governança como **views do workbench** (docking), todos
somente-leitura:

| Painel | Dados REAIS expostos | Endpoints |
|--------|---------------------|-----------|
| **🛡 Trust** (pilar 8) | Autonomia, Memória, Skills | /api/autonomy, /api/memory, /api/skills |
| **📊 Control** (pilar 9) | Modelos (ollama), Estado, Git | /api/ai/models, /api/git/status |
| **🗺️ Map** (pilar 6) | Símbolos + dependentes do projeto | /api/intel/symbols, /api/intel/dependents |

Recursos que o backend NÃO expõe (agents:status, tokens:usage,
context:indexed) aparecem como **"Não disponível (backend não expõe)"** — a
honestidade P14.

### Por quê

1. **Custo-benefício máximo**: expor backend testado dá o impacto Enterprise
   em horas, não semanas (mandamento 3: separar visual de estado).
2. **Mandamento 2**: os painéis reutilizam o sistema de componentes (dock +
   panel) — não são telas isoladas.
3. **P14**: nada de mock — o que não existe é declarado indisponível.

### Consequências

- A UI do cosca-code mostra a IA da casa por dentro (o que pode, o que sabe,
   o que usa).
- A sensação de "plataforma coesa" (critério 1 do professor) cresce.
- Endpoint de leitura de agentes (GET /api/agents/status) fica como sugestão
   para destravar o pilar 5 — com aprovação do Don.

### Alternativas rejeitadas

- **Criar endpoints novos no backend**: rejeitado nesta fase — a ordem do Don
  foi "sem risco ao backend".
- **Mockar agents/tokens/context**: rejeitado — P14 + mandamento 14.
