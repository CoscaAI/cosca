---
name: semantic-visual-embedding
description: Use when the user asks to convert a visual reference or concept into a vector representation (SVG, drawable, paths) while preserving semantic identity.
---

# VECTOR REPRESENTATION PROTOCOL — Representação Vetorial Semântica

> **Version**: 2.0.0 | **Status**: active | **Owner**: Kernel / Semantic Memory | **Last Updated**: 2026-08-26
> **Tipo**: SKILL (procedimento operacional geral). Complementa o [SEMANTIC_REPRESENTATION_PROTOCOL](../shared/SEMANTIC_REPRESENTATION_PROTOCOL.md) (invariável).

---

## DESCRIPTION

Transformar uma **referência visual/conceito** em uma **representação vetorial** (SVG, vector drawable, paths) **preservando a identidade semântica** do objeto. **Geral e reutilizável para QUALQUER entidade**: veículo, pessoa, animal, prédio, ferramenta, máquina, árvore, personagem, equipamento, ícone de ação, marcador de mapa, cena.

> **A tarefa NÃO é transformar uma imagem em formas arbitrárias.**
> É **descobrir a estrutura visual que torna aquilo reconhecível** e codificá-la em geometria vetorial.

**Regra de ouro:**
> **IDENTIDADE > SIMILARIDADE > DECORAÇÃO.**
> **IDENTIDADE PRIMEIRO. GEOMETRIA DEPOIS. DECORAÇÃO POR ÚLTIMO.**
> Nunca compensar falta de compreensão adicionando detalhes.
> Um vetor simples e semanticamente correto é **superior** a um vetor detalhado que representa o objeto errado.

**FAIL** se: parecer outro objeto, parecer símbolo genérico, vista errada, proporção errada, detalhes mascararem estrutura incorreta, silhueta não preservar identidade, componente com semântica incorreta.

---

## PIPELINE OBRIGATÓRIO

```
IMAGEM/CONCEITO
  → IDENTIFICAÇÃO
  → VISTA
  → SILHUETA
  → PARTES SEMÂNTICAS
  → RELAÇÕES ESPACIAIS
  → GEOMETRIA
  → VETOR
  → VALIDAÇÃO
```

**Nunca começar desenhando paths.** Começar pela **análise estrutural do objeto**.

---

## PASSO 1 — IDENTIFICAR

Antes de criar qualquer vetor, determine:
- **objeto** (o que é)
- **categoria** (classe funcional)
- **identidade específica** (modelo/tipo reconhecível)
- **orientação** (aponta para onde)
- **vista** (qual ângulo)
- **proporções** (relações dimensionais)
- **características distintivas** (o que o diferencia)

**Pergunta obrigatória:**
> "Se eu remover cor, sombra e detalhes, quais formas ainda fazem esse objeto ser reconhecível?"
> Essas formas são **prioritárias**.

```
"carro"          → veículo genérico pode ser aceitável
"van"            → deve ter estrutura de van
"Fiat Fiorino"   → deve preservar características reconhecíveis da Fiorino
"ambulância"     → NÃO pode ser produzida só porque tem 4 rodas e carroceria parecida
```

---

## PASSO 2 — DEFINIR A VISTA

Escolha explicitamente (nunca por ser "mais fácil"):
- frontal, traseira, lateral, superior/top-down, perspectiva, isométrica, 3/4

> **NUNCA** substituir uma vista lateral por top-down apenas porque é mais fácil.
> A vista é definida pelo **contexto de uso** (ex: marcador de mapa rotacionável → top-down; ícone de catálogo → lateral/3-4; sinalização → frontal).

---

## PASSO 3 — EXTRAIR A SILHUETA

Determine:
- contorno externo
- proporção (largura/altura)
- relações entre partes
- pontos estruturais importantes

**Teste de silhueta:**
> Se o vetor for preenchido com **uma única cor** e **todos os detalhes removidos**, ainda reconheço o objeto?
> Se NÃO → a estrutura está errada. **Corrigir antes de detalhar.**

---

## PASSO 4 — DECOMPOR EM COMPONENTES SEMÂNTICOS

Separe mentalmente o objeto em componentes **com função visual** (não para preencher espaço):

