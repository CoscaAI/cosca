# =============================================================================
# install-service.ps1 — Cosca Windows: instala o daemon 24/7 como Tarefa
# Agendada (equivalente do systemd user SEM sudo no Linux).
#
# Referencia Linux: deploy/cosca-serve.service + target `install-service`
# do Makefile. Os arquivos .service continuam validos para Linux.
#
# Opcao A (PADRAO — sem dependencia externa): Tarefa Agendada
#   - Trigger AtLogOn: inicia `cosca.exe serve` no logon do usuario atual.
#   - Roda como o proprio usuario (LogonType Interactive, RunLevel Limited):
#     NAO precisa de admin, igual ao systemd --user.
#   - Restart on failure: RestartCount 3 / RestartInterval 1 min (analogo ao
#     Restart=always + RestartSec=5 do systemd).
#   - ExecutionTimeLimit zero: daemon pode rodar 24/7 (sem o timeout default
#     de 72h que mataria o serve).
#   - MultipleInstances IgnoreNew: nunca 2 copias do serve rodando.
#   - Segredos NUNCA na definicao da tarefa: o proprio binario carrega
#     %USERPROFILE%\.config\cosca\serve.env (internal/env/load.go) — arquivo
#     protegido por ACL, analogo ao EnvironmentFile=0600 do Linux.
#
# Opcao B (alternativa — Windows Service real): NSSM. Use -ShowNssm ou veja
#   deploy/README-WINDOWS.md (secao "Opcao B: NSSM").
#
# Uso:
#   powershell -NoProfile -ExecutionPolicy Bypass -File deploy\install-service.ps1
#
# Parametros (todos opcionais):
#   -TaskName <nome>   Nome da tarefa agendada (default: cosca-serve)
#   -CoscaExe  <path>  Caminho do binario (default: %USERPROFILE%\.cosca\bin\cosca.exe)
#   -WorkingDir <dir>  WorkingDirectory da tarefa (default: %USERPROFILE%\Documents\cosca)
#   -HealthUrl <url>   Health check (default: http://127.0.0.1:14120/health)
#   -NoStart           Registra a tarefa mas NAO inicia
#   -ShowNssm          Imprime as instrucoes NSSM (Opcao B) e sai
# =============================================================================

[CmdletBinding()]
param(
    [string]$TaskName    = 'cosca-serve',
    [string]$CoscaExe    = (Join-Path $env:USERPROFILE '.cosca\bin\cosca.exe'),
    [string]$WorkingDir  = (Join-Path $env:USERPROFILE 'Documents\cosca'),
    [string]$HealthUrl   = 'http://127.0.0.1:14120/health',
    [int]$HealthTimeoutSeconds = 90,
    [switch]$NoStart,
    [switch]$ShowNssm
)

$ErrorActionPreference = 'Stop'

function Write-Step  { param([string]$Msg) Write-Host ("  >  " + $Msg) -ForegroundColor Cyan }
function Write-OK    { param([string]$Msg) Write-Host ("  [OK]   " + $Msg) -ForegroundColor Green }
function Write-Fail  { param([string]$Msg) Write-Host ("  [FAIL] " + $Msg) -ForegroundColor Red }
function Write-WarnL { param([string]$Msg) Write-Host ("  [WARN] " + $Msg) -ForegroundColor Yellow }

# ---------------------------------------------------------------------------
# Opcao B (documentada) — NSSM: Windows Service real, para quem quiser boot
# sem logon de usuario (equivalente ao systemd --user + enable-linger).
# ---------------------------------------------------------------------------
function Show-NssmGuide {
    Write-Host @'

  ============================================================
   Opcao B — Windows Service real via NSSM
  ============================================================
  NSSM (Non-Sucking Service Manager) registra o cosca como um
  servico do Windows (Service Control Manager). Requer admin UMA
  vez (registro do servico). NSSM cuida de restart automatico,
  redirecionamento de stdout/stderr para arquivos de log e boot
  sem logon de usuario (comportamento de enable-linger).

  1. Instale o NSSM: https://nssm.cc (ou: winget install NSSM)

  2. Registre o servico (PowerShell ADMINISTRADOR):
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

  3. Comandos:
       nssm start cosca-serve
       nssm stop cosca-serve
       nssm status cosca-serve
       nssm restart cosca-serve
       nssm remove cosca-serve confirm

  Nota: com NSSM o servico roda independente do logon. Escolha a
  conta com cuidado: LocalSystem NAO tem %USERPROFILE% — prefira
  a conta do Don (This account) se a config depender do home.
'@
}

