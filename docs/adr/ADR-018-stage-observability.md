# ADR-018: Stage Observability — projeção do percurso de uma mudança (sem misturar as máquinas)

> **Status:** Proposed | **Owner:** cosca-kernel + cosca-architecture | **Last Updated:** 2026-08-28
> **Revisão:** aguardando Don + cosca-cto. **Design aditivo — não quebra o Root.**
> **Referência (base):** mineração `awslabs/aidlc-workflows` (lifecycle estruturado por estágios) + `ADR-016` (Evolution Engine) + `docs/roadmap/evo-plano-evolutivo.md` §1.

---

## 0. Contexto — o Cosca tem o veredicto, falta o percurso

O Cosca é **riquíssimo em gates por-concern** — `internal/proposal` (autorização ZERO-LLM, I1/I2),
`internal/quarantine` (admissão epistêmica, I6), `internal/skilleval` (avaliação, I1), `internal/deliberate`
(convergência). Cada um decide soberanamente. Mas o Cosca é **pobre no eixo do processo**: hoje não existe
uma espinha que diga *"em que estágio essa mudança está?"*. A condução de uma mudança é efeito colateral de
comandos ad-hoc (`cosca skill benchmark`, `cosca skill evolve`), sem visibilidade observável de *where are
we / which gate decides next / what's the artifact*.

O `aidlc-workflows` (AWS) resolveu isso com um **workflow de mudança estado-persistente, resumível e
por-artefato** (INCEPTION → CONSTRUCTION → OPERATIONS, cada fase com `stage` nomeado, artefato durável por
estágio, e "wait for explicit approval" entre estágios). A diferença de soberania: a validação do AI-DLC é
maioritariamente **humana + LLM**; a do Cosca é **gate ZERO-LLM**. Mineramos a *estrutura de lifecycle*, não
os workflows.

## 1. A tese — dois eixos que não se confundem

O termo "lifecycle" mistura dois eixos distintos:

| Eixo | Pergunta | Já projetado? |
|---|---|---|
| **A — lifecycle do artefato** | "a skill/capacidade pode ser ADMITIDA?" | ✅ Sim (`internal/skills/lifecycle.go`: proposta→quarentena→validada→ativa; `quarantine`; `skilleval` gate) |
| **B — lifecycle do processo** | "em que estágio a MUDANÇA está?" | ❌ **Não** — é aqui que entra |

**A tese:** o Cosca **NÃO precisa de uma máquina de Stage nova**; precisa de uma **PROJEÇÃO de Stage**
(mesmo padrão de `internal/skills/lifecycle.go`): somente-leitura, declarando um **roll ordenado** e fazendo
**um único contrato** — *cada avanço de stage referencia o gate determinístico que já decidiu, nunca
re-implementa*. Assim conecta **sem misturar**.

> **Condição de aceite:** o Stage é **observabilidade**, **nunca control plane**. Se o requisito for
> controlar a orquestração, usa-se o `internal/workflow` (DAG tipado) — que já existe; aí o Stage vira
> metadata do nó, não uma segunda máquina. A introdução de uma orquestração de stages que julgue com LLM
> ("qual o próximo stage? passou?") **violaria I1/I2** e é proibida.

## 2. A projeção — `internal/evolution`

```go
type Stage string // "task"|"requirement"|"design"|"implement"|"test"|"evaluate"|"promote"
var StageOrder = []Stage{ "task","requirement","design","implement","test","evaluate","promote" }

type StagePhase struct {
    Stage       Stage      // estágio atingido
    ArtifactRef string     // hash imutável (ProposalHash/EvidenceHash/commit) — dado, não autoridade
    GateUsed    string     // proposal|quarantine|skilleval|deliberate|workflow — referência, não reimplementa
    Verdict     string     // APPROVE/DENY | PASS/FAIL | EMIT_*
    At          time.Time
}

type StageRoll struct {   // projeção append-only
    Project   string
    Current   Stage
    Completed []StagePhase
    Next      Stage
    UpdatedAt time.Time
}

func CanStageAdvance(cur, next Stage) bool  // pura, determinística (I1): avança exatamente 1
func (r *StageRoll) Advance(phase) (*StageRoll, error)  // append-only, não muta o original
```