```
VEÍCULO
├── carroceria
├── cabine (para-brisa, janela, porta)
├── capô / teto
├── compartimento de carga
├── retrovisores, para-choques, faróis, lanternas
├── rodas + eixos
└── elementos distintivos
```

Cada componente deve ter **propósito semântico**, não decorativo arbitrário.

---

## PASSO 5 — REGISTRAR RELAÇÕES ESPACIAIS

Antes da geometria, registre as relações (mais importante que o formato exato):

- esquerda/direita · frente/traseira · cima/baixo
- alinhamento · simetria · proporção · distância
- sobreposição · contato · escala relativa

```
RODA
├── pertence ao veículo
├── possui eixo
├── está ABAIXO da carroceria
├── tem par correspondente (simetria)
└── distância proporcional entre eixos
```

---

## PASSO 6 — CLASSIFICAR (HIERARQUIZAR)

Classifique cada elemento:

| Classe | Definição | Exemplo |
|--------|-----------|---------|
| **ESSENCIAL** | Sem ele o objeto perde identidade | cabine curta + caixa de carga (Fiorino) |
| **ESTRUTURAL** | Mantém categoria e proporção | carroceria, teto, eixos |
| **DISTINTIVO** | Diferencia de outros da mesma categoria | retrovisores, emblema |
| **DECORATIVO** | Pode ser removido sem perda de identidade | texturas, logotipos, sombras |

**Prioridade de implementação:** `ESSENCIAL → ESTRUTURAL → DISTINTIVO → DECORATIVO`.

---

## PASSO 7 — CONSTRUIR A GEOMETRIA

Só depois da estrutura semântica definida, escolha:
- `path` · `rect` · `circle` · `ellipse` · `line` · `polygon`
- curvas Bézier · strokes · fills

**Preferir geometria simples quando preserva identidade.**
Não adicionar complexidade sem ganho semântico.

---

## PASSO 8 — PRESERVAR PROPORÇÕES

**Nunca** use dimensões arbitrárias apenas porque cabem no viewport. Estabeleça:
- comprimento · largura · altura
- distância entre eixos
- posição das rodas/janelas/elementos distintivos

> Não transformar: van em hatch · caminhão em carro · pessoa em boneco genérico · cachorro em animal genérico · prédio em cubo.

---

## PASSO 9 — ESTILO (SÓ DEPOIS DA IDENTIDADE)

Aplique por último (flat, outline, filled, monochrome, duotone, material, mapa, UI, game).

**Estilo NUNCA pode alterar a identidade.**

---

## PASSO 10 — REDUÇÃO / ESCALA

Teste o vetor em: grande · médio · pequeno (ex: 32–48px para mapa).
Remova detalhes que não sobrevivem ao tamanho real de uso.

---

## PASSO 11 — TESTE DE IDENTIDADE

> "Se eu remover cor, textura, sombra e texto, o objeto continua reconhecível?"
> Se NÃO → revisar a **estrutura**, não adicionar detalhes.

---

## PASSO 12 — TESTE DE CONFUSÃO

> "O resultado pode ser confundido com outro objeto próximo?"
> Se SIM → identificar **qual característica estrutural está faltando** e corrigir.

---

## VALIDAÇÃO FINAL (obrigatória)

- [ ] **Identidade** — o objeto solicitado é o representado?
- [ ] **Vista** — a vista pedida foi respeitada?
- [ ] **Silhueta** — corresponde ao objeto?
- [ ] **Proporção** — relações dimensionais plausíveis?
- [ ] **Estrutura** — partes essenciais existem e estão posicionadas?
- [ ] **Semântica** — algum elemento introduziu identidade errada?
- [ ] **Escala** — continua reconhecível no tamanho real?

**REGRA DE FAIL (rejeitar se):** parecer outro objeto · símbolo genérico · vista errada · proporção errada · detalhes mascararam estrutura incorreta · silhueta não preservar identidade · componente com semântica incorreta.

---

## VETOR SEMÂNTICO INTERMEDIÁRIO (ANTES DO SVG)

Produza um "vetor semântico" conceitual antes dos paths:

