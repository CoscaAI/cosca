# =============================================================================
# docker-run.ps1 — Cosca Runtime Docker Container (Windows / PowerShell 5.1)
# Porta Windows do docker-run.sh — mesmos comandos, flags e comportamento.
# =============================================================================
# Uso:
#   .\docker-run.ps1              Constroi e inicia o container
#   .\docker-run.ps1 --stop       Para e remove o container
#   .\docker-run.ps1 --shell      Entra no container com bash
#   .\docker-run.ps1 --rebuild    Rebuilda a imagem e reinicia
#   .\docker-run.ps1 --logs       Segue os logs do container
#   .\docker-run.ps1 --help       Mostra esta ajuda
# =============================================================================
# Notas da porta Windows:
#   - O fallback "sg docker -c" (grupo "docker" no Linux) NAO existe no
#     Windows. O acesso ao daemon e via named pipe do Docker Desktop: aqui
#     apenas verificamos `docker info` e, se falhar, abortamos com mensagem.
#   - JWT secret: openssl rand -hex 32 -> gerado com
#     [System.Security.Cryptography.RandomNumberGenerator] (32 bytes -> 64 hex).
#   - Caminhos usam Join-Path / $HOME (equivale a ~ no bash).
# =============================================================================

$ErrorActionPreference = 'Stop'
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }

# --- Cores para output ---
$RED    = "`e[0;31m"
$GREEN  = "`e[0;32m"
$YELLOW = "`e[1;33m"
$BLUE   = "`e[0;34m"
$NC     = "`e[0m"

function log_info  { Write-Host "$BLUE[INFO]$NC  $($args -join ' ')" }
function log_ok    { Write-Host "$GREEN[OK]$NC    $($args -join ' ')" }
function log_warn  { Write-Host "$YELLOW[WARN]$NC  $($args -join ' ')" }
function log_error { Write-Host "$RED[ERROR]$NC $($args -join ' ')" }

# --- Configuracoes ---
$SCRIPT_DIR = $PSScriptRoot
if (-not $SCRIPT_DIR) { $SCRIPT_DIR = Split-Path -Parent $MyInvocation.MyCommand.Path }
$DOCKER_DIR  = Join-Path $HOME '.cosca-docker'
$CONFIG_DIR  = Join-Path $DOCKER_DIR 'config'
$DATA_DIR    = Join-Path $DOCKER_DIR 'data'
$IMAGE_NAME    = 'cosca-runtime'
$CONTAINER_NAME = 'cosca-runtime'
$JWT_SECRET_FILE = Join-Path $CONFIG_DIR 'jwt-secret'
$CONFIG_FILE     = Join-Path $CONFIG_DIR 'config.yaml'

# --- Funcoes ---

function Show-Help {
@'
Cosca Runtime — Docker Container

USO:
  .\docker-run.ps1 [FLAG]

FLAGS:
  (sem flags)   Constroi a imagem e inicia o container em background.
                Mostra os logs apos o inicio.
  --stop        Para e remove o container (preserva volumes).
  --shell       Abre um shell bash dentro do container.
  --rebuild     Rebuilda a imagem Docker e reinicia o container.
  --logs        Segue os logs do container (Ctrl+C para sair).
  --help        Mostra esta mensagem de ajuda.

VOLUMES PERSISTENTES:
  $HOME\.cosca-docker\config\   → /home/cosca/.config/cosca (config.yaml, JWT)
  $HOME\.cosca-docker\data\     → /home/cosca/.cosca (memoria, conhecimento, leis)

PORTAS:
  14120 → REST API + Dashboard
  14121 → Metrics (Prometheus)
  14122 → gRPC

PRIMEIRO BOOT:
  Provider configurado como "none" (modo deterministico, sem IA).
  JWT secret gerado automaticamente. Altere $HOME\.cosca-docker\config\config.yaml
  para configurar um provider real (openai, anthropic, ollama, etc.).

EXEMPLOS:
  .\docker-run.ps1                  # Primeiro inicio
  .\docker-run.ps1 --logs           # Ver logs
  .\docker-run.ps1 --shell          # Entrar no container
  curl http://localhost:14120/health  # Verificar saude
  .\docker-run.ps1 --rebuild        # Rebuildar apos alteracoes
  .\docker-run.ps1 --stop           # Parar tudo
'@
}

