# cosca-service.ps1 — Gerencia Cosca daemon como Windows Scheduled Task
param(
    [Parameter(Position=0)]
    [ValidateSet("start","stop","status","restart","install","uninstall","logs")]
    [string]$Action = "status"
)

$TaskName = "CoscaServe"
$CoscaExe = "C:\Users\Henrique\go\bin\cosca.exe"
$WorkingDir = "C:\Users\Henrique\Documents\cosca"
$LogDir = "$WorkingDir\.cosca\logs"
$LogFile = "$LogDir\cosca-serve.log"

if (!(Test-Path $LogDir)) { New-Item -ItemType Directory -Path $LogDir -Force | Out-Null }

function Start-CoscaServe {
    $existing = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if ($existing -and $existing.State -eq "Running") {
        Write-Host "[OK] Cosca serve ja esta rodando" -ForegroundColor Green
        return
    }

    # Script block simples
    $scriptBlock = "`$env:COSCA_ALLOW_NO_ROOT='1'; & '$CoscaExe' serve --data-dir '$WorkingDir\.cosca' 2>&1"

    $actionDef = New-ScheduledTaskAction `
        -Execute "powershell.exe" `
        -Argument "-NoProfile -NonInteractive -Command `"$scriptBlock`"" `
        -WorkingDirectory $WorkingDir

    $triggerDef = New-ScheduledTaskTrigger -AtLogon
    $settingsDef = New-ScheduledTaskSettingsSet `
        -AllowStartIfOnBatteries `
        -DontStopIfGoingOnBatteries `
        -ExecutionTimeLimit (New-TimeSpan -Days 365) `
        -RestartCount 3 `
        -RestartInterval (New-TimeSpan -Minutes 1)

    $principalDef = New-ScheduledTaskPrincipal -UserId "$env:USERNAME" -LogonType Interactive

    if ($existing) { Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false }

    Register-ScheduledTask `
        -TaskName $TaskName `
        -Action $actionDef `
        -Trigger $triggerDef `
        -Settings $settingsDef `
        -Principal $principalDef `
        -Description "Cosca AI Orchestration Daemon" | Out-Null

    Start-ScheduledTask -TaskName $TaskName
    Start-Sleep -Seconds 3

    $task = Get-ScheduledTask -TaskName $TaskName
    if ($task.State -eq "Running") {
        Write-Host "[OK] Cosca serve iniciado" -ForegroundColor Green
        Write-Host "  Logs: $LogFile" -ForegroundColor Cyan
        Write-Host "  Auto-start: AtLogon" -ForegroundColor Cyan
    } else {
        Write-Host "[ERRO] Falha ao iniciar" -ForegroundColor Red
    }
}

function Stop-CoscaServe {
    $task = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if ($task) {
        Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 1
        Write-Host "[OK] Cosca serve parado" -ForegroundColor Yellow
    } else {
        Write-Host "[INFO] Nenhuma task encontrada" -ForegroundColor Gray
    }
}

function Get-CoscaStatus {
    $task = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if (!$task) {
        Write-Host "[OFFLINE] Task nao registrada" -ForegroundColor Red
        Write-Host "  Execute: .\cosca-service.ps1 install" -ForegroundColor Gray
        return
    }

    Write-Host "Task: $TaskName | State: $($task.State) | LastRun: $($task.LastRunTime)" -ForegroundColor $(if ($task.State -eq "Running") {"Green"} else {"Yellow"})

    $port = Get-NetTCPConnection -LocalPort 14120 -ErrorAction SilentlyContinue
    if ($port) { Write-Host "Port 14120: LISTENING" -ForegroundColor Green }
    else { Write-Host "Port 14120: NOT LISTENING" -ForegroundColor Red }
}

function Install-CoscaService {
    Write-Host "Instalando Cosca Service..." -ForegroundColor Cyan

    $scriptBlock = "`$env:COSCA_ALLOW_NO_ROOT='1'; & '$CoscaExe' serve --data-dir '$WorkingDir\.cosca' 2>&1"

    $actionDef = New-ScheduledTaskAction `
        -Execute "powershell.exe" `
        -Argument "-NoProfile -NonInteractive -Command `"$scriptBlock`"" `
        -WorkingDirectory $WorkingDir

    $triggerDef = New-ScheduledTaskTrigger -AtLogon
    $settingsDef = New-ScheduledTaskSettingsSet `
        -AllowStartIfOnBatteries `
        -DontStopIfGoingOnBatteries `
        -ExecutionTimeLimit (New-TimeSpan -Days 365) `
        -RestartCount 3 `
        -RestartInterval (New-TimeSpan -Minutes 1)

    $principalDef = New-ScheduledTaskPrincipal -UserId "$env:USERNAME" -LogonType Interactive

    $existing = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if ($existing) { Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false }

    Register-ScheduledTask `
        -TaskName $TaskName `
        -Action $actionDef `
        -Trigger $triggerDef `
        -Settings $settingsDef `
        -Principal $principalDef `
        -Description "Cosca AI Orchestration Daemon" | Out-Null

    Write-Host "[OK] Task registrada" -ForegroundColor Green
    Write-Host "  Trigger: AtLogon (auto-start no boot)" -ForegroundColor Cyan
    Write-Host "  Execute: .\cosca-service.ps1 start" -ForegroundColor Gray
}

function Uninstall-CoscaService {
    Stop-CoscaServe
    $task = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if ($task) {
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
        Write-Host "[OK] Task removida" -ForegroundColor Yellow
    }
}

switch ($Action) {
    "start"    { Start-CoscaServe }
    "stop"     { Stop-CoscaServe }
    "status"   { Get-CoscaStatus }
    "restart"  { Stop-CoscaServe; Start-Sleep -Seconds 2; Start-CoscaServe }
    "install"  { Install-CoscaService }
    "uninstall"{ Uninstall-CoscaService }
}
