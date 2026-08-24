# Threat Model + Plano de Blindagem — Cofre do Cosca no Windows

> **Status**: ativo (P0/P1) | **Owner**: Security Chief | **Last Updated**: 2026-08-24
> **Escopo**: blindar o "Cofre" (zona air-gap de validação semântica + IA local) contra
> entrada de dados não validada, no runtime Windows (win32) com WSL2 ativo.
> **Resumo executivo**: o cosca **não tem sandbox real no Windows** — a brecha central era que
> os launchers instalados injetavam `COSCA_ALLOW_NO_ROOT=1` por padrão (opt-in virou default),
> e o `bwrap` não existe no Windows. Nesta rodada a **camada (b) foi implementada**: a env
> passou a ser **nunca default** nos launchers e o fail-closed agora é **real** (sem opt-in o
> processo é NEGADO/exit 1). Permanecem pendentes (a) Cofre/WSL2+bwrap e (c) blindagem nativa.

---

## 0. Estado atual verificado (evidências)

| Item | Fato | Onde |
|---|---|---|
| **bwrap no Windows** | `jailAvailable()` sempre `false`; `ReexecInJail()` cai em `jailFallback` | `pkg/cosca/jail_windows.go` (24-36) |
| **Opt-in virou default** | **CORRIGIDO (2026-08-24)**: `deploy/install-service.ps1` L136, `cosca-serve.bat` L2, `cosca-service.ps1` L24/93 **não injetam mais** `COSCA_ALLOW_NO_ROOT=1`. Opt-in agora é explícito e nunca default | launchers instalados (verde/nêutro) |
| **Claim de fail-closed** | `deploy/README-WINDOWS.md` §4/§11 alega "fail-closed preservado" mas fica **falso** porque o opt-in é default — só emite `SECURITY WARNING` | `deploy/README-WINDOWS.md` (63-82, 229-235) |
| **Bypass silencioso** | ~40 comandos passam por `isAdminCommand()` e pulam `ReexecInJail()` inteiro → rodam sem sandbox **incondicionalmente e sem aviso** | `main.go` (via learnings 2026-08-24) |
| **Gate por-comando** | `gate_other.go` `findBwrap()` retorna `""` no Windows → `execDirect` = `cmd /c <comando>` sem confinamento de workspace/rede/recurso | `internal/chat/sandbox/gate.go`, `gate_other.go` |
| **execpolicy** | tokenizer não interpreta `| && ; $()` — prefix rules contornáveis via `cmd /c` / `sh -c`; no Linux o bwrap contém o blast, no Windows **vaza** | execpolicy |
| **Segredos** | `vault.go` AES-256-GCM 0600, mas a **chave deriva de `COSCA_JWT_SECRET`** (ponto único); `serve.env` legível por qualquer processo na sessão do usuário | `internal/secrets/vault.go` |
| **WSL2 ativo, SEM distro** | `wsl --status` v2.7.12 / kernel 6.1; `wsl -l -v` → "nenhuma distribuição instalada" | comando real (2026-08-24) |
| **WSL2 bwrap** | bwrap é Linux; para usar no WSL2 é preciso instalar distro + `bubblewrap` e validar `apparmor_restrict_unprivileged_userns` (kernel 6.1) | `jail_linux.go` (ApparmorUsernsSysctlPath) |

---

## 1. Threat Model

### 1.1 Atores (A)

| ID | Ator | Nota |
|---|---|---|
| **A1** | Cosca externo / agente não-confiável | fonte de prompt-injection; tem capacidade de exploração e **não é a fonte da verdade** (invariante oracle §25) |
| **A2** | Skill / plugin / workload malicioso | código que o pipeline executa — a superfície de execução real |
| **A3** | Dados de entrada externos (ingest/import/scan) | payloads malformados ou intencionalmente maliciosos |
| **A4** | Saída de modelo local (Ollama) comprometida | modelo que tenta sacar a jaula / escapar do Cofre |
| **A5** | Processo/usuário local no host | se o host for comprometido, lê tudo da sessão do usuário |
| **A6** | Operador/Don mal-informado | pode achar que está "seguro" porque só vê um `SECURITY WARNING` |

