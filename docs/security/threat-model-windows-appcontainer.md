# Threat Model — Sandbox Nativo Windows (AppContainer + Job Object)

> **Status:** ativo (proposto, aguardando gate do Don) | **Owner:** Security Chief | **Last Updated:** 2026-09-01
> **Escopo:** isolar a execução de comandos de agentes/tools no host Windows via **AppContainer + Job Object**, substituindo o fallback direto (sem sandbox) atual. Complementa `threat-model-cofre-windows.md` — resolve o item **(c) Blindagem Nativa Host** como mecanismo principal (decisão do Don: nativo, sem WSL2, sem VM).
> **Fonte de design:** ADR-034 (docs/adr/ADR-034-native-windows-sandbox.md).

---

## 1. Assets (o que protegemos)

| ID | Asset | Nota |
|---|---|---|
| **AS1** | Workspace do Don (código, `.cosca/`, memória, knowledge) | o patrimônio |
| **AS2** | Credenciais: `COSCA_JWT_SECRET`, `COSCA_METRICS_SECRET`, vault AES-256-GCM (chave derivada de JWT), provider API keys | vazar = perder tudo (V5 do threat model do Cofre) |
| **AS3** | Processo do agente/tool (o que o LLM manda executar) | código não confiável por definição |
| **AS4** | Host Windows (FS fora do workspace, `%APPDATA%`, `C:\Windows`, rede) | superfície de blast |
| **AS5** | Disponibilidade do host (CPU, memória, disco) | fork bomb / estouro / exaustão |

---

## 2. Atores (A)

| ID | Ator | Nota |
|---|---|---|
| **A1** | Agente não-confiável (o LLM que decide) | prompt-injection; não é a fonte da verdade |
| **A2** | Tool/plugin/workload malicioso | código que o gate executa — a superfície real |
| **A3** | Dados de entrada externos | payloads malformados |
| **A4** | Saída de modelo comprometida (Ollama) | tenta sacar a jaula / exfiltrar |
| **A5** | Processo/usuário local do host | se o host cai, lê tudo (fora do alcance do gate) |
| **A6** | Operador mal-informado | acha que está isolado quando não está |

---

## 3. Vetores (V) mapeados por componente do sandbox

### 3.1 Contenção de filesystem (AppContainer grants)

| ID | Vetor | Mitigado por AppContainer? | Nota residual |
|---|---|---|---|
| **V8** | Escape de FS / path traversal saindo do workspace | ✅ **SIM** — sem grant, acesso negado por default | grant é só o workspace (RO/RW por modo); `internal/embed` RO explícito |
| **V5** | Leitura de segredos (`serve.env`, vault, `.cosca/jail-secrets.env`) | ✅ **SIM** — fora do workspace, sem grant → Access Denied | se o segredo estiver DENTRO do workspace grantado, **vaza** — garantir que `.cosca/` sensível não seja grantado ao agente |
| **V-FS1** | *Path disclosure* (saber que um caminho existe) | ⚠️ **parcial** — erro de API pode revelar existência | risco residual documentado; sem grant o conteúdo não é legível |
| **V-FS2** | Symlink/junction dentro do workspace apontando para fora | ⚠️ **verificar na Onda 2** — ACL segue symlink? | teste obrigatório (critério de aceite) |

### 3.2 Contenção de rede (AppContainer network capability)

| ID | Vetor | Mitigado? | Nota |
|---|---|---|---|
| **V6** | Exfiltração de dados via rede (knowledge, memória, segredos) | ✅ **SIM** — sem network capability, socket/HTTP falham | loopback só com capability explícita; default fail-closed |
| **V-R1** | Loopback não confiável (agente alcança APIs locais 1412x, Ollama 11434) | ✅ **SIM** — loopback também é capability | decidir por política: tools de provider (Ollama) precisam de loopback; negar por default e permitir via allowlist de comando |

### 3.3 Contenção de recurso (Job Object)

| ID | Vetor | Mitigado? | Nota |
|---|---|---|---|
| **V-DoS1** | Estouro de memória (heap além do teto) | ✅ **SIM** — `JOB_OBJECT_LIMIT_PROCESS_MEMORY` (6 GiB default, `COSCA_SANDBOX_MEMORY_MB`) | host intacto; processo morto pelo Job |
| **V-DoS2** | Fork bomb / excesso de processos | ✅ **SIM** — `JOB_OBJECT_LIMIT_ACTIVE_PROCESS` | teto imposto pelo Job |
| **V-DoS3** | Processo pendurado / idle | ✅ **SIM** — `processutil.Run` (IdleTimeout + MaxRuntime + tree-kill) | já existe no Linux; reusado |
| **V-DoS4** | Processos órfãos após término | ✅ **SIM** — `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` | fechar handle = derrubar árvore inteira |
| **V-DoS5** | Exaustão de disco (escrita infinita no workspace) | ⚠️ **parcial** — sem quota nativa; mitigar com ACL + monitoramento | risco residual: o workspace grantado é gravável no modo Workspace |

