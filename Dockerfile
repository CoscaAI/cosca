# syntax=docker.io/docker/dockerfile:1

# =============================================================================
# Cosca Runtime — Docker Container
# =============================================================================
# Multi-stage build: compila o binario Go em alpine, roda em ubuntu minimal.
# Usuario nao-root (cosca). Se bubblewrap falhar (ex: userns restrito),
# o serve cai para modo no-root com warning — NAO trava.
# =============================================================================

# Stage 1: Build
# =============================================================================
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /src

# Cache das dependencias (camada separada para rebuilds rapidos)
COPY go.mod go.sum ./
RUN go mod download

# Copia o codigo fonte e compila
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /cosca ./cmd/cosca

# Stage 2: Runtime
# =============================================================================
FROM ubuntu:24.04

# Dependencias runtime: bubblewrap (jail), ca-certificates (TLS), curl (healthcheck)
RUN apt-get update && apt-get install -y --no-install-recommends \
    bubblewrap \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Usuario nao-root
RUN useradd -m -s /bin/bash cosca
USER cosca
WORKDIR /home/cosca

# Diretorios Cosca + manifesto de integridade (criado como root antes do USER)
RUN mkdir -p /home/cosca/.cosca/audit /home/cosca/.config/cosca && echo '{"version":"1","algorithm":"sha256","root":".","created_at":"2026-01-01T00:00:00Z","files":[]}' > /home/cosca/.cosca/audit/memory-integrity-manifest.json

# Copia o binario compilado
COPY --from=builder /cosca /usr/local/bin/cosca

# Portas: REST, Metrics, gRPC
EXPOSE 14120 14121 14122

# Healthcheck: endpoint /health (liveness probe) a cada 30s
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:14120/health || exit 1

# Entrypoint: sobe o serve. O provider e definido pelo config.yaml montado via volume
# (provider: "none" no primeiro boot = modo deterministico seguro, sem IA).
# O manifesto de integridade ja foi criado durante o build da imagem.
# CORS: permite o frontend Next.js (localhost:3000) acessar a API.
ENTRYPOINT ["cosca", "serve", "--host", "0.0.0.0", "--port", "14120", "--metrics-port", "14121", "--cors-origins", "http://localhost:3000"]