### 1.2 Vetores (V) e o que cada um compromete

| ID | Vetor | O que compromete |
|---|---|---|
| **V1** | **Prompt-injection no system prompt** (AGENTS.md/agent definitions injetados sem verificação — maior autoridade do contexto) | autoridade de decisão do agente; integridade semântica |
| **V2** | **Execução de código não confiável sem jaula** (gate → `cmd /c` não-confinado por causa do bwrap ausente + opt-in) | **comprometimento total da sessão do usuário** na máquina Windows; confidencialidade + integridade + disponibilidade |
| **V3** | **Bypass via `isAdminCommand`** (~40 comandos pulam a jaula; silencioso) | o mesmo de V2, mas **sem nem emitir aviso** — o pior caso |
| **V4** | **execpolicy sem parser de shell** (`\| && ; $()`) — prefix rules contornáveis | V2 com ofuscação; no Windows vira escape do perímetro |
| **V5** | **Vazamento de segredo** (serve.env legível; JWT/vault por `COSCA_JWT_SECRET`; segredo em argv via `--setenv` histórico) | todas as chaves (JWT, metrics, vault AES) |
| **V6** | **Exfiltração de dados do Cofre** (knowledge.db, memória assinada, chain) sem egress restrito | confidencialidade do patrimônio (memória/raízes) |
| **V7** | **Entrada não validada ganhando autoridade** (fabricação de provenance/`Confidence HIGH` sem evidência) | integridade semântica do Cofre; a função central do Oráculo é exatamente rejeitar isto |
| **V8** | **Escape de filesystem** (path traversal / symlink saindo do workspace) | leitura/escrita fora do Cofre |
| **V9** | **Breach do air-gap** (WSL2 NAT fala rede do Windows; sem egress block a zona "air-gap" alcança a internet) | a premissa fundamental do Cofre — validação **sem** interferência externa |
| **V10** | **Estado de segurança falso** (warning ruidoso tratado como "isolado"; doc alega fail-closed com opt-in default) | segurança por ilusão; decisão errada do Don/A6 |

### 1.3 Mapeamento STRIDE

| Categoria | Vetores |
|---|---|
| **Spoofing** | V7 (provenance forjada), V1 (prompt-injection) |
| **Tampering** | V8, V1 (memória/chain), V2 |
| **Repudiation** | V10 (security.log best-effort, não tamper-proof) |
| **Information Disclosure** | V5, V6 |
| **Denial of Service** | V2 (recursos), V1 |
| **Elevation of Privilege** | V2, V4, V5 |

### 1.4 Linha de base: o que o Cofre precisa manter
1. Nenhuma informação externa ganha autoridade só por entrar (invariante §25-29).
2. Code executado fica **contido** (FS/rede/recurso) — nunca com privilégio do usuário.
3. Segredos **nunca** vazam para o processo/agente.
4. Air-gap real: a zona Cofre **não alcança a internet** de fato, não só "no papel".

---

## 2. Blindagem em Camadas (priorizada)

> **Ordem de prioridade é a chave**: (b) fecha a brecha central de default fail-open;
> (a) é o isolamento real que o Cofre exige; (c) é defense-in-depth (NÃO substitui a);
> (d) é pré-requisito de autoridade e já existe — não pode regredir.

### (b) FECHAR A BOMBA DO `COSCA_ALLOW_NO_ROOT` — **P0**
**Problema**: o opt-in explícito foi transformado em default pelos launchers instalados. O
fail-closed do `jailFallback` (exit 1) existe, mas é anulado em produção.

