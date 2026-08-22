# Cosca no Windows — Deploy (pt-BR)

> **Status**: ativo | **Escopo**: migração Ubuntu → Windows aprovada pelo Don
> **Referência Linux**: `deploy/cosca-serve.service` + targets `install-service` e `restart` do Makefile.
> Os arquivos `.service` **continuam válidos para Linux** — este diretório agora contém o equivalente Windows ao lado.

---

## 1. Visão geral da equivalência

| Linux (systemd user, sem sudo) | Windows (Tarefa Agendada, sem admin) |
|---|---|
| `make install-service` | `deploy\install-service.ps1` |
| `cosca-serve.service` (`Restart=always`, `RestartSec=5`) | Tarefa agendada `cosca-serve` (`RestartCount 3` / `RestartInterval 1 min`) |
| `systemctl --user start/stop/status/restart cosca-serve.service` | `Start/Stop/Get/Restart-ScheduledTask -TaskName cosca-serve` |
| `make restart` | `deploy\restart-service.ps1` |
| `~/.cosca/bin/cosca` | `%USERPROFILE%\.cosca\bin\cosca.exe` |
| `~/.config/cosca/config.yaml` | `%USERPROFILE%\.config\cosca\config.yaml` |
| `~/.config/cosca/serve.env` (`EnvironmentFile=`, 0600) | `%USERPROFILE%\.config\cosca\serve.env` (lido pelo binário; proteja com ACL) |
| journald | Histórico da Tarefa Agendada (`Get-ScheduledTaskInfo`) |
| `loginctl enable-linger` | **N/A** — Tarefa Agendada roda só com o usuário logado; para boot sem logon use a Opção B (NSSM) |

A Tarefa Agendada é a **Opção A (recomendada)**: não requer dependência externa nem admin, reproduzindo as mesmas garantias do systemd user — roda como o usuário atual, inicia no logon e reinicia em caso de falha.

---

## 2. Pré-requisitos

- Windows 10/11 (PowerShell 5.1+, já incluído).
- Go instalado (para buildar): `winget install GoLang.Go` ou https://go.dev/dl.
- Nada de administrador para a Opção A.

---

## 3. Buildar o binário Windows

Na raiz do repositório:

```powershell
go build -o bin\cosca.exe .\cmd\cosca
```

Cross-compile a partir do Linux/macOS:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o dist\cosca-windows-amd64.exe .\cmd\cosca
```

Instalar no home do usuário (local do binário usado pelos scripts):

```powershell
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.cosca\bin" | Out-Null
Copy-Item -LiteralPath .\bin\cosca.exe -Destination "$env:USERPROFILE\.cosca\bin\cosca.exe"
```

(Se tiver `make` + Git Bash, `make install` também funciona.)

> O `install-service.ps1` e o `restart-service.ps1` verificam a presença do binário
> em `%USERPROFILE%\.cosca\bin\cosca.exe` e instruem o build caso falte.

---

## 4. Jaula no Windows — fail-closed preservado

**`bwrap` (Bubblewrap) não existe no Windows.** A implementação
`pkg/cosca/jail_windows.go` segue o **mesmo contrato fail-closed** do Linux:

1. `jailAvailable()` reporta **indisponível** (`bubblewrap (bwrap) is not supported on Windows`);
2. `ReexecInJail()` cai no `jailFallback`, que **emite o SECURITY WARNING**, grava no security log e
   **só continua sem sandbox com `COSCA_ALLOW_NO_ROOT=1`** (opt-in explícito);
3. **Sem o opt-in, o processo sai com `exit 1`** — nada roda sem sandbox silenciosamente.

Os scripts instalados já definem `COSCA_ALLOW_NO_ROOT=1` no launcher da tarefa — portanto, ao
iniciar, o cosca **emite o SECURITY WARNING** em stderr indicando que está rodando **sem sandbox**.

### ⚠️ Risco e recomendação

- **Risco**: sem a jaula, um workload (skill/plugin/código executado pelo pipeline) roda com as
  permissões do processo — não há isolamento de filesystem/namespace.
- **Recomendação**: o cosca mantém os defaults fail-closed (REST escuta em `127.0.0.1`, CORS
  desabilitado, registro público desabilitado). Para **código não confiável**, rode o workload em
  **Docker** (veja `Dockerfile` e `docker-compose.yml` do projeto) em vez de expor o host.

---

## 5. Instalação (Opção A — Tarefa Agendada)

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File deploy\install-service.ps1
```

