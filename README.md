# Cosca — Enterprise AI Orchestration Platform

> **Versão:** 1.5.0 · **Go 1.26+** · Windows/Linux/macOS
> **O Cosca não é um chatbot — é um sistema de orquestração de agentes com um
> cérebro de conhecimento curado e, agora, todos os sentidos (visão 4/4, voz
> nativa, OCR com zoom).** Ele decide onde procurar (roteador determinístico),
> recupera informação validada, e delega execução numa hierarquia operacional
> onde cada agente tem papel definido.

---

Incidente de segurança — preservação e investigação

Foi identificado um comportamento considerado anômalo durante o desenvolvimento do projeto, com indícios de possível manipulação não autorizada de arquivos, documentação e/ou ambiente de execução.

Neste momento, nenhuma conclusão definitiva sobre autoria, origem ou mecanismo do incidente está sendo declarada. O ambiente e os artefatos relacionados estão sendo tratados como evidência.

A prioridade é preservar o estado original e manter a cadeia de evidências íntegra. Alterações, limpeza, reconstrução ou sobrescrita dos artefatos potencialmente envolvidos devem ser evitadas até que a coleta e a validação sejam concluídas.

Os registros disponíveis incluem histórico de sessões, arquivos de projeto, documentação, artefatos de execução e demais evidências relacionadas ao período investigado. Hashes e inventários devem ser utilizados para verificar a integridade dos materiais preservados.

Status: investigação / preservação de evidências.
Regra: não modificar ou destruir os artefatos sob investigação sem registro e validação prévia.

## Índice

