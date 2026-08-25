# Kinematics — Operação da Família (Cosca v1.5.0)

> **Propósito**: registro operacional da máquina + CLI, para que uma nova sessão
> continue exatamente de onde a anterior parou. Fonte de verdade do "como usar".

## 1. A FAMÍLIA (portfólio)

| Projeto | Caminho | O que é | Estado atual |
|---------|---------|---------|--------------|
| **Cosca** | `C:\Users\Henrique\Documents\cosca` | Framework/orquestrador (53 agents, 34 engines, 28 skills, 41 depts) | v1.5.0, serve ativo |
| **cosca-code** | `C:\Users\Henrique\Documents\projects\cosca-code` | AI-native code assistant (Wails + 8 exec engines) | READ-ONLY (execGuard) |
| **cosca-trader** | `C:\Users\Henrique\Documents\projects\cosca-trader` | Plataforma crypto (OMS, risk, gate científico) | portão científico ativo |
| **HornFit** | `C:\Users\Henrique\Documents\projects\hornfit` | OS academia/condomínio (NestJS+Next+Prisma) | Docker de pé (pg/redis/minio) |
| **Unreal Projects** | `C:\Users\Henrique\Documents\Unreal Projects` | Mundo 3D/jogo (MetaHumans, ALS) | ponte Cosca↔Unreal |

## 2. ARQUITETURA DA MÁQUINA (divisão física — CRÍTICA)

| Camada | Onde vive | Porta | O que faz |
|--------|-----------|-------|-----------|
| **Serve (Cosca)** | WSL2 Ubuntu-24.04 (user `cosca`) | 14120/14121/14122 | REST API + knowledge + memory. `systemctl --user` |
| **Ollama (IA local)** | WSL2 | 11435 | Modelos locais (qwen2.5-coder:14b, nomic-embed) — ROCm/RX 6700 XT |
| **HornFit pg/redis/minio** | Docker Desktop | 5432/6379/9000 | infra do HornFit |
| **GitHub / auth** | DPAPI machine-bound | — | identidade do Don (não-token no TTY) |
| **GPU / ROCm** | SÓ no WSL | — | o CLI de hardware no Windows NÃO vê a GPU (reporta Vendor:none) |

> **Comandos seguros (WSL2)**: `wsl.exe -d Ubuntu-24.04 -u cosca -- <cmd>`
> **NUNCA** `sudo -u cosca` (erro 216/GROUP). **NUNCA** `pkill` (usar `kill -TERM`).

## 3. MÁQUINA — verificação rápida (o que checar ao acordar)

```powershell
# serve ativo? (WSL)
wsl.exe -d Ubuntu-24.04 -u cosca -- systemctl --user is-active cosca-serve
# health
wsl.exe -d Ubuntu-24.04 -u cosca -- curl -s http://127.0.0.1:14120/health
```

## 4. CLI — os 106 grupos (por categoria)

### Núcleo (estado)
`status` `doctor` `health` `version` (1.5.0) `config` `despertar` `kernel` `don`
`capability` `machine` `hardware` `runtime`

### IA / Orquestração
`run` `exec` `agent` `chat` `provider` `model` `task` (18 tasks) `eval` `gate`
`delegate` `propose` `benchmark`

### Conhecimento / Memória
`knowledge` `memory` `search` `symbols` `session` `graph` `index` `sync`
`quarantine` `conflict` `decision` `evidence`

### Criativos (o que o Don tem e não sabia)
`media` (ffmpeg) `ngraph` (node graph) `render` `world` (Unreal) `bridge`
`gpu` `provenance` `capability`

### Orçamento / Ética
`acquisition` `budget` `evidence` `quarantine` `conflict` `slop` `provenance`
`cv` `svm` `bug` `license`

## 5. MEMÓRIA SEMÂNTICA MODULAR (knowledge.db, 260MB)

- **Banco**: `.cosca/knowledge.db` (local) + `/home/cosca/cosca/.cosca/knowledge.db` (WSL, serve)
- **Tabelas chave**: `documents` (2387), `chunks` (38854), `vectors` (29000), `entities` (36659), `code_blocks`
- **Memória em 4 níveis**: `tier` = permanent / long / medium / short (via `expires_at`)
- **Busca**: `cosca knowledge search "query"` (FTS5 + BM25 + vector, camadas)
- **Leis epistemológicas**: `knowledge status` (K-01 a K-05) — verificação de claims

## 6. COMO CONTINUAR DE ONDE PAROU (próxima sessão)

1. **Despertar**: `cosca despertar` → lê identidade/estado do knowledge.db
2. **Ler este doc** → recupera o contexto operacional da máquina
3. **`knowledge status` + `doctor`** → valida o estado real
4. **`memory list` / `symbols search`** → recupera aprendizados anteriores

## 7. REGRA DE OURO (do loop de morte — não repetir)

- **Bug do parser**: auditar o INSTRUMENTO antes de culpar a ARQUITETURA.
- **Cicatriz na memória**: não consertar o passado, não criar novas cicatrizes.
- **Editar o embed** (`internal/embed/cosca`): só com ordem explícita do Don + re-assinar.
- **Memória > Velocidade**; PARAR e chamar o Don quando o loop começar.