O script faz, nesta ordem:

1. **Verifica** `%USERPROFILE%\.cosca\bin\cosca.exe` (se faltar, instrui `go build -o`).
2. **Cria a tarefa** `cosca-serve` com:
   - trigger **AtLogOn** do usuário atual (`Interactive`, `RunLevel Limited` — sem admin);
   - ação: `powershell.exe` oculto executando `cosca.exe serve` (via `-EncodedCommand`, sem problemas de quoting);
   - `ExecutionTimeLimit` **zero** (daemon 24/7, sem o timeout padrão de 72 h);
   - **restart on failure** (`RestartCount 3`, `RestartInterval 1 min`);
   - `MultipleInstances IgnoreNew` (nunca duas cópias do serve).
3. **Idempotente**: se a tarefa já existe, é parada e substituída pela definição nova.
4. **Inicia** a tarefa.
5. **Valida o health** em `http://127.0.0.1:14120/health`.
6. Imprime os comandos de start/stop/status.

Flags úteis: `-NoStart` (registra sem iniciar), `-ShowNssm` (imprime a Opção B), além de
`-TaskName`, `-CoscaExe`, `-WorkingDir`, `-HealthUrl`.

---

## 6. Operação (iniciar / parar / status / reiniciar)

```powershell
# Status
Get-ScheduledTask -TaskName cosca-serve
Get-ScheduledTaskInfo -TaskName cosca-serve        # histórico: last run, exit code, etc.

# Iniciar / parar / reiniciar
Start-ScheduledTask    -TaskName cosca-serve
Stop-ScheduledTask     -TaskName cosca-serve
Restart-ScheduledTask  -TaskName cosca-serve

# Remover
Unregister-ScheduledTask -TaskName cosca-serve -Confirm:$false
```

Versão `cmd` (schtasks):

```bat
schtasks /Query /TN cosca-serve
schtasks /Run  /TN cosca-serve
schtasks /End  /TN cosca-serve
```

### Rebuild + restart em 1 comando

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File deploy\restart-service.ps1
```

Semântica idêntica ao `make restart`:

1. Garante o binário novo (auto-build se `bin\cosca.exe` faltar; `-Build` força rebuild).
2. Descobre o **PID exato** que escuta na porta 14120 (`Get-NetTCPConnection`, fallback `netstat -ano`).
3. Encerra **pelo PID exato** (`Stop-Process -Id`) — **P12: nunca `taskkill /IM`**.
4. Instala com **rename atômico** (`Copy-Item` → `.cosca.exe.new` → `Move-Item -Force`).
5. Reinicia a tarefa agendada e valida o health.

Flags: `-Build`, `-SkipBuild`, `-SkipInstall`, `-Force` (encerra o processo da porta mesmo se o nome não for `cosca`).

---

## 7. Opção B — NSSM (Windows Service real)

Para quem quiser um serviço do Windows de verdade (boot **sem logon de usuário**, equivalente ao
`systemd --user` + `enable-linger`):

```powershell
winget install NSSM   # ou https://nssm.cc