**Como implementar**:
1. **Remover** `COSCA_ALLOW_NO_ROOT=1` de todos os launchers:
   - `deploy/install-service.ps1` L136 (bloco `if (-not $env:COSCA_ALLOW_NO_ROOT) { ... = '1' }`)
   - `deploy/README-WINDOWS.md` L165 (guia NSSM `AppEnvironmentExtra`) e §4/§11 (alegação de fail-closed — reescrever)
   - `cosca-serve.bat` L2
   - `cosca-service.ps1` L24/L93
2. **Tornar o fail-closed autoritativo**: o default (sem opt-in) já faz `exit 1`; garantir que
   **nenhum** caminho de execução rode sem sandbox apenas com aviso. Em especial: mover os ~40
   comandos de `isAdminCommand()` para o MESMO gate de jaula (ou bloqueá-los no Windows até
   haver jail).
3. **Fechar o gate por-comando**: em `gate_other.go` (Windows), **não** permitir `execDirect`
   silencioso. Sem bwrap → falhar (erro), não rodar `cmd /c` não-confinado, mesmo com opt-in.
4. **Opt-in remanescente** (se o Don quiser trusted-dev): trocar a env por uma **config
   declarativa falha-fechada** (`security.allow_no_sandbox: false` default) que exige ação
   explícita do operador e registra `CRITICAL` no security.log — nunca um default de script.

**Onde**: `deploy/*`, `pkg/cosca/jail.go` + `jail_windows.go`, `internal/chat/sandbox/gate*.go`, `main.go` (isAdminCommand).
**Risco**: quebra o loop de dev "trusted-dev" (o `serve` não sobe sem jail no Windows).
**Trade-off**: é a correção que impede o comprometimento **silencioso e default** — o
trade-off de conveniência é aceitável porque (a) fornece o jail alternativo.

---

### (a) JAIL WSL2 + BWRAP (o isolamento real do Cofre) — **P0**
**Problema**: bwrap não existe no Windows; o Cofre (Oráculo + IA local) precisa de isolamento
de namespace/filesystem **de verdade**, e a arquitetura de 2 zonas (Cofre sem internet +
Kernel com internet) precisa disso.

**Como implementar**:
1. **Instalar distro** no WSL2 (hoje **nenhuma**): `wsl --install -d Ubuntu-24.04`. Criar
   **usuário padrão não-root** (o `jail_linux.go` **recusa root** — bwrap como root anula a jaula).
2. **Instalar bwrap na distro**: `sudo apt install bubblewrap`. Validar
   `kernel.apparmor_restrict_unprivileged_userns` no kernel 6.1 do WSL2 (se `=1`, bwrap falha
   com "setting up uid map: Permission denied" — ver `jail_linux.go` ApparmorUsernsSysctlPath).
3. **Reaproveitar a lógica Linux testada**: `buildJailArgs` (workspace = `/` da jaula,
   `--unshare-all --unshare-user --clearenv --die-with-parent`, binds de sistema ro, tmpfs de
   build, secrets via `.cosca/jail-secrets.env` sem `--setenv`). O cosca **dentro da distro**
   roda o Cofre; o lado Windows é thin client (o listener REST/gRPC pode viver na distro com
   `localhost` forwarding, ou ser proxied).
4. **Air-gap de verdade**: por padrão **sem** `--share-net`; e, dentro da distro, bloquear
   egress (sem default route / firewall) para a zona Cofre. A zona Kernel permanece com
   internet — separação de 2 zonas conforme ADR planejado.
5. **`.wslconfig`**: carimbar memória/CPU (`[wsl2] memory=... processors=...`) e, para a zona
   isolada, desligar `networkingMode=mirrored`/loopback que diminua o isolamento de rede.

**Onde**: novo agente/executor `jail_wsl2` que invoca `wsl.exe`/`bwrap` remoto; nova implementação
em `pkg/cosca/` (ex.: `jail_windows.go` passa a delegar ao WSL2 em vez de cair em fallback mudo).
**Risco**: (1) **air-gap não é automático** — WSL2 NAT conversa com a rede do Windows; sem
egress block a zona "isolada" alcança a internet. (2) custo operacional alto (manter distro,
garantias de recurso do WSL2, latência de boot). (3) AppArmor/userns no kernel 6.1 do WSL2 pode
exigir `sysctl` para permitir user namespaces.
**Trade-off**: reuso máximo (lógica de jail já testada/endurecida no Linux) vs. custo de
operação e o risco de "air-gapped no papel" se o egress não for realmente bloqueado.

