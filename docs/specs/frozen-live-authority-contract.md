# Contrato de Autoridade — FROZEN × LIVE × RUNTIME

**Espécie:** Especificação cirúrgica (SPEC) de contrato de autoridade.
**Data:** 2026-09-04
**Autor:** cosca-architecture (documentação apenas; **não** implementa, **não** sincroniza, **não** regenere embed, **não** commita, **não** altera boot).
**Revisado por:** professor/Don (autoridade aprovadora).
**Escopo:** Definir a **fronteira de autoridade** entre o cérebro embutido (FROZEN), a superfície do OpenCode (LIVE) e o estado derivado (RUNTIME). Documentação-only — nenhum `.go`, `.opencode/` ou boot são alterados por esta SPEC.

---

## 0. Convenções de classificação epistemológica

Toda afirmação desta SPEC é rotulada com um dos quatro níveis (repetidos aqui por consistência):

| Etiqueta | Significado | Uso neste documento |
|---|---|---|
| **`FACT`** | Verificado no código/disco atual, independe de interpretação. | Descreve o estado de partida do pipeline. |
| **`MEASURED`** | Valor lido diretamente do disco (hash, tamanho, data, contagem). | Dá os números de ancoragem. |
| **`INFERRED`** | Conclusão derivada de fatos com premissa explícita. | Liga fatos às regras. |
| **`DECISION`** | Regra **normativa** deste contrato (mandato aprovado). | Define o comportamento exigido, incondicionalmente. |

**Regra de precedência entre etiquetas:** onde um `DECISION` colide com um `FACT`/`INFERRED` incompatível, **prevalece o `DECISION`** — o contrato é normative sobre o estado atual, e a implementação deverá convergir para o contrato.

---

## 1. Modelo de autoridade (princípio fundador)

> **`DECISION`** — A fronteira de autoridade é, e permanece:

```text
+--------------------------------------------------------------------------+
|  FROZEN  (internal/embed/cosca/)   →  ÚNICA autoridade semântica         |
|  LIVE    (.opencode/cosca/)          →  superfície operacional ao OpenCode|
|  RUNTIME (.cosca/)                   →  estado derivado                   |
+--------------------------------------------------------------------------+
                                  Precedência:  FROZEN > LIVE > RUNTIME
```

- **`DECISION`** **FROZEN manda.** É o conjunto compilado no binário (`go:embed`), read-only, consumido por boot/init/fallback/ORC/knowledge. É a única fonte da verdade semântica.
- **`DECISION`** **LIVE é superfície, não segunda autoridade.** Serve ao OpenCode (templates, skills, workflows, prompts, memória de edição). É editável e é onde a *evolução* nasce — mas nenhuma divergência do LIVE tem valor semântico *sobre* o FROZEN.
- **`DECISION`** **RUNTIME é derivado.** Todo conteúdo do RUNTIME é regenerável a partir do FROZEN; nenhum estado do RUNTIME é autoridade.
- **`DECISION`** **Regra de precedência:** FROZEN prevalece em qualquer conflito. Se o LIVE diverge, o estado é `DRIFT` e o FROZEN **continua autoridade durante todo o drift** (não há "promoção automática" nem "vitória do mais novo").
- **`DECISION`** **Não matar a evolução; conter a evolução.** O LIVE continua sendo a área onde o agente aprende. O que muda é que aprender no LIVE **não** torna automaticamente aquele conteúdo autoridade — para isso exigirá **promoção explícita e verificável** (§4).

### 1.1 O que deve ser preservado obrigatoriamente (guard-rails)

> **`DECISION`** — As seguintes invariantes **nunca** podem ser quebradas por qualquer rotina de reconciliação, detecção ou promoção:

