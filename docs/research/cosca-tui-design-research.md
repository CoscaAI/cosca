# COSCA TUI DESIGN RESEARCH

> 30 repositórios estudados. Padrões extraídos, não copiados.
> Aplicado ao Cosca Terminal — 2026-08-10

---

## PRIORIDADE 1 — Estudo concluído

### Crush (Karya) — AI Agent TUI
**Categoria:** AI Coding Agent
**Padrões extraídos:**
- Single-process TUI com cell-based rendering
- Unified leader key (Ctrl+Space) com which-key popup
- Block-based execution display (não raw streaming)
- Human-in-the-loop gates (Plan → Diff → Verify)
- Agent-agnostic task memory (troca de agente sem perder estado)
- Layered agent instructions (global → project → task)
- Permission gradient UI (cores por nível de risco)

**Aplicado ao Cosca:** ✅
- Command palette (`:`) K9s-style
- Risk-colored tool calls (read=cyan, write=gold, exec=orange, destructive=red)
- Phase progress bar no HUD
- Tab architecture substituindo toggle booleans

**O que NÃO copiar:**
- Custom TUI stack (Cosca já usa bubbletea)
- Neovim embedded (deferido para web console)
- Crush JSON config (Cosca tem sistema próprio)

### Lazygit — UX de terminal complexo
**Categoria:** Git TUI
**Padrões extraídos:**
- Tab bar com painéis fixos (não toggle)
- Single-letter mnemonics context-sensitive
- `?` keybindings overlay filtrável
- `+`/`-` fullscreen toggle
- Split layout (side panel + main content)
- `Tab`/`[`/`]` navegação entre painéis

**Aplicado ao Cosca:** ✅
- PanelID enum (Chat, Tasks, Agents, Files, System)
- Tab/Shift+Tab ciclagem
- Teclas numéricas 1-5 para painéis
- Breadcrumbs de navegação

### K9s — Dashboard operacional
**Categoria:** Kubernetes TUI
**Padrões extraídos:**
- `:` command palette (recurso + namespace + filtro)
- Pulses view (health dashboard colorido)
- Breadcrumbs hierárquicos
- `/` filtro com regex + inverso + labels
- Hotkeys configuráveis (YAML)
- Column-based sorting (Shift-N, Shift-A, Shift-S)

**Aplicado ao Cosca:** ✅
- `:agent`, `:model`, `:health`, `:cost`, `:status`, `:quit`
- Breadcrumbs: `Chat > Tasks > task-1 > Logs`

**Pendente:**
- System health pulses dashboard
- Hotkeys configuráveis (hotkeys.yaml)
- Filtro `/` nos painéis

### Yazi — File/Task UX
**Categoria:** File Manager
**Padrões extraídos:**
- Preview async com prioridade
- Navegação rápida
- Task management com progresso, cancelamento, prioridade
- Plugins

### Bubble Tea + Bubbles + Lip Gloss
**Categoria:** Framework TUI Go
**Padrões extraídos:**
- Multi-model composition (cada painel = tea.Model)
- `bubbles/key` + `bubbles/help` (atalhos tipados)
- `bubbles/table` com sorting, filtering, selection
- `bubbles/list` com custom delegate
- `lipgloss.JoinHorizontal` + `Width`/`Height` para layouts responsivos
- Mouse support (`tea.WithMouseCellMotion`)
- Viewport high-performance mode
- Design system com Theme struct

**Aplicado ao Cosca:** ✅
- `theme/theme.go` — TokyoNight Theme struct com métodos Panel(), Header(), TabBar(), StatusBar(), Selected(), RiskLevel()
- Estilos deduplicados entre `ui/` e `terminal/`

**Pendente:**
- `bubbles/key` + `bubbles/help` para substituir strings hardcoded
- `bubbles/table` para Tasks/Files
- Mouse support

---

## ARQUITETURA ATUAL DO COSCA TERMINAL

```
internal/chat/ui/
├── theme/
│   └── theme.go              ✅ NOVO — Theme struct + TokyoNight
├── terminal/
│   ├── model.go              ✅ Tab architecture, colon-prompt, breadcrumbs, risk colors
│   ├── styles.go             ✅ Refatorado via theme
│   ├── hud.go                ✅ Phase progress bar, ActiveTask, cost burn rate
│   ├── tasks.go              ✅ TaskPanelView
│   ├── commands.go           ✅ Colon commands + slash commands
│   ├── files.go              ✅ FilesPanelView
│   └── diff.go               ✅ DiffPanelView
└── model.go                  (chat TUI — a unificar)
```

---

## MELHORIAS APLICADAS (09/08 → 10/08)

| # | Melhoria | Inspiração | Status |
|---|----------|-----------|--------|
| 1 | Theme package unificado | Lip Gloss design system | ✅ |
| 2 | Tab architecture (PanelID) | Lazygit tab bar | ✅ |
| 3 | Command palette (`:`) | K9s colon prompt | ✅ |
| 4 | Risk-colored tool calls | Claude Code | ✅ |
| 5 | Phase progress bar | Warp blocks | ✅ |
| 6 | Breadcrumbs | K9s breadcrumbs | ✅ |
| 7 | Enhanced HUD (burn rate) | Aider model HUD | ✅ |
| 8 | Models compostos (tea.Model) | Bubble Tea Elm Architecture | 📋 |
| 9 | bubbles/table para Tasks | Bubbles table | 📋 |
| 10 | bubbles/key + help overlay | Bubbles key/help | 📋 |
| 11 | Mouse support | Bubble Tea mouse | 📋 |
| 12 | System health pulses | K9s pulses | 📋 |
| 13 | Hotkeys configuráveis | K9s hotkeys.yaml | 📋 |
| 14 | Filtro `/` nos painéis | Lazygit/K9s filter | 📋 |

---

## PRIORIDADE 2 — Próximos estudos

Ratatui, Textual, Zellij, Lazydocker, btop, bottom, GitUI, Glow, Glamour

## PRIORIDADE 3 — Estudos futuros

Aider, OpenHands, Goose, OpenDev, Atuin, Navi, Helix, Procs, Tmux, Wish, Awesome TUIs
