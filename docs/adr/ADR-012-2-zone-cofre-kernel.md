# ADR-012: Arquitetura de 2 Zonas — Cofre (air-gap: IA local + Oracle puro) e Kernel (internet)

> **Status**: Proposed | **Owner**: cosca-architecture (Architecture Chief) | **Last Updated**: 2026-08-24
> **Revisão**: aguardando cosca-cto + cosca-security + Don. **Decisão de design — NÃO implementada.**
> **Fonte**: `.opencode/cosca/memory/context/cognitive-state.md` (plano pendente, item 2) + `ORACLE_PROTOCOL.md` + `SECURITY_PROTOCOL.md`
> **Relação**: formaliza o desenho de "2 zonas" que hoje vive apenas fragmentado no `cognitive-state.md`. Não altera código.

---

## 0. Mapa objetivo do que o Cosca JÁ tem (o "estado atual" para o gap)

Antes de declarar o gap, o mapeamento real (evitando achar lacuna onde não existe):

| Bloco real hoje | Onde | O que cobre |
|---|---|---|
| **Oráculo (fronteira semântica)** | `internal/oracle/oracle.go` | `SemanticPackage{Intent, Result, Source, Provenance, Evidence, Context, Actions, Changes, Confidence}`, `Decision{ACCEPT, ACCEPT_WITH_CAVEAT, REJECT, INCONCLUSIVE}`, `Verdict{Decision, ContractGaps, SemanticGaps, Required, Questions, Reason}`, `Gate{MemoryCheck}` + `Evaluate()` **fail-closed**. Já tem as invariantes §25-§29 (valida significado, nunca a palavra do externo, hipótese→fato só com evidência, INCONCLUSIVE = "ainda não sabemos"). |
| **Protocolo de busca** | `internal/oracle/search.go` | `SearchIntent`, `Source{memory, codebase, docs, external}`, `SearchRequest.Valid()` (intenção+query+fontes; busca sem propósito = bloqueada), `Provenance`, `EvidenceClass{FACT, MEASURED, EVIDENCE, INFERRED, HYPOTHESIS}`, `RelevanceClass`, `SemanticScore`, `SearchLayer` (busca em camadas), `SearchResponse`, `Conflict`, `SearchHypothesis` (anti-confirmação). |
| **Gate no serve (já conectado!)** | `api/rest/handler/run.go` (`oracleGate`) | O `RunHandler` inicializa `oracle.NewGate()` e valida a `SemanticPackage` de toda request **antes** de qualquer execução (fronteira do Cofre, §0). `REJECT` → HTTP 400 + audit log; `ACCEPT`/`CAVEAT` → fluxo normal. |
| **Cofre de dados (data-vault)** | `internal/secrets/vault.go` | `Vault` SQLite AES-256-GCM, chave derivada de `COSCA_JWT_SECRET` via HKDF-SHA256 (domínio `cosca-secrets-vault-v1`), arquivo `0600` via `privfile.EnsurePrivateDBFile`, WAL, upsert, `EncryptForExport`/`DecryptFromImport`. Guarda `secrets.db`. |
| **Cofre de conhecimento** | `.cosca/knowledge.db` + chain | `SECURITY_PROTOCOL` chama de "Cofre": conhecimento, segredos, chain. **Conteúdo** da Zona Cofre. |
| **Jaula (isolation)** | `pkg/cosca/jail.go` + `jail_linux.go` + `jail_windows.go` | Auto-jail bwrap fail-closed: reexecuta o binário na bolha, `--clearenv`, segredos via `jail-secrets.env` (nunca no argv), `--info-fd` prova de boot, `jailAllowedEnv` (só `COSCA_*` cruza), constraints fail-closed. **Linux-only.** |
| **Windows** | `deploy/README-WINDOWS.md` | Runtime win32. `jail_windows.go` reporta `jailAvailable()=false`; `ReexecInJail()` cai no `jailFallback` → **SECURITY WARNING** + só continua com opt-in `COSCA_ALLOW_NO_ROOT=1`. Tarefa agendada `cosca-serve` em `127.0.0.1` (loopback), CORS off, registro público off. **WSL2 ativo, SEM distro instalada ainda.** |
| **Regra de ouro / Lei do cofre** | `internal/embed/cosca/MODEL_PROTOCOL.md` §5 | "Local primeiro, nuvem só se valer"; "NUNCA enviar o contexto do cérebro à nuvem" — identidade, memória, conhecimento e conversas são propriedade da família; nuvem é um túnel que entra/sai do cofre sem ninguém ver (L46). |
| **Oráculo de medição** | `internal/evals/ORACLE_SPEC.md` | Oráculo **de fitness** (avaliar descoberta/execução via `secret_verify` caixa-preta) — **conceitualmente distinto** do Oráculo semântico de fronteira. Não confundir os dois. |