---

### (c) BLINDAGEM NATIVA HOST — **P1** (defense-in-depth / fallback enquanto (a) amadurece)
**Problema**: até o jail WSL2 estar pronto, o host precisa de layers defensivos; e mesmo depois,
defense-in-depth é necessário. **Não substitui (a).**

**Como implementar** (em `internal/chat/sandbox/` + `internal/hardening`):
1. **Job Objects** (melhor ROI, low cost): `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`
   (≈`--die-with-parent`), `JOB_OBJECT_LIMIT_ACTIVE_PROCESS`, `JOB_OBJECT_LIMIT_PROCESS_MEMORY`
   e `JOB_OBJECT_LIMIT_JOB_MEMORY`. Envolver **todo** subprocesso de agente. (`golang.org/x/sys/windows`
   já é dependência direta — ver go.mod v0.47.0.)
2. **Low Integrity / Restricted Token** (moderate): token Low-IL limita escrita a `%USERPROFILE%`
   e ao sistema e impede privilégios; **não** impede leitura/exfiltração — protege integridade,
   não confidencialidade.
3. **AppContainer** (Win8+, moderate/high): escopo FS+rede por capacidade — o mais próximo de
   bwrap/namespace; custo alto e bordas de compat com `cmd.exe`/`powershell.exe` PATH.
4. **`SetProcessMitigationPolicy`**: `PROCESS_CREATION_MITIGATION_POLICY_BLOCK_NON_MICROSOFT_BINARIES`,
   `..._NO_CHILD_PROCESS`, `..._DEP_ENABLE`, `..._FORCE_RELOCATE_IMAGES` (ASLR).
5. **Firewall de saída restrito**: Windows Defender Firewall / WFP — `cosca.exe`/WSL pode
   alcançar **apenas** loopback 1412x + Ollama (11434/11435); bloquear internet; e tratar a zona
   Cofre isolada na rede.

**Onde**: `internal/chat/sandbox/gate_other.go` (spawner), novo `internal/chat/sandbox/windows_job.go`,
`internal/hardening/hardening_*.go`.
**Risco**: AppContainer é alto custo e tem bordas de PATH; Job Object + Low-IL **não bloqueiam
leitura/exfiltração** (só contenção de processo/recurso). Um workload pode ainda ler
`serve.env`/vault.
**Trade-off**: proteção de processo/recurso de baixo custo sem fechar a exfiltração — por isso
é **P1**, nunca a resposta principal.

---

### (d) ORÁCULO COMO GUARD DE ENTRADA — **P0 (contínuo)**
**Problema**: nenhuma entrada externa pode ganhar autoridade sem passar pelo Gate. É a função
central do `internal/oracle` e **já está presente** — o risco é regredir ou ter caminho de
ingress que não passe por ele.

**Como implementar / reforçar**:
1. **Ingress gate-first**: todo dado externo que entra no Cofre carrega um `SemanticPackage`
   completo e passa por `Gate.Evaluate` — nada é ACEITO apenas por existir (invariante §25-29).
2. **Aplicar `SearchRequest.Valid()` / `SearchResult.Valid()` / `Provenance.Verifiable()`** na
   fronteira: sem proveniência verificável para `FACT/MEASURED` → rejeitar/rebaixar; sem evidência
   com `Confidence HIGH` → **REJECT** (promoção ilegítima).
3. **Quarentena de dados derivados de fonte externa**: resultados externos ficam como
   `EXTERNAL INPUT` até validação; **nunca** auto-gravados em conhecimento/memória assinada sem
   um registro de proveniência assinado.
