# =============================================================================
# restart-service.ps1 — Cosca Windows: garante o binario novo, encerra o serve
# pelo PID EXATO da porta, instala com rename atomico e reinicia a Tarefa
# Agendada.
#
# Equivalente Windows de: make restart (Makefile).
#
# P12 (principio do Makefile): NUNCA matar por imagem (taskkill /IM cosca.exe
# ou Stop-Process -Name). Sempre descobrir o PID EXATO pelo dono da porta e
# encerrar por esse PID.
#
# Fluxo (mesma semantica do make restart):
#   1. Garante o binario novo (auto-build se faltar; -Build para forcar).
#   2. Descobre o PID que escuta na porta 14120 (Get-NetTCPConnection, com
#      fallback para netstat -ano).
#   3. Encerra pelo PID exato e aguarda a porta liberar.
#   4. Instala com rename atomico (Copy-Item -> .cosca.exe.new ->
#      Move-Item -Force), o mesmo padrao do Makefile (cp .cosca.new + mv).
#   5. Reinicia a tarefa agendada e valida o health em /health.
#
# Uso:
#   powershell -NoProfile -ExecutionPolicy Bypass -File deploy\restart-service.ps1
#
# Flags:
#   -Build        Rebuilda bin\cosca.exe (go build -o bin\cosca.exe .\cmd\cosca)
#   -SkipBuild    Nao builda automaticamente se o binario de origem faltar
#   -SkipInstall  Reinicia a tarefa sem atualizar o binario instalado
#   -Force        Encerra o processo da porta 14120 mesmo se o nome nao for cosca
# =============================================================================

[CmdletBinding()]
param(
    [string]$TaskName   = 'cosca-serve',
    [int]$Port          = 14120,
    [string]$HealthUrl  = 'http://127.0.0.1:14120/health',
    [string]$InstallDir = (Join-Path $env:USERPROFILE '.cosca\bin'),
    [string]$SourceExe  = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..\bin\cosca.exe')),
    [int]$StopTimeoutSeconds   = 30,
    [int]$HealthTimeoutSeconds = 120,
    [switch]$Build,
    [switch]$SkipBuild,
    [switch]$SkipInstall,
    [switch]$Force
)

$ErrorActionPreference = 'Stop'
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))

function Write-Step  { param([string]$Msg) Write-Host ("  >  " + $Msg) -ForegroundColor Cyan }
function Write-OK    { param([string]$Msg) Write-Host ("  [OK]   " + $Msg) -ForegroundColor Green }
function Write-Fail  { param([string]$Msg) Write-Host ("  [FAIL] " + $Msg) -ForegroundColor Red }
function Write-WarnL { param([string]$Msg) Write-Host ("  [WARN] " + $Msg) -ForegroundColor Yellow }

# Descobre o PID que escuta na porta (State Listen). P12: retorna o PID exato.
function Get-ListenerPid {
    param([int]$ListenPort)
    $conn = Get-NetTCPConnection -LocalPort $ListenPort -State Listen -ErrorAction SilentlyContinue |
        Where-Object { $_.OwningProcess -gt 0 } | Select-Object -First 1
    if ($conn) { return [int]$conn.OwningProcess }
    # Fallback: netstat -ano (parse locale-independente)
    foreach ($line in (netstat -ano)) {
        if ($line -match ('TCP\s+[^\s]+:' + $ListenPort + '\s+[^\s]+\s+LISTENING\s+(\d+)')) {
            return [int]$Matches[1]
        }
    }
    return $null
}

Write-Host "============================================================"
Write-Host " Cosca — restart (Windows)"
Write-Host "============================================================"