> Conclusão do mapa: **a semântica da fronteira já existe e está conectada no serve** (`internal/oracle` + `oracleGate`). O que **não existe** é: (1) a **separação formal de rede** entre as 2 zonas (air-gap do Cofre), (2) a **definição do que reside em cada zona**, (3) a **IA local no loop** com a garantia de que ela **nunca promota REJECT→ACCEPT**, (4) o **jail efetivo em Windows/WSL2** (hoje `bwrap` inexistente + sem distro), e (5) um **documento formal** para o que hoje é só um plano no `cognitive-state.md`.

---

## 1. Contexto / Problema (por que as 2 zonas)

O Cosca tem duas naturezas que tensionam entre si:

1. **O Cofre é a identidade.** Conhecimento, memória, conversas e segredos são propriedade da família (MODEL_PROTOCOL §5.4, L46). Isso é o ativo mais sensível — nunca deve ir à nuvem.

2. **O Kernel precisa do mundo.** Descoberta, verificação de dependências, consulta a fontes externas, APIs, internet. Isso é necessidade de exploração.

O risco atual: **não há fronteira de rede** entre "o que pensa" e "o que explora". Um único processo (serve) mistura os dois, com a mesma sessão, o mesmo contexto e — hoje, sem jail no Windows — o mesmo sandbox vazio (`COSCA_ALLOW_NO_ROOT=1` default nos scripts). Se o Kernel (exploração, com internet) é comprometido, ele arrasta o Cofre (identidade) junto. A "Lei do cofre" é uma regra declarada, mas **não é uma propriedade arquitetural garantida**: nada impede hoje que uma request externa toque a memória assinada com privilégio de execução.

**Problema em uma frase:** separar a capacidade de *exploração* (Kernel, internet) da capacidade de *validação + memória* (Cofre, air-gap), com o Oráculo como o único ponto de entrada entre elas, e a IA local apenas *compreendendo* — nunca *liberando*.

---

## 2. Decisão (as 2 zonas)

**Adotar** a arquitetura de **2 zonas**, air-gap por princípio:

| Zona | Identidade | Rede | Papel |
|---|---|---|---|
| **Zona Cofre** (a jaula) | IA local (ollama, modelo local qwen) + **Oracle puro** + data-vault (`knowledge.db`, `secrets.db`, `chain`) | **AIR-GAP — sem internet** (regra: nenhuma chamada de saída; entradas só pelo Oráculo) | **Pensar, validar, lembrar.** Compreende o que o Kernel trouxe; decide ACCEPT/CAVEAT/REJECT/INCONCLUSIVE; guarda a memória assinada. |
| **Zona Kernel** (Cosca externo) | Orchestrator, CLI, pipeline agentes, providers, SDKs | **COM internet** | **Explorar, executar, descobrir, construir.** Busca, roda testes, consulta fontes externas. Nunca escreve na memória assinada diretamente — sempre via Oráculo. |

**Princípio estrutural (a regra que sustenta tudo):** *aquilo que entra na Zona Cofre não ganha autoridade por entrar.* Toda informação externa é `EXTERNAL INPUT` até passar pelo Oráculo. A Zona Cofre **não confia** no Kernel — confia apenas no pacote semântico validado.

