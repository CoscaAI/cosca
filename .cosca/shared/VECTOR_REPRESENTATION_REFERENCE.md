# VECTOR REPRESENTATION — Corpus de Referência & Pesquisa de Habilidades

> **Version**: 1.0.0 | **Status**: active | **Owner**: Kernel / Semantic Memory | **Last Updated**: 2026-08-26
> **Propósito**: material de estudo formal da skill `semantic-visual-embedding` + resultado da pesquisa do Don sobre como aumentar a habilidade de representação vetorial semântica.
> **Uso**: estudar como os grandes design systems **estruturam semanticamente** (objeto → categoria → representação), NÃO copiar código.

---

## 1. CONCLUSÃO DA PESQUISA (importante)

**Não existe um modelo pronto de "gerar SVG por IA com identidade semântica correta" no GitHub.** As buscas por "LLM SVG generation", "SVG semantic embedding" retornaram apenas repositórios obscuros (0-5 stars).

**O que isso ensina:** o valor NÃO está em copiar um modelo/arquivo. Está em:
1. **Estudar ícones sets de alta qualidade** (como transformam significado em geometria);
2. **Ter um MÉTODO (protocolo/skill) para derivar uma representação** — que é exatamente o que o professor ensinou.
3. **A identidade vem da análise estrutural, não de um dataset.**

---

## 2. REPOSITÓRIOS DE REFERÊNCIA (stars/licença verificados hoje via GitHub API)

### Ícones sets de alta qualidade — estudar semântica + geometria

| Repo | Stars | Licença | Por quê estudar |
|------|-------|---------|-----------------|
| **google/material-design-icons** | 53.8k | Apache-2.0 | O mesmo conceito em estilos/variantes; redução de objetos a formas vetoriais |
| **feathericons/feather** | 25.9k | MIT | Ícones minimalistas consistentes (linha, poucos nós, grid limpo) |
| **lucide-icons/lucide** | 24.2k | ISC | Fork do Feather, mais moderno; ótima engenharia de paths limpos |
| **tabler/tabler-icons** | 21.5k | MIT | 6.100+ ícones consistentes (grid, strokes, variações) — melhor ponto de partida |
| **mui/material-ui** | 98.9k | MIT | Como um design system integra ícones com tema |
| **chakra-ui/chakra-ui** | 40.6k | MIT | Design system + iconografia coesa |
| **twbs/icons** (Bootstrap) | 8.1k | MIT | SVG icon library oficial do Bootstrap |
| **Remix-Design/RemixIcon** | 8.3k | Apache-2.0 | Sistema de ícones "neutro", consistente |
| **iconify/iconify** | 6.3k | MIT | Agregador de dezenas de sets; metadados + nomeação semântica |
| **coreui/coreui-icons** | 2.1k | — | Design premium |
| **phosphor-icons/core** | 370 | MIT | Família de ícones (thin/light/regular/bold/fill/duotone) |
| **ant-design/ant-design-icons** | 1.1k | MIT | Iconografia antd |

### Semântica + identidade visual (foco em objeto/modelo)

| Repo | Stars | Licença | Por quê |
|------|-------|---------|---------|
| **OpenStreetMap/map-icons** | 49 | — | Ícones de mapa por categoria (vehicle, transport, routing); relação objeto→categoria→representação. **Apesar de pequeno, é o mais alinhado ao caso** |
| **cardog-ai/icons** | 27 | — | Ícones automotivos (marcas, mono/color); linguagem visual automotiva |
| **vehiclespecs/brand-logos** | <10 | — | Identidade visual de marcas |

> ⚠️ Nota: os 3 últimos (que o professor citou) são **pequenos/desatualizados**. Os gigantes de qualidade são Material, Feather, Lucide, Tabler, Iconify.

### Semântica + metadados (OpenMoji)

| Repo | Stars | Licença | Por quê |
|------|-------|---------|---------|
| **hfg-gmuend/openmoji** | 4.5k | CC-BY-SA-4.0 | Semântica visual + identidade + composição (SVG fonte, export, metadados, guia de estilo) |

---

## 3. COMO EXTRAIR CONHECIMENTO DE UM SET (não copiar código)

Ao estudar um SVG, extrair (NÃO memorizar o path):

```
conceito        → o que representa
vista           → ângulo/perspectiva
silhueta        → contorno externo (teste de identidade)
componentes     → partes semânticas
relações        → proporção, alinhamento, simetria, contato
geometria       → path/rect/circle/line, curvas Bézier, nós
estilo          → flat/outline/filled, stroke width, grid
viewport        → sistema de coordenadas, escala
```

> **O SVG é uma implementação de uma representação semântica. Não confundir código SVG com a semântica.**

---

## 4. LIÇÕES QUE ESTES SETS ENSINAM

1. **Consistência de grid** (ex: 24x24) e **viewport unificado** — a identidade vem da forma, não do tamanho.
2. **Poucos nós** = legível em escala pequena (regra de escala do professor).
3. **Nomeação semântica em camadas** (ex: `vehicle/van`, `transport/delivery`) — mesma hierarquia objeto→categoria→representação.
4. **Estilo não muda identidade** — o mesmo objeto em outline/filled/duotone é o mesmo objeto.
5. **Metadados** (tag, categoria, estilo) acompanham o SVG — exatamente o `vetor + metadados` do protocolo.

---

## 5. RECOMENDAÇÃO DE ENGENHARIA PARA O COSCA

Para elevar a habilidade de geração vetorial sem **depender de copiar**:

- **Estudar os sets acima** como corpus (principalmente Tabler, Feather, Lucide, Material).
- **Manter a skill `semantic-visual-embedding`** como o MÉTODO (identidade → silhueta → partes → relações → geometria → validação).
- **Atrelar metadados** a cada representação gerada (`identity`, `class`, `context`, `scope`, `provenance`).
- **Validar por silhueta** (remover cor/detalhe → ainda reconhecível?) antes de declarar sucesso.
- **Registrar aprendizados** a cada nova entidade, para generalizar o princípio.

> **Proibido:** copiar SVG cegamente / nao confundir similaridade com identidade / substituir objeto pedido pelo "mais parecido".

---

## RELATED

- [Skill semantic-visual-embedding](../skills/semantic-visual-embedding/SKILL.md)
- [Protocolo de Representação Semântica](../shared/SEMANTIC_REPRESENTATION_PROTOCOL.md)
- [Learning do Kernel](../memory/agent/cosca-kernel/learnings.md)

## HISTORY

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0.0 | 2026-08-26 | Kernel | Pesquisa do Don + corpus do professor + estudo dos design systems vetoriais |
