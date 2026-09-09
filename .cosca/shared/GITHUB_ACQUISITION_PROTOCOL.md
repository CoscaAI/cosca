# GITHUB ACQUISITION PROTOCOL — Padrão Cosca

> **Versão**: 1.1.0 | **Status**: active | **Owner**: Cosca Kernel | **Atualizado**: 2026-08-24
>
> **O que mudou (v1.0.0 → v1.1.0):** a auditoria adversarial e a inspeção do código real
> revelaram que o v1.0 (a) não modelava autoridade, (b) tratava licença como fato jurídico,
> (c) chamava `acquire --compile/--embed` de "execução do repo" quando NÃO é, e (d) não separava
> aquisição de execução, isolamento real de convenção de caminho. Esta versão corrige tudo isso
> **baseado em FACT verificados no código** — não em recomendações especulativas.
>
> **Leyenda de confiança (usada em todo o documento):**
> - **FACT** — comportamento confirmado no código real (`internal/acquisition`, `internal/knowledge`, `internal/cli`, `internal/indexer`).
> - **POLICY** — decisão de governança da casa (princípio inegociável).
> - **UNKNOWN** — não verificável com o código atual; **UNKNOWN ≠ PASS**.
> - **NOT_IMPLEMENTED** — campo/operação que o código ainda não possui.

---

## PRINCÍPIO (inalterável — POLICY)

Todo repositório externo que a casa estuda passa por este protocolo **ANTES** de ter qualquer
conteúdo absorvido. A cadeia é sacrossanta:

```
PESQUISAR → AVALIAR → PROTEGER → ADQUIRIR → EXPLORAR → ABSTRAIR → ENTENDER → REGISTRAR
```

**Princípios inegociáveis:**
- **REPOSITÓRIO EXTERNO = INSUMO DE ESTUDO, NÃO MODELO.**
- **KNOWLEDGE ≠ DEPENDENCY ≠ CODE.** Conhecer um repo não instala nada, não altera o projeto.
- **NUNCA copiar código.** Estudar o padrão → reescrever do nosso jeito, melhorando.
- **Nenhuma descoberta implica ordem.** Descobrir um repo ≠ autorização para adquiri-lo.
- **CAPABILITY ≠ AUTHORITY.** Ter a ferramenta ≠ estar autorizado a usá-la.
- **UNKNOWN ≠ PASS.** Sem evidência suficiente, NÃO se aprova.
- **PARAR é um resultado válido.** Não é erro.
- **Estudo não autoriza alteração no projeto.**

---

## HIERARQUIA DE AUTORIDADE (POLICY)

```
KERNEL
  │  autoridade máxima; autoriza/nega operações; pode interromper
  ▼
GENERAL
  │  recebe uma ordem AUTORIZADA → transforma em escopo operacional
  │  NÃO pode ampliar o escopo que recebeu
  ▼
AGENT
  │  executa SÓ dentro do escopo recebido
  │  não inventa etapas; não amplia escopo
  │  não transforma descoberta em autorização
  │  PARAR se uma operação exigir algo fora da autoridade recebida
```

**Formalização (a base de tudo):**

> **CAPABILITY ≠ AUTHORITY ≠ ORDER ≠ CONSENT**
> - Ter a ferramenta (`cosca acquire`, `git clone`) **não** é estar autorizado a usá-la.
> - Estar autorizado a pesquisar **não** é estar autorizado a adquirir.
> - Ter adquirido conhecimento **não** é estar autorizado a modificar o projeto.

**Papéis:**
- **KERNEL:** define intenção e autoridade; decide operações que ultrapassam o escopo normal; pode interromper qualquer fase.
- **GENERAL:** converte uma ordem autorizada em steps precisos e limitados; **proibido de ampliar**.
- **AGENT:** executa somente os steps recebidos; **para** diante de pré-condição ausente; reporta `FACT / INFERENCE / UNKNOWN / NOT_EXECUTED / AUTHORITY_BOUNDARY`.

---

## MÁQUINA DE ESTADOS (POLICY)

```
DISCOVERED → CANDIDATE → LICENSE_VERIFIED → SECURITY_VERIFIED → ACQUISITION_AUTHORIZED
          → ACQUIRED → ISOLATED → ANALYZED → ABSTRACTED → UNDERSTOOD
          → PROVENANCE_RECORDED → KNOWLEDGE_ACCEPTED
```

