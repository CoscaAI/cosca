# ADR-034: Sandbox Nativo Windows — AppContainer + Job Object (substitui a dependência de bubblewrap no host Windows)

> **Status:** Proposed (aguardando aprovação do Don) | **Owner:** cosca-architecture + cosca-security | **Last Updated:** 2026-09-01
> **Revisão:** decisão de DESIGN — NÃO implementada. Define o norte e o plano; a implementação (Onda 2) só ocorre após o gate do Don.
> **Fonte (ordem do Don, 2026-09-01):** "quero usar sandbox nativo do windows" — sem WSL2, sem VM, nativo no host.
> **Relação:** promove a camada **(c) Blindagem Nativa Host** do `docs/security/threat-model-cofre-windows.md` (P1 defense-in-depth) a **mecanismo principal de isolamento** para execução de agentes/tools no Windows. Complementa (não substitui) o auto-jail de arranque `pkg/cosca/jail_windows.go` e o gate por-comando `internal/chat/sandbox/gate*.go`.

---

## 0. Mapa honesto do estado atual (o que JÁ EXISTE — crítica antes do gap)

> **Nota de veracidade (auditada em código):** hoje, no Windows, **não existe isolamento real de execução**:

| Peça | Onde | Estado no Windows |
|---|---|---|
| `findBwrap()` retorna `""` | `internal/chat/sandbox/gate_other.go` (build `!linux`) | ✅ não há bwrap — mas cai em execução direta |
| `execDirect` / `execWithoutSandbox` | `gate.go:137,222` | ⚠️ roda `cmd /c <comando>` sem confinamento, exigindo apenas `COSCA_ALLOW_NO_ROOT=1` |
| `jailAvailable()` → `false` | `pkg/cosca/jail_windows.go:32` | ✅ fail-closed no **arranque** (ReexecInJail → jailFallback → exit 1 sem opt-in) |
| `IsAvailable()` no Gate | `gate.go:94` | ⚠️ `bwrapPath == ""` → false → **sem enforcement OS-level** |
| `Command()` / `Execute()` | `gate.go:36,120` | ⚠️ fallback direto; o `Rails` (validação de caminho) é a única camada restante |
| Limites de recurso (RLIMIT_*) | `gate_linux.go:69-85` | ❌ não existe equivalente no Windows (`gate_other.go` não os aplica) |
| Detecção de container | `jail.go:204 IsRunningInContainer()` | ✅ presente mas irrelevante p/ host Windows |

**Conclusão honesta:** o auto-jail de arranque está fail-closed (não sobe sem opt-in), mas o **gate por-comando** — a execução de tools do agente — roda **sem isolamento OS** no Windows. O `Rails` valida caminhos, mas não contém processo, memória, rede nem escrita fora do workspace.

---

## 1. Contexto / Problema

O runtime do Cosca usa **bubblewrap (bwrap)** para isolar a execução de comandos de agentes/tools no Linux: `--unshare-all` (namespaces PID/UTS/net/mount), `--ro-bind`/`--bind` (FS), `--clearenv` + `--setenv` (env), `--die-with-parent` (morte em cascata), RLIMIT_AS/FSIZE/NOFILE (recursos).

O bwrap é **Linux-only**. No Windows ele **não existe** e nunca será instalado. Resultado atual: o gate por-comando cai em execução direta — o `Don` só sobe o serve com opt-in e a execução de comandos não confiáveis **não é contida** em nível de SO (V2/V3 do threat model).

**Problema em uma frase:** o Windows precisa de um mecanismo de isolamento **nativo** (sem WSL2, sem VM, sem emulação) que entregue o mesmo resultado de segurança que o bwrap entrega no Linux — contenção de processo, memória, FS e rede — com **detecção automática** de disponibilidade.

**Decisão do Don:** sandbox nativo do Windows. O mecanismo nativo mais próximo do bwrap é **AppContainer** (isolamento por capacidade/SID, análogo de `--unshare` + `--ro-bind`) combinado com **Job Object** (limites de recurso e kill-on-close, análogo de RLIMIT_* + `--die-with-parent`).

---

## 2. Decisão — AppContainer + Job Object como o sandbox nativo do Windows

**Adotar** um `gate_windows.go` (build tag `windows`) que executa comandos dentro de um **AppContainer** (perfil de sandbox por SID) sob um **Job Object** (limites de memória/processos + kill-on-close), com **detecção automática** de disponibilidade via API do sistema (sem flag manual).

### 2.0 Princípios vinculantes (Mandamentos + LEI DO COFRE)

