# ADR-001: Backend do core em **Go** (não Node/TypeScript, não Rust)

> **Status:** Proposta 🔵 (aguarda aprovação do Don)
> **Owner:** cosca-architecture (Architecture Chief) | **Last Updated:** 2026-09-01
> **Projeto:** RIZOMAI — plataforma API-first de gestão de redes sociais (um post → todas as redes)
> **Referência (base):** `docs/reports/zernio-mineracao/arquitetura-zernio.md` §2/§10, `integracoes-zernio.md` §5, `produto-zernio.md` §9.

---

## 0. Contexto

O RIZOMAI é uma API de integrações **multi-plataforma**: 1 post → N redes, com fan-out paralelo,
retry por target, OAuth broker server-side, webhooks, upload streaming até 5 GB e workers de
publicação/entrega/refresh. O ecossistema Zernio (nosso blueprint) usa **várias linguagens — mas
todas para SDKs gerados por OpenAPI**; a linguagem do CORE é o que importa. Restrições do projeto:
startup LATAM enxuta (~R$1.000/mês de infra), velocidade de desenvolvimento alta, manutenibilidade,
ecossistema de SDKs das redes sociais, e público final = desenvolvedores + AI agents.

## 1. Decisão

**O core do RIZOMAI será escrito em Go (golang).**

Escolha: `Go 1.22+` (workspace multi-módulo), `net/http` + roteador leve (`chi`), `pgx` para
Postgres, `River` para jobs (ver ADR-003). Deploys como **binário único estático**.

Justificativa custo-benefício para uma startup enxuta:

| Critério | Go | Node/TS | Rust |
|---|---|---|---|
| **Concorrência p/ fan-out e workers** | goroutines + channels (nativa) | event loop (I/O ok, CPU sofre) | tokio (excelente) |
| **Custo de infra (RSS/cold start)** | ~40–80 MB, binário único | ~150 MB+ por processo + node_modules | ~10 MB, binário único |
| **Velocidade de dev** | alta (stdlib + tooling simples) | alta (ecossistema enorme) | média (borrow checker) |
| **Ecossistema de integração** | bom p/ HTTP/Postgres/filas | melhor p/ libs sociais | menor |
| **Time/contratação no BR** | bom e crescente | ótimo | pequeno |
| **Manutenibilidade (monorepo)** | workspace tipado, `go vet`/testes | tipado se TS, porém ferramental pesado | forte, porém verboso |

1. **Concorrência é o requisito central.** Publicar 1 post em 5 redes exige fan-out em paralelo com
   isolamento de falha por target; workers de webhook/refresh rodam 24/7. Goroutines entregam isso
   com modelo mental simples e sem async/await em toda a árvore de chamadas.
2. **Custo de runtime baixo = mais plataforma pelo mesmo orçamento.** Com ~R$1.000/mês, um binário
   Go de ~50 MB (sem node_modules, sem JVM) roda em instância mínima; Node precisaria de mais RAM
   por processo e cold start mais lento; Rust economizaria mais, mas ao custo de velocidade de dev.
3. **O contrato (OpenAPI) é agnóstico de linguagem.** SDKs para clientes serão **gerados**
   (TypeScript/Python/Go) — a linguagem do core não restringe o ecossistema de SDKs (lição Zernio §8:
   eles têm 8 linguagens todas geradas do mesmo `openapi.yaml`).
4. **Soberania técnica do Don/workspace.** O Cosca já é Go (`internal/*`, `go.mod`): o Don conhece a
   linguagem, há padrões de trabalho reutilizáveis (workqueue, retry, single-binary) e custo zero de
   aprendizado novo.
5. **Fila/agendamento/streaming têm primitivas maduras em Go** (River, Asynq, `io.Copy` para upload
   streaming, `net/http` para presign) — nada do MVP exige Node.

## 2. Consequências

**Prós**
- Custo de infra mínimo (binário único, memória baixa) — cabe em R$1.000/mês.
- Concorrência robusta e previsível para fan-out/retry/webhooks.
- Deploy trivial (um binário + env), rollback simples, sem imagem pesada.
- Ecossistema Go maduro para o problema: pgx, River, chi, OpenAPI Generator (Go).
- Consistência com o workspace Cosca → reuso de padrões e experiência do Don.

**Contras**
- Menos bibliotecas prontas de integração social do que Node (mitigado: escrevemos os conectores
  contra as specs oficiais das plataformas — o trabalho é nosso mesmo assim, Zernio provou).
- Boilerplate e verbosidade (mitigado: camada de service + geradores internos).
- Alguns fluxos de plataforma têm exemplos oficiais só em Node/Python (mitigado: specs `openapi-specs`
  e `best-practices` minerados; documentar por conector).

## 3. Alternativas consideradas

1. **Node/TypeScript** — ecossistema de libs sociais e velocidade inicial melhores. **Rejeitado**:
   runtime mais caro (memória/CPU por processo) para 4–6 workers + API, concorrência single-threaded
   menos previsível para fan-out pesado e processamento de mídia, e monorepo com `node_modules`
   encarece CI/deploy. Node continua **primeira linguagem dos SDKs gerados para clientes**.
2. **Rust** — performance máxima e memória mínima. **Rejeitado para o core no MVP**: velocidade de
   desenvolvimento e contratação no BR não justificam o ganho numa plataforma dominada por I/O de
   rede (onde Go já chega perto). Pode virar worker específico (ex.: processamento de vídeo) no futuro,
   sem mudar esta decisão.
3. **Python** — ótimo p/ SDK de cliente e protótipos, ruim p/ core concorrente e deploy (GIL,
   runtime pesado). **Rejeitado** para o core; Python continua como SDK de cliente gerado.

## 4. Referências

- `arquitetura-zernio.md` §8 (SDKs gerados × hand-written — core é independente da linguagem de SDK).
- `produto-zernio.md` §9.2 (postura API-first; custo de infra é diferencial competitivo).
- ADR-004 (estrutura do monorepo em Go workspace).

---

*ADR de Fase 1 — RIZOMAI. Decisão de stack, aguardando aprovação do Don.*
