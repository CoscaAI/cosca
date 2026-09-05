# SYNTHESIS — Comparação dos Sistemas e Reconciliação

> Nenhum sistema é verdade absoluta. Aqui: onde concordam, onde divergem, e a reconciliação.

## Onde concordam (universal por consenso)

| Tema | Consenso | Fontes |
|---|---|---|
| Anti "AI slop" (genérico) | Identidade própria, 1 elemento de destaque, restrição | Anthropic, Impeccable, better-web-ui, UI UX Pro Max |
| Acessibilidade é base | Focus, teclado, contraste, reduced-motion, semântica | Radix, React Aria, Vercel, Carbon, Impeccable, UI UX Pro Max |
| Estados completos | Loading/empty/error/success sempre | Vercel, Anthropic, better-web-ui, Impeccable |
| Feedback claro | Sucesso confirma, erro explica e orienta | Anthropic, Polaris, Impeccable |
| Sem emojis como ícones | Ícones SVG consistentes | Impeccable, UI UX Pro Max, Anthropic |
| Cópia = design | Nomear pelo que o usuário controla, voz ativa | Anthropic, Vercel writing, Polaris |
| Testar/validar antes de entregar | Critique + audit antes de ship | Impeccable, better-web-ui |

## Onde divergem (e a reconciliação)

### Dark vs Light
- **UI UX Pro Max**: Dark OLED recomendado para operação/dados; light para wellness/lifestyle
- **Impeccable**: nem toca o assunto (aceita ambos)
- **Reconciliação**: é CONTEXTUAL. Se o domínio justifica (dados/operação/noite), dark com contraste 4.5:1. Senão light. Não é default universal.

### Densidade
- **Carbon/Primer**: densidade alta é virtude (enterprise data-heavy)
- **Anthropic**: não menciona densidade — foca identidade
- **Reconciliação**: densidade é CONTEXTUAL à tarefa. Tabela de dados → densa; onboarding → espaçosa. Regra: "informação suficiente, interface limpa".

### Gradientes / cor
- **Impeccable**: proíbe gradientes roxo→azul (AI slop)
- **UI UX Pro Max**: recomenda paleta por indústria, sem gradiente default
- **Anthropic**: 1 cor de destaque com moderação
- **Reconciliação**: gradiente é OPINION — só quando o domínio pede; cor de destaque única é a prática segura.

### Cards
- **Impeccable**: anti card-soup, não aninhar cards
- **Radix/shadcn**: Card é primitivo legítimo
- **Reconciliação**: Card resolve agrupamento REAL. Anti-pattern é card sem conteúdo próprio ou aninhado sem razão.

### Processo de design
- **Anthropic**: brainstorm → plano → crítica → build → crítica (2 passes)
- **better-web-ui**: gerar 5 direções, comparar, aplicar 1
- **Impeccable**: commands (shape → craft → critique → polish)
- **Reconciliação**: todas concordam em **planejar antes de codar e criticar depois**. O Cosca adota: plano compacto (tokens+assinatura) → crítica anti-template → build → critique/audit.

### Skills vs Regras determinísticas
- **UI UX Pro Max / better-web-ui**: conhecimento como skill/instruções
- **Impeccable**: 59 regras determinísticas (CI-friendly, sem LLM)
- **Reconciliação**: as duas camadas se complementam — heurística (o Cosca raciocina) + regras verificáveis (o Cosca valida). Ambas entram no Cosca Design Intelligence.

## Contradições identificadas

1. **"Tudo acessível" vs "densidade enterprise"** — equilibrar: densidade com alvos de toque ≥ 24px e contraste OK.
2. **"Identidade distinta" vs "consistência de sistema"** — a identidade vive no FOUNDATION (cor/tipo/assinatura); a consistência vive nos PATTERNS (componentes). São camadas diferentes, não rivais.
3. **"Boldness" vs "previsibilidade enterprise"** — ousadia no elemento-assinatura; previsibilidade nos fluxos.

## Melhores práticas combinadas (síntese do Cosca)

1. **FOUNDATION com identidade**: tokens semânticos + 1 cor de destaque + tipografia com personalidade (display/body/utility) + spacing/radius/sombra/motion consistente.
2. **PRIMITIVES acessíveis**: componentes com Radix-level a11y (keyboard, focus, aria), composição via composição (shadcn philosophy), API de tokens.
3. **PATTERNS de produto**: search/filtros/tabelas/forms/dashboards/CRUD/detail/timeline/nav — padronizados e reutilizáveis.
4. **EXPERIENCE completa**: loading (skeleton), empty (convite), error (recuperação), success (confirmação).
5. **VALIDAÇÃO em 2 camadas**: heurística (crítica de design) + determinística (checklist/audit).
