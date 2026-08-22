# Go SDK Reference

> **Status**: aspirational | **Owner**: SDK Chief | **Last Updated**: 2026-07-28

## Overview

> **Nota importante:** O Go SDK como client library standalone **ainda não existe** neste estágio do projeto (v1.4.0-dev).
>
> Atualmente, a API pública Go está disponível via `pkg/cosca/` (tipos compartilhados e interfaces), e o acesso programático se dá através da REST API (porta 14120) ou do CLI (`cosca`).

---

## Uso Atual (Go)

### Tipos e interfaces públicas

```go
import "github.com/CoscaAI/cosca/pkg/cosca"
```

O pacote `pkg/cosca/` expõe os tipos compartilhados do ecossistema Cosca:

- `cosca.Agent` — definição de agente
- `cosca.Skill` — definição de skill
- `cosca.MemoryRecord` — registro de memória
- `cosca.ProviderConfig` — configuração de provider LLM
- `cosca.RuntimeConfig` — configuração de runtime
- `cosca.KnowledgeStats` — estatísticas do knowledge engine
- `cosca.PluginInfo` — informação de plugin
- `cosca.WorkflowDef` — definição de workflow

### Acesso via REST API

Para acesso programático completo, utilize a REST API:

```bash
curl http://localhost:14120/api/health
```

### SDK TypeScript

O SDK oficial completo está disponível para TypeScript:

```bash
npm install @cosca/sdk
```

Veja a [referência do TypeScript SDK](typescript.md).

---

## Roadmap — Go SDK

O Go SDK como client library (similar ao TypeScript SDK) está planejado para uma release futura e incluirá:

- `client.New()` — cliente HTTP para a REST API
- Métodos tipados para Knowledge, Memory, Agents, Plugins, Workflows
- Suporte a streaming (SSE) para execuções de agentes
- Gerenciamento de conexão e retry automático

---

> **Related**: [TypeScript SDK Reference](typescript.md) | [API Reference](../api-reference/overview.md)
