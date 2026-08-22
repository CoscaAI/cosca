# CHECKLIST — Validação de Interface (critérios de aceite)

> Rodar ANTES de considerar qualquer UI "pronta". Cada item é critério de aceite.
> Combinado de: Impeccable (detector), Vercel guidelines, UI UX Pro Max, Anthropic, Radix/React Aria.

## A. Acessibilidade
- [ ] Focus visível em tudo (navegação por teclado completa)
- [ ] Contraste texto ≥ 4.5:1 (WCAG AA) — on-color escuro sobre cor saturada
- [ ] `prefers-reduced-motion` respeitado (animações desligadas)
- [ ] Semântica correta (button/link/role/dialog/aria-label)
- [ ] Ícones com aria-label quando ação sem texto
- [ ] Alvos de toque ≥ 24px (ideal 32px+)

## B. Estados
- [ ] Loading: skeleton ou contexto (nunca só spinner central)
- [ ] Empty: explica + convida à ação
- [ ] Error: o que aconteceu + como corrigir + retry
- [ ] Success: confirmação imediata (toast/inline)
- [ ] Partial loading: não bloquear página inteira por seção

## C. Interação
- [ ] cursor-pointer em todo elemento clicável
- [ ] Hover com transição 150–300ms
- [ ] Ações destrutivas com confirmação (o que será perdido)
- [ ] Feedback em toda ação (sucesso/erro)
- [ ] Teclado: Esc fecha dialog/drawer, Tab percorre, Enter ativa

## D. Consistência
- [ ] Mesma ação = mesmo padrão em toda a app (editar: drawer OU modal, nunca os dois sem razão)
- [ ] Status sempre com badge no mesmo tom (verde=ok, vermelho=erro, âmbar=aguardando)
- [ ] Espaçamento/radius/tipografia vindos de tokens (nunca arbitrários)
- [ ] Cada tela tem 1 ação primária clara

## E. Texto & conteúdo
- [ ] Sem emojis como ícones
- [ ] Voz ativa, sentence case, sem "clique aqui"/"submit"
- [ ] Botão e resultado com mesmo nome ("Publicar" → "Publicado")
- [ ] Texto reflow sem clip (chips/badges/labels) — wrap ou +n operável
- [ ] Cópia de empty/error escrita do lado do usuário

## F. Responsividade
- [ ] 375 / 768 / 1024 / 1440 verificados
- [ ] Tabela: overflow-x ou cards em mobile
- [ ] Sidebar → drawer, filtros → sheet em telas pequenas
- [ ] Ações secundárias para menus em telas pequenas

## G. Anti-AI-slop
- [ ] Nenhum emoji como ícone
- [ ] Sem gradiente roxo→azul default
- [ ] Sem Inter/arial como única tipografia
- [ ] Sem cards aninhados sem razão
- [ ] Sem números decorativos (01/02/03) sem significado
- [ ] Sem easing bounce
- [ ] Não tem cara de "template SaaS genérico"

## H. Performance (Vercel)
- [ ] Sem waterfull de fetch
- [ ] Bundle sem dependências desnecessárias
- [ ] Listas longas virtualizadas (quando aplicável)
- [ ] Imagens com dimensões + lazy

## I. Validação pós-mudança
- [ ] typecheck + build + testes verdes
- [ ] Inspeção visual comparada com páginas vizinhas
- [ ] Anti-patterns detectáveis corrigidos (grep emojis, contraste, states)

---
**Nota**: se algo abaixo de 8/10 em Visual · UX · Technical UX · Product perception, continue refinando.
