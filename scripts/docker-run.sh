#!/usr/bin/env bash
# =============================================================================
# docker-run.sh — Cosca Runtime Docker Container (conveniencia)
# =============================================================================
# Uso:
#   ./docker-run.sh              Constroi e inicia o container
#   ./docker-run.sh --stop       Para e remove o container
#   ./docker-run.sh --shell      Entra no container com bash
#   ./docker-run.sh --rebuild    Rebuilda a imagem e reinicia
#   ./docker-run.sh --logs       Segue os logs do container
#   ./docker-run.sh --help       Mostra esta ajuda
# =============================================================================
set -euo pipefail

# Wrapper automatico: se o usuario nao tem acesso direto ao socket do Docker
# (grupo "docker" nao carregado na sessao), usa "sg docker -c" como fallback.
# Ex: sg docker -c "docker info"
if ! docker info >/dev/null 2>/dev/null; then
    docker() { sg docker -c "docker $(printf '%q ' "$@")"; }
fi

# --- Cores para output ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info()  { echo -e "${BLUE}[INFO]${NC}  $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

# --- Configuracoes ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_DIR="$HOME/.cosca-docker"
CONFIG_DIR="$DOCKER_DIR/config"
DATA_DIR="$DOCKER_DIR/data"
IMAGE_NAME="cosca-runtime"
CONTAINER_NAME="cosca-runtime"
JWT_SECRET_FILE="$CONFIG_DIR/jwt-secret"
CONFIG_FILE="$CONFIG_DIR/config.yaml"

# --- Funcoes ---

show_help() {
    cat << 'EOF'
Cosca Runtime — Docker Container

USO:
  ./docker-run.sh [FLAG]

FLAGS:
  (sem flags)   Constroi a imagem e inicia o container em background.
                Mostra os logs apos o inicio.
  --stop        Para e remove o container (preserva volumes).
  --shell       Abre um shell bash dentro do container.
  --rebuild     Rebuilda a imagem Docker e reinicia o container.
  --logs        Segue os logs do container (Ctrl+C para sair).
  --help        Mostra esta mensagem de ajuda.

VOLUMES PERSISTENTES:
  ~/.cosca-docker/config/   → /home/cosca/.config/cosca (config.yaml, JWT)
  ~/.cosca-docker/data/     → /home/cosca/.cosca (memoria, conhecimento, leis)

PORTAS:
  14120 → REST API + Dashboard
  14121 → Metrics (Prometheus)
  14122 → gRPC

PRIMEIRO BOOT:
  Provider configurado como "none" (modo deterministico, sem IA).
  JWT secret gerado automaticamente. Altere ~/.cosca-docker/config/config.yaml
  para configurar um provider real (openai, anthropic, ollama, etc.).

EXEMPLOS:
  ./docker-run.sh                  # Primeiro inicio
  ./docker-run.sh --logs           # Ver logs
  ./docker-run.sh --shell          # Entrar no container
  curl http://localhost:14120/health  # Verificar saude
  ./docker-run.sh --rebuild        # Rebuildar apos alteracoes
  ./docker-run.sh --stop           # Parar tudo
EOF
}

stop_container() {
    log_info "Parando container '$CONTAINER_NAME'..."
    docker stop "$CONTAINER_NAME" 2>/dev/null || true
    docker rm "$CONTAINER_NAME" 2>/dev/null || true
    log_ok "Container '$CONTAINER_NAME' parado e removido."
}

build_image() {
    log_info "Construindo imagem Docker '$IMAGE_NAME'..."
    log_info "WORKDIR: $SCRIPT_DIR"
    docker build -t "$IMAGE_NAME" -f "$SCRIPT_DIR/Dockerfile" "$SCRIPT_DIR"
    log_ok "Imagem '$IMAGE_NAME' construida com sucesso."
}

ensure_directories() {
    mkdir -p "$DOCKER_DIR" "$CONFIG_DIR" "$DATA_DIR"
    # Manifesto de integridade: criado no host antes do container subir,
    # pois o volume montado sobrescreve o diretorio da imagem.
    local audit_dir="$DATA_DIR/audit"
    local manifest="$audit_dir/memory-integrity-manifest.json"
    mkdir -p "$audit_dir"
    if [ ! -f "$manifest" ]; then
        echo '{"version":"1","algorithm":"sha256","root":".","created_at":"2026-01-01T00:00:00Z","files":[]}' > "$manifest"
        log_info "Manifesto de integridade criado em $manifest"
    fi
}