| # | Invariante | Descrição | Proteção |
|---|---|---|---|
| G1 | **Boot consome FROZEN** | O boot/`opencode.go` continua lendo `internal/embed/cosca/` (KERNEL, DESPERTAR, AUTO_EVOLUTION_PROTOCOL). | Nenhuma mudança pode redirecionar o boot para ler o LIVE. |
| G2 | **OpenCode continua funcionando** | `.opencode/cosca/` **não** é removido; templates de `.opencode/opencode.json` (`KERNEL.md`, `engines/`, `workflows/`, `HELP.md`) continuam resolvíveis. | Nenhuma operação pode deletar/conteúdo-destruir o LIVE. |
| G3 | **AUTO_EVOLUTION preservado** | O protocolo existente (`shared/AUTO_EVOLUTION_PROTOCOL.md`, no LIVE e no FROZEN) não é desativado; a evolução do agente continua acontecendo no LIVE. | Nenhuma rotina pode interromper a escrita de aprendizados no LIVE. |
| G4 | **Capacidade de evolução preservada** | O agente pode continuar aprendendo (memória, blocos, aprendizados) no LIVE. | A reconciliação/promoção nunca deve bloquear a evolução. |
| G5 | **Exclusivos do FROZEN preservados** | Arquivos presentes só no FROZEN (ex.: `memory/agent/*/blocks/`, `merkle/`, `chain.dat`, protocolos) **nunca** são apagados por uma sync. | Nenhum `copy-all`/delete automático. |
| G6 | **Nenhuma operação destrutiva silenciosa** | Qualquer alteração em disco com potencial de perda é precedida de backup/snapshot e dry-run. | Ver §3.3 e §4.5. |

> **`INFERRED`** — Os invariantes G2/G5 estão em tensão com o código atual: `upgrade.go` e `opencode_test.go` declaram `.opencode/cosca/` "legado a remover" e `.cosca/framework/` como o local do framework. Esta SPEC **rejeita** esse modelo de "migração" e o substitui pelo modelo de zonas (FROZEN/LIVE/RUNTIME). A implementação de conformidade com este contrato deverá reconciliar essa contradição **sem** remover o LIVE.

---

## 2. Detector de drift — Frozen ↔ Live

### 2.1 Definição (`DECISION`)

O detector compara **FROZEN ↔ LIVE** (não LIVE↔snapshot-do-LIVE; isso é `cosca gate catalog`, um detector de integridade interna distinto — ver §8).

**Método de comparação:** por **caminho relativo canônico** (normalizado com `/` — lembre-se do bug de `filepath.Rel` no Windows) + **hash de conteúdo** (SHA-256).

### 2.2 Estados de drift (`DECISION`)

| Estado | Condição | Comportamento |
|---|---|---|
| `MATCH` | Arquivo existe em ambos; hash idêntico. | Em sincronia. |
| `LIVE_NEWER` | Existe em ambos; hash difere; conteúdo do LIVE mais novo (ex.: data de escrita ou mtime maior, **ou** base de proposta explícita). | Drift. FROZEN segue autoridade. |
| `FROZEN_NEWER` | Existe em ambos; hash difere; conteúdo do FROZEN mais novo (ou sem proposta correspondente). | Drift. FROZEN segue autoridade. |
| `ONLY_FROZEN` | Existe apenas no FROZEN (ex.: `blocks/`, `merkle/`, `chain.dat`, protocolos). | **Preservar** (G5). Não é deletado, não é copiado ao LIVE a menos que uma promoção explícita o peça. |
| `ONLY_LIVE` | Existe apenas no LIVE (ex.: aprendizado/evolução ainda não promovido). | Drift latente. Registrar como evidência de evolução a ser promovida (se aplicável). |

### 2.3 Regras do detector (`DECISION`)

- **`DECISION`** Drift **nunca** é resolvido silenciosamente. Toda divergência é **reportada** (saída estruturada: caminho, estado, hash de cada lado) e **persistida** como evidência.
- **`DECISION`** **FROZEN continua autoridade durante qualquer drift.** O detector nunca escolhe um "vencedor" com base em "mais novo". O máximo que faz é **classificar** e **orientar** (ex.: `LIVE_NEWER` sinaliza "candidato a promoção"; `FROZEN_NEWER` sinaliza "não promover").
- **`DECISION`** A comparação é **determinística** (hash + caminho). Nenhum LLM decide o estado — LLM é apenas intérprete opcional do relatório.
- **`DECISION`** O detector é **idempotente** e **não-destrutivo**: apenas lê. Não escreve nada.

### 2.4 Saída do detector (`DECISION`)

Cada entrada de drift carrega:

```text
{
  "path":       "<caminho relativo canônico>",
  "state":      "MATCH|LIVE_NEWER|FROZEN_NEWER|ONLY_FROZEN|ONLY_LIVE",
  "frozen_sha": "<sha256 ou null>",
  "live_sha":   "<sha256 ou null>",
  "frozen_m":   "<mtime ou null>",
  "live_m":     "<mtime ou null>",
  "action":     "none|report|preserve|propose_promotion"   // sugestão, nunca aplicada aqui
}
```

---

## 3. Reconciliação — FROZEN → LIVE (materialização segura do LIVE a partir do FROZEN)

### 3.1 Objetivo (`DECISION`)

Torna o LIVE **compatível** com o FROZEN para que o OpenCode veja a superfície alinhada à autoridade — **sem destruir** conteúdo vivo nem **apagar** exclusivos do FROZEN.

### 3.2 Inspiração no comportamento atual (`FACT`)

`embed.MaterializeFallback` (`internal/embed/embed.go:107-148`) já demonstra o padrão correto a seguir para a direção **embed → disco**: idempotente, aditivo (reescreve os embutidos), **deixa arquivos extras intactos** ("stale extra files are left untouched"). A reconciliação FROZEN→LIVE deve adotar o **mesmo espírito**, mas com guard-rails mais rígidos.

### 3.3 Regras da reconciliação (`DECISION`)

> As regras abaixo são **obrigatórias** e se aplicam a qualquer operação que materialize o LIVE a partir do FROZEN.

1. **NUNCA `copy-all` destrutivo.** Nunca apagar/conteúdo-destruir a árvore de destino antes de copiar. Operação é **aditiva** (upsert por arquivo).
2. **NUNCA deletar automaticamente exclusivos do FROZEN.** Arquivos `ONLY_FROZEN` (`blocks/`, `merkle/`, `chain.dat`, protocolos) **jamais** são removidos por uma sincronização — nem do FROZEN, nem por reflexo da eventual tentativa de remover do LIVE (ver G5).
3. **Preservar como EVIDÊNCIA** arquivos do LIVE divergentes (`LIVE_NEWER`) quando forem produtos de evolução legítima. Eles não são sobrescritos cegamente; são retidos e sinalizados como candidatos a promoção (§4).
4. **Operação IDEMPOTENTE.** Rodar 1× ou N× produz o mesmo resultado. Reexecução é segura.
5. **Backup/snapshot ANTES de qualquer alteração.** Antes de qualquer escrita, criar um snapshot/backup do estado de destino (ex.: `.cosca/backups/` ou diretório temporário versionável).
6. **DRY-RUN obrigatório antes da aplicação.** A operação **deve** suportar `--dry-run`, que imprime exatamente o conjunto de ações (criar/atualizar/ignorar/preservar) sem escrever. **Dry-run é pré-requisito** para aplicar.
7. **Falha segura (fail-closed).** Se qualquer invariante G1–G6 for violado, a operação **aborta** e não escreve nada.

### 3.4 Matriz de tratamento por estado (`DECISION`)

| Estado | Ação da reconciliação FROZEN→LIVE |
|---|---|
| `MATCH` | Nada (já em sincronia). |
| `FROZEN_NEWER` | Atualizar o LIVE a partir do FROZEN (FROZEN é autoridade). |
| `LIVE_NEWER` | **Não** sobrescrever automaticamente. Manter o LIVE como **evidência** e reportar `LIVE_NEWER` (candidato a promoção §4) — mesmo que por enquanto o FROZEN siga autoridade. |
| `ONLY_FROZEN` | **Preservar** no FROZEN. Não copiar ao LIVE por padrão (a menos que a promoção o exija). Nunca deletar. |
| `ONLY_LIVE` | Reconciliar para **não-bloquear** o OpenCode (o LIVE precisa dos seus arquivos vivos); registrar como drift/evidência. Não é "autoridade nova". |

> **`INFERRED`** — A regra de `LIVE_NEWER` na direção FROZEN→LIVE é a mais sensível: materializar o LIVE a partir do FROZEN **não** pode apagar evolução legítima do agente. Daí o tratamento "preservar como evidência" em vez de "sobrescrever".

---

## 4. Promoção — LIVE → FROZEN

> **Parte mais sensível do contrato.** A promoção é o único caminho legítimo pelo qual conteúdo do LIVE se torna autoridade no FROZEN.

### 4.1 Princípio (`DECISION`)