# PowerShell ADMINISTRADOR (uma única vez):
$exe = "$env:USERPROFILE\.cosca\bin\cosca.exe"
nssm install cosca-serve $exe serve
nssm set cosca-serve AppDirectory "$env:USERPROFILE\Documents\cosca"
nssm set cosca-serve AppEnvironmentExtra COSCA_ALLOW_NO_ROOT=1 COSCA_PIPELINE_ENABLED=true
nssm set cosca-serve Start SERVICE_AUTO_START
nssm set cosca-serve AppExit Default Restart
nssm set cosca-serve AppRestartDelay 5000
nssm set cosca-serve AppStdout "$env:USERPROFILE\.cosca\serve.log"
nssm set cosca-serve AppStderr "$env:USERPROFILE\.cosca\serve.err.log"
nssm start cosca-serve
```

Operação: `nssm start/stop/status/restart cosca-serve`, remoção: `nssm remove cosca-serve confirm`.

> **Conta**: `LocalSystem` não tem `%USERPROFILE%` do Don — se a config depender do home,
> configure a conta do Don em "This account".

---

## 8. Configuração

- **Config principal**: `%USERPROFILE%\.config\cosca\config.yaml`
- **Segredos / env**: `%USERPROFILE%\.config\cosca\serve.env`

O binário carrega o `serve.env` **automaticamente** (`internal/env/load.go`, análogo ao
`EnvironmentFile=` do systemd). Segredos **nunca** devem ir para a definição da tarefa ou para
este repositório — no Linux o arquivo é `0600`; no Windows, restrinja com ACL do NTFS:

```powershell
icacls "$env:USERPROFILE\.config\cosca\serve.env" /inheritance:r /grant:r "$env:USERNAME:F"
```

---

## 9. Portas e health check

| Porta | Serviço | Observação |
|---|---|---|
| **14120** | REST API | `GET /health` (liveness) e `GET /v1/health` (runtime) |
| **14121** | Prometheus metrics | protegido por `COSCA_METRICS_SECRET` |
| **14122** | gRPC | Knowledge/Memory/Runtime services |

```powershell
Invoke-RestMethod http://127.0.0.1:14120/health    # esperado: 200
```

Use `127.0.0.1` (não `localhost`) nos scripts: o servidor escuta em loopback IPv4 e `localhost`
pode resolver para `::1` primeiro.

---

## 10. Limitações conhecidas (Windows vs Linux)

| Tema | Linux | Windows | Impacto |
|---|---|---|---|
| **Sinais** | `SIGTERM`/`SIGINT` → graceful shutdown | Sem sinais POSIX; `Stop-Process` = `TerminateProcess` (abrupto) | O encerramento via `restart-service.ps1` não roda o graceful shutdown. Para desligamento gracioso use `POST /v1/kernel/emergency/stop` (admin) ou Ctrl+C num terminal dedicado |
| **Restart on failure** | `Restart=always` (qualquer saída) | Só re-inicia se o exit code for **diferente de 0** | `RestartCount 3` cobre falhas de crash; saída 0 = sucesso e não re-inicia |
| **Sessão** | `systemd --user` sobrevive a logout com `linger` | Tarefa Agendada roda apenas com o usuário logado | Para 24/7 independente de logon, use NSSM (Opção B) |
| **Permissões** | `chmod`/`chown`/`NoNewPrivileges` | ACLs NTFS; sem SUID; sem `NoNewPrivileges` | Proteja config/segredos com `icacls`; o binário roda com os privilégios do usuário |
| **Separador de path** | `/` | `\` (e `/` aceito pelo Go) | Scripts PowerShell devem usar `Join-Path`/`Split-Path`, nunca concatenar com `/` |
| **File-lock (exe)** | `mv` sobre binário em uso funciona (unlink) | **Não é possível sobrescrever um `.exe` em execução** | Por isso o restart encerra o processo **antes** do `Move-Item` (rename atômico) |
| **SQLite** | locks POSIX | locks de arquivo do Windows (mesma máquina, mais restritivo entre processos) | Nunca rode dois processos no mesmo banco; o WAL (modernc.org) funciona normalmente |
| **Metrics** | endpoint idem | endpoint idem | `COSCA_METRICS_SECRET` é obrigatório fora do dev mode — mesmo comportamento |
| **Logs** | journald (`journalctl --user -u cosca-serve`) | Histórico da tarefa (`Get-ScheduledTaskInfo`) + stdout via NSSM | Para logs persistentes de aplicação, configure NSSM ou redirecione a saída |

---

## 11. Segurança (resumo)

- **Fail-closed preservado**: sem `COSCA_ALLOW_NO_ROOT=1` o binário não sobe sem sandbox no
  Windows (`exit 1` + SECURITY WARNING).
- **Segredos**: apenas em `%USERPROFILE%\.config\cosca\serve.env` (protegido por ACL), nunca na
  definição da tarefa nem em scripts.
- **Rede**: defaults `127.0.0.1` + CORS desabilitado + registro público desabilitado.
- **Código não confiável**: use Docker — o Windows não tem bwrap.
