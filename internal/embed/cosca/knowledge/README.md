# COSCA ENGINEERING INTELLIGENCE MATRIX

> A camada de inteligência de engenharia do Cosca — 15 stacks unificados.
> **Não são 15 skills isoladas.** São um sistema: conhecimento + especialistas + workflows.

## A matriz

```
                         COSCA
                           │
             ┌─────────────┴─────────────┐
             │                           │
        PRODUCT BRAIN              ENGINEERING BRAIN
             │                           │
          UI / UX (01)              ARCHITECTURE (08)
             │                    ┌──────┼──────┐
        FRONTEND (02)           API (05)  BACKEND (04)  DATA (06)
             │                    │      │       │
         MOBILE (03)              └──────┼───────┘
                                        │
                              DISTRIBUTED SYSTEMS (07)
                                        │
                    ┌───────────────────┼───────────────────┐
                    │                   │                   │
                 SECURITY (09)        TEST (10)          DEVOPS (11)
                    │                   │                   │
                    └───────────────────┼───────────────────┘
                                        │
                                  RELIABILITY (12)
                                        │
                                  OBSERVABILITY (13)
                                        │
                                  PERFORMANCE (14)
                                        │
                                  AI / AGENTS (15)
```

## Índice dos stacks

| # | Stack | Diretório | Foco |
|---|---|---|---|
| 01 | Product/UI/UX/Design | `design/` | decidir o porquê visual |
| 02 | Frontend Engineering | `frontend/` | construir frontend escalável |
| 03 | Mobile Engineering | `mobile/` | mobile ≠ desktop pequeno |
| 04 | Backend Engineering | `backend/` | complexidade proporcional ao problema |
| 05 | API Engineering | `api/` | API é contrato |
| 06 | Database Engineering | `database/` | banco é engenharia, não ORM |
| 07 | Distributed Systems | `distributed/` | assume failure |
| 08 | Software Architecture | `architecture/` | boundaries e trade-offs |
| 09 | Security Engineering | `security/` | segurança é processo |
| 10 | Testing/QA | `testing/` | coverage ≠ qualidade |
| 11 | DevOps/CI/CD | `devops/` | repetível, auditável, versionado |
| 12 | SRE/Reliability | `sre/` | falhar de forma controlada |
| 13 | Observability | `observability/` | comportamento → informação acionável |
| 14 | Performance | `performance/` | ciência experimental, não opinião |
| 15 | AI/LLM/Agents | `ai/` | a plataforma que o Cosca É |
| 16 | E-Commerce/Commerce | `ecommerce/` | sistema distribuído de comércio |
| 17 | Security (Zero Trust/Defense in Depth) | `security/` | secure by design |
| 18 | Visual Media / Image / PDF / SVG | `visual-media/` | escolher o pipeline pelo conteúdo |
| 19 | Video / Audio / Media | `video-media/` | vídeo é dado temporal |
| 20 | Audio / DSP / Music / Speech | `audio/` | entender o sinal antes de processar |

## Pipeline de feature (workflow mestre)

```
Nova feature
   │
   ▼
PRODUCT ANALYSIS      ← design/01
   │
   ▼
ARCHITECTURE          ← architecture/08
   │
   ├──── DATABASE      ← database/06
   ├──── API           ← api/05
   ├──── BACKEND       ← backend/04
   ├──── FRONTEND      ← frontend/02
   └──── MOBILE        ← mobile/03 (quando aplicável)
         │
         ▼
      SECURITY          ← security/09
         │
         ▼
      TEST              ← testing/10
         │
         ▼
   PERFORMANCE          ← performance/14
         │
         ▼
  OBSERVABILITY         ← observability/13
         │
         ▼
      REVIEW
         │
         ▼
     APPROVAL
```

## Como consultar

1. **Antes de projetar**: consulte o stack do domínio (README do diretório).
2. **Cada decisão**: registre no formato unificado (`_SCHEMA.md`).
3. **Auditar UI/frontend**: `CHECKLIST.md` do `design/` + stack correspondente.
4. **Feature completa**: siga o pipeline acima, consultando cada stack na fase certa.
5. **Mapa completo**: `STACK-CATALOG.md` (50 stacks: 19 implementados + roadmap) + **ativação multi-stack** (o orquestrador ativa os stacks pela natureza da tarefa).
6. **Referências**: `REPOSITORIES.md` — índice mestre de ~300 repositórios curados, organizado por stack (preenche o conhecimento faltante dos roadmap).

## Formato unificado

Todo conhecimento dos 15 stacks usa o MESMO schema (`_SCHEMA.md`) e os MESMOS tipos:
`principle · pattern · heuristic · anti-pattern · architecture · decision · trade-off · failure-mode · best-practice · counter-example · reference · benchmark · checklist · review-rule`

## Especialistas e workflows

- **Agentes-arquiteto** (15): `ARCHITECTS.md`
- **Workflows** (9 + pipeline mestre): `WORKFLOWS.md`

> Cada stack produz conhecimento com o mesmo shape → o Knowledge Compiler trata todos uniformemente.
