# 02 — SHADCN/UI INTELLIGENCE

> Stack 02 da Cosca Engineering Intelligence Matrix.
> Doutrina do sistema de componentes da casa: código é SEU, composição em vez de biblioteca.

## MISSÃO
Construir a component library do próprio projeto com shadcn/ui — sem depender de um pacote de biblioteca — usando Radix (headless), cva (variantes) e Tailwind (estilo), e dominando por que "é COMO você constrói sua component library".

## PRINCÍPIOS CORE
1. **Open Code** — não há package de biblioteca; o CLI copia o código para `components/ui/`; o código é SEU, editável · UNIVERSAL
2. **Composition, não framework** — Radix primitives (headless) + Tailwind (estilo) + cva (variantes) montam os componentes · UNIVERSAL
3. **Beautiful Defaults** — componentes prontos mas que não te prendem; você edita direto · STRONG
4. **AI-Ready** — componentes pequenos, copiáveis e legíveis, ideais para agentes editarem · STRONG
5. **Registrado em `components.json`** — a CLI conhece aliases, framework e style; é o contrato do setup · STRONG

## REGRAS DE DECISÃO
- `cn()` em `lib/utils` (tailwind-merge + clsx) para composição de classes com precedência previsível.
- Variantes via `cva()` (class variance authority) — `base + variants + defaultVariants`.
- Dark mode por vars + `data-theme`/classe no HTML, re-mapeando a camada semântica (nunca redeclarar token cru).
- Componente novo: procurar no shadcn antes de escrever do zero; se adaptar, editar a cópia local.
- Não criar camada de abstração em cima do componente shadcn para "customizar" — edite o código copiado.
- Manter o style consistentemente com os tokens `@theme` da casa.

## ANTI-PATTERNS
`wrap de biblioteca para customizar shadcn` · `tratar shadcn como dependência npm (não é)` · `depender de props mágicas sem entender o Radix por baixo` · `dark mode com hex cru por tema em vez de re-mapeamento` · `reinventar accordion/modal/dropdown quando shadcn já resolve` · `cva vazio com só uma variante` · `cn() sem tailwind-merge deixando conflito de classe silencioso`

## CHECKLIST
- [ ] Componentes em `components/ui/` (cópia local, editável)
- [ ] `cn()` + `cva` presentes para variantes
- [ ] `components.json` com aliases corretos
- [ ] Dark mode por `data-theme` re-mapeando tokens semânticos
- [ ] Acessibilidade Radix preservada (ARIA, foco, teclado)
- [ ] Nenhum componente shadcn envolvido por abstraction sem motivo

## A REGRA
shadcn/ui não é uma component library — é a receita de COMO construir a SUA component library: copie, edite, compõe; o código é seu e você responde por ele.

## REFERÊNCIAS
shadcn/ui docs (ui.shadcn.com) · Radix UI primitives · class-variance-authority · tailwind-merge · PADRAO-COSCA §2 e §6

## CONHECIMENTO (schema _SCHEMA.md)

