# COSCA — Configuração de Ambiente (Windows & Linux)

> **Guia prático de como deixar o COSCA funcionando de ponta a ponta.**
> Baseado no comportamento REAL do código (P2) — os passos que realmente
> destravam e os traps que travam o boot.
> **Versão:** 1.5.0 · **Data:** 2026-09-05

---

## 1. O que o Cosca exige para funcionar

O serve (`cosca serve`) **não sobe** se faltar qualquer uma destas 3 camadas
(fail-closed — o sistema bloqueia de propósito, por proteção):

| # | Requisito | O que acontece sem ele |
|---|-----------|------------------------|
| 1 | **Family chain válida** | `FAMILY CHAIN BREACH` — boot bloqueado |
| 2 | **Jaula (sandbox)** | `SECURITY WARNING` — no Windows sem opt-in, `exit 1` |
| 3 | **JWT secret** | `COSCA_JWT_SECRET not set — refusing to start` |

---

## 2. Windows (passo a passo)

### 2.1 Pré-requisitos
- **Go 1.26+** (ou binário pré-compilado)
- **Git** (para a chain de integridade)
- **Ollama** (opcional, para embeddings/busca semântica)

### 2.2 Build
```powershell
cd C:\path\to\cosca
go build -o cosca.exe ./cmd/cosca
go build -o cosca-check.exe ./cmd/cosca-check
```

### 2.3 Inicializar
```powershell
cosca init          # cria .cosca/ + configuração
cosca doctor        # diagnóstico
```

### 2.4 Subir o serve — **a parte crítica no Windows**

No Windows **não existe `bwrap`** (a jaula). Então o serve exige o **opt-in
explícito**. Defina as variáveis de ambiente **no mesmo processo**:

```powershell
# PowerShell (definir no processo atual)
$env:COSCA_ALLOW_NO_ROOT="1"      # opt-in explícito (obrigatório no Windows)
$env:COSCA_DEV_MODE="true"        # cria admin dev + modo desenvolvimento
$env:COSCA_JWT_SECRET="SEGREDO-MUITO-FORTE-COM-MAIS-DE-32-BYTE"  # ≥32 bytes

cosca serve
```

**Persistir o JWT no `.env`** (recomendado — para não re-digitar):
```
# C:\path\to\cosca\.env   (gitignored NUNCA commitar)
COSCA_JWT_SECRET=SEGREDO-MUITO-FORTE-COM-MAIS-DE-32-BYTE
```

> **⚠️ IMPORTANTE (Windows):**
> - `COSCA_ALLOW_NO_ROOT=1` é **opt-in de segurança**. Sem ele, o serve faz
>   `exit 1` (fail-closed). Nunca injete por default em script — isso anularia
>   a proteção.
> - O aviso `SECURITY WARNING: jail unavailable` que aparece no start é
>   **esperado no Windows** — não é erro, é o aviso de que a jaula bwrap não
>   está disponível.

### 2.5 Validar
```powershell
# Em outro terminal:
curl.exe http://127.0.0.1:14120/health   # → 200
curl.exe http://127.0.0.1:14120/ready    # → {"ready":true,...}
```

---

## 3. Linux / WSL2 (passo a passo)

No Linux/WSL2, a **jaula `bwrap` existe** — então o sandbox real é aplicado
(mais seguro). O `COSCA_ALLOW_NO_ROOT` é o escape-hatch, mas em geral você
**não precisa** dele se o `bwrap` estiver instalado.

### 3.1 Instalar bwrap (Ubuntu/Debian)
```bash
sudo apt-get update && sudo apt-get install -y bubblewrap
```

### 3.2 Build e auto-jail
```bash
go build -o cosca ./cmd/cosca
cosca init
cosca doctor
```

### 3.3 Subir o serve
```bash
export COSCA_DEV_MODE=true
export COSCA_JWT_SECRET="SEGREDO-MUITO-FORTE-COM-MAIS-DE-32-BYTE"
cosca serve
```

> No Linux, o auto-jail reexecuta o binário **dentro do bwrap** (`COSCA_JAILED=1`).
> O workspace é montado em `/`. Isso é o modo **mais seguro**.

### 3.4 Se o bwrap falhar
```bash
# Opt-in explícito (desenvolvimento local, NUNCA produção):
export COSCA_ALLOW_NO_ROOT=1
cosca serve
```

---

## 4. Configuração (`config.yaml`)

O arquivo `config.yaml` (em `.cosca/`) controla o runtime:

```yaml
mode: development          # development | production
provider:
    name: ollama           # provider ativo (ollama, local)
    model: qwen2.5-coder:latest
    base_url: ""           # ex: http://localhost:11434
embedding:
    model: nomic-embed-text
    dimensions: 768
```

### Variáveis de ambiente relevantes

| Env | Efeito |
|-----|--------|
| `COSCA_ALLOW_NO_ROOT=1` | opt-in p/ rodar sem bwrap (Windows/WSL sem jaula) |
| `COSCA_DEV_MODE=true` | cria admin dev + modo desenvolvimento |
| `COSCA_JWT_SECRET` | segredo do JWT (≥32 bytes, obrigatório) |
| `COSCA_ENABLE_REGISTRATION=true` | habilita `/v1/auth/register` |
| `COSCA_LOG_LEVEL=debug` | logs detalhados |
| `COSCA_DEV=true` | modo debug verbose |
| `COSCA_ENABLE_EXTERNAL_PROVIDERS=true` | habilita cloud providers |

---

## 5. O que FAZER se algo travar (checklist)

### 5.1 O serve não sobe
```sql
1. cosca-check               # a chain está válida?
     → BREACH?  → cosca-check --sign-auto  (re-assina)
2. COSCA_JWT_SECRET setado?  → se não, defina (≥32 bytes)
3. COSCA_ALLOW_NO_ROOT=1?    → Windows (obrigatório)
```

### 5.2 A busca semântica não retorna
```powershell
cosca knowledge vectors-backfill   # re-embed chunks (escreve na fonte)
cosca db build                     # sincroniza fonte → módulos
cosca db verify                    # valida o split (deve dar "íntegro")
```

### 5.3 Algum banco estourou 100 MB
```powershell
cosca db check --gate      # identifica qual
# VACUUM (compactar — recupera freelist, não apaga dado):
#   em uma cópia, teste; se ok, aplicar no real com backup
```

---

## 6. Segurança — postura por plataforma

| Plataforma | Jaula | Postura |
|-----------|-------|---------|
| **Linux/WSL2** | bwrap real | Sandbox completo, mais seguro. Agentes rodam isolados. |
| **Windows** | bwrap indisponível | Exige `COSCA_ALLOW_NO_ROOT=1` (opt-in). Agente roda com o privilégio do processo. Para código não confiável, use WSL2 ou a zona Cofre (air-gap). |

> **Regra da casa:** nunca rode código NÃO confiável no host Windows sem a
> jaula. O caminho seguro é WSL2 + bwrap ou a zona Cofre.

---

## 7. Resumo (o "checklist destravar")

```powershell
# 1. Build
go build -o cosca.exe ./cmd/cosca

# 2. Init
cosca init

# 3. Envs (Windows)
$env:COSCA_ALLOW_NO_ROOT="1"
$env:COSCA_DEV_MODE="true"
$env:COSCA_JWT_SECRET="<segredo-32+bytes>"

# 4. Serve
cosca serve

# 5. Validar
curl http://127.0.0.1:14120/health   # 200 = funcionando
```

---

> **"Se o Cosca não sobe, não é bug — é a chain, a jaula ou o JWT te protegendo.
> Diagnosticar antes de contornar. Fail-closed é amigo."**
