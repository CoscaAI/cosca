# REASONING — Design Reasoning & Decision Engine

> O Cosca deve **justificar decisões**, não executar templates. Toda escolha responde a uma pergunta.

## 1. Tradução de templates → razão

| Em vez de "Use cards" | Pensar "Qual informação precisa ser agrupada?" |
|---|---|
| Em vez de "Use animação" | "Que mudança de estado precisa ser comunicada?" |
| Em vez de "Use sidebar" | "Qual é a IA e a frequência de navegação?" |
| Em vez de "Use dark mode" | "Existe necessidade real de contraste/ambiente/preferência?" |
| Em vez de "Use cards" | "Qual é a relação hierárquica entre os dados?" |
| Em vez de "Use tabela" | "Como o usuário escaneia, compara e age sobre esses dados?" |

## 2. DECISION ENGINE

```
USUÁRIO
   ↓
OBJETIVO (o que ele quer alcançar?)
   ↓
CONTEXTO (onde está? frequência? dispositivo? pressão?)
   ↓
INFORMAÇÃO (o que ele precisa ver, nesta ordem?)
   ↓
AÇÃO (qual a ação primária? secundárias?)
   ↓
FEEDBACK (o que confirma que a ação funcionou?)
   ↓
ESTADO (o que a interface mostra depois?)
   ↓
PRÓXIMA AÇÃO (o que ele faz agora? está óbvio?)
```

**Regra**: o design é consequência do fluxo. Se uma escolha visual não sobrevive a esse encadeamento, é decoração.

## 3. Processo (Anthropic frontend-design + melhorado)

1. **Ancorar no assunto** — nomear o produto, o público, o único trabalho da página. A identidade vem do mundo do produto (materiais, instrumentos, vernáculo).
2. **Plano compacto** (antes de codar):
   - **Cor**: 4–6 hex nomeados
   - **Tipo**: 2+ papéis (display com personalidade usado com moderação, body, utility para dados)
   - **Layout**: 1 conceito em prosa + wireframe ASCII
   - **Assinatura**: 1 elemento único pelo qual a página será lembrada
3. **Crítica do plano** — se parece o que você faria para QUALQUER página, revise. (Os 3 "looks" de IA a evitar: creme+serif+terracota; quase-preto+acento ácido; broadsheet hairline.)
4. **Construir seguindo o plano** — derivar TODA cor/tipo do plano, não improvisar.
5. **Criticar de novo** — "tire um acessório" (conselho de Chanel). Boldness em UM lugar.

## 4. Restrição e auto-crítica

- Gaste a ousadia em um único lugar; o resto quieto e disciplinado.
- Corte qualquer decoração que não sirva ao briefing.
- Não tomar risco também é risco — mas risco com justificativa.
- Qualidade mínima sem anunciar: responsivo até mobile, foco visível, reduced-motion.

## 5. Cópia como material de design

- Escreva do lado do usuário da tela ("gerencia notificações", não "configura webhook").
- Nomeie pelo que o usuário controla.
- Voz ativa como padrão: "Salvar alterações", não "Enviar". Mesmo nome no botão e no resultado ("Publicar" → toast "Publicado").
- Erros não pedem desculpa e nunca são vagos.
- Tela vazia é convite à ação.
- Especificidade > esperteza.

## 6. Quando ousar vs. quando conter

| A interface... | Use |
|---|---|
| parece genérica demais | `bolder` (mais confiança/contraste/impacto) |
| está barulhenta/densamente saturada | `quieter` (reduz intensidade sem perder hierarquia) |
| tem excesso de controles/competindo | `distill` (remove complexidade) |
| funciona mas está sem alma | `delight` (personalidade após fundamentos OK) |
| precisa comunicar estado | `animate` (movimento proposital) |