### 3.4 Elevação / escape

| ID | Vetor | Mitigado? | Nota |
|---|---|---|---|
| **V-E1** | Escape do AppContainer → privilégio do host | ✅ **SIM** (parcial) — low integrity + restricted token + SID | AppContainer é o mecanismo da Store; MS hardening; mas **não é à prova de bugs** — assumir residual baixo, não zero |
| **V-E2** | Executar binário fora do workspace | ✅ **SIM** — AppContainer exige grant; PATH absoluto | comando resolve binário apenas em grants; testar tools que precisam de `cmd.exe`/`powershell.exe` (borda de compat) |
| **V-E3** | Env herdado com credenciais | ✅ **SIM** — env explícito (allowlist `validateEnv`/`safeEnv`) | nunca herda ambiente do host |

---

## 4. STRIDE

| Categoria | Vetores |
|---|---|
| **Spoofing** | V-E1 (forjar SID AppContainer — baixo), A1 (prompt-injection, fora do gate) |
| **Tampering** | V8, V-FS2, V-E2 |
| **Repudiation** | V-DoS3 (agente apaga evidência — mitigar com log do gate fora do workspace) |
| **Information Disclosure** | V5, V-FS1, V6 |
| **Denial of Service** | V-DoS1, V-DoS2, V-DoS3, V-DoS5 |
| **Elevation of Privilege** | V-E1, V-E2, V-E3 |

---

## 5. Linha de base do sandbox (o que o gate NATIVO precisa manter)

1. **Código executado fica contido** — FS fora do workspace, rede e recursos são negados por default; grants explícitos por comando.
2. **Segredos nunca vazam** — o processo não alcança nada fora do grant; `.cosca/` sensível não é grantado.
3. **Fail-closed real** — AppContainer indisponível ⇒ **erro**, nunca execução direta silenciosa (a não ser `SandboxFull` + opt-in).
4. **Morte limpa** — nenhum processo órfão, nenhuma memória vazando, kill-on-close garantido.
5. **Detecção automática** — se o SO suporta AppContainer, usa; sem flag manual, sem config.

---

## 6. Gaps e recomendações (para a Onda 2)

| # | Gap | Recomendação | Prioridade |
|---|---|---|---|
| **G1** | `.cosca/` sensível pode estar dentro do workspace grantado | Garantir que grants NÃO incluam `.cosca/audit`, `jail-secrets.env`, `knowledge.db`; grants de subpastas específicas, não do workspace inteiro quando possível | **P0** |
| **G2** | Loopback é capability — providers (Ollama) precisam de rede local | Policy explícita: negar rede por default; permitir loopback apenas para comandos de provider marcados; nunca rede externa | **P0** |
| **G3** | Borda de compat `cmd.exe`/`powershell.exe` PATH | Testar os comandos reais de tool; PATH absoluto; documentar tools que exigem ajuste | P1 |
| **G4** | Symlink/junction saindo do workspace | Teste de contenção dedicado (V-FS2) + Rails já valida caminhos | **P0** |
| **G5** | Exaustão de disco dentro do grant | Monitorar escrita; considerar quota via ACL/API de quota quando viável | P1 |
| **G6** | Log do gate (evidência de ação) dentro do alcance do agente | Log de segurança em local FORA do workspace (ex.: `%ProgramData%` ACL) | P1 |
| **G7** | Path disclosure residual | Aceitar + documentar (AppContainer não esconde FS) | P2 |

---

## 7. Testes de contenção (shed test — Onda 2)

1. Gravar em `C:\Windows\system32\config\...` → **denegado**.
2. Ler `serve.env` fora do workspace → **denegado**.
3. `socket` para host externo → **falha** (sem capability).
4. Alocar memória além do teto → **morto pelo Job Object**.
5. Criar N processos → **teto ACTIVE_PROCESS imposto**.
6. Árvore de filhos → **kill-on-close derruba todos**.
7. Modo ReadOnly: escrita no workspace → **denegada**.
8. AppContainer indisponível (simulado) → **Execute retorna erro** (fail-closed).

---

## 8. Risco residual (comunicação honesta ao Don)

- **Escape do AppContainer:** baixo, não-zero (qualquer sandbox de SO tem bugs; AppContainer é o mais endurecido do Windows).
- **Path disclosure:** aceito (conteúdo não é legível; existência pode vazar via erro).
- **Disco dentro do grant:** exaustão possível sem quota (mitigação parcial).
- **Exfiltração por channels não-rede** (ex.: agente escreve resultado em arquivo do grant e o operador copia manualmente): fora do alcance do gate — é controle humano.
- **AppContainer NÃO esconde o FS** (MAC, não namespace): o modelo de ameaça é "bloqueia acesso", não "oculta mundo".

---

> **Pendente:** gate do Don para Onda 2 (implementação). Este threat model é o contrato de segurança da implementação — nenhum código de produção antes da aprovação.