- **`DECISION`** A promoção **NÃO é automática**. Exige **intenção explícita** (comando/flag dedicado, sem default).
- **`DECISION`** Exige **hash/base revision conhecido** — a promoção referencia a revisão do FROZEN sobre a qual a proposta nasceu.
- **`DECISION`** Exige **re-read imediatamente antes do apply** — nada é promovido com base em estado desatualizado.
- **`DECISION`** **Rejeitar** a aplicação sobre estado alterado (o Frozen mudou desde a criação da proposta → conflito → abortar).

### 4.2 Fluxo da promoção (`DECISION`)

```
[intenção explícita]  →  (1) PROPOSE   registrar proposta (path, live_sha, base_frozen_sha, material, motivo, autor)
                      →  (2) BASE      registrar a revisão FROZEN de origem (base revision)
                      →  (3) COMPARE   detecção de drift para o path proposto (estado LIVE_NEWER necessário)
                      →  (4) REREAD    reler FROZEN imediatamente antes do apply; comparar sha atual x base_sha
                      →  (5) GATE      se FROZEN atual ≠ base_sha  →  CONFLITO  →  REJEITAR (fail-closed)
                      →  (6) APPLY     snapshot BEFORE -> escrever no FROZEN -> verificar hash (idempotente)
                      →  (7) SNAPSHOT  snapshot AFTER (provenance/evidence)
                      →  (8) REBUILD   rebuild do binário (o FROZEN é go:embed; muda exige recompilar)
                      →  (9) TESTES    rodar a matriz de testes (§7)
                      →  (10) AUDIT    rodar auditoria de reconciliação/promoção (§6)
                      →  (11) COMMIT   (fora do escopo desta SPEC — só quando autorizado)
```

### 4.3 Regras de conflito (`DECISION`)

- **`DECISION`** Se o FROZEN mudou desde a criação da proposta (`base_frozen_sha` ≠ `frozen_sha` corrente no reread), a promoção é **rejeitada** (`REJECTED_CONFLICT`). Não existe "merge forçado".
- **`DECISION`** Se o conteúdo do LIVE mudou desde a proposta (`live_sha` corrente ≠ `live_sha` da proposta), a promoção é **invalidade** — a proposta não reflete mais a intenção registrada; exige repropor.
- **`DECISION`** Em qualquer conflito a promoção **aborta** e **nada é escrito**.

### 4.4 Provenance/evidence (`DECISION`)

Cada promoção gera um registro imutável de **provenance** contendo:

```text
{
  "promotion_id":   "<uuid>",
  "path":           "<caminho>",
  "base_frozen_sha": "<sha>",
  "frozen_sha_before": "<sha>",
  "frozen_sha_after":  "<sha>",
  "live_sha":         "<sha>",
  "snapshot_before":  "<local do snapshot>",
  "snapshot_after":   "<local do snapshot>",
  "outcome":          "APPLIED|REJECTED_CONFLICT|REJECTED_INVALID|REJECTED_NO_INTENT",
  "actor":            "<agente/humano>",
  "timestamp":        "<ISO-8601>"
}
```

### 4.5 Pós-promoção (`DECISION`)

Toda promoção aplicada **deve**:
1. **Rebuild** do binário (o FROZEN é embutido; sem rebuild a promoção não está ativa).
2. **Testes** (matriz §7) — exigência de aceitação.
3. **Auditoria** (§6) — verificação determinística de que a promoção foi a única mudança no FROZEN.

---

## 5. Guarda de autoridade

> **`DECISION`** — Camada de proteção que impede violações de precedência.

1. **Nenhuma rotina do OpenCode pode sobrescrever o FROZEN implicitamente.** Só o fluxo de promoção explícito (§4) escreve no `internal/embed/cosca/`. Qualquer outra escrita no FROZEN é **violação**.
2. **Nenhum agente decide sozinho qual lado vence.** A decisão de precedência é do **detector** (determinístico, hash) e o *veredito* de promoção é de um **gate de autoridade** (re-read + conflito). Um agente pode *propor*; não pode *aplicar* unilateralmente.
3. **Em conflito: `FAIL-CLOSED` + `DRIFT`.** Se detectada divergência não resolvida, o sistema **falha fechado** (não escreve, não sobreescreve, não apaga) e **registra DRIFT**. Nunca escolhe um lado por conveniência.
4. **FROZEN sempre prevalece.** Quando há ambiguidade ou conflito, a autoridade é o FROZEN; o estado divergente é `DRIFT` até promoção explícita (que, por sua vez, exige passos de segurança).