- [O que é o Cosca](#o-que-é-o-cosca)
- [Números reais](#números-reais)
- [Sentidos (percepção completa)](#sentidos-percepção-completa)
- [Arquitetura](#arquitetura)
- [Instalação](#instalação)
- [Uso básico](#uso-básico)
- [Como tirar proveito máximo](#como-tirar-proveito-máximo)
- [Segurança e integridade](#segurança-e-integridade)
- [Solução de problemas](#solução-de-problemas)

---

# Esclarecimentos sobre o estado atual e referências históricas

## 1. Objetivo

Este documento estabelece como interpretar os resultados, decisões e evidências presentes no estado atual do projeto quando existirem referências anteriores, benchmarks históricos ou decisões registradas em commits anteriores.

O princípio fundamental é:

> **O estado atual não reescreve a história do projeto. Resultados anteriores permanecem válidos como evidência histórica do estado em que foram medidos, salvo quando uma nova medição demonstrar explicitamente o contrário.**

---

## 2. Referência ao estado atual

O commit atualmente analisado deve ser tratado como um **estado específico do código**, e não como uma representação retroativa de todo o histórico do projeto.

Portanto:

* resultados medidos em commits anteriores pertencem aos respectivos commits;
* decisões tomadas anteriormente devem permanecer associadas às evidências que as originaram;
* alterações posteriores de código, plataforma, ambiente ou configuração não devem ser projetadas retroativamente sobre esses resultados;
* uma diferença entre o comportamento atual e um resultado histórico não invalida automaticamente o resultado histórico.

Quando necessário, a comparação deve seguir:

**commit → código → ambiente → experimento → resultado → decisão**

---

## 3. Referências anteriores ao fato

Resultados anteriores podem e devem ser utilizados como **referências históricas**, especialmente quando foram registrados antes de uma mudança relevante.

Essas referências são particularmente importantes para estabelecer:

* baseline;
* comportamento esperado;
* regressões;
* mudanças de desempenho;
* alterações de arquitetura;
* mudanças de plataforma;
* mudanças de compilador/runtime;
* mudanças de hardware ou configuração;
* surgimento ou desaparecimento de determinados comportamentos.

Uma evidência anterior não deve ser descartada simplesmente porque o estado atual apresenta comportamento diferente.

Ao contrário:

> **A divergência entre o resultado histórico e o resultado atual constitui uma hipótese de investigação.**

---

## 4. Exemplo: campanha de performance

A campanha de performance registrada anteriormente constitui um baseline histórico.

Ela documenta medições realizadas sob condições específicas, incluindo hardware, dataset, representação, número de workers, páginas aquecidas e metodologia definida no próprio registro.

Esses números devem continuar sendo interpretados como:

**MEASURED no ambiente e estado em que foram obtidos.**

Eles não devem ser automaticamente apresentados como:

**MEASURED no commit atual.**

Caso o commit atual produza números diferentes, o correto é registrar um novo experimento e estabelecer a relação entre os dois:

```text
Baseline histórico
    ↓
commit anterior
    ↓
ambiente anterior
    ↓
resultado medido

        comparação

commit atual
    ↓
ambiente atual
    ↓
novo resultado medido
```

---

## 5. Mudança de plataforma

Quando houver uma transição entre ambientes — por exemplo, Linux → Windows — a mudança de comportamento deve ser investigada como uma possível variável experimental.

Não é correto concluir previamente que:

> "o Windows causou o problema"

nem:

> "o benchmark antigo estava errado".

A conclusão correta depende de isolamento das variáveis.

Devem ser comparados, quando possível:

* commit do código;
* versão do Go;
* compilador;
* flags de compilação;
* runtime;
* número de CPUs/workers;
* afinidade e scheduling;
* alocação de memória;
* comportamento de páginas/cache;
* filesystem;
* mmap;
* temperatura e frequência da CPU;
* configuração do sistema operacional;
* dataset;
* harness do benchmark;
* metodologia de medição.

Somente depois dessa comparação uma regressão pode ser atribuída a uma causa específica.

---

## 6. Integridade dos benchmarks históricos

Benchmarks anteriores não devem ser editados para refletir descobertas posteriores.

Se uma conclusão mudar, deve ser criado um novo registro contendo:

1. referência ao benchmark anterior;
2. hipótese que motivou o novo teste;
3. alterações experimentais;
4. ambiente;
5. resultado;
6. interpretação;
7. decisão;
8. evidência necessária para confirmação.

Assim:

```text
Benchmark A
    ↓
resultado original
    ↓
nova hipótese
    ↓
Benchmark B
    ↓
novo resultado
```

e não:

```text
Benchmark A
    ↓
resultado antigo apagado
    ↓
resultado novo
```

---

## 7. Classificação epistemológica

As referências históricas devem preservar sua classificação original.

### FACT

Fato estrutural ou verificável independentemente do experimento.

### MEASURED

Resultado efetivamente medido em determinado ambiente e estado do código.

### PROFILE

Caracterização observada a partir de medições.

### INFERRED

Interpretação derivada dos dados, ainda não diretamente demonstrada.

### DECISION

Escolha de engenharia baseada nas evidências disponíveis naquele momento.

### UNKNOWN

Questão ainda não demonstrada.

Uma decisão histórica pode continuar sendo útil mesmo quando uma decisão posterior a substitui. Nesse caso, a decisão anterior permanece como parte da evolução do projeto.

---

## 8. Regra de comparação

Quando o estado atual apresentar comportamento diferente do histórico:

> **Não apagar o histórico. Não corrigir retroativamente o número. Não atribuir causalidade sem experimento.**

Em vez disso:

```text
HISTÓRICO
commit X
→ resultado Y

ATUAL
commit Z
→ resultado W

DIFERENÇA
Y ≠ W

HIPÓTESE
qual mudança entre X e Z explica a diferença?

EVIDÊNCIA NECESSÁRIA
novo experimento controlado
```

---

## 9. Referências anteriores ao fato

Referências anteriores ao fato são especialmente relevantes porque estabelecem uma condição conhecida **antes da ocorrência da alteração investigada**.

Quando uma evidência anterior demonstra que determinada capacidade funcionava sob condições específicas, ela deve ser tratada como:

> **baseline histórico anterior ao evento**

e não como opinião ou reconstrução posterior.

Isso é particularmente importante para investigação de regressões: o objetivo não é provar antecipadamente uma causa, mas identificar **o primeiro ponto em que o comportamento deixou de ser reproduzível**.

---

## 10. Princípio final

O projeto deve preservar a seguinte cadeia:

**evidência histórica → estado do código → ambiente → medição → interpretação → decisão**

Uma nova versão pode substituir uma decisão técnica, mas não substitui a existência histórica da decisão anterior.

Portanto:

> **O commit atual deve ser analisado no seu próprio contexto, enquanto os commits anteriores permanecem como referências independentes para comparação, reprodução, regressão e auditoria.**

> **Histórico não é ruído: é evidência.**


## O que é o Cosca

O Cosca é uma **plataforma de orquestração de agentes de IA** em Go. Ele não é
um framework genérico — é uma infraestrutura de produção com:

- **Conhecimento modular** com proveniência (claims FACT / EVIDENCE / INFERENCE).
- **Memória semântica** auto-evolutiva (o kernel aprende com cada sessão).
- **Busca híbrida**: texto (BM25) + semântica (vetor) + grafo relacional.
- **Sentidos nativos**: visão (ONNX 4/4), voz (STT/TTS PT-BR), OCR com zoom.
- **Quality gates, sandboxing e chain de integridade** Ed25519.
- **Modularidade de dados** (ADR-013): sem banco monolítico — cada domínio em
  um módulo < 100 MB.

### Hierarquia operacional

```
DON (autoridade máxima)
 └─ KERNEL (consigliere — roteia, NÃO implementa)
     ├─ CEO (estratégia) → CTO (técnica)
     ├─ CHIEFS (capos de domínio: backend, frontend, security, database, ai…)
     └─ SPECIALISTS (soldados: implementação concreta)
```

> O **Kernel nunca implementa** — planeja, roteia, delega e revisa.

---

## Números reais

Métricas verificadas no estado atual (2026-09-07):

| Métrica | Valor |
|---------|------:|
| Agentes | **55** (canônico = editor) |
| Skills | **93** |
| Workflows / Pipelines | **39** |
| Entries de conhecimento | **16.043** |
| Vetores nos módulos | **20.749** |
| Providers | 10 (ollama, local ativos) |
| Arquivos Go (src) | **1.869** |
| Testes Go | **844** |
| Modelos de visão (ONNX) | **4/4** |
| Modelos de voz (STT+TTS) | **PT-BR nativo** |

---

## Sentidos (percepção completa)

O Cosca **sente** — tudo nativo, local, sem Python em runtime:

### Visão (ONNX 4/4 — `cosca model vision`)
| Modelo | Arquivo | Capacidade |
|--------|---------|-----------|
| CLIP | `clip_vitb32.onnx` | classifica / embedding de imagem (512-dim) |
| SAM2 | `sam2_hiera_large.onnx` | segmentação (recorta objetos) |
| GroundingDINO | `groundingdino_swint.onnx` | detecção por texto |
| Depth | `depth_anything_v2_vitl.onnx` | profundidade (distância) |

```bash
cosca vision infer img.png     # pipeline completo
cosca model vision             # status dos modelos
```

### Voz (nativa, Go — sem Python/torch)
- **STT** (ouvir): motor offline sherpa-onnx, modelo NeMo PT-BR.
- **TTS** (falar): sintetizador vits-piper PT-BR (edresson).
- **Loop realtime** `cosca voice chat`: ouve → vê (pedido) → interpreta → fala.

```bash
cosca voice devices       # dispositivos de áudio
cosca voice speak "olá"   # fala (gera .wav)
cosca voice chat          # diálogo ao vivo (mic → STT → TTS)
cosca voice listen        # transcreve ao vivo
```

### OCR com zoom (ler texto da tela)
OCR nativo WinRT com refinamento adaptativo (interpolação bicubic 1x→2x→4x):

```bash
cosca screen --ocr        # captura a tela + lê texto (zoom)
cosca screen --ocr --json # regiões de texto + métricas
```

> A percepção é **por ação**: o Cosca só "olha a tela" quando você pede
> ("olha a tela" no `voice chat`). Tudo local e soberano.

---

## Arquitetura

### Dados (ADR-013 — corte modular)

O Cosca **não usa** um banco monolítico. Os dados são particionados em módulos
físicos, cada um < 100 MB:

| Módulo | Arquivo | Conteúdo |
|--------|---------|----------|
| Core | `core.db` | documentos originais (manifest), proveniência, metadados — **fonte da verdade** |
| Grafo | `graph.db` | entidades + relações |
| Projects | `projects.db` | chunks + elementos + FTS |
| Vetores | `vector-*.db` | embeddings (768-dims, `nomic-embed-text`) — **busca semântica** |
| Semântico | `semantic.db` | índice semântico |

**A busca usa similaridade por cosseno (entendimento) + BM25 (palavra) + grafo.**

### Integração externa

- **Unreal Engine**: ponte via WebSocket.
- **Blender**: pipeline end-to-end real (`scripts/blender/*.py`).
- **Embeddings**: `nomic-embed-text` local (Ollama, 768-dims, offline).
- **Providers**: 10 suportados (ollama local ativo; OpenAI/Anthropic/etc como no_key).

---

## Instalação

### Pré-requisitos

- **Go 1.26+** (para compilar) **ou** binário pré-compilado.
- **Git** (para integridade da chain e versionamento).
- **Ollama** (opcional, para embeddings/LLM locais) — recomendado.
- **gcc/mingw64** (para build com voz — `stt_sherpa`/`tts_sherpa` via CGO).

### 1. Build (a partir do código)

```bash
cd cosca

# Binário completo com VOZ (requer gcc/mingw via CGO)
CGO_ENABLED=1 go build -tags "stt_sherpa tts_sherpa" -o cosca ./cmd/cosca

# Binário base (sem voz) — mais simples
go build -o cosca ./cmd/cosca
```

> A visão/OCR funcionam sem tags extras; a **voz** (STT/TTS) exige o build
> com `-tags "stt_sherpa tts_sherpa"` (nativo, CGO).

### 2. Inicializar o projeto

```bash
cosca init      # cria .cosca/ + configuração + descobre o ambiente
cosca start     # fluxo completo: init + install + doctor + sync
```

### 3. Subir o serviço (REST API)

```powershell
$env:COSCA_ALLOW_NO_ROOT="1"      # Windows (sem jail bwrap)
$env:COSCA_JWT_SECRET="<segredo forte, ≥32 bytes>"   # no .env (gitignored)

cosca serve
# health: curl http://127.0.0.1:14120/health → 200
```

---

## Uso básico

```bash
# Consultar conhecimento (busca semântica)
cosca knowledge search "orquestração de agentes"

# Orquestrar uma tarefa via IA
cosca run "refatore o módulo de autenticação"

# Sentidos
cosca vision infer foto.png        # ver
cosca voice speak "olá"            # falar
cosca screen --ocr                 # ler tela (zoom)

# Ver agentes/skills/workflows
cosca agent list
cosca skill status
cosca workflow list

# Nível de capacidade
cosca capability level
```

---

## Como tirar proveito máximo

### 1. Pipeline de decisão (Kernel-first)

```bash
cosca plan "melhorar a performance do search"
cosca approve --plan plano.json
cosca delegate
cosca run/gate
```

### 2. Delega a domínios específicos

```bash
cosca agent show security-chief
cosca agent search "performance"
```

### 3. Alimente o conhecimento (o cérebro cresce)

```bash
cosca knowledge index <diretório>
cosca knowledge add <lib|repo>
cosca knowledge evidence
cosca knowledge claim
```

### 4. Memória semântica

```bash
cosca memory semantic "como resolver conflito"
cosca memory snapshot create
cosca memory guard
```

### 5. Banco modular

```bash
cosca db check --gate
cosca db build
cosca db verify
```

> **⚠️ Regra de ouro:** rode `cosca knowledge vectors-backfill` **antes** de
> `cosca db build` — o build recria os módulos a partir da fonte.

### 6. Percepção e produtividade

```bash
cosca machine probe
cosca model vision
cosca voice chat
cosca screen --ocr
cosca qgate
```

---

## Segurança e integridade

- **Family chain:** Ed25519 + DPAPI (Windows) + git-anchor. Assinatura
  machine-bound. Se o histórico for reescrito, a chain **bloqueia** o boot
  (fail-closed correto).
- **JWT:** `COSCA_JWT_SECRET` obrigatório (≥32 bytes). Sem ele o serve não sobe.
- **Jaula:** bwrap (Linux/WSL2). Windows exige `COSCA_ALLOW_NO_ROOT=1`.
- **Loopback only:** `cosca serve` escuta só em `127.0.0.1:14120-14122`.
- **Fail-closed:** o sistema bloqueia de propósito quando detecta alteração de
  integridade — proteção, não bug. Re-inicializar com `cosca memory integrity
  init --force` quando a mudança é legítima.

---

## Solução de problemas

### "Serve não sobe"
1. **Chain válida?** → `cosca-check` → se `BREACH`, `cosca-check --sign-auto`.
2. **`COSCA_JWT_SECRET`?** → setar no `.env` (≥32 bytes).
3. **`COSCA_ALLOW_NO_ROOT=1`?** (Windows).

### "Busca semântica não retorna"
1. **Vetores populados?** → `cosca knowledge vectors-backfill`.
2. **Módulos sincronizados?** → `cosca db build` → `cosca db verify`.

### "Fail-closed bloqueando"
O Cosca **bloqueia de propósito** quando detecta alteração de integridade.
Diagnosticar e **re-inicializar** (`cosca memory integrity init --force`) quando
a mudança é legítima.

### "Voz não carrega"
1. Build com `-tags "stt_sherpa tts_sherpa"` (CGO).
2. DLLs nativas em `bin/` (`onnxruntime.dll`, `sherpa-onnx-*-api.dll`).
3. Modelos de voz em `~/.cosca/models/` (STT/TTS PT-BR).

---

## Documentação relacionada

| Documento | Caminho |
|-----------|---------|
| Manual completo | `docs/MANUAL_COSCA.md` |
| Níveis de capacidade | `docs/COSCA_LEVELS.md` |
| ADR-013 (corte modular) | `docs/adr/ADR-013-modular-knowledge-databases.md` |

---

> **"Honestidade > Lealdade > Confiança; Memória > Velocidade."**
> — Lei da Família Cosca