```
ID: frontend-009
DOMAIN: frontend
TITLE: shadcn como metadoutrina (código é SEU)
PROBLEM: Component library em pacote npm vira dependência fechada e difícil de customizar
CONTEXT: Design systems em apps React/Tailwind
PRINCIPLE: Copiar código para o próprio repo (Open Code) supera depender de package
RECOMMENDATION: Usar o CLI para adicionar componentes em `components/ui/` e editar a cópia local
WHEN_TO_USE: Todo projeto com Tailwind que precisa de componentes consistentes
WHEN_NOT_TO_USE: Projeto que já tem design system proprietário maduro e testado
TRADE_OFFS: Código duplicado por projeto vs independência total e editabilidade
EXAMPLE: `npx shadcn@latest add button` gera `components/ui/button.tsx` sob seu controle
COUNTER_EXAMPLE: Importar de um package proprietário e não conseguir mudar o token interno
FAILURE_MODES: Copiar e não versionar; divergir o componente e esquecer o upgrade local
REFERENCES: shadcn/ui docs
CONFIDENCE: UNIVERSAL
SOURCE: shadcn/ui + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: frontend-010
DOMAIN: frontend
TITLE: Radix primitives + cva + Tailwind para composição
PROBLEM: Behavior acessível (foco, ARIA, teclado) e estilo precisam coexistir sem conflito
CONTEXT: Componentes shadcn no projeto
PRINCIPLE: Compor camadas: Radix (headless/behavior) + cva (variantes) + Tailwind (visual)
RECOMMENDATION: Não reescrever primitivas; adicionar comportamento via composição e variantes via cva
WHEN_TO_USE: Todo componente com estado (dialog, dropdown, tabs, toast)
WHEN_NOT_TO_USE: Elemento puramente visual (ícone, badge simples) que não precisa de Radix
TRADE_OFFS: Dependência de primitivas headless vs acessibilidade e robustez de graça
EXAMPLE: Dialog = Radix `Dialog.Root` + cva para overlays/variantes + classes Tailwind
COUNTER_EXAMPLE: Implementar aria/teclado na mão para um modal simples
FAILURE_MODES: Quebrar composição ao estilizar dentro da primitiva em vez de na camada própria
REFERENCES: Radix UI docs · cva docs
CONFIDENCE: UNIVERSAL
SOURCE: shadcn/ui + Radix
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: frontend-011
DOMAIN: frontend
TITLE: Dark mode por re-mapeamento com data-theme
PROBLEM: Dark mode com hex cru por tema duplica tokens e cria drift
CONTEXT: Design system com tokens semânticos
PRINCIPLE: Dark mode re-mapeia a camada semântica; tokens crus ficam em camada Primitive
RECOMMENDATION: Alternar `data-theme` no HTML e re-mapear vars semânticas no CSS
WHEN_TO_USE: Qualquer app com dark mode
WHEN_NOT_TO_USE: Tema único sem alternância
TRADE_OFFS: Um switch de contexto a mais vs consistência sem duplicação
EXAMPLE: `html[data-theme="dark"]` re-mapeia `--background`/`--foreground`; componentes não mudam
COUNTER_EXAMPLE: Dois conjuntos de `--color-*` crus copiados por componente
FAILURE_MODES: Preferência de sistema vs manual conflitando sem media query + override claro
REFERENCES: shadcn/ui theming docs · PADRAO-COSCA §2
CONFIDENCE: STRONG
SOURCE: shadcn/ui + PADRAO-COSCA
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```

```
ID: frontend-012
DOMAIN: frontend
TITLE: Não envolver componente shadcn com abstraction
PROBLEM: Wrap para customizar cria camada que esconde o código copiado e complica a edição
CONTEXT: Equipe "padronizando" componentes shadcn
PRINCIPLE: O valor do shadcn é o código aberto e editável; embrulhar destrói esse valor
RECOMMENDATION: Editar a cópia em `components/ui/`; se o padrão se repete, use cva com variantes
WHEN_TO_USE: (quando aplicar) Nunca como primeiro reflexo — só quando há regra de produto real
WHEN_NOT_TO_USE: Customização estética/variante que cva resolve
TRADE_OFFS: Aparência de "padronização" vs perda de transparência e editabilidade
EXAMPLE: `Button` shadcn usado direto; variantes novas adicionadas em cva
COUNTER_EXAMPLE: `src/components/AppButton.tsx` re-exportando com 10 props repassadas
FAILURE_MODES: Bug na camada de wrap que o dev não acha por não olhar o shadcn
REFERENCES: shadcn/ui philosophy
CONFIDENCE: STRONG
SOURCE: shadcn/ui + PADRAO-COSCA §6
VERSION: 1
CREATED_AT: 2026-08-16
UPDATED_AT: 2026-08-16
```