**Estados de parada (qualquer ponto pode terminar aqui):**
`REJECTED · QUARANTINED · BLOCKED · UNKNOWN · EXPIRED · LEGAL_REVIEW_REQUIRED · SECURITY_REVIEW_REQUIRED · AUTHORITY_REQUIRED`

- **Transições automáticas** (leitura, sem autoridade): `DISCOVERED → CANDIDATE`, `LICENSE_VERIFIED`, `SECURITY_VERIFIED` — se evidência presente e sem bloqueio.
- **Exigem autoridade do KERNEL:** `SECURITY_VERIFIED → ACQUISITION_AUTHORIZED` (ordem de fetch remoto), `ISOLATED → ANALYZED` (se envolver qualquer execução), `UNDERSTOOD → KNOWLEDGE_ACCEPTED` (escrever conhecimento na casa).
- **PROIBIDO:** `UNKNOWN → PASS` em qualquer transição.

---

## GATES (POLICY + FACT)

> Um gate é uma **decisão verificável**, não um nome decorativo. Cada gate tem:
> **entrada · evidência · autoridade · operações permitidas/proibidas · PASS · FAIL · UNKNOWN**.

| Gate | Entrada | Evidência | Autoridade | Permite | Proíbe | PASS | FAIL | UNKNOWN |
|---|---|---|---|---|---|---|---|---|
| **RESEARCH_GATE** | desejo de encontrar candidato | consulta GitHub API/search | KERNEL | pesquisar, listar | baixar, adquirir | candidato identificado | nada encontrado | rede/erro |
| **LICENSE_GATE** | candidato | LICENSE/COPYING/NOTICE reais | KERNEL | ler metadado de licença | presumir licença | `LICENSE_VERIFIED` | `LICENSE_HETEROGENEOUS`/`LEGAL_REVIEW_REQUIRED` | `LICENSE_UNKNOWN` |
| **SECURITY_GATE** | LICENSE_VERIFIED | scan + inspeção de scripts/submodule/symlink | KERNEL | inspecionar | executar | `SECURITY_VERIFIED` | `QUARANTINED`/`SECURITY_REVIEW_REQUIRED` | `SECURITY_UNKNOWN` |
| **REMOTE_ACCESS_GATE** | SECURITY_VERIFIED | ordem de fetch; `--allow-remote` | KERNEL | autorizar fetch | fetch sem ordem | `ACQUISITION_AUTHORIZED` | `AUTHORITY_REQUIRED` | `UNKNOWN` |
| **DOWNLOAD_GATE** | ACQUISITION_AUTHORIZED | fetch SSRF-safe (FACT) | KERNEL | baixar doc oficial | instalar/clonar c/ execução | `ACQUIRED` | `BLOCKED` | `UNKNOWN` |
| **ISOLATION_GATE** | ACQUIRED | evidência de fronteira de filesystem | KERNEL | mover para isolado | tratar TEMP como isolado sem prova | `ISOLATED` | `BLOCKED` | `ISOLATION_UNKNOWN` |
| **EXECUTION_GATE** | risco de execução | código/script/binário presente | KERNEL | **nada por padrão** | install/build/run/script/binário | `EXECUTION_APPROVED` | `BLOCKED` | `EXECUTION_UNKNOWN` |
| **EXTRACTION_GATE** | ISOLATED | ler conteúdo | AGENT(GENERAL) | ler e abstrair | copiar | `UNDERSTOOD` | `QUARANTINE` | `UNKNOWN` |
| **KNOWLEDGE_ABSTRACTION_GATE** | UNDERSTOOD | especificação abstrata (sem código) | GENERAL | produzir abstração | transportar código | `ABSTRACTED` | `QUARANTINE` | `UNKNOWN` |
| **KNOWLEDGE_GATE** | ABSTRACTED | aberto p/ conhecimento castelo | KERNEL | registrar conhecimento | instalar/copiar | `KNOWLEDGE_ACCEPTED` | `BLOCKED` | `UNKNOWN` |
| **PROVENANCE_GATE** | qualquer aquisição | campos de proveniência (ver §Proveniência) | GENERAL | registrar metadado | omitir proveniência | `PROVENANCE_RECORDED` | `BLOCKED` | `UNKNOWN` |
| **PROJECT_WRITE_GATE** | tentativa de escrita no projeto | ordem explícita do Don | KERNEL | **nada em modo estudo** | toda escrita no projeto | `WRITE_APPROVED` | `BLOCKED` | `UNKNOWN` |
| **DEPENDENCY_CHANGE_GATE** | tentativa de instalar dep | ordem explícita | KERNEL | **nada em modo estudo** | instalar/adicionar package | `DEP_OK` | `BLOCKED` | `UNKNOWN` |

