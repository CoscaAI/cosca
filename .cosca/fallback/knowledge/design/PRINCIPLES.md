# PRINCIPLES — Design Principles Classified

> Regras extraídas dos sistemas estudados, **classificadas por alcance e confiança**.
> Confiança: **UNIVERSAL** (≥3 sistemas independentes concordam) · **STRONG** (boa evidência) · **CONTEXTUAL** (depende do produto) · **OPINION** (preferência) · **EXPERIMENTAL**.

---

## 1. UX UNIVERSAL — aplica a qualquer produto

### Feedback imediato e claro · UNIVERSAL
Toda ação tem resposta visível. Sucesso confirma, erro explica. "Publish" produz toast "Published" (Anthropic/Impeccable/Polaris concordam).

### Previsibilidade · UNIVERSAL
Mesma ação → mesmo resultado. Padrão aprendido em uma tela vale em todas (consistência = aprendizado).

### Hierarquia visual · UNIVERSAL
Uma coisa é a mais importante em cada tela. Se tudo grita, nada fala. (Anthropic: "spend your boldness in one place").

### Prevenção e recuperação de erros · UNIVERSAL
Erro explica *o que aconteceu e como corrigir*, na voz da interface, sem desculpas e sem vaga. "Não conseguimos carregar estes dados. Tente novamente." (Vercel/Impeccable/Anthropic).

### Acessibilidade é base, não extra · UNIVERSAL
Focus visível, teclado, contraste ≥4.5:1, `prefers-reduced-motion`, semântica (Radix/React Aria/Vercel/Carbon/Impeccable — todos).

### Estados são momentos de direção · UNIVERSAL
Loading (skeleton), Empty (convite à ação), Error (recuperação), Success (confirmação). Nenhum pode ser "tela branca + spinner".

### Cópia é material de design · UNIVERSAL
"Words appear for one reason: to make it easier to understand." Voz ativa, sentence case, nomear pelo que o usuário controla ("notificações", não "webhook config").

### Cada elemento faz UMA coisa · UNIVERSAL
"Let each element do exactly one job. A label labels, an example demonstrates."

---

## 2. CONTEXTUAL — depende do produto

### Densidade de dados · CONTEXTUAL (enterprise/dashboard)
Primer/Carbon/IBM: alta densidade com escaneabilidade. Dashboards de operação priorizam leitura rápida. Consumer app: densidade baixa. **Decisão: qual a frequência e o contexto de uso?**

### Tabelas: scrolling > densidade > filtros > bulk actions · CONTEXTUAL (enterprise)
Carbon/Primer: tabela precisa de scan rápido, filtros, seleção, ações em massa. O fluxo é: `Tabela → Densidade → Escaneamento → Filtragem → Ações → Produtividade`.

### Navegação reflete arquitetura de informação · CONTEXTUAL
Sidebar ≠ default. Perguntar: "Qual a IA e frequência de navegação?" (Anthropic questiona templates). Sidebar para nav frequente/multi-módulo; tabs para seções; top nav para poucas seções.

### Dark mode · CONTEXTUAL
"Existe necessidade real de contraste/ambiente?" (UI UX Pro Max: Dark OLED para ferramentas de operação; light para wellness/lifestyle). Não é default universal.

### Gráficos precisam de dados reais · CONTEXTUAL
"decorative charts" é anti-pattern (Impeccable). Só usar chart se a informação é temporal/comparativa e o dado existe.

### Cores semânticas por domínio · CONTEXTUAL
Operações/manutenção: verde=operacional, vermelho=incidente, âmbar=aguardando (UI UX Pro Max). Fintech: confiança/estabilidade. Wellness: calma.

---

## 3. ESTÉTICO — preferências, não regras

Não virar regra universal:

- Glassmorphism / Claymorphism / Brutalism / Neumorphism / Bento
- Gradientes (especialmente roxo→azul — AI slop)
- Cantos arredondados vs. retos
- Sombras
- Dark vs. Light como *preferência* (é contextual quando justificado)

**Regra**: preferência estética só se aplica se responde a "para esse produto/usuário, essa escolha comunica a identidade certa?"

---

## 4. CONFIANÇA — como classificar

| Se... | Então |
|---|---|
| 3+ sistemas independentes concordam | UNIVERSAL — tratar como lei |
| 1–2 sistemas recomendam | STRONG/CONTEXTUAL — justificar no domínio |
| Um sistema só, sobre estética | OPINION — nunca regra |
| Contradiz prática estabelecida | EXPERIMENTAL — exigir prova de valor |

**Exemplos de convergência (universal por consenso):**
- Foco visível + teclado + contraste (Radix + React Aria + Vercel + Carbon + Impeccable)
- Feedback de sucesso/erro (Anthropic + Impeccable + Polaris)
- Empty state como convite à ação (better-web-ui + Vercel + Anthropic)
- Anti-emojis-como-ícones (Impeccable + UI UX Pro Max + Anthropic)
- Restrição: 1 elemento de destaque (Anthropic + Impeccable + Refactoring UI)
