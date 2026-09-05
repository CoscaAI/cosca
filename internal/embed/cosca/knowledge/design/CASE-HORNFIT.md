# CASE-HORNFIT — Análise e Direção de Design

> Aplicação do Cosca Design Intelligence ao HornFit (assistência técnica de equipamentos de academia).
> Estado atual → problemas → oportunidades → prioridade → direção visual.

## 1. Estado atual

- **Stack**: Next.js 15 + Tailwind 3.4 + design tokens semânticos (dark) + 14 componentes base + DataTable/Drawer/Modal/Toast
- **Estratégia de cor**: operacional (verde primário + vermelho p/ incidente + âmbar p/ manutenção) — já aplicada
- **Tipografia**: Fira Sans (UI) + Fira Code (dados)
- **Shell**: sidebar colapsável agrupada + header (breadcrumb, notificações, perfil)
- **Telas excelentes**: Dashboard (KPIs+charts), Clientes, OS (drawer+timeline), Login, Portal

## 2. Problemas de UX (auditoria)

| Problema | Impacto | Severidade |
|---|---|---|
| CRUDs restantes (contratos/estoque/faturamento/crm) com tabelas cruas, sem menus contextuais consistentes | Consistência | Alta |
| Detalhe de entidades (cliente/contrato) não tem página própria — só lista + modal de edição | Fricção | Média |
| Filtros avançados (data, múltiplos critérios) ausentes nas listas grandes | Produtividade | Média |
| Ações em massa (seleção múltipla, bulk) ausentes | Enterprise | Média |
| Empty/error states inconsistentes em páginas secundárias | Confiança | Média |
| Notificações existem mas não navegam para o recurso | Fricção | Baixa |

## 3. Problemas visuais

- Tabelas secundárias ainda com aparência "genérica" (mesmo após tokens)
- Detalhe de OS é drawer, mas detalhe de outros módulos não existe
- Dashboard tem 2 charts, mas "atividade recente" é simples
- Página de relatórios é lista de seções, não uma narrativa de dados

## 4. Problemas de arquitetura (frontend)

- Componentes de página duplicam lógica de CRUD (load/submit/delete repetidos em cada página)
- Sem hook genérico `useCrud` / `useList` — cada página reinventa
- Sem roteamento de detalhe (todas as entidades são list+modal)
- Status labels/badges duplicados por página (parcialmente centralizado em `statusTone`)

## 5. Oportunidades de design

1. **Camada CRUD genérica**: hook `useList`/`useCrud` + PageHeader + DataTable + DrawerForm — uma implementação, N páginas
2. **Detalhe de entidade** (drawer rico) para cliente, contrato, fatura — com timeline/atividades
3. **Filtros avançados** (barra de filtros reutilizável) nas listas grandes
4. **Ações em massa** nas tabelas (seleção + bulk)
5. **Relatórios como narrativa**: KPIs → charts → insights → exportação
6. **Notificações navegáveis** (click → recurso)
7. **Empty/error consistentes** via DataTable (já parcial)

## 6. Prioridade

| # | Item | Impacto | Esforço |
|---|---|---|---|
| 1 | Hook CRUD genérico + propagar padrão às páginas restantes | Alto | Médio |
| 2 | Drawer de detalhe (cliente/contrato/fatura) | Alto | Médio |
| 3 | Barra de filtros avançados nas listas grandes | Alto | Médio |
| 4 | Ações em massa (seleção + bulk) | Médio | Médio |
| 5 | Relatórios como narrativa de dados | Médio | Alto |
| 6 | Notificações navegáveis | Baixo | Baixo |

## 7. Direção visual proposta (identidade própria)

### Personalidade
**"Oficina técnica de precisão"** — operacional, confiável, orientado a dados, sem ruído. O HornFit não é uma startup de SaaS qualquer; é a ferramenta de trabalho de quem mantém academias funcionando. A UI deve parecer um instrumento de precisão, não um brinquedo.

### Visual language
- **Fundo**: azul-meia-noite profundo (`#0F172A`) — foco, contraste, ambientes de operação
- **Superfícies**: 3 níveis (surface / raised / hover) com bordas sutis, sombras mínimas
- **Cor**: verde operacional (`#16A34A`) como única cor de ação primária; vermelho apenas incidente; âmbar manutenção; azul informação — **cor que significa algo, não decoração**
- **Destaque (assinatura)**: o "indicador operacional" — um dot verde pulsante sutil nos estados vivos (OS em andamento, chamados abertos), lembrando telemetria/monitoramento (o mundo do cliente: máquinas com luzes de status)

### Densidade
- Alta nas tabelas/operação (scan rápido), média nos forms — "informação suficiente, interface limpa"

### Tipografia
- **Fira Sans** (UI, precisa e técnica) + **Fira Code** para números/dados/OS — reforça o mood de instrumento

### Filosofia de componentes
- Primitivos acessíveis (Radix-level), composição > props booleanas, tudo derivado de tokens
- Uma implementação por padrão (DataTable, DrawerForm) — nunca cinco versões

### Filosofia de motion
- Motion comunica ESTADO (transição de status, nova mensagem, carregamento), 150–300ms, reduced-motion respeitado. Nada de efeito por efeito.

### Navegação
- Sidebar agrupada por fluxo (Operações/Comercial/Atendimento/Gestão) — reflete a IA; breadcrumb em toda página; detalhe em drawer para manter contexto

### Dashboard
- Narrativa: "como está a operação" (KPIs com contexto) → "o que precisa de atenção" (inadimplência, estoque baixo) → "o que está acontecendo" (atividade recente) → ação

## 8. Recomendação de execução (roadmap)

1. **Camada CRUD genérica** (`useList` + `useCrud` + `DrawerForm`) → refatorar clientes/OS como referência
2. **Propagar** aos CRUDs restantes (contratos/estoque/faturamento/crm)
3. **Detalhe em drawer** para cliente/contrato/fatura (com timeline)
4. **Filtros avançados** + **ações em massa**
5. **Relatórios-narrativa**
6. **Polimento final** (CHECKLIST completo) + validação visual

## 9. Critérios de validação

- CHECKLIST.md (design/CHECKLIST.md) 100% verificado
- Contraste ≥4.5:1, teclado, reduced-motion
- Sem emojis, sem card soup, sem gradiente roxo→azul
- Consistência: mesma ação = mesmo padrão
- Nota ≥8/10 em Visual · UX · Technical UX · Product perception
