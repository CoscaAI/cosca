# Tree Generation Research: L-system vs Space Colonization

## L-Systems (Lindenmayer Systems)

**Como funciona:** Gramática formal que define regras de substituição de strings. Cada símbolo representa uma ação de desenho (avançar, girar, ramificar).

**Exemplo:**
```
W = "X"
X → "F+[[X]-X]-F[-FX]+X"
F → "FF"
```

**Vantagens:**
- Simples de implementar
- Determinístico (mesma seed = mesmo resultado)
- Bom para árvores com padrão recursivo claro (coníferas, samambaias)
- Performance excelente
- Fácil de controlar profundidade/complexidade

**Desvantagens:**
- Resultados podem parecer artificiais (muito simétricos)
- Não respeita espaço livre naturalmente
- Difícil modelar competição por luz
- Ramificação pode parecer repetitiva

**Melhor para:**
- Coníferas (pinheiro, abeto)
- Plantas com padrão fractal claro
- Situações onde performance é crítica
- Primeira implementação rápida

---

## Space Colonization Algorithm

**Como funciona:** Simula competição por espaço/luz. Pontos-âncora são distribuídos no volume desejado. Ramos crescem em direção aos pontos mais próximos, "colonizando" o espaço.

**Vantagens:**
- Resultados muito mais orgânicos
- Respeita espaço livre naturalmente
- Modela competição por luz
- Copa irregular e natural
- Ramificação não simétrica

**Desvantagens:**
- Mais complexo de implementar
- Mais lento (requer iterações)
- Menos determinístico (depende de distribuição de pontos)
- Mais difícil de controlar parâmetros específicos

**Melhor para:**
- Árvores de folha larga (carvalho, bétula)
- Árvores em ambientes competitivos
- Florestas (múltiplas árvores interagindo)
- Resultado visual orgânico

---

## Recomendação

**Usar L-system como base** para:
- Tronco e galhos principais
- Estrutura hierárquica
- Controle de profundidade

**Combinar com Space Colonization** para:
- Distribuição de copa
- Folhagem em clusters
- Variação orgânica

**Para a primeira implementação:**
Começar com L-system puro (mais simples). Depois adicionar Space Colonization para copa.

---

## Referências

1. **Lindenmayer, A.** (1968). "Mathematical models for cellular interaction in development"
2. **Runions, A. et al.** (2005). "Modeling and Visualization of Leaf Canopy Development"
3. **Palubicki, W. et al.** (2009). "Self-organizing tree models for image synthesis"
4. **Boudon, P. et al.** (2006). "Interactive Design of Realistic Trees"
