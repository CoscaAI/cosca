# RECOVERY_PROTOCOL — o mecanismo formal para sair de estados não compreendidos

> **Versão**: 1.0.0 | **Status**: active | **Criado**: 2026-08-17 (L385)
> **Base**: a investigação L369→L384 (reconstrução determinística provada,
> identidade por digest, fail-closed, Cognitive Recoverability).
> **Categoria**: Governança.

## Propósito

Quando o Cosca perceber que o sistema "saiu do controle" — estado suspeito,
regressão, corrupção, comportamento inesperado, alteração não compreendida
ou perda de confiança no estado atual — ele deve possuir um mecanismo formal
para: determinar se existe um problema, identificar o último estado
conhecido-bom, avaliar o comprometimento, reconstruir com identidade
verificada, provar a reconstrução e saber quando PARAR e pedir autorização.

## A invariante central

```
LAST COMMIT ≠ LAST VALID COMMIT ≠ LAST KNOWN-GOOD STATE
```

Um commit pode estar perfeitamente correto e não representar o estado
operacional que se quer recuperar. O recovery procura o último estado
**conhecido-bom** (chain, merkle, verify limpo), não o último commit.

## O lema

> 🎩 **Não restaure aquilo que você ainda não entende.**

Às vezes o sistema parece quebrado, mas está apenas **diferente**. O
recovery diferencia corrupção de diferença antes de tocar no estado.

## A espinha dorsal

```
DETECTAR → QUESTIONAR → PROVAR → CORRIGIR → RECUPERAR → EXPLICAR
```

## O mapa de camadas (o que cada recovery toca)

| Camada | Tipo | Recovery |
|---|---|---|
| Git (código, protocolos, CONSTITUTION) | fonte de verdade | verificar o commit válido |
| Memória (learnings, blocks, chain, merkle) | fonte de verdade | verificar a chain; blocks imutáveis |
| family_chain (git-anchored) | fonte de verdade | `cosca-check` verifica |
| knowledge.db + índices | derivado | re-index + re-embed (identidade verificada) |
| Identidade (digest + fail-closed) | protegida | verificar o digest antes de qualquer reconstrução |
| Config (config.yaml) | local | reconstruir defaults reproduzíveis |
| Caches, sessões, runtime | descartável/ambiental | ignorar ou recriar |

## As 9 fases

### 1. DETECTAR — existe um problema?
Sinais (alarmes): `cosca doctor` com falhas · `cosca knowledge verify` com
issues · chain/merkle divergente · comportamento inesperado · queda de
recall · ingestão estranha · o guard DENY inesperado.

### 2. QUESTIONAR — o interrogatório (L377)
```
O que estou vendo? · É corrupção real ou estado apenas DIFERENTE?
Qual hipótese? · O que sei vs o que infiro? · Qual evidência?
```
Casos históricos: o provider caiu no local (L369) · o digest mudou ·
a config não foi carregada (L383) — todos eram "diferente", não corrupção.

### 3. IDENTIFICAR — o último estado conhecido-bom
Fontes: o último `verify` limpo registrado · o último block da chain ·
o merkle · o último commit que passou na suíte · a última fotografia
(snapshot da campanha). **Não o último commit — o último VÁLIDO.**

### 4. AVALIAR — o que está comprometido
Classificar cada camada (o mapa acima): fonte de verdade vs derivado vs
descartável. O que é **recuperável** (a chain, o conhecimento versionado)
vs **reconstruível** (db, índices) vs **perdido por design** (sessões).

### 5. DECIDIR — o recovery é seguro?
A régua: **não tocar no que não entende**. Se a causa é UNKNOWN → PARAR e
pedir autorização humana (o Cognitive Contract: DENY e explique). Se o
problema é só diferença (ex.: provider/config) → corrigir a causa, não o
estado.

### 6. RECONSTRUIR — com identidade verificada
O caminho provado (L378-L379): re-index + re-embed **com o digest verificado
e o fail-closed ativo** — nunca com fallback silencioso. Backup antes
(regra da casa). O dedup preventivo e o filtro de triviais ativos.

### 7. PROVAR — a reconstrução foi bem-sucedida
Repetir o L375/Fase 6: comparação semântica (cosine/cross-query) ·
`verify` limpo · `doctor` verde · o digest confirmado. Os números da prova
registrados.

### 8. PRESERVAR — congelar o estado bom
Nova fotografia (snapshot) · chain/merkle atualizados · commit ·
o estado validado vira o novo last-known-good.

### 9. PARAR — quando pedir autorização humana
Sempre que: o risco for alto · a causa permanecer UNKNOWN · a ação for
destrutiva · o recovery tocar produção sem a ordem explícita do Don.

## As regras de ouro

1. **O erro não destrói a história** — a conclusão errada vira evidência
   (o L373 existe porque o L369 existiu). O recovery registra o que
   detectou, decidiu, reconstruiu e provou — o rastro.
2. **Recovery ≠ rollback cego** — "voltar para o último commit" é UMA das
   opções, não o default. Primeiro entender, depois agir.
3. **Cada camada tem seu recovery** — a memória (chain/merkle), o
   conhecimento (re-index/re-embed), a identidade (digest), a config
   (defaults reproduzíveis).
4. **O recovery deixa rastro** — para a próxima vez ser mais rápida: o
   histórico de recoveries é memória de engenharia.
5. **Preservar o que está válido** — se o estado atual é válido (provado),
   não mexer (a lição do L380).

## Verificação

O protocolo é testado pela sua própria disciplina: um recovery real
(quando necessário) segue as 9 fases e o rastro é auditável. A
reconstrução determinística já foi provada (L379: 138/138 compatíveis).