4. **Anti-confirmação já presente** (busca evidência contra a hipótese) e **guard de causalidade**
   (correlação ≠ causa) — manter como gate, não como heurística opcional.

**Onde**: `internal/oracle/oracle.go`, `internal/oracle/search.go`, wiring no caminho de admissão
do pipeline/agent.
**Risco**: o Oráculo é um guard **semântico**, não uma máquina virtual — não contém execução.
Se usado **no lugar** do isolamento, cria falsa segurança.
**Trade-off**: autoridade semântica limpa vs. a necessidade de um isolamento OS real por trás —
**nunca** substitui (a)/(b), apenas as complementa.

---

## 3. Veredito priorizado

| Prioridade | Camada | Ação | Justificativa |
|---|---|---|---|
| **P0-1** | (b) Fechar opt-in default | **IMPLEMENTADO (2026-08-24)**: env removida dos launchers; fail-closed real (sem opt-in = exit 1) no auto-jail e no gate por-comando. Verificação ampla de `isAdminCommand`/gate → ainda **P0 pendente (2ª rodada)** | fecha a brecha central (fail-open default) |
| **P0-2** | (a) Jail WSL2 + bwrap | instalar distro não-root, bwrap, 2 zonas, air-gap | único isolamento real de FS/namespace/rede |
| **P0-3** | (d) Oráculo guard | ingress gate-first p/ todo dado externo | autoridade semântica; não regredir |
| **P1-1** | (c) Blindagem nativa | Job Object + Low-IL + mitigações + egress firewall | defense-in-depth; não substitui (a) |
| **P1-2** | execpolicy parser | interpretar `\| && ; $()` | fecha V4 (prefix-bypass) nos dois OS |
| **P1-3** | SandboxTool | registrar com jail estrito OU manter desregistrado | elimina risco latente (V2/V3) |

---

## 4. Riscos de trade-off (os 3 principais)

1. **Air-gap "no papel"** — WSL2 NAT conversa com a rede do Windows; se o egress não for
   realmente bloqueado (default route/firewall na distro), a zona Cofre "isolada" continua
   alcançando a internet → o requisito central do Cofre se perde silenciosamente, com custo
   operacional alto e latência de boot do WSL2.

2. **Fail-closed autoritativo quebra o dev loop** — ao remover o opt-in default, o `serve`
   Windows não sobe sem jail; e a re-auditoria do `isAdminCommand` precisa estar correta, senão
   troca-se um fail-open **visível** (com warning) por um **silencioso** (comando sem jail e
   sem aviso) — pior.

3. **Falsa sensação de segurança com (c)** — Job Object + Low-IL dão contenção de
   processo/recurso mas **não** bloqueiam leitura/exfiltração (`serve.env`, vault). Tratar (c)
   como a resposta à exfiltração cria uma segurança que não existe; a única contenção de
   exfiltração real é o isolamento OS/rede de (a).

---

## 5. Notas de implementação (não-alteradas — registro)

- **Vault**: a chave AES-256-GCM deriva de `COSCA_JWT_SECRET` por HKDF-SHA256 (`vault.go`). Se
  esse segredo vaza (por falta de isolamento V2/V5), **todo** o vault é decifrável — mais um
  motivo para priorizar (a)/(b).
- **`serve.env`**: no Windows a proteção é ACL NTFS (`icacls ... /inheritance:r /grant:r ...`) —
  já documentado em `deploy/README-WINDOWS.md` §8; manter e reforçar com o egress firewall.
- **WSL2**: hoje **sem distro**; o passo (a)-1 é pré-requisito. A lógica Linux (`jail_linux.go`)
  é o ativo de maior reuso.

---

> **Pendente de aprovação do Don**: Fase 2 (AppContainer vs Windows Container) para paridade
> total FS/leitura/rede, e a decisão sobre manter um opt-in declarativo (não-default) para
> trusted-dev. Aguardando gate do Don/Core antes de qualquer código (conforme learnings
> cosca-security 2026-08-24).