```
OBJECT
├── identity            (ex: fiat_fiorino)
├── viewpoint           (ex: lateral_orthographic)
├── silhouette          (descrição da silhueta)
├── proportions         (relações dimensionais)
├── structural_parts    (listas das partes)
├── identity_features   (o que torna reconhecível)
├── optional_features   (pode variar)
├── forbidden_features  (o que NÃO pode ter)
└── vector_geometry     (paths/curvas)
```

### Exemplo — Fiat Fiorino lateral 2D

```
OBJECT
├── identity: Fiat Fiorino
├── vista: lateral ortográfica
├── identidade: van compacta de carga; cabine curta; grande volume traseiro;
│              teto relativamente alto; 2 portas laterais; 2 rodas;
│              para-brisa inclinado; capô curto; traseira vertical
├── NÃO CONFUNDIR COM: ambulância, hatch, sedã, furgão grande, caminhão, brinquedo/Lego
├── geometria principal: silhouette, cabine, compartimento de carga, rodas,
│                        janelas, portas, retrovisor, lanternas/faróis
├── representação: SVG/paths; proporções preservadas; poucos nós; escala independente
└── validação: "ainda reconhecível como Fiorino sem cor, logo e detalhes?"
```

---

## CORPUS DE REFERÊNCIA (estudar, não copiar)

Estude grandes coleções como **exemplos de engenharia vetorial** e aprenda como designers transformam significado em geometria.

| Fonte | Enfoque |
|-------|---------|
| **OpenStreetMap/map-icons** | Ícones de mapa em SVG por categoria (vehicle, transport, routing); força relação objeto→categoria→representação |
| **Cardog Icons** | Ícones automotivos em SVG, centenas de marcas mono/coloridas; linguagem visual automotiva consistente |
| **vehiclespecs/brand-logos** | Identidade visual de marcas em vetor |
| **Tabler Icons** | Milhares de SVGs consistentes (grid, paths, strokes, variações) — melhor ponto de partida |
| **Material Design Icons** | Mesmo conceito em estilos diferentes; redução a formas vetoriais |
| **OpenMoji** | Semântica visual + identidade + composição (SVG fonte/export, metadados, guia de estilo) |

**Não memorizar código SVG — extrair:** conceito, vista, silhueta, componentes, relações, geometria, estilo, viewport, sistema de coordenadas.
> O SVG é uma **implementação** de uma representação semântica. **Não confundir código SVG com a semântica.**

---

## GENERALIZAÇÃO

Depois de aprender um objeto, **testar a regra em objetos diferentes**: veículos (carro, van, caminhão, ônibus, moto, ambulância) → animais (cachorro, gato, cavalo, pássaro) → objetos (cadeira, mesa, computador, telefone, bicicleta).

> O objetivo é aprender o **princípio de representação**, não decorar uma forma.

---

## EXEMPLO CRÍTICO

**Pedido:** "Fiat Fiorino lateral 2D para marcador de mapa."

| Resultado | Veredito |
|-----------|----------|
| "retângulo + quatro rodas" | **FAIL** |
| "van genérica" | PARCIAL/FAIL se identidade específica pedida |
| "Fiorino reconhecível em silhueta lateral, cabine curta, caixa compacta, proporções características, 4 rodas posicionadas, elementos distintivos mínimos" | **PASS** |

---

## PRINCÍPIO FINAL

Aprender, **não** "como desenhar uma Fiorino", mas:
> **"Como converter identidade visual em representação vetorial."**
> **IDENTIDADE PRIMEIRO. GEOMETRIA DEPOIS. DECORAÇÃO POR ÚLTIMO.**

Reutilizável para qualquer objeto, veículo, personagem, equipamento, símbolo ou cena.

---

## RELATED

- [Protocolo de Representação Semântica](../shared/SEMANTIC_REPRESENTATION_PROTOCOL.md) — a invariante
- [Learning do caso Fiorino](../memory/agent/cosca-kernel/learnings.md)
- [SKILL_TEMPLATE](../SKILL_TEMPLATE.md)

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-08-26 | Kernel | Esqueleto inicial |
| 2.0.0 | 2026-08-26 | Kernel | Reescrita com o protocolo completo do professor (12 passos, regras de FAIL, corpus, vetor semântico intermediário, generalização) |
