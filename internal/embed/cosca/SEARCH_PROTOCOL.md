# SEARCH_PROTOCOL — Busca Semântica de Alta Precisão

> **Versão**: 1.0.0 | **Status**: active | **Criado**: 2026-08-18
> **Base**: conversa do Don com o professor (COSCA SEARCH PROTOCOL, 31 seções)
> + a ordem do Don: *"vc tbm nao ficar buscando lixo e usar uma forma de
> protocolo de busca"*.
> **Categoria**: Conhecimento / Oráculo.
> **Implementação**: `internal/oracle/search.go` + `internal/oracle/search_filter.go`.

## Propósito

Impedir que o Cosca transforme uma busca em coleta indiscriminada de
arquivos, palavras ou resultados irrelevantes.

> **Não buscar tudo que contém a palavra. Buscar aquilo que explica a
> intenção. Quantidade de resultados não é qualidade.**

## 1. Primeiro entender, depois buscar

Antes de executar qualquer busca, determinar: `INTENT · ENTITY · CONTEXT ·
QUESTION · EXPECTED_EVIDENCE · SCOPE`. A busca deriva da intenção, não da
frase original.

## 2. Não usar a query crua como verdade

Uma pergunta humana pode conter palavras que não são bons termos de busca.
Derivar conceitos e priorizar os mais prováveis.

## 3. Busca em camadas

Nunca começar pelo universo inteiro. Ordem preferencial:

```
CAMADA 1 identidade/conceito exato
CAMADA 2 entidades relacionadas
CAMADA 3 sinônimos/equivalências
CAMADA 4 busca estrutural ampla
CAMADA 5 exploração geral (só se necessário)
```

Expansão somente quando a camada anterior não produzir evidência suficiente.
Implementado em `SearchLayer` (search.go).

## 4. Precisão antes de recall

Preferir 3 resultados altamente relevantes a 500 parcialmente relacionados.

## 5. Resultado ≠ match

`TEXT MATCH ≠ SEMANTIC RELEVANCE`. Um arquivo conter a palavra não o torna
relevante.

## 6. Classificação de resultados

Todo resultado recebe: `DIRECT · RELATED · CONTEXTUAL · WEAK · NOISE`
(implementado em `RelevanceClass`).

## 7. Não entregar lixo

`NOISE` nunca é apresentado. `WEAK` só quando necessário para explicar
ausência. Priorizar `DIRECT/RELATED/CONTEXTUAL` (implementado em
`FilterAndRank` → `Presentable`).

## 8. Score semântico

`RELEVANCE + INTENT_MATCH + ENTITY_MATCH + CONTEXT_MATCH + RECENCY +
PROVENANCE + EVIDENCE_VALUE` − penalidades (`keyword_only`, `duplicate`,
`outdated`, `low_provenance`, `hypothesis_not_fact`). Implementado em
`SemanticScore.Compute()`.

## 9. Duplicatas

Se dez arquivos contêm a mesma informação, não apresentar os dez. Agrupar:
10 resultados → 3 conteúdos distintos → 1 evidência principal + referências.
Implementado por deduplicação de conteúdo em `FilterAndRank`.

## 10. Resultados conflitantes

Dois resultados relevantes discordam → `CONFLICT`, nunca escolher o primeiro.
Investigar: qual é mais recente? melhor proveniência? evidência? estado atual?
Implementado em `detectConflict` → `SearchResponse.Conflict`.

## 11. Recência

Estado atual → priorizar informação recente. Histórico → priorizar o período
solicitado. Nunca usar documento antigo para responder sobre o estado atual.

## 12. Proveniência

Quanto maior a importância da afirmação, maior a exigência de proveniência.
Preferência: artefato verificável → arquivo específico → commit → benchmark →
registro de execução → inferência. Texto narrativo convincente não é evidência.

## 13. Busca iterativa

`ENTENDER → BUSCAR → RANQUEAR → INTERPRETAR → VERIFICAR SUFICIÊNCIA`. Se
suficiente: STOP. Se insuficiente: refinar query e buscar de novo. Nunca
ampliar só porque ainda existem resultados possíveis.

