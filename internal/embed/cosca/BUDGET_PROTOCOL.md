# BUDGET PROTOCOL — O seu dinheiro (tokens, tempo, custo)

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — a família não torra token à toa
> **Propósito**: referência operacional ÚNICA para contabilizar e limitar o custo
> de IA e de aquisição de evidência. Complementa o MODEL_PROTOCOL (a escolha) —
> isto é a disciplina.

---

## 1. O BUDGET cognitivo — antes de chamar IA

Antes de chamar o modelo, o sistema contabiliza: **Tokens, Tempo, Custo**.

```bash
cosca budget default                    # o budget padrão
cosca budget check --tokens N --time D  # dry-run: o consumo proposto cabe?
cosca budget check --tokens N --cost $X # com custo
```

**A escada (sempre nessa ordem)**:
```
1. local knowledge   → resolve? pronto (custo $0)
2. search determinístico → resolve? pronto
3. cached evidence   → resolve? pronto
4. chamar o modelo   → só agora, e só enquanto o budget permitir
```

---

## 2. O BUDGET de aquisição — não entrar em espiral

Quando o sistema precisa buscar evidência externa, o orçamento limita:
fontes, arquivos, rede, tempo, tokens.

```bash
cosca acquisition default
cosca acquisition budget check --sources N [--files/--network-mb/--time]
```

Impede a espiral: *não sei → busca → não sabe → busca mais → contexto explode
→ custo explode*.

---

## 3. LOCAL vs NUVEM — a decisão de custo

| Cenário | Escolha | Custo |
|---------|---------|-------|
| Task simples (classificação, extração) | determinístico / ollama local | $0 |
| Raciocínio médio | qwen local (se a GPU aguenta) | $0 |
| Qualidade alta / task pesada | nuvem (openai/deepseek) | $/token |

**Regra**: o LLM local (ollama/qwen) é grátis e roda na GPU da casa (L16).
Nuvem só quando o local não aguenta (VRAM/qualidade/task).

---

## 4. GOTCHAS

1. **Budget é diagnóstico (read-only)** — não altera estado; disciplina, não
   executa.
2. **A escada é inegociável** — pular pro modelo sem tentar knowledge/cache é
   torrar dinheiro.
3. **Custo explode em espiral** — o acquisition budget existe pra cortar isso.
4. **Local é ativo da casa** — a RX 6700 XT já foi paga; usar nuvem sem precisar
   é desperdício.

---

## 5. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — a disciplina do custo |