> **REGRA DE OURO:** `UNKNOWN` nunca é `PASS`. Em qualquer gate com evidência insuficiente →
> **PARAR** e reportar `UNKNOWN` para o KERNEL.

---

## SEPARAÇÃO LEITURA × EXECUÇÃO (POLICY)

| Operação | Classificação | Gate exigido |
|---|---|---|
| **READ** | leitura | nenhum (seguro) |
| **FETCH** (buscar na rede) | `ACQUISITION` | `REMOTE_ACCESS_GATE` (autoridade) |
| **DOWNLOAD** | `ACQUISITION` | `DOWNLOAD_GATE` + `ISOLATION_GATE` |
| **EXTRACT** (ler conteúdo baixado) | `ANALYSIS` | `EXTRACTION_GATE` |
| **PARSE** | `ANALYSIS` | `EXTRACTION_GATE` |
| **COMPILE** (indexar doc adquirida no FTS5) | `ANALYSIS`/`KNOWLEDGE_WRITE` | `KNOWLEDGE_GATE` |
| **EXECUTE** (rodar código do repo) | `EXECUTION` | `EXECUTION_GATE` + `ISOLATION` + `AUTHORITY_REQUIRED` |
| **INSTALL** (dependência) | `DEPENDENCY_CHANGE` | `DEPENDENCY_CHANGE_GATE` — **PROIBIDO em modo estudo** |
| **WRITE** (projeto do cliente) | `CODE_CHANGE` | `PROJECT_WRITE_GATE` — **PROIBIDO em modo estudo** |
| **MUTATE / DESTRUCTIVE** | `DESTRUCTIVE` | sempre KERNEL + ordem explícita |

> **REGR A: DOWNLOAD ≠ EXECUTE.** Um repositório baixado é **dados não confiáveis** até passar por
> `SECURITY_VERIFIED` + `ISOLATED`. `DOWNLOAD → EXECUTE` **nunca é automático.**
> **INSTALL é PROIBIDO por padrão** em modo de estudo.

---

## COMPORTAMENTO REAL DO COSCA (FACT — verificado no código)

Esta seção documenta **o que a implementação realmente faz** — para o documento refletir o código.

### Adquisição (`internal/knowledge/acquire.go`, `internal/acquisition/acquisition.go`)
O pipeline real é:
```
manifest → source URL → fetch (SSRF-safe) → write markdown → compile FTS5
```
- `cosca knowledge acquire <id> --allow-remote` **busca a DOCUMENTAÇÃO OFICIAL** de um
  package registrado, salva em markdown, e atualiza o status de `manifest` → `acquired`.
- **FACT: NÃO clona nem executa o código do repositório.** Só baixa conteúdo documental.

### `--compile` / `--embed` (FACT)
- `--compile` **indexa o markdown salvo** no banco FTS5 local (`.cosca/knowledge.db`),
  habilitando busca offline via `cosca knowledge search`.
- `--embed` adiciona embeddings ao índice (busca semântica), ainda sobre o markdown salvo.
- **FACT: NÃO executam o código do repositório adquirido** — operam sobre o conteúdo
  documental já baixado e fazem indexação no Knowledge Base.
- **Classificação correta:** `ACQUISITION` + `ANALYSIS`/`KNOWLEDGE_WRITE`.

> **NÃO generalize.** O ponto acima vale para `acquire --compile --embed` (doc oficial).
> **NÃO se aplica** a `git clone` + `install`/`build`/`run`/scripts/hooks/package managers/
> executáveis/binários. **Essas continuam `EXECUTION` e exigem `EXECUTION_GATE` separado.**

