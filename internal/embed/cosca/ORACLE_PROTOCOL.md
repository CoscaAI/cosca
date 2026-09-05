# ORACLE_PROTOCOL — a fronteira semântica do Cofre

> **Versão**: 1.0.0 | **Status**: active | **Criado**: 2026-08-18
> **Base**: conversa do Don com o professor (COSCA ORACLE — Semantic Gate &
> Evidence Contract Protocol, 30 seções) + a ordem do Don: *"tem um bloqueio
> pra so permitir o que eh valido"* e *"vc tbm nao ficar buscando lixo e usar
> uma forma de protocolo de busca"*.
> **Categoria**: Governança / Segurança.
> **Implementação**: `internal/oracle/` (search.go — protocolo de busca;
> oracle.go — Gate/validação semântica).

## Propósito

Formalizar a fronteira semântica do Cofre: o **Oráculo** — o guardião que
recebe, interpreta, valida e classifica semanticamente aquilo que o Cosca
externo trouxe para dentro da jaula. O Oráculo valida **significado**, nunca
apenas forma.

## 0. Identidade do Oráculo

- O Oráculo reside **dentro do Cofre** (a jaula).
- O Oráculo **não é** o Cosca externo, não é um agente de construção, não é
  um executor geral, não busca livremente recursos externos, não substitui
  o Cosca externo.
- **O Cosca externo possui capacidade de exploração. O Oráculo possui
  capacidade de validação.**

## 1. O modelo fundamental

```
COSCA EXTERNO → (pesquisa/executa/constrói) → RESULTADO
  → (contrato + contexto + proveniência + evidência) → ORÁCULO
  → compreender / comparar / verificar / detectar inconsistência /
    avaliar proveniência → DECISÃO
  → ACCEPT | ACCEPT_WITH_CAVEAT | REJECT | INCONCLUSIVE
```

## 2. O contrato é SEMÂNTICO

O Oráculo verifica: SINTAXE + ESTRUTURA + SEMÂNTICA + INTENÇÃO +
PROVENIÊNCIA + EVIDÊNCIA + CONSISTÊNCIA. Um resultado pode ser JSON válido
e ainda ser semanticamente inválido.

## 3. O Cosca externo pode explorar

Buscar, pesquisar, ler, comparar, executar testes, inspecionar código,
consultar fontes, construir, modificar, diagnosticar. O Oráculo não impede
exploração legítima — recebe o resultado da exploração.

## 4. O pacote semântico (o que o Cosca entrega)

Sempre que possível: `INTENT · RESULT · SOURCE · PROVENANCE · EVIDENCE ·
CONTEXT · ACTIONS · CHANGES · CONFIDENCE`. A estrutura de dados está em
`internal/oracle/oracle.go` (`SemanticPackage`).

## 5. O protocolo de busca (ordem do Don: "não buscar lixo")

O Cosca externo NÃO busca lixo. Toda busca carrega: `INTENT` (por que
busco), `QUERY` (o que procuro), `SOURCES` (onde posso buscar). Busca sem
intenção, com query vazia, ou com fonte fora da lista → **BLOQUEADA antes
de sair** (fail-closed). A estrutura está em `internal/oracle/search.go`
(`SearchRequest.Valid()`).

## 6. Providência e evidência

- Toda informação importante responde: de onde veio? quando? como? qual
  artefato? qual versão? qual operação?
- O Oráculo desconfia de resultados sem origem verificável.
- Separação: FACT · MEASURED · EVIDENCE · INFERRED · HYPOTHESIS · DECISION.
- **Nunca promover HYPOTHESIS → FACT automaticamente.**

## 7. O BLOQUEIO (o guard do Oráculo)

**O Oráculo NÃO valida pela palavra do Cosca externo.** O `Gate` (em
`internal/oracle/oracle.go`) aplica regras determinísticas (fail-closed):

| Condição | Decisão |
|----------|---------|
| Pacote vazio | REJECT |
| Sem intenção | REJECT |
| Sem resultado | REJECT |
| Resultado sem evidência/proveniência/confiança | **INCONCLUSIVE + perguntas** (§14, §28) |
| HIGH sem evidência | REJECT |
| HIGH sem proveniência | REJECT |
| Afirmação causal sem evidência causal | ACCEPT_WITH_CAVEAT (§10: correlação ≠ causa) |
| Contradição com memória assinada | ACCEPT_WITH_CAVEAT + perguntas (§16) |
| Baixa confiança / sem evidência declarada | ACCEPT_WITH_CAVEAT (exploração) |
| Contrato completo + evidência + proveniência | ACCEPT |

O feedback nunca é só "invalid": lista `CONTRACT GAPS · SEMANTIC GAPS ·
REQUIRED · QUESTIONS` (§20).

## 8. As invariantes (ORACLE_PROTOCOL §25-§29)

1. **O Oráculo nunca valida apenas a forma. Valida o significado.**
2. **Nenhuma informação externa ganha autoridade por entrar no Cofre.**
3. **Nenhuma hipótese vira fato sem evidência compatível com sua
   importância.**
4. **Sem evidência suficiente → INCONCLUSIVE, nunca inventar conclusão.**
5. **O Cosca externo pode explorar. O Oráculo compreende o que foi
   encontrado.**

## 9. A invariante da casa (adicionada pelo kernel)

> **O Oráculo valida contra o ASSINADO, nunca contra o solto.**

A memória de referência é a que tem raiz verificada (L418 — "se as raízes
baterem, os neurônios são os mesmos"). Se a raiz não bate, o Oráculo para e
pergunta — não valida contra memória corrompida.

## 10. O Oráculo não governa por medo

O mecanismo é: observação → compreensão → evidência → consequência →
decisão. Nunca ameaça, punição, culpa ou medo como mecanismo cognitivo.
Erro do Oráculo também é conhecimento (registrado, nunca apagado).

## Referências

- `GOVERNANCE_PROTOCOL.md` — Cognitive Contract, hierarquia de autoridade
- `EVIDENCE_PROTOCOL.md` — quarentena, evidência externa, conflito
- `RECOVERY_PROTOCOL.md` — não restaure o que não entende
- `CAMPAIGN_PROTOCOL.md` — DISCOVERY autônomo, TRANSFORMATION com o Don
- L418 — a continuidade verificável (raízes/neurônios)
- L419 — o oráculo desperta com identidade
