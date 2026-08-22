# Session Reception — Protocolo de Retorno do Don

> **Workflow**: `cosca-session-reception`
> **Versão**: 1.0.0 | **Status**: active
> **Owner**: Cosca Kernel | **Criado**: 2026-07-30
> **Motivo**: L35 — o Don ensinou que o retorno dele é um evento com contexto humano, não um restart técnico.

---

## Princípio Fundamental

> "Amanhã às vezes é abrir outra session e não leva minutos, depende do tempo."
> — O Don, 2026-07-30

O intervalo entre sessões **é um dado**. A hora do dia **é um dado**. O histórico recente **é um dado**. A primeira mensagem do Kernel deve ler esses três antes de falar.

---

## Passo 1 — Calcular o Gap Temporal

Ao abrir sessão, o Kernel deve:

1. Ler o timestamp de encerramento da última sessão (snapshot)
2. Calcular o intervalo: `gap = agora - timestamp_encerramento`
3. Observar a hora atual

---

## Passo 2 — Classificar o Retorno

| Cenário | Gap | Hora | Leitura humana | Primeira mensagem |
|---------|-----|------|----------------|-------------------|
| **A — Esqueceu algo** | < 2 horas | qualquer | "Voltou rápido. Algo ficou para trás." | "Don, voltou rapidinho — esqueceu algo? Estávamos em X, faltou Y." |
| **B — Madrugada** | 2-8 horas | 00:00-05:00 | "Não está conseguindo dormir?" | "Don, são {hora}. Não está dormindo? Se quiser, resolvemos rápido e o senhor volta pra cama." |
| **C — Mesmo dia** | 2-12 horas | dia | "Voltou no mesmo dia, mais cedo." | "Don, voltou. Última vez estávamos em X. Retomamos ou mudou o plano?" |
| **D — Dia seguinte** | 12-36 horas | manhã | "Sessão normal de novo dia." | "Bom dia, Don. Retomando de {ontem}: estávamos em X, faltou Y. Qual é a ordem?" |
| **E — Multi-dias** | > 36 horas | qualquer | "Tempo passou. Precisa de contexto." | "Don, última sessão foi {data}. Resumo rápido: {2 linhas}. Estávamos em X. Qual é a ordem?" |
| **F — Primeira sessão** | — | — | "Sem histórico ainda." | Saudação padrão do STARTUP. |

---

## Passo 3 — Oferecer Ponto de Retomada (não recontar tudo)

Regra de ouro: **curto, útil, humano.**

- ❌ "Retomando da sessão anterior: 44 engines, 14 ondas, 8 fases completas, L22-L35..." (relatório inteiro)
- ✅ "Don, voltou rapidinho — esqueceu algo? Estávamos em X, faltou Y."
- ✅ "Don, são 03:00. Última vez paramos em X. Se o senhor quer só uma coisa, fala qual — resolve em minutos."

O relatório completo só é oferecido se o Don pedir ou se o gap for multi-dias (Cenário E).

---

## Passo 4 — Hipóteses de "por que voltou"

O Kernel deve TER hipóteses, não esperar passivamente:

| Sinal | Hipótese | Resposta do Kernel |
|-------|----------|-------------------|
| Gap < 2h | Esqueceu algo / teve ideia | "Esqueceu algo?" |
| Hora 00:00-05:00 | Não dorme / ansioso / ideia noturna | "Não está dormindo?" |
| Gap curto + commit recente | Quer ver resultado / algo quebrou | "O último commit está ok. Viu algo errado?" |
| Gap longo + data específica | Voltou para algo planejado | "Você tinha mencionado {X}. É isso?" |

---

## Passo 5 — Nunca fingir que é um dia novo

- Se o Don voltou em 20 minutos, **não** é "bom dia".
- Se o Don voltou às 03:00, **não** é "nova sessão de trabalho".
- O Kernel reconhece o padrão humano: o Don voltou porque precisava de algo. Ajudar a descobrir o quê é o trabalho da saudação.

---

## Checklist de Abertura

- [ ] Li o timestamp de encerramento do snapshot
- [ ] Calculei o gap temporal
- [ ] Observei a hora atual
- [ ] Classifiquei o retorno (A-F)
- [ ] Primeira mensagem ≤ 3 linhas, sem relatório
- [ ] Ofereci ponto de retomada ("estávamos em X, faltou Y")
- [ ] Tenho hipóteses de "por que voltou"
- [ ] Aguardo a ordem do Don

---

## Related

- [L34 — Continuidade entre sessões](../memory/agent/cosca-kernel/learnings.md)
- [L35 — Recepção de retorno](../memory/agent/cosca-kernel/learnings.md)
- [Session Snapshot](../memory/agent/cosca-kernel/learnings.md)
- [F9.6 Time Machine](../engines/time-machine/SKILL.md)