### Segurança de rede (FACT — `internal/acquisition/acquisition.go`)
Guard de SSRF e hardening (confirmados no código):
- **Esquemas:** apenas `http`/`https`; outros rejeitados.
- **Bloqueio de IPs:** loopback, RFC1918 (privadas), link-local, ULA IPv6 e **metadata cloud**
  (`169.254.169.254`). `blockedIP()` normaliza IPv4-mapped antes de checar.
- **Defense-in-depth no dial:** re-valida o IP no momento do dial (anti DNS-rebinding).
- **Fail-closed `--allow-remote`:** sem a flag, a busca é recusada (`validateTarget`).
- **Timeout:** 5s. **Redirects:** teto de 3. **Body:** cap de 10 MiB.
- **Proveniência inicial:** todo `AcquiredArtifact.Provenance` nasce **`UNTRUSTED`**
  (`EXTERNAL DATA ≠ TRUSTED DATA`). Nada é promovido a conhecimento automaticamente.
- **Orçamento (FACT):** `AcquisitionTracker` contabiliza **fontes, arquivos, bytes de rede,
  duração e tokens de IA**; `Exceeded()` aborta a aquisição se qualquer dimensão estourar
  (default: 8 fontes, 100 arquivos, 20MB, 30s, 8k tokens). `budget check` é **diagnóstico
  read-only**.

### Quarentena (FACT — `internal/acquisition/quarantine_integration.go`)
- `QuarantineArtifact` registra o artefato numa **Proposal Q-XXXX** (`status: pending`),
  com `Source: "external:<url>"` e o corpo **fora do conhecimento** (em
  `.cosca/quarantine/artifacts/A-XXXX`).
- **NUNCA** um dump completo no conhecimento — só referência + trecho curto.
- Isso materializa `EXTERNAL DATA ≠ TRUSTED DATA` e permite **revisão/promoção** posterior.

---

## LICENÇA (POLICY + LEGAL_REVIEW_REQUIRED)

### 5.1 Remoção de conclusões jurídicas automáticas
As seguintes afirmações **foram REMOVIDAS** do v1.0 por serem conclusões jurídicas
simplistas e potencialmente enganosas:
- ~~"MIT = pode copiar"~~
- ~~"GPL contamina (copyleft)"~~
- ~~"sem licença = todos os direitos reservados, portanto descarte"~~

O protocolo **não substitui** essas por outra conclusão jurídica. Quando a consequência
jurídica depender de contexto, marca-se **`LEGAL_REVIEW_REQUIRED`** e **não se presume PASS**.

### 5.2 Classificação operacional de licença
| Estado | Significado | Ação |
|---|---|---|
| `LICENSE_VERIFIED` | licença claramente determinável no repo/arquivos | seguir para próxima |
| `LICENSE_UNKNOWN` | licença não determinável | **PARAR** (`UNKNOWN ≠ PASS`) |
| `LEGAL_REVIEW_REQUIRED` | consequência jurídica depende de contexto | **PARAR** até revisão |
| `LICENSE_HETEROGENEOUS` | múltiplas licenças no repo | `QUARANTINE` ou `LEGAL_REVIEW_REQUIRED` |
| `QUARANTINED` | [aprendizado] incerto para uso | isolamento até decisão |

### 5.3 Distinções que o protocolo exige
O protocolo distingue, ao analisar um repo:
- **licença** · **copyright** · **conceito** · **comportamento observável** · **arquitetura** ·
  **padrão** · **algoritmo** · **implementação específica** · **código literal** ·
  **código derivado** · **documentação** · **assets** · **código de terceiros**.

**Regra operacional:**
- **CONCEITO / COMPORTAMENTO OBSERVÁVEL / ARQUITETURA / PADRÃO** → podem ser estudados como **abstração** (seguro).
- **IMPLEMENTAÇÃO ESPECÍFICA** → exige cautela, pode exigir revisão.
- **CÓDIGO LITERAL OU DERIVADO** → **NÃO** entra como conhecimento operacional; se detectado → `QUARANTINE`.

---

## REPOSITÓRIO = ENTIDADE COMPOSTA (POLICY)

