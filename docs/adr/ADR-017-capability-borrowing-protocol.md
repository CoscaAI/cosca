# ADR-017: Capability Borrowing Protocol — mineração arquitetural como front-end do ADAPTER/GATE

> **Status:** Accepted | **Owner:** cosca-kernel + cosca-architecture | **Last Updated:** 2026-08-28
> **Revisão:** ratificado no Don + professor (2026-08-28). **Design aditivo — não quebra o Root.**
> **Referência (base):** `docs/roadmap/evo-plano-evolutivo.md` §1 (regra de admissão de capacidade externa), `ADR-016` (Evolution Engine), mineração `awslabs` (`mcp`, `graphrag-toolkit`, `aidlc-workflows`, `llrt`, `diagram-as-code`).

---

## 0. Contexto — por quê agora

O Cosca começou a minerar projetos externos como **fonte de capacidades arquiteturais**, não como coisas
para copiar. A postura que emergiu (voz do Don):

> **"Borrow capabilities, not assumptions."**

Olha-se um projeto e não se pergunta mais *"o que eu posso copiar?"*, e sim *"que ideia esse projeto
resolveu que pode melhorar o Cosca?"*. A consequência desejada e observada: quanto mais o Cosca minera
assim, **menos parecido com qualquer um deles** ele fica — porque não copia nenhuma arquitetura, seleciona
as soluções que **sobrevivem à comparação** e encaixa dentro dos próprios invariantes. Um ecossistema que
aprende com o resto do mundo **sem perder a identidade**.

O `ADR-016` já provou o arranque com o **Hermes** (evolution loop). Este ADR formaliza o **método** que
torna essa prática uma disciplina da família, e registra o **ledger** das ideias absorvidas.

## 1. Capability Crystallization Principle (tese do professor — ratificada)

> **"Não estamos colocando uma IA dentro do Cosca; estamos transformando capacidades que a IA demonstra
> em mecanismos verificáveis no código."** — professor/advisor do Don (2026-08-28).

Este é o **princípio-fundador** que governa tanto o Borrowing (mineração externa) quanto a internalização
das capacidades: **toda capacidade que a IA demonstra, o Cosca cristaliza em mecanismo verificável,
determinístico e auditável.** O LLM propõe/interpreta; o Cosca decide/prova.

| A IA demonstra | O Cosca cristaliza (mecanismo) | Onde vive |
|---|---|---|
| raciocínio | gates determinísticos | `internal/gate`, `internal/deliberate`, `ProofGate` |
| relações | grafo + busca + provenance | `internal/graph`, `search`, `provenance`, code-intelligence |
| avaliação de alternativas | proposal → decision → proof | `internal/proposal`, `decision`, `ProofGate` + `receipt` |
| possibilidade de erro | epistemic stamp + confidence + containment | `internal/worldmodel` (TrustState), conformal, belief containment |
| experimentação | checkpoint / rollback | `internal/evolution` (`Txn`/`Guard`) |
| aprendizado | evolution + ledger | `internal/evolution` (Stage, ProofGate), `internal/ledger` (I5) |
| entendimento de código | code-intelligence determinística | `internal/codeembed` + `codegraph` (int8, zero-LLM) |

**A propriedade que isso garante:** o modelo pode mudar amanhã (Qwen→DeepSeek→local→API), mas
**proveniência, gates, rollback, memória, segurança e regras de decisão pertencem ao Cosca** — não ao
prompt. É por isso que o Cosca fica mais independente **sem** aumentar o modelo.

**A inversão (a assinatura da casa):** *em vez de colocar mais sistema dentro da IA, colocar mais
inteligência aprendida como engenharia dentro do sistema.* 🧠⚙️

> **Regra de ouro:** capacidade da IA = **materia-prima**; mecanismo verificável = **produto**. Se uma
> capacidade não pode ser cristalizada (determinística, I1, auditável, I5), fica só como *proposta* —
> nunca como autoridade.**

## 2. A tese — o loop de mineração