ensure_jwt_secret() {
    if [ ! -f "$JWT_SECRET_FILE" ]; then
        log_info "Gerando JWT secret aleatorio..."
        openssl rand -hex 32 > "$JWT_SECRET_FILE"
        chmod 600 "$JWT_SECRET_FILE" 2>/dev/null || true
        log_ok "JWT secret gerado em $JWT_SECRET_FILE"
    else
        log_info "JWT secret ja existe em $JWT_SECRET_FILE"
    fi
}

ensure_config() {
    if [ ! -f "$CONFIG_FILE" ]; then
        local jwt_secret
        jwt_secret="$(cat "$JWT_SECRET_FILE")"
        log_info "Criando config.yaml minimo (provider=none, deterministico)..."
        cat > "$CONFIG_FILE" << YAMLEOF
# Cosca Runtime — Configuracao Docker
# Gerado automaticamente pelo docker-run.sh no primeiro boot.
# Edite este arquivo para configurar um provider real.
# Altere provider.name de "none" para: openai, anthropic, deepseek, ollama, etc.

provider:
  name: "none"

server:
  jwt_secret: "${jwt_secret}"
YAMLEOF
        log_ok "Config criado em $CONFIG_FILE"
        log_warn "Provider = 'none' (modo deterministico, sem IA)."
        log_warn "Edite $CONFIG_FILE para configurar um provider real."
    else
        log_info "Config ja existe em $CONFIG_FILE (preservado — idempotente)."
    fi
}

start_container() {
    # Para container anterior se existir
    docker stop "$CONTAINER_NAME" 2>/dev/null || true
    docker rm "$CONTAINER_NAME" 2>/dev/null || true

    log_info "Iniciando container '$CONTAINER_NAME'..."
    local jwt_secret
    jwt_secret="$(cat "$JWT_SECRET_FILE")"
    docker run \
        --name "$CONTAINER_NAME" \
        -p 14120:14120 \
        -p 14121:14121 \
        -p 14122:14122 \
        -v "$CONFIG_DIR:/home/cosca/.config/cosca" \
        -v "$DATA_DIR:/home/cosca/.cosca" \
        -e COSCA_JWT_SECRET="$jwt_secret" \
        -e COSCA_JAILED=1 \
        -e COSCA_DEV_MODE=true \
        -d \
        "$IMAGE_NAME"

    # Aguarda o container iniciar
    log_info "Aguardando container iniciar..."
    sleep 2

    # Verifica se o container esta rodando
    if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        log_ok "Container '$CONTAINER_NAME' iniciado com sucesso."
        echo ""
        log_info "Endpoints disponiveis:"
        echo "  REST API:  http://localhost:14120"
        echo "  Health:    http://localhost:14120/health"
        echo "  Ready:     http://localhost:14120/ready"
        echo "  Metrics:   http://localhost:14121/metrics"
        echo ""
        log_info "Logs do container (Ctrl+C para sair):"
        echo "---"
        docker logs -f "$CONTAINER_NAME"
    else
        log_error "Container nao iniciou. Verificando logs..."
        docker logs "$CONTAINER_NAME" 2>/dev/null || true
        exit 1
    fi
}

shell_into_container() {
    if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        log_info "Entrando no container '$CONTAINER_NAME'..."
        docker exec -it "$CONTAINER_NAME" bash
    else
        log_error "Container '$CONTAINER_NAME' nao esta rodando."
        log_info "Inicie com: ./docker-run.sh"
        exit 1
    fi
}

show_logs() {
    if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        docker logs -f "$CONTAINER_NAME"
    else
        log_error "Container '$CONTAINER_NAME' nao esta rodando."
        log_info "Inicie com: ./docker-run.sh"
        exit 1
    fi
}

# --- Main ---

case "${1:-}" in
    --stop)
        stop_container
        ;;
    --shell)
        shell_into_container
        ;;
    --rebuild)
        stop_container
        build_image
        ensure_directories
        ensure_jwt_secret
        ensure_config
        start_container
        ;;
    --logs)
        show_logs
        ;;
    --help|-h|help)
        show_help
        ;;
    "")
        # Modo padrao: setup + build + start
        if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
            log_warn "Container '$CONTAINER_NAME' ja esta rodando."
            log_info "Use --rebuild para rebuildar, --stop para parar, --shell para entrar."
            log_info "Mostrando logs atuais..."
            docker logs -f "$CONTAINER_NAME"
            exit 0
        fi
        ensure_directories
        ensure_jwt_secret
        ensure_config
        build_image
        start_container
        ;;
    *)
        log_error "Flag desconhecida: $1"
        echo ""
        show_help
        exit 1
        ;;
esac
