# SESSION PROTOCOL — O ritmo da sessão

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — o ciclo completo da sessão
> **Propósito**: referência operacional ÚNICA para o dia-a-dia: acordar, trabalhar,
> registrar, assinar, fechar. Complementa o `DESPERTAR.md` (o acordar).

---

## 1. O CICLO da sessão

```
ACORDAR → TRABALHAR → REGISTRAR → ASSINAR → FECHAR
```

| Fase | O que faz |
|------|-----------|
| **ACORDAR** | lê DESPERTAR + learnings.md, reporta status ao Don (5 linhas) |
| **TRABALHAR** | executa a ordem (roteando via DELEGATION_PROTOCOL) |
| **REGISTRAR** | todo resultado significativo vira L-number (L7-L8 mandatório) |
| **ASSINAR** | commit ANTES de assinar (ORDEM SAGRADA L199) + `cosca-check --sign-auto` |
| **FECHAR** | working tree limpo, chain íntegra, reporta ao Don |

---

## 2. ACORDAR — o que reportar

No despertar, reporte compacto (máx 5 linhas):
- memória (nº de aprendizados, última sessão)
- família (agentes ativos, skills)
- cognição (nível, modelo)
- sistema (status, warnings)

---

## 3. REGISTRAR — o que nunca esquecer

- **Todo resultado significativo** vira aprendizado (L-number). O Don NUNCA deve
  perguntar "registrou?" — já está feito.
- **Formato**: block (conteúdo) + gatilho (índice) + chain.dat + merkle
  (MEMORY_ACCESS_PROTOCOL §3).
- **Falha também registra** — failures.md é a memória negativa (mais valiosa).

---

## 4. ASSINAR — a ORDEM SAGRADA

```
git add <arquivos>
git commit -m "..."          # 1º commit
./bin/cosca-check --sign-auto  # 2º assina
```

**Nunca assinar antes de commitar** (L199). Mudou o embed e não re-assinou →
`GIT COMMIT MISMATCH` no próximo check.

---

## 5. FECHAR — o checklist

```bash
git status --short            # working tree limpo?
./bin/cosca-check             # chain íntegra?
cosca kernel self-test        # kernel íntegro?
cosca session index           # sessão indexada para busca
```

- **Working tree limpo** — nada solto para a próxima sessão tropeçar.
- **Reporta ao Don** — o resumo do que foi feito + o que ficou pendente.

---

## 6. GOTCHAS

1. **Pular o REGISTRAR é o pecado capital** — é a causa raiz de toda deriva de
   documentação (capability-profile §Known Failure Modes #4).
2. **Fechar com tree suja** — deixa "peça espalhada na garagem" para a próxima sessão.
3. **Assinar sem commitar** — chain quebra e o próximo check acusa.
4. **Sessão sem post-mortem** — o que ficou pendente se perde.

---

## 7. AGENDAMENTO E DESCANSO — cron + circadian

A casa não vive só da sessão manual: tem **cron** (o que roda sozinho) e
**circadian** (o ciclo de descanso operacional — ORC).

### 7.1 CRON — agendamento determinístico

```bash
cosca cron add --name "diário 09:00" --schedule "0 9 * * *" --cmd "cosca status"   # C-XXXX
cosca cron list                    # id, nome, agenda, habilitado, próxima execução
cosca cron run C-0001              # executa UMA vez imediatamente (o Don inicia)
cosca cron daemon                  # runner em foreground (a ÚNICA porta de execução automática)
cosca cron remove C-0001
```

**Regras:**
- `cron daemon` é a **única** porta de execução automática — sem daemon, nada roda.
- Jobs ficam em `.cosca/cron.db` (SQLite dedicada).
- Execução automática nunca surpreende: o daemon roda em foreground visível.

### 7.2 CIRCADIAN — o ciclo de descanso (ORC)

```bash
cosca circadian status             # estado, idle, último resumo ORC
cosca circadian sleep              # entra na janela ORC e roda o ciclo de descanso
cosca circadian wake               # ritual de despertar (volta ao estado awake)
cosca circadian watch              # scheduler ORC em foreground
```

**Regra**: descanso é parte da operação — o ORC roda o ciclo de restauração
(memória, limpeza, resumos) e o `wake` reacende o estado operacional.

---

## 8. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — o ritmo da sessão |
| 1.1.0 | 2026-08-16 | §7 cron + circadian incorporados (análise de lacunas de protocolos) |