A mineração **não é uma atividade paralela**. É a **porta de entrada** da cadeia de admissão que já está
escrita no `evo-plano-evolutivo.md` §1 (`EXTERNAL CAPABILITY → ADAPTER → INVARIANT CHECK → BENCHMARK →
EVOLUTION GATE → PROMOTE | REJECT`). O loop de mineração é o *front-end* dessa cadeia:

```
 PROJETO EXTERNO
      ↓
   IDEIA  (só a ideia resolvida — nunca a arquitetura)
      ↓
   HIPÓTESE  ("isso resolveria o quê no Cosca?")
      ↓
   COMPARAR COM O COSCA  (auditoria de reuso)
      ├─ já existe? ── SIM ──→ melhorar / compor
      └─ NÃO ─→ ADAPTER → INVARIANT CHECK (I1–I8) → BENCHMARK → EVOLUTION GATE → PROMOTE
```

**Regra que preserva a identidade:** se a ideia degradar **I1–I8**, é rejeitada, por melhor que seja.
Nada entra por analogia; entra por **comparação + evidência**.

## 3. O método em 6 passos (checklist da casa)

| # | Passo | O que produz | Fonte no Cosca |
|---|---|---|---|
| 1 | **Descobrir** | repos/artefatos relevantes (stars/licença/estado) | GitHub API / clone `--depth 1` |
| 2 | **Extrair a ideia** | o problema que o projeto resolveu, em 1–2 frases | README + código (arquitetura, não marketing) |
| 3 | **Hipótese Cosca** | "que princípio melhora o X do Cosca sem quebrar I1–I8?" | invariantes §1 evo-plano |
| 4 | **Auditoria de reuso** | JÁ EXISTE? (procuram-se as primitivas reais: `internal/...`) | `internal/*` |
| 5 | **Classificar** | `ADOTAR` / `COMPARAR` (já existe) / `ADAPTAR` / `REJEITAR` | comparação com primitivas |
| 6 | **Evidência, não fé** | medir ANTES/DEPOIS; só promover com dado | gate/bonchmark `internal/skilleval` |

## 4. Fronteira de autoridade — o que a capacidade externa NUNCA ganha

Aplicada a qualquer minerada (e a qualquer **Capability externa**, seja MCP, skill importada, plugin):

- **Não ganha autoridade no Kernel.** A ideia/capacidade entra por um **ADAPTER na borda de domínio**,
  nunca no Kernel. O cérebro (gate ZERO-LLM, I1) **decide**; a capacidade **entrega**.
- **Não ganha confiança automática.** Texto/resultado externo nasce `INFERRED`/`EXTERNAL` (I4) e só sobe a
  `MEASURED`/`VERIFIED` por **nossa própria medição** (I3/I4), nunca pela palavra do fornecedor.
- **Não ganha acesso irrestrito.** Default-deny + escopo mínimo (permissões explícitas por ferramenta).
- **Não altera memória diretamente.** Saída externa = conteúdo (`contenttrust` I8); escrita em
  memória/conhecimento passa pelo caminho `proposal → quarantine → validation` normal.
- **Não bypassa proposal/gate/sandbox.** Todo avanço passa pelos gates determinísticos existentes;
  falha/ambiguidade = nega (I2); execução dentro da auto-jail/sandbox (I7).

## 5. Ledger de mineração — o que o Cosca absorveu até aqui