## 14. Critério de parada

Parar quando: pergunta respondida + evidência suficiente + sem contradição
relevante + confiança adequada. Não pesquisar indefinidamente para aumentar
referências.

## 15. Ausência também é resultado

Sem evidência relevante → não inventar. Responder `NOT_FOUND` ou
`INSUFFICIENT_EVIDENCE` (implementado em `SearchOutcome`).

## 16. Não preencher lacunas com imaginação

Se existe `CARRO_PROTOCOL.md`, não assumir que airbag/motor/pneu existem.
Separar: `FOUND · NOT_FOUND · INFERRED · UNKNOWN`.

## 17. Contexto da conversa

"A segunda pergunta herda o contexto da primeira." Se o usuário perguntou
sobre o carro e depois sobre o airbag → `ENTITY=carro, CONTEXT=estado,
SUBJECT=airbag`. A busca fica mais específica, não independente.

## 18. Memória não é lixeira

Não consultar toda a memória para qualquer pergunta. Prioridade: memória
diretamente relacionada → contextual → histórica necessária. Evitar memory dump.

## 19. Protocolos têm prioridade semântica

Protocolo específico > documentação genérica > código aleatório. Pergunta
sobre recuperação → `RECOVERY_PROTOCOL`, não dezenas de arquivos com "recovery".

## 20. Código também precisa de contexto

Não retornar código só porque contém a palavra. Mostrar somente o trecho
necessário.

## 21. Busca de código

Preferência: `symbol → function → type → module → file → repository-wide text
search`. Não começar sempre com grep global.

## 22. Busca semântica + estrutural

Combinar: relevância semântica + estrutura de arquivos + relação de símbolos +
proveniência. Ex.: "por que o Oráculo não valida?" → conceitos: Oracle,
validation, contract, semantic gate, decision, evidence (não só "validate").

## 23. O Oráculo pode questionar a busca

"Esses resultados têm correspondência textual, mas não relação semântica
suficiente." O Cosca então refina a busca.

## 24. Proteção contra busca autoalimentada

Cuidado com exploração infinita (buscar → texto diz "busque X" → buscar X →
...). Toda nova busca deve ter `REASON` ligada à `ORIGINAL_INTENT`. Sem relação
suficiente: STOP.

## 25. Curiosidade ≠ necessidade

Descobrir algo interessante não significa que precisa investigar. Perguntar:
"Isso é necessário para responder a pergunta atual?" Se NÃO → não expandir.

## 26-27. Busca como investigação + anti-confirmação

`QUESTION → HYPOTHESES → EVIDENCE SEARCH → CONTRADICTION SEARCH → VALIDATION →
CONCLUSION`. Buscar evidência A FAVOR E CONTRA a hipótese (implementado em
`SearchHypothesis`). O objetivo é a verdade operacional, não confirmar a
primeira explicação.

## 28. Resposta final

Construída a partir dos resultados relevantes: afirmação principal →
evidência → contexto necessário → incerteza conhecida. Não despejar o
mecanismo interno de busca.

## 29. Regra de ouro

> **Se o resultado não ajuda a responder a intenção, ele não é informação.
> É ruído.**

## 30. Regra do Oráculo

O Oráculo ensina o Cosca a perguntar: **"Por que estou trazendo este
resultado?"** Sem resposta semanticamente válida → NÃO TRAZER.

## 31. Definição final

O Cosca não é o sistema que encontra mais coisas. É o sistema que:
`encontra → entende → filtra → verifica → conecta → para`.

A busca perfeita retorna **o mínimo necessário para sustentar uma resposta
correta**.

## Referências

- `ORACLE_PROTOCOL.md` — o oráculo valida o que a busca trouxe
- `KNOWLEDGE_SEARCH_PROTOCOL.md` — busca no knowledge.db
- `EVIDENCE_PROTOCOL.md` — quarentena, proveniência, conflito
- L422/L423 — implementação do oráculo