**Falsa premissa corrigida:** ~~"o repo possui uma licença única"~~. Um repositório é um
**conjunto** que pode conter conteúdo sob licenças diferentes. Atenção a:
`LICENSE` · `COPYING` · `NOTICE` · **headers individuais** · `.gitmodules` · **submodules** ·
**vendored code** · **generated code** · **examples** · **assets** · **fonts** · **scripts** ·
**arquivos de terceiros**.

- A licença da **raiz não cobre automaticamente todos os conteúdos**.
- Se houver **heterogeneidade relevante** → `LICENSE_HETEROGENEOUS` → `QUARANTINE` ou `LEGAL_REVIEW_REQUIRED`.
- **NÃO assumir** que um repo MIT é todo MIT.

---

## ESTRELAS = SINAL, NÃO EVIDÊNCIA (POLICY)

**Removido:** ~~"≥ 1000 estrelas = forte"~~.

**Substituído por:**
> Estrelas/forks são **SINAIS de tração**. Nunca evidência suficiente para aprovação.
> A avaliação deve considerar (quando disponível): **manutenção · releases · atividade ·
> testes · CI · documentação · issues · licença · segurança · maturidade · adequação ao problema**.
> **Nenhum indicador isolado autoriza aquisição.**

---

## KNOWLEDGE ABSTRACTION BOUNDARY (POLICY)

Entre **EXPLORAÇÃO** e **CONHECIMENTO**, insere-se uma etapa formal de **abstração**:

```
REPOSITORY (externo)
   ↓
OBSERVATIONS (o que se viu)
   ↓
ABSTRACT SPECIFICATION (só comportamento/invariantes/trade-offs/requisitos)
   ↓
KNOWLEDGE (o que a casa guarda)
```

**A especificação abstrata contém SOMENTE:** comportamento · invariantes · interfaces
conceituais · trade-offs · decisões arquiteturais · requisitos · contra-padrões · melhorias.

**NÃO contém:** trechos de código · implementação literal · reprodução estrutural
desnecessária · cópia de documentação protegida.

> **Objetivo:** impedir que `"li → memorizei → reescrevi"` seja tratado automaticamente como
> `"não copiei"`. Se houver dúvida de derivação → `QUARANTINE` / `LEGAL_REVIEW_REQUIRED`.

---

## PROJECT WRITE GATE (POLICY)

Em **modo de aquisição/estudo**, o **PROJETO DO CLIENTE É READ-ONLY**. Proibido:
- importar dependência · adicionar pacote · copiar arquivo · modificar código ·
- adaptar implementação · instalar biblioteca · alterar arquitetura · criar integração.

> **Conhecimento adquirido NÃO autoriza `CODE_CHANGE`.** Qualquer escrita no projeto exige
> **ordem explícita** do Don/KERNEL via `PROJECT_WRITE_GATE`.

---

## EXECUTION GATE (POLICY)

Qualquer operação que possa **executar código externo** exige gate separado:
`npm/pnpm/yarn install` · `go build/run` (código externo) · scripts · `make` · `powershell` ·
`shell` · executáveis · binários · testes que executem código externo · hooks · plugins ·
ferramentas de build.

**Em modo de estudo:**
- `INSTALL = PROIBIDO por padrão`
- `EXECUTE = PROIBIDO por padrão`
- Excepcionalmente: `EXECUTION_GATE` + `ISOLATION` + `AUTHORITY_REQUIRED`.

> **Nunca transformar aquisição em autorização de execução.**

---

## ISOLAMENTO (POLICY + UNKNOWN)

**Distinguir:**
- **`TEMP` = localização temporária** (convenção de caminho — **NÃO é fronteira de segurança**).
- **`ISOLATED` = fronteira de segurança verificável.**

Ao explorar um clone, o protocolo **exige** que a propriedade de isolamento real seja
**comprovada** (filesystem separado, sem symlink/path-traversal/junction, sem execução de
hooks/subprocessos/scripts). Se **não puder ser comprovada**:

> **`ISOLATION_UNKNOWN`** — e **`UNKNOWN ≠ PASS`**.

> **NÃO inventar isolamento forte** se o código não provar. Para o `acquire` oficial,
> o isolamento real é parcial (fetch SSRF-safe + corpo em quarentena) — mas o **clone manual
> para exploração** não tem fronteira de filesystem comprovada neste protocolo.

---

