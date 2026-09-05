# ANTI-PATTERNS — Database

> Coleção de coisas que o Cosca deve **evitar**. Para cada: problema, por que acontece, como detectar, como corrigir, quando pode ser aceitável.

## AI SLOP (assinaturas de interface gerada por IA)

### A1. Emojis como ícones
- **Problema**: inconsistente, não-escalável, quebra em contexto profissional
- **Por que acontece**: atalho de agente sem biblioteca de ícones
- **Detectar**: grep por emoji em componentes
- **Corrigir**: biblioteca SVG consistente (Lucide/Heroicons/Phosphor)
- **Aceitável**: comunicações informais fora da UI (nunca em produto)

### A2. Inter em tudo + gradiente roxo→azul
- **Problema**: assinatura imediata de "SaaS genérico de IA"
- **Corrigir**: fonte com personalidade própria ao domínio; 1 cor de destaque, não gradiente
- **Aceitável**: nunca como default; só se o domínio justifica

### A3. Cards dentro de cards (card soup)
- **Problema**: sem hierarquia real, tudo vira caixa
- **Por que acontece**: "card" como resposta padrão a qualquer agrupamento
- **Corrigir**: hierarquia real — borda/separador/espaçamento em vez de caixa aninhada
- **Aceitável**: agrupamento real de entidade (ex.: card de contrato com conteúdo próprio)

### A4. Cinza puro / preto puro sem tint
- **Problema**: plano, sem profundidade
- **Corrigir**: sempre tintar (preto-azulado, cinza-azulado) — Impeccable

### A5. Texto cinza-claro sobre fundo colorido
- **Problema**: contraste quebrado
- **Corrigir**: on-color escuro quando o fundo é claro; nunca cinza sobre cor saturada

### A6. Números decorativos (01/02/03) sem significado
- **Problema**: estrutura falsa (Anthropic)
- **Corrigir**: numeração só se o conteúdo É uma sequência real (processo/timeline)
- **Aceitável**: quando a ordem carrega informação

### A7. Easing bounce/elástico
- **Problema**: parece datado (Impeccable)
- **Corrigir**: easing suave (cubic-bezier 0.16,1,0.3,1), 150–300ms

### A8. Cards/tiles de ícone arredondado acima de todo heading
- **Problema**: tell clássico de AI slop (Impeccable)
- **Corrigir**: variar o tratamento; ícone só quando comunica

---

## UX (geral)

### U1. Dashboard = grade de cards sem narrativa
- **Problema**: "Card Card Card Tabela Tabela" sem hierarquia
- **Corrigir**: responder "como está o negócio / o que precisa de atenção / o que mudou / o que fazer agora"; KPIs com contexto + atenção + ação

### U2. Gráficos decorativos
- **Problema**: chart sem dado temporal/comparativo real
- **Corrigir**: só usar se a informação justifica; senão, número + tendência textual
- **Aceitável**: mock/placeholder explícito

### U3. Estados escondidos (sem loading/empty/error)
- **Problema**: usuário não sabe o que aconteceu
- **Corrigir**: skeleton no loading, convite no empty, recuperação no error

### U4. Modal abuse (tudo em modal)
- **Problema**: contexto perdido, fluxos quebrados
- **Corrigir**: modal para confirmação/edição curta; drawer para contexto lateral; página para entidades complexas

### U5. Ações destrutivas sem confirmação
- **Problema**: perda irreversível
- **Corrigir**: dialog de confirmação declarando o que será perdido

### U6. Ícone sozinho ambíguo
- **Problema**: sem tooltip/label, ação incompreensível
- **Corrigir**: tooltip + aria-label; ou texto

### U7. Formulário gigante
- **Problema**: carga cognitiva alta, desistência
- **Corrigir**: sections/steps/accordion/progressive disclosure

### U8. Mobile como reflexão
- **Problema**: "encolher o desktop"
- **Corrigir**: 375/768/1024/1440; tabela→cards, sidebar→drawer, filtros→sheet

### U9. Animações sem propósito
- **Problema**: ruído, desconforto (Motion)
- **Corrigir**: motion comunica mudança de estado; senão, remove. reduced-motion sempre.

### U10. Link/botão sem cursor-pointer e focus
- **Problema**: não parece clicável; inacessível por teclado
- **Corrigir**: cursor-pointer global, focus-visible

### U11. Texto com reflow quebrado (chips/badges/labels truncados sem caminho)
- **Problema**: conteúdo inacessível (Vercel/UI UX Pro Max)
- **Corrigir**: wrap ou "+n" operável; truncamento com caminho acessível

### U12. Tabelas sem overflow-x em mobile
- **Problema**: quebra layout
- **Corrigir**: wrapper overflow-x-auto ou cards

---

## Como detectar (rápido)

1. `grep` por emojis, fontes padrão (Inter/arial), gradientes roxo→azul
2. Contraste: rodar check (WCAG 4.5:1)
3. `prefers-reduced-motion` presente?
4. Cada tela tem loading/empty/error?
5. Ações destrutivas confirmam?
6. Tudo que é clicável tem cursor-pointer + focus?
