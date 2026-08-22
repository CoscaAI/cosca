# KNOWLEDGE SCHEMA — Formato Unificado

> Todos os 15 stacks produzem conhecimento com EXATAMENTE o mesmo shape.
> O Knowledge Compiler trata tudo uniformemente.

## Tipos de conhecimento

| Tipo | Pergunta que responde |
|---|---|
| `principle` | Qual é a verdade fundamental? |
| `pattern` | Qual é a solução reutilizável? |
| `heuristic` | Qual é a regra de bolso para decidir? |
| `anti-pattern` | O que evitar e por quê? |
| `architecture` | Como estruturar o sistema? |
| `decision` | Qual escolha foi feita e por quê? |
| `trade-off` | O que se ganha e o que se perde? |
| `failure-mode` | Como isso falha? |
| `best-practice` | Qual é a prática recomendada? |
| `counter-example` | Quando a regra não se aplica? |
| `reference` | Onde aprender mais? |
| `benchmark` | Como medir? |
| `checklist` | Como validar antes de entregar? |
| `review-rule` | O que verificar numa revisão? |

## Schema de uma entrada de conhecimento

```
ID:           <stack>-<seq> (ex.: design-014, backend-003)
DOMAIN:       <design|frontend|mobile|backend|api|database|distributed|architecture|security|testing|devops|sre|observability|performance|ai>
TITLE:        <nome curto e específico>
PROBLEM:      <qual problema real do usuário/sistema resolve>
CONTEXT:      <em que situação se aplica>
PRINCIPLE:    <a verdade fundamental por trás>
RECOMMENDATION: <o que fazer>
WHEN_TO_USE:  <quando aplicar>
WHEN_NOT_TO_USE: <quando NÃO aplicar>
TRADE_OFFS:   <ganhos e perdas>
EXAMPLE:      <exemplo concreto>
COUNTER_EXAMPLE: <contra-exemplo>
FAILURE_MODES: <como pode falhar>
REFERENCES:   <fontes>
CONFIDENCE:   <UNIVERSAL|STRONG|CONTEXTUAL|OPINION|EXPERIMENTAL>
SOURCE:       <de onde veio (skill/repo/experiência)>
VERSION:      1
CREATED_AT:   <data>
UPDATED_AT:   <data>
```

## Exemplo (design)

```
ID: design-014
DOMAIN: design
TITLE: Vermelho como cor de ação primária em domínio operacional
PROBLEM: Em produtos de manutenção/operação, vermelho primário gruda "erro" em toda a UI
CONTEXT: Produtos onde verde=operacional, âmbar=aguardando, vermelho=incidente
PRINCIPLE: Cor significa estado; a cor primária deve ecoar o estado "ok", não o "erro"
RECOMMENDATION: Verde como primário; vermelho só para destructive/incidente
WHEN_TO_USE: Dashboards de operação, monitoramento, manutenção
WHEN_NOT_TO_USE: Branding de energia/urgência onde vermelho é identidade
TRADE_OFFS: Identidade "energia" vs semântica operacional clara
EXAMPLE: HornFit (verde #16A34A primário, texto escuro p/ contraste)
COUNTER_EXAMPLE: App de delivery com vermelho de marca é OK se é identidade
FAILURE_MODES: Contraste < 4.5:1 se texto claro sobre verde
REFERENCES: UI UX Pro Max (192 regras por indústria)
CONFIDENCE: CONTEXTUAL
SOURCE: ui-ux-pro-max + aplicação HornFit
VERSION: 1
```

## Regras do formato

1. Toda entrada usa ESTE schema — sem campos extras por domínio.
2. `CONFIDENCE` segue a regra de convergência (3+ sistemas = UNIVERSAL).
3. `PROBLEM` sempre do ponto de vista do USUÁRIO, não do sistema.
4. Se não houver `COUNTER_EXAMPLE`, a entrada provavelmente é incompleta.