- **Fail-closed (LEI DO COFRE):** se AppContainer não estiver disponível (edição Windows, APIs ausentes, falha ao criar perfil), o gate **recusa** execução de `SandboxWorkspace`/`SandboxReadOnly` com erro claro — **nunca** cai em execução direta silenciosa. Apenas o modo `SandboxFull` (equivalente ao `execDirect` explícito) permanece atrás de opt-in.
- **Opt-in nunca vira default:** `COSCA_ALLOW_NO_ROOT=1` continua sendo o único caminho para execução sem sandbox — e apenas para o modo Full (dev trusted). Nenhum launcher injeta por default (postura da rodada 2026-08-24).
- **Reuso, não duplicação:** `Gate`, `NewGate`, `Execute`, `Command`, `IsAvailable`, `IsVerifiable`, `ValidatePath`, `processutil.Run` (timeouts + tree-kill) permanecem; só muda o **backend** de isolamento por plataforma (build tags), espelhando o padrão `gate_linux.go`.
- **Não tocar `internal/embed`** (cérebro ancestral, P8).
- **Detecção automática:** `isAppContainerAvailable()` checa as APIs de AppContainer no runtime (CreateAppContainerProfile/tentativa de criação de perfil efêmero) e o `Gate.IsAvailable()`/`IsVerifiable()` passam a refletir o sandbox nativo — sem config manual.
- **Segredos nunca entram:** o processo AppContainer roda com env restrito (allowlist `validateEnv`/`safeEnv` existentes); credenciais do host não são herdadas.
- **Trilha verificável (P3):** falha de sandbox ≠ resultado de comando — preserva o padrão do `execBwrap` (gate_linux.go:148-155): erro de estabelecimento é erro, não resultado.

---

## 3. Desenho Técnico

### 3.1 Mapeamento bwrap → AppContainer + Job Object

| bwrap (Linux) | Equivalente Windows | Mecanismo |
|---|---|---|
| `--unshare-all` (PID/UTS/net/mount namespaces) | **AppContainer SID** (isolamento por capacidade + low integrity) | `CreateAppContainerProfile` / `CreateAppContainerSidFromName` → `CreateProcess` com `PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES` |
| `--ro-bind <workspace> <workspace>` (read-only) | **Grant de pasta read-only no perfil AppContainer** | `AddCapability`/`AddDllDirectory` não; grants de pasta via API de perfil (ex.: `AddAppContainerProfileFolder` grant com acesso somente leitura) |
| `--bind <workspace>` (gravável) | **Grant de pasta gravável no perfil** | idem, com acesso RW somente à pasta do workspace |
| Host FS fora do workspace | **NEGADO por default** (AppContainer não concede acesso) | AppContainer: acesso ao FS só via grants explícitos; o resto do FS é inacessível por default |
| `--clearenv` + `--setenv` | **Env explícito no CreateProcess + Restricted Token** | `lpEnvironment` explícito; `CreateRestrictedToken` (opcional) para remover privilégios/SIDs |
| `--die-with-parent` | **`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`** | `CreateJobObject` + `SetInformationJobObject` → fechar handle = matar toda a árvore |
| `RLIMIT_AS` (memória) | **`JOB_OBJECT_LIMIT_PROCESS_MEMORY`** | `JOBOBJECT_EXTENDED_LIMIT_INFORMATION.BasicLimitInformation.LimitFlags` + `ProcessMemoryLimit` |
| `RLIMIT_FSIZE` (tamanho de arquivo) | **`JOB_OBJECT_LIMIT_PROCESS_MEMORY` parcial + quotas de disco por ACL** | sem equivalente direto de rlimit; mitigar via ACL do workspace + monitoramento (documentar limitação) |
| `RLIMIT_NOFILE` (FDs) | **limite de handles por processo** (heurística) | sem equivalente direto; documentar como risco residual |
| `--unshare-net` | **AppContainer sem network capability** | AppContainer não concede rede por default; loopback precisa de capability específica (`lpCapabilities` com `SID` de loopback/network) |
| `--chdir <workdir>` | `lpCurrentDirectory` no `CreateProcess` | — |
| `--proc /proc`, `--dev /dev` | — (não aplicável; AppContainer não monta pseudo-FS) | documentar: isolamento é por ACL, não por mount |

### 3.2 Arquitetura e localização do código

```
internal/chat/sandbox/
├── gate.go              (inalterado — contrato Gate, modos, Rails, processutil)
├── gate_linux.go        (build linux — bwrap existente, inalterado)
├── gate_other.go        (build !linux — passa a NÃO ser o caminho do Windows)
├── gate_windows.go      (NOVO, build windows — AppContainer + Job Object)
├── appcontainer_windows.go  (NOVO — wrappers x/sys/windows p/ perfil/SID/atributos)
├── jobobject_windows.go     (NOVO — wrappers CreateJobObject/limits/kill-on-close)
└── gate_windows_test.go     (NOVO — testes de contenção: FS fora, memória, kill-on-close)
```