---

## 6. Integridade

### 6.1 Manifestos/hashes (`DECISION`)

| Zona | Requisito |
|---|---|
| **FROZEN** | Manifesto/hash do FROZEN (árvore de caminhos + SHA-256), versionável, usado como referência de autoridade e âncora de revisão. Ex.: `catalog.manifest` do FROZEN ou equivalente. |
| **LIVE** | Manifesto/hash do LIVE (caminhos + SHA-256), usado para comparar drift. Ex.: `catalog.manifest` já existente em `.opencode/cosca/` (§8). |
| **Revisão** | Identificação de **version/revision** — cada árvore expõe um id de revisão (ex.: hash raiz da árvore) para detectar "mudou desde a base". |

### 6.2 Auditoria (`DECISION`)

- **Auditoria de reconciliações:** cada reconciliação FROZEN→LIVE registra `reconciled_at`, `mode` (dry-run/apply), `actions` (lista de criar/atualizar/preservar), `snapshot_before`, `outcome`.
- **Auditoria de promoções:** cada promoção (§4.4) é registrada e auditable.
- **`DECISION`** A auditoria é **determinística** (não depende de LLM) e deve ser capaz de responder "quem/porquê/quando mudou este arquivo do FROZEN?".

### 6.3 Rollback determinístico (`DECISION`)

- Toda operação de reconciliação/promoção que escreve tem um **snapshot** (`snapshot_before`) que permite **restaurar** o estado exato.
- **`DECISION`** O rollback é **determinístico**: restaura do snapshot sem ambiguidade (estado anterior conhecido por hash).
- **`DECISION`** Nunca se apaga o snapshot após o apply (retenção mínima configurável; pelo menos `N` versões — padrão de retenção do runtime).

---

## 7. Matriz de testes

> **`DECISION`** — A matriz abaixo **define o critério de aceitação** para qualquer implementação de detecção/reconciliação/promoção.

| ID | Cenário | Resultado esperado |
|---|---|---|
| T1 | `MATCH` (um arquivo idêntico em ambas as zonas) | Detector reporta `MATCH`; reconciliação não altera nada. |
| T2 | `FROZEN_NEWER` (FROZEN mais novo) | Detector reporta `FROZEN_NEWER`; reconciliação FROZEN→LIVE atualiza o LIVE. |
| T3 | `LIVE_NEWER` (LIVE mais novo) | Detector reporta `LIVE_NEWER`; reconciliação **preserva** o LIVE como evidência (não sobrescreve). |
| T4 | Conteúdo divergente (mesmo caminho, hash diferente) | Detector diferencia `LIVE_NEWER`/`FROZEN_NEWER` por hashes; nunca "empata" silenciosamente. |
| T5 | Arquivo só no FROZEN (`ONLY_FROZEN`) | Detector reporta `ONLY_FROZEN`; **nunca** deletado; não copiado ao LIVE por padrão. |
| T6 | Arquivo só no LIVE (`ONLY_LIVE`) | Detector reporta `ONLY_LIVE`; reconciliação não-bloqueia; registro como drift/evidência. |
| T7 | Conflito durante plan→apply (FROZEN mudou entre dry-run e apply) | Promoção é **rejeitada** (`REJECTED_CONFLICT`); nada escrito. |
| T8 | Promoção aprovada | Aplica quando: hash base ok + reread ok + sem conflito; gera provenance + snapshot before/after. |
| T9 | Promoção rejeitada | Rejeita quando: base sha divergente ou live sha inválido; do `REJECTED_CONFLICT`/`REJECTED_INVALID`. |
| T10 | Rollback | Restaura exatamente o snapshot anterior; estado volta ao hash de `snapshot_before`. |
| T11 | Crash durante reconciliação | A operação é idempotente/atômica por arquivo; ao reexecutar, não corrompe; snapshots preservados. |
| T12 | Repetição / idempotência | Rodar reconcile 2× = 1× (mesmo resultado); rodar promote com mesma proposta já aplicada → no-op ou rejeição. |
| T13 | Tentativa de sobrescrever FROZEN sem autorização | Guarda de autoridade (§5) bloqueia; resultado é `FAIL-CLOSED` + registro de tentativa não-autorizada (quando aplicável). |
| T14 | Boot ainda lê FROZEN (G1) | Após qualquer promoção/reconciliação, o boot/`opencode.go` continua referenciando `internal/embed/cosca/`. |
| T15 | `.opencode/opencode.json` templates resolvíveis (G2) | Após reconciliação FROZEN→LIVE, `KERNEL.md`, `engines/`, `workflows/` continuam existindo no LIVE. |
| T16 | Exclusivos do FROZEN preservados (G5) | Após reconciliação, `blocks/`, `merkle/`, `chain.dat` e protocolos do FROZEN intactos. |
| T17 | AUTO_EVOLUTION preservado (G3) | Após tudo, `shared/AUTO_EVOLUTION_PROTOCOL.md` presente e o agente ainda pode aprender no LIVE. |
| T18 | Modelo de separação — zonas distintas | O indexador (`vectorDomain`) ainda distingue `embed-*` / `opencode` / `fallback`. |

