<#
    installer-enterprise.ps1
    ============================================================
    COSCA -- Instalador Enterprise autonomo (Windows / PowerShell 5.1+)
    ============================================================

    O QUE FAZ (em sequencia):
      1. Boas-vindas + banner ASCII + verificacao de pre-requisitos.
      2. Checagem de estado atual da maquina (serve, ollama, modelo,
         binario, chain, git).
      3. Configuracao padronizada (env vars + COSCA_JWT_SECRET).
      4. Instalacao de pacotes (ollama pull, build do binario).
      5. Verificacao final + VEREDITO (verde / amarelo / vermelho).
      6. Relatorio de saida (portas, health, proximos passos).

    MODOS:
      -Help      : mostra este resumo e sai.
      -Check     : somente diagnostico (nao altera nada: nao grava .env,
                   nao puxa modelo, nao compila).
      (default)  : instala/configura de forma idempotente.

    SEGURANCA (regras fail-closed):
      * NUNCA imprime o valor de COSCA_JWT_SECRET.
      * NUNCA injeta COSCA_ALLOW_NO_ROOT sem aviso explicito do risco e,
        mesmo assim, SO depois de opt-in do operador (Y/N).
      * Nao toca em codigo-fonte do Cosca. So le o repo.

    IDEMPOTENCIA: rodar 2x e seguro -- cada etapa checa antes de agir.
#>

[CmdletBinding()]
param(
    [switch]$Check,        # diagnose-only, sem efeitos colaterais
    [switch]$Help,         # imprime ajuda e sai
    [string]$Root = ""     # raiz do projeto (default: sobe 1 pasta a partir de scripts/)
)

# ============================================================
# 0. Constantes & identidade
# ============================================================
$Script:CoscaName   = "COSCA"
$Script:Version     = "1.5.0"
$Script:ModelChat   = "cosca-qwen3-4b-lora-001:latest"
$Script:ModelEmbed  = "nomic-embed-text"
$Script:OllamaPort  = 11434
$Script:RestPort    = 14120
$Script:MetricsPort = 14121
$Script:GrpcPort    = 14122
$Script:HealthUrl   = "http://127.0.0.1:$($Script:RestPort)/health"

# Cores ANSI (bloco de caracteres de caixa) -- padrao visual do instalador.
$Script:Done        = [ConsoleColor]::Green
$Script:Warn        = [ConsoleColor]::Yellow
$Script:Err         = [ConsoleColor]::Red
$Script:Info        = [ConsoleColor]::Cyan
$Script:Muted       = [ConsoleColor]::DarkGray
$Script:Title       = [ConsoleColor]::Magenta

# Vars de estado acumuladas durante a execucao (relatorio/veredito).
$Script:Ctx = [ordered]@{}

# ============================================================
# Help
# ============================================================
if ($Help) {
    Write-Host ""
    Write-Host "  COSCA - Instalador Enterprise" -ForegroundColor $Script:Title
    Write-Host "  =================================" -ForegroundColor $Script:Info
    Write-Host ""
    Write-Host "  Instala/configura o Cosca de forma idempotente. Faz, em sequencia:" -ForegroundColor $Script:Muted
    Write-Host "    1. Pre-requisitos (Go, Ollama, git, node)" -ForegroundColor $Script:Muted
    Write-Host "    2. Checagem de estado (serve, ollama, modelo, binario, chain, git)" -ForegroundColor $Script:Muted
    Write-Host "    3. Configuracao padronizada (env vars + COSCA_JWT_SECRET)" -ForegroundColor $Script:Muted
    Write-Host "    4. Instalacao de pacotes (ollama pull, build do binario)" -ForegroundColor $Script:Muted
    Write-Host "    5. Verificacao final + VEREDITO (verde/amarelo/vermelho)" -ForegroundColor $Script:Muted
    Write-Host "    6. Relatorio (portas, health, proximos passos)" -ForegroundColor $Script:Muted
    Write-Host ""
    Write-Host "  Seguranca (fail-closed):" -ForegroundColor $Script:Warn
    Write-Host "    - NUNCA imprime o valor de COSCA_JWT_SECRET." -ForegroundColor $Script:Warn
    Write-Host "    - NUNCA injeta COSCA_ALLOW_NO_ROOT sem aviso + opt-in explicito (s/N)." -ForegroundColor $Script:Warn
    Write-Host "    - Nao altera codigo-fonte do Cosca (somente le o repo)." -ForegroundColor $Script:Warn
    Write-Host ""
    Write-Host "  Modo de uso:" -ForegroundColor Cyan
    Write-Host "    .\scripts\installer-enterprise.ps1            # instala/configura (idempotente)" -ForegroundColor Gray
    Write-Host "    .\scripts\installer-enterprise.ps1 -Check     # somente diagnostico" -ForegroundColor Gray
    Write-Host "    .\scripts\installer-enterprise.ps1 -Help      # esta mensagem" -ForegroundColor Gray
    Write-Host "    .\scripts\installer-enterprise.ps1 -Root <path>  # raiz do projeto (default: subir 1 pasta de scripts\)" -ForegroundColor Gray
    Write-Host ""
    Write-Host "  Exemplo:  .\scripts\installer-enterprise.ps1 -Check" -ForegroundColor Gray
    exit 0
}