Fluxo em `Gate.Execute` (Windows):
1. `mode == SandboxReadOnly | SandboxWorkspace` → se `isAppContainerAvailable()` → `execAppContainer(ctx, cmd, mode)`; senão → **erro fail-closed** (não execução direta).
2. `mode == SandboxFull` → `execDirect` com opt-in `COSCA_ALLOW_NO_ROOT=1` (como hoje).
3. `IsAvailable()` retorna `isAppContainerAvailable()`; `IsVerifiable()` idem (consistente com o contrato).

`execAppContainer`:
- Cria perfil AppContainer efêmero (`CreateAppContainerProfile` com nome único por execução) — SID derivado do nome (determinístico por chamada).
- Concede acesso somente ao workspace (RW no modo Workspace, RO no modo ReadOnly).
- Sem capability de rede por default; capability de loopback **apenas se** o comando exigir (política configurável, fail-closed default).
- Cria Job Object com `KILL_ON_JOB_CLOSE` + `PROCESS_MEMORY` (limite padrão 6 GiB, configurável via `COSCA_SANDBOX_MEMORY_MB` — mantém compatibilidade com o env do Linux) + `ACTIVE_PROCESS` (teto de processos, análogo anti-fork-bomb).
- `CreateProcess` com atributo `PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES` + env restrito (allowlist) + `lpCurrentDirectory` = workspace.
- Atribui o processo ao Job Object (`AssignProcessToJobObject`).
- Roda via `processutil.Run` (idle timeout, hard cap, tree-kill) — mesmo executor do Linux.
- Close do Job handle no cleanup → kill-on-close derruba a árvore inteira (análogo `--die-with-parent`).

### 3.3 Detecção automática (sem flag manual)

`isAppContainerAvailable()`:
1. `os.Getenv("COSCA_FORCE_JAIL")` não interfere (AppContainer é nativo, não jail de arranque).
2. Tentativa de `CreateAppContainerProfile` com nome temporário único + remoção imediata. Sucesso → disponível; falha (Win8- ausente, APIs bloqueadas por policy, etc.) → indisponível.
3. Resultado é cacheado por processo (uma vez por `NewGate`).

A detecção **não depende** de config YAML, flag de CLI nem env. Se o SO suporta, usa; se não, fail-closed.

### 3.4 Modos sandbox no Windows

| Modo | Windows | Comportamento |
|---|---|---|
| `SandboxReadOnly` | AppContainer + grant RO do workspace | comando lê workspace, não escreve (ACL RO), não alcança resto do FS, sem rede |
| `SandboxWorkspace` | AppContainer + grant RW do workspace | idem + escrita só no workspace; `internal/embed` permanece RO (grant explícito RO por cima) |
| `SandboxFull` | `execDirect` + opt-in | sem sandbox; requer `COSCA_ALLOW_NO_ROOT=1` explícito |

### 3.5 Interação com o auto-jail de arranque

- O auto-jail (`pkg/cosca/jail_windows.go` + `ReexecInJail`) **permanece como está**: no Windows, `jailAvailable()=false` → fail-closed no arranque (exit 1 sem opt-in) é uma decisão de **arranque do daemon**, não de execução de comando.
- O AppContainer/Job Object resolve a execução **por-comando** (o gate), que é onde o agente roda código. São **camadas complementares**:
  - Auto-jail: "o daemon não sobe sem postura explícita de segurança".
  - Gate AppContainer: "cada comando que o daemon executa é contido".
- **Decisão desta ADR:** manter o auto-jail fail-closed como está (não regredir a rodada 2026-08-24) e adicionar o gate nativo por-comando. O `stderr.log` do arranque deixa de ser a única barreira — cada execução passa a ter contenção real.

---

## 4. Limitações honestas (o que AppContainer NÃO faz)