function Stop-Container {
    log_info "Parando container '$CONTAINER_NAME'..."
    $null = docker stop $CONTAINER_NAME 2>$null
    $null = docker rm $CONTAINER_NAME 2>$null
    log_ok "Container '$CONTAINER_NAME' parado e removido."
}

function Build-Image {
    log_info "Construindo imagem Docker '$IMAGE_NAME'..."
    log_info "WORKDIR: $SCRIPT_DIR"
    docker build -t $IMAGE_NAME -f (Join-Path $SCRIPT_DIR 'Dockerfile') $SCRIPT_DIR
    if ($LASTEXITCODE -ne 0) {
        log_error "Falha ao construir a imagem."
        exit 1
    }
    log_ok "Imagem '$IMAGE_NAME' construida com sucesso."
}

function Ensure-Directories {
    New-Item -ItemType Directory -Force -Path $DOCKER_DIR, $CONFIG_DIR, $DATA_DIR | Out-Null
    # Manifesto de integridade: criado no host antes do container subir,
    # pois o volume montado sobrescreve o diretorio da imagem.
    $auditDir = Join-Path $DATA_DIR 'audit'
    $manifest = Join-Path $auditDir 'memory-integrity-manifest.json'
    New-Item -ItemType Directory -Force -Path $auditDir | Out-Null
    if (-not (Test-Path -LiteralPath $manifest)) {
        '{"version":"1","algorithm":"sha256","root":".","created_at":"2026-01-01T00:00:00Z","files":[]}' |
            Set-Content -LiteralPath $manifest -Encoding UTF8
        log_info "Manifesto de integridade criado em $manifest"
    }
}

function New-JwtSecret {
    # Equivalente a: openssl rand -hex 32  →  32 bytes aleatorios em hex (64 chars)
    $bytes = New-Object byte[] 32
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
    return -join ($bytes | ForEach-Object { $_.ToString('x2') })
}

function Ensure-JwtSecret {
    if (-not (Test-Path -LiteralPath $JWT_SECRET_FILE)) {
        log_info "Gerando JWT secret aleatorio..."
        Set-Content -LiteralPath $JWT_SECRET_FILE -Value (New-JwtSecret) -Encoding UTF8 -NoNewline
        # (chmod 600 nao se aplica no Windows — o arquivo herda a ACL do perfil do usuario)
        log_ok "JWT secret gerado em $JWT_SECRET_FILE"
    } else {
        log_info "JWT secret ja existe em $JWT_SECRET_FILE"
    }
}

function Ensure-Config {
    if (-not (Test-Path -LiteralPath $CONFIG_FILE)) {
        $jwtSecret = (Get-Content -LiteralPath $JWT_SECRET_FILE -Raw).Trim()
        log_info "Criando config.yaml minimo (provider=none, deterministico)..."
        @"
# Cosca Runtime — Configuracao Docker
# Gerado automaticamente pelo docker-run.ps1 no primeiro boot.
# Edite este arquivo para configurar um provider real.
# Altere provider.name de "none" para: openai, anthropic, deepseek, ollama, etc.

provider:
  name: "none"

server:
  jwt_secret: "$jwtSecret"
"@ | Set-Content -LiteralPath $CONFIG_FILE -Encoding UTF8
        log_ok "Config criado em $CONFIG_FILE"
        log_warn "Provider = 'none' (modo deterministico, sem IA)."
        log_warn "Edite $CONFIG_FILE para configurar um provider real."
    } else {
        log_info "Config ja existe em $CONFIG_FILE (preservado — idempotente)."
    }
}

function Test-ContainerRunning {
    $names = docker ps --format '{{.Names}}' 2>$null
    return (@($names) -contains $CONTAINER_NAME)
}