**Princípio de separação de rede:** a Zona Cofre é **air-gap por construção**. Isso significa:
- Nenhum egresso de rede permitido (tudo out-bound bloqueado); o único caminho é o **Oráculo**, um ponto de entrada unidirecional.
- A Zona Cofre **não** resolve DNS, **não** faz handshake de internet, **não** chama APIs remotas. O modelo local (ollama/qwen) é o único provedor de IA — não há fallback para a nuvem dentro do Cofre.
- A Zona Kernel tem internet, mas **nunca** acessa diretamente o data-vault; passa pelo Oráculo.

**Status:** Proposed (pendente de revisão cosca-cto + cosca-security + Don). A implementação não é parte deste ADR.

---

## 3. Desenho Técnico

### 3.1 Definição formal das zonas

```
┌────────────────────────────── ZONA KERNEL (internet) ──────────────────────────────┐
│  Orchestrator · CLI · pipeline agentes · providers · SDKs                         │
│  explora / executa / descobre / constrói  →  monta o PACOTE SEMÂNTICO              │
└──────────────────────────────────────┬──────────────────────────────────────────────┘
                                       │  ÚNICO ponto de entrada do Cofre
                                       │  (contrato + contexto + proveniência + evidência)
                                       ▼
┌──────────────────────────── ZONA COFRE (air-gap, sem internet) ────────────────────┐
│  ORÁCULO (Gate fail-closed)  ──▶  DECISÃO                                           │
│     ▲                                     ACCEPT | ACCEPT_WITH_CAVEAT                │
│     │ valida SIGNIFICADO                   REJECT | INCONCLUSIVE                     │
│  IA LOCAL (ollama/qwen) ── compreende ──▶ vira INPUT, nunca vira VEREDITO           │
│  DATA-VAULT (knowledge.db · secrets.db · chain) — memória ASSINADA                  │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 O que reside em cada zona

**Zona Cofre (air-gap):**
- `internal/oracle` — `Gate.Evaluate()` (determinístico, fail-closed) + `SearchRequest`/protocolo.
- IA local: **ollama + modelo local qwen** (único executor de IA; leitura de contexto via MODEL_PROTOCOL §5 — local primeiro, token por token, **nunca** payload do cérebro à nuvem).
- Data-vault: `internal/secrets` (AES-256-GCM), `knowledge.db`, `family_chain.dat`, memória assinada (raízes verificadas, L418).
- `internal/embed/cosca/` — identidade, leis, protocolos (read-only).

**Zona Kernel (internet):**
- Orchestrator, pipeline de agentes, `internal/circuitbreaker`, providers (openai/deepseek via **payload sanitizado + aprovação por sessão**), SDKs, CLI, `internal/search` (busca híbrida), `internal/knowledge` (engine, com facada no Oráculo — ver §3.4).
- É o único com egresso de rede (**fora do Cofre**).

### 3.3 Princípio de separação de rede (air-gap)

A garantia central **não é** "confiar que o modelo não sai" — **é** "a Zona Cofre não tem rota de saída". Mecanismos (em ordem de prioridade):

1. **Isolamento de processo/namespace** (ideal): `bwrap` (Linux) com `/proc/sys/net` restrito, sem bind das interfaces de rede do host, `--unshare-all` + `--share-net` **não** (para o Cofre, o padrão é **nao compartilhar rede**). Dentro da bolha, o nome de rede é isolado e não há rota default.
2. **Windows/WSL2**: como `bwrap` é Linux-only e o host é win32, o caminho é **WSL2** (v2 ativo, kernel 6.1, **SEM distro ainda**) — mas o WSL2 default usa NAT da Hyper-V **com internet**. **Air-gap exige** remover a rota default (`ip route del default`), desabilitar o DNS (`/etc/resolv.conf` → vazio quebrado), e bloquear egresso. **Se não der para garantir isso, o Cofre não está air-gap** (ver §3.7 e Riscos).
3. **Camada 2 (native Windows)** quando o WSL2 não baste: `AppContainer`/`Job` + **Low-IL** + **firewall out-block** por perfil (item 4 do plano pendente). É o fallback de fail-closed.
4. **Audição contínua**: qualquer tentativa de egresso (socket out) é **registrada como BREACH** e vira falha (fail-closed), nunca apenas log de passagem.

> **Verdade crítica**: hoje (win32 + WSL2 sem distro) **nenhuma** das garantias acima está estabelecida. O Cofre está, na prática, rodando sem isolamento. Este ADR define o **alvo**; a fatia 1 (§6) não pode declarar "2 zonas ativas" até a rota de egresso estar bloqueada de verdade.

### 3.4 Fluxo de entrada de dados (Kernel → Oráculo → DECISÃO)

O Kernel **nunca** entrega um valor cru. Entrega um **pacote semântico** (o que já existe em `SemanticPackage`) + **protocolo de busca** (o que já existe em `SearchRequest`). Fluxo:

1. **Kernel** monta: `Intent` (por que) · `Result` (o que achou) · `Source` (de onde) · `Provenance` (origem verificável: commit/hash/URL) · `Evidence` (o que sustenta: comando/teste/medição) · `Context` (cenário) · `Actions`/`Changes` · `Confidence`.
2. **Ingress point único**: um único ponto de entrada na Zona Cofre (equivalente ao `oracleGate` já no `RunHandler`). Nada entra no Cofre por outro caminho.
3. **Oráculo (camada 1 — determinística, sem LLM)**: `Gate.Evaluate(pkg)`:
   - Pacote vazio → **REJECT**; sem intenção → **REJECT**; sem resultado → **REJECT**.
   - Resultado sem evidência+proveniência+confiança → **INCONCLUSIVE** + perguntas (nunca inventar).
   - `Confidence HIGH` sem evidência → **REJECT**; sem proveniência → **REJECT**.
   - Afirmação causal sem evidência causal (correlação ≠ causa) → **ACCEPT_WITH_CAVEAT**.
   - Contradição com memória assinada → **ACCEPT_WITH_CAVEAT** + objeto de investigação.
   - Contrato completo + evidência + proveniência → **ACCEPT**.
4. **Oráculo (camada 2 — memória assinada):** `Gate.MemoryCheck` valida contra as raízes verificadas (L418). **Se a raiz não bate, para e pergunta** (invariante da casa: valida contra o ASSINADO, nunca o solto).
5. **IA local no loop (compreensão, não veredito):** a IA local (ollama/qwen) pode **enriquecer a compreensão** — ler os gaps, sugerir perguntas, ajudar a interpretar se o resultado é causal ou correlacional, detectar nuances semânticas que o string-matching determinístico não pega. **Mas:**
   - **A saída da IA local é INPUT, nunca VEREDITO.** O `Decision` final vem **só** do `Gate.Evaluate()` determinístico.
   - **Proibido**: IA local não pode converter `REJECT`→`ACCEPT`, não pode promover `HYPOTHESIS`→`FACT`, não pode "explicar de outra forma" um pacote que a camada 1 rejeitou.
   - **Posição no fluxo**: a IA local age **dentro** de `Gate.MemoryCheck`/`Evaluate`, como um *compreensor* que preenche `SemanticGaps`/`Questions`, **nunca** como *votante* que terceiriza a decisão.
6. **DECISÃO** → `Verdict{Decision, ContractGaps, SemanticGaps, Required, Questions, Reason}` → feedback estruturado ao Kernel (nunca só "invalid").

**Regra de segurança do loop:** *o determinístico decide; a IA compreende.* Fail-closed em dois níveis: (a) se a camada determinística não passa, REJECT/INCONCLUSIVE vence sem apelação; (b) se a IA local falhar/indisponível, o Cofre **degrada para L0** (determinístico puro) — **nunca** abre a exceção "deixa passar porque a IA acha que é bom".

### 3.5 Compatibilidade com o que JÁ existe

| Exigência do ADR | Reuso existente | Adaptação |
|---|---|---|
| Contrato semântico | `SemanticPackage` (`internal/oracle/oracle.go`) | **Já pronto.** O fluxo Kernel→Oráculo já é este tipo. |
| Gate fail-closed | `Gate.Evaluate()` + `Decision`/`Verdict` | **Já pronto** (regras 1-7 + §10 correlação≠causa). |
| Ingress point no serve | `oracleGate` no `RunHandler` | **Já conectado.** Revou de "fronteira do Cofre §0". |
| Protocolo de busca | `SearchRequest.Valid()` (`internal/oracle/search.go`) | **Já pronto.** Pode ser acoplado ao ingress (Kernel sempre monta `SearchRequest` antes de buscar). |
| Cofre de segredos | `internal/secrets/vault.go` (AES-256-GCM) | **Já pronto** e já reside na "Zona Cofre" por convenção. |
| Isolamento (Linux) | `pkg/cosca/jail.go` (bwrap fail-closed) | Reusar; **adicionar** restrição de rede dentro da bolha (o jail atual não bloqueia egresso — hoje o `--clearenv` + `jailAllowedEnv` protege env, mas não rede). |
| Regra de ouro / Lei do cofre | `MODEL_PROTOCOL` §5 | **Já é a regra de negócio** que o ADR transforma em propriedade arquitetural. |
| Egresso auditado | `internal/security` (SECURITY_PROTOCOL, watchdog, `cosca memory watch`) | Estender para detectar egresso do Cofre como BREACH. |

### 3.6 O que precisa ser criado (gap real — nada disso existe hoje)

1. **`cage/netpolicy`** (ou em `pkg/cosca`): política de rede de egresso da Zona Cofre — deny-all out, allowloopback apenas para a porta do Oráculo. Enforces o air-gap.
2. **`jail` com restrição de rede**: no bwrap, `--share-net` desligado + bind do oráculo local; no Windows/WSL2, o equivalente (bloqueio de rota default + `resolv.conf` quebrado + AppContainer/Low-IL + firewall out-block).
3. **WSL2 distro + bwrap**: instalar distro + `bwrap` dentro do WSL2 (hoje ausente) — ou validar que o AppContainer/Low-IL nativo basta e descartar bwrap. **Decisão de implementação** (não deste ADR).
4. **`internal/oracle/ai_adapter.go`**: adaptador que faz a IA local *compreender* (preencher `SemanticGaps`/`Questions`) mas **sem poder** tocar `Decision`. Contrato: `func (a *LocalAI) Comprehend(pkg SemanticPackage, gaps []string) (insights []string, err error)` — **nunca** `func (...) (Decision, error)`.
5. **Envelope de entrada** (tamper-evident): contrato estrito + assinatura/HMAC do pacote entre Kernel→Oráculo, para que um Kernel comprometido não injete um pacote forjado que pareça assinado pelo Cofre.
6. **`internal/oracle` w/ NetworkGuard**: `Gate` ganha um `EgressGuard` opcional que, se detectar tentativa de egresso durante `Evaluate()`, marca **REJECT + BREACH** (fail-closed), nunca log-ignora.

---

## 4. Impacto e o que NÃO muda

**Muda (aditivo):**
- Novas garantias de rede (air-gap) e novos componentes (NetPolicy, AI adapter, envelope, EgressGuard).
- `Gate.Evaluate()` ganha o estado "causal" mais fino (hoje é string-matching) via IA local como **compreensor** — mas o veredito continua determinístico.
- A Zona Cofre passa a **não** ter egresso; o Kernel mantém o egresso.

**NÃO muda (o que já é a fundação):**
- `internal/oracle` (semântica + gate) — intocado, é o coração do ADR.
- `internal/secrets/vault.go` — intocado.
- `api/rest/handler/run.go` (`oracleGate`) — intocado; é o protótipo do ingress point.
- `internal/knowledge` / `internal/search` (no Kernel) — mantidos; só perdem acesso direto ao data-vault (passam a via Oráculo).
- `pkg/cosca/jail.go` fail-closed — intocado como contrato; ganha a restrição de rede.

**Impacto operacional:**
- Toda request que hoje toca a memória assinada passa a ir pelo Oráculo (latência + clareza de decisão).
- O Cofre passa a exigir **prova de air-gap** antes de ser chamado de "Cofre" — sem isso, o runtime cai para o modo de menor privilégio com aviso.

---

## 5. Trade-offs / Riscos

| Risco | Mitigação |
|---|---|
| **Air-gap do Cofre é só "best-effort" (rede vaza via WSL2 default NAT).** No Windows o WSL2 padrão tem internet; se a rota default / DNS não for explicitamente derrubado, o "sem internet" do Cofre é uma **meia-verdade** e a premissa do Oracle puro quebra. | Só declara "2 zonas ativas" quando a rota de egresso estiver bloqueada (teste de egresso falhando), com **AppContainer/Low-IL + firewall out-block** como segunda camada. Adicionar um `cosca cofre netcheck --strict` que falha se houver qualquer egresso. |
| **`COSCA_ALLOW_NO_ROOT=1` (default nos scripts) mina o fail-closed.** Se o jail não sobe, o Cofre roda sem isolamento *e o ADR fica só no papel*. É o item 3 do plano pendente (remover opt-in default). | Remover o default; em produção o runtime deve **negar** acesso ao data-vault sem jail/air-gap provado. Este ADR trata o Cofre como zona **não-confiável até prova de isolamento**. |
| **REJECT/INCONCLUSIVE excessivo (falso-negativo de disponibilidade).** O gate fail-closed pode bloquear requests legítimas do Kernel, matando a UX de exploração. | Manter `ACCEPT_WITH_CAVEAT` como modo default para "resultado útil com incerteza conhecida", REJECT só para contrato/semântica incompatível ou HIGH sem evidência; `INCONCLUSIVE` + perguntas em vez de `REJECT` quando falta base (não é falha, é "ainda não sabemos"). |
| **IA local vira ponto de fraqueza / promotor não-autorizado.** Se a IA local puder influenciar a decisão, um pacote malicioso pode ser "persuadido" a ACCEPT (jaula de LLM). | Contrato rígido: IA local devolve `insights`, **nunca** `Decision`. O veredito é 100% `Gate.Evaluate()` determinístico. Teste de regressão: nenhum caminho de código permite que a IA local mude um `REJECT`. |
| **Envelope entre zonas sem integridade.** Um Kernel comprometido (com internet) pode forjar um pacote que pareça legítimo. | Assinatura/HMAC do pacote; a memória assinada (raízes, L418) é a âncora — pacote sem raiz válida é `EXTERNAL INPUT` (nunca FACT). |

---

## 6. Fronteira do incremento — Fatia 1 recomendada (valor isolado primeiro)

**Escopo bounded (o que dá o valor imediato, sem over-engineering):**

1. **Formalizar o ADR** (este documento) — o desenho deixa de ser só um plano no `cognitive-state.md`.
2. **`internal/oracle/ai_adapter.go`** — IA local só compreende (`Comprehend`), nunca decide; testes provam que `REJECT` é inalterável.
3. **Envelope Kernel→Oráculo** — contrato + assinatura do pacote (tamper-evident).
4. **`cosca cofre netcheck --strict`** — comando que bloqueia o egresso e falha se houver qualquer saída (a prova objetiva do air-gap).
5. **Política de rede do Cofre** — deny-all out.

**Critério de saída da fatia 1:** `go test ./internal/oracle/...` verde; teste que **prova** que a IA local não altera `REJECT`; `cosca cofre netcheck --strict` **falha** ao detectar qualquer egresso; zero regressão em `go test ./...`.

**Gate de custo explícito (o que está FORA):**
- Instalar/validar WSL2-distro + bwrap — **decisão de implementação** (pode cair para AppContainer/Low-IL nativo). Não é pré-requisito para o ADR, mas **é** para o air-gap real.
- Migrar o data-vault de SQLite para algum canal mais forte — **fora de escopo**; `secrets.vault` já é AES-256-GCM.
- Isolamento físico/VM ou hardware root-of-trust — **fase posterior** (custo alto, benefício baixo para um bitol local-first).

---

## 7. Alternativas Consideradas

| Alternativa | Veredito |
|---|---|
| **Uma única zona (status quo):** serve único, mesma sessão, mesma internet, mesma memória. | **Rejeitado como alvo.** É exatamente o risco que a Lei do cofre (MODEL_PROTOCOL §5.4) e a L46 já punem. Não tem fronteira: comprometeu o Kernel, vazou o Cofre. |
| **Cofre na nuvem (VPC privada)**: separar por conta/credenciais em vez de por rede. | **Rejeitado.** Contradiz a regra de ouro (contexto do cérebro nunca à nuvem) e o L46. Air-gap local é o único padrão que satisfaz "propriedade da família". |
| **Oráculo com LLM remoto como juiz** (delegar a decisão a um LLM na nuvem). | **Rejeitado.** Anti-pattern "usar LLM para validar LLM" sem âncora; não-determinístico; exige sacar o cérebro pra nuvem. **Compatível** apenas com o Oráculo *de medição* (`ORACLE_SPEC.md`, `secret_verify` caixa-preta) — que é outro conceito. |
| **Apenas regime de permissões** (RBAC/ACL no banco) sem separar rede. | **Rejeitado como suficiente.** RBAC não impede um processo comprometido de ler/escrever; a separação de rede é o que dá a garantia real (defesa em profundidade). |
| **Cofre depende do Modelo de IA (um único provider local, sem fallback).** | **Adotado.** A IA local (ollama/qwen) é o único executor dentro do Cofre; regra de ouro §2 (local primeiro) satisfeita por construção. |
| **Mover o Oráculo para o Kernel** (validar fora, mandar só o ok). | **Rejeitado.** Se o Oráculo fica no Kernel, um Kernel comprometido ignora o Oráculo. O Oráculo **reside dentro do Cofre** (ORACLE_PROTOCOL §0) — validação não pode estar no lado que quer ser validado. |

---

## 8. Related

- `.opencode/cosca/memory/context/cognitive-state.md` — item 2 do plano pendente (fonte do desenho).
- `internal/embed/cosca/ORACLE_PROTOCOL.md` — a fronteira semântica do Cofre (fonte normativa; §0, §4, §5, §7, §8, invariantes §25-§29).
- `internal/embed/cosca/MODEL_PROTOCOL.md` §5 — regra de ouro / Lei do cofre (a regra de negócio que este ADR arquiteturaliza).
- `internal/embed/cosca/SECURITY_PROTOCOL.md` — a "Casa": Cofre (dados), Cérebro, Chave, Runtime.
- `internal/oracle/oracle.go` + `internal/oracle/search.go` — o coração do Oráculo (reuso).
- `internal/secrets/vault.go` — AES-256-GCM (data-vault).
- `api/rest/handler/run.go` — `oracleGate` (ingress point já conectado).
- `pkg/cosca/jail.go` + `jail_linux.go` + `jail_windows.go` — isolamento fail-closed (Linux-only; Windows = fallback com opt-in).
- `deploy/README-WINDOWS.md` — runtime win32; base para a camada 2 (AppContainer/Job/Low-IL/firewall).
- `internal/evals/ORACLE_SPEC.md` — Oráculo **de medição** (distinto; não confundir).
- `docs/adr/ADR-011-rag-fidelity-gates.md` — precedente de formato ADR + mapeamento do estado atual.

---

> **Decisão pendente de revisão (cosca-cto + cosca-security + Don).** O desenho formaliza o que já existe como substância (`internal/oracle` + `oracleGate` + `secrets.vault` + `jail` fail-closed) e isola o gap real: air-gap efektivo + IA local compreendendo (nunca decidindo) + envelope íntegro. A fatia 1 é bounded e dá valor isolado — a separação de rede só é "real" quando o `netcheck --strict` prova que o Cofre não tem egresso.
