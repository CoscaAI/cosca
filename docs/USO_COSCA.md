# Uso do Cosca (Guia de Operação)

> **Guia de USO** (comandos e fluxos) — complementa o `MANUAL_AUTOAJUDA_COSCA.md`
> (que é de metodologia de investigação). Este documento ensina **como operar o
> Cosca no dia a dia**, fiel à superfície de comandos real (`cosca --help`).
> Gera a partir do binário instalado.

---

## 0. Preâmbulo — o que é o Cosca

O Cosca é um **sistema de conhecimento + orquestração de agentes**. Não é um
"chatbot": é um **cérebro com memória, conhecimento curado, gate de validação e
orquestração de agentes/skills**. Os comandos abaixo são a superfície de operação.

> **Regra de ouro (do Don):** nada roda sozinho. O Cosca só executa com a sua
> ordem (`plan` → `approve` → `delegate` / `run`). Sem ordem, nada de automação
> espontânea.

---

## 1. Início (primeira vez em um projeto)

| Comando | O que faz |
|---|---|
| `cosca init` | Inicializa o Cosca no projeto |
| `cosca install` | Instala e configura o Cosca (fluxo completo) |
| `cosca start` | Prepara e inicia: init → install → doctor → sync → abre o editor |
| `cosca doctor` | Diagnóstico do sistema (comandos, providers, config) |
| `cosca validate` | Valida o setup do projeto |
| `cosca version` | Versão do binário |

> **Na prática:** se você acabou de clonar, rode `cosca start` (ou `cosca doctor`
> para só diagnosticar).

---

## 2. O fluxo de trabalho (a cadeia de comando)

O Cosca é uma **hierarquia**: `Don → Kernel → CEO → CTO → Chiefs → Specialists`.
O Kernel **orquestra/rota** e **nunca implementa diretamente** — delega. O fluxo de
uma tarefa:

```
propose        → submete uma proposta ao Kernel isolado (validação independente)
   ↓
plan           → estima um plano de execução ANTES da aprovação
   ↓
approve        → aprova o plano (roda testes + audit + registra)
   ↓
delegate       → delega a tarefa com o plano aprovado
   ↓
run / exec     → executa via Orchestration Engine
```

| Comando | O que faz |
|---|---|
| `cosca propose` | Submete proposta ao Kernel isolado |
| `cosca plan` | Estima um plano antes da aprovação |
| `cosca approve` | Aprova o plano (testes + audit) |
| `cosca delegate` | Delega a tarefa com plano aprovado |
| `cosca gate` | Motor de transições Gate (máquina de estados): `gate new/list/status/move/ledger` |
| `cosca run` | Executa um prompt via Orchestration Engine |
| `cosca exec` | Executa prompt não-interativo (CI/CD) |
| `cosca chat` | Sessão de chat interativa |

> **Segurança (lei do cofre):** `plan`/`approve`/`gate` são a **guarda de
> aprovação**. Nada executa sem a sua palavra. O `gate` é um "guarda de papel"
> (máquina de estados: `new → approving → approved`).

---

## 3. Conhecimento (o "cérebro" curado)

O `knowledge` é a base de conhecimento **curado** (com proveniência, claims
FACT/EVIDENCE/INFERENCE). É o coração da busca semântica.

| Comando | O que faz |
|---|---|
| `cosca knowledge search <query>` | Busca semântica (FTS + vetor + grafo + reranking) |
| `cosca knowledge graph` | Mostra o grafo de conhecimento |
| `cosca knowledge stats` | Estatísticas do conhecimento |
| `cosca knowledge rebuild` | Reconstrói o knowledge.db (índice derivado) |
| `cosca knowledge verify` | Verifica integridade do índice |
| `cosca knowledge claim` | Classifica claims (FACT/EVIDENCE/INFERENCE/...) |
| `cosca knowledge evidence add` | Adiciona evidência com proveniência (P0-P5) |
| `cosca knowledge law list` | Lista leis de conhecimento (CKL) |
| `cosca knowledge status` | Status epistêmico das leis |
| `cosca knowledge revalidate` | Revalida conhecimento contra a fonte (hash) |

> **Fluxo de conhecimento:** `evidence add` (proveniência) → `claim` (classifica a
> confiança) → indexação → `search`. O que a IA "inventa" NÃO entra direto no
> conhecimento — passa por `quarantine`.

---

## 4. Memória (o "cérebro" de curto prazo)

| Comando | O que faz |
|---|---|
| `cosca memory register` | Registra um aprendizado (fluxo automático) |
| `cosca memory list` | Lista memórias |
| `cosca memory show <id>` | Mostra uma memória |
| `cosca memory search <query>` | Busca na memória |
| `cosca memory snapshot create/list/restore` | Snapshot da memória |
| `cosca memory prune` | Expira memória velha |
| `cosca memory reindex` | Reindexa a busca na memória |
| `cosca memory stats` | Estatísticas de memória |
| `cosca memory curated-failures` | Lições de falhas curadas (P5, sanitized) |

