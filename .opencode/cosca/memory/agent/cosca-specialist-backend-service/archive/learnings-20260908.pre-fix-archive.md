# cosca-specialist-backend-service - learnings.md EDITOR PRE-FIX

> Arquivo gerado em 20260908. Conteudo preservado - leia por grep, nunca inteiro.

# cosca-specialist-backend-service — Semantic Learnings

> Auto-evolution memory. Search before acting. Record after learning.

## Session: 2026-08-23 — FATIA D1: deliberação determinística evidência-gated (ADR-011, Bloco 1)

### 2026-08-23 — Pacote `internal/deliberate` (zero-LLM, gates externos ao modelo)
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend-service |
| **Task** | Criar `internal/deliberate` determinístico (A2–A7): convergência, loop-detector, confiança aritmética, evidência L0–L5+M1–M7, conflito cross-vote, SPEC validável |
| **Technique** | Pure-functions por arquivo (types/convergence/confidence/evidence/pool/spec), sem estado/goroutines, tudo testável sem LLM |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #deliberate #convergence #confidence-breakdown #evidence-gating #zero-llm #adr-011 #a2 #a4 #a7 |
| **Related** | ADR-011 §3.1; CONFIDENCE_MODEL.md (internal/embed/cosca/engines/evidence/, NÃO memory/); internal/confidence/tracker.go (reuso conceitual, não duplicado) |
| **Learned** | (1) **Nome colide em Go**: tipo `Emit` + função `Emit` no mesmo pacote → compile error. Manter o tipo `Emit` e renomear a função para `EvaluateEmit` (documentar o porquê). (2) **"Zero achismo"** = position é `Effective()` só quando `Substantiated && len(EvidenceIDs)>0`; posições não-efetivas são IGNORADAS na convergência (contribuem 0), não somam concordância. (3) **Convergência** `Σ(peso×concordância)` por dimensão (Recommendation .30/Premises .25/Risks .25/Timing .20) exige agrupar positions por dimensão → precisei de um campo `Dimension` extra em `Position` (extensão justificada). (4) Dimensão descoberta contribui **0, sem renormalizar** (gap vira 0, não pass) — isso torna "Pesos somam 1.0" e o limiar 70% testável com floats (usar `assert.InDelta` para esperado 0.0; `assert.InEpsilon` dá erro com expected 0). (5) `ComputeConfidence`: Final = clamp01(Base + Σ deltas); breakdown é trace determinístico `base=.. x=.. final=..`. (6) `EvidenceConfidence(ev any)`: type-switch para `Evidence`/`*Evidence` (nil pointer → L0=0.20); não-evidência → 0.20 conservador. (7) `ValidateSpec`: CONFIDENCE é numérica (0,1]; mensagem de erro de CONFIDENCE **não** contém "required" → teste deve usar substring por caso. |
| **Next** | Integração (fatia seguinte): port `DecisionDeliberator` em `internal/orchestration/ports.go` + `DeliberateConfig` no `OrchestratorConfig`; adapters critic/review |

### 2026-08-23 — Envelope uniforme de resultado (results pkg)
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend-service |
| **Task** | Implement `internal/results` envelope uniforme {success,data,degraded,exitCode} (paridade D6) |
| **Technique** | Manager pattern — novo pacote `results`, envelope value-type + construtores OK/Fail/Degraded, conversão de borda via `ResultError` |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #results #envelope #degraded #exit-code #boundary #adr-011 |
| **Related** | internal/adapter (a migrar em H2), internal/policy, finding V13 |
| **Learned** | Invarante central: `Success == (exitCode == 0)` mantido pelos construtores. `Fail(0,...)` normaliza exitCode→1 para nunca parecer sucesso na borda. `Degraded(...)` é `Success=true` + `Degraded=true` + `exitCode=0` — **não é error**; `WasError()`/`Err()` o tratam como sucesso. `Err()` retorna `*ResultError` que preserva o `Result` (resgata exitCode/reason na fronteira sem parse de string). |
| **Next** | H2: migrar `internal/adapter` para retornar `Result` (aditivo, mantém `(*T, error)`) |