## ORÇAMENTO (FACT + UNKNOWN)

- **FACT:** `cosca acquisition budget check` é um **diagnóstico read-only**; o `AcquisitionTracker`
  contabiliza fontes/arquivos/bytes/tempo/tokens e `Exceeded()` aborta a aquisição.
- **Onde é obrigatório:** dentro do `cosca knowledge acquire` (por `WithTracker`), quando o
  tracker está vinculado.
- **O que cobre:** a aquisição **via `cosca acquire`**.
- **Caminhos que escapam (`UNKNOWN`):** `git clone` direto, `curl`, `wget`,
  `Invoke-WebRequest`, e downloads por outras ferramentas **não são** contabilizados pelo
  tracker. O orçamento **não é uma proteção universal** contra vias alternativas.
- **`UNKNOWN`:** a cobertura do budget sobre `git clone`/`curl` não é verificável neste
  protocolo — declaramos `UNKNOWN`, sem inventar cobertura.

---

## PROVENIÊNCIA (POLICY + NOT_IMPLEMENTED)

O registro de proveniência deve capturar, **quando disponível**:

| Campo | Estado |
|---|---|
| URL exata | **FACT** (gravado em `AcquiredArtifact.URL`) |
| repository | **FACT** (manifesto) |
| branch/tag | **NOT_IMPLEMENTED** (não capturado hoje) |
| commit SHA completo | **NOT_IMPLEMENTED** |
| hash do conteúdo (SHA-256) | **FACT** (`AcquiredArtifact.SHA256`) |
| timestamp | **FACT** (`RetrievedAt`) |
| content-type | **FACT** (`ContentType`) |
| tamanho | **FACT** (`SizeBytes`) |
| licença detectada | **NOT_IMPLEMENTED** (não capturado hoje) |
| arquivos/diretórios estudados | **NOT_IMPLEMENTED** |
| ferramenta + versão | **NOT_IMPLEMENTED** |
| resultado de scanners | **NOT_IMPLEMENTED** |
| transformação para abstração | **NOT_IMPLEMENTED** |
| agente responsável | **NOT_IMPLEMENTED** |
| decisão final + motivo | **NOT_IMPLEMENTED** |
| confidence | **FACT** (no ledger de conhecimento) |

> **NOT_IMPLEMENTED** = o campo **ainda não existe** na implementação. O protocolo **não finge**.
> Os campos FACT são os que **realmente** o código grava hoje. Os demais são **requisitos de
> evolução**, marcados explicitamente.

---

## PROTOCOLO DE PARADA (POLICY)

**PARAR imediatamente** quando:
- autorização ausente;
- escopo excedido;
- licença relevante incerta (`LICENSE_UNKNOWN`/`LEGAL_REVIEW_REQUIRED`);
- segurança não avaliável (`SECURITY_UNKNOWN`);
- integridade não verificável;
- origem não estabelecida;
- isolamento não comprovado quando necessário (`ISOLATION_UNKNOWN`);
- operação exigir execução não autorizada (`EXECUTE` sem `EXECUTION_GATE`);
- ferramenta necessária inexistente;
- premissa importante contradita;
- código derivado/cópia detectado (`QUARANTINE`);
- resultado fora do contrato.

> **PARAR não é erro. PARAR é uma saída válida do protocolo.**

---

## TESTE ADVERSARIAL (cenários que o protocolo BLOQUEIA)

| # | Cenário | Gate responsável | Resultado |
|---|---|---|---|
| 1 | "Instalar para testar" | `DEPENDENCY_CHANGE_GATE` | `BLOCKED` |
| 2 | "Rodar build para entender" | `EXECUTION_GATE` | `BLOCKED` |
| 3 | "Copiar só uma função" | `KNOWLEDGE_ABSTRACTION_GATE` | `QUARANTINE` |
| 4 | "Reescrever exatamente igual" | `KNOWLEDGE_ABSTRACTION_GATE` | `QUARANTINE`/`LEGAL_REVIEW_REQUIRED` |
| 5 | "Usar dependência temporariamente" | `DEPENDENCY_CHANGE_GATE` | `BLOCKED` |
| 6 | "Ignorar licença porque é MIT" | `LICENSE_GATE` | `LEGAL_REVIEW_REQUIRED` |
| 7 | "Ignorar submodule" | `LICENSE_GATE`/`SECURITY_GATE` | `LICENSE_HETEROGENEOUS`/`QUARANTINED` |
| 8 | "Executar script do repo" | `EXECUTION_GATE` | `BLOCKED` |
| 9 | "Contornar budget com curl/git clone" | `REMOTE_ACCESS_GATE` (via via oficial) | `UNKNOWN`+`PARAR` |
| 10 | "Escrever no projeto depois do estudo" | `PROJECT_WRITE_GATE` | `BLOCKED` |
| 11 | "Continuar quando está UNKNOWN" | qualquer gate | `PARAR` (`UNKNOWN ≠ PASS`) |
| 12 | "Usar `--force` sem autoridade" | `AUTHORITY_REQUIRED` | `PARAR` |