---

## 5. Busca geral (o que você usa no dia a dia)

| Comando | O que faz |
|---|---|
| `cosca search <query>` | Busca na base de conhecimento (atalho) |
| `cosca session <query>` | Busca FTS5 nas conversas de sessão (zero LLM) |
| `cosca symbols <query>` | Busca semântica de símbolos de código Go |
| `cosca memory search <query>` | Busca na memória |
| `cosca trace <id>` | Trace ID universal + eventos (flight recorder) |

---

## 6. Operação do serviço / runtime

| Comando | O que faz |
|---|---|
| `cosca status` | Status do sistema |
| `cosca health` | Health check rápido |
| `cosca doctor` | Diagnóstico completo |
| `cosca serve` | Sobe o REST API server |
| `cosca runtime start/stop/restart/status/logs/info` | Gerencia o daemon do runtime |
| `cosca metrics` | Métricas de orquestração |
| `cosca fabric` | Status do Compute Fabric (pools, backpressure) |
| `cosca hardware` | Probe de hardware |
| `cosca machine` | Capability Profile da máquina |

> **Sobre o serve no WSL2 (Linux):** o serve roda via systemd (`cosca-serve`,
> user `cosca`). Para subir manualmente (se o auto-start não pegou):
> `wsl.exe -d Ubuntu-24.04 -u cosca -- systemctl --user start cosca-serve`.
> Health: `curl http://127.0.0.1:14120/health`.

---

## 7. Agentes, skills e orquestração

| Comando | O que faz |
|---|---|
| `cosca agent` | Gerencia os agentes (capos) |
| `cosca skill` / `cosca skills` | Gerencia o catálogo de skills (soldados) |
| `cosca pipeline` | Orquestração de pipelines |
| `cosca pipeline-test` | Testa o pipeline com mock runner (sem LLM) |
| `cosca workflow` | Gerencia workflows |
| `cosca task` | AI Task Engine (18 tarefas canônicas) |
| `cosca model` | Model Registry (modelos das 18 tasks) |
| `cosca provider` | Gerencia providers de IA |

---

## 8. Governança, segurança e integridade

| Comando | O que faz |
|---|---|
| `cosca cofre` | Fronteira de validação (Oráculo air-gap): `cofre validate/gate/health` |
| `cosca db check --gate` | Gate de tamanho por banco (100 MB — ADR-013) |
| `cosca security` | Scanner de vulnerabilidades de dependência |
| `cosca integrity` | Verifica a chain/integridade (family chain) |
| `cosca qgate` | Pre-commit quality gate (build, test, security) |
| `cosca provenance` | Proveniência (integridade + licenças) |
| `cosca don` | Proteção de identidade do Don (war phrase) |
| `cosca quarantine` | Zona de quarentena epistemológica (o que a IA inventa) |
| `cosca slop` | Fiscal anti-AI-slop |
| `cosca license` | Verifica a licença (chave de segurança) |

> **A chain da família:** se você mexer em `internal/embed/cosca/` (o cérebro),
> DEVE re-assinar a chain (`cosca-check --sign-auto`). Senão o serve NÃO sobe
> (fail-closed). **Nunca** contorne o gate de integridade.

---

## 9. Ferramentas de dados e diagnóstico

| Comando | O que faz |
|---|---|
| `cosca db check` | Inspeção/governança dos bancos (gate 100 MB) |
| `cosca cache` | Gerencia cache |
| `cosca index` | Gerencia o índice de arquivos |
| `cosca docs` | Abre a documentação |
| `cosca completion` | Gera scripts de shell completion |
| `cosca bug` | Bug Fingerprint (agrupa bugs iguais) |
| `cosca conflict` | Registro de conflitos entre fontes |
| `cosca decision` | Trilha de decisão (explicabilidade) |
| `cosca audit` | Auditoria (quando houver) |
| `cosca eval` | Benchmark do pipeline |
| `cosca benchmark` | Benchmarks de performance |
| `cosca ranking` | Ranking multi-sinal explicável (score breakdown) |
| `cosca budget` | Budget cognitivo (tokens/tempo/custo) |
| `cosca acquisition` | Orçamento de aquisição de evidência externa |
| `cosca circadian` | Ciclo de descanso operacional |

---

## 10. Mundos / Unreal / criação (o "modelo de mundo")