**Persistência (I5):** `StageRollStore` — JSONL append-only (last snapshot vence), espelhando a disciplina
do ledger. **Observabilidade:** `cosca stage status` / `cosca stage new <projeto>`.

## 3. Por que isto alimenta o Evolution Engine (G1/F2 do ADR-016)

O **G1** (lacuna nº 1 do ADR-016) é o *decoder* `execução → outcome → lesson → proposta → case .eval.yaml →
gate`. O decoder precisa saber **em que estágio a mudança está** para gerar o case e propor a evolução. A
projeção de Stage dá exatamente o **percurso** que o G1 consome — sem tocar na soberania dos gates. O `Next`
do roll indica para onde o `outcome` deve apontar; o `ArtifactRef` (hash) garante que o case é amarrado ao
artefato imutável da etapa.

## 4. ADOTAR / COMPARAR / REJEITAR (frente ao AI-DLC)

**✅ ADOTAR (sem tocar I1–I8):**
- **Artefato imutável por estágio** (`ArtifactRef = hash`) — já nativo no Cosca (binding anti-mutação,
  `provenance` I3, `ledger` I5). Reuso.
- **Estado persistente + resumível** (análogo ao `aidlc-state.md`) — `StageRoll` append-only, viabiliza G1.
- **Gate por estágio que referencia os gates ZERO-LLM existentes** — `CanStageAdvance` puro; quem julga
  continua sendo `skilleval`/`proposal`/`deliberate`. Reforça I1/I2.

**🔍 APENAS COMPARAR (já existe no Cosca):**
- Contexto empacotado por etapa → `internal/context`, `internal/sessionindex`.
- Auditoria raw por etapa → `internal/audit`, `internal/trace`.
- "Nunca deleta, só arquiva" → `internal/quarantine` (archive), `internal/skills/usage.go`.

**❌ REJEITAR (contraria invariantes):**
- Gate humano "wait for explicit approval" como gate primário (substituir determinismo por julgamento) — I1/I2.
- LLM/judge decidindo "próximo estágio" ou "o estágio passou" — I1/I2.
- Template rígido de 3 fases como script obrigatório — acoplamento; adota-se só o *vocabulário*.
- Fase "Operations" como estágio real agora — a entrega já é governada por `release`/`updater`.

## 5. O que NÃO fazer agora (escopo / anti-roadmap)

1. **NÃO criar `internal/evolution` como máquina controladora** que orquestra proposal/quarantine/skilleval
   (segue o precedente de `skills/lifecycle.go`: *só projeção, nenhum mecanismo paralelo de estado*).
2. **NÃO implementar um "stage judge" por LLM.**
3. **NÃO wirear o `StageRoll` no Evolution Engine antes de F1/F2** — o roll é *overlay* que F1 (skill status)
   e F2 (decoder outcome→proposta) **consomem**; não é pré-requisito.
4. **NÃO versionar o `StagePhase` como contrato externo** até haver consumidor real (o decoder G1) — evita
   dead abstraction.
5. **NÃO copiar a mecânica de 200 skills** (quantidade é consequência, não arquitetura).

## 6. Recomendação

1. **Adotar a projeção de Stage como observabilidade** (implementado em `internal/evolution` + `cosca stage`).
2. **Mantê-la desacoplada** dos gates: ela só observa e referencia; nunca decide.
3. **Consumir no G1/F2** (decoder outcome→proposta) quando essas fases entrarem, usando `ArtifactRef`+`Next`.

---

*O Cosca já tem o veredicto (gate ZERO-LLM por-concern); a projeção de Stage dá o percurso — conectando os
gates existentes sem misturá-los, e sem nunca deixar o LLM decidir o que os gates decidem.*
