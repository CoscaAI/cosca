# ADR-008: Cosca Code — Workbench Dinâmico com Docking (Layout Engine)

> **Status:** Accepted ✅ | **Owner:** Cosca Kernel | **Last Updated:** 2026-08-14
> **Implementation:** Fase 1 concluída (L301) — docking real no cosca-code
> **Fonte:** visão do professor (2026-08-14) — mandamento 5 da doutrina

## Context

O cosca-code tinha um workbench FIXO no padrão VS Code (ActivityBar → sidebar →
editor → terminal). O professor propôs (conversa 2026-08-14): transformar em um
**Cosca Dynamic Workbench** onde qualquer painel pode ir para qualquer lugar
(esquerda/direita/cima/baixo/tab), com floating, layouts salvos e, no futuro,
a IA montando o workspace.

### Decisão

**Adotar o Dockview (dockview + dockview-react) como motor de docking** — não
reinventar o layout engine. O workbench vira um layout em ÁRVORE (split → group
→ panel), com:

- Drag & drop com drop zones inteligentes (top/center/left/right/bottom)
- Tabs, grupos, splits aninhados
- Floating panels e docking de volta (Fase 2)
- Serialização do layout (base para Fase 3: layouts salvos)
- Persistência em localStorage (versionada)

### Por quê Dockview e não construir

1. **Mandamento 15 da doutrina**: dependência só se vale o custo — o Dockview
   entrega árvore + drop zones + floating + serialização PRONTOS e testados.
2. O valor do Cosca está nas fases 2-4 (IA compõe o layout), não no engine de
   drag.
3. Mandamento 5: o Layout Engine é tratado como componente central, mas
   REUTILIZANDO uma base madura.

### Consequências

- Painéis do cosca-code (Explorer, Editor, Terminal, AI, Git, Problems,
  Trust, Control, Map) viraram views registradas no dock.
- O layout persiste entre sessões (chave versionada — invalida layouts antigos).
- Base pronta para Fase 2 (floating), Fase 3 (presets), Fase 4 (IA monta).

### Alternativas rejeitadas

- **Coordenadas fixas (x/y)**: rejeitada — mandamento 19 (layout semântico,
  não coordenadas).
- **Layout engine próprio do zero**: rejeitado — mandamento 15 (custo alto,
  sem valor diferencial).