function Start-Container {
    # Para container anterior se existir
    $null = docker stop $CONTAINER_NAME 2>$null
    $null = docker rm $CONTAINER_NAME 2>$null

    log_info "Iniciando container '$CONTAINER_NAME'..."
    $jwtSecret = (Get-Content -LiteralPath $JWT_SECRET_FILE -Raw).Trim()
    docker run `
        --name $CONTAINER_NAME `
        -p 14120:14120 `
        -p 14121:14121 `
        -p 14122:14122 `
        -v "${CONFIG_DIR}:/home/cosca/.config/cosca" `
        -v "${DATA_DIR}:/home/cosca/.cosca" `
        -e "COSCA_JWT_SECRET=$jwtSecret" `
        -e COSCA_JAILED=1 `
        -e COSCA_DEV_MODE=true `
        -d `
        $IMAGE_NAME

    if ($LASTEXITCODE -ne 0) {
        log_error "docker run falhou."
        exit 1
    }

    # Aguarda o container iniciar
    log_info "Aguardando container iniciar..."
    Start-Sleep -Seconds 2

    # Verifica se o container esta rodando
    if (Test-ContainerRunning) {
        log_ok "Container '$CONTAINER_NAME' iniciado com sucesso."
        Write-Host ""
        log_info "Endpoints disponiveis:"
        Write-Host "  REST API:  http://localhost:14120"
        Write-Host "  Health:    http://localhost:14120/health"
        Write-Host "  Ready:     http://localhost:14120/ready"
        Write-Host "  Metrics:   http://localhost:14121/metrics"
        Write-Host ""
        log_info "Logs do container (Ctrl+C para sair):"
        Write-Host "---"
        docker logs -f $CONTAINER_NAME
    } else {
        log_error "Container nao iniciou. Verificando logs..."
        $null = docker logs $CONTAINER_NAME 2>$null
        exit 1
    }
}

function Enter-ContainerShell {
    if (Test-ContainerRunning) {
        log_info "Entrando no container '$CONTAINER_NAME'..."
        docker exec -it $CONTAINER_NAME bash
    } else {
        log_error "Container '$CONTAINER_NAME' nao esta rodando."
        log_info "Inicie com: .\docker-run.ps1"
        exit 1
    }
}

function Show-Logs {
    if (Test-ContainerRunning) {
        docker logs -f $CONTAINER_NAME
    } else {
        log_error "Container '$CONTAINER_NAME' nao esta rodando."
        log_info "Inicie com: .\docker-run.ps1"
        exit 1
    }
}

# --- Main ---
$flag = $args[0]

# --help nunca precisa do daemon (mesmo comportamento do bash: o check do
# socket so redefine docker(), nao aborta o --help).
if ($flag -eq '--help' -or $flag -eq '-h' -or $flag -eq 'help') {
    Show-Help
    exit 0
}

# --- Verificacao do daemon Docker (substitui o wrapper "sg docker" do bash) ---
$dockerOk = $true
try {
    $null = docker info 2>&1
    if ($LASTEXITCODE -ne 0) { $dockerOk = $false }
} catch {
    $dockerOk = $false
}
if (-not $dockerOk) {
    Write-Host "docker info falhou. O Docker esta instalado e o Docker Desktop esta rodando?" -ForegroundColor Red
    Write-Host "No Windows nao ha grupo 'docker' (sg docker) — o acesso e feito via named pipe do Docker Desktop." -ForegroundColor Yellow
    exit 1
}

switch ($flag) {
    '--stop' {
        Stop-Container
    }
    '--shell' {
        Enter-ContainerShell
    }
    '--rebuild' {
        Stop-Container
        Build-Image
        Ensure-Directories
        Ensure-JwtSecret
        Ensure-Config
        Start-Container
    }
    '--logs' {
        Show-Logs
    }
    { $null -eq $_ -or $_ -eq '' } {
        # Modo padrao (sem flag): setup + build + start
        if (Test-ContainerRunning) {
            log_warn "Container '$CONTAINER_NAME' ja esta rodando."
            log_info "Use --rebuild para rebuildar, --stop para parar, --shell para entrar."
            log_info "Mostrando logs atuais..."
            docker logs -f $CONTAINER_NAME
            exit 0
        }
        Ensure-Directories
        Ensure-JwtSecret
        Ensure-Config
        Build-Image
        Start-Container
    }
    default {
        log_error "Flag desconhecida: $flag"
        Write-Host ""
        Show-Help
        exit 1
    }
}