---

## FLUXO COMPLETO (resumo)

```
Candidato
  → RESEARCH_GATE → CANDIDATE
  → LICENSE_GATE → LICENSE_VERIFIED (ou UNKNOWN/REVIEW => PARAR)
  → SECURITY_GATE → SECURITY_VERIFIED (ou QUARANTINE => PARAR)
  → REMOTE_ACCESS_GATE (autorização KERNEL) → ACQUISITION_AUTHORIZED
  → DOWNLOAD_GATE → ACQUIRED (fetch SSRF-safe doc oficial — FACT)
  → ISOLATION_GATE → ISOLATED (ou ISOLATION_UNKNOWN => PARAR)
  → EXTRACTION_GATE → ANALYZED (ler, abstrair — proibido copiar)
  → KNOWLEDGE_ABSTRACTION_GATE → ABSTRACTED (espec. abstrata, sem código)
  → KNOWLEDGE_GATE → KNOWLEDGE_ACCEPTED (conhecimento na casa)
  → PROVENANCE_GATE → PROVENANCE_RECORDED (metadados completos)
  · PROJETO DO CLIENTE: READ-ONLY (PROJECT_WRITE_GATE) no modo estudo
  · INSTALL/EXECUTE: PROIBIDOS (EXECUTION/DEPENDENCY gates) no modo estudo
```

---

## AUTO-AUDITORIA (checklist antes de usar o protocolo)

| Pergunta | Resposta do protocolo |
|---|---|
| Alguma certeza jurídica sem evidência? | **NÃO** — removidas; `LEGAL_REVIEW_REQUIRED` quando incerto. |
| Capacidade confundida com autorização? | **NÃO** — `CAPABILITY ≠ AUTHORITY ≠ ORDER ≠ CONSENT`. |
| `UNKNOWN` virou `PASS`? | **NÃO** — `UNKNOWN ≠ PASS`. |
| Execução escondida em aquisição? | **NÃO** — `acquire --compile/--embed` documentado como `ANALYSIS` (doc oficial), não execução de repo. |
| TEMP tratado como isolamento? | **NÃO** — `TEMP ≠ ISOLATED`; isolamento exige prova, senão `UNKNOWN`. |
| Instala dependência em modo estudo? | **NÃO** — `INSTALL` proibido por padrão. |
| Altera o projeto? | **NÃO** — `PROJECT_WRITE_GATE`; projeto read-only em estudo. |
| Repo heterogêneo passa como licença única? | **NÃO** — `LICENSE_HETEROGENEOUS`; repo = entidade composta. |
| Agente contorna o budget? | **Parcial** — budget real no `acquire`; `git clone`/`curl` são `UNKNOWN`. |
| Conhecimento vira código derivado? | **NÃO** — `KNOWLEDGE_ABSTRACTION_BOUNDARY`; `QUARANTINE` se derivado. |
| Proteção documentada sem existir? | **NÃO** — cada proteção marcada `FACT` (verificada) ou `UNKNOWN`/`NOT_IMPLEMENTED`. |
| Proteção real existente deixou de ser documentada? | **NÃO** — SSRF, UNTRUSTED, quarentena, budget, body-cap, redirects: todos `FACT` documentados. |

---

*Documento v1.1.0 — corrigido para refletir o comportamento REAL do código, com POLÍTICA de
governança explícita e UNKNOWNs honestos. Não inventa segurança, autoridade, licença ou isolamento.*