if ($ShowNssm) {
    Show-NssmGuide
    exit 0
}

Write-Host "============================================================"
Write-Host " Cosca — instalacao Windows (Tarefa Agendada, sem admin)"
Write-Host "============================================================"

try {
    # ------------------------------------------------------------------
    # Passo 1 — verifica se o binario existe
    # ------------------------------------------------------------------
    Write-Step "Verificando binario: $CoscaExe"
    if (-not (Test-Path -LiteralPath $CoscaExe)) {
        Write-Fail "Binario nao encontrado: $CoscaExe"
        Write-Host ""
        Write-Host "  Para buildar e instalar (na raiz do repositorio cosca):"
        Write-Host '    go build -o bin\cosca.exe .\cmd\cosca'
        Write-Host ("    New-Item -ItemType Directory -Force -Path (Join-Path `$env:USERPROFILE '.cosca\bin') | Out-Null")
        Write-Host ("    Copy-Item -LiteralPath .\bin\cosca.exe -Destination (Join-Path `$env:USERPROFILE '.cosca\bin\cosca.exe')")
        Write-Host "  (alternativa, se tiver make + Git Bash: make install)"
        exit 1
    }
    Write-OK "Binario presente"

    # ------------------------------------------------------------------
    # Passo 2 — monta a acao: powershell.exe escondido que executa
    #          `cosca.exe serve`. EncodedCommand evita problemas de
    #          quoting no XML da tarefa. O serve.env e carregado pelo
    #          proprio binario (internal/env/load.go) — segredos nunca
    #          ficam gravados na definicao da tarefa.
    # ------------------------------------------------------------------
    Write-Step "Montando launcher da tarefa (EncodedCommand)"
    $launcher = @'
$ErrorActionPreference = 'Continue'
# Defaults espelhando o cosca-serve.service (Linux). Segredos NAO ficam
# aqui: o binario carrega %USERPROFILE%\.config\cosca\serve.env sozinho.
if (-not $env:COSCA_PIPELINE_ENABLED) { $env:COSCA_PIPELINE_ENABLED = 'true' }
if (-not $env:COSCA_ALLOW_NO_ROOT)    { $env:COSCA_ALLOW_NO_ROOT    = '1' }
try {
    & '<EXE>' serve
    $code = $LASTEXITCODE
} catch {
    [Console]::Error.WriteLine($_.Exception.Message)
    $code = 1
}
if ($null -eq $code) { $code = 1 }
exit $code
'@
    $launcher = $launcher.Replace('<EXE>', $CoscaExe)
    $encoded  = [Convert]::ToBase64String([System.Text.Encoding]::Unicode.GetBytes($launcher))

    $psExe = (Get-Command powershell.exe).Source
    $action = New-ScheduledTaskAction `
        -Execute $psExe `
        -Argument ('-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -EncodedCommand ' + $encoded) `
        -WorkingDirectory $WorkingDir `
        -Id 'CoscaServe'

    # ------------------------------------------------------------------
    # Passo 3 — trigger (logon do usuario), principal (sem admin) e
    #          settings (restart on failure + time limit ilimitado)
    # ------------------------------------------------------------------
    Write-Step "Configurando trigger / principal / settings"
    $trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
    $principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Limited
    $settings = New-ScheduledTaskSettingsSet `
        -MultipleInstances IgnoreNew `
        -ExecutionTimeLimit ([TimeSpan]::Zero) `
        -RestartCount 3 `
        -RestartInterval (New-TimeSpan -Minutes 1) `
        -StartWhenAvailable `
        -AllowStartIfOnBatteries `
        -DontStopIfGoingOnBatteries

    # ------------------------------------------------------------------
    # Passo 4 — registro idempotente (se ja existe, atualiza)
    # ------------------------------------------------------------------
    Write-Step "Registrando tarefa '$TaskName' (idempotente)"
    $existing = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if ($existing) {
        Write-WarnL "Tarefa '$TaskName' ja existe — parando e substituindo pela nova definicao"
        Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction Stop
    }
    $description = 'Cosca — Serve daemon 24/7 (REST 14120, metrics 14121, gRPC 14122). Equivalente Windows de cosca-serve.service (systemd user).'
    Register-ScheduledTask `
        -TaskName $TaskName `
        -Action $action `
        -Trigger $trigger `
        -Principal $principal `
        -Settings $settings `
        -Description $description | Out-Null
    Write-OK "Tarefa registrada: $TaskName (trigger: logon de $env:USERNAME)"

    # ------------------------------------------------------------------
    # Passo 5 — inicia a tarefa e valida o health em localhost:14120/health
    # ------------------------------------------------------------------
    if ($NoStart) {
        Write-WarnL "-NoStart definido — tarefa criada, NAO iniciada"
    } else {
        Write-Step "Iniciando tarefa..."
        Start-ScheduledTask -TaskName $TaskName
        Write-OK "Start-ScheduledTask emitido"

        Write-Step "Validando health: $HealthUrl (timeout: ${HealthTimeoutSeconds}s)"
        $deadline = (Get-Date).AddSeconds($HealthTimeoutSeconds)
        $healthy = $false
        while ((Get-Date) -lt $deadline) {
            try {
                $resp = Invoke-WebRequest -Uri $HealthUrl -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop
                if ($resp.StatusCode -eq 200) { $healthy = $true; break }
            } catch { }
            Start-Sleep -Seconds 2
        }
        if ($healthy) {
            Write-OK "Health OK: $HealthUrl"
        } else {
            Write-Fail "Health nao respondeu em ${HealthTimeoutSeconds}s"
            Write-Host "  Dica: veja o historico da tarefa:"
            Write-Host "    Get-ScheduledTaskInfo -TaskName $TaskName"
            exit 1
        }
    }

    # ------------------------------------------------------------------
    # Passo 6 — comandos uteis de start/stop/status + Opcao B
    # ------------------------------------------------------------------
    Write-Host ""
    Write-Host "  ============================================================"
    Write-Host "   Cosca instalado como Tarefa Agendada (usuario, sem admin)"
    Write-Host "  ============================================================"
    Write-Host "  Binario : $CoscaExe"
    Write-Host "  Tarefa  : $TaskName"
    Write-Host "  Config  : $(Join-Path $env:USERPROFILE '.config\cosca\config.yaml')"
    Write-Host "  Env     : $(Join-Path $env:USERPROFILE '.config\cosca\serve.env') (lido pelo binario)"
    Write-Host "  Health  : $HealthUrl"
    Write-Host ""
    Write-Host "  Comandos uteis (PowerShell):"
    Write-Host "    Iniciar   : Start-ScheduledTask  -TaskName $TaskName"
    Write-Host "    Parar     : Stop-ScheduledTask   -TaskName $TaskName"
    Write-Host "    Status    : Get-ScheduledTask    -TaskName $TaskName"
    Write-Host "    Historico : Get-ScheduledTaskInfo -TaskName $TaskName"
    Write-Host "    Reiniciar : Restart-ScheduledTask -TaskName $TaskName"
    Write-Host "    Remover   : Unregister-ScheduledTask -TaskName $TaskName -Confirm:`$false"
    Write-Host "    Rebuild+restart: powershell -NoProfile -ExecutionPolicy Bypass -File deploy\restart-service.ps1"
    Write-Host ""
    Write-Host "  Versao schtasks (cmd):"
    Write-Host "    schtasks /Run /TN $TaskName"
    Write-Host "    schtasks /End /TN $TaskName"
    Write-Host "    schtasks /Query /TN $TaskName"
    Write-Host ""
    Write-Host "  Alternativa (Opcao B) — Windows Service real via NSSM:"
    Write-Host "    powershell -NoProfile -ExecutionPolicy Bypass -File deploy\install-service.ps1 -ShowNssm"
    Write-Host "    (ou veja a secao 'Opcao B: NSSM' em deploy\README-WINDOWS.md)"
} catch {
    Write-Fail ("Falha: " + $_.Exception.Message)
    exit 1
}
