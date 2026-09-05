# Revisão do Contrato de Autoridade — SPEC vs Código Existente

**Data:** 2026-09-04
**Revisor:** cosca-architecture (documentação apenas — **sem** implementar, sincronizar, deletar, gerar embed, commitar ou alterar boot/código/config).
**Alvo:** validar `docs/specs/frozen-live-authority-contract.md` (FROZEN > LIVE > RUNTIME) contra o estado real do código.
**Método:** leitura/análise estática + medições (`grep`/contagens/hash). Nenhuma escrita em código, embed, boot ou `.opencode/`.

---

## 0. Convenção de classificação

| Etiqueta | Significado |
|---|---|
| **`FACT`** | Verificado no código/disco (arquivo:linha). |
| **`MEASURED`** | Valor/contagem lida do disco. |
| **`CONTRADICTION`** | O código faz algo que a SPEC **proíbe** ou que contradiz o modelo FROZEN>LIVE>RUNTIME. |
| **`RISK`** | Impacto/probabilidade do problema (operacional, de autoridade, de quebra do OpenCode). |
| **`DECISION REQUIRED`** | Decisão que precisa do Don/professor antes de qualquer ação. |

> Regra de leitura: a SPEC tem guard-rails **G1–G6** (§1.1) e as seções §2–§6. Onde o código viola uma guard-rail, registro `CONTRADICTION` + `RISK` + `DECISION REQUIRED`.

---

## 1. RESUMO EXECUTIVO

O código protege bem o **FROZEN** e o **RUNTIME**, mas existe uma **assimetria grave**: o **LIVE (`.opencode/cosca/`) não é protegido por nenhuma guard-rail equivalente**. A confirmação mais forte disto é:

- Há **4 guardas** impedindo remoção/escrita implícita no FROZEN (`level/gate.go`, `policy/policy.go`, `kernel/identity.go` Lei #8, `proposal/rules.go`).
- Há **1 rotina** que **apaga o LIVE** (`cosca upgrade` → `os.RemoveAll(".opencode/cosca")`) **sem passar por nenhuma dessas guardas**, **sem backup da árvore**, e **sem nenhuma referência na lista de "destinos sensíveis"** — enquanto o `.opencode/opencode.json` referencia o LIVE em **122 caminhos distintos (200 ocorrências)**.

**Veredicto preliminar:** a SPEC está **correta** quanto ao destino (FROZEN>LIVE>RUNTIME), mas o **código atual está em contradição** com ela em pelo menos **3 pontos concretos** (A1, A2, A3), com **2 lacunas de segurança** (B1, B2) e com **2 nuances de taxonomia** (C1, C2).

---

## 2. ACHADOS — CONTRADIÇÕES (código viola a SPEC)

### A1. ⚠️ CRÍTICA — `cosca upgrade` DELETA o LIVE (`.opencode/cosca/`) sem guarda nem backup

- **`FACT`** `internal/cli/upgrade.go:177-184` (`detectLegacyIssues`) adiciona `.opencode/cosca/` à lista de "legacy issues":
  ```go
  opencodeCosca := filepath.Join(dir, ".opencode", "cosca")
  if info, err := os.Stat(opencodeCosca); err == nil && info.IsDir() {
      issues = append(issues, LegacyIssue{ ..., Issue: "Framework legado (.opencode/cosca/)",
          Action: "remover (framework agora vive em internal/embed/cosca/)" })
  }
  ```
- **`FACT`** `internal/cli/upgrade.go:131-137` executa a limpeza:
  ```go
  for _, iss := range issues {
      formatter.Bullet("Limpando: " + iss.Path)
      if err := os.RemoveAll(iss.Path); err != nil { ... continue }
      report.ActionsTaken = append(report.ActionsTaken, "removed → "+iss.Path)
  }
  ```
  Ou seja, **fora do `--dry-run`**, `cosca upgrade` faz `os.RemoveAll(".../.opencode/cosca")`.
- **`FACT`** O backup em `upgrade.go:120-128` copia **`.cosca/`** para `.cosca/backups/pre-upgrade-*`. **`.opencode/cosca/` NÃO é coberto**: a árvore é apagada **sem snapshot**.
- **`FACT`** `osm` de referências: **`MEASURED`** `.opencode/opencode.json` contém **200 ocorrências** de `.opencode/cosca/` e **122 caminhos** distintos, incluindo:
  - `KERNEL.md` (10×), `HELP.md`;
  - `engines/{documentation,evolution,planning,wizard}/SKILL.md` (4×);
  - `workflows/{bug-fix,code-review,project-init,refactoring}.md` (4×);
  - `memory/context/cognitive-state.md`, `memory/testing/patterns.md`;
  - `shared/AUTO_EVOLUTION_PROTOCOL.md`, `shared/PROJECT_CONTEXT.md`;
  - **~55×** `agents/<agente>/PROMPT.md` e **~55×** `memory/agent/<agente>/learnings.md`.
- **`FACT`** Os 11 `command` templates (`deploy, docs, evolve, feature, fix, help-cosca, init, plan, refactor, review, status`) e o `default_agent: cosca-kernel` dependem desses arquivos.
- **`CONTRADICTION`** Viola **G2** da SPEC (§1.1: "`.opencode/cosca/` **não** é removido") e **§5** ("nenhuma rotina sobrescreve/deleta… implicitamente"). É a rotina que **deleta o LIVE**.
- **`RISK`** **Máxima.** Executar `cosca upgrade` num projeto Cosca destrói o LIVE: quebra todos os 11 comandos do OpenCode, o `default_agent`, e apaga **toda a memória de evolução dos agentes** (AUTO_EVOLUTION lê/grava `.opencode/cosca/memory/agent/<agente>/learnings.md` — ver `AUTO_EVOLUTION_PROTOCOL.md:6,9`). Não há backup.
- **`DECISION REQUIRED`** **Urgente.** (a) Confirmar se `cosca upgrade` deve **NUNCA** tocar `.opencode/cosca/` (removê-lo da lista de legacy e protegê-lo). (b) Se mantida a migração de legacy, exigir **dry-run obrigatório + backup da árvore + guarda de autoridade** antes de qualquer remoção.

### A2. 🟠 ALTA — `cosca upgrade` também DELETA `.cosca/fallback/` (alvo ativo de materialização)

- **`FACT`** `internal/cli/upgrade.go:186-196` adiciona `.cosca/fallback/` à lista de legacy ("Sync target obsoleto"), com `Action: "remover (framework sync agora usa .cosca/framework/)"`.
- **`FACT`** `internal/embed/embed.go:107-148` (`MaterializeFallback`) escreve **em `.cosca/fallback/`** (não `.cosca/framework/`), com `FallbackDirs = {"knowledge","memory","workflows","engines"}` (`embed.go:100`).
- **`FACT`** `cmd/cosca-indexer/main.go:113,423` indexa `.cosca/fallback/memory/agent/` como fonte **`scope=global`** ("fallback").
- **`MEASURED`** `.cosca/fallback/` contém **15343 arquivos** — está **ativo**, não é "obsoleto".
- **`CONTRADICTION`** Violação dupla: (a) o comentário "agora usa `.cosca/framework/`" está **desatualizado** (o destino real é `.cosca/fallback/`); (b) apagar `.cosca/fallback/` **destrói estado derivado materializado** e o indexador perde a fonte `fallback` até a próxima materialização.
- **`RISK`** Moderado—alto. A árvore é regenerável (vem do FROZEN), mas o `upgrade` a apaga **sem preservar** e deixaria o índice de conhecimento sem a fonte `fallback` global no curto prazo.
- **`DECISION REQUIRED`** Confirmar o destino de materialização canônico. Se é `.cosca/fallback/`, o `upgrade` não deve apagá-lo; o texto "agora usa `.cosca/framework/`" deve ser corrigido.

### A3. 🟠 MÉDIA — `cosca skills sync` ESCREVE no FROZEN (`internal/embed/cosca/skills/`) implicitamente

- **`FACT`** `internal/cli/skills.go:50` chama `runSkillsSync(cmd, "internal/embed/cosca/skills", ...)`.
- **`FACT`** `internal/cli/skills.go:71-84` faz `os.WriteFile(catalog, ...)` em `filepath.Join(root, skillsCatalogName)` = `internal/embed/cosca/skills/SKILLS_CATALOG.md`. Ou seja, `cosca skills sync` **regrava um arquivo dentro do FROZEN**.
- **`FACT`** A regravação é implícita (o comando `skills sync` não é "promoção live→frozen", é um "gerar inventário" que sobrescreve no caminho do embed).
- **`CONTRADICTION`** Viola **§4** (promoção live→frozen exige processo explícito) e **§5** ("nenhuma rotina sobrescreve o FROZEN implicitamente"), e **G6** (ausência de fluxo de proteção à escrita no embed). Não passa por `level/gate.go` nem por `policy/policy.go` (P8).
- **`RISK`** Baixo-médio (é um arquivo de catálogo/derivado, não conteúdo semântico), mas é uma **brecha do princípio** "só promoção explícita escreve no FROZEN". Uma futura automação baseada nesse padrão poderia escalar para conteúdo semântico.
- **`DECISION REQUIRED`** Decidir se `cosca skills sync` deve (a) escrever **fora** do embed (ex.: `docs/` ou `.cosca/`), ou (b) ser tratado como **promoção explícita** com dry-run + snapshot + gate P8.

---

## 3. ACHADOS — LACUNAS DE SEGURANÇA (a SPEC identifica risco que o código deixa aberto)

### B1. 🔴 ALTA — `sensitiveRmTarget` protege FROZEN e RUNTIME, mas **OMITE o LIVE**

- **`FACT`** `internal/proposal/rules.go:68-84` (`sensitiveRmTarget`) bloqueia remoção recursiva de:
  ```go
  for _, s := range []string{
      "/", "/*", ".cosca", ".git",
      "internal/embed", "internal/embed/cosca",
      "knowledge.db", "gate.db", "audit.db",
      "$home", "~", "/home", "/etc", "/usr", "/var", "/boot",
  } { ... }
  ```
- **`FACT`** A lista contém **`.cosca` (RUNTIME)** e **`internal/embed/cosca` (FROZEN)** — mas **NÃO contém `.opencode` nem `.opencode/cosca` (LIVE)**.
- **`CONTRADICTION`** A SPEC **§5** e **§1.1 G2** exigem que o LIVE seja protegido de destruição implícita. O "cofre" de segurança (`LawRules`/`sensitiveRmTarget`) **deixa o LIVE fora do cofre**.
- **`RISK`** Alto. Agrava o A1: mesmo que uma operação seja *proposta* pelo sistema, o guarda não barraria a remoção do LIVE. A **assimetria** de proteção é exatamente o oposto do que a SPEC pede (FROZEN e RUNTIME protegidos, LIVE vulnerável).
- **`DECISION REQUIRED`** Adicionar `.opencode` / `.opencode/cosca` à lista de destinos sensíveis em `proposal/rules.go` (e auditar rotinas de remoção), para que a remoção do LIVE requeira autorização explícita como a do FROZEN.

### B2. 🟠 ALTA — Lei do Kernel protege FROZEN de remoção, mas **não há lei análoga para o LIVE**

- **`FACT`** `internal/kernel/identity.go:83` Lei nº 8:
  > "Integridade do embed — *Nenhuma remoção do internal/embed/cosca/ sem confirmação explícita do Don.*"
- **`FACT`** O guarda `level/gate.go:180-192` (`isBrainEdit`) nega edição no `internal/embed/cosca` nos níveis L1/L2, permitindo só no L3-SOBERANO (com aval do Don). `policy/policy.go:172-176` também exige "confirmação explícita (P8)" para escrita no cérebro.
- **`CONTRADICTION`** A SPEC **§1.1 G2** exige proteção do LIVE; **não existe** uma lei/governança equivalente para `.opencode/cosca/`. O LIVE é a única zona sem lei de proteção no Kernel.
- **`RISK`** Alto. Sem governança explícita, o LIVE fica sujeito a limpeza acidental (A1) sem qualquer atrito de autoridade.
- **`DECISION REQUIRED`** Criar uma **lei/governança análoga** para o LIVE (ex.: "nenhuma remoção de `.opencode/cosca/` sem confirmação explícita do Don"), espelhando a Lei nº 8.

---

## 4. ACHADOS — NUANCES DE TAXONOMIA (a SPEC deve clarificar)

### C1. 🟡 MÉDIA — `.cosca/fallback/**` é classificado como `scope=global`

- **`FACT`** `internal/knowledge/classify.go:195-196` e `internal/knowledge/ingest.go:169-170` marcam `internal/embed/cosca/**` **e** `.cosca/fallback/**` como `scope=global`.
- **`FACT`** `cmd/cosca-indexer/main.go:418-427` (`scopeFor`): `"fallback"` → `ScopeGlobal`, `"embed"` → `ScopeGlobal`, `"opencode"` → `ScopeProject`.
- **`RISK`** Nuance. A SPEC **§1** diz "RUNTIME é estado **derivado**". Tratar a cópia materializada (fallback) como `global` **conflaciona derivado-com-autoridade**. Na prática é defensável (a fallback é byte-idêntica ao embed), mas é **impreciso** em relação ao modelo: o *conteúdo* é de framework (global), porém a *zona* é derivada (RUNTIME).
- **`DECISION REQUIRED`** Confirmar se a classificação de escopo deve ser **por proveniência de conteúdo** (fallback = global, pois é cópia do embed) ou **por zona** (fallback = derivado). Recomendo registrar como **global** (conteúdo de framework) — mas com **nota explícita** de que é derivado, para não erguer a fallback a "segunda autoridade".

### C2. 🟡 BAIXA — comentário desatualizado "framework lives in `.cosca/framework/`"

- **`FACT`** `internal/editors/opencode/opencode.go:7` e `:33` afirmam "the framework lives in `.cosca/framework/`"; `opencode_test.go:81` repete "the framework lives in .cosca/framework".
- **`FACT`** O framework real consumido é `internal/embed/cosca/` (FROZEN) e `.cosca/framework/` é um **stub de 11 arquivos** (ver relatório de auditoria anterior).
- **`RISK`** Baixo (só comentário), mas é a **semente do modelo mental** que leva o `upgrade` a achar `.opencode/cosca` "legado". Não muda comportamento, mas **reforça a contradição conceitual**.
- **`DECISION REQUIRED`** Corrigir o comentário para refletir o modelo de zonas (FROZEN/LIVE/RUNTIME) no código e no teste, para que a justificativa deixa de alimentar a limpeza incorreta.

---

## 5. ACHADOS — CONSISTENTES COM A SPEC (FACT — não exigem ação)

| # | Arquivo:linha | Comportamento | Alinhamento |
|---|---|---|---|
| F1 | `internal/editors/opencode/opencode.go:21,34,42,198-203` | Boot e prompts referenciam `internal/embed/cosca/...` (FROZEN). | **G1 ✓** (boot lê FROZEN). |
| F2 | `internal/editors/opencode/opencode.go:150-249` | `Setup` escreve apenas `.opencode/opencode.json` (não cria `.opencode/cosca`). | **G2 ✓** (não deleta, não cria a árvore do OpenCode). |
| F3 | `internal/cli/embed.go:65,77` | `cosca embed audit` é **read-only** ("NUNCA modifica nada"). | **§4/§5** — não escreve no FROZEN. |
| F4 | `internal/embed/embed.go:102-106` | `MaterializeFallback` é idempotente e **aditivo** ("stale extra files are left untouched"). | **§3** base correta (não-destrutiva). |
| F5 | `internal/level/gate.go:180-192`, `policy/policy.go:172-176`, `level.go:159` | FROZEN protegido via `isBrainEdit`/P8 (VDeny em L1/L2). | **§5** — ninguém sobrescreve o FROZEN implicitamente. |
| F6 | `internal/cli/fallback.go:8-9` | `materializeFallback` delega a `embed.MaterializeFallback` (FROZEN→disco). | **Direção** correta (embed→fallback). |
| F7 | `internal/knowledge/classify.go:187,199` / `ingest.go:167-172` | `.opencode/cosca/memory/agent/**` → `scope=project`; embed → global. | **§1** — LIVE é superfície/projeto, não autoridade. |
| F8 | `internal/cli/catalog_gate.go` | `cosca gate catalog` compara **LIVE→snapshot-do-LIVE** (integridade interna). | **§8** — é eixo ortogonal, não substitui o detector FROZEN↔LIVE. |

---

## 6. DIAGNÓSTICO SINTÉTICO

A SPEC está **arquiteturalmente correta** (FROZEN>LIVE>RUNTIME), mas a revisão revela uma **inversão de proteção**:

- **A zona que a SPEC quer proteger (LIVE) é a menos protegida no código.** FROZEN tem 3 guardas + 1 lei; RUNTIME tem entry na `sensitiveRmTarget`; **LIVE não tem nenhuma.**
- **Há 1 comando que destrói o LIVE com sucesso** (`cosca upgrade`) — seria o único caminho de violação da guard-rail G2.
- **Há 1 comando que escreve implicitamente no FROZEN** (`cosca skills sync`) — viola o princípio de promoção explícita.

Isto significa que a SPEC precisa ser não só um "contrato", mas um **gatilho para blindar o código**: o primeiro passo de conformidade é fechar as lacunas B1/B2 (proteger o LIVE) e neutralizar/explícitar A1/A2/A3.

---

## 7. LISTA DE DECISION REQUIRED (priorizada)

| Prioridade | Item | Resumo | Zona afetada | Como a SPEC orienta |
|---|---|---|---|---|
| **P0** | **A1** | `cosca upgrade` `RemoveAll(".opencode/cosca")` — deleta o LIVE (122 refs) sem backup. | LIVE | G2, §3, §5 — nunca delete a árvore com referências ativas. |
| **P0** | **B1** | `sensitiveRmTarget` omite `.opencode/cosca` do cofre de remoção. | LIVE | §5 — guarda de autoridade deve cobrir o LIVE. |
| **P1** | **B2** | Sem lei/governança do Kernel protegendo `.opencode/cosca/` de remoção. | LIVE | §1.1 G2 + §5 — criar lei análoga à Lei nº 8. |
| **P1** | **A2** | `cosca upgrade` deleta `.cosca/fallback/` (ativo) + comentário "agora `.cosca/framework/`" errado. | RUNTIME | §1.3 — RUNTIME é derivado; preservar a árvore materializada. |
| **P2** | **A3** | `cosca skills sync` escreve `internal/embed/cosca/skills/SKILLS_CATALOG.md` implicitamente. | FROZEN | §4, §5 — escrita no FROZEN só via promoção explícita. |
| **P2** | **C1** | `.cosca/fallback/**` classificado como `global` (derivado × autoridade). | RUNTIME | §1 — clarificar proveniência vs zona. |
| **P3** | **C2** | Comentário "framework lives in `.cosca/framework/`" desatualizado (opencode.go:7,33; test:81). | — | §1.1 — corrigir o modelo mental que alimenta o `upgrade`. |

---

## 8. CONFIRMAÇÃO DE NÃO-INTERFERÊNCIA

Nenhuma alteração foi feita em código, embed, boot, `.opencode/`, ou config. Este relatório é **somente leitura/análise** — o único arquivo criado é este documento em `docs/reports/`.

> **Observação sobre `git status`:** os itens `.opencode/opencode.json`, `laboratory/*.py` e `internal/agentbus/` já apareciam como `M`/`??` **antes** desta sessão (modificações pré-existentes do working tree). Eu apenas **li** esses arquivos; não os editei. Os únicos artefatos introduzidos por mim nesta sessão são o relatório e, na sessão anterior, o par de documentos de auditoria/SPEC.

---

*Fim do relatório de revisão.*
