# CARRO PROTOCOL — Checkup e religada do sistema

> **Versão**: 1.0.0 | **Status**: canônico | **Dono**: cosca-kernel
> **Criado**: 2026-08-16, por ordem do Don — "esse aqui poderia se chamar carro"
> **Propósito**: quando o Don fala **"carro"** ou **"ativar protocolo carro"**, o kernel
> vai DIRETO ao ponto — sem re-descobrir, sem re-investigar do zero. Este arquivo é o
> runbook único de diagnóstico + religada do sistema. Nasceu do L288.

---

## 1. O QUE É "carro"

A metáfora da família (L209): "como tá o carro?" = "como está o Cosca?". Checkup do
carro = auditoria real do sistema — não narrativa, VERDADE medida (P13).

---

## 2. GATILHO — quando executar

| Fala do Don | Ação |
|-------------|------|
| `carro` | checkup completo (diagnóstico + religar o que estiver caído) |
| `ativar protocolo carro` | idem — carregar ESTE arquivo e executar o §3 |

**Regra de ouro**: ir direto ao §3. Não re-ler o framework inteiro. Não re-investigar
o que já está mapeado aqui. Executar, verificar, reportar.

---

## 3. CAMINHO RÁPIDO (runbook, na ordem)

```bash
# 1. DIAGNÓSTICO em 3 camadas (paralelo)
cosca doctor                         # runtime/serve/memory/knowledge — o ⚠ aponta o alvo
cosca hardware                       # CPU/RAM/GPU (GPU: confirmar com rocminfo — ver §5)
systemctl --user status cosca-serve.service   # serve ativo? (transient ou persistente?)

# 2. SERVIÇOS (checar de uma vez)
curl -s --max-time 3 http://127.0.0.1:14120/ready    # serve (REST)
curl -s --max-time 3 http://127.0.0.1:11434/api/version  # ollama

# 3. RELIGADA do serve (se caído) — o fix padrão
cp deploy/cosca-serve.service ~/.config/systemd/user/
systemctl --user reset-failed cosca-serve.service    # despeja a transient fantasma
systemctl --user daemon-reload
systemctl --user enable --now cosca-serve.service

# 4. VERIFICAR (prova, não fé)
curl -s http://127.0.0.1:14120/ready   # espera {"ready":true,"subsystems":{...:"healthy"}}
systemctl --user show cosca-serve.service -p NRestarts   # 0 = estável
```

**Só religar se o Don mandou** — checkup puro = reportar e perguntar.

---

## 4. FALHAS CONHECIDAS → FIX (não re-diagnosticar)

| Sintoma | Causa raiz | Fix |
|---------|-----------|-----|
| serve DOWN, `status=2/INVALIDARGUMENT` | unit **transient** sem `EnvironmentFile` (sem `COSCA_JWT_SECRET`) | instalar `deploy/cosca-serve.service` + `reset-failed` + `enable --now` (§3) |
| `systemctl enable` falha: "is transient or generated" | transient fantasma sombreando a persistente | `reset-failed` → `daemon-reload` → `enable` |
| serve DOWN, `status=1/FAILURE` | gate de integridade / `logger.Fatal` (JWT ausente etc.) | ver §5 gotcha 1 (jaula engole o erro real) |
| `cosca hardware` diz "ROCm: no" | falso negativo do probe | `rocminfo \| grep gfx` — se gfx1030 aparece, ROCm ESTÁ lá |
| serve crash-loop após assinar a chain | assinatura mexendo no workspace (transitório) | `Restart=always` self-heal — esperar, conferir `NRestarts` |

---

## 5. VERDADE DO HARDWARE (P13 — medir, nunca assumir)

- GPU real: `lspci | grep -iE 'vga|3d'` + `rocminfo | grep -iE 'gfx|Marketing'`.
- AMD RX 6700 XT = gfx1030. ROCm presente se `rocminfo` lista `gfx1030`.
- Não confiar só no `cosca hardware` para ROCm (falso negativo conhecido).

---

## 6. GOTCHAS

1. **A jaula engole o stderr do processo interno** — `jail.go` captura `bwrapStderr` e
   só imprime em falha de sandbox. Para ver o erro REAL do serve: rodar com
   `COSCA_JAILED=1` (pula o reexec) e capturar stderr direto.
2. **"status=2/INVALIDARGUMENT" é o NOME do systemd para exit code 2** — não é erro de
   argumento; é o exit do processo interno propagado por `jailExitCodeFromRun`.
3. **Unit transient sombreia a persistente** — mesma nome, a transient vence enquanto
   carregada. `reset-failed` despeja.
4. **serve sem `serve.env` não sobe** — `serve.go` recusa sem `COSCA_JWT_SECRET`.
   O `EnvironmentFile=%h/.config/cosca/serve.env` é obrigatório.
5. **JWT nunca no argv** — é entregue via `jail-secrets.env` (0600). Confirmar no `ps`
   que não aparece.
6. **`Restart=always` é a resiliência** — a unit antiga (transient) não tinha; qualquer
   queda deixava o motor parado. A persistida self-heal.
7. **Assinar a family chain com o serve rodando = crash-loop transitório** — entre o
   `commit` e o `sign` (ORDEM SAGRADA L199), o git commit hash é NOVO mas a
   `family_chain.dat` ainda é VELHA; o boot do serve (`bootstrap.Compose` → `integrity.Check`)
   bloqueia com "family chain breach" (fail-closed, exit 1) e entra em restart. O
   `Restart=always` self-heal assim que o sign termina. **Workaround limpo**: parar o
   serve (`systemctl --user stop cosca-serve`) → commit → sign → religar. Ou aceitar
   os ~40s de loop.

---

## 7. HISTÓRICO

| Versão | Data | Mudança |
|--------|------|---------|
| 1.0.0 | 2026-08-16 | Criado por ordem do Don — o runbook do checkup (nasceu do L288) |
