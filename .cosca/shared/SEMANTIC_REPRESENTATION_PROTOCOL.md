# SEMANTIC REPRESENTATION PROTOCOL — Similaridade ≠ Identidade

> **Version**: 1.0.0 | **Status**: active | **Owner**: Kernel / Semantic Memory | **Last Updated**: 2026-08-26
> **Tipo**: PROTOCOL (invariável do sistema) — não pode ser violado por nenhum agente.
> **Origem**: caso concreto "Fiat Fiorino → ambulância/carrinho Lego"; ensinamento do professor.

---

## ESTE É UM PROTOCOLO, NÃO UM LEARNING

Este documento define uma **invariante de arquitetura/comportamento** do Cosca. Diferente de um *learning* (que registra "aprendi isso") ou de uma *skill* (que diz "se precisar, faça assim"), um **protocolo** diz: **"mesmo que alguém tente fazer diferente, esta regra não pode ser violada."**

As camadas são separadas e não se confundem:

| Camada | Pergunta | Onde vive |
|--------|----------|-----------|
| **LEARNING** | "O que aprendi?" | `.cosca/memory/agent/.../learnings.md` (caso Fiorino) |
| **SKILL** | "Se precisar fazer, como?" | `.cosca/skills/semantic-visual-embedding/SKILL.md` |
| **PROTOCOL** | "Que regra não pode ser violada?" | **este arquivo** (`shared/SEMANTIC_REPRESENTATION_PROTOCOL.md`) |

---

## PRINCÍPIO FUNDAMENTAL (INVARIANTE)

> **Similaridade vetorial NUNCA equivale a identidade semântica.**
> O vetor é uma *aproximação* do significado, não a *autoridade* sobre o significado.

- Similaridade **visual ≠** identidade **semântica**.
- Duas imagens podem ser visualmente parecidas e representar **entidades diferentes**.
- Duas imagens podem ter estilos, cores ou ângulos diferentes e representar **a mesma entidade**.
- `aparência ≠ identidade ≠ função ≠ contexto`.

**Esta regra não admite exceção.** Nenhum agente pode, ao responder uma solicitação de entidade específica, substituí-la pela "mais parecida" com base em proximidade vetorial.

---

## CAMADAS DE UMA REPRESENTAÇÃO VISUAL

Ao vetorizar/renderizar uma imagem, raciocine **conceitualmente em camadas** — da mais rígida à mais flexível:

| # | Camada | O que é | Exemplo (Fiat Fiorino) | Pode variar? |
|---|--------|---------|------------------------|:---:|
| 1 | **Identidade** | O que o objeto É | Fiat Fiorino (não "carro"/"van") | **NÃO** |
| 2 | **Classe** | Categoria | veículo → utilitário → van compacta | NÃO |
| 3 | **Estrutura** | Silhueta/geometria que o torna reconhecível | cabine dianteira, caixa de carga, 4 rodas, proporção de van, eixos | **NÃO** |
| 4 | **Atributos** | Detalhes visuais secundários | cor, rodas, faróis, acabamento, estado | SIM |
| 5 | **Contexto** | Onde/para que | ícone top-down em mapa de rotas | SIM |
| 6 | **Função** | O que a representação precisa permitir | reconhecimento rápido, rotação por heading, legibilidade | — |

**Regra de precedência:** Identidade (1) > Estrutura (3) > Similaridade semântica. Atributos (4) e estilização (5/6) nunca podem eliminar a identidade ou a estrutura.

---

## REGRA DE INVARIÂNCIA

Separe o que **deve permanecer** do que **pode variar**:

**INVARIANTES (nunca perdem):**
- identidade
- classe essencial
- estrutura
- função principal

**VARIÁVEIS (podem mudar sem alterar identidade):**
- cor, iluminação, perspectiva, fundo, textura
- estilo artístico, resolução
- pequenas alterações visuais

> O embedding deve permitir que **representações diferentes da mesma entidade** (foto · vista topo · ícone 2D · render em outro estilo) permaneçam semanticamente próximas, **desde que identidade e estrutura sejam preservadas**.

---

## REGRA ANTI-FALSO-POSITIVO

**Não assumir: "parece parecido → é a mesma coisa."**

- `Fiat Fiorino ≠ Fiat Tempra` (ambos Fiat, entidades diferentes)
- `Fiat Fiorino ≠ ambulância` (formato de van parecido, entidades diferentes)
- `Fiat Fiorino ≠ van genérica` (agregação de classe, perde identidade)

A similaridade **auxilia recuperação**; **não substitui identificação**.

---

## VETOR + METADADOS (NUNCA SÓ VETOR)

Não dependa exclusivamente do embedding. Associe **metadados estruturados** à representação:

```
identity      → exatamente o quê é (ex: fiat_fiorino)
class         → categoria (vehicle / van)
subclass      → subcategoria (delivery_van)
attributes    → cor, rodas, acessórios
context       → ícone top-down para mapa
function      → reconhecimento + rotação por heading
scope         → em qual espaço de conhecimento isto é válido
provenance    → origem da representação
epistemic     → nível de certeza (source: explicit request vs inference)
```

| Pergunta | Quem responde |
|----------|---------------|
| "O que é semanticamente parecido?" | **Vetor** |
| "O que exatamente é isso?" | **Metadados** |
| "Em que espaço posso usar?" | **Scope** |

---

## REGRA DE RECUPERAÇÃO

A recuperação de representações deve combinar **quatro fatores**, nunca apenas cosine similarity:

1. **similaridade vetorial** (propor candidatos)
2. **identidade** (o que exatamente é)
3. **contexto** (onde será usado)
4. **scope** (onde é válido)
5. **proveniência** (de onde veio)

> **Nunca trate o maior cosine similarity como verdade absoluta.**

---

## TESTE DE QUALIDADE DE UMA REPRESENTAÇÃO VETORIAL

Antes de considerar válida, teste:

- [ ] **Mesmo objeto, aparência diferente** → permanece semanticamente próximo.
- [ ] **Objetos visualmente parecidos, identidade diferente** → permanecem distinguíveis.
- [ ] **Mesmo objeto, contexto diferente** → identidade permanece recuperável.
- [ ] **Objeto genérico vs entidade específica** → entidade específica mantém identidade, não é reduzida à categoria.
- [ ] **Teste de silhueta** → com cor/textura/sombra removidos, a silhueta ainda distingue a entidade de um sedã/ambulância/SUV/caminhão/brinquedo? Se NÃO, falhou.

---

## PRINCÍPIO PERMANENTE

> **O embedding representa significado; os metadados preservam identidade; o contexto determina relevância.**
> **Nunca confundir proximidade vetorial com equivalência semântica.**
> **Vetores encontram conceitos próximos. Identidade determina a entidade. Estrutura determina o reconhecimento. Contexto determina a representação.**

---

## ENFORCEMENT

- Este protocolo é **invariável** e aplicável a **qualquer entidade específica**: veículos, pessoas, animais, ferramentas, equipamentos, edifícios, marcas, personagens, componentes técnicos.
- Qualquer agente que gere imagem/ícone/modelo 2D/3D/UI deve consultar este protocolo **antes** de gerar.
- Violação = substituir entidade solicitada pela "mais parecida" por proximidade vetorial. **Nunca aceitável.**

---

## HISTÓRICO

| Versão | Data | Autor | Mudanças |
|--------|------|-------|----------|
| 1.0.0 | 2026-08-26 | Kernel | Criação a partir do caso Fiorino + ensinamento do professor |