try {
    # ------------------------------------------------------------------
    # Passo 1 — binario de origem (bin\cosca.exe)
    # ------------------------------------------------------------------
    $go = Get-Command go -ErrorAction SilentlyContinue
    $needsBuild = $Build -or (-not (Test-Path -LiteralPath $SourceExe))
    if ($needsBuild) {
        if ($SkipBuild) { throw "Binario de origem nao encontrado: $SourceExe" }
        if (-not $go) { throw "go nao encontrado no PATH. Builda manualmente: go build -o bin\cosca.exe .\cmd\cosca" }
        Write-Step "Buildando: go build -o bin\cosca.exe .\cmd\cosca"
        Push-Location $repoRoot
        try {
            & go build -o bin\cosca.exe .\cmd\cosca
            if ($LASTEXITCODE -ne 0) { throw "go build falhou (exit $LASTEXITCODE)" }
        } finally { Pop-Location }
        $SourceExe = [System.IO.Path]::GetFullPath((Join-Path $repoRoot 'bin\cosca.exe'))
    }
    Write-OK "Binario novo: $SourceExe"

    # ------------------------------------------------------------------
    # Passo 2 — PID EXATO na porta (P12: nunca taskkill por imagem)
    # ------------------------------------------------------------------
    Write-Step "Descobrindo PID que escuta na porta $Port..."
    $procId = Get-ListenerPid -ListenPort $Port
    if ($procId) {
        $proc = Get-Process -Id $procId -ErrorAction SilentlyContinue
        if (-not $proc) {
            Write-WarnL "PID $procId listado mas processo ja nao existe — seguindo"
            $procId = $null
        } elseif ($proc.ProcessName -ne 'cosca' -and -not $Force) {
            throw "Porta $Port e ocupada por '$($proc.ProcessName)' (PID $procId), nao pelo cosca. Abortando (P12). Use -Force se for intencional."
        }
    }

    if ($procId) {
        Write-Step "Encerrando cosca (PID exato via porta $Port): $procId"
        Stop-Process -Id $procId -Force
        $deadline = (Get-Date).AddSeconds($StopTimeoutSeconds)
        while ((Get-Date) -lt $deadline) {
            $procGone = -not (Get-Process -Id $procId -ErrorAction SilentlyContinue)
            $portGone = -not (Get-ListenerPid -ListenPort $Port)
            if ($procGone -and $portGone) { break }
            Start-Sleep -Milliseconds 500
        }
        $procGone = -not (Get-Process -Id $procId -ErrorAction SilentlyContinue)
        $portGone = -not (Get-ListenerPid -ListenPort $Port)
        if (-not ($procGone -and $portGone)) {
            throw "Processo $procId nao terminou em ${StopTimeoutSeconds}s (porta $Port ainda ocupada)."
        }
        Write-OK "Processo encerrado (PID $procId)"
    } else {
        Write-WarnL "Nenhum processo escutando na porta $Port — pulando kill"
    }

    # ------------------------------------------------------------------
    # Passo 3 — instalacao atomica do binario (cp + mv), como o Makefile
    # ------------------------------------------------------------------
    if (-not $SkipInstall) {
        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
        $dest = Join-Path $InstallDir 'cosca.exe'
        $tmp  = Join-Path $InstallDir '.cosca.exe.new'
        Write-Step "Instalando binario com rename atomico -> $dest"
        Copy-Item -LiteralPath $SourceExe -Destination $tmp -Force
        $moved = $false
        for ($i = 0; $i -lt 5; $i++) {
            try {
                Move-Item -LiteralPath $tmp -Destination $dest -Force -ErrorAction Stop
                $moved = $true
                break
            } catch {
                Start-Sleep -Seconds 1
            }
        }
        if (-not $moved) {
            Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
            throw "Falha ao substituir $dest (Windows file-lock?). O processo antigo foi encerrado; tente de novo."
        }
        Write-OK "Binario atualizado: $dest"
    } else {
        Write-WarnL "-SkipInstall — binario instalado NAO foi atualizado"
    }

    # ------------------------------------------------------------------
    # Passo 4 — reinicia a tarefa agendada + health check
    # ------------------------------------------------------------------
    $task = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if (-not $task) {
        Write-WarnL "Tarefa '$TaskName' nao existe — instale primeiro: deploy\install-service.ps1"
    } else {
        Write-Step "Reiniciando tarefa '$TaskName'..."
        Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 1
        Start-ScheduledTask -TaskName $TaskName
        Write-OK "Start-ScheduledTask emitido"

        Write-Step "Aguardando health: $HealthUrl (timeout: ${HealthTimeoutSeconds}s)"
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
            Write-Fail "Health nao respondeu em ${HealthTimeoutSeconds}s — veja Get-ScheduledTaskInfo -TaskName $TaskName"
            exit 1
        }
    }
    Write-OK "Restart concluido."
} catch {
    Write-Fail ("Falha: " + $_.Exception.Message)
    exit 1
}
