# COSCA DESIGN INTELLIGENCE

> **Stack 01 da Cosca Engineering Intelligence Matrix** (`../README.md`).
> **Padrão oficial de design do Cosca.** Capacidade de design reutilizável — não um gerador de componentes, mas um sistema que entende *porquê*.
>
> **Ordem de consulta**: este README → `PRINCIPLES.md` → `REASONING.md` → `ANTI_PATTERNS.md` → `SYNTHESIS.md` → `CHECKLIST.md`.

## O que é

O Cosca Design Intelligence é a camada de conhecimento que transforma pesquisa de UI/UX (estudada em sistemas líderes) em **capacidade de decisão de design**. O Cosca não deve saber "qual cor usar" — deve saber **"por que essa cor, nesse contexto, para esse usuário, nessa interação, nesse estado?"**.

## Fontes de estudo (CORE)

| Fonte | O que resolve | Lição central |
|---|---|---|
| **UI UX Pro Max** | Design system data-driven por indústria | 192 regras por domínio; gerar design system via raciocínio (não gosto pessoal) |
| **Anthropic frontend-design** | UI genérica de IA | "Ground it in the subject" — identidade vem do mundo do produto; 1 elemento-assinatura; restrição ("tire um acessório") |
| **Vercel web-design-guidelines** | UI funcional → production-grade | 100+ regras verificáveis (a11y, foco, forms, motion, i18n, theming) |
| **Impeccable** | Detectar/elevar UI existente | 59 regras determinísticas anti-AI-slop + 23 comandos (audit/critique/polish/distill/bolder/quieter) |
| **better-web-ui** | Dar "gosto" ao agente | 25 skills; doutrina compartilhada; "taste, not decoration"; gerar 5 direções antes de aplicar 1 |
| **Radix / shadcn / MUI / Chakra / React Aria** | Arquitetura de componentes | Acessibilidade é primitivo; composição; tokens; keyboard/focus management |
| **Polaris / Primer / Spectrum / Carbon** | Enterprise UX | Consistência de sistema; densidade; workflows complexos; acessibilidade |
| **Motion (Framer)** | Motion design | Animação comunica mudança de estado — senão é ruído; reduced-motion |

## Como usar (protocolo do Cosca)

1. **ANTES de desenhar**: consultar `PRINCIPLES.md` (classificação universal/contextual) + `REASONING.md` (modelo decisão).
2. **Durante**: checar `ANTI_PATTERNS.md` contra a solução.
3. **Depois**: validar com `CHECKLIST.md` (cada item é critério de aceite).

## Modelo mental central

```
USUÁRIO → OBJETIVO → CONTEXTO → INFORMAÇÃO → AÇÃO → FEEDBACK → ESTADO → PRÓXIMA AÇÃO
```

O design é **consequência do fluxo**, nunca o contrário.

## Arquitetura do conhecimento

```
knowledge/design/
├── README.md        ← você está aqui
├── PRINCIPLES.md    ← princípios (universal / contextual / estético) + confiança
├── REASONING.md     ← design reasoning + decision engine (porquê primeiro)
├── ANTI_PATTERNS.md ← banco de anti-patterns (detectar + corrigir)
├── SYNTHESIS.md     ← comparação dos sistemas + contradições + reconciliação
├── CHECKLIST.md     ← critérios de validação pré-entrega
└── CASE-HORNFIT.md  ← estudo de caso aplicado (HornFit)
```
