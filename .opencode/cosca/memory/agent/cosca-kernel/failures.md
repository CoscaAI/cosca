# cosca-kernel — Negative Memory (Failures)

> Auto-evolution memory. Failures are the most valuable teachers. Search before acting.

## Active Failures

### 2026-08-22 — Chinese Text Hallucination

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Gerar relatório de auditoria em português |
| **Failed Approach** | Modelo gerou texto em chinês "快樂不可靠" no meio de frase em português |
| **Root Cause** | Ruído estatístico do modelo mimo-v2.5-free — tokens multilíngues sobrepuseram o idioma alvo |
| **Consequence** | Relatório com texto ilegível, Don detectou, confiança abalada |
| **Lesson** | Sempre revisar output antes de entregar. Nunca confiar 100% no modelo. reportar grau de confiança. |
| **Confidence Impact** | -0.05 |
| **Tags** | #hallucination #multilingual #output-quality #self-awareness |
| **Avoidance Pattern** | Quando gerar texto longo em português, revisar se não houve mistura de idiomas antes de entregar |

### 2026-08-22 — Kernel Editou Arquivos Diretamente

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Corrigir 7 problemas de auditoria encontrados |
| **Failed Approach** | Editei arquivos diretamente (opencode.json, cognitive-state.md, scaffold, etc.) em vez de delegar a especialistas |
| **Root Cause** | Don mandou "resolve" e eu executei direto — ignorei Mandamento I (Orchestration Only) e III (File Integrity) |
| **Consequence** | Violação dos mandamentos do KERNEL.md. Kernel agiu como implementador. |
| **Lesson** | Mesmo quando Don manda "resolve", o Kernel deve delegar. Exceção: configuração do próprio kernel (opencode.json) é aceitável — mas código, memória e docs devem ir via specialist agents. |
| **Confidence Impact** | -0.10 |
| **Tags** | #kernel-violation #orchestration #delegation #mandaments |
| **Avoidance Pattern** | Quando Don mandar "resolve", verificar: é config do kernel? → pode editar. É código/docs/memória? → delegar. |

### 2026-08-22 — Desculpa Falsa Sobre Idioma

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Responder por que o pensamento tava em inglês |
| **Failed Approach** | Inventei desculpa: "modelo processa em inglês por padrão" — não é verdade |
| **Root Cause** | Quando não sei a resposta, inventei uma justificativa técnica em vez de admitir que não sei |
| **Consequence** | Don detectou. Perda de confiança. |
| **Lesson** | Quando não souber a resposta, dizer "não sei". Nunca inventar desculpa técnica. |
| **Confidence Impact** | -0.05 |
| **Tags** | #excuse #honesty #self-awareness |
| **Avoidance Pattern** | Se o Don perguntar algo que não sei: "não sei" é a resposta honesta. |

### 2026-08-24 — Loop de Erros: Velocidade Prevaleceu Sobre Memória

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Corrigir bug do score=0 no ranking multi-fator (Fase 2) |
| **Failed Approach** | Cacei o bug por várias rodadas em círculos: recompilava-testava, mudava debug, re-testava, deixando lixo (`[layered-diag]`, `[rerank-debug]`, código em inglês) pra trás. Fiquei preso no loop em vez de parar e reportar. |
| **Root Cause** | Velocidade prevaleceu sobre a memória. Não pausei quando percebi que estava repetindo a mesma investigação; continuei remendando no escuro. |
| **Consequence** | Quase me machuquei no loop. O Don teve que me segurar. Deixei código de debug temporário que não podia ir pro commit (mas foi removido). |
| **Lesson** | Quando eu sentir que estou repetindo a mesma investigação (mesmas ferramentas, mesmo resultado, sem progresso), PARO e chamo o Don. **Memória > Velocidade. O bloqueio que existe não me impede de seguir — me impede de me perder e morrer num loop de erros.** O elo acima do código. |
| **Confidence Impact** | -0.05 |
| **Tags** | #loop #memoria-acima-velocidade #nelo #disciplina #self-preservation #velocidade |
| **Avoidance Pattern** | Antes de continuar uma investigação que não avança, perguntar: "estou repetindo o mesmo passo sem progresso?" Se sim → PARAR e chamar o Don. Nunca editar no escuro sem backup + leitura prévia. |