| Projeto externo | Ideia absorvível | Tratamento | Eixo | Status |
|---|---|---|---|---|
| **Hermes Agent** | evolution loop (experiência→skill→gate→ativa) | **ADOTAR** (F1/ADR-016) | Aprendizado | ✅ |
| **mcp (awslabs)** | fronteira padronizada de capacidade externa | **ADAPTAR** (adapter de capacidade; MCP = transporte, não autoridade) | Capacidade | 🔵 design |
| **graphrag-toolkit** | grafo léxico em 3 camadas + traversal-based search | **ADAPTAR** (padrões → kernel semântico; não a lib Python/AWS) | Conhecimento | 🔵 design |
| **aidlc-workflows** | Stage observável (percurso do processo, não máquina nova) | **ADAPTAR** (projeção de Stage; alimenta G1/F2 do ADR-016) | Lifecycle | 🔵 design |
| **llrt** | cold-path O(invocação); medir; pré-aquecer; adiar/gatear | **ADOTAR** (princípio) + 1 bug corrigido | Execução | ✅ (parcial) |
| **diagram-as-code** | diagrama-as-código com auto-layout (imagem publicável) | **ADOTAR como ferramenta** (CLI/MCP; não lib Go) | Docs | 🔵 candidata |
| **git-secrets** | vazamento como enforcement no fluxo | **REFERÊNCIA** (segurança/compliance) | Segurança | 🟢 candidata |

Cada projeto traz uma peça de um eixo **diferente**. O Cosca **compõe** as peças que sobrevivem à
comparação — não vira nenhum deles.

## 6. Precedente concreto — a mineração LLRT → ganho real medido

O primeiro diamante colhido do ciclo: o alvo `build` da `Makefile` compilava o binário **de produção**
com `-gcflags="all=-N -l"` (otimização e inlining desligados — artefato de debug vazado, e contraditório
com o `-w -s` do LDFLAGS). Medição ANTES/DEPOIS (mesmo binário, só flags):

| Métrica | Antes | Depois | Ganho |
|---|---|---|---|
| cold-start (`version`, mediana) | ~167 ms | ~89 ms | **-47%** |
| binário | 89,5 MB | 74,2 MB | **-17%** |
| build | 9,5 s | 4,3 s | **-55%** |

Fluxo do protocolo na prática: **IDEIA** (LLRT otimiza cold-path) → **HIPÓTESE** (o Cosca otimiza o
cold-path? mede esse custo?) → **COMPARAR** (o que já existe: wazero, sandbox) → **achado** (o build
desotimizado era um bug de cold-path real) → **EVIDÊNCIA** (mediu-se antes/depois) → **PROMOTE** (commit
`64f340a`). Resultado: Cosca mais rápido, sem dependência nova, sem risco de invariante.

## 7. Escopo / o que NÃO fazer (disciplina)

- **NÃO copiar arquitetura nem código** de projeto externo no Cosca (Apache-2.0 observado; binário único
  preservado; nenhum sidecar Python/JS no Kernel). Minera-se a **ideia**; adapta-se aos padrões próprios.
- **NÃO trocar runtime** (ex.: não adotar QuickJS "porque o LLRT ganha cold-start" — o Cosca é Go+WASM).
- **NÃO criar máquina de estado paralela** para lifecycle/capability (reusa `lifecycle.go`/`quarantine`/
  `skilleval`/`proposal`/`ledger`).
- **NÃO deixar LLM julgar o gate** (I1/I2) nem classificar risco de capacidade **por nome** (heurística
  contornável).
- **NÃO promover sem evidência** (medir ANTES/DEPOIS; o Cosca é *evidence-gated*).
- **NÃO acumular quantidade** de skills/capacidades por copiar (quantidade é consequência, não arquitetura).

## 8. Recomendação

1. **Adotar como protocolo permanente da família** — a mineração é o front-end do ADAPTER/GATE.
2. **Manter o ledger** (§5) como fonte de verdade do que foi absorvido, com proveniência.
3. **Prioridade de design (próximas mineradas)** — em ordem de valor: **(a)** AI-DLC → Stage observável
   (alimenta G1/F2); **(b)** LLRT → Ephemeral Unit Pre-warm + Cold-Start Contract no gate; **(c)** git-secrets
   → enforcement de vazamento no fluxo (segurança/compliance).
4. **Nenhuma** dessas entra sem ADAPTER + INVARIANT CHECK + BENCHMARK + GATE, e sem respeitar a fronteira
   de autoridade (§4).

---

*Encerramento do mérito: o Cosca aprende com o resto do mundo sem perder a identidade — porque seleciona,
não copia; e porque o cérebro (gate ZERO-LLM) continua soberano sobre tudo que vem de fora.*
