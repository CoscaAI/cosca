# TESTING PROTOCOL — A espinha dorsal da qualidade

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — como testar, não só os limiares
> **Propósito**: referência operacional ÚNICA para testar (o quê, como, em que
> ordem). Complementa o `QUALITY_GATES.md` (Gate 2.5) e o `AUDIT_PROTOCOL.md`.

---

## 1. A PIRÂMIDE — o que testar primeiro

```
        ┌─────────────┐
        │    E2E      │  poucos, fluxo crítico (o Don usa)
        ├─────────────┤
        │ Integration │  contratos entre módulos
        ├─────────────┤
        │    Unit     │  muitos, a lógica pura (coração)
        └─────────────┘
```

- **Unit** — a lógica pura. Mais numerosos, mais rápidos, mais valiosos.
- **Integration** — os contratos (módulo ↔ módulo, DB, API).
- **E2E** — o fluxo completo que o Don/usuário usa. Poucos, mas provam o todo.

---

## 2. AS REGRAS (incondicionais)

1. **Determinismo** — 0 testes flaky. Flaky é pior que sem teste (corrói a confiança).
2. **Independência** — nenhum teste depende de outro.
3. **Caminho feliz + borda + erro** — testar os 3, não só o feliz.
4. **Teste prova o comportamento, não a implementação** — refatorar não pode quebrar teste.
5. **Cobertura é meio, não fim** — 80% no código mudado (Gate 2.5), mas cobertura
   sem asserção é número vazio.

---

## 3. O ATAQUE à cobertura (provado em L214-L218)

Quando a cobertura está baixa, ataque em fases, não de uma vez:

```
18,7% → 32,7% → 41,9% → 62,7%  (fases 1→4, por valor/ROI)
```

- **Value-driven** — ataque primeiro o que tem mais valor (não o mais fácil).
- **Cada fase fecha um domínio** (analytics, rebac, router, tool-contract...).
- **Medir a cada fase** — cobertura sem medição é wishful thinking (P13).

---

## 4. GOTCHAS

1. **Flaky é bug de teste, não do código** — isola e corrige a causa raiz
   (L211: o finalizer do GC causava flakiness, não o teste).
2. **Mock demais = teste de mentira** — mock o que é externo, não o que é seu.
3. **Coverage 100% ≠ qualidade** — cobertura cega sem asserção real é vaidade.
4. **E2E ausente = regressão invisível** — o Don descobre o bug (L213).

---

## 5. O FISCAL — anti-AI-slop na saída

Qualidade também é escrita: a casa fiscaliza o OUTPUT (documentos, docs, código
gerado) contra padrões de **AI slop** (palavras banidas, frases vazias, em-dash
abusivo) — o `cosca slop`:

```bash
cosca slop check README.md docs/     # fiscaliza arquivos/dirs
cosca slop check . --skip-patterns   # pula fixtures/dados de padrão
# exit code: 0 = limpo, 1 = slop encontrado
```

**Regras:**
- Rodar antes de entregar docs ao Don (release gate de escrita).
- `--skip-patterns` para dados/fixtures — slop é sobre escrita, não dados.
- O binário vem do slopguard e é reconstruído se os fontes forem mais novos.

---

## 6. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — como testar, não só os limiares |
| 1.1.0 | 2026-08-16 | §5 Fiscal anti-slop incorporado (análise de lacunas de protocolos) |