> **`INFERRED`** — Os testes T14–T18 garantem que o contrato **não** mata a evolução nem quebra o OpenCode — são os testes de não-regressão dos guard-rails G1–G6.

---

## 8. Relação com mecanismos existentes

- **`FACT`** `cosca gate catalog` (`internal/cli/catalog_gate.go`) **já existe** e detecta drift do **LIVE contra seu próprio snapshot** (`catalog.manifest`), com os invariantes A (INDEX.md), B (frontmatter), C (cross-refs), D (mojibake).
- **`DECISION`** Este contrato **não** substitui `cosca gate catalog`. Trata-se de um **novo eixo ortogonal**: `catalog gate` → *integridade interna do LIVE*; este contrato → *divergência de autoridade FROZEN ↔ LIVE*. Ambos coexistirão.
- **`DECISION`** O detector deste contrato reutiliza o **conceito generate-and-diff** do `catalog` (manifesto + comparação por hash), mas compara **FROZEN × LIVE** (dois cérebros), não LIVE×snapshot-do-LIVE.
- **`INFERRED`** Qualquer implementação futura deve reusar o método de normalização de caminhos já corrigido para Windows (`filepath.ToSlash`) referido no `embed-audit-ingestao-2026-09-01.md` — o mesmo erro (comparar `\` com `/` canônico) não pode se repetir no detector FROZEN↔LIVE.

---

## 9. Resumo dos termos do contrato

| Termo | Definição | Seção |
|---|---|---|
| Autoridade | FROZEN > LIVE > RUNTIME | §1 |
| Guard-rails | G1–G6 (boot, OpenCode, AUTO_EVOLUTION, evolução, exclusivos FROZEN, não-destrutivo) | §1.1 |
| Detector | Frozen↔Live por caminho+hash; estados `MATCH`/`LIVE_NEWER`/`FROZEN_NEWER`/`ONLY_FROZEN`/`ONLY_LIVE`; nunca silencioso | §2 |
| Reconciliação | FROZEN→LIVE aditiva/idempotente; sem copy-all destrutivo; preserva exclusivos; dry-run+backup | §3 |
| Promoção | Live→Frozen **explícita**; base revision; reread; rejeição em conflito; provenance; rebuild+testes+audit | §4 |
| Guarda de autoridade | Nenhum agente decide; fail-closed + DRIFT; FROZEN prevalece | §5 |
| Integridade | Manifestos/hashes; auditoria das reconciliações/promoções; rollback determinístico | §6 |
| Testes | Matriz T1–T18 | §7 |
| Coexistência | Não substitui `cosca gate catalog`; reusa generate-and-diff | §8 |

---

## 10. Fora do escopo desta SPEC

- **`DECISION`** Não opera nenhuma mudança de código, embed ou boot — esta SPEC é normativa (contrato) e documental.
- **`DECISION`** A implementação técnica, quando autorizada, deverá seguir esta SPEC e a matriz de testes (T1–T18), e **nunca** violar os guard-rails G1–G6.
- **`DECISION`** O commit é uma ação separada, autorizada externamente (fora do escopo da SPEC).

---

*Fim da SPEC. Documentação-only: nenhum `.go`, `.opencode/`, embed ou boot foi alterado.*