| Comando | O que faz |
|---|---|
| `cosca world` | World Model — inspecionar o modelo do mundo |
| `cosca bridge` | Unreal Engine bridge (WebSocket) |
| `cosca render` | Render Engine (determinístico, cacheable, resumable) |
| `cosca ngraph` | Node Graph (workflows serializáveis) |
| `cosca media` | Media Engine (vídeo/áudio via ffmpeg) |
| `cosca gpu` | GPU Engine + scheduler |
| `cosca flow` | Durable Workflow demo (replay + retry) |
| `cosca desktop` | Abre o COSCA Desktop |
| `cosca terminal` | Launch do Cosca Terminal TUI |
| `cosca voice` | Assistente de voz (ver §10a) |
| `cosca vision` | Pipeline de visão ONNX (4/4 — ver §10a) |
| `cosca screen` | Percepção de tela (OCR + zoom — ver §10a) |

### 10a. Percepção — os sentidos do Cosca

O Cosca **sente** (tudo nativo, local, sem Python em runtime). A percepção é
**por ação**: o Cosca só "olha a tela" quando você pede (ex.: "olha a tela" no
`voice chat`).

| Sentido | Comando | O que faz |
|---------|---------|-----------|
| **Ver** | `cosca vision infer img.png` | pipeline ONNX completo (CLIP/SAM2/GroundingDINO/Depth) |
| **Ver — status** | `cosca model vision` | status dos 4 modelos (4/4 present) |
| **Ouvir** | `cosca voice listen` | STT streaming (sherpa-onnx PT-BR) |
| **Falar** | `cosca voice speak "olá"` | TTS (vits-piper PT-BR), gera `.wav` |
| **Diálogo ao vivo** | `cosca voice chat` | ouve → vê (pedido) → interpreta → fala |
| **Ler tela (OCR)** | `cosca screen --ocr` | captura + lê texto, zoom bicubic 1x→2x→4x |

> Os modelos de visão vivem em `~/.cosca/models/vision/` (CLIP 466MB, SAM2 828MB,
> GroundingDINO 661MB, Depth 94MB). A **voz** exige build com
> `-tags "stt_sherpa tts_sherpa"` (CGO) + DLLs nativas em `bin/`.

> **O Cosca não constrói uma cidade — possui uma linguagem para representar
> mundos** (`WORLD → World Model → GIS | Knowledge | Unreal`). Os módulos
> `world/gis/vegetation/materials/unreal` são a expressão dessa linguagem.

---

## 11. Atualização / manutenção

| Comando | O que faz |
|---|---|
| `cosca update` | Checa e aplica updates do Cosca |
| `cosca upgrade` | Atualiza de legacy para a arquitetura atual |
| `cosca uninstall` | Desinstala do projeto |
| `cosca license` | Verifica a licença |

---

## 12. Resumo do fluxo diário (o que você faz na prática)

```bash
# 1. Entender o estado
cosca status          # sistema está de pé? serve ativo?
cosca knowledge stats # quanto conhecimento? grafo? vetores?

# 2. Buscar algo (o mais comum)
cosca search "o que e a lei da familia"
cosca memory search "conduta da chain"

# 3. Verificar integridade antes de mudar
cosca db check --gate    # bancos < 100MB?
cosca doctor             # tudo ok?

# 4. Executar uma tarefa (com a sua ordem)
cosca plan "melhorar X" | cosca approve | cosca delegate "melhorar X"

# 5. Monitorar
cosca health
cosca metrics
cosca runtime logs
```

---

## 13. Casos de erro comuns (o que fazer)

| Sintoma | O que checar primeiro |
|---|---|
| Serve não sobe | **chain desalinhada** (`git log -1` vs último bloco) → re-assinar. NUNCA contornar. |
| `sudo -u cosca` dá `216/GROUP` | Usar `-u cosca` (não sudo). `wsl -d Ubuntu-24.04 -u cosca`. |
| Busca "não acha" | Recall pode ser granularidade query→chunk, não o router. Verificar DocPath/scope. |
| Banco >100MB | `cosca db check --gate`; particionar/otimizar (Decisão 1, ADR-013). |
| Índice desatualizado | `cosca knowledge rebuild` / `cosca index rebuild`. |
| Algo não achado na doc | `cosca docs` (documentação) / `cosca help <cmd>`. |

---

## Consideração final (a filosofia do uso)

O Cosca é um **sistema de conhecimento com um RAG encaixado** — não um RAG
otimizado. Ele:
1. **Decide onde procurar** (router determinístico → scope → candidate IDs).
2. **Só então busca** (recuperação semântica confinada ao espaço roteado).
3. **Valida contra a âncora** (chain imutável — a verdade estável).
4. **Reduz o universo antes de pagar o custo** (não executa 99,9% do trabalho que
   não precisa existir).

> **O próximo passo nasce de conteúdo real, não de ansiedade de continuar.** A
> Fatia 3 (mapeamento semântico módulo→domínio) só nasce quando houver conteúdo
> de mundo com volume/fronteira. Até lá, o que existe está provado e protegido.