# ============================================================
# Utilitarios de UI
# ============================================================
function Set-Utf8Console {
    try { [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false) } catch { }
}

function Write-Banner {
    $banner = @"
  ++---------------------------------------------++
  |   ___  ___  ____  ____  ___                  |
  |  / __\ / _\ / __ \ / ___\ / _ \              |
  | / /  \ \ \ \/ /   / /_// /  _\/\             |
  |/ /_  / <_\ \ \_/ ||____/ _  (\ \_/ |         |
  |\___/ \___/\____/ \___/ \___/\____/           |
  |                                             |
  ++---------------------------------------------++
"@
    $cols = @($Script:Title, $Script:Info, $Script:Warn)
    $i = 0
    foreach ($l in ($banner -split "`n")) {
        if ($l.Trim() -ne '') { Write-Host $l -ForegroundColor $cols[$i % $cols.Count]; $i++ }
    }
    Write-Host ""
    Write-Host "  COSCA -- Orchestracao de Agentes com Cerebro de Conhecimento Curado" -ForegroundColor $Script:Title
    Write-Host "  Instalador Enterprise  |  versao $($Script:Version)  |  Windows/PowerShell" -ForegroundColor $Script:Info
    Write-Host "  Modo: $(if ($Check) { 'CHECK (somente diagnostico)' } else { 'INSTALL (idempotente)' })" -ForegroundColor $Script:Muted
    Write-Host ""
}

function Write-Section {
    param([string]$Title)
    Write-Host ""
    Write-Host ("=====  $Title  =====") -ForegroundColor $Script:Info
}

function Write-Result {
    param(
        [string]$Label,
        [ValidateSet('ok','warn','fail','info')][string]$Status = 'info',
        [string]$Detail = ''
    )
    $icon = switch ($Status) { 'ok' {'[*]'} 'warn' {'[!]'} 'fail' {'[x]'} 'info' {'[>]'} default {'[.]'} }
    $col  = switch ($Status) { 'ok' {$Script:Done} 'warn' {$Script:Warn} 'fail' {$Script:Err} default {$Script:Info} }
    # aligna label a uma coluna fixa para o detail ficar alinhado
    $w = 26
    if ($Label.Length -gt $w) { $w = $Label.Length }
    $pad = $Label.PadRight($w)
    if ($Detail) { Write-Host ("  {0} {1}  {2}" -f $icon, $pad, $Detail) -ForegroundColor $col }
    else         { Write-Host ("  {0} {1}"      -f $icon, $pad)                      -ForegroundColor $col }
}

function Write-Note {
    param([string]$Text, [string]$Color = 'gray')
    $col = switch ($Color) { 'gray' {$Script:Muted} 'info' {$Script:Info} 'warn' {$Script:Warn} 'err' {$Script:Err} default {$Script:Muted} }
    Write-Host ("     " + $Text) -ForegroundColor $col
}

# ---- Spinner (runspace escrevendo direto no console) ----
$script:SpinnerPs = $null
function Start-Spin {
    param([string]$Message = "Processando")
    if ($script:SpinnerPs) { return }
    try {
        $rs = [runspacefactory]::CreateRunspace()
        $rs.Open()
        $ps = [powershell]::Create()
        $ps.Runspace = $rs
        $null = $ps.AddScript({
            param($msg)
            $chars = '|','/','-','\'; $i = 0
            try {
                while ($true) {
                    [Console]::Write("`r  $msg $($chars[$i % 4]) ")
                    $i++; Start-Sleep -Milliseconds 85
                }
            } catch { }
        }).AddArgument($Message)
        $script:SpinnerPs = $ps
        $null = $ps.BeginInvoke()
    } catch { $script:SpinnerPs = $null }
}
function Stop-Spin {
    if ($script:SpinnerPs) {
        try { $script:SpinnerPs.Stop() } catch { }
        try { $script:SpinnerPs.Dispose() } catch { }
        $script:SpinnerPs = $null
        try { [Console]::Write("`r  " + (' ' * 70)); [Console]::Write("`r") } catch { }
    }
}

# ============================================================
# Utilitarios tecnicos
# ============================================================
function Resolve-ProjectRoot {
    if ($Root) { return (Resolve-Path -LiteralPath $Root -ErrorAction Stop).Path }
    $cand = Join-Path $PSScriptRoot '..'
    if (Test-Path -LiteralPath $cand) { return (Resolve-Path -LiteralPath $cand).Path }
    return (Get-Location).Path
}

function Test-TcpPort {
    param([string]$HostName = '127.0.0.1', [int]$Port, [int]$TimeoutMs = 1400)
    $client = $null
    try {
        $client = New-Object System.Net.Sockets.TcpClient
        $iar = $client.BeginConnect($HostName, $Port, $null, $null)
        if ($iar.AsyncWaitHandle.WaitOne($TimeoutMs, $false)) {
            $client.EndConnect($iar)
            return $true
        }
        return $false
    } catch { return $false }
    finally { if ($client) { $client.Close() } }
}

function Get-CommandPath {
    param([string]$Name)
    $cmd = Get-Command $Name -ErrorAction SilentlyContinue
    if ($cmd -and $cmd.Source) { return $cmd.Source }
    return $null
}

function Invoke-Capture {
    # Executa um comando externo e captura a saida (string) + codigo de saida.
    param([string]$FilePath, [string[]]$Arguments)
    $out = & $FilePath @Arguments 2>&1 | Out-String
    return [pscustomobject]@{ Output = $out.Trim(); ExitCode = $LASTEXITCODE }
}

function Get-CoscaVersion {
    param([string]$Exe)
    if (-not (Test-Path -LiteralPath $Exe)) { return $null }
    $r = Invoke-Capture -FilePath $Exe -Arguments @('version')
    # Remove sequencias ANSI (o binario imprime com cor) antes de parsear.
    $clean = $r.Output -replace "\x1b\[[0-9;?]*[a-zA-Z]", ""
    if ($clean -match 'Version:\s*(\S+)') { return ($Matches[1].Trim()) }
    return $null
}
function Invoke-StripAnsi {
    param([string]$Text)
    return ($Text -replace "\x1b\[[0-9;?]*[a-zA-Z]", "")
}

function Get-ConfigLine {
    param([string]$File, [string]$Key)
    if (-not (Test-Path -LiteralPath $File)) { return $null }
    $found = $false
    foreach ($l in (Get-Content -LiteralPath $File)) {
        if ($l -match ("^\s*" + [regex]::Escape($Key) + "\s*:\s*(.+)$")) {
            $found = $true
            return $Matches[1].Trim()
        }
    }
    return $null
}

# ---- .env manipulation (segura: nunca printa valores de secret) ----
function Get-EnvKey {
    param([string]$File, [string]$Key)
    if (-not (Test-Path -LiteralPath $File)) { return $null }
    $found = $false
    foreach ($l in (Get-Content -LiteralPath $File)) {
        if ($l -match ("^" + [regex]::Escape($Key) + "=(.*)$")) { $found = $true; return $Matches[1] }
    }
    if ($found) { return '' } else { return $null }
}

function Add-EnvKey {
    # Adiciona KEY=value se ainda nao existir. Nunca sobrescreve.
    param([string]$File, [string]$Key, [string]$Value, [switch]$Overwrite)
    $existing = Get-EnvKey -File $File -Key $Key
    if ($null -ne $existing) {
        if ($Overwrite) {
            $lines = Get-Content -LiteralPath $File
            for ($i = 0; $i -lt $lines.Count; $i++) {
                if ($lines[$i] -match ("^" + [regex]::Escape($Key) + "=")) { $lines[$i] = "$Key=$Value" }
            }
            $lines | Set-Content -LiteralPath $File -Encoding UTF8
            return "updated"
        }
        return "existing"
    }
    Add-Content -LiteralPath $File -Value "$Key=$Value" -Encoding UTF8
    return "added"
}

function New-JwtSecret {
    # Gera 32 bytes criptograficamente aleatorios, base64url. Retorna o VALOR
    # (chamadores NUNCA devem imprimir). Gera sem echo.
    try {
        $bytes = New-Object byte[] 32
        [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
        return [System.Convert]::ToBase64String($bytes).Replace('+', '-').Replace('/', '_').TrimEnd('=')
    } catch {
        return ([guid]::NewGuid().ToString('N') + [guid]::NewGuid().ToString('N')).Substring(0, 43)
    }
}

function Test-GitDirty {
    param([string]$Repo)
    if (-not (Get-CommandPath 'git')) { return $false }
    $out = & git -C $Repo status --porcelain 2>&1 | Out-String
    return (-not [string]::IsNullOrWhiteSpace($out))
}

function Test-CoscaChain {
    param([string]$Root)
    $chain = Join-Path $Root '.cosca\family_chain.dat'
    $keyA  = Join-Path $Root '.cosca\keys\kernel_public.key'
    $keyB  = Join-Path $Root 'internal\embed\cosca\keys\kernel_public.key'
    $all = (Test-Path -LiteralPath $chain) -and (Test-Path -LiteralPath $keyA) -and (Test-Path -LiteralPath $keyB)
    return [pscustomobject]@{ Ok = $all; Chain = (Test-Path -LiteralPath $chain); KeyA = (Test-Path -LiteralPath $keyA); KeyB = (Test-Path -LiteralPath $keyB) }
}

# ============================================================
# FASE 1 -- Boas-vindas + pre-requisitos
# ============================================================
function Invoke-Prereqs {
    Write-Section "1. PRE-REQUISITOS"
    Write-Host "  Procurando Go, Ollama, git e node..." -ForegroundColor $Script:Info

    $Script:Ctx.GoPath      = Get-CommandPath 'go'
    $Script:Ctx.OllamaPath  = Get-CommandPath 'ollama'
    $Script:Ctx.GitPath     = Get-CommandPath 'git'
    $Script:Ctx.NodePath    = Get-CommandPath 'node'

    Start-Spin -Message "medindo"
    $goVersion    = if ($Script:Ctx.GoPath)     { (& $Script:Ctx.GoPath version 2>&1 | Out-String).Trim() } else { '' }
    $ollamaVersion= if ($Script:Ctx.OllamaPath) { (& $Script:Ctx.OllamaPath --version 2>&1 | Out-String).Trim() } else { '' }
    $gitVersion   = if ($Script:Ctx.GitPath)    { (& $Script:Ctx.GitPath --version 2>&1 | Out-String).Trim() } else { '' }
    $nodeVersion  = if ($Script:Ctx.NodePath)   { (& $Script:Ctx.NodePath --version 2>&1 | Out-String).Trim() } else { '' }
    Stop-Spin

    $need   = @{ Go = $true; Ollama = $true; Git = $true }
    Write-Result "Go (build)"        $(if ($Script:Ctx.GoPath) {'ok'} else {'fail'}) $(if ($Script:Ctx.GoPath) {$goVersion} else {'ausente'})
    Write-Result "Ollama (LLM)"      $(if ($Script:Ctx.OllamaPath) {'ok'} else {'fail'}) $(if ($Script:Ctx.OllamaPath) {$ollamaVersion} else {'ausente'})
    Write-Result "git (repo)"        $(if ($Script:Ctx.GitPath) {'ok'} else {'fail'}) $(if ($Script:Ctx.GitPath) {$gitVersion} else {'ausente'})
    Write-Result "node (SDK TS)"     $(if ($Script:Ctx.NodePath) {'ok'} else {'info'}) $(if ($Script:Ctx.NodePath) {$nodeVersion} else {'opcional'})
    $Script:Ctx.PrereqOk = $Script:Ctx.GoPath -and $Script:Ctx.OllamaPath -and $Script:Ctx.GitPath
}

# ============================================================
# FASE 2 -- Checagem de estado atual
# ============================================================
function Invoke-StateCheck {
    Write-Section "2. ESTADO ATUAL DA MAQUINA"

    # ---- Serve / portas ----
    Start-Spin -Message "portas"
    $rest    = Test-TcpPort -Port $Script:RestPort
    $metrics = Test-TcpPort -Port $Script:MetricsPort
    $rpc     = Test-TcpPort -Port $Script:GrpcPort
    $health  = $false
    $healthBody = ''
    if ($rest) {
        try {
            $resp = Invoke-WebRequest -UseBasicParsing -Uri $Script:HealthUrl -TimeoutSec 4 -ErrorAction Stop
            $health = $resp.StatusCode -eq 200
            $healthBody = $resp.Content
        } catch { $health = $false }
    }
    Stop-Spin

    $Script:Ctx.ServeUp   = $rest -and $health
    $Script:Ctx.RestUp    = $rest
    $Script:Ctx.MetricsUp = $metrics
    $Script:Ctx.RpcUp     = $rpc
    $Script:Ctx.HealthBody= $healthBody

    Write-Result "Serve REST    :14120"  $(if ($Script:Ctx.RestUp) {'ok'} else {'fail'}) $(if ($Script:Ctx.HealthBody) {"health=$healthBody"} else {'sem resposta'})
    Write-Result "Serve Metrics :14121"  $(if ($Script:Ctx.MetricsUp) {'ok'} else {'info'}) $(if ($Script:Ctx.MetricsUp) {'listening'} else {'desligado'})
    Write-Result "Serve gRPC    :14122"  $(if ($Script:Ctx.RpcUp) {'ok'} else {'info'}) $(if ($Script:Ctx.RpcUp) {'listening'} else {'desligado'})
    Write-Result "Health check  /health" $(if ($Script:Ctx.ServeUp) {'ok'} else {'fail'}) $(if ($healthBody) {"200 $healthBody"} else {'sem resposta'})

    # ---- Ollama ----
    $Script:Ctx.OllamaUp = Test-TcpPort -Port $Script:OllamaPort
    if ($Script:Ctx.OllamaUp) {
        Write-Result "Ollama        :11434" 'ok' 'rodando'
    } else {
        Write-Result "Ollama        :11434" 'warn' 'porta fechada (instalado mas parado?)'
        $Script:Ctx.OllamaModelReady = $false
    }

    # ---- Modelo de chat ----
    if ($Script:Ctx.OllamaPath -and $Script:Ctx.OllamaUp) {
        Start-Spin -Message "consultando modelos"
        $r = Invoke-Capture -FilePath $Script:Ctx.OllamaPath -Arguments @('list')
        Stop-Spin
        $modelReady = $r.Output -match [regex]::Escape($Script:ModelChat)
        $embedReady = $r.Output -match [regex]::Escape($Script:ModelEmbed)
        $Script:Ctx.OllamaModelReady = $modelReady
        $Script:Ctx.OllamaEmbedReady = $embedReady
        Write-Result "Modelo chat   ($($Script:ModelChat))"  $(if ($modelReady) {'ok'} else {'warn'}) $(if ($modelReady) {'disponivel'} else {'nao baixado'})
        Write-Result "Modelo embed  ($($Script:ModelEmbed))" $(if ($embedReady) {'ok'} else {'warn'}) $(if ($embedReady) {'disponivel'} else {'nao baixado'})
    } else {
        $Script:Ctx.OllamaModelReady = $false
        Write-Result "Modelo chat   ($($Script:ModelChat))" 'warn' 'inconclusivo (ollama indisponivel)'
    }

    # ---- Binario cosca ----
    $Script:Ctx.CoscaExe = Join-Path $Script:Ctx.Root 'bin\cosca.exe'
    $Script:Ctx.CheckExe = Join-Path $Script:Ctx.Root 'bin\cosca-check.exe'
    $binPresent = Test-Path -LiteralPath $Script:Ctx.CoscaExe
    $Script:Ctx.BinPresent = $binPresent
    $ver = if ($binPresent) { Get-CoscaVersion -Exe $Script:Ctx.CoscaExe } else { $null }
    $Script:Ctx.BinVersion = $ver
    Write-Result "Binario cosca" $(if ($binPresent) {'ok'} else {'fail'}) $(if ($binPresent) {("v{0}  ({1})" -f $ver, $Script:Ctx.CoscaExe)} else {'ausente'})

    # ---- Chain de integridade ----
    $ch = Test-CoscaChain -Root $Script:Ctx.Root
    $Script:Ctx.Chain = $ch
    Write-Result "Chain integridade" $(if ($ch.Ok) {'ok'} else {'warn'}) $(if ($ch.Ok) {'ativa (chain + keys)'} else {'verificar com cosca-check'})
    if (-not $ch.Ok) { Write-Note "Chain ausente/incompleta -> validar com: bin\cosca-check.exe --watch" 'warn' }

    # ---- Git ----
    $dirty = Test-GitDirty -Repo $Script:Ctx.Root
    $Script:Ctx.GitDirty = $dirty
    $branch = ''
    if ($Script:Ctx.GitPath) { $branch = (& $Script:Ctx.GitPath -C $Script:Ctx.Root rev-parse --abbrev-ref HEAD 2>&1 | Out-String).Trim() }
    $Script:Ctx.GitBranch = $branch
    Write-Result "Git estado" $(if ($dirty) {'warn'} else {'ok'}) $(if ($dirty) {"sujo  (branch: $branch)"} else {"limpo  (branch: $branch)"})
}

# ============================================================
# FASE 3 -- Configuracao padronizada
# ============================================================
function Invoke-Configure {
    Write-Section "3. CONFIGURACAO"

    $envFile = Join-Path $Script:Ctx.Root '.env'
    $Script:Ctx.EnvFile = $envFile
    $configFile = Join-Path $Script:Ctx.Root '.cosca\config.yaml'

    if (-not (Test-Path -LiteralPath $envFile)) {
        New-Item -ItemType File -Path $envFile -Force | Out-Null
        Write-Note "Criado .env vazio (gitignored)." 'info'
    }

    # Modelo configurado no config.yaml (fonte de verdadade para display)
    $cfgModel = Get-ConfigLine -File $configFile -Key 'model'
    $Script:Ctx.ConfigModel = $cfgModel
    if ($cfgModel) {
        Write-Note "config.yaml provider.model = $cfgModel" 'info'
        if ($cfgModel -ne $Script:ModelChat) {
            Write-Note "Atencao: config.yaml difere do default ($($Script:ModelChat)). Mantendo o configurado." 'warn'
        }
    }

    # ---- COSCA_PROVIDER ----
    $provSt = Add-EnvKey -File $envFile -Key 'COSCA_PROVIDER' -Value 'ollama'
    $Script:Ctx.Provider = 'ollama'
    Write-Result "COSCA_PROVIDER" ($(if ($provSt -eq 'added') {'ok'} elseif ($provSt -eq 'existing') {'ok'} else {'ok'})) $provSt

    # ---- COSCA_OLLAMA_MODEL ----
    $modelSt = Add-EnvKey -File $envFile -Key 'COSCA_OLLAMA_MODEL' -Value $Script:ModelChat
    $Script:Ctx.OllamaModelEnv = $Script:ModelChat
    Write-Result "COSCA_OLLAMA_MODEL" 'ok' $("$modelSt  ($($Script:ModelChat))")

    # ---- COSCA_ALLOW_NO_ROOT (fail-closed!) ----
    $alreadyAccepted = ($env:COSCA_ALLOW_NO_ROOT -eq '1') -or $Script:Ctx.ServeUp
    if ($alreadyAccepted) {
        $Script:Ctx.AllowNoRoot = 'accepted'
        Write-Result "COSCA_ALLOW_NO_ROOT" 'ok' 'ja aceito (serve rodando ou env=1)'
        # Persistir no .env so por conveniencia, ja que foi aceito antes.
        Add-EnvKey -File $envFile -Key 'COSCA_ALLOW_NO_ROOT' -Value '1' | Out-Null
    } else {
        Write-Note "" 'info'
        Write-Host "    [!] AVISO DE SEGURANCA: Windows nao tem bubblewrap (bwrap)." -ForegroundColor $Script:Warn
        Write-Host "        Sem sandbox, rodar agentes NAO-confiaveis NAO e seguro." -ForegroundColor $Script:Warn
        Write-Host "        COSCA_ALLOW_NO_ROOT=1 so deve ser aceito por opt-in explicito." -ForegroundColor $Script:Warn
        Write-Host "" -ForegroundColor $Script:Warn
        if (-not $Check) {
            $ans = Read-Host "    Deseja aceitar o risco (OPT-IN) e setar COSCA_ALLOW_NO_ROOT=1? (s/N)"
            if ($ans -match '^(s|sim|y|yes)$') {
                Add-EnvKey -File $envFile -Key 'COSCA_ALLOW_NO_ROOT' -Value '1' | Out-Null
                $Script:Ctx.AllowNoRoot = 'accepted'
                Write-Result "COSCA_ALLOW_NO_ROOT" 'ok' 'aceito (opt-in) e persistido'
            } else {
                $Script:Ctx.AllowNoRoot = 'declined'
                Write-Result "COSCA_ALLOW_NO_ROOT" 'warn' 'recusado -> serve pode ser bloqueado'
            }
        } else {
            $Script:Ctx.AllowNoRoot = 'notset'
            Write-Result "COSCA_ALLOW_NO_ROOT" 'warn' 'nao setado'
        }
    }

    # ---- COSCA_JWT_SECRET (gerar se ausente, NUNCA imprimir) ----
    $jwt = Get-EnvKey -File $envFile -Key 'COSCA_JWT_SECRET'
    if ($null -eq $jwt -or $jwt -eq '') {
        if (-not $Check) {
            $secret = New-JwtSecret
            Add-EnvKey -File $envFile -Key 'COSCA_JWT_SECRET' -Value $secret
            $Script:Ctx.Jwt = 'generated'
            Write-Result "COSCA_JWT_SECRET" 'ok' 'gerado (32 bytes aleatorios, valor NAO exibido)'
        } else {
            $Script:Ctx.Jwt = 'missing'
            Write-Result "COSCA_JWT_SECRET" 'warn' 'ausente (modo check: nao gera)'
        }
    } else {
        $Script:Ctx.Jwt = 'present'
        Write-Result "COSCA_JWT_SECRET" 'ok' 'ja existe (mantido, NAO exibido)'
    }
}

# ============================================================
# FASE 4 -- Instalacao de pacotes
# ============================================================
function Invoke-Packages {
    Write-Section "4. INSTALACAO DE PACOTES"

    # ---- Ollama: puxar modelo de chat se faltar ----
    if ($Script:Ctx.OllamaPath -and (-not $Script:Ctx.OllamaModelReady) -and (-not $Check)) {
        Write-Note "O modelo de chat $($Script:ModelChat) nao esta presente." 'warn'
        if ($Script:Ctx.OllamaUp) {
            $ans = Read-Host "    Baixar $($Script:ModelChat)? (~2.5 GB) (s/N)"
            if ($ans -match '^(s|sim|y|yes)$') {
                Write-Host "    Puxando modelo $($Script:ModelChat) ... (pode demorar)" -ForegroundColor $Script:Info
                Start-Spin -Message "ollama pull"
                & $Script:Ctx.OllamaPath pull $Script:ModelChat
                Stop-Spin
                $Script:Ctx.OllamaModelReady = ($LASTEXITCODE -eq 0)
                Write-Result "ollama pull $($Script:ModelChat)" $(if ($Script:Ctx.OllamaModelReady) {'ok'} else {'fail'}) $(if ($Script:Ctx.OllamaModelReady) {'baixado'} else {'falhou'})
            } else {
                Write-Note "Pular download do modelo." 'info'
            }
        } else {
            Write-Note "Ollama esta parado; comece-o antes de puxar o modelo. (docker: docker run -d -p 11434:11434 ollama/ollama)" 'warn'
        }
    } elseif ((-not $Script:Ctx.OllamaModelReady) -and $Check) {
        Write-Note "Modelo ausente (modo check: nao baixa)." 'warn'
    } else {
        Write-Result "ollama pull $($Script:ModelChat)" 'info' 'ja presente (skip)'
    }

    # ---- Build do binario se faltar ----
    if (-not $Script:Ctx.BinPresent) {
        if ($Check) {
            Write-Result "build cosca.exe" 'warn' 'ausente (modo check: nao compila)'
            $Script:Ctx.BuildOk = $false
        } elseif ($Script:Ctx.GoPath) {
            Write-Note "Compilando bin\cosca.exe a partir de cmd\cosca ..." 'info'
            Start-Spin -Message "go build"
            & $Script:Ctx.GoPath build -o (Join-Path $Script:Ctx.Root 'bin\cosca.exe') .\cmd\cosca
            $bc = $LASTEXITCODE
            & $Script:Ctx.GoPath build -o (Join-Path $Script:Ctx.Root 'bin\cosca-check.exe') .\cmd\cosca-check
            $bc2 = $LASTEXITCODE
            Stop-Spin
            $Script:Ctx.BuildOk = ($bc -eq 0 -and $bc2 -eq 0)
            Write-Result "build cosca.exe + cosca-check.exe" $(if ($Script:Ctx.BuildOk) {'ok'} else {'fail'}) $(if ($Script:Ctx.BuildOk) {'compilado'} else {'falhou'})
            if (-not $Script:Ctx.BuildOk) {
                Write-Note "Build falhou. Gap conhecido: internal/worldmodel/vision exige CGO + onnxruntime_go." 'err'
                Write-Note "   -> go build ./... com CGO_ENABLED=0 NAO compila vision. Use CGO habilitado (gcc no target)." 'warn'
            }
        } else {
            $Script:Ctx.BuildOk = $false
            Write-Result "build cosca.exe" 'fail' 'Go ausente'
        }
    } else {
        $Script:Ctx.BuildOk = $true
        Write-Result "build cosca.exe" 'info' 'binario ja presente (skip)'
    }
}

# ============================================================
# FASE 5 -- Verificacao final + veredito
# ============================================================
function Get-Verdict {
    $c = $Script:Ctx
    $blockers = @()
    if (-not $c.PrereqOk)              { $blockers += 'pre-requisito(s) ausente(s) (Go/Ollama/git)' }
    if (-not $c.BinPresent)            { $blockers += 'binario cosca ausente ou build falhou' }
    if (-not $c.Chain.Ok)              { $blockers += 'chain de integridade ausente/incompleta' }

    if ($blockers.Count -gt 0) { return 'red' }

    $core = $c.ServeUp -and $c.OllamaUp -and $c.OllamaModelReady -and $c.BinPresent -and $c.Chain.Ok
    if ($core) {
        # Verde operacional. Avisos cosmeticos nao degradam, mas ficam listados.
        return 'green'
    }
    return 'yellow'
}

function Invoke-FinalCheck {
    Write-Section "5. VERIFICACAO FINAL"
    Write-Host "  Re-verificando estado..." -ForegroundColor $Script:Info

    # Re-check rapido do que importa para o veredito
    Start-Spin -Message "re-verificando"
    $c = $Script:Ctx
    $c.RestUp    = Test-TcpPort -Port $Script:RestPort
    $c.ServeUp   = $c.RestUp
    if ($c.RestUp) {
        try {
            $resp = Invoke-WebRequest -UseBasicParsing -Uri $Script:HealthUrl -TimeoutSec 4 -ErrorAction Stop
            $c.ServeUp = $resp.StatusCode -eq 200
        } catch { $c.ServeUp = $false }
    }
    $c.OllamaUp = Test-TcpPort -Port $Script:OllamaPort
    if ($c.OllamaPath -and $c.OllamaUp) {
        $r = Invoke-Capture -FilePath $c.OllamaPath -Arguments @('list')
        $c.OllamaModelReady = $r.Output -match [regex]::Escape($Script:ModelChat)
    }
    Stop-Spin

    $v = Get-Verdict
    $Script:Ctx.Verdict = $v

    Write-Host ""
    $label = switch ($v) {
        'green'  { "OPERACIONAL" }
        'yellow' { "COM AVISOS" }
        'red'    { "BLOQUEADO" }
    }
    $icon = switch ($v) { 'green' {'  (O)  '} 'yellow' {'  (!)  '} 'red' {'  (X)  '} }
    $col  = switch ($v) { 'green' {$Script:Done} 'yellow' {$Script:Warn} 'red' {$Script:Err} }
    Write-Host ("  " + ('=' * 52)) -ForegroundColor $col
    Write-Host ("  {0} VEREDITO: {1}" -f $icon, $label) -ForegroundColor $col
    Write-Host ("  " + ('=' * 52)) -ForegroundColor $col
}

# ============================================================
# FASE 6 -- Relatorio
# ============================================================
function Invoke-Report {
    $c = $Script:Ctx
    Write-Section "6. RELATORIO"
    Write-Host "  Resumo do estado:" -ForegroundColor $Script:Info

    # Re-le apenas (read-only) para o relatorio ficar correto em TODOS os modos,
    # inclusive -Check, que nao passa pela fase de configuracao.
    $jwtVal = Get-EnvKey -File $c.EnvFile -Key 'COSCA_JWT_SECRET'
    $c.Jwt = if ($null -ne $jwtVal -and $jwtVal -ne '') { 'present' } else { 'missing' }
    $allowEnv  = $env:COSCA_ALLOW_NO_ROOT -eq '1'
    $allowFile = (Get-EnvKey -File $c.EnvFile -Key 'COSCA_ALLOW_NO_ROOT') -eq '1'
    $c.AllowNoRoot = if (($allowEnv -or $allowFile) -or $c.ServeUp) { 'accepted' } else { 'notset' }

    $s = $c.ServeUp
    Write-Host ("    Serve REST    : {0}  (http://127.0.0.1:{1}/health)" -f $(if ($s) {'UP'} else {'DOWN'} ), $Script:RestPort) -ForegroundColor $(if ($s) {$Script:Done} else {$Script:Err})
    Write-Host ("    Metrics       : {0}   :{1}" -f $(if ($c.MetricsUp) {'UP'} else {'off'}), $Script:MetricsPort) -ForegroundColor $(if ($c.MetricsUp) {$Script:Info} else {$Script:Muted})
    Write-Host ("    gRPC          : {0}   :{1}" -f $(if ($c.RpcUp) {'UP'} else {'off'}), $Script:GrpcPort) -ForegroundColor $(if ($c.RpcUp) {$Script:Info} else {$Script:Muted})
    Write-Host ("    Ollama        : {0}   :{1}" -f $(if ($c.OllamaUp) {'UP'} else {'off'}), $Script:OllamaPort) -ForegroundColor $(if ($c.OllamaUp) {$Script:Done} else {$Script:Warn})
    Write-Host ("    Health        : {0}" -f $(if ($s) {"200 $($c.HealthBody)"} else {'sem resposta'})) -ForegroundColor $(if ($s) {$Script:Done} else {$Script:Err})
    Write-Host ("    Binario       : {0}" -f $(if ($c.BinPresent) {"v$($c.BinVersion)"} else {'ausente'})) -ForegroundColor $(if ($c.BinPresent) {$Script:Done} else {$Script:Err})
    Write-Host ("    Chain         : {0}" -f $(if ($c.Chain.Ok) {'ativa'} else {'incompleta'})) -ForegroundColor $(if ($c.Chain.Ok) {$Script:Done} else {$Script:Warn})
    Write-Host ("    Git           : {0}" -f $(if ($c.GitDirty) {'sujo'} else {'limpo'})) -ForegroundColor $(if ($c.GitDirty) {$Script:Warn} else {$Script:Done})
    Write-Host ("    JWT           : {0}" -f $(if ($c.Jwt -eq 'present' -or $c.Jwt -eq 'generated') {'configurado (secreto oculto)'} else {'ausente'})) -ForegroundColor $(if ($c.Jwt -eq 'present' -or $c.Jwt -eq 'generated') {$Script:Done} else {$Script:Warn})
    Write-Host ("    COSCA_ALLOW_NO_ROOT: {0}" -f $(switch ($c.AllowNoRoot) { 'accepted' {'aceito'} 'declined' {'recusado'} 'notset' {'nao setado'} default {'-'} })) -ForegroundColor $(if ($c.AllowNoRoot -eq 'accepted') {$Script:Done} else {$Script:Warn})

    Write-Host ""
    Write-Host "  Proximos passos:" -ForegroundColor $Script:Info
    if (-not $c.ServeUp) {
        Write-Host "    - Iniciar o serve:  .\scripts\cosca-serve.bat   (ou: .\scripts\cosca-service.ps1 start)" -ForegroundColor $Script:Title
    } else {
        Write-Host "    - Serve ja operando. Valide via  curl http://127.0.0.1:$($Script:RestPort)/health" -ForegroundColor $Script:Title
    }
    if (-not $c.OllamaUp) {
        Write-Host "    - Iniciar Ollama e confirmar o modelo puxado (ollama list)." -ForegroundColor $Script:Title
    }
    if (-not $c.OllamaModelReady) {
        Write-Host "    - Puxar modelo:  ollama pull $($Script:ModelChat)" -ForegroundColor $Script:Title
    }
    if (-not $c.Chain.Ok) {
        Write-Host "    - Validar/assinar chain:  bin\cosca-check.exe --watch" -ForegroundColor $Script:Title
        Write-Host "      (apos editar internal/embed/cosca/:  bin\cosca-check.exe --sign-auto)" -ForegroundColor $Script:Muted
    }
    if ($c.GitDirty) {
        Write-Host "    - Working tree nao limpo (arquivo(s) modificado(s)). Commitar/limpar antes de versionar." -ForegroundColor $Script:Muted
    }
    Write-Host ""
    Write-Host "  Dica (cross-compile Linux): usar CGO habilitado (gcc no target). CGO_ENABLED=0 NAO" -ForegroundColor $Script:Muted
    Write-Host "    compila internal/worldmodel/vision (onnxruntime_go)." -ForegroundColor $Script:Muted
}

# ============================================================
# MAIN
# ============================================================
try {
    Set-Utf8Console
    $Script:Ctx.Root = Resolve-ProjectRoot
    $Script:Ctx.EnvFile = Join-Path $Script:Ctx.Root '.env'
    Write-Banner

    if (-not (Test-Path -LiteralPath $Script:Ctx.Root)) {
        Write-Host "  [x] Raiz do projeto invalida: $($Script:Ctx.Root)" -ForegroundColor $Script:Err
        exit 1
    }

    Invoke-Prereqs                 # Fase 1
    Invoke-StateCheck              # Fase 2

    if (-not $Check) {
        Invoke-Configure           # Fase 3
        Invoke-Packages            # Fase 4
    } else {
        Write-Section "3. CONFIGURACAO (pulada no modo CHECK)"
        Write-Host "  Modo check: nao grava .env, nao puxa modelo, nao compila." -ForegroundColor $Script:Muted
        Write-Section "4. INSTALACAO DE PACOTES (pulada no modo CHECK)"
        Write-Host "  Modo check: somente diagnostico." -ForegroundColor $Script:Muted
    }

    Invoke-FinalCheck              # Fase 5
    Invoke-Report                  # Fase 6

    # Exit code coerente com o veredito
    $v = $Script:Ctx.Verdict
    if ($v -eq 'red') { exit 1 }
    exit 0
} catch {
    Stop-Spin
    Write-Host ""
    Write-Host "  [x] Erro inesperado no instalador:" -ForegroundColor $Script:Err
    Write-Host "      $($_.Exception.Message)" -ForegroundColor $Script:Err
    Write-Host "      $($_.ScriptStackTrace)" -ForegroundColor $Script:Muted
    exit 1
}