1. **Não é namespace/mount:** AppContainer usa Mandatory Access Control (SID + capabilities). O host FS **permanece visível** (não é um "mundo escondido" como o bwrap); o que muda é o **acesso**: sem grant, o processo não abre arquivos fora do workspace. Riscos residuais: *path disclosure* (sabe que o caminho existe), *timing attacks*, e qualquer vazamento de caminho via erro de API.
2. **Não há `--proc`/`--dev` virtual:** o processo vê o FS real do host (com ACLs restritas), não um pseudo-FS montado. Compatibilidade com tools que esperam `/proc` não se aplica (Windows).
3. **RLIMIT_FSIZE/NOFILE sem equivalente direto:** limites de tamanho de arquivo e FDs são mitigados por ACL + limites de Job Object parciais; documentar como risco residual (disco pode ser exaurido dentro do workspace grantado se não houver quota — mitigação: teto de escrita monitorada).
4. **Rede é all-or-nothing por capability:** sem network capability o processo não acessa a rede (bom para isolamento), mas habilitar rede é via capability — não há "portas específicas" por AppContainer (isso é firewall/WFP, fora do escopo desta ADR; o egress firewall do threat model (c)-5 segue recomendado).
5. **Compatibilidade de PATH/cmd.exe:** processos AppContainer têm bordas de compatibilidade com `cmd.exe`/`powershell.exe` em PATH/hereditariedade de ambiente (citado no threat model existente). Mitigação: caminho absoluto do executável + env explícito; testar os comandos reais de tool no Onda 2.
6. **Perfil por execução tem custo:** criar/remover perfil AppContainer por comando tem custo de latência (ms). Mitigação: reuso de perfil por sessão quando seguro (sem grants acumulados por comando não confiável).

---

## 5. Testes e validação (critérios de aceite da Onda 2)

1. **Contenção de FS:** comando tenta `WriteFile` em `C:\Windows\system32\...` → **denegado** (Access Denied). Comando tenta ler `serve.env` fora do workspace → **denegado**.
2. **Contenção de memória:** comando aloca além do teto do Job Object → **morto pelo Job Object** (processo encerrado, host intacto).
3. **Kill-on-close:** comando lança árvore de processos filho e o handle do Job é fechado → **árvore inteira morta** (nenhum órfão).
4. **Fork bomb:** comando tenta criar N processos além do `ACTIVE_PROCESS` → **teto imposto**, host não é derrubado.
5. **Rede:** comando tenta `socket`/`HTTP` externo → **falha** (sem network capability). Loopback só quando habilitado explicitamente.
6. **ReadOnly vs Workspace:** no modo RO, escrita no workspace → denegada; no modo Workspace, escrita no workspace → ok, fora → denegada.
7. **Fail-closed:** simular AppContainer indisponível → `Execute` retorna **erro** (não executa direto).
8. **Regressão:** `go build ./...` + `go vet ./...` + testes existentes do sandbox verdes (`gate_test.go`, `rails_test.go`).

---

## 6. Consequências

**Positivas:**
- Isolamento real de execução no Windows — fecha V2/V3 do threat model no host.
- Nativo, sem WSL2, sem VM, sem custo de manter distro.
- Detecção automática: nenhuma ação manual do operador.
- Reuso máximo da arquitetura existente (Gate, processutil, Rails, modes).

**Negativas / trade-offs:**
- Mecanismo diferente do bwrap (MAC por SID vs namespaces) — **não é cópia 1:1**; exigirá ajustes de compat para algumas tools.
- Custo de latência por execução (criação/remoção de perfil).
- Limitações documentadas (RLIMIT_FSIZE/NOFILE, rede all-or-nothing, path disclosure residual).
- Esforço de implementação não-trivial (Onda 2: ~4 arquivos + testes) e revisão de segurança obrigatória.

**Alternativas consideradas (e rejeitadas):**
- **WSL2 + bwrap (threat model (a)):** rejeitado pelo Don — quer nativo, sem WSL2, sem VM, sem manter distro.
- **Windows Sandbox (VM leve Hyper-V):** rejeitado — é VM, mesmo critério do Don.
- **Job Object sozinho:** insuficiente — não isola FS/rede (só processo/recurso); AppContainer é necessário para a contenção FS/rede.
- **Low Integrity / Restricted Token sozinho:** insuficiente — protege integridade, não confidencialidade (threat model já documenta).

---

## 7. Plano de execução (Onda 2 — após gate do Don)

1. **cosca-architecture + cosca-security**: revisar esta ADR + threat model complementar (`docs/security/threat-model-windows-appcontainer.md`).
2. **Implementação** (`internal/chat/sandbox/`): `gate_windows.go` + `appcontainer_windows.go` + `jobobject_windows.go` + testes.
3. **cosca-qa**: validar critérios de aceite §5.
4. **Relatório final ao Don** com evidências (testes verdes + capturas de contenção).

---

> **Pendente de aprovação do Don:** gate para iniciar a Onda 2 (implementação). Enquanto isso, o ADR segue como proposta. Conforme decisão 2026-08-24, conhecimento do projeto vai para `docs/` — este documento é a fonte.