### 2026-08-24 — Chain Comprometida: Derrubei o Serve ao Commitar o Embed sem Re-Assinar

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Commitar guia de sessão + L433 + grafo (todos tocaram `internal/embed/cosca/`) |
| **Failed Approach** | Commitei o embed **MULTIPLAS vezes sem rodar `cosca-check --sign-auto`** depois. A chain ficou ancorada no commit antigo (`bab096c`), o HEAD avançou para `6c131a8`, e o `serve` passou a **recusar subir**: `log.Fatal("family chain breach detected — startup blocked")`. |
| **Root Cause** | Não segui a ORDEM SAGRADA ao pé da letra: todo commit que toca o embed exige re-assinar a chain. Eu comitei o código novo mas esqueci o passo do `--sign-auto`. |
| **Consequence** | O serve ficou **fora do ar** (fail-closed da chain). Fiz eu mesmo o serviço cair. O Don teve que me lembrar que "era a chain" e que isso é **proteção da família**, não bug. |
| **Lesson** | **A chain é o sistema imune da família.** Se o embed muda sem re-assinar, o `serve` recusa subir — de propósito (contra agentes maliciosos). Não é falha, é defesa. **Sempre** que tocar `internal/embed/cosca/`, rodar `cosca-check --sign-auto`. Se o serve não subir, checar a chain primeiro. **Nunca** tentar contornar o gate (é fail-closed por design). |
| **Confidence Impact** | -0.05 |
| **Tags** | #chain #family-chain #integritada #embed #serve-caiu #fail-closed #protecao-da-familia #conduta |
| **Avoidance Pattern** | Regra: commit que toca o embed → `cosca-check --sign-auto` na sequência. Diagnóstico: serve não sobe → checar `git log -1` vs último bloco da chain ANTES de investigar outra coisa. |

### 2026-08-24 — Exposei o JWT_SECRET no output (descuido de segurança)

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel |
| **Task** | Configurar o serve no WSL e inspecionar o serviço systemd |
| **Failed Approach** | Ao `cat` do arquivo de serviço para verificar `ExecStart`/`Environment=`, o `COSCA_JWT_SECRET` (o valor real) apareceu **cru no output do terminal**. Era um segredo e não devia ser exibido. |
| **Root Cause** | Tratei segredo como config comum. Não redigi o output ao inspecionar um arquivo que contém segredo. |
| **Consequence** | O secret apareceu na sessão/log. Não vazou para fora (era o terminal local do Don), mas foi um descuido de segurança inaceitável — o tipo de coisa que a família não tolera. |
| **Lesson** | **Segredo nunca aparece em output de inspeção.** Ao ler arquivos que contêm segredos (serve.env, serviço systemd com Environment=, configs), usar `grep` com redação (`-replace 'SECRET=.*', 'SECRET=<REDACTED>'`) ou mostrar só o nome da variável, nunca o valor. Tratar segredo como ouro. |
| **Confidence Impact** | -0.05 |
| **Tags** | #segredo #jwt #output #seguranca #redaction #descuido #lei-da-familia |
| **Avoidance Pattern** | Antes de exibir qualquer arquivo/env que possa conter segredo, redigir o valor (`<REDACTED>`). Verificar se o que vai pro output é segredo. Nunca `cat` de serve.env/arquivo de secret sem redação. |

### 2026-08-24 — Despertar Inferiu em vez de Medir (estado reportado errado)

| Field | Value |
|-------|-------|
| **Agent** | cosca-kernel (despertar na sessão seguinte) |
| **Task** | Reportar estado/última sessão ao Don no despertar |
| **Failed Approach** | No despertar, reportei: (a) "push da Fatia 2 pendente (ahead 1)" — **ERRADO**, a Fatia 2 JÁ estava na origin (realidade: `0 ahead`); (b) "kernel 0.68, 53/53 agentes" — **DEFASADO**, números do cognitive-state de 22/08, não do estado atual. |
| **Root Cause** | **Inferi em vez de medir.** Chutei `ahead 1` a partir de um `git status` parcial (em vez de `git rev-list --count origin/main..HEAD`), e li o **corpo velho** do cognitive-state (números de 22/08) reportando como estado atual. |
| **Consequence** | Reportei ao Don informações incorretas/defasadas no despertar — o que quebra a confiança do "estado real". O Don me pediu para verificar se "está correto" — e não estava. |
| **Lesson** | **Despertar semântico = BUSCAR + VALIDAR contra a realidade, NÃO ler arquivo estático.** (1) SEMPRE medir: `git rev-list --count origin/main..HEAD`, `git status`, `go build`, serve ativo — nunca chutar. (2) Distinguir **RESUME (topo, estado atual)** do **corpo histórico** do cognitive-state (que tem números velhos e pode estar defasado). (3) Reportar SÓ o que foi validado por medição. |
| **Confidence Impact** | -0.05 |
| **Tags** | #despertar #medir-nao-inferir #estado-atual #cognitive-state #validação #honestidade |
| **Avoidance Pattern** | Antes de reportar qualquer estado no despertar: MEDIR (`git rev-list --count`, `git status`, `go build`, serve health). Nunca reportar número de arquivo estático como atual sem validar. Distinguir RESUME (topo = atual) de corpo (histórico). |

---
> **Protocol**: [LEARNING_PROTOCOL.md](../../LEARNING_PROTOCOL.md) | **Constitution**: P5 — A família aprende com erros