### 2026-08-23 — MCPPolicy default-deny + dangerous patterns + budget (policy pkg)
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend-service |
| **Task** | Implementar `mcp_policy.go`, `dangerous.go`, `budget.go` em `internal/policy` (default-deny, P1, V13, orçamento 200/turno) |
| **Technique** | Aditivo ao `policy` existente — reusa `Decision`/`Allow`/`Deny` (não recria enum). Classificação de capacidade por nome (determinística, sem parser de shell). Matching por substring (reuses `has(...)` do `DefaultRules`) |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #policy #mcp-policy #default-deny #dangerous-patterns #budget #v13 #security |
| **Related** | V13 (execpolicy não interpreta operadores de shell), ruflo-patterns (mcp-policy.json), internal/execpolicy/Tokenize |
| **Learned** | Default-deny = dois fatores: tool precisa estar em `AllowedTools` E a capability (Shell/Network/FileWrite) precisa estar explicitamente habilitada. `Evaluate` retorna `Deny` como **Decision, não error** — error só para uso inválido (nil receiver, tool vazia). Orçamento default 200 em `DefaultMaxToolCalls`; `ToolBudget.Consume` normaliza `Max<=0`→200. Padrões perigosos: `rm -rf`, `sudo`, `curl|sh/bash`, `ssh`, `git push --force/-f`, operadores `|`,`&&`,`;`,`` ` `` — específicos antes de genéricos para retornar o padrão mais informativo. |
| **Next** | H2: interpretar operadores de shell no `execpolicy` (parser leve), integrar `MCPPolicy` al `adapter` |

## Referências de padrões do projeto (constantes)
- Testes: table-driven com `github.com/stretchr/testify/assert` + `require` (convenção ampla no repo).
- `go build ./...`, `go vet ./...`, `go test ./internal/results/... ./internal/policy/...` verdes (go 1.26.7, Windows).
- `internal/embed/cosca` é intocável (P8) — nada foi alterado lá.
- Erros sempre envolvidos com contexto (`fmt.Errorf("...: %w", err)`); nunca `panic`.
### 2026-08-29 - MCP cognitive tools: trace_id gerado mas nunca gravado no flight recorder (auditoria do cerebro)
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend-service |
| **Task** | Corrigir bug: tools de LEITURA (recall/context/project/reason) geravam TraceID novo via NewContextPacket mas NUNCA chamavam e.Trace.Append - cosca.trace <id> devolvia "sem eventos". |
| **Technique** | Manager pattern: Engine.Trace nil-able. Helper DRY (e *Engine) recordTraceEvent(packet *ContextPacket, action, details string) faz Append best-effort Actor:"mcp", Result via esultFor(len(packet.Context)) ("success"/"empty"). Reaproveita o packet ja criado (mesmo TraceID). _ = ignora erro p/ nunca abortar (trace = auditoria, nao pre-requisito). Nao alterou callLearn (ja gravava) nem callTrace (leitor do proprio trace). |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #mcpserver #trace #flight-recorder #best-effort #auditoria #context-packet #manager-pattern |
| **Related** | internal/mcpserver/tools.go, internal/trace/store.go Append (append-only, db==nil ou TraceID invalido retornam erro), internal/mcpserver/context_packet.go NewContextPacket (so gera ID, nao grava) |
| **Learned** | (1) NewContextPacket gera TraceID mas NAO escreve no ledger - a gravacao e responsabilidade de quem consome o packet. (2) Append de trace e best-effort por design: se e.Trace == nil ou db falha, a tool NUNCA deve falhar - sempre devolver o packet. (3) Reutilizar o packet, nao regenerar - gravar com o MESMO TraceID devolvido, senao cosca.trace nao acha. (4) 	race.Store.Append retorna erro se s.db == nil ou TraceID invalido - por isso o guard if e.Trace == nil. (5) mustTraceStore(t) (trace.NewStore(t.TempDir()+"/trace.db")) e o helper p/ testar auditoria com SQLite temporario. |
| **Next** | Aplicar o mesmo a callObserve e reavaliar callTrace se o escopo expandir (nao alterado sem ordem). |

### 2026-08-29 - Fechamento do flight recorder: callObserve grava OBSERVE + callTrace carrega trace_id LIDO
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend-service |
| **Task** | Fechar os 2 pontos restantes: (1) callObserve gerava TraceID novo mas NAO gravava no ledger; (2) callTrace lia args.TraceID mas devolvia packet com trace_id NOVO (imprecisao semantica / auto-referencia). |
| **Technique** | (1) Em callObserve: capturar o packet em variavel, chamar e.recordTraceEvent(packet,"OBSERVE",fmt.Sprintf(...)) com video/frames/events/items (guarda nil-safe p/ result), devolver o MESMO packet. (2) Em callTrace: trocar NewContextPacket(...) por construcao manual &ContextPacket{Query,Context,Confidence:packetConfidence,TraceID:args.TraceID} — o packet carrega o trace LIDO, nao um novo. Removido o retorno IsError do caso vazio: agora devolve packet honesto (context vazio + confidence 0) com o trace lido. NUNCA chamar recordTraceEvent dentro de callTrace (leitor; gravar criaria loop do trace lendo a si mesmo). |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #mcpserver #trace #flight-recorder #callObserve #callTrace #auto-referencia #context-packet #semantica-lida |
| **Related** | internal/mcpserver/tools.go callObserve/callTrace; internal/trace/store.go Get (Parse invalido retorna erro); internal/mcpserver/context_packet.go packetConfidence; TestObserve_RecordTraceEvent + TestTrace_EmptyReturnsSameTraceID (tools_test.go) |
| **Learned** | (1) Leitura de trace NAO se auto-audita: as tools de leitura de AGIR (recall/context/reason/project/observe) gravam no ledger; callTrace E O LEITOR do ledger e, por isso, NAO deve gravar — se gravasse, o trace registraria a si proprio (loop). (2) callTrace precisa montar o packet manualmente (Query/Context/Confidence/TraceID) para carregar o trace LIDO; NewContextPacket geraria ID novo. (3) Caso vazio de callTrace NAO deve virar IsError de texto: deve voltar packet honesto (context vazio, confidence 0) com o trace lido — assim o cliente decodifica um ContextPacket real (contrato ADR-028 §3). (4) packetConfidence([]) == 0 (honesto "nao sei") e len(packet.Context) e 0 quando trace vazio. (5) TestVisionFunc deterministica (testObserveVision) injeta um vision.PipelineResult com 1 PerceptEvent OBSERVED p/ testar callObserve sem video real. Compatibilidade: TestReadTools_RecordTraceEvent (recall/context/reason) e TestReadTools_NilTraceIsBestEffort continuam verdes. |
| **Next** | Rodada completa: cerebro grava toda reacao de leitura (recall/context/reason/project/observe) no flight recorder e callTrace carrega o trace lido corretamente. |

### 2026-08-30 - SOLITEK: substituir protocolo manual Niimbot B1 pela lib @mmote/niimbluelib
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend-service |
| **Task** | Rewrite de `NiimbotService.printLabel` (bug "começa e para" na print por serial COM3) para usar a lib oficial `@mmote/niimbluelib` (comprova imprime na B1). Projeto externo em E:\Projects\soliket\apps\backend (NestJS 10 + Prisma, fora da workspace cosca). |
| **Technique** | Remover todo o frame-building manual (buildFrame/packLine/countBitmapCounts/buildSettingFrames/sendFrames/writePacket/CMD.*/FRAME_*). Usar `instantiateClient('node-serial').setPort('COM3')`; `client.abstraction.newPrintTask('B1', opts)`; `B1PrintTask.printInit->printPage->waitForFinished->printEnd`; `ImageEncoder.encode(source, 'top')`. A lib NAO tem canvas no Node => fornecer um custom `ImageSource` (width/height/isPixelNonWhite) que lê PNG via sharp `ensureAlpha().raw()` RGBA e indexa `(y*w+x)*4`. Layout v6 (384x240) + QR via `qrcode.toBuffer` + composição SVG->PNG via sharp. |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #niimbot #b1 #thermal-printer #usb-serial #COM3 #niimbluelib #sharp #qrcode #manager-pattern #service-orders #solitek |
| **Related** | @mmote/niimbluelib (dist/cjs): `NiimbotAbstractClient.abstraction`, `B1PrintTask` extends `AbstractPrintTask` (printInit/printPage/waitForFinished/printEnd), `ImageSource.isPixelNonWhite(x,y,printDirection)`, `PrintOptions` (speed: 0|1 tipado mas B1 ignora). tsconfig do projeto: `esModuleInterop=false` => `const sharp = require('sharp')`; `skipLibCheck=true`. |
| **Learned** | (1) `instantiateClient('node-serial')` retorna `NiimbotAbstractClient` (SÓ connect/disconnect/isConnected) — `setPort` exista SOMENTE em `NiimbotNodeSerialClient`, precisa cast. (2) `B1PrintTask` nao tem `printEnd()` próprio; herda de `AbstractPrintTask.printEnd()` (abstraction.printEnd). (3) `ImageEncoder.encode` é sina: recebe um `ImageSource` sincrono — carregar o raw ANTES (init) senao isPixelNonWhite retorna false. (4) `ImageEncoder.encode` agrupa linhas em `rowsData` (comprime linhas brancas em `dataType:'void'`) — nao assume rowsData.length === height. (5) layout v6: rect de contorno precisa `fill="none" stroke="#222"` — no SVG default fill=black e stroke=none, entao transcrever `stroke-width="4"` sozinho gera retangulo preto/invisivel (correcao necessaria). (6) `speed: 3` vs tipo `0|1` => cast `as unknown as PrintOptions['speed']` (B1 ignora speed). (7) Fechar a porta sempre no `finally` (`if client.isConnected() await client.disconnect()` catch best-effort) p/ nao vazar o recurso COMx. (8) Validar sem abrir porta: chamar buildLabelImage + makeImageSource + ImageEncoder.encode no service instanciado com deps dummy. |
| **Next** | Teste manual real com hardware (Don) — abrir COM3 e imprimir etiqueta v6; confirmar modelo reconhecido 4096/B1 e ausência do sintoma "começa e para". |

### 2026-09-08 — ADR-013 Decisão 1 revisada: gate de tamanho vira OPT-IN (--limit-mb); default = relatório informativo
| Field | Value |
|-------|-------|
| **Agent** | cosca-specialist-backend-service |
| **Task** | Mudar `cosca db check --gate` para NUNCA falhar por tamanho sem `--limit-mb` (bancos agora DERIVADOS/regeneráveis, não versionados — premissa do teto 100MB morreu). FromConfigMB/dbhealth: limite <= 0 = "sem teto". |
| **Technique** | (1) `FromConfigMB`: limitMB <= 0 → `Limits{}` (0/0/0), warnMB ignorado; N>0 preserva histórico (FailBytes=teto, WarnBytes=warnMB|80%). (2) `Check`: `l.LimitBytes <= 0` → NÃO re-default p/ DefaultLimits (antes 0→DefaultLimits era a armadilha); zero-Limits agora = no-limit. (3) `measure`: sem teto → `PercentOfLimit = PercentOf100MB` (fallback ReferenceCeiling p/ não dividir por zero) + StatusOK sempre (Classify nunca roda). (4) CLI: flag `--limit-mb` default 100 → 0; help/header/texto ajustados; resumo warn/fail só quando LimitBytes>0. (5) recovery.go usa DefaultLimits() explícito — intacto. |
| **Level** | 2 |
| **Outcome** | success |
| **Tags** | #dbhealth #adr-013 #gate #limit-mb #no-limit #derived-databases #manager-pattern #cli |
| **Related** | internal/dbhealth/dbhealth.go (DefaultLimitBytes/ReferenceCeiling consts, FromConfigMB, Check, measure), internal/cli/db.go (NewDBCheckCommand, printDBCheckText), internal/cli/recovery.go (DefaultLimits inalterado), docs/adr/ADR-013 §2.2.1, cli_db_test.go (growTestDB) |
| **Learned** | (1) **Representar "sem teto" como `Limits{}` (0/0/0) e tratar `LimitBytes <= 0` no Check como no-limit** exige cuidado: zero-value Options mudou de "DefaultLimits" para "sem teto" — aceitável pois callers reais ou usam FromConfigMB (db.go) ou DefaultLimits() (recovery.go); testes que passavam Options sem Limits só usavam DBs pequenos (ok continua ok). (2) **Divisão por zero real**: `PercentOfLimit = db/LimitBytes` com LimitBytes=0 → +Inf; o fallback p/ ReferenceCeiling na coluna "% do teto" é a métrica informativa pedida. (3) **PRAGMA page_count não muda durante transação aberta** (header p/ commit) — para crescer arquivo SQLite até >100MB não dá p/ loopear lendo page_count dentro da tx; calcular rows antecipadamente (sizeBytes/1MiB + margem de páginas) e verificar depois do commit. (4) growTestDB: rows = size/chunk + 32 garante cruzar o alvo (1MiB payload ≈ 1MiB + fração de página/row). (5) Testes CLI usam `-run "DB"` mas nomes dbhealth quase nunca contêm "DB" (só FindsRuntimeDBs) — rodar o pacote inteiro para cobrir FromConfigMB/measure/Check. (6) `gofmt -l internal/cli` lista ~20 arquivos pré-existentes fora do padrão — não tocar (não é regressão minha). |
| **Next** | Nenhuma pendência técnica; acompanhar se recovery/doctor deve também adotar no-limit default (fora do escopo desta ordem — recovery.go usa DefaultLimits e só imprime, não seta allOK=false por tamanho). |

