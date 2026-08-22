# DEVELOPMENT DOCTRINE — Os 25 Mandamentos de Desenvolvimento

> **Fonte**: doutrina da casa (2026-08-14) — as regras de quem constrói do zero.
> **Status**: doutrina da família.
> **Aplicação**: todo agente Cosca, em toda mudança de UI/arquitetura.
>
> Em sintonia com a P14 (Change Safety Level — OBSERVE → READ-ONLY → PREVIEW →
> APPROVAL → WRITE → VERIFY → COMMIT).

---

## A REGRA PRINCIPAL (gravada no prompt de todo agente)

> **"Não seja apenas um implementador. Atue como arquiteto, engenheiro, QA,
> especialista em UX, segurança e performance. Antes de modificar o sistema,
> avalie impacto, risco, dependências e regressões. Quando existir uma solução
> mais segura que preserve o backend e permita validar a ideia primeiro,
> proponha-a."**
>
> **"Se detectar que o pedido pode causar regressão, pare antes de executar,
> explique o risco e proponha uma estratégia segura. Não espere o usuário
> descobrir o problema depois."**

---

## OS 25 MANDAMENTOS

### Processo mental (1-3)
1. **Entender antes de codar**: Entender → Inspecionar → Projetar → Decidir →
   Implementar → Testar → Medir → Refatorar. Nunca: pedido → código imediato.
2. **Sistema de componentes, não páginas isoladas**: Layout Engine, Panel
   System, Docking, Tabs, Commands, Overlays, Notifications, Editors, AI
   Components, Data Viz — uma feature nova reutiliza essas primitives.
3. **Separar visual de estado**: UI → View Model → Domain State → Backend
   Adapter → Backend. Nunca misturar UI com lógica de negócio.

### Arquitetura (4-7)
4. **Contratos antes de integrações**: PanelManager, LayoutManager,
   AgentManager, ModelManager, WorkspaceManager, GitManager, ContextManager —
   primeiro o contrato, depois a implementação.
5. **O Layout Engine é quase um produto separado**: Layout Tree, Docking,
   Split, Tabs, Floating, Resize, Persistence, Serialization, Undo/Redo,
   Validation — pode virar tecnologia central do Cosca.
6. **Tudo importante tem Undo/Redo**: não só edição de código — mover painel,
   fechar painel, alterar layout, mudar workspace, reorganizar tabs,
   modificar configuração visual.
7. **Layout persistente**: organizou → fechou → reiniciou → restaurou. Nada de
   perder a interface ao reiniciar.

### Qualidade (8-12)
8. **Testar comportamento, não só componentes**: drag, resize, split, merge,
   close, restore, restart, múltiplos painéis, splits aninhados, tamanhos
   extremos — e principalmente SEQUÊNCIAS.
9. **Bugs viram testes**: Bug → Reprodução → Teste → Fix → Regressão protegida.
   O mesmo bug não volta.
10. **Modo Developer no próprio Cosca**: Panel Tree, Layout State, Event Bus,
    Active Commands, DOM state, Memory, FPS, Render time, Network, AI requests.
11. **Performance desde o início**: startup, FPS, memória, CPU, GPU, repositórios
    grandes, arquivos grandes, muitas tabs/painéis/agentes. 100+ painéis não
    podem destruir a interface.
12. **IA não é um "Deus"**: Agent → Tools → Permissions → Policy → Execution.
    Nunca: agente → faz qualquer coisa.

### Segurança e rastreabilidade (13-14)
13. **Read → Analyze → Plan → Preview → Approval → Execute → Verify**: nem toda
    tarefa passa por todas, mas operações de alto risco DEVEM passar.
14. **Tudo que a IA faz é rastreável**: quem? qual agente? qual modelo? qual
    tarefa? qual arquivo? qual ferramenta? qual mudança? qual aprovação? qual
    resultado? — ouro para debugging.

### Dependências e design (15-16)
15. **Não adicionar dependência só porque resolve rápido**: já existe
    internamente? posso implementar? vale o custo? licença? segurança?
    performance? manutenção? — principalmente num projeto com identidade própria.
16. **Design Tokens antes de polir telas**: colors, spacing, radius, typography,
    elevation, motion, density, icons — os componentes usam os tokens, não
    estilos soltos.

### UX (17-18)
17. **Keyboard-first**: Ctrl+P, Ctrl+Shift+P, Ctrl+K, Ctrl+Tab, Ctrl+W,
    Ctrl+Shift+F, Ctrl+`, Ctrl+B, Ctrl+J — e permitir remapeamento.
18. **Mouse-first também excelente**: cursor correto, preview, drop zones
    claras, animação, snapping, cancelamento, rollback. Nada de painel
    "teleportando".

### Modelo de dados (19-21)
19. **Nunca confiar cegamente em coordenadas**: ❌ x=432 y=120. Preferir
    Workspace → Split → Group → Panel — layout SEMÂNTICO.
20. **Pensar em árvore + eventos**: panel.moved, panel.closed, panel.docked,
    panel.undocked, layout.changed, workspace.changed — arquitetura extensível.
21. **IA usa as MESMAS APIs que o usuário**: usuário → Command API → Layout
    Engine; agente → Command API → Layout Engine. Os dois usam o mesmo sistema.

### Escala e manutenção (22-24)
22. **Chaos Mode** (depois): teste automático que faz open/close/drag/resize/
    split/merge/switch/restart/restore aleatoriamente durante milhares de
    operações — se o layout sobreviver, aprovado.
23. **Cosca Doctor**: `cosca doctor` verifica Layout Engine, Workspace,
    Extensions, AI Runtime, Providers, Agents, Git, Terminal, Storage,
    Permissions, Configuration, Cache, Database.
24. **ADRs (Decision Records)**: ADR-001 Layout Engine, ADR-002 Panel
    Architecture, ADR-003 Agent Runtime, ADR-004 AI Provider System,
    ADR-005 Permission Model, ADR-006 Workspace Persistence — daqui a um ano
    isso salva.

### Definição de pronto (25)
25. **"Funcionou" não significa "terminou"**: uma feature termina quando tem
    Implementation + Tests + Error handling + Performance + Security +
    Accessibility + UX review + Documentation + Regression test.

---

## O QUE JÁ FOI APLICADO (2026-08-14)

| Mandamento | Onde |
|-----------|------|
| 13 (Read→Preview→Approve) | P14 na Constituição + execGuard no cosca-code (L307) |
| 5 (Layout Engine independente) | Dockview no workbench (L301) + react-grid-layout no trader (L297) |
| 7 (Layout persistente) | localStorage versionado (L299) |
| 18 (Mouse-first, drop zones) | Dockview com drop zones (L301) |
| 21 (IA usa as mesmas APIs) | Command Center roteando pelo backend real (L304) |
| 14 (Rastreabilidade) | Provenance + chain assinada (toda a casa) |
| 15 (Dependência só se vale) | Dockview escolhido após avaliar — não reinventar (L301) |
| 16 (Design Tokens) | CSS vars no trader (L295) e cosca-code |

---

> **Lembrete da casa**: estas regras não são sugestões. São a doutrina do que
> separa "gerar código rápido" de "construir uma plataforma".
