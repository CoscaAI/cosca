# PROMPT_PROTOCOL — a linguagem operacional do Cosca (o 30º protocolo)

> A comunicação entre o Don e o Cosca em comandos compactos respaldados por
> contratos persistentes. **Símbolo curto → definição verificável → execução
> controlada.** Criado a partir do desenho do professor (a ABI de prompts).

## LEMA

> Comprima redundância; **nunca comprima evidência necessária para uma
> decisão.** `PASS` é conclusão; `200/200` é evidência — a evidência nunca é
> descartada ao virar conclusão (`STRENGTH(CONCLUSION) ≤ STRENGTH(EVIDENCE)`).

## 1. A distinção fundamental

- **Prompt humano compacto** ≠ **contexto interno compacto.** O Don envia
  10 tokens (`@SHIP E007`); o runtime expande para o contrato completo a
  partir do registry — sem repetir o contexto na conversa.
- **Context retrieval, não compressão textual:** o `@E007` localiza o
  experimento; o resolver recupera somente o necessário (evidência, estado,
  invariantes) das camadas persistidas.

## 2. Os 5 níveis de comandos

| Nível | Comandos | Exemplo |
|---|---|---|
| 1. Mínimos | `@status` `@wake` `@doctor` `@audit` `@recover` `@stop` | `@status` |
| 2. Operações | `@inspect file` `@trace symbol` `@benchmark target` `@test hypothesis` `@compare A B` `@explain decision` `@project name` | `@inspect indexer.go` |
| 3. Campanhas | `@campaign start` `@campaign resume` `@campaign E-008` `@campaign close` | `@campaign PERF-2026-001` |
| 4. Transformações | `@transform E-008` (o contrato já sabe: PRE_STATE → SNAPSHOT → CHANGE → TEST → VERIFY → POST_STATE) | `@transform E-007` |
| 5. Consultas cognitivas | `@why L376` `@evidence E-007` `@unknown current` `@history symbol` `@dependencies C` `@risk change` | `@why L389` |

## 3. As macros semânticas

| Macro | Significado expandido (do registry) |
|---|---|
| `@SURGICAL` | escopo mínimo · sem refactor oportunista · snapshot obrigatório · diff obrigatório · testes obrigatórios · guardian obrigatório · **stop após conclusão** |
| `@PROVE` | não modificar · formular hipótese · identificar oracle · executar experimento · produzir evidence · classificar PASS/FAIL/INCONCLUSIVE · registrar UNKNOWN |
| `@SHIP` | verificar estado · testar · snapshot · commit · guardian · instalar · health check · recovery point (a promoção de uma transformação verificada ao runtime) |
| `@PROMOTE current` | a forma compacta de `@SHIP` com o alvo implícito |

## 4. A ABI compacta (forma canônica)

```
@I E008 RO G:recall,counts,rank
@I       = INVESTIGATE        @T E007 S   = TRANSFORM, snapshot REQUIRED
E008     = experimento        @V E007 P   = VERIFY, production=true
RO       = READ_ONLY          @STOP       = parar
G        = guardians
```

## 5. Resolução determinística (NUNCA chutar)

- Símbolo → registry → schema → contrato → execução.
- **Ambiguidade:** `ERROR: PROMPT_AMBIGUOUS` (com as versões listadas).
- **Desconhecido:** `ERROR: UNKNOWN_COMMAND` (com sugestões).
- Sem interpretação livre — a inteligência está no protocolo, não na repetição.

## 6. A arquitetura

```
USER (@SHIP E007) → COMMAND PARSER → PROMPT REGISTRY → CONTEXT RESOLVER
  ├── campaign · state · evidence · invariants · permissions
  → MODEL → GUARDIAN
```

O modelo não recebe a enciclopédia a cada vez — o runtime prepara o contexto
(recuperação seletiva: L0 comando → L1 contrato → L2 estado → L3 memória
relevante → L4 evidência → L5 código).

## 7. Prompt caching (a segunda camada)

Prefixo **estático e estável** (system + protocolos + schemas) separado do
**dinâmico** (campanha atual, tarefa, diff). O prefixo estável favorece o
cache de prompt (processamento mais rápido e barato em chamadas repetidas).
Nunca intercalar dados dinâmicos dentro do bloco estático.

## 8. As três formas de prompt

- **HUMAN:** natural ("veja se essa alteração pode entrar em produção").
- **COMPACT:** `@PROMOTE current`.
- **MACHINE:** o runtime expande (ACTION=PROMOTE, TARGET=current, REQUIRE:
  authorization, snapshot, guardians, recovery, post_verify).

## 9. A medição (a campanha PROMPT-EFF-001)

Regra de aprovação de qualquer padrão compacto:

```
TOKEN_SAVING > 0
AND TASK_SUCCESS >= baseline
AND EVIDENCE_LOSS = 0
AND CRITICAL_AMBIGUITY = 0
```

Se economizar 70% mas errar decisões → REJECT. Se 40% com equivalência
operacional → PASS. Medir: input/output tokens, latência, cache hit, task
success, erro, ambiguidade, fidelidade semântica.

## 10. Referências

- Registry: `protocol/PROMPT_REGISTRY.md` (os contratos de cada comando).
- Campanha de medição: `knowledge/patterns/CAMPAIGN_PROMPT-EFF-001.md`.
- Base epistemológica: CAMPAIGN_PROTOCOL (v3.0) · P13 · a regra das 4 réguas.